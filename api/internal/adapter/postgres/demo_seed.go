package postgres

import (
	"context"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
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
		if s.Stock > 0 { // every unit of stock has a movement: the opening count
			err := (SetupRepo{}).InsertMovement(ctx, tx, app.StockMovement{ID: "sm_" + s.ID, ServiceID: s.ID, Kind: "OPENING", Quantity: s.Stock, At: d.SeededAt})
			if err != nil {
				return err
			}
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
	if d.FrontDeskID == "" { // an installer import and a seed without sample money have none
		return nil
	}
	return insertDemoMoney(ctx, tx, t, d)
}

// insertDemoMoney writes the sample bank events, their alerts and the closed shift through the repositories the use cases use.
// The front-desk user is the one the role picker would create for RECEPTIONIST (userFor finds it by role).
func insertDemoMoney(ctx context.Context, tx app.Tx, t Tx, d app.DemoData) error {
	if _, err := (IdentityRepo{}).CreateUser(ctx, tx, d.FrontDeskID, strings.ToLower(string(access.RoleReceptionist)), access.RoleReceptionist, "vi"); err != nil {
		return err
	}
	if err := (IdentityRepo{}).GrantAllBuildings(ctx, tx, d.FrontDeskID, access.EDIT); err != nil {
		return err
	}
	for _, e := range d.Events {
		if _, err := (PaymentRepo{}).InsertEvent(ctx, tx, e.ID, e.Event); err != nil {
			return err
		}
		if err := (PaymentRepo{}).SetEventResult(ctx, tx, e.Event, payment.ResultUnmatched); err != nil {
			return err
		}
	}
	q := sqlcgen.New(t)
	for _, a := range d.Alerts {
		if err := (AlertWriter{}).Raise(ctx, tx, a.Draft); err != nil {
			return err
		}
		if a.Read {
			_, err := q.MarkAlertRead(ctx, sqlcgen.MarkAlertReadParams{ReadAt: ts(a.At), ReadBy: optText(d.FrontDeskID), TenantID: t.tenant, AlertID: a.Draft.ID})
			if err != nil {
				return wrap("mark sample alert read", err)
			}
		}
	}
	if _, err := (ShiftRepo{}).Open(ctx, tx, d.Shift.NewShift); err != nil {
		return err
	}
	return (ShiftRepo{}).Close(ctx, tx, d.Shift.Close)
}
