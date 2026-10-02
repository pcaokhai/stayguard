package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	sepaySecretField = "sepay_secret"
	auditInstaller   = "INSTALLER_SEPAY_UPDATED"
	auditImport      = "INSTALLER_TENANT_IMPORTED"
)

var (
	// ErrTenantUnknown: no guesthouse has that code.
	ErrTenantUnknown = errors.New("unknown guesthouse code")
	// ErrAccountChoice: --account is needed because the guesthouse has no single account to connect.
	ErrAccountChoice = errors.New("name the account to connect with --account")
)

// NewTenant is a guesthouse to create.
type NewTenant struct{ ID, Name, GuesthouseCode, HookID, Locale, TimeZone string }

// TenantSetupRepo creates the tenant row an import starts from.
type TenantSetupRepo interface {
	InsertTenant(ctx context.Context, tx Tx, t NewTenant) error
}

// Installer is what the installer CLI does on the server: import a guesthouse, print its webhook address,
// store the SePay secret and read the connection. Every change leaves an INSTALLER audit entry and, for
// SePay, an alert the owner sees. The secret is write-only: nothing here returns it.
type Installer struct {
	uow     UnitOfWork
	tenants TenantByCode
	setup   TenantSetupRepo
	layout  DemoSeedRepo
	bank    BankRepo
	staff   StaffRepo
	authDB  AuthRepo
	enc     Encryptor
	hasher  PinHasher
	pins    PinGenerator
	tokens  TokenGenerator
	audit   AuditWriter
	alerts  AlertWriter
	ids     IDGenerator
	clock   Clock
}

func NewInstaller(uow UnitOfWork, tenants TenantByCode, setup TenantSetupRepo, layout DemoSeedRepo, bank BankRepo, staff StaffRepo,
	authDB AuthRepo, enc Encryptor, hasher PinHasher, pins PinGenerator, tokens TokenGenerator, audit AuditWriter, alerts AlertWriter,
	ids IDGenerator, clock Clock) *Installer {
	return &Installer{uow, tenants, setup, layout, bank, staff, authDB, enc, hasher, pins, tokens, audit, alerts, ids, clock}
}

func (i *Installer) tenant(ctx context.Context, code string) (string, error) {
	id, ok, err := i.tenants.TenantByCode(ctx, code)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrTenantUnknown
	}
	return id, nil
}

// WebhookPath is the path part of the webhook address for a guesthouse: /v1/webhooks/bank/<hookId>.
func (i *Installer) WebhookPath(ctx context.Context, code string) (string, error) {
	id, err := i.tenant(ctx, code)
	if err != nil {
		return "", err
	}
	var hook string
	err = i.uow.Do(ctx, id, func(ctx context.Context, tx Tx) error {
		st, err := i.bank.SepayState(ctx, tx)
		if err != nil {
			return err
		}
		if hook = st.HookID; hook != "" {
			return nil
		}
		if hook, err = i.tokens.New(); err != nil {
			return err
		}
		return i.bank.SetHook(ctx, tx, hook)
	})
	return "/v1/webhooks/bank/" + hook, err
}

// SetSecret stores the provider secret encrypted and connects one receiving account: the named one, or the
// only account the guesthouse has. An account flagged make-default-when-connected becomes the QR default.
func (i *Installer) SetSecret(ctx context.Context, code, secret, accountID string) error {
	if secret == "" {
		return &ValidationError{"secret", "required"}
	}
	id, err := i.tenant(ctx, code)
	if err != nil {
		return err
	}
	enc, err := i.enc.Encrypt(id, sepaySecretField, []byte(secret))
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}
	return i.uow.Do(ctx, id, func(ctx context.Context, tx Tx) error {
		acc, err := i.accountToConnect(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if err = i.bank.SetSecret(ctx, tx, enc); err != nil {
			return err
		}
		if err = i.bank.Connect(ctx, tx, acc.ID); err != nil {
			return err
		}
		if acc.MakeDefaultWhenConnected {
			if _, err = i.bank.SetDefault(ctx, tx, acc.ID); err != nil {
				return err
			}
		}
		now := i.clock.Now()
		if err = i.alerts.Raise(ctx, tx, AlertDraft{ID: i.ids.New(alertIDPrefix), Kind: AlertSepayUpdated,
			Details: map[string]string{"at": now.UTC().Format(time.RFC3339)}}); err != nil {
			return err
		}
		return i.audit.Append(ctx, tx, AuditEntry{ID: i.ids.New(auditPrefix), Action: auditInstaller, EntityType: "bank_account", EntityID: acc.ID})
	})
}

func (i *Installer) accountToConnect(ctx context.Context, tx Tx, id string) (BankAccountRow, error) {
	if id != "" {
		row, ok, err := i.bank.Account(ctx, tx, id)
		if err != nil || !ok {
			return BankAccountRow{}, errOr(err, ErrNotFound)
		}
		return row, nil
	}
	rows, err := i.bank.ListAccounts(ctx, tx)
	if err != nil {
		return BankAccountRow{}, err
	}
	if len(rows) != 1 {
		return BankAccountRow{}, ErrAccountChoice
	}
	return rows[0], nil
}

// InstallerStatus is the connection state for the CLI.
type InstallerStatus struct {
	SepayState
	Accounts []BankAccountView
}

func (i *Installer) Status(ctx context.Context, code string) (InstallerStatus, error) {
	id, err := i.tenant(ctx, code)
	if err != nil {
		return InstallerStatus{}, err
	}
	var out InstallerStatus
	err = i.uow.Do(ctx, id, func(ctx context.Context, tx Tx) error {
		if out.SepayState, err = i.bank.SepayState(ctx, tx); err != nil {
			return err
		}
		rows, err := i.bank.ListAccounts(ctx, tx)
		for _, r := range rows {
			out.Accounts = append(out.Accounts, r.BankAccountView)
		}
		return err
	})
	return out, err
}

// installerAudit is the entry written once per import: ids only.
func (i *Installer) importAudit(ctx context.Context, tx Tx, tenantID string) error {
	after, _ := json.Marshal(map[string]string{"tenantId": tenantID})
	return i.audit.Append(ctx, tx, AuditEntry{ID: i.ids.New(auditPrefix), Action: auditImport, EntityType: "tenant", EntityID: tenantID, After: after})
}
