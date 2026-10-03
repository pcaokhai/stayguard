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

// PendingPayment is the state of a checked-out stay's open invoice, enough to resume the check-out. PaymentID is the pending
// transfer, empty when no QR was asked for. Deposit is what was taken at check-in, Received the bank money that did not settle the
// invoice yet, Remaining what is still to pay and RefundDue the deposit to give back when it exceeds the bill. CreatedAt is the
// server time of the check-out.
type PendingPayment struct {
	PaymentID                                      string
	Total, Deposit, Received, Remaining, RefundDue int64
	CreatedAt                                      time.Time
}

// NewPendingPayment derives Remaining from the total, the deposit and the partial bank money.
func NewPendingPayment(paymentID string, total, deposit, received, refundDue int64, createdAt time.Time) *PendingPayment {
	return &PendingPayment{PaymentID: paymentID, Total: total, Deposit: deposit, Received: received,
		Remaining: max(total-deposit-received, 0), RefundDue: refundDue, CreatedAt: createdAt.UTC()}
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
