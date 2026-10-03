package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// invoiceBillCodeKey is the unique constraint on (tenant_id, bill_code).
const invoiceBillCodeKey = "invoices_tenant_id_bill_code_key"

// BillingRepo implements app.BillingStayRepo. The tenant always comes from the Tx, never from callers.
type BillingRepo struct{}

var _ app.BillingStayRepo = BillingRepo{}

func (BillingRepo) StayByID(ctx context.Context, tx app.Tx, stayID string) (app.StayRecord, bool, error) {
	return StayRepo{}.StayByID(ctx, tx, stayID)
}

func (BillingRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

// LockStay returns the stay (with its extras) under a row lock that lasts until the transaction ends.
func (BillingRepo) LockStay(ctx context.Context, tx app.Tx, stayID string) (app.StayRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.StayRecord{}, false, err
	}
	q := sqlcgen.New(t)
	r, err := q.LockStayByID(ctx, sqlcgen.LockStayByIDParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.StayRecord{}, false, nil
	}
	if err != nil {
		return app.StayRecord{}, false, wrap("lock stay", err)
	}
	extras, err := q.ListStayExtras(ctx, sqlcgen.ListStayExtrasParams{TenantID: t.tenant, StayID: stayID})
	if err != nil {
		return app.StayRecord{}, false, wrap("select stay extras", err)
	}
	rec, err := toStayRecord(sqlcgen.GetStayByIDRow(r), extras)
	return rec, err == nil, err
}

func (BillingRepo) InsertExtra(ctx context.Context, tx app.Tx, e app.NewExtra) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	if e.Quantity < 1 || e.Quantity > math.MaxInt32 { // the column is an integer
		return fmt.Errorf("insert extra: quantity %d out of range", e.Quantity)
	}
	err = sqlcgen.New(t).InsertStayExtra(ctx, sqlcgen.InsertStayExtraParams{
		ID: e.ID, TenantID: t.tenant, StayID: e.StayID, ServiceID: e.ServiceID, Quantity: int32(e.Quantity),
		UnitAmount: e.UnitAmount, Amount: e.Amount, CreatedAt: pgtype.Timestamptz{Time: e.CreatedAt, Valid: true}, CreatedBy: optText(e.CreatedBy),
	})
	return writeFailure("insert extra", err)
}

func (BillingRepo) MarkCheckedOut(ctx context.Context, tx app.Tx, stayID string, at time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).MarkStayCheckedOut(ctx, sqlcgen.MarkStayCheckedOutParams{
		TenantID: t.tenant, StayID: stayID, At: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return wrap("check out stay", err)
	}
	if n != 1 {
		return stay.ErrNotActive
	}
	return nil
}

func (BillingRepo) InvoiceByStay(ctx context.Context, tx app.Tx, stayID string) (app.InvoiceRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.InvoiceRecord{}, false, err
	}
	r, err := sqlcgen.New(t).GetInvoiceByStay(ctx, sqlcgen.GetInvoiceByStayParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.InvoiceRecord{}, false, nil
	}
	if err != nil {
		return app.InvoiceRecord{}, false, wrap("select invoice", err)
	}
	return app.InvoiceRecord{ID: r.ID, StayID: r.StayID, BillCode: r.BillCode, Status: r.Status, Quote: r.Quote,
		Total: r.Total, CreatedAt: r.CreatedAt.Time, PaidAt: timePtr(r.PaidAt)}, true, nil
}

// InsertInvoice maps a unique violation of the bill code to app.ErrBillCodeConflict (the use case retries);
// any other failure, including a second invoice for one stay, is a plain error without row values.
func (BillingRepo) InsertInvoice(ctx context.Context, tx app.Tx, n app.NewInvoice) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	err = sqlcgen.New(t).InsertInvoice(ctx, sqlcgen.InsertInvoiceParams{
		ID: n.ID, TenantID: t.tenant, StayID: n.StayID, BillCode: n.BillCode, Quote: n.Quote, Total: n.Total,
		CreatedAt: pgtype.Timestamptz{Time: n.CreatedAt, Valid: true}})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == pgUniqueViolation && pe.ConstraintName == invoiceBillCodeKey {
		return fmt.Errorf("insert invoice failed: sqlstate %s constraint %q: %w", pe.Code, pe.ConstraintName, app.ErrBillCodeConflict)
	}
	return writeFailure("insert invoice", err)
}

func (BillingRepo) BillCodeTaken(ctx context.Context, tx app.Tx, code string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	taken, err := sqlcgen.New(t).BillCodeTaken(ctx, sqlcgen.BillCodeTakenParams{TenantID: t.tenant, BillCode: code})
	return taken, wrap("select bill code", err)
}

// writeFailure keeps what an operator needs (SQLSTATE and constraint name) and drops the rest: the pg
// error text and detail quote the row's values.
func writeFailure(what string, err error) error {
	if err == nil {
		return nil
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s failed: sqlstate %s constraint %q", what, pe.Code, pe.ConstraintName)
	}
	return fmt.Errorf("%s failed", what)
}
