package app

import (
	"context"
	"time"
)

// CheckInRoom is the room as check-in needs it: status, building and its unit type's rate plan.
type CheckInRoom struct {
	ID, Code, BuildingID string
	StoredStatus         string
	RatePlan             []byte
	RatePlanVersion      int
}

// NewStay is the row inserted at check-in; IDNumberEnc is already ciphertext (nil when none).
type NewStay struct {
	ID, RoomID, RentalType, GuestName, GuestPhone string
	IDNumberEnc                                   []byte
	Deposit                                       int64
	CheckInAt                                     time.Time
	RatePlanSnapshot                              []byte
	RatePlanSchema                                int
}

type ExtraRecord struct {
	ServiceCode string
	Name        LocalizedName
	Quantity    int64
	UnitAmount  int64
}

// StayRecord is a stored stay with its room, building and extras.
type StayRecord struct {
	ID, RoomID, RoomCode, BuildingID, RentalType, Status string
	GuestName, GuestPhone                                string
	IDNumberEnc                                          []byte
	Deposit                                              int64
	CheckInAt                                            time.Time
	CheckOutAt                                           *time.Time
	RatePlanSnapshot                                     []byte
	Extras                                               []ExtraRecord
}

// StayRepo adapters filter by the tenant of the Tx.
type StayRepo interface {
	// Room with forUpdate takes the row lock that serializes concurrent check-ins.
	Room(ctx context.Context, tx Tx, roomID string, forUpdate bool) (CheckInRoom, bool, error)
	InsertStay(ctx context.Context, tx Tx, s NewStay) error
	// MarkRoomOccupied returns room.ErrNotVacant when no vacant row was updated: the backstop.
	MarkRoomOccupied(ctx context.Context, tx Tx, roomID string) error
	StayByID(ctx context.Context, tx Tx, stayID string) (StayRecord, bool, error)
	Timezone(ctx context.Context, tx Tx) (string, error)
}
