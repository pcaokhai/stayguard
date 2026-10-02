package app

import (
	"context"
	"time"
)

// BankAccountView is a receiving account without its number.
type BankAccountView struct {
	ID, BankBin, BankName, AccountNoMasked, AccountName string
	IsDefault                                           bool
	SepayStatus                                         string // CONNECTED or PENDING
	LastWebhookAt                                       *time.Time
}

// BankAccountRow adds what the use cases need to decide.
type BankAccountRow struct {
	BankAccountView
	MakeDefaultWhenConnected bool
}

// NewBankAccount is an account to store; Enc is the encrypted JSON of bank, number and name.
type NewBankAccount struct {
	ID, BankBin, BankName, Masked, Name, Fingerprint string
	Enc                                              []byte
	IsDefault, Connected, MakeDefaultWhenConnected   bool
}

// PropertyView is the guesthouse details the owner edits.
type PropertyView struct {
	ID, GuesthouseCode, Name                               string
	Address, Phone                                         *string
	QRExpiryMinutes, IDRetentionDays, FrontDeskHistoryDays int
}

// PropertyPatch changes the given fields only.
type PropertyPatch struct {
	Name, Address, Phone                                   *string
	QRExpiryMinutes, IDRetentionDays, FrontDeskHistoryDays *int
}

// SepayState is the connection of the tenant as the installer and the owner see it.
type SepayState struct {
	DefaultConnected bool
	HasSecret        bool
	HookID           string
	LastWebhookAt    *time.Time
	SignatureOK      *bool
}

// BankRepo is tenant-scoped like IdentityRepo.
type BankRepo interface {
	Property(ctx context.Context, tx Tx) (PropertyView, bool, error)
	UpdateProperty(ctx context.Context, tx Tx, id string, p PropertyPatch) error
	ListAccounts(ctx context.Context, tx Tx) ([]BankAccountRow, error)
	// Account locks the row; false when unknown.
	Account(ctx context.Context, tx Tx, id string) (BankAccountRow, bool, error)
	// InsertAccount reports ErrConflict when the same account (fingerprint) exists.
	InsertAccount(ctx context.Context, tx Tx, a NewBankAccount) error
	// SetDefault makes a CONNECTED account the only default; false when it is not connected.
	SetDefault(ctx context.Context, tx Tx, id string) (bool, error)
	Connect(ctx context.Context, tx Tx, id string) error
	// DeleteAccount deletes a non-default account; false when there was none.
	DeleteAccount(ctx context.Context, tx Tx, id string) (bool, error)
	SepayState(ctx context.Context, tx Tx) (SepayState, error)
	SetHook(ctx context.Context, tx Tx, hookID string) error
	SetSecret(ctx context.Context, tx Tx, enc []byte) error
	// SecretEnc is the encrypted webhook secret (nil when none is set); only the webhook check reads it.
	SecretEnc(ctx context.Context, tx Tx) ([]byte, error)
	AccountBlobs(ctx context.Context, tx Tx) ([]AccountBlob, error)
	TouchWebhook(ctx context.Context, tx Tx, accountID string, at time.Time) error
	SetSignatureOK(ctx context.Context, tx Tx, ok bool) error
}

// AccountBlob is a receiving account as stored: the encrypted JSON of bank, number and name.
type AccountBlob struct {
	ID  string
	Enc []byte
}

// TenantByHook finds a tenant from its webhook id before any tenant is known (policy tenants_hook_lookup).
type TenantByHook interface {
	TenantByHook(ctx context.Context, hookID string) (tenantID string, found bool, err error)
}
