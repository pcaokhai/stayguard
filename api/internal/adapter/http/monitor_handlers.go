package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// MonitorService is what the owner monitoring handlers need from the application layer (SG-902, SG-903).
type MonitorService interface {
	ListAlerts(ctx context.Context, c app.Caller, q app.AlertsQuery) (app.AlertPage, error)
	MarkAlertRead(ctx context.Context, c app.Caller, alertID string) error
	ListTransactions(ctx context.Context, c app.Caller, q app.TransactionsQuery) (app.TransactionPage, error)
	LinkTransfer(ctx context.Context, c app.Caller, eventID, retryID, invoiceID string) (app.TransactionRow, error)
	ListAuditLogs(ctx context.Context, c app.Caller, q app.AuditQuery) (app.AuditPage, error)
	ListInvoices(ctx context.Context, c app.Caller, amount *int64) ([]app.InvoiceCandidate, error)
}

// WithMonitor adds the alert, transaction and activity-log use cases.
func (s Server) WithMonitor(m MonitorService) Server { s.monitor = m; return s }

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toAlert(a app.AlertRow) gen.Alert {
	d := a.Details
	return gen.Alert{Id: a.ID, Kind: gen.AlertKind(a.Kind), CreatedAt: a.CreatedAt, RoomCode: nilIfEmpty(a.RoomCode),
		ShiftId: nilIfEmpty(a.ShiftID), StayId: nilIfEmpty(a.StayID), ActorName: nilIfEmpty(a.ActorName), Amount: a.Amount, Details: &d}
}

func (s Server) ListAlerts(ctx context.Context, req gen.ListAlertsRequestObject) (gen.ListAlertsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	p := req.Params
	q := app.AlertsQuery{Cursor: deref(p.Cursor)}
	if p.Unread != nil {
		q.UnreadOnly = *p.Unread
	}
	if p.Kind != nil {
		q.Kind = string(*p.Kind)
	}
	page, err := s.monitor.ListAlerts(ctx, c, q)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Alert, len(page.Items))
	for i, a := range page.Items {
		items[i] = toAlert(a)
	}
	return gen.ListAlerts200JSONResponse{Items: items, NextCursor: nilIfEmpty(page.NextCursor)}, nil
}

func (s Server) MarkAlertRead(ctx context.Context, req gen.MarkAlertReadRequestObject) (gen.MarkAlertReadResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := s.monitor.MarkAlertRead(ctx, c, req.AlertId); err != nil {
		return nil, err
	}
	return gen.MarkAlertRead204Response{}, nil
}

func (s Server) ListTransactions(ctx context.Context, req gen.ListTransactionsRequestObject) (gen.ListTransactionsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	p := req.Params
	q := app.TransactionsQuery{Query: deref(p.Q), Cursor: deref(p.Cursor)}
	if p.From != nil {
		q.From = &p.From.Time
	}
	if p.To != nil {
		q.To = &p.To.Time
	}
	if p.Filter != nil {
		q.Kind = string(*p.Filter)
	}
	page, err := s.monitor.ListTransactions(ctx, c, q)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Transaction, len(page.Items))
	for i, r := range page.Items {
		items[i] = toTransaction(r)
	}
	return gen.ListTransactions200JSONResponse{Items: items, NextCursor: nilIfEmpty(page.NextCursor)}, nil
}

func toTransaction(r app.TransactionRow) gen.Transaction {
	return gen.Transaction{Id: r.ID, At: r.At, Amount: r.Amount, Method: gen.PaymentMethod(r.Method), RoomCode: nilIfEmpty(r.RoomCode),
		BillCode: nilIfEmpty(r.BillCode), Reconciliation: gen.TransactionReconciliation(r.Reconciliation),
		TransferNote: nilIfEmpty(r.TransferNote), PaymentEventId: nilIfEmpty(r.PaymentEventID), ShiftId: nilIfEmpty(r.ShiftID)}
}

func (s Server) LinkTransferToInvoice(ctx context.Context, req gen.LinkTransferToInvoiceRequestObject) (gen.LinkTransferToInvoiceResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	row, err := s.monitor.LinkTransfer(ctx, c, req.EventId, req.Params.IdempotencyKey.String(), req.Body.InvoiceId)
	if err != nil {
		return nil, err
	}
	return gen.LinkTransferToInvoice200JSONResponse(toTransaction(row)), nil
}

func (s Server) ListAuditLogs(ctx context.Context, req gen.ListAuditLogsRequestObject) (gen.ListAuditLogsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	p := req.Params
	page, err := s.monitor.ListAuditLogs(ctx, c, app.AuditQuery{From: p.From.Time, To: p.To.Time, ActorID: deref(p.ActorId),
		Category: deref(p.Category), Query: deref(p.Q), Cursor: deref(p.Cursor)})
	if err != nil {
		return nil, err
	}
	items := make([]gen.AuditEntry, len(page.Items))
	for i, e := range page.Items {
		d := e.Details
		items[i] = gen.AuditEntry{Id: e.ID, At: e.At, ActorName: e.ActorName, Category: gen.AuditEntryCategory(e.Category), Action: e.Action, Details: &d}
		if e.ActorRole != "" {
			role := gen.Role(e.ActorRole)
			items[i].ActorRole = &role
		}
	}
	return gen.ListAuditLogs200JSONResponse{Items: items, NextCursor: nilIfEmpty(page.NextCursor)}, nil
}

func (s Server) ListInvoices(ctx context.Context, req gen.ListInvoicesRequestObject) (gen.ListInvoicesResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.monitor.ListInvoices(ctx, c, req.Params.Amount)
	if err != nil {
		return nil, err
	}
	items := make([]gen.InvoiceCandidate, len(rows))
	for i, r := range rows {
		items[i] = gen.InvoiceCandidate{InvoiceId: r.InvoiceID, BillCode: r.BillCode, RoomCode: r.RoomCode, GuestName: r.GuestName,
			CheckedOutAt: r.CheckedOutAt, Total: r.Total, Paid: r.Paid, Balance: r.Balance}
	}
	return gen.ListInvoices200JSONResponse{Items: items}, nil
}
