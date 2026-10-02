package app

import (
	"context"
	"time"
)

// Service is one extras catalogue entry as the API shows it.
type Service struct {
	Code  string
	Name  LocalizedName
	Price int64
	Stock int64
	// The owner's view adds these; the front desk list leaves them at their zero values.
	Unit           string
	LowStockAt     int
	OnSale         bool
	LatestUnitCost *int64
	SoldLast7Days  int64
}

// ServiceRow adds the id the stock update and the extras row refer to.
type ServiceRow struct {
	Service
	ID string
}

// ServiceRepo adapters filter by the tenant of the Tx.
type ServiceRepo interface {
	// List is ordered by code. Items that are not on sale are left out unless includeOffSale; since starts the
	// window of SoldLast7Days.
	List(ctx context.Context, tx Tx, includeOffSale bool, since time.Time) ([]Service, error)
	ByCodes(ctx context.Context, tx Tx, codes []string) ([]ServiceRow, error)
	// DecrementStock returns stay.ErrInsufficientStock when no row had enough stock: the atomic guard
	// is the adapter's UPDATE ... WHERE stock >= qty, never a read followed by a write.
	// It returns the price of the row it decremented (UPDATE ... RETURNING price): that is the unit amount,
	// because the price ByCodes read may be stale.
	DecrementStock(ctx context.Context, tx Tx, serviceID string, qty int64) (unitPrice int64, err error)
	// RecordSale writes the SALE movement of a decrement: the room of the stay and the person who added the extra.
	RecordSale(ctx context.Context, tx Tx, movementID, serviceID string, qty int64, stayID, actorID string, at time.Time) error
}
