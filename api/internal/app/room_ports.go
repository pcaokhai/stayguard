package app

import (
	"context"
	"errors"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrPricingUnavailable = errors.New("pricing unavailable")
	// ErrFeatureDisabled is returned when the slice flag is off; the http adapter maps it.
	ErrFeatureDisabled = errors.New("feature disabled")
)

// BuildingLevels gives the effective level per building; an id missing from the map means NONE.
type BuildingLevels interface {
	Levels(ctx context.Context, c Caller, buildingIDs []string) (map[string]access.Level, error)
}

type LocalizedName struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

type BuildingRow struct{ ID, Code, Name string }

// FloorView is a floor of a building in display order.
type FloorView struct {
	ID, Name string
	Order    int
}

type StayRow struct {
	ID, RentalType, GuestName string
	CheckInAt                 time.Time
	RatePlanSnapshot          []byte
	// CheckOutAt and Pending are set for a checked-out stay whose invoice is still open (the room stays OCCUPIED until it is paid).
	CheckOutAt *time.Time
	Pending    *PendingPayment
}

// PendingPayment is the money state of a checked-out stay's open invoice. PaymentID is the pending transfer, empty when no QR was
// asked for yet. Received is the deposit plus bank money that did not settle the invoice; Remaining is what is left to pay.
type PendingPayment struct {
	PaymentID                  string
	Total, Received, Remaining int64
}

// NewPendingPayment derives the amounts from the invoice total, the deposit and the partial bank money.
func NewPendingPayment(paymentID string, total, deposit, partial int64) *PendingPayment {
	received := deposit + partial
	return &PendingPayment{PaymentID: paymentID, Total: total, Received: received, Remaining: max(total-received, 0)}
}

type RoomRow struct {
	ID, Code, BuildingID string
	Floor                int
	FloorID, FloorName   string // the name falls back to the level number when the floor has none
	UnitTypeCode         string
	UnitTypeName         LocalizedName
	StoredStatus         string
	Note                 *string
	Stay                 *StayRow
}

// RoomFilter: both empty means every room of the tenant; otherwise one building or one room.
type RoomFilter struct{ BuildingID, RoomID string }

// RoomRepo has one query shape per call, so a request never issues a query per room.
type RoomRepo interface {
	Buildings(ctx context.Context, tx Tx) ([]BuildingRow, error)
	Rooms(ctx context.Context, tx Tx, f RoomFilter) ([]RoomRow, error)
	Floors(ctx context.Context, tx Tx) ([]FloorRow, error)
	Timezone(ctx context.Context, tx Tx) (string, error)
}

// StayQuoter returns the running total in whole VND (rule 1); the pricing engine binds it later.
type StayQuoter interface {
	RunningTotal(ctx context.Context, snapshot []byte, rt room.RentalType, checkIn, now time.Time, loc *time.Location) (int64, error)
}
