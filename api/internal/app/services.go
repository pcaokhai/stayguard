package app

import (
	"context"
	"fmt"
)

// ListServices returns the extras catalogue ordered by code. Any building level will do: the catalogue
// belongs to the tenant, not to a building.
func (b *Billing) ListServices(ctx context.Context, c Caller) ([]Service, error) {
	if err := b.checkRole("listServices", c); err != nil {
		return nil, err
	}
	var out []Service
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		if out, err = b.services.List(ctx, tx); err != nil {
			return fmt.Errorf("services: %w", err)
		}
		return nil
	})
	return out, err
}
