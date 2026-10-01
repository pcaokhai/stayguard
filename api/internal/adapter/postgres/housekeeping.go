package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// HousekeepingRepo implements app.HousekeepingRepo. The tenant always comes from the Tx.
type HousekeepingRepo struct{}

var _ app.HousekeepingRepo = HousekeepingRepo{}

func (HousekeepingRepo) ToClean(ctx context.Context, tx app.Tx) ([]app.CleanRoom, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListToCleanRooms(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list rooms to clean", err)
	}
	out := make([]app.CleanRoom, len(rows))
	for i, r := range rows {
		out[i] = app.CleanRoom{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Status: "TO_CLEAN", CleaningSince: r.CleaningSince.Time}
	}
	return out, nil
}

func (HousekeepingRepo) LockRoom(ctx context.Context, tx app.Tx, roomID string) (app.CleanRoom, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.CleanRoom{}, false, err
	}
	r, err := sqlcgen.New(t).LockRoomForClean(ctx, sqlcgen.LockRoomForCleanParams{TenantID: t.tenant, UnitID: roomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CleanRoom{}, false, nil
	}
	if err != nil {
		return app.CleanRoom{}, false, wrap("lock room", err)
	}
	return app.CleanRoom{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Status: r.Status, CleaningSince: r.CleaningSince.Time}, true, nil
}

func (HousekeepingRepo) MarkClean(ctx context.Context, tx app.Tx, roomID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).MarkRoomClean(ctx, sqlcgen.MarkRoomCleanParams{TenantID: t.tenant, UnitID: roomID})
	if err != nil {
		return wrap("mark room clean", err)
	}
	if n != 1 {
		return app.ErrRoomNotToClean
	}
	return nil
}
