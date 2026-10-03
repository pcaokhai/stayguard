package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// MonitorRepo implements app.MonitorRepo. The tenant always comes from the Tx.
type MonitorRepo struct{}

var _ app.MonitorRepo = MonitorRepo{}

func (MonitorRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func cursorArgs(at *time.Time, id string) (pgtype.Timestamptz, pgtype.Text) {
	if at == nil {
		return pgtype.Timestamptz{}, pgtype.Text{}
	}
	return ts(*at), pgtype.Text{String: id, Valid: true}
}

func (MonitorRepo) Alerts(ctx context.Context, tx app.Tx, f app.AlertFilter) ([]app.AlertRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListAlertsParams{TenantID: t.tenant, UnreadOnly: f.UnreadOnly, Kind: optText(f.Kind), RowLimit: int32(f.Limit), UnresolvedOnly: f.UnresolvedOnly} // #nosec G115 -- page size constant
	arg.CursorAt, arg.CursorID = cursorArgs(f.CursorAt, f.CursorID)
	rows, err := sqlcgen.New(t).ListAlerts(ctx, arg)
	if err != nil {
		return nil, wrap("list alerts", err)
	}
	out := make([]app.AlertRow, len(rows))
	for i, r := range rows {
		details := map[string]string{}
		if err := json.Unmarshal(r.Details, &details); err != nil {
			return nil, fmt.Errorf("decode details of alert %s: %w", r.ID, err)
		}
		out[i] = app.AlertRow{ID: r.ID, Kind: r.Kind, RoomCode: r.RoomCode, ShiftID: r.ShiftID, StayID: r.StayID,
			ActorName: r.ActorName, Amount: intPtr(r.Amount), Details: details, CreatedAt: r.CreatedAt.Time.UTC(), ResolvedAt: timePtr(r.ResolvedAt), Resolution: r.Resolution.String}
	}
	return out, nil
}

func (MonitorRepo) MarkAlertRead(ctx context.Context, tx app.Tx, id, userID string, at time.Time) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).MarkAlertRead(ctx, sqlcgen.MarkAlertReadParams{TenantID: t.tenant, AlertID: id, ReadAt: ts(at), ReadBy: optText(userID)})
	return n == 1, wrap("mark alert read", err)
}

func (MonitorRepo) Transactions(ctx context.Context, tx app.Tx, f app.TransactionFilter) ([]app.TransactionRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListTransactionsParams{TenantID: t.tenant, FromAt: ts(f.From), ToAt: ts(f.To), Kind: f.Kind,
		Pattern: optText(likePattern(f.Query)), RowLimit: int32(f.Limit)} // #nosec G115 -- page size constant
	arg.CursorAt, arg.CursorID = cursorArgs(f.CursorAt, f.CursorID)
	rows, err := sqlcgen.New(t).ListTransactions(ctx, arg)
	if err != nil {
		return nil, wrap("list transactions", err)
	}
	out := make([]app.TransactionRow, len(rows))
	for i, r := range rows {
		out[i] = app.TransactionRow{ID: r.ID, At: r.HappenedAt.Time.UTC(), Amount: r.Amount, Method: r.Method, RoomCode: r.RoomCode,
			BillCode: r.BillCode, Reconciliation: r.Reconciliation, TransferNote: r.TransferNote, PaymentEventID: r.PaymentEventID, ShiftID: r.ShiftID}
	}
	return out, nil
}

func (MonitorRepo) AuditLogs(ctx context.Context, tx app.Tx, f app.AuditFilter) ([]app.AuditRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListAuditLogsParams{TenantID: t.tenant, FromAt: ts(f.From), ToAt: ts(f.To), ActorID: optText(f.ActorID),
		Prefixes: f.Prefixes, Pattern: optText(likePattern(f.Query)), RowLimit: int32(f.Limit)} // #nosec G115 -- page size constant
	if arg.Prefixes == nil {
		arg.Prefixes = []string{}
	}
	arg.CursorAt, arg.CursorID = cursorArgs(f.CursorAt, f.CursorID)
	rows, err := sqlcgen.New(t).ListAuditLogs(ctx, arg)
	if err != nil {
		return nil, wrap("list audit logs", err)
	}
	out := make([]app.AuditRow, len(rows))
	for i, r := range rows {
		out[i] = app.AuditRow{ID: r.ID, At: r.CreatedAt.Time, ActorName: r.ActorName, ActorRole: r.ActorRole, Action: r.Action, After: r.After}
	}
	return out, nil
}

func (MonitorRepo) LongToClean(ctx context.Context, tx app.Tx, before time.Time) ([]app.LongToClean, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListLongToClean(ctx, sqlcgen.ListLongToCleanParams{TenantID: t.tenant, Before: ts(before)})
	if err != nil {
		return nil, wrap("list long to clean", err)
	}
	out := make([]app.LongToClean, len(rows))
	for i, r := range rows {
		out[i] = app.LongToClean{RoomCode: r.RoomCode, Since: r.Since.Time}
	}
	return out, nil
}

func (MonitorRepo) OpenTickets(ctx context.Context, tx app.Tx) ([]app.OpenTicket, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListOpenTickets(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list open tickets", err)
	}
	out := make([]app.OpenTicket, len(rows))
	for i, r := range rows {
		out[i] = app.OpenTicket{ID: r.ID, RoomCode: r.RoomCode}
	}
	return out, nil
}

func (MonitorRepo) PendingLeave(ctx context.Context, tx app.Tx) ([]app.PendingLeave, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListPendingLeave(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list pending leave", err)
	}
	out := make([]app.PendingLeave, len(rows))
	for i, r := range rows {
		out[i] = app.PendingLeave{ID: r.ID, UserName: r.UserName}
	}
	return out, nil
}

func (MonitorRepo) UnpaidInvoices(ctx context.Context, tx app.Tx) ([]app.InvoiceCandidate, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListUnpaidInvoices(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list unpaid invoices", err)
	}
	out := make([]app.InvoiceCandidate, len(rows))
	for i, r := range rows {
		paid := r.Deposit + r.Reported
		out[i] = app.InvoiceCandidate{InvoiceID: r.ID, BillCode: r.BillCode, RoomCode: r.RoomCode, GuestName: r.GuestName,
			CheckedOutAt: r.CheckOutAt.Time.UTC(), Total: r.Total, Paid: paid, Balance: max(r.Total-paid, 0)}
	}
	return out, nil
}
