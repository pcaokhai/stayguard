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

type StayRow struct {
	ID, RentalType, GuestName string
	CheckInAt                 time.Time
	RatePlanSnapshot          []byte
}

type RoomRow struct {
	ID, Code, BuildingID string
	Floor                int
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
	Timezone(ctx context.Context, tx Tx) (string, error)
}

// StayQuoter returns the running total in whole VND (rule 1); the pricing engine binds it later.
type StayQuoter interface {
	RunningTotal(ctx context.Context, snapshot []byte, rt room.RentalType, checkIn, now time.Time, loc *time.Location) (int64, error)
}
