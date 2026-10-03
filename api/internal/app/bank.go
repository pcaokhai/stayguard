package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
)

const (
	bankRoute        = "POST /v1/owner/bank-accounts"
	bankIDPrefix     = "ba"
	statusConnected  = "CONNECTED"
	statusPending    = "PENDING"
	maxAccountName   = 60
	minAccountNo     = 6
	maxAccountNo     = 20
	maxPropertyField = 200
)

// bankAccountFieldOf binds an encrypted account to its tenant and id.
func bankAccountFieldOf(id string) string { return bankAccountField + "/" + id }

// Bank is property settings, receiving accounts and the SePay status for the owner.
type Bank struct {
	uow   UnitOfWork
	repo  BankRepo
	auth  *Auth
	enc   Encryptor
	idem  IdempotencyStore
	audit AuditWriter
	ids   IDGenerator
	clock Clock
	authz access.Authorizer
}

func NewBank(uow UnitOfWork, repo BankRepo, auth *Auth, enc Encryptor, idem IdempotencyStore, audit AuditWriter, ids IDGenerator, clock Clock) *Bank {
	return &Bank{uow: uow, repo: repo, auth: auth, enc: enc, idem: idem, audit: audit, ids: ids, clock: clock}
}

func (b *Bank) GetProperty(ctx context.Context, c Caller) (PropertyView, error) {
	if err := b.authz.Check("getProperty", c.Role, access.EDIT); err != nil {
		return PropertyView{}, err
	}
	var v PropertyView
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var ok bool
		var err error
		if v, ok, err = b.repo.Property(ctx, tx); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		return nil
	})
	return v, err
}

func inRange(v *int, lo, hi int) bool { return v == nil || (*v >= lo && *v <= hi) }

func (b *Bank) UpdateProperty(ctx context.Context, c Caller, p PropertyPatch) (PropertyView, error) {
	if err := b.authz.Check("updateProperty", c.Role, access.EDIT); err != nil {
		return PropertyView{}, err
	}
	switch {
	case p.Name != nil && (strings.TrimSpace(*p.Name) == "" || len(*p.Name) > maxPropertyField):
		return PropertyView{}, &ValidationError{"name", "required, at most 200 characters"}
	case p.Address != nil && len(*p.Address) > maxPropertyField, p.Phone != nil && len(*p.Phone) > maxPropertyField:
		return PropertyView{}, &ValidationError{"address", "too long"}
	case !inRange(p.QRExpiryMinutes, 5, 240):
		return PropertyView{}, &ValidationError{"qrExpiryMinutes", "between 5 and 240"}
	case !inRange(p.IDRetentionDays, 1, 365):
		return PropertyView{}, &ValidationError{"idRetentionDays", "between 1 and 365"}
	case !inRange(p.FrontDeskHistoryDays, 1, 90):
		return PropertyView{}, &ValidationError{"frontDeskHistoryDays", "between 1 and 90"}
	}
	var v PropertyView
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, ok, err := b.repo.Property(ctx, tx)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if err = b.repo.UpdateProperty(ctx, tx, cur.ID, p); err != nil {
			return err
		}
		if v, _, err = b.repo.Property(ctx, tx); err != nil {
			return err
		}
		return b.record(ctx, tx, c, auditPropertyUpdated, "property", cur.ID, nil)
	})
	return v, err
}

func (b *Bank) ListAccounts(ctx context.Context, c Caller) ([]BankAccountView, error) {
	if err := b.authz.Check("listBankAccounts", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	var out []BankAccountView
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := b.repo.ListAccounts(ctx, tx)
		if err != nil {
			return err
		}
		out = make([]BankAccountView, len(rows))
		for i, r := range rows {
			out[i] = r.BankAccountView
		}
		return nil
	})
	return out, err
}

// BankAccountInput is the body of createBankAccount. AccountNo is write-only.
type BankAccountInput struct {
	OwnerPin, BankBin, AccountNo, AccountName string
	MakeDefaultWhenConnected                  bool
}

func digits(s string, lo, hi int) bool {
	if len(s) < lo || len(s) > hi {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func validateAccountInput(in BankAccountInput) error {
	switch {
	case access.ValidatePinFormat(in.OwnerPin) != nil:
		return &ValidationError{"ownerPin", "must be six digits"}
	case !digits(in.BankBin, 6, 6):
		return &ValidationError{"bankBin", "six digits"}
	case !digits(in.AccountNo, minAccountNo, maxAccountNo):
		return &ValidationError{"accountNo", "6 to 20 digits"}
	case strings.TrimSpace(in.AccountName) == "" || len(in.AccountName) > maxAccountName:
		return &ValidationError{"accountName", "required, at most 60 characters"}
	}
	return nil
}

// CreateAccount adds a PENDING account; it becomes the QR default only after the installer connects it.
func (b *Bank) CreateAccount(ctx context.Context, c Caller, key string, in BankAccountInput) (BankAccountView, error) {
	if err := b.authz.Check("createBankAccount", c.Role, access.EDIT); err != nil {
		return BankAccountView{}, err
	}
	if key == "" || len(key) > maxIdempotencyKeyBytes {
		return BankAccountView{}, ErrInvalidIdempotencyKey
	}
	if err := validateAccountInput(in); err != nil {
		return BankAccountView{}, err
	}
	ctx = context.WithoutCancel(ctx) // a cancelled request still counts a wrong owner PIN
	var view BankAccountView
	var outcome error
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		failure, err := b.auth.requireOwnerPin(ctx, tx, c.UserID, in.OwnerPin, b.clock.Now())
		if err != nil || failure != nil {
			outcome = failure
			return err
		}
		fp := hex.EncodeToString(b.enc.Fingerprint(tx.TenantID(), bankAccountField, []byte(in.BankBin+"/"+in.AccountNo)))
		body, _ := json.Marshal(map[string]any{"bankBin": in.BankBin, "accountFingerprint": fp, "accountName": in.AccountName, "makeDefaultWhenConnected": in.MakeDefaultWhenConnected})
		oc, err := b.idem.Begin(ctx, tx, bankRoute, key, RequestHash(body))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &view)
		}
		if view, err = b.insert(ctx, tx, c, in, fp); err != nil {
			return err
		}
		stored, err := json.Marshal(view)
		if err != nil {
			return fmt.Errorf("encode bank account response: %w", err)
		}
		return b.idem.Complete(ctx, tx, bankRoute, key, statusCreated, stored)
	})
	if err != nil {
		return BankAccountView{}, err
	}
	return view, outcome
}

func (b *Bank) insert(ctx context.Context, tx Tx, c Caller, in BankAccountInput, fp string) (BankAccountView, error) {
	id := b.ids.New(bankIDPrefix)
	plain, err := json.Marshal(map[string]string{"bankBin": in.BankBin, "accountNo": in.AccountNo, "accountName": in.AccountName})
	if err != nil {
		return BankAccountView{}, fmt.Errorf("encode bank account: %w", err)
	}
	enc, err := b.enc.Encrypt(tx.TenantID(), bankAccountFieldOf(id), plain)
	if err != nil {
		return BankAccountView{}, fmt.Errorf("encrypt bank account: %w", err)
	}
	a := NewBankAccount{ID: id, BankBin: in.BankBin, BankName: payment.BankName(in.BankBin), Masked: maskAccount(in.AccountNo),
		Name: in.AccountName, Fingerprint: fp, Enc: enc, MakeDefaultWhenConnected: in.MakeDefaultWhenConnected}
	if err = b.repo.InsertAccount(ctx, tx, a); err != nil {
		return BankAccountView{}, err
	}
	if err = b.record(ctx, tx, c, auditBankAccountAdded, "bank_account", id, nil); err != nil {
		return BankAccountView{}, err
	}
	return BankAccountView{ID: id, BankBin: a.BankBin, BankName: a.BankName, AccountNoMasked: a.Masked, AccountName: a.Name, SepayStatus: statusPending}, nil
}

// MakeDefault makes a connected account the QR default after the owner re-enters their PIN.
func (b *Bank) MakeDefault(ctx context.Context, c Caller, id, ownerPin string) (BankAccountView, error) {
	return b.change(ctx, c, "makeDefaultBankAccount", id, ownerPin, auditBankAccountDefault, func(ctx context.Context, tx Tx, row BankAccountRow) error {
		if row.SepayStatus != statusConnected {
			return ErrConflict // rule 19: only after the installer connects SePay
		}
		if ok, err := b.repo.SetDefault(ctx, tx, id); err != nil || !ok {
			return errOr(err, ErrConflict)
		}
		return nil
	})
}

// RemoveAccount deletes a non-default account after the owner re-enters their PIN.
func (b *Bank) RemoveAccount(ctx context.Context, c Caller, id, ownerPin string) error {
	_, err := b.change(ctx, c, "removeBankAccount", id, ownerPin, auditBankAccountRemoved, func(ctx context.Context, tx Tx, row BankAccountRow) error {
		if row.IsDefault {
			return ErrConflict // the default cannot be removed
		}
		if ok, err := b.repo.DeleteAccount(ctx, tx, id); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		return nil
	})
	return err
}

// change runs one owner-PIN-protected change on an existing account and audits it.
func (b *Bank) change(ctx context.Context, c Caller, op, id, ownerPin, action string, apply func(context.Context, Tx, BankAccountRow) error) (BankAccountView, error) {
	if err := b.authz.Check(op, c.Role, access.EDIT); err != nil {
		return BankAccountView{}, err
	}
	if access.ValidatePinFormat(ownerPin) != nil {
		return BankAccountView{}, &ValidationError{"ownerPin", "must be six digits"}
	}
	ctx = context.WithoutCancel(ctx) // a cancelled request still counts a wrong owner PIN
	var view BankAccountView
	var outcome error
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		failure, err := b.auth.requireOwnerPin(ctx, tx, c.UserID, ownerPin, b.clock.Now())
		if err != nil || failure != nil {
			outcome = failure
			return err
		}
		row, ok, err := b.repo.Account(ctx, tx, id)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if err = apply(ctx, tx, row); err != nil {
			return err
		}
		if now, ok, gerr := b.repo.Account(ctx, tx, id); gerr == nil && ok {
			view = now.BankAccountView
		}
		return b.record(ctx, tx, c, action, "bank_account", id, nil)
	})
	return view, errOr(err, outcome)
}

// SepayStatus is the connection of the default account.
func (b *Bank) SepayStatus(ctx context.Context, c Caller) (SepayState, error) {
	if err := b.authz.Check("getSepayStatus", c.Role, access.EDIT); err != nil {
		return SepayState{}, err
	}
	var s SepayState
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		s, err = b.repo.SepayState(ctx, tx)
		return err
	})
	return s, err
}

// record appends an audit row with ids only: never an account number, name or PIN.
func (b *Bank) record(ctx context.Context, tx Tx, c Caller, action, entity, entityID string, after map[string]any) error {
	e := AuditEntry{ID: b.ids.New(auditPrefix), ActorID: c.UserID, Action: action, EntityType: entity, EntityID: entityID}
	if after != nil {
		raw, err := json.Marshal(after)
		if err != nil {
			return err
		}
		e.After = raw
	}
	return b.audit.Append(ctx, tx, e)
}
