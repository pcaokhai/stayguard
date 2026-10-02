package postgres

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// DemoSeedRepo implements app.DemoSeedRepo. The tenant always comes from the Tx.
type DemoSeedRepo struct{}

var _ app.DemoSeedRepo = DemoSeedRepo{}

// InsertDemoData writes parents before children; rooms go in with their final status and the sample
// stays reuse the check-in insert.
func (DemoSeedRepo) InsertDemoData(ctx context.Context, tx app.Tx, d app.DemoData) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	tid := t.tenant
	if err := q.InsertProperty(ctx, sqlcgen.InsertPropertyParams{ID: d.PropertyID, TenantID: tid, Name: d.PropertyName,
		Address: text(d.PropertyAddress), Phone: text(d.PropertyPhone)}); err != nil {
		return wrap("insert property", err)
	}
	for _, a := range d.BankAccounts {
		if err := (BankRepo{}).InsertAccount(ctx, tx, a); err != nil {
			return err
		}
	}
	for _, b := range d.Buildings {
		if err := q.InsertBuilding(ctx, sqlcgen.InsertBuildingParams{ID: b.ID, TenantID: tid, PropertyID: d.PropertyID, Code: b.Code, Name: b.Name}); err != nil {
			return wrap("insert building", err)
		}
	}
	for _, f := range d.Floors {
		if err := q.InsertFloor(ctx, sqlcgen.InsertFloorParams{ID: f.ID, TenantID: tid, BuildingID: f.BuildingID, Level: f.Level}); err != nil {
			return wrap("insert floor", err)
		}
	}
	for _, u := range d.UnitTypes {
		if err := q.InsertUnitType(ctx, sqlcgen.InsertUnitTypeParams{ID: u.ID, TenantID: tid, Code: u.Code, Name: u.Name, RatePlan: u.RatePlan, RatePlanVersion: u.Version}); err != nil {
			return wrap("insert unit type", err)
		}
	}
	for _, r := range d.Rooms {
		if err := q.InsertUnit(ctx, sqlcgen.InsertUnitParams{ID: r.ID, TenantID: tid, BuildingID: r.BuildingID, FloorID: r.FloorID, UnitTypeID: r.UnitTypeID, Code: r.Code, Status: r.Status}); err != nil {
			return wrap("insert room", err)
		}
	}
	for _, s := range d.Services {
		if err := q.InsertService(ctx, sqlcgen.InsertServiceParams{ID: s.ID, TenantID: tid, Code: s.Code, Name: s.Name, Price: s.Price, Stock: s.Stock}); err != nil {
			return wrap("insert service", err)
		}
	}
	for _, s := range d.Stays {
		if err := (StayRepo{}).InsertStay(ctx, tx, s); err != nil {
			return err
		}
	}
	for _, e := range d.Extras {
		if err := (BillingRepo{}).InsertExtra(ctx, tx, e); err != nil {
			return err
		}
	}
	return nil
}
