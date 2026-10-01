package postgres

import (
	"context"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// OwnerRepo implements app.OwnerRepo. The tenant always comes from the Tx.
type OwnerRepo struct{}

var _ app.OwnerRepo = OwnerRepo{}

func (OwnerRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (OwnerRepo) Revenue(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.BuildingRevenue, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).OwnerRevenueByBuilding(ctx, sqlcgen.OwnerRevenueByBuildingParams{FromAt: ts(from), ToAt: ts(to), TenantID: t.tenant})
	if err != nil {
		return nil, wrap("owner revenue", err)
	}
	out := make([]app.BuildingRevenue, len(rows))
	for i, r := range rows {
		out[i] = app.BuildingRevenue{BuildingID: r.ID, Name: r.Name, Cash: r.Cash, Transfer: r.Transfer}
	}
	return out, nil
}

func (OwnerRepo) LatestPayments(ctx context.Context, tx app.Tx, limit int) ([]app.PaymentSummary, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).OwnerLatestPayments(ctx, sqlcgen.OwnerLatestPaymentsParams{TenantID: t.tenant, MaxRows: int32(limit)}) //nolint:gosec // limit is a small constant
	if err != nil {
		return nil, wrap("owner latest payments", err)
	}
	out := make([]app.PaymentSummary, len(rows))
	for i, r := range rows {
		out[i] = app.PaymentSummary{PaymentID: r.ID, RoomCode: r.RoomCode, Method: r.Method, Amount: r.Amount, At: r.PaidAt.Time.UTC()}
	}
	return out, nil
}
