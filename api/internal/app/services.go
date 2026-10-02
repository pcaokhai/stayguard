package app

import (
	"context"
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// soldWindowDays is the window of an item's recent sales.
const soldWindowDays = 7

// ListServices returns the extras catalogue ordered by code. Any building level will do: the catalogue
// belongs to the tenant, not to a building.
func (b *Billing) ListServices(ctx context.Context, c Caller) ([]Service, error) {
	if err := b.checkRole("listServices", c); err != nil {
		return nil, err
	}
	var out []Service
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		// The owner and manager manage the catalogue and see items that are not on sale.
		manages := c.Role == access.RoleOwner || c.Role == access.RoleManager
		if out, err = b.services.List(ctx, tx, manages, b.clock.Now().AddDate(0, 0, -soldWindowDays)); err != nil {
			return fmt.Errorf("services: %w", err)
		}
		if !manages { // costs and sales volumes are the owner's: the desk sees code, name, price and stock
			for i := range out {
				out[i] = Service{Code: out[i].Code, Name: out[i].Name, Price: out[i].Price, Stock: out[i].Stock}
			}
		}
		return nil
	})
	return out, err
}
