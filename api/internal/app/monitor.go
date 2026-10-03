package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	monitorPageSize = 50
	// maxMonitorRangeDays bounds one transactions or activity-log query.
	maxMonitorRangeDays = 366
	maxAuditDetails     = 20
)

var transactionKinds = map[string]bool{"ALL": true, "TRANSFER": true, "CASH": true, "NEEDS_ACTION": true}

// Monitor holds the owner's read side: alerts, transactions and the activity log (SG-902, SG-903).
type Monitor struct {
	uow   UnitOfWork
	repo  MonitorRepo
	clock Clock
	guard
}

func NewMonitor(uow UnitOfWork, repo MonitorRepo, levels BuildingLevels, clock Clock) *Monitor {
	return &Monitor{uow: uow, repo: repo, clock: clock, guard: guard{levels: levels}}
}

type AlertPage struct {
	Items      []AlertRow
	NextCursor string
}

type AlertsQuery struct {
	UnreadOnly   bool
	Kind, Cursor string
}

var alertKinds = map[string]bool{AlertAccountLocked: true, AlertCashOver: true, AlertCashShort: true, AlertDamageReported: true,
	AlertLeaveRequested: true, AlertPaymentMismatch: true, AlertOverpaid: true, AlertPaymentPartial: true, AlertPaymentUnpaid: true, AlertRefundPending: true, AlertSepayUpdated: true, AlertStayTimeEdited: true,
	AlertStocktakeDifference: true, AlertUnmatchedTransfer: true, AlertUnusedRoomReport: true}

// ListAlerts returns alerts newest first.
func (m *Monitor) ListAlerts(ctx context.Context, c Caller, q AlertsQuery) (AlertPage, error) {
	if err := m.checkRole("listAlerts", c); err != nil {
		return AlertPage{}, err
	}
	if q.Kind != "" && !alertKinds[q.Kind] {
		return AlertPage{}, stay.NewValidationError([]stay.FieldError{{Path: "kind", Code: stay.CodePattern}})
	}
	f := AlertFilter{UnreadOnly: q.UnreadOnly, Kind: q.Kind, Limit: monitorPageSize + 1}
	var err error
	if q.Cursor != "" {
		if f.CursorAt, f.CursorID, err = decodeCursor(q.Cursor); err != nil {
			return AlertPage{}, err
		}
	}
	var out AlertPage
	err = m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := m.repo.Alerts(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("alerts: %w", err)
		}
		out.Items = rows
		if len(rows) > monitorPageSize {
			last := rows[monitorPageSize-1]
			out.Items, out.NextCursor = rows[:monitorPageSize], encodeCursor(last.CreatedAt, last.ID)
		}
		return nil
	})
	return out, err
}

// MarkAlertRead marks an alert read; reading it again changes nothing.
func (m *Monitor) MarkAlertRead(ctx context.Context, c Caller, alertID string) error {
	if err := m.checkRole("markAlertRead", c); err != nil {
		return err
	}
	return m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		ok, err := m.repo.MarkAlertRead(ctx, tx, alertID, c.UserID, storedTime(m.clock.Now()))
		if err != nil {
			return fmt.Errorf("mark alert read: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	})
}

type TransactionPage struct {
	Items      []TransactionRow
	NextCursor string
}

type TransactionsQuery struct {
	From, To            *time.Time // calendar days; both empty means today
	Kind, Query, Cursor string
}

// ListTransactions lists money events in the chosen days, newest first.
func (m *Monitor) ListTransactions(ctx context.Context, c Caller, q TransactionsQuery) (TransactionPage, error) {
	if err := m.checkRole("listTransactions", c); err != nil {
		return TransactionPage{}, err
	}
	kind := q.Kind
	if kind == "" {
		kind = "ALL"
	}
	if !transactionKinds[kind] {
		return TransactionPage{}, stay.NewValidationError([]stay.FieldError{{Path: "filter", Code: stay.CodePattern}})
	}
	var out TransactionPage
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		from, to, err := m.window(ctx, tx, q.From, q.To)
		if err != nil {
			return err
		}
		f := TransactionFilter{From: from, To: to, Kind: kind, Query: strings.TrimSpace(q.Query), Limit: monitorPageSize + 1}
		if q.Cursor != "" {
			if f.CursorAt, f.CursorID, err = decodeCursor(q.Cursor); err != nil {
				return err
			}
		}
		rows, err := m.repo.Transactions(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("transactions: %w", err)
		}
		out.Items = rows
		if len(rows) > monitorPageSize {
			last := rows[monitorPageSize-1]
			out.Items, out.NextCursor = rows[:monitorPageSize], encodeCursor(last.At, last.ID)
		}
		return nil
	})
	return out, err
}

// window turns calendar days of the tenant zone into [start of first, start of day after last); no days means today.
func (m *Monitor) window(ctx context.Context, tx Tx, fromDay, toDay *time.Time) (time.Time, time.Time, error) {
	loc, err := loadZone(ctx, tx, m.repo)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	first := dayOf(m.clock.Now().In(loc), loc)
	last := first
	if fromDay != nil {
		first = dayOf(fromDay.In(time.UTC), loc)
		last = first
	}
	if toDay != nil {
		last = dayOf(toDay.In(time.UTC), loc)
		if fromDay == nil {
			first = last
		}
	}
	switch {
	case last.Before(first):
		return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMin}})
	case last.Sub(first) > maxMonitorRangeDays*24*time.Hour:
		return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMax}})
	}
	return first, last.AddDate(0, 0, 1), nil
}

// AuditEntryView is one activity-log line; Details are the scalar values the audit row recorded (ids, statuses, amounts).
type AuditEntryView struct {
	ID, ActorName, ActorRole, Category, Action string
	At                                         time.Time
	Details                                    map[string]string
}

type AuditPage struct {
	Items      []AuditEntryView
	NextCursor string
}

type AuditQuery struct {
	From, To                         time.Time
	ActorID, Category, Query, Cursor string
}

// ListAuditLogs reads the append-only activity log, newest first (owner only).
func (m *Monitor) ListAuditLogs(ctx context.Context, c Caller, q AuditQuery) (AuditPage, error) {
	if err := m.checkRole("listAuditLogs", c); err != nil {
		return AuditPage{}, err
	}
	var prefixes []string
	if q.Category != "" {
		var ok bool
		if prefixes, ok = auditPrefixesOf(q.Category); !ok {
			return AuditPage{}, stay.NewValidationError([]stay.FieldError{{Path: "category", Code: stay.CodePattern}})
		}
	}
	var out AuditPage
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		from, to, err := m.window(ctx, tx, &q.From, &q.To)
		if err != nil {
			return err
		}
		f := AuditFilter{From: from, To: to, ActorID: q.ActorID, Query: strings.TrimSpace(q.Query), Prefixes: prefixes, Limit: monitorPageSize + 1}
		if q.Cursor != "" {
			if f.CursorAt, f.CursorID, err = decodeCursor(q.Cursor); err != nil {
				return err
			}
		}
		rows, err := m.repo.AuditLogs(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("audit logs: %w", err)
		}
		next := ""
		if len(rows) > monitorPageSize {
			last := rows[monitorPageSize-1]
			rows, next = rows[:monitorPageSize], encodeCursor(last.At, last.ID)
		}
		out.NextCursor = next
		out.Items = make([]AuditEntryView, len(rows))
		for i, r := range rows {
			out.Items[i] = AuditEntryView{ID: r.ID, ActorName: r.ActorName, ActorRole: r.ActorRole, Category: auditCategoryOf(r.Action),
				Action: r.Action, At: r.At.UTC(), Details: AuditDetailsOf(r)}
		}
		return nil
	})
	return out, err
}

// AuditDetailsOf is what the activity log shows for a row: the scalar values of its "after" document, plus the room and
// the bill resolved from the entity. A detail the document already has is never replaced.
func AuditDetailsOf(r AuditRow) map[string]string {
	out := auditDetails(r.After)
	for k, v := range map[string]string{"room": r.Room, "bill": r.Bill} {
		if _, has := out[k]; !has && v != "" {
			out[k] = v
		}
	}
	return out
}

// auditDetails flattens the top-level scalar values of an audit row's "after" document into display strings.
func auditDetails(after []byte) map[string]string {
	out := map[string]string{}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(after))
	dec.UseNumber() // large amounts must not turn into 1e+06
	if len(after) == 0 || dec.Decode(&doc) != nil {
		return out
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(out) == maxAuditDetails {
			break
		}
		switch v := doc[k].(type) {
		case string:
			out[k] = v
		case json.Number:
			out[k] = v.String()
		case bool:
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// ListInvoices lists the invoices still to be paid (owner only). With an amount, those whose balance equals it come first,
// each group newest check-out first.
func (m *Monitor) ListInvoices(ctx context.Context, c Caller, amount *int64) ([]InvoiceCandidate, error) {
	if err := m.checkRole("listInvoices", c); err != nil {
		return nil, err
	}
	if amount != nil && *amount < 1 {
		return nil, stay.NewValidationError([]stay.FieldError{{Path: "amount", Code: stay.CodeMin}})
	}
	var out []InvoiceCandidate
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := m.repo.UnpaidInvoices(ctx, tx)
		if err != nil {
			return fmt.Errorf("unpaid invoices: %w", err)
		}
		out = rows
		if amount != nil {
			sort.SliceStable(out, func(i, j int) bool { return (out[i].Balance == *amount) && (out[j].Balance != *amount) })
		}
		return nil
	})
	return out, err
}
