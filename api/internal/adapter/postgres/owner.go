package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
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

// CashExpected is the cash that should be in the drawers for [from, to), worked out with shift.ExpectedCash like the shift screen: the
// opening float of the first shift open in the window, plus every cash line of those shifts and the owner cash with no shift, in minus
// out. The float a shift leaves is the next shift's opening float (the same cash), so only the first one is counted.
func (OwnerRepo) CashExpected(ctx context.Context, tx app.Tx, from, to time.Time) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	q := sqlcgen.New(t)
	shifts, err := q.OwnerCashShifts(ctx, sqlcgen.OwnerCashShiftsParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return 0, wrap("owner cash shifts", err)
	}
	var opening, in, out int64
	for i, s := range shifts {
		if i == 0 {
			opening = s.OpeningFloat
		}
		in += s.CashIn
		out += s.CashOut
	}
	own, err := q.OwnerCashNoShift(ctx, sqlcgen.OwnerCashNoShiftParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return 0, wrap("owner cash", err)
	}
	n, err := shift.ExpectedCash(opening, in+own.CashIn, out+own.CashOut)
	if err != nil {
		return 0, fmt.Errorf("expected cash: %w", err)
	}
	return n, nil
}
