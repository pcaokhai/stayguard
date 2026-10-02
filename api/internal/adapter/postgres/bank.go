package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// BankRepo implements app.BankRepo; the tenant always comes from the Tx.
type BankRepo struct{}

var _ app.BankRepo = BankRepo{}

func (BankRepo) Property(ctx context.Context, tx app.Tx) (app.PropertyView, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.PropertyView{}, false, err
	}
	r, err := sqlcgen.New(t).GetProperty(ctx, t.tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PropertyView{}, false, nil
	}
	if err != nil {
		return app.PropertyView{}, false, wrap("select property", err)
	}
	return app.PropertyView{ID: r.ID, GuesthouseCode: r.GuesthouseCode.String, Name: r.Name, Address: textPtr(r.Address), Phone: textPtr(r.Phone),
		QRExpiryMinutes: int(r.QrExpiryMinutes), IDRetentionDays: int(r.IDRetentionDays), FrontDeskHistoryDays: int(r.FrontDeskHistoryDays)}, true, nil
}

func (BankRepo) UpdateProperty(ctx context.Context, tx app.Tx, id string, p app.PropertyPatch) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("update property", sqlcgen.New(t).UpdateProperty(ctx, sqlcgen.UpdatePropertyParams{
		Name: text(p.Name), Address: text(p.Address), Phone: text(p.Phone),
		QrExpiryMinutes: optInt4(p.QRExpiryMinutes), IDRetentionDays: optInt4(p.IDRetentionDays), FrontDeskHistoryDays: optInt4(p.FrontDeskHistoryDays),
		TenantID: t.tenant, ID: id,
	}))
}

func (BankRepo) ListAccounts(ctx context.Context, tx app.Tx) ([]app.BankAccountRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListBankAccounts(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list bank accounts", err)
	}
	out := make([]app.BankAccountRow, len(rows))
	for i, r := range rows {
		out[i] = app.BankAccountRow{BankAccountView: app.BankAccountView{ID: r.ID, BankBin: r.BankBin, BankName: r.BankName,
			AccountNoMasked: r.AccountNoMasked, AccountName: r.AccountName, IsDefault: r.IsDefault, SepayStatus: r.SepayStatus,
			LastWebhookAt: optTime(r.LastWebhookAt)}, MakeDefaultWhenConnected: r.MakeDefaultWhenConnected}
	}
	return out, nil
}

func (BankRepo) Account(ctx context.Context, tx app.Tx, id string) (app.BankAccountRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.BankAccountRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetBankAccount(ctx, sqlcgen.GetBankAccountParams{TenantID: t.tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.BankAccountRow{}, false, nil
	}
	if err != nil {
		return app.BankAccountRow{}, false, wrap("select bank account", err)
	}
	return app.BankAccountRow{BankAccountView: app.BankAccountView{ID: r.ID, BankBin: r.BankBin, BankName: r.BankName,
		AccountNoMasked: r.AccountNoMasked, AccountName: r.AccountName, IsDefault: r.IsDefault, SepayStatus: r.SepayStatus,
		LastWebhookAt: optTime(r.LastWebhookAt)}, MakeDefaultWhenConnected: r.MakeDefaultWhenConnected}, true, nil
}

func (BankRepo) InsertAccount(ctx context.Context, tx app.Tx, a app.NewBankAccount) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	status := "PENDING"
	if a.Connected {
		status = "CONNECTED"
	}
	return wrap("insert bank account", sqlcgen.New(t).InsertBankAccount(ctx, sqlcgen.InsertBankAccountParams{
		ID: a.ID, TenantID: t.tenant, BankBin: a.BankBin, BankName: a.BankName, AccountEnc: a.Enc, AccountNoMasked: a.Masked,
		AccountName: a.Name, AccountFp: a.Fingerprint, IsDefault: a.IsDefault, SepayStatus: status,
		MakeDefaultWhenConnected: a.MakeDefaultWhenConnected,
	}))
}

func (BankRepo) SetDefault(ctx context.Context, tx app.Tx, id string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	q := sqlcgen.New(t)
	if err = q.ClearDefaultBankAccount(ctx, t.tenant); err != nil {
		return false, wrap("clear default bank account", err)
	}
	n, err := q.SetDefaultBankAccount(ctx, sqlcgen.SetDefaultBankAccountParams{TenantID: t.tenant, ID: id})
	return n > 0, wrap("set default bank account", err)
}

func (BankRepo) Connect(ctx context.Context, tx app.Tx, id string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("connect bank account", sqlcgen.New(t).ConnectBankAccount(ctx, sqlcgen.ConnectBankAccountParams{TenantID: t.tenant, ID: id}))
}

func (BankRepo) DeleteAccount(ctx context.Context, tx app.Tx, id string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).DeleteBankAccount(ctx, sqlcgen.DeleteBankAccountParams{TenantID: t.tenant, ID: id})
	return n > 0, wrap("delete bank account", err)
}

func (BankRepo) SepayState(ctx context.Context, tx app.Tx) (app.SepayState, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.SepayState{}, err
	}
	r, err := sqlcgen.New(t).GetSepayState(ctx, t.tenant)
	if err != nil {
		return app.SepayState{}, wrap("select sepay state", err)
	}
	s := app.SepayState{DefaultConnected: r.DefaultConnected, HasSecret: r.HasSecret,
		HookID: r.HookID.String, LastWebhookAt: optTime(r.LastWebhookAt)}
	if r.SepaySignatureOk.Valid {
		s.SignatureOK = &r.SepaySignatureOk.Bool
	}
	return s, nil
}

func (BankRepo) SetHook(ctx context.Context, tx app.Tx, hookID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set hook id", sqlcgen.New(t).SetTenantHook(ctx, sqlcgen.SetTenantHookParams{HookID: text(&hookID), TenantID: t.tenant}))
}

func (BankRepo) SetSecret(ctx context.Context, tx app.Tx, enc []byte) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set sepay secret", sqlcgen.New(t).SetSepaySecret(ctx, sqlcgen.SetSepaySecretParams{SepaySecretEnc: enc, TenantID: t.tenant}))
}

// TenantSetupRepo implements app.TenantSetupRepo.
type TenantSetupRepo struct{}

var _ app.TenantSetupRepo = TenantSetupRepo{}

func (TenantSetupRepo) InsertTenant(ctx context.Context, tx app.Tx, n app.NewTenant) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert tenant", sqlcgen.New(t).InsertTenant(ctx, sqlcgen.InsertTenantParams{ID: n.ID, Name: n.Name,
		GuesthouseCode: text(&n.GuesthouseCode), HookID: text(&n.HookID), DefaultLocale: n.Locale, TimeZone: n.TimeZone}))
}

func (BankRepo) SecretEnc(ctx context.Context, tx app.Tx) ([]byte, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	b, err := sqlcgen.New(t).GetSepaySecretEnc(ctx, t.tenant)
	return b, wrap("select sepay secret", err)
}

func (BankRepo) AccountBlobs(ctx context.Context, tx app.Tx) ([]app.AccountBlob, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListBankAccountBlobs(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list account blobs", err)
	}
	out := make([]app.AccountBlob, len(rows))
	for i, r := range rows {
		out[i] = app.AccountBlob{ID: r.ID, Enc: r.AccountEnc}
	}
	return out, nil
}

func (BankRepo) TouchWebhook(ctx context.Context, tx app.Tx, accountID string, at time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("touch webhook", sqlcgen.New(t).TouchBankAccountWebhook(ctx, sqlcgen.TouchBankAccountWebhookParams{TenantID: t.tenant, ID: accountID, At: pgtype.Timestamptz{Time: at, Valid: true}}))
}

func (BankRepo) SetSignatureOK(ctx context.Context, tx app.Tx, ok bool) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set signature state", sqlcgen.New(t).SetSepaySignatureOK(ctx, sqlcgen.SetSepaySignatureOKParams{TenantID: t.tenant, Ok: pgtype.Bool{Bool: ok, Valid: true}}))
}

// TenantByHook reads one tenant id in its own read-only transaction; the hook setting is transaction-local.
func (r *TenantResolver) TenantByHook(ctx context.Context, hookID string) (id string, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, unitOfWorkTimeout)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("begin hook lookup: %w", err)
	}
	defer func() {
		if rerr := rollback(ctx, tx); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT set_config('app.hook_id', $1, true)`, hookID); err != nil {
		return "", false, fmt.Errorf("set hook id: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT id FROM app.tenants WHERE hook_id = $1`, hookID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("select tenant by hook: %w", err)
	}
	return id, true, nil
}
