package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// ServiceRepo implements app.ServiceRepo. The tenant always comes from the Tx, never from callers.
type ServiceRepo struct{}

var _ app.ServiceRepo = ServiceRepo{}

func (ServiceRepo) List(ctx context.Context, tx app.Tx) ([]app.Service, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListServices(ctx, t.tenant)
	if err != nil {
		return nil, wrap("select services", err)
	}
	out := make([]app.Service, len(rows))
	for i, r := range rows {
		row, err := toServiceRow(r.ID, r.Code, r.Name, r.Price, r.Stock)
		if err != nil {
			return nil, err
		}
		out[i] = row.Service
	}
	return out, nil
}

func (ServiceRepo) ByCodes(ctx context.Context, tx app.Tx, codes []string) ([]app.ServiceRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListServicesByCodes(ctx, sqlcgen.ListServicesByCodesParams{TenantID: t.tenant, Codes: codes})
	if err != nil {
		return nil, wrap("select services by code", err)
	}
	out := make([]app.ServiceRow, len(rows))
	for i, r := range rows {
		if out[i], err = toServiceRow(r.ID, r.Code, r.Name, r.Price, r.Stock); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecrementStock relies on the guard in the UPDATE (stock >= qty): no row back means not enough stock
// (or no such service for this tenant, which the caller cannot tell apart and need not).
func (ServiceRepo) DecrementStock(ctx context.Context, tx app.Tx, serviceID string, qty int64) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	price, err := sqlcgen.New(t).DecrementServiceStock(ctx, sqlcgen.DecrementServiceStockParams{
		TenantID: t.tenant, ServiceID: serviceID, Qty: qty})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, stay.ErrInsufficientStock
	}
	if err != nil {
		return 0, wrap("decrement service stock", err)
	}
	return price, nil
}

func toServiceRow(id, code string, rawName []byte, price, stock int64) (app.ServiceRow, error) {
	var name struct{ VI, EN string }
	if err := json.Unmarshal(rawName, &name); err != nil {
		// The raw value could hold tenant data; report the service id only.
		return app.ServiceRow{}, fmt.Errorf("decode name of service %s: %w", id, err)
	}
	return app.ServiceRow{ID: id, Service: app.Service{Code: code, Name: app.LocalizedName(name), Price: price, Stock: stock}}, nil
}
