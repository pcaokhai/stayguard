package app

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrExpenseAutomatic: payroll, maintenance and stock lines are written by the system (HTTP 409 EXPENSE_AUTOMATIC).
	ErrExpenseAutomatic = errors.New("expense line is automatic")
	// ErrPayrollPaid: a paid payroll line no longer changes (HTTP 409 PAYROLL_PAID).
	ErrPayrollPaid = errors.New("payroll line is paid")
	// ErrPayrollNothing: no unpaid line to mark paid (HTTP 409 PAYROLL_NOTHING_TO_PAY).
	ErrPayrollNothing = errors.New("no unpaid payroll line")
	// ErrPayrollFuture: a month that has not started cannot be paid (HTTP 409 PAYROLL_FUTURE).
	ErrPayrollFuture = errors.New("month has not started")
)

// ExpenseRow is a stored expense line; Month is YYYY-MM.
type ExpenseRow struct {
	ID, Month, Category, Source, Note, AttachmentAssetID string
	Amount                                               int64
	PaidOn                                               *time.Time
	Recurring                                            bool
}

// NewExpense is a line a person writes (MANUAL, or RECURRING when it repeats).
type NewExpense struct {
	ExpenseRow
	CreatedBy string
}

// AutoExpense is a line the system posts. Source and RefID make it unique: posting it again changes nothing.
type AutoExpense struct {
	ID, Source, RefID, Category, Month, Note, CreatedBy string
	Amount                                              int64
	PaidOn                                              *time.Time
}

// ExpenseLedger is how other use cases post automatic expense lines in their own transaction: payroll when a line is
// paid, a maintenance ticket when it is DONE, stock when it is bought in. Posting twice is harmless.
type ExpenseLedger interface {
	PostAuto(ctx context.Context, tx Tx, e AutoExpense) error
}

// Share is an amount under a key (a category, a rental type, a building, a payment method or a month).
type Share struct {
	Key, Source string
	Amount      int64
}

// ExpenseRepo filters by the tenant of the Tx.
type ExpenseRepo interface {
	ExpenseLedger
	Timezone(ctx context.Context, tx Tx) (string, error)
	Insert(ctx context.Context, tx Tx, e NewExpense) error
	ByID(ctx context.Context, tx Tx, id string) (ExpenseRow, bool, error)
	OfMonth(ctx context.Context, tx Tx, month string) ([]ExpenseRow, error)
	// Update and Delete report false when no editable line (MANUAL or RECURRING) matched.
	Update(ctx context.Context, tx Tx, e ExpenseRow) (bool, error)
	Delete(ctx context.Context, tx Tx, id string) (bool, error)
	// Summary groups a month by category and source.
	Summary(ctx context.Context, tx Tx, month string) ([]Share, error)
	EarliestRecurring(ctx context.Context, tx Tx) (string, bool, error)
	RecurringRan(ctx context.Context, tx Tx, month string) (bool, error)
	CopyRecurring(ctx context.Context, tx Tx, fromMonth, toMonth string) error
	MarkRecurringRan(ctx context.Context, tx Tx, month string) error
	Revenue(ctx context.Context, tx Tx, from, to time.Time) (int64, error)
}

// ReportRepo adds what the income and cost report reads. Ranges of instants are [from, to).
type ReportRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	Revenue(ctx context.Context, tx Tx, from, to time.Time) (int64, error)
	RevenueByMonth(ctx context.Context, tx Tx, from, to time.Time, zone string) (map[string]int64, error)
	RevenueByRentalType(ctx context.Context, tx Tx, from, to time.Time) ([]Share, error)
	RevenueByBuilding(ctx context.Context, tx Tx, from, to time.Time) ([]Share, error)
	// RevenueDeposit is the part of paid invoices the deposit covered; RevenueBalanceByMethod the rest, by payment method.
	RevenueDeposit(ctx context.Context, tx Tx, from, to time.Time) (int64, error)
	RevenueBalanceByMethod(ctx context.Context, tx Tx, from, to time.Time) ([]Share, error)
	ExpenseByMonth(ctx context.Context, tx Tx, fromMonth, toMonth string) (map[string]int64, error)
	ExpenseByCategory(ctx context.Context, tx Tx, fromMonth, toMonth string) ([]Share, error)
	OccupiedRoomDays(ctx context.Context, tx Tx, zone string, fromDay, toDay time.Time) (int64, error)
	RoomCount(ctx context.Context, tx Tx) (int64, error)
}

// PayrollStaff is a person on the payroll with their contract.
type PayrollStaff struct {
	UserID, Name, Position string
	Removed                bool
	PayType                string
	Rate, Allowance        int64
	StandardShifts         int
}

// PayrollStored is what the owner entered for a person and month, and the frozen line once it is paid.
type PayrollStored struct {
	Bonus, Deduction int64
	Note, Status     string
	Frozen           []byte
	PaidAt           *time.Time
}

// PayrollRepo filters by the tenant of the Tx.
type PayrollRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	Staff(ctx context.Context, tx Tx, month string, first, last time.Time) ([]PayrollStaff, error)
	Lines(ctx context.Context, tx Tx, month string) (map[string]PayrollStored, error)
	Save(ctx context.Context, tx Tx, month, userID string, bonus, deduction int64, note string) error
	// Lock takes the row lock of a stored line; found is false when the owner has entered nothing yet.
	Lock(ctx context.Context, tx Tx, month, userID string) (status string, found bool, err error)
	// Freeze marks the line PAID with its figures and returns ErrPayrollPaid when it already was.
	Freeze(ctx context.Context, tx Tx, month, userID string, frozen []byte, at time.Time, by string) error
}
