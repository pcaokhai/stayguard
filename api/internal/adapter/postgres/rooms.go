package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// RoomRepo implements app.RoomRepo. The tenant always comes from the Tx, never from callers.
type RoomRepo struct{}

var _ app.RoomRepo = RoomRepo{}

func (RoomRepo) Buildings(ctx context.Context, tx app.Tx) ([]app.BuildingRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListBuildings(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list buildings", err)
	}
	out := make([]app.BuildingRow, len(rows))
	for i, r := range rows {
		out[i] = app.BuildingRow{ID: r.ID, Code: r.Code, Name: r.Name}
	}
	return out, nil
}

func (RoomRepo) Rooms(ctx context.Context, tx app.Tx, f app.RoomFilter) ([]app.RoomRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListRooms(ctx, sqlcgen.ListRoomsParams{TenantID: t.tenant, BuildingID: f.BuildingID, UnitID: f.RoomID})
	if err != nil {
		return nil, wrap("list rooms", err)
	}
	out := make([]app.RoomRow, len(rows))
	for i, r := range rows {
		if out[i], err = toRoomRow(r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (RoomRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", err
	}
	tz, err := sqlcgen.New(t).TenantTimezone(ctx, t.tenant)
	return tz, wrap("select tenant timezone", err)
}

func toRoomRow(r sqlcgen.ListRoomsRow) (app.RoomRow, error) {
	var name struct{ VI, EN string }
	if err := json.Unmarshal(r.UnitTypeName, &name); err != nil {
		// The raw value could hold tenant data; report the unit id only.
		return app.RoomRow{}, fmt.Errorf("decode unit type name of room %s: %w", r.ID, err)
	}
	row := app.RoomRow{
		ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Floor: int(r.FloorLevel), FloorID: r.FloorID, FloorName: floorLabel(r.FloorName, int(r.FloorLevel)),
		UnitTypeCode: r.UnitTypeCode, UnitTypeName: app.LocalizedName(name), StoredStatus: r.Status,
	}
	if r.HasNote {
		note := r.Note
		row.Note = &note
	}
	if r.StayID.Valid {
		row.Stay = &app.StayRow{
			ID: r.StayID.String, RentalType: r.StayRentalType.String, GuestName: r.StayGuestName.String,
			CheckInAt: r.StayCheckInAt.Time, RatePlanSnapshot: r.StayRatePlanSnapshot,
		}
		if r.InvID.Valid {
			row.Stay.CheckOutAt = timePtr(r.StayCheckOutAt)
			row.Stay.Pending = app.NewPendingPayment(r.InvPaymentID, r.InvTotal, r.InvDeposit, r.InvReceived, r.InvRefundDue, r.InvCreatedAt.Time)
		}
	}
	return row, nil
}

func (RoomRepo) Floors(ctx context.Context, tx app.Tx) ([]app.FloorRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListFloors(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list floors", err)
	}
	out := make([]app.FloorRow, len(rows))
	for i, r := range rows {
		out[i] = app.FloorRow{ID: r.ID, BuildingID: r.BuildingID, Name: floorLabel(r.Name, int(r.Level)), Level: int(r.Level)}
	}
	return out, nil
}
