package app

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrLeaveConflict: a roster change or a leave request clashes with leave that stands (HTTP 409 LEAVE_CONFLICT).
	ErrLeaveConflict = errors.New("clashes with leave")
	// ErrLeaveState: the action does not apply to the request in its current status (HTTP 409 LEAVE_STATE).
	ErrLeaveState = errors.New("leave request is in another status")
	// ErrRosterNotEmpty: the week to copy into already has assignments (HTTP 409 ROSTER_NOT_EMPTY).
	ErrRosterNotEmpty = errors.New("the week already has assignments")
)

// RosterCell is one person on one shift of a day; Date is a calendar day at midnight UTC.
type RosterCell struct {
	UserID string
	// UserName is filled when assignments are read; a change names the person by id only.
	UserName string `json:"userName,omitempty"`
	Date     time.Time
	Shift    string
}

// LeaveRow is a stored leave request; From and To are calendar days at midnight UTC, Shift is empty for the whole day.
type LeaveRow struct {
	ID, UserID, UserName, Shift, Kind, Reason, CoverUserID, Status, DeclineReason string
	From, To                                                                      time.Time
	CreatedAt                                                                     time.Time
	DecidedAt                                                                     *time.Time
}

// LeaveFilter: every field is optional. Standing keeps only requests that can still change the roster.
type LeaveFilter struct {
	Status, UserID string
	From, To       *time.Time
	Standing       bool
}

// RosterRepo filters by the tenant of the Tx.
type RosterRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	Assignments(ctx context.Context, tx Tx, from, to time.Time) ([]RosterCell, error)
	// ActiveUsers answers which of the ids are people who have not been removed.
	ActiveUsers(ctx context.Context, tx Tx, ids []string) (map[string]bool, error)
	Remove(ctx context.Context, tx Tx, cells []RosterCell) error
	Add(ctx context.Context, tx Tx, cells []RosterCell, by string) error
	CountBetween(ctx context.Context, tx Tx, from, to time.Time) (int, error)
	CopyWeek(ctx context.Context, tx Tx, srcFrom time.Time, by string) error
	Scheduled(ctx context.Context, tx Tx, userID string, day time.Time) ([]string, error)

	Leave(ctx context.Context, tx Tx, f LeaveFilter) ([]LeaveRow, error)
	LeaveByID(ctx context.Context, tx Tx, id string) (LeaveRow, bool, error)
	// LockLeave takes the row lock of a request; false when there is none.
	LockLeave(ctx context.Context, tx Tx, id string) (bool, error)
	InsertLeave(ctx context.Context, tx Tx, l LeaveRow) error
	SetLeaveStatus(ctx context.Context, tx Tx, id, status, declineReason string, at time.Time, by string) error
	OverlappingLeave(ctx context.Context, tx Tx, userID string, from, to time.Time) (int, error)
	PaidLeaveDays(ctx context.Context, tx Tx, userID string, yearStart, yearEnd time.Time) (int, error)
	AnnualLeaveDays(ctx context.Context, tx Tx, userID string) (int, error)
}
