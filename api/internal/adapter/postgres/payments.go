package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// PaymentRepo implements app.PaymentRepo. The tenant always comes from the Tx, never from callers.
type PaymentRepo struct{}

var _ app.PaymentRepo = PaymentRepo{}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (PaymentRepo) LockInvoice(ctx context.Context, tx app.Tx, invoiceID string) (app.PayInvoice, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.PayInvoice{}, false, err
	}
	r, err := sqlcgen.New(t).LockInvoiceForPayment(ctx, sqlcgen.LockInvoiceForPaymentParams{TenantID: t.tenant, InvoiceID: invoiceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PayInvoice{}, false, nil
	}
	if err != nil {
		return app.PayInvoice{}, false, wrap("lock invoice", err)
	}
	return app.PayInvoice{ID: r.ID, Status: r.Status, BillCode: r.BillCode, StayID: r.StayID, BuildingID: r.BuildingID, Quote: r.Quote}, true, nil
}

func toPaymentRecord(r sqlcgen.GetPaymentByIDRow) app.PaymentRecord {
	rec := app.PaymentRecord{ID: r.ID, InvoiceID: r.InvoiceID, Method: r.Method, Status: r.Status, Amount: r.Amount,
		BillCode: r.BillCode, BuildingID: r.BuildingID}
	if r.ReceivedAmount.Valid {
		rec.ReceivedAmount = &r.ReceivedAmount.Int64
	}
	if r.PaidAt.Valid {
		at := r.PaidAt.Time.UTC()
		rec.PaidAt = &at
	}
	if r.TransactionID.Valid {
		rec.TransactionID = &r.TransactionID.String
	}
	return rec
}

func (PaymentRepo) PaymentByID(ctx context.Context, tx app.Tx, id string) (app.PaymentRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.PaymentRecord{}, false, err
	}
	r, err := sqlcgen.New(t).GetPaymentByID(ctx, sqlcgen.GetPaymentByIDParams{TenantID: t.tenant, PaymentID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PaymentRecord{}, false, nil
	}
	if err != nil {
		return app.PaymentRecord{}, false, wrap("select payment", err)
	}
	return toPaymentRecord(r), true, nil
}

func (PaymentRepo) PendingForInvoice(ctx context.Context, tx app.Tx, invoiceID string) (app.PaymentRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.PaymentRecord{}, false, err
	}
	r, err := sqlcgen.New(t).GetPendingPaymentForInvoice(ctx, sqlcgen.GetPendingPaymentForInvoiceParams{TenantID: t.tenant, InvoiceID: invoiceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PaymentRecord{}, false, nil
	}
	if err != nil {
		return app.PaymentRecord{}, false, wrap("select pending payment", err)
	}
	return toPaymentRecord(sqlcgen.GetPaymentByIDRow(r)), true, nil
}

func (PaymentRepo) PendingTransfers(ctx context.Context, tx app.Tx) ([]app.PendingTransfer, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListPendingTransfers(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list pending transfers", err)
	}
	out := make([]app.PendingTransfer, len(rows))
	for i, r := range rows {
		out[i] = app.PendingTransfer{PaymentID: r.ID, InvoiceID: r.InvoiceID, StayID: r.StayID, BillCode: r.BillCode, Amount: r.Amount}
	}
	return out, nil
}

func (PaymentRepo) InsertPendingTransfer(ctx context.Context, tx app.Tx, p app.NewPayment) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert transfer", sqlcgen.New(t).InsertPendingTransfer(ctx, sqlcgen.InsertPendingTransferParams{
		ID: p.ID, TenantID: t.tenant, InvoiceID: p.InvoiceID, Amount: p.Amount,
		ReferenceCode: pgtype.Text{String: p.BillCode, Valid: true}, CreatedAt: ts(p.At)}))
}

func (PaymentRepo) InsertCashPayment(ctx context.Context, tx app.Tx, p app.NewPayment) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert cash payment", sqlcgen.New(t).InsertCashPayment(ctx, sqlcgen.InsertCashPaymentParams{
		ID: p.ID, TenantID: t.tenant, InvoiceID: p.InvoiceID, Amount: p.Amount, At: ts(p.At)}))
}

func (PaymentRepo) ExpirePending(ctx context.Context, tx app.Tx, invoiceID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("expire pending", sqlcgen.New(t).ExpirePendingForInvoice(ctx, sqlcgen.ExpirePendingForInvoiceParams{TenantID: t.tenant, InvoiceID: invoiceID}))
}

func (PaymentRepo) InsertEvent(ctx context.Context, tx app.Tx, id string, ev app.PaymentEvent) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).InsertPaymentEvent(ctx, sqlcgen.InsertPaymentEventParams{
		ID: id, TenantID: pgtype.Text{String: t.tenant, Valid: true}, Provider: ev.Provider, ExternalID: ev.ExternalID,
		Amount: ev.Amount, ReferenceCode: pgtype.Text{String: ev.Content, Valid: true}, ReceivedAt: ts(ev.ReceivedAt)})
	return n == 1, wrap("insert payment event", err)
}

func (PaymentRepo) SetEventResult(ctx context.Context, tx app.Tx, ev app.PaymentEvent, result string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set event result", sqlcgen.New(t).SetPaymentEventResult(ctx, sqlcgen.SetPaymentEventResultParams{
		Result: result, TenantID: pgtype.Text{String: t.tenant, Valid: true}, Provider: ev.Provider, ExternalID: ev.ExternalID}))
}

func (PaymentRepo) SettleTransfer(ctx context.Context, tx app.Tx, paymentID string, paidAt time.Time, received int64, transactionID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).SettleTransfer(ctx, sqlcgen.SettleTransferParams{PaidAt: ts(paidAt),
		ReceivedAmount: pgtype.Int8{Int64: received, Valid: true}, TransactionID: pgtype.Text{String: transactionID, Valid: true},
		TenantID: t.tenant, PaymentID: paymentID})
	if err != nil {
		return wrap("settle transfer", err)
	}
	if n != 1 {
		return app.ErrPaymentNotPending
	}
	return nil
}

func (PaymentRepo) MarkMismatch(ctx context.Context, tx app.Tx, paymentID string, received int64, transactionID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).MarkTransferMismatch(ctx, sqlcgen.MarkTransferMismatchParams{
		ReceivedAmount: pgtype.Int8{Int64: received, Valid: true}, TransactionID: pgtype.Text{String: transactionID, Valid: true},
		TenantID: t.tenant, PaymentID: paymentID})
	if err != nil {
		return wrap("mark mismatch", err)
	}
	if n != 1 {
		return app.ErrPaymentNotPending
	}
	return nil
}

func (PaymentRepo) CloseInvoice(ctx context.Context, tx app.Tx, invoiceID, stayID string, at time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	n, err := q.MarkInvoicePaid(ctx, sqlcgen.MarkInvoicePaidParams{PaidAt: ts(at), TenantID: t.tenant, InvoiceID: invoiceID})
	if err != nil {
		return wrap("mark invoice paid", err)
	}
	if n != 1 {
		return app.ErrInvoiceNotOpen
	}
	return wrap("release room", q.ReleaseRoomToClean(ctx, sqlcgen.ReleaseRoomToCleanParams{TenantID: t.tenant, StayID: stayID}))
}

func (PaymentRepo) BankAccount(ctx context.Context, tx app.Tx) ([]byte, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	b, err := sqlcgen.New(t).GetTenantBankAccount(ctx, t.tenant)
	return b, wrap("select bank account", err)
}
