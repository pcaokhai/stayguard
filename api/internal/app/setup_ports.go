package app

import (
	"context"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

type FloorRow struct {
	ID, BuildingID, Name string // Name falls back to the level number
	Level                int
}

// NewRoom is a room to store; Attributes is the JSON object of features and the maintenance note.
type NewRoom struct {
	ID, BuildingID, FloorID, UnitTypeID, Code, Status string
	Attributes                                        []byte
}

type UnitTypeRow struct {
	ID, Code  string
	Name      LocalizedName
	RatePlan  []byte
	Version   int
	UpdatedAt time.Time
}

// RoomSetupRow is a room with what updateRoom decides on; Attributes is the stored JSON object.
type RoomSetupRow struct {
	ID, Code, BuildingID, FloorID, UnitTypeID, Status string
	FloorLevel                                        int
	FloorName                                         string
	UnitTypeCode                                      string
	UnitTypeName                                      LocalizedName
	Retired, HasGuest                                 bool
	Attributes                                        []byte
}

// RoomUpdate is the full new state of a room row.
type RoomUpdate struct {
	ID, Code, UnitTypeID, Status string
	Retired                      bool
	Attributes                   []byte
}

// ServiceItem is an item with its stock and cost as the owner manages it.
type ServiceItem struct {
	ID, Code       string
	Name           LocalizedName
	Price, Stock   int64
	Unit           string
	LowStockAt     int
	OnSale         bool
	LatestUnitCost *int64
}

// ServicePatch changes the given fields only; there is no stock field on purpose.
type ServicePatch struct {
	Name       *LocalizedName
	Price      *int64
	Unit       *string
	LowStockAt *int
	OnSale     *bool
}

// StockMovement is one change of an item's stock; Quantity is signed.
type StockMovement struct {
	ID, ServiceID, Kind string
	Quantity            int64
	UnitCost            *int64
	Ref, ActorID        *string
	At                  time.Time
}

// SetupRepo is tenant-scoped like IdentityRepo.
type SetupRepo interface {
	FirstProperty(ctx context.Context, tx Tx) (string, bool, error)
	// InsertBuilding reports ErrConflict when the code exists.
	InsertBuilding(ctx context.Context, tx Tx, id, propertyID, code, name string) error
	Building(ctx context.Context, tx Tx, id string) (BuildingRow, bool, error)
	RenameBuilding(ctx context.Context, tx Tx, id, name string) error
	StatusCounts(ctx context.Context, tx Tx, buildingID string) (room.Counts, error)
	InsertFloor(ctx context.Context, tx Tx, id, buildingID string, level int, name *string) error
	Floor(ctx context.Context, tx Tx, id string) (FloorRow, bool, error)
	MaxFloorLevel(ctx context.Context, tx Tx, buildingID string) (int, error)
	// Floors of one building in display order.
	Floors(ctx context.Context, tx Tx, buildingID string) ([]FloorView, error)
	// ExistingCodes is the subset of codes that already name a room.
	ExistingCodes(ctx context.Context, tx Tx, codes []string) ([]string, error)
	InsertRoom(ctx context.Context, tx Tx, r NewRoom) error
	UnitTypeByCode(ctx context.Context, tx Tx, code string) (UnitTypeRow, bool, error)
	UnitTypes(ctx context.Context, tx Tx) ([]UnitTypeRow, error)
	UpdateRatePlan(ctx context.Context, tx Tx, id string, plan []byte, version int, now time.Time) error
	// Room locks the row.
	Room(ctx context.Context, tx Tx, id string) (RoomSetupRow, bool, error)
	UpdateRoom(ctx context.Context, tx Tx, u RoomUpdate) error
	Timezone(ctx context.Context, tx Tx) (string, error)
	// Service locks the row; false when unknown.
	Service(ctx context.Context, tx Tx, code string) (ServiceItem, bool, error)
	ServiceCodeTaken(ctx context.Context, tx Tx, code string) (bool, error)
	InsertService(ctx context.Context, tx Tx, s ServiceItem) error
	UpdateService(ctx context.Context, tx Tx, id string, p ServicePatch) error
	// AddStock adds (or with a negative qty removes) stock; unitCost, when set, becomes the latest cost. Returns the new stock.
	AddStock(ctx context.Context, tx Tx, serviceID string, qty int64, unitCost *int64) (int64, error)
	InsertMovement(ctx context.Context, tx Tx, m StockMovement) error
	// Movements is a page of an item's history, newest first, starting after the cursor.
	Movements(ctx context.Context, tx Tx, serviceID string, kind *string, after *MovementCursor, limit int) ([]MovementRow, error)
	ServiceHasSales(ctx context.Context, tx Tx, serviceID string) (bool, error)
	// DeleteService removes an item that was never sold, with its movements.
	DeleteService(ctx context.Context, tx Tx, serviceID string) error
	InsertStocktake(ctx context.Context, tx Tx, id, actorID string, note *string, valueDifference int64, at time.Time) error
}

// MovementCursor is the position after the last row of a page.
type MovementCursor struct {
	At time.Time
	ID string
}

// MovementRow is one history line with the name of whoever caused it.
type MovementRow struct {
	ID, Kind  string
	Quantity  int64
	UnitCost  *int64
	Ref       *string
	At        time.Time
	ActorName string
}
