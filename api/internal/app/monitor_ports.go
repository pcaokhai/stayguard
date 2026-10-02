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
}

type AlertFilter struct {
	UnreadOnly bool
	Kind       string
	CursorAt   *time.Time
	CursorID   string
	Limit      int
}

// TransactionRow is one money event: a paid payment, a transfer with a wrong amount, or a bank event nobody matched.
type TransactionRow struct {
	ID, Method, RoomCode, BillCode, Reconciliation, TransferNote, PaymentEventID, ShiftID string
	At                                                                                    time.Time
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
}
