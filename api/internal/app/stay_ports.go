package app

import (
	"context"
	"errors"
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

// ExtraRecord is one extras row; ServiceCode is the service code, not its id.
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

// NewExtra is the row inserted when an extra is added; the amount (quantity times unit) is computed by the adapter.
type NewExtra struct {
	ID, StayID, ServiceID string
	Quantity, UnitAmount  int64
	Amount                int64 // Quantity times UnitAmount; the table CHECK still guards it
	CreatedAt             time.Time
}

// NewInvoice is the row inserted at check-out; Quote is the frozen quote JSON.
type NewInvoice struct {
	ID, StayID, BillCode string
	Quote                []byte
	Total                int64
	CreatedAt            time.Time
}

type InvoiceRecord struct {
	ID, StayID, BillCode, Status string
	Quote                        []byte
	Total                        int64
	CreatedAt                    time.Time
}

// ErrBillCodeConflict: InsertInvoice hit the unique bill code of the tenant (a concurrent check-out took
// the probed code). The adapter maps the unique violation to it; Checkout retries the whole unit of work.
var ErrBillCodeConflict = errors.New("bill code already taken")

// BillingStayRepo is what the billing use cases need from the stay store. It is separate from
// StayRepo so the check-in use cases and their adapter wiring stay as they are. Filters by the tenant of the Tx.
type BillingStayRepo interface {
	StayByID(ctx context.Context, tx Tx, stayID string) (StayRecord, bool, error)
	Timezone(ctx context.Context, tx Tx) (string, error)
	// LockStay takes the row lock that serializes extras and check-out on one stay.
	LockStay(ctx context.Context, tx Tx, stayID string) (StayRecord, bool, error)
	InsertExtra(ctx context.Context, tx Tx, e NewExtra) error
	// MarkCheckedOut returns stay.ErrNotActive when the stay was not ACTIVE: the backstop.
	MarkCheckedOut(ctx context.Context, tx Tx, stayID string, at time.Time) error
	InvoiceByStay(ctx context.Context, tx Tx, stayID string) (InvoiceRecord, bool, error)
	InsertInvoice(ctx context.Context, tx Tx, n NewInvoice) error
	// BillCodeTaken is an exact match on the tenant's bill codes; the unique constraint is the backstop.
	BillCodeTaken(ctx context.Context, tx Tx, code string) (bool, error)
}
