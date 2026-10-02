package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

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
