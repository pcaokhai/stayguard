package app

import "context"

// Service is one extras catalogue entry as the API shows it.
type Service struct {
	Code  string
	Name  LocalizedName
	Price int64
	Stock int64
}

// ServiceRow adds the id the stock update and the extras row refer to.
type ServiceRow struct {
	Service
	ID string
}

// ServiceRepo adapters filter by the tenant of the Tx.
type ServiceRepo interface {
	// List is ordered by code.
	List(ctx context.Context, tx Tx) ([]Service, error)
	ByCodes(ctx context.Context, tx Tx, codes []string) ([]ServiceRow, error)
	// DecrementStock returns stay.ErrInsufficientStock when no row had enough stock: the atomic guard
	// is the adapter's UPDATE ... WHERE stock >= qty, never a read followed by a write.
	DecrementStock(ctx context.Context, tx Tx, serviceID string, qty int64) error
}
