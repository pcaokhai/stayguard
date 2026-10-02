package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// SetupRepo implements app.SetupRepo; the tenant always comes from the Tx.
type SetupRepo struct{}

var _ app.SetupRepo = SetupRepo{}

func (SetupRepo) FirstProperty(ctx context.Context, tx app.Tx) (string, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", false, err
	}
	id, err := sqlcgen.New(t).FirstPropertyID(ctx, t.tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return id, err == nil, wrap("select property", err)
}

func (SetupRepo) InsertBuilding(ctx context.Context, tx app.Tx, id, propertyID, code, name string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert building", sqlcgen.New(t).InsertBuilding(ctx, sqlcgen.InsertBuildingParams{ID: id, TenantID: t.tenant, PropertyID: propertyID, Code: code, Name: name}))
}

func (SetupRepo) Building(ctx context.Context, tx app.Tx, id string) (app.BuildingRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.BuildingRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetBuilding(ctx, sqlcgen.GetBuildingParams{TenantID: t.tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.BuildingRow{}, false, nil
	}
	return app.BuildingRow{ID: r.ID, Code: r.Code, Name: r.Name}, err == nil, wrap("select building", err)
}

func (SetupRepo) RenameBuilding(ctx context.Context, tx app.Tx, id, name string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("rename building", sqlcgen.New(t).UpdateBuildingName(ctx, sqlcgen.UpdateBuildingNameParams{TenantID: t.tenant, ID: id, Name: name}))
}

func (SetupRepo) StatusCounts(ctx context.Context, tx app.Tx, buildingID string) (room.Counts, error) {
	t, err := pgTx(tx)
	if err != nil {
		return room.Counts{}, err
	}
	rows, err := sqlcgen.New(t).BuildingStatusCounts(ctx, sqlcgen.BuildingStatusCountsParams{TenantID: t.tenant, BuildingID: buildingID})
	if err != nil {
		return room.Counts{}, wrap("count rooms", err)
	}
	var c room.Counts
	for _, r := range rows {
		n := int(r.N)
		switch room.Status(r.Status) {
		case room.StatusVacant:
			c.Vacant = n
		case room.StatusOccupied:
			c.Occupied = n
		case room.StatusToClean:
			c.ToClean = n
		case room.StatusMaintenance:
			c.Maintenance = n
		}
	}
	return c, nil
}

func (SetupRepo) InsertFloor(ctx context.Context, tx app.Tx, id, buildingID string, level int, name *string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert floor", sqlcgen.New(t).InsertFloor(ctx, sqlcgen.InsertFloorParams{ID: id, TenantID: t.tenant, BuildingID: buildingID,
		Level: int32(level), Name: text(name)})) //nolint:gosec // a floor level is at most a few dozen
}

func (SetupRepo) Floor(ctx context.Context, tx app.Tx, id string) (app.FloorRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.FloorRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetFloor(ctx, sqlcgen.GetFloorParams{TenantID: t.tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FloorRow{}, false, nil
	}
	return app.FloorRow{ID: r.ID, BuildingID: r.BuildingID, Level: int(r.Level)}, err == nil, wrap("select floor", err)
}

func (SetupRepo) MaxFloorLevel(ctx context.Context, tx app.Tx, buildingID string) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).MaxFloorLevel(ctx, sqlcgen.MaxFloorLevelParams{TenantID: t.tenant, BuildingID: buildingID})
	return int(n), wrap("max floor level", err)
}

func (SetupRepo) ExistingCodes(ctx context.Context, tx app.Tx, codes []string) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	out, err := sqlcgen.New(t).ExistingRoomCodes(ctx, sqlcgen.ExistingRoomCodesParams{TenantID: t.tenant, Codes: codes})
	return out, wrap("existing room codes", err)
}

func (SetupRepo) InsertRoom(ctx context.Context, tx app.Tx, r app.NewRoom) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert room", sqlcgen.New(t).InsertRoom(ctx, sqlcgen.InsertRoomParams{ID: r.ID, TenantID: t.tenant, BuildingID: r.BuildingID,
		FloorID: r.FloorID, UnitTypeID: r.UnitTypeID, Code: r.Code, Status: r.Status, Attributes: r.Attributes}))
}

func localized(raw []byte) (app.LocalizedName, error) {
	var n app.LocalizedName
	return n, json.Unmarshal(raw, &n)
}

func unitTypeRow(id, code string, name, plan []byte, version int32, at pgtype.Timestamptz) (app.UnitTypeRow, error) {
	n, err := localized(name)
	if err != nil {
		return app.UnitTypeRow{}, wrap("decode unit type name", err)
	}
	return app.UnitTypeRow{ID: id, Code: code, Name: n, RatePlan: plan, Version: int(version), UpdatedAt: at.Time}, nil
}

func (SetupRepo) UnitTypeByCode(ctx context.Context, tx app.Tx, code string) (app.UnitTypeRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.UnitTypeRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetUnitTypeByCode(ctx, sqlcgen.GetUnitTypeByCodeParams{TenantID: t.tenant, Code: code})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.UnitTypeRow{}, false, nil
	}
	if err != nil {
		return app.UnitTypeRow{}, false, wrap("select unit type", err)
	}
	u, err := unitTypeRow(r.ID, r.Code, r.Name, r.RatePlan, r.RatePlanVersion, r.UpdatedAt)
	return u, err == nil, err
}

func (SetupRepo) UnitTypes(ctx context.Context, tx app.Tx) ([]app.UnitTypeRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListUnitTypes(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list unit types", err)
	}
	out := make([]app.UnitTypeRow, len(rows))
	for i, r := range rows {
		if out[i], err = unitTypeRow(r.ID, r.Code, r.Name, r.RatePlan, r.RatePlanVersion, r.UpdatedAt); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (SetupRepo) UpdateRatePlan(ctx context.Context, tx app.Tx, id string, plan []byte, version int, now time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("update rate plan", sqlcgen.New(t).UpdateUnitTypeRatePlan(ctx, sqlcgen.UpdateUnitTypeRatePlanParams{RatePlan: plan,
		RatePlanVersion: int32(version), Now: pgtype.Timestamptz{Time: now, Valid: true}, TenantID: t.tenant, ID: id})) //nolint:gosec // a version counts edits
}

func (SetupRepo) Room(ctx context.Context, tx app.Tx, id string) (app.RoomSetupRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.RoomSetupRow{}, false, err
	}
	r, err := sqlcgen.New(t).LockRoomForSetup(ctx, sqlcgen.LockRoomForSetupParams{TenantID: t.tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.RoomSetupRow{}, false, nil
	}
	if err != nil {
		return app.RoomSetupRow{}, false, wrap("lock room", err)
	}
	n, err := localized(r.UnitTypeName)
	if err != nil {
		return app.RoomSetupRow{}, false, wrap("decode unit type name", err)
	}
	return app.RoomSetupRow{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, FloorID: r.FloorID, UnitTypeID: r.UnitTypeID, Status: r.Status,
		FloorLevel: int(r.FloorLevel), UnitTypeCode: r.UnitTypeCode, UnitTypeName: n, Retired: r.Retired, HasGuest: r.HasGuest, Attributes: r.Attributes}, true, nil
}

func (SetupRepo) UpdateRoom(ctx context.Context, tx app.Tx, u app.RoomUpdate) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("update room", sqlcgen.New(t).UpdateRoomRow(ctx, sqlcgen.UpdateRoomRowParams{Code: u.Code, UnitTypeID: u.UnitTypeID, Status: u.Status,
		Retired: u.Retired, Attributes: u.Attributes, TenantID: t.tenant, ID: u.ID}))
}

func (SetupRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", err
	}
	z, err := sqlcgen.New(t).TenantTimezone(ctx, t.tenant)
	return z, wrap("tenant time zone", err)
}

func (SetupRepo) Service(ctx context.Context, tx app.Tx, code string) (app.ServiceItem, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.ServiceItem{}, false, err
	}
	r, err := sqlcgen.New(t).GetServiceForUpdate(ctx, sqlcgen.GetServiceForUpdateParams{TenantID: t.tenant, Code: code})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ServiceItem{}, false, nil
	}
	if err != nil {
		return app.ServiceItem{}, false, wrap("lock service", err)
	}
	n, err := localized(r.Name)
	if err != nil {
		return app.ServiceItem{}, false, wrap("decode service name", err)
	}
	s := app.ServiceItem{ID: r.ID, Code: r.Code, Name: n, Price: r.Price, Stock: r.Stock, Unit: r.Unit, LowStockAt: int(r.LowStockAt), OnSale: r.OnSale}
	if r.LatestUnitCost.Valid {
		s.LatestUnitCost = &r.LatestUnitCost.Int64
	}
	return s, true, nil
}

func (SetupRepo) ServiceCodeTaken(ctx context.Context, tx app.Tx, code string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	ok, err := sqlcgen.New(t).ServiceCodeTaken(ctx, sqlcgen.ServiceCodeTakenParams{TenantID: t.tenant, Code: code})
	return ok, wrap("service code taken", err)
}

func (SetupRepo) InsertService(ctx context.Context, tx app.Tx, s app.ServiceItem) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	name, err := json.Marshal(s.Name)
	if err != nil {
		return err
	}
	return wrap("insert service", sqlcgen.New(t).InsertServiceItem(ctx, sqlcgen.InsertServiceItemParams{ID: s.ID, TenantID: t.tenant, Code: s.Code, Name: name,
		Price: s.Price, Stock: s.Stock, Unit: s.Unit, LowStockAt: int32(s.LowStockAt), OnSale: s.OnSale, LatestUnitCost: nullInt8(s.LatestUnitCost)})) //nolint:gosec // validated non-negative count
}

func nullInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func (SetupRepo) UpdateService(ctx context.Context, tx app.Tx, id string, p app.ServicePatch) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	var name []byte
	if p.Name != nil {
		if name, err = json.Marshal(*p.Name); err != nil {
			return err
		}
	}
	var onSale pgtype.Bool
	if p.OnSale != nil {
		onSale = pgtype.Bool{Bool: *p.OnSale, Valid: true}
	}
	var price pgtype.Int8
	if p.Price != nil {
		price = pgtype.Int8{Int64: *p.Price, Valid: true}
	}
	return wrap("update service", sqlcgen.New(t).UpdateServiceItem(ctx, sqlcgen.UpdateServiceItemParams{Name: name, Price: price, Unit: text(p.Unit),
		LowStockAt: optInt4(p.LowStockAt), OnSale: onSale, TenantID: t.tenant, ID: id}))
}

func (SetupRepo) AddStock(ctx context.Context, tx app.Tx, serviceID string, qty int64, unitCost *int64) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).AddServiceStock(ctx, sqlcgen.AddServiceStockParams{Qty: qty, UnitCost: nullInt8(unitCost), TenantID: t.tenant, ID: serviceID})
	return n, wrap("add stock", err)
}

func (SetupRepo) InsertMovement(ctx context.Context, tx app.Tx, m app.StockMovement) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert stock movement", sqlcgen.New(t).InsertStockMovement(ctx, sqlcgen.InsertStockMovementParams{ID: m.ID, TenantID: t.tenant,
		ServiceID: m.ServiceID, Kind: m.Kind, Quantity: int32(m.Quantity), UnitCost: nullInt8(m.UnitCost), Ref: text(m.Ref), ActorID: text(m.ActorID), //nolint:gosec // bounded by maxRestock
		CreatedAt: pgtype.Timestamptz{Time: m.At, Valid: true}}))
}

func (SetupRepo) Movements(ctx context.Context, tx app.Tx, serviceID string, kind *string, after *app.MovementCursor, limit int) ([]app.MovementRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	p := sqlcgen.ListStockMovementsParams{TenantID: t.tenant, ServiceID: serviceID, Kind: text(kind), PageSize: int32(limit)} //nolint:gosec // a page size is a small constant
	if after != nil {
		p.BeforeAt, p.BeforeID = pgtype.Timestamptz{Time: after.At, Valid: true}, pgtype.Text{String: after.ID, Valid: true}
	}
	rows, err := sqlcgen.New(t).ListStockMovements(ctx, p)
	if err != nil {
		return nil, wrap("list stock movements", err)
	}
	out := make([]app.MovementRow, len(rows))
	for i, r := range rows {
		out[i] = app.MovementRow{ID: r.ID, Kind: r.Kind, Quantity: int64(r.Quantity), Ref: textPtr(r.Ref), At: r.CreatedAt.Time, ActorName: r.ActorName}
		if r.UnitCost.Valid {
			out[i].UnitCost = &r.UnitCost.Int64
		}
	}
	return out, nil
}

func (SetupRepo) ServiceHasSales(ctx context.Context, tx app.Tx, serviceID string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	ok, err := sqlcgen.New(t).ServiceHasSales(ctx, sqlcgen.ServiceHasSalesParams{TenantID: t.tenant, ServiceID: serviceID})
	return ok, wrap("service has sales", err)
}

func (SetupRepo) DeleteService(ctx context.Context, tx app.Tx, serviceID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("delete service", sqlcgen.New(t).DeleteServiceItem(ctx, sqlcgen.DeleteServiceItemParams{TenantID: t.tenant, ID: serviceID}))
}

func (SetupRepo) InsertStocktake(ctx context.Context, tx app.Tx, id, actorID string, note *string, valueDifference int64, at time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert stocktake", sqlcgen.New(t).InsertStocktake(ctx, sqlcgen.InsertStocktakeParams{ID: id, TenantID: t.tenant, ActorID: text(&actorID),
		Note: text(note), ValueDifference: valueDifference, CreatedAt: pgtype.Timestamptz{Time: at, Valid: true}}))
}
