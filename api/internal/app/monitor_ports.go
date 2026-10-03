package app

import (
	"context"
	"time"
)

// AlertRow is an alert as the owner reads it.
type AlertRow struct {
	ID, Kind, RoomCode, ShiftID, StayID, ActorName string
	Amount                                         *int64
	Details                                        map[string]string
	CreatedAt                                      time.Time
	ResolvedAt                                     *time.Time
	Resolution                                     string
}

type AlertFilter struct {
	UnreadOnly bool
	// UnresolvedOnly leaves out alerts that stopped needing the owner (the invoice was paid, the transfer linked).
	UnresolvedOnly bool
	Kind           string
	CursorAt       *time.Time
	CursorID       string
	Limit          int
}

// TransactionRow is one money event: a paid payment, a transfer with a wrong amount, or a bank event nobody matched.
type TransactionRow struct {
	ID, Method, RoomCode, BillCode, Reconciliation, TransferNote, PaymentEventID, ShiftID string
	Kind                                                                                  string // PAYMENT or CASH_REFUND (a negative amount)
	At                                                                                    time.Time
	ReceivedAt, SettledAt                                                                 *time.Time // when the bank money arrived, when it was settled or linked
	Amount                                                                                int64
}

// TransactionFilter: From inclusive, To exclusive; Kind is ALL, TRANSFER, CASH or NEEDS_ACTION.
type TransactionFilter struct {
	From, To time.Time
	Kind     string
	Query    string
	CursorAt *time.Time
	CursorID string
	Limit    int
}

type AuditRow struct {
	ID, ActorName, ActorRole, Action string
	At                               time.Time
	After                            []byte
	Room, Bill                       string // resolved at read time from the entity; empty when the entity has none
}

// AuditFilter: Prefixes are action prefixes of one category (empty means every category).
type AuditFilter struct {
	From, To time.Time
	ActorID  string
	Query    string
	Prefixes []string
	CursorAt *time.Time
	CursorID string
	Limit    int
}

// OpenTicket is a maintenance ticket that is not DONE.
type OpenTicket struct{ ID, RoomCode string }

// PendingLeave is a leave request waiting for the owner.
type PendingLeave struct{ ID, UserName string }

type LongToClean struct {
	RoomCode string
	Since    time.Time
}

// MonitorRepo filters by the tenant of the Tx.
type MonitorRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	Alerts(ctx context.Context, tx Tx, f AlertFilter) ([]AlertRow, error)
	// MarkAlertRead reports false when the alert does not exist.
	MarkAlertRead(ctx context.Context, tx Tx, id, userID string, at time.Time) (bool, error)
	Transactions(ctx context.Context, tx Tx, f TransactionFilter) ([]TransactionRow, error)
	AuditLogs(ctx context.Context, tx Tx, f AuditFilter) ([]AuditRow, error)
	LongToClean(ctx context.Context, tx Tx, before time.Time) ([]LongToClean, error)
	OpenTickets(ctx context.Context, tx Tx) ([]OpenTicket, error)
	// UnpaidInvoices lists open checked-out invoices with something to pay, newest check-out first.
	UnpaidInvoices(ctx context.Context, tx Tx) ([]InvoiceCandidate, error)
	PendingLeave(ctx context.Context, tx Tx) ([]PendingLeave, error)
}

// InvoiceCandidate is a checked-out invoice still to be paid. Paid is the deposit plus money the bank reported that did not
// settle the invoice; Balance is what is left.
type InvoiceCandidate struct {
	InvoiceID, BillCode, RoomCode, GuestName string
	CheckedOutAt                             time.Time
	Total, Paid, Balance                     int64
	RefundDue                                int64 // set for an invoice that only waits for the deposit refund
}
