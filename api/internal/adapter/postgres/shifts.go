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

// ShiftRepo implements app.ShiftRepo. The tenant always comes from the Tx.
type ShiftRepo struct{}

var (
	_ app.ShiftRepo  = ShiftRepo{}
	_ app.OpenShifts = ShiftRepo{}
)

func (ShiftRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (ShiftRepo) BuildingIDs(ctx context.Context, tx app.Tx) ([]string, error) {
	return StayHistoryRepo{}.BuildingIDs(ctx, tx)
}

func (ShiftRepo) LockOpen(ctx context.Context, tx app.Tx, userID string) (app.ShiftRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.ShiftRecord{}, false, err
	}
	r, err := sqlcgen.New(t).LockOpenShift(ctx, sqlcgen.LockOpenShiftParams{TenantID: t.tenant, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ShiftRecord{}, false, nil
	}
	if err != nil {
		return app.ShiftRecord{}, false, wrap("lock open shift", err)
	}
	return app.ShiftRecord{ID: r.ID, UserID: r.UserID, UserName: r.UserName, Status: r.Status, Code: r.ShiftCode,
		OpenedAt: r.OpenedAt.Time, ClosedAt: timePtr(r.ClosedAt), OpeningFloat: r.OpeningFloat}, true, nil
}

func (ShiftRepo) Open(ctx context.Context, tx app.Tx, s app.NewShift) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).InsertShift(ctx, sqlcgen.InsertShiftParams{ID: s.ID, TenantID: t.tenant, UserID: s.UserID,
		ShiftCode: s.Code, OpenedAt: ts(s.OpenedAt), OpeningFloat: s.OpeningFloat})
	return n == 1, writeFailure("insert shift", err)
}

func (ShiftRepo) LastFloatLeft(ctx context.Context, tx app.Tx) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	f, err := sqlcgen.New(t).LastFloatLeft(ctx, t.tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, wrap("select last float", err)
	}
	return f.Int64, nil
}

func (ShiftRepo) AddEntry(ctx context.Context, tx app.Tx, e app.CashEntry) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("insert cash entry", sqlcgen.New(t).InsertCashEntry(ctx, sqlcgen.InsertCashEntryParams{
		ID: e.ID, TenantID: t.tenant, ShiftID: e.ShiftID, Kind: e.Kind, Amount: e.Amount, StayID: optText(e.StayID),
		PaymentID: optText(e.PaymentID), Description: optText(e.Description), CreatedBy: optText(e.CreatedBy), CreatedAt: ts(e.CreatedAt)}))
}

func (ShiftRepo) Cash(ctx context.Context, tx app.Tx, shiftID string) (int64, int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, 0, err
	}
	r, err := sqlcgen.New(t).ShiftCash(ctx, sqlcgen.ShiftCashParams{TenantID: t.tenant, ShiftID: shiftID})
	return r.CashIn, r.CashOut, wrap("sum shift cash", err)
}

func (ShiftRepo) Transfers(ctx context.Context, tx app.Tx, from, to time.Time) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).TransfersBetween(ctx, sqlcgen.TransfersBetweenParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	return n, wrap("sum transfers", err)
}

func (ShiftRepo) Close(ctx context.Context, tx app.Tx, c app.ShiftClose) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).CloseShift(ctx, sqlcgen.CloseShiftParams{TenantID: t.tenant, ShiftID: c.ShiftID, ClosedAt: ts(c.At),
		ExpectedCash: int8v(c.Expected), CountedCash: int8v(c.Counted), Difference: int8v(c.Diff), FloatLeft: int8v(c.FloatLeft),
		Reason: optText(c.Reason), ReasonRecordedAt: reasonAt(c), HandoverToUserID: optText(c.HandoverTo), Counts: c.CountsJSON})
	if err != nil {
		return wrap("close shift", err)
	}
	if n != 1 {
		return app.ErrShiftNotOpen
	}
	return nil
}

func reasonAt(c app.ShiftClose) pgtype.Timestamptz {
	if c.Reason == "" {
		return pgtype.Timestamptz{}
	}
	return ts(c.At)
}

func int8v(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: true} }

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func (ShiftRepo) ByID(ctx context.Context, tx app.Tx, shiftID string) (app.ShiftRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.ShiftRecord{}, false, err
	}
	r, err := sqlcgen.New(t).GetShiftByID(ctx, sqlcgen.GetShiftByIDParams{TenantID: t.tenant, ShiftID: shiftID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ShiftRecord{}, false, nil
	}
	if err != nil {
		return app.ShiftRecord{}, false, wrap("select shift", err)
	}
	rec := app.ShiftRecord{ID: r.ID, UserID: r.UserID, UserName: r.UserName, Status: r.Status, Code: r.ShiftCode,
		OpenedAt: r.OpenedAt.Time, ClosedAt: timePtr(r.ClosedAt), OpeningFloat: r.OpeningFloat, ReasonAt: timePtr(r.ReasonRecordedAt)}
	rec.Expected, rec.Counted, rec.Difference = intPtr(r.ExpectedCash), intPtr(r.CountedCash), intPtr(r.Difference)
	if r.Reason.Valid {
		rec.Reason = &r.Reason.String
	}
	return rec, true, nil
}

func intPtr(n pgtype.Int8) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func (ShiftRepo) CashIn(ctx context.Context, tx app.Tx, shiftID string) ([]app.CashInRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListShiftCashIn(ctx, sqlcgen.ListShiftCashInParams{TenantID: t.tenant, ShiftID: shiftID})
	if err != nil {
		return nil, wrap("list shift cash", err)
	}
	out := make([]app.CashInRow, len(rows))
	for i, r := range rows {
		out[i] = app.CashInRow{RoomCode: r.RoomCode, RentalType: r.RentalType, At: r.CreatedAt.Time, Amount: r.Amount}
	}
	return out, nil
}

func (ShiftRepo) MonthStats(ctx context.Context, tx app.Tx, userID string, from, to time.Time) (int64, int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, 0, err
	}
	r, err := sqlcgen.New(t).StaffMonthStats(ctx, sqlcgen.StaffMonthStatsParams{TenantID: t.tenant, UserID: userID, FromAt: ts(from), ToAt: ts(to)})
	return r.ShiftsWithDifference, r.TotalShort, wrap("staff month stats", err)
}

func (ShiftRepo) ListClosed(ctx context.Context, tx app.Tx, f app.ClosedFilter) ([]app.ClosedShiftRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListClosedShiftsParams{TenantID: t.tenant, UserID: optText(f.UserID), OnlyDifferences: f.OnlyDifferences,
		RowLimit: int32(f.Limit)} // #nosec G115 -- page size constant
	if !f.From.IsZero() {
		arg.FromAt, arg.ToAt = ts(f.From), ts(f.To)
	}
	if f.CursorAt != nil {
		arg.CursorAt, arg.CursorID = ts(*f.CursorAt), pgtype.Text{String: f.CursorID, Valid: true}
	}
	rows, err := sqlcgen.New(t).ListClosedShifts(ctx, arg)
	if err != nil {
		return nil, wrap("list closed shifts", err)
	}
	out := make([]app.ClosedShiftRow, len(rows))
	for i, r := range rows {
		out[i] = app.ClosedShiftRow{ID: r.ID, UserName: r.UserName, Code: r.ShiftCode, OpenedAt: r.OpenedAt.Time.UTC(),
			ClosedAt: r.ClosedAt.Time.UTC(), Difference: r.Difference.Int64}
	}
	return out, nil
}

// HasOpenShift lets staff removal refuse while a person still holds an open shift (app.OpenShifts).
func (r ShiftRepo) HasOpenShift(ctx context.Context, tx app.Tx, userID string) (bool, error) {
	_, ok, err := r.LockOpen(ctx, tx, userID)
	return ok, err
}

func (ShiftRepo) Scheduled(ctx context.Context, tx app.Tx, userID string, day time.Time) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	codes, err := sqlcgen.New(t).ListScheduledShifts(ctx, sqlcgen.ListScheduledShiftsParams{TenantID: t.tenant, UserID: userID,
		Day: pgtype.Date{Time: day, Valid: true}})
	return codes, wrap("list scheduled shifts", err)
}

func (ShiftRepo) UnpaidInvoices(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.InvoiceCandidate, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListShiftUnpaidInvoices(ctx, sqlcgen.ListShiftUnpaidInvoicesParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return nil, wrap("list shift unpaid invoices", err)
	}
	out := make([]app.InvoiceCandidate, len(rows))
	for i, r := range rows {
		paid := r.Deposit + r.Reported
		out[i] = app.InvoiceCandidate{InvoiceID: r.ID, BillCode: r.BillCode, RoomCode: r.RoomCode, GuestName: r.GuestName,
			CheckedOutAt: r.CheckOutAt.Time.UTC(), Total: r.Total, Paid: paid, Balance: max(r.Total-paid, 0), RefundDue: r.RefundDue}
	}
	return out, nil
}
