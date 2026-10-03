package app

import (
	"context"
	"errors"
	"time"
)

// ErrShiftNotOpen is the backstop of closing: the shift was already closed (HTTP 409 SHIFT_NOT_OPEN).
var ErrShiftNotOpen = errors.New("shift is not open")

// ShiftRecord is a stored shift. Expected, Counted and Difference are set once it is closed.
type ShiftRecord struct {
	ID, UserID, UserName, Status, Code string
	OpenedAt                           time.Time
	ClosedAt                           *time.Time
	OpeningFloat                       int64
	Expected, Counted, Difference      *int64
	Reason                             *string
	ReasonAt                           *time.Time
}

type NewShift struct {
	ID, UserID, Code string
	OpenedAt         time.Time
	OpeningFloat     int64
}

// CashEntry is one line of the drawer ledger; kinds are the domain/shift constants.
type CashEntry struct {
	ID, ShiftID, Kind, StayID, PaymentID, Description, CreatedBy string // ShiftID is empty for owner cash with no shift
	Amount                                                       int64
	CreatedAt                                                    time.Time
	ByOwner                                                      bool
}

// ShiftClose is what closing writes.
type ShiftClose struct {
	ShiftID, Reason, HandoverTo string
	At                          time.Time
	Expected, Counted, Diff     int64
	FloatLeft                   int64
	CountsJSON                  []byte
}

type CashInRow struct {
	RoomCode, RentalType string
	At                   time.Time
	Amount               int64
	ByOwner              bool // recorded by the owner or a manager on this shift
}

type ClosedShiftRow struct {
	ID, UserName, Code string
	OpenedAt, ClosedAt time.Time
	Difference         int64
}

// ClosedFilter: From and To (when From is set) bound closed_at; UserID is optional.
type ClosedFilter struct {
	From, To        time.Time
	UserID          string
	OnlyDifferences bool
	CursorAt        *time.Time
	CursorID        string
	Limit           int
}

// ShiftRepo filters by the tenant of the Tx.
type ShiftRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	BuildingIDs(ctx context.Context, tx Tx) ([]string, error)
	// Scheduled lists the roster shifts of a person on a day (tenant-local calendar day at midnight UTC), in day order.
	Scheduled(ctx context.Context, tx Tx, userID string, day time.Time) ([]string, error)
	// LockOpen takes the row lock of the user's open shift; false when there is none.
	LockOpen(ctx context.Context, tx Tx, userID string) (ShiftRecord, bool, error)
	// LockOpenForStay is the open shift of a person who can edit the stay's building (the latest opened), locked; false when none.
	LockOpenForStay(ctx context.Context, tx Tx, stayID string) (ShiftRecord, bool, error)
	// Open inserts an open shift; it reports false when the user already has one (a race that was lost).
	Open(ctx context.Context, tx Tx, s NewShift) (bool, error)
	// LastFloatLeft is the float of the shift closed last; zero when none was closed yet.
	LastFloatLeft(ctx context.Context, tx Tx) (int64, error)
	AddEntry(ctx context.Context, tx Tx, e CashEntry) error
	Cash(ctx context.Context, tx Tx, shiftID string) (in, out int64, err error)
	Transfers(ctx context.Context, tx Tx, from, to time.Time) (int64, error)
	// UnpaidInvoices lists the invoices not fully paid that belong to the window: checked out in it, or with bank money that arrived in it.
	UnpaidInvoices(ctx context.Context, tx Tx, from, to time.Time) ([]InvoiceCandidate, error)
	// Close returns ErrShiftNotOpen when the shift was not open: the backstop.
	Close(ctx context.Context, tx Tx, c ShiftClose) error
	ByID(ctx context.Context, tx Tx, shiftID string) (ShiftRecord, bool, error)
	CashIn(ctx context.Context, tx Tx, shiftID string) ([]CashInRow, error)
	// Movements are the shift's ledger lines in time order, amounts unsigned; the use case signs them.
	Movements(ctx context.Context, tx Tx, shiftID string) ([]CashMovement, error)
	MonthStats(ctx context.Context, tx Tx, userID string, from, to time.Time) (withDifference, totalShort int64, err error)
	ListClosed(ctx context.Context, tx Tx, f ClosedFilter) ([]ClosedShiftRow, error)
}

// CashRecord is a cash movement of a command (a deposit, a cash payment, a refund) to put on the drawer ledger.
type CashRecord struct {
	Kind, StayID, PaymentID string
	Amount                  int64
}

// CashLedger is what the check-in and payment use cases call in their own transaction. Only a receptionist has a drawer:
// for any other role it does nothing. Amounts of zero are skipped.
type CashLedger interface {
	Record(ctx context.Context, tx Tx, c Caller, e CashRecord) error
}

// CashMovement is one ledger line of a shift as the screens list it. Kind is OPENING_FLOAT, DEPOSIT, PAYMENT, REFUND or PAYOUT; Amount
// is signed (REFUND and PAYOUT are negative), so the movements of a shift add up to its expected cash.
type CashMovement struct {
	At                       time.Time
	RoomCode, BillCode, Kind string
	Amount                   int64
	ByOwner                  bool
}
