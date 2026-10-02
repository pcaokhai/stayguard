package app

import (
	"context"
	"time"
)

// StayEdit is one correction of a stay (kind CHECK_IN or MOVE) as stored.
type StayEdit struct {
	ID, StayID, Kind, ActorID  string
	OldCheckInAt, NewCheckInAt *time.Time
	ReasonCode, Note           string
	FromRoomCode, ToRoomCode   string
	CreatedAt                  time.Time
}

// MovedStay is what a move writes: the stay now sits in ToRoomID, priced by that room type's plan.
type MovedStay struct {
	StayID, ToRoomID, RentalType string
	RatePlanSnapshot             []byte
	RatePlanSchema               int
}

// StayEditRepo filters by the tenant of the Tx. LockStay and Room(forUpdate) take the row locks that
// serialize edits against check-out and check-in.
type StayEditRepo interface {
	LockStay(ctx context.Context, tx Tx, stayID string) (StayRecord, bool, error)
	Room(ctx context.Context, tx Tx, roomID string, forUpdate bool) (CheckInRoom, bool, error)
	Timezone(ctx context.Context, tx Tx) (string, error)
	// FirstRecordedCheckIn is the check-in time before any correction: the old time of the first CHECK_IN edit.
	FirstRecordedCheckIn(ctx context.Context, tx Tx, stayID string) (time.Time, bool, error)
	// UpdateCheckIn returns stay.ErrNotActive when the stay was not ACTIVE: the backstop.
	UpdateCheckIn(ctx context.Context, tx Tx, stayID string, at time.Time) error
	// MoveStay returns stay.ErrNotActive (stay not ACTIVE) or room.ErrNotVacant (one-active-stay index).
	MoveStay(ctx context.Context, tx Tx, m MovedStay) error
	MarkRoomOccupied(ctx context.Context, tx Tx, roomID string) error
	// MarkRoomToClean turns an OCCUPIED room TO_CLEAN and returns room.ErrNotOccupied otherwise.
	MarkRoomToClean(ctx context.Context, tx Tx, roomID string) error
	InsertEdit(ctx context.Context, tx Tx, e StayEdit) error
}
