//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	roomTenantA = "tnt_rm_a"
	// The isolation test seeds its own pair: the migrated database is shared by the package.
	isoTenantA = "tnt_rm_iso_a"
	isoTenantB = "tnt_rm_iso_b"
)

var roomCheckIn = time.Date(2026, 10, 1, 1, 30, 0, 0, time.UTC)

// seedRoomTenant inserts a tenant with 2 buildings, 2 floors, 2 unit types and 4 rooms, through
// the owner connection: A1 has an ACTIVE stay, A2 only a CHECKED_OUT one and a note, B1 is vacant.
func seedRoomTenant(t *testing.T, db, id string) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`INSERT INTO app.tenants (id, name, time_zone) VALUES ('{t}', 'n', 'Asia/Ho_Chi_Minh')`,
		`INSERT INTO app.properties (id, tenant_id, name) VALUES ('{t}_p', '{t}', 'p')`,
		`INSERT INTO app.buildings (id, tenant_id, property_id, code, name) VALUES ('{t}_b1', '{t}', '{t}_p', 'B-A', 'Block A'), ('{t}_b2', '{t}', '{t}_p', 'B-B', 'Block B')`,
		`INSERT INTO app.floors (id, tenant_id, building_id, level) VALUES ('{t}_f1', '{t}', '{t}_b1', 1), ('{t}_f2', '{t}', '{t}_b1', 2), ('{t}_f3', '{t}', '{t}_b2', 1)`,
		`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version) VALUES
			('{t}_ut1', '{t}', 'STD', '{"vi":"Tieu chuan","en":"Standard"}', '{}', 1),
			('{t}_ut2', '{t}', 'VIP', '{"vi":"Cao cap","en":"VIP"}', '{}', 1)`,
		`INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status, attributes) VALUES
			('{t}_u1', '{t}', '{t}_b1', '{t}_f1', '{t}_ut1', 'A101', 'OCCUPIED', '{}'),
			('{t}_u2', '{t}', '{t}_b1', '{t}_f2', '{t}_ut2', 'A201', 'VACANT', '{"note":"sea view"}'),
			('{t}_u3', '{t}', '{t}_b1', '{t}_f2', '{t}_ut1', 'A202', 'MAINTENANCE', '{"note":5}'),
			('{t}_u4', '{t}', '{t}_b2', '{t}_f3', '{t}_ut1', 'B101', 'VACANT', '{}')`,
		`INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, guest_name, check_in_at, rate_plan_snapshot) VALUES
			('{t}_s1', '{t}', '{t}_u1', 'OVERNIGHT', 'ACTIVE', 'Guest One', '2026-10-01T01:30:00Z', '{"v":1}'),
			('{t}_s2', '{t}', '{t}_u2', 'HOURLY', 'CHECKED_OUT', 'Gone', '2026-09-30T01:30:00Z', '{}')`,
	}
	conn := connAs(t, db, "owner")
	for _, s := range stmts {
		if _, err := conn.Exec(ctx, strings.ReplaceAll(s, "{t}", id)); err != nil {
			t.Fatalf("seed %s: %v\n%s", id, err, s)
		}
	}
}

func roomsFor(ctx context.Context, t *testing.T, uow *UnitOfWork, tenant string, f app.RoomFilter) []app.RoomRow {
	t.Helper()
	var rows []app.RoomRow
	err := uow.Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error {
		var err error
		rows, err = RoomRepo{}.Rooms(ctx, tx, f)
		return err
	})
	if err != nil {
		t.Fatalf("rooms: %v", err)
	}
	return rows
}

func TestRoomsQuery_SG201_AC2(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedRoomTenant(t, db, roomTenantA)
	uow := NewUnitOfWork(newAppPool(t, db, 4))
	repo := RoomRepo{}

	all := roomsFor(ctx, t, uow, roomTenantA, app.RoomFilter{})
	if len(all) != 4 {
		t.Fatalf("got %d rooms, want 4", len(all))
	}
	wantOrder := []string{"A101", "A201", "A202", "B101"}
	for i, c := range wantOrder {
		if all[i].Code != c {
			t.Errorf("order[%d] = %s, want %s", i, all[i].Code, c)
		}
	}

	r := all[0]
	if r.Floor != 1 || r.UnitTypeCode != "STD" || r.UnitTypeName != (app.LocalizedName{VI: "Tieu chuan", EN: "Standard"}) ||
		r.StoredStatus != "OCCUPIED" || r.Note != nil || r.BuildingID != roomTenantA+"_b1" {
		t.Errorf("A101 = %+v", r)
	}
	if r.Stay == nil || r.Stay.ID != roomTenantA+"_s1" || r.Stay.RentalType != "OVERNIGHT" || r.Stay.GuestName != "Guest One" ||
		!r.Stay.CheckInAt.Equal(roomCheckIn) || string(r.Stay.RatePlanSnapshot) != `{"v": 1}` {
		t.Errorf("A101 stay = %+v", r.Stay)
	}
	if all[1].Stay != nil {
		t.Errorf("A201 has only a CHECKED_OUT stay, got %+v", all[1].Stay)
	}
	if all[1].Note == nil || *all[1].Note != "sea view" || all[1].UnitTypeCode != "VIP" || all[1].Floor != 2 {
		t.Errorf("A201 = %+v", all[1])
	}
	if all[2].Note != nil {
		t.Errorf("non-string note must be nil, got %v", *all[2].Note)
	}

	if got := roomsFor(ctx, t, uow, roomTenantA, app.RoomFilter{BuildingID: roomTenantA + "_b2"}); len(got) != 1 || got[0].Code != "B101" {
		t.Errorf("building filter = %+v", got)
	}
	if got := roomsFor(ctx, t, uow, roomTenantA, app.RoomFilter{RoomID: roomTenantA + "_u2"}); len(got) != 1 || got[0].Code != "A201" {
		t.Errorf("room filter = %+v", got)
	}

	_ = uow.Do(ctx, roomTenantA, func(ctx context.Context, tx app.Tx) error {
		bs, err := repo.Buildings(ctx, tx)
		if err != nil || len(bs) != 2 || bs[0] != (app.BuildingRow{ID: roomTenantA + "_b1", Code: "B-A", Name: "Block A"}) || bs[1].Code != "B-B" {
			t.Errorf("buildings = %+v err=%v", bs, err)
		}
		tz, err := repo.Timezone(ctx, tx)
		if err != nil || tz != "Asia/Ho_Chi_Minh" {
			t.Errorf("timezone = %q err=%v", tz, err)
		}
		return nil
	})
}

func TestRoomIsolation_SG201_AC4(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedRoomTenant(t, db, isoTenantA)
	seedRoomTenant(t, db, isoTenantB)
	uow := NewUnitOfWork(newAppPool(t, db, 4))

	if got := roomsFor(ctx, t, uow, isoTenantA, app.RoomFilter{}); len(got) != 4 {
		t.Errorf("tenant A sees %d rooms, want 4", len(got))
	}
	for _, f := range []app.RoomFilter{{BuildingID: isoTenantB + "_b1"}, {RoomID: isoTenantB + "_u1"}} {
		if got := roomsFor(ctx, t, uow, isoTenantA, f); len(got) != 0 {
			t.Errorf("filter %+v leaked %d rooms across tenants", f, len(got))
		}
	}
	_ = uow.Do(ctx, isoTenantA, func(ctx context.Context, tx app.Tx) error {
		bs, err := RoomRepo{}.Buildings(ctx, tx)
		if err != nil || len(bs) != 2 {
			t.Errorf("buildings = %+v err=%v", bs, err)
		}
		for _, b := range bs {
			if b.ID[:len(isoTenantA)] != isoTenantA {
				t.Errorf("foreign building %s", b.ID)
			}
		}
		return nil
	})

	// The owner connection bypasses RLS: the explicit tenant filter alone must still isolate.
	ownerTx, err := connAs(t, db, "owner").Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ownerTx.Rollback(ctx) }()
	var tx app.Tx = Tx{tx: pgx.Tx(ownerTx), tenant: isoTenantA}
	rooms, err := RoomRepo{}.Rooms(ctx, tx, app.RoomFilter{})
	if err != nil || len(rooms) != 4 {
		t.Fatalf("RLS-bypassed rooms = %d err=%v, want 4", len(rooms), err)
	}
	rooms, _ = RoomRepo{}.Rooms(ctx, tx, app.RoomFilter{RoomID: isoTenantB + "_u1"})
	if len(rooms) != 0 {
		t.Error("explicit filter leaked a foreign room")
	}
	bs, _ := RoomRepo{}.Buildings(ctx, tx)
	if len(bs) != 2 {
		t.Errorf("RLS-bypassed buildings = %d, want 2", len(bs))
	}
}
