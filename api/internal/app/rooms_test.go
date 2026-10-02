package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const (
	tenantA  = "tn_a"
	tenantB  = "tn_b"
	zoneName = "Asia/Ho_Chi_Minh"
	snapJSON = `{"graceMinutes":15,"overnight":{"windowEnd":"12:00"},"daily":{"windowEnd":"11:00"}}`
)

// roomsNow is 2026-03-10 14:00 in Ho Chi Minh (UTC+7).
var roomsNow = time.Date(2026, 3, 10, 7, 0, 0, 0, time.UTC)

type roomsEnv struct {
	svc    *Rooms
	repo   *fakeRoomRepo
	levels *fakeLevels
	quoter *fakeQuoter
	uow    *fakeUoW
}

func newRoomsEnv(rooms []RoomRow, lv map[string]access.Level) *roomsEnv {
	repo := &fakeRoomRepo{
		tz: zoneName,
		buildings: map[string][]BuildingRow{
			tenantA: {{ID: "bl_1", Code: "A", Name: "Block A"}, {ID: "bl_2", Code: "B", Name: "Block B"}, {ID: "bl_3", Code: "C", Name: "Block C"}},
			tenantB: {{ID: "bl_x", Code: "X", Name: "Other"}},
		},
		rooms: map[string][]RoomRow{tenantA: rooms, tenantB: {{ID: "rm_x", Code: "X1", BuildingID: "bl_x", StoredStatus: "VACANT"}}},
	}
	levels := &fakeLevels{levels: lv}
	quoter := &fakeQuoter{total: 350000}
	uow := &fakeUoW{}
	return &roomsEnv{NewRooms(uow, repo, levels, quoter, fixedClock{roomsNow}), repo, levels, quoter, uow}
}

func caller(role access.Role) Caller { return Caller{TenantID: tenantA, UserID: "us_1", Role: role} }

func vacant(id, building string) RoomRow {
	return RoomRow{ID: id, Code: id, BuildingID: building, StoredStatus: "VACANT"}
}

func stayed(id, building, rt string, checkIn time.Time) RoomRow {
	r := vacant(id, building)
	r.StoredStatus = "OCCUPIED"
	r.Stay = &StayRow{ID: "st_" + id, RentalType: rt, GuestName: "Guest", CheckInAt: checkIn, RatePlanSnapshot: []byte(snapJSON)}
	return r
}

var allLevels = map[string]access.Level{"bl_1": access.EDIT, "bl_2": access.VIEW, "bl_3": access.NONE}

func TestListBuildings_SG201_AC1(t *testing.T) {
	overdueIn := roomsNow.Add(-30 * time.Hour) // overnight, expected end passed yesterday
	rooms := []RoomRow{
		vacant("r1", "bl_1"), stayed("r2", "bl_1", "HOURLY", roomsNow.Add(-2*time.Hour)),
		stayed("r3", "bl_1", "OVERNIGHT", overdueIn),
		{ID: "r4", Code: "r4", BuildingID: "bl_2", StoredStatus: "TO_CLEAN"},
		{ID: "r5", Code: "r5", BuildingID: "bl_2", StoredStatus: "MAINTENANCE"},
		vacant("r6", "bl_3"),
	}
	for _, role := range []access.Role{access.RoleOwner, access.RoleReceptionist, access.RoleHousekeeping} {
		t.Run(string(role), func(t *testing.T) {
			env := newRoomsEnv(rooms, allLevels)
			got, err := env.svc.ListBuildings(context.Background(), caller(role))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 2 || got[0].ID != "bl_1" || got[1].ID != "bl_2" {
				t.Fatalf("NONE building must be dropped: %+v", got)
			}
			if got[0].Level != access.EDIT || got[1].Level != access.VIEW || got[0].Code != "A" || got[0].Name != "Block A" {
				t.Fatalf("levels or names wrong: %+v", got)
			}
			want := room.Counts{Vacant: 1, Occupied: 1, Overdue: 1}
			if got[0].Counts != want {
				t.Fatalf("counts = %+v, want %+v", got[0].Counts, want)
			}
			if got[1].Counts != (room.Counts{ToClean: 1, Maintenance: 1}) {
				t.Fatalf("counts b2 = %+v", got[1].Counts)
			}
			if len(env.uow.tenants) != 1 || env.uow.tenants[0] != tenantA {
				t.Fatalf("uow tenants = %v", env.uow.tenants)
			}
		})
	}
}

func TestListBuildings_Tenant_SG201_AC1(t *testing.T) {
	env := newRoomsEnv([]RoomRow{vacant("r1", "bl_1")}, map[string]access.Level{"bl_x": access.EDIT, "bl_1": access.EDIT})
	c := caller(access.RoleOwner)
	c.TenantID = tenantB
	got, err := env.svc.ListBuildings(context.Background(), c)
	if err != nil || len(got) != 1 || got[0].ID != "bl_x" || got[0].Counts != (room.Counts{Vacant: 1}) {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestListRooms_SG201_AC2(t *testing.T) {
	note := "near lift"
	r1 := stayed("r1", "bl_1", "OVERNIGHT", roomsNow.Add(-90*time.Minute-30*time.Second))
	r1.Floor, r1.UnitTypeCode, r1.Note = 2, "VIP", &note
	r1.UnitTypeName = LocalizedName{VI: "Phong VIP", EN: "VIP room"}
	overdue := stayed("r2", "bl_1", "DAILY", roomsNow.Add(-48*time.Hour))
	env := newRoomsEnv([]RoomRow{r1, vacant("r3", "bl_1"), overdue, vacant("r9", "bl_2")}, allLevels)
	c := caller(access.RoleReceptionist)

	got, err := env.svc.ListRooms(context.Background(), c, "bl_1", nil)
	if err != nil || len(got) != 3 {
		t.Fatalf("got %d rooms, err %v", len(got), err)
	}
	g := got[0]
	if g.Floor != 2 || g.UnitTypeCode != "VIP" || g.UnitTypeName.EN != "VIP room" || g.UnitTypeName.VI != "Phong VIP" || *g.Note != note || g.Status != room.StatusOccupied {
		t.Fatalf("room fields: %+v", g)
	}
	s := g.ActiveStay
	if s == nil || s.ID != "st_r1" || s.GuestName != "Guest" || s.ElapsedMinutes != 90 || s.RunningTotal != 350000 || s.RentalType != room.Overnight {
		t.Fatalf("stay: %+v", s)
	}
	if got[1].ActiveStay != nil || got[1].Status != room.StatusVacant {
		t.Fatalf("vacant room: %+v", got[1])
	}
	if got[2].Status != room.StatusOverdue {
		t.Fatalf("daily past 11:00+grace must be OVERDUE: %+v", got[2])
	}
	if len(env.quoter.calls) != 2 {
		t.Fatalf("quoter calls = %d, want 2", len(env.quoter.calls))
	}
	q := env.quoter.calls[0]
	if q.snapshot != snapJSON || q.rt != room.Overnight || !q.checkIn.Equal(r1.Stay.CheckInAt) || !q.now.Equal(roomsNow) || q.zone != zoneName {
		t.Fatalf("quote call: %+v", q)
	}

	for _, st := range []room.Status{room.StatusOverdue, room.StatusVacant, room.StatusOccupied} {
		got, err := env.svc.ListRooms(context.Background(), c, "bl_1", &st)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range got {
			if v.Status != st {
				t.Fatalf("filter %s returned %s", st, v.Status)
			}
		}
		if len(got) != 1 {
			t.Fatalf("filter %s: %d rooms", st, len(got))
		}
	}
}

func TestRoomsQueryCount_SG201_AC5(t *testing.T) {
	counts := map[int]int{}
	for _, n := range []int{1, 50} {
		var rows []RoomRow
		for i := range n {
			rows = append(rows, stayed(fmt.Sprintf("r%d", i), "bl_1", "HOURLY", roomsNow.Add(-time.Hour)))
		}
		env := newRoomsEnv(rows, allLevels)
		if _, err := env.svc.ListRooms(context.Background(), caller(access.RoleOwner), "bl_1", nil); err != nil {
			t.Fatal(err)
		}
		counts[n] = env.repo.calls
	}
	if counts[1] != counts[50] {
		t.Fatalf("repo calls vary with rooms: %v", counts)
	}
}

func TestRoomAccess_SG201_AC4(t *testing.T) {
	rooms := []RoomRow{vacant("r1", "bl_1"), vacant("r3", "bl_3")}
	ctx := context.Background()
	t.Run("NONE is building forbidden", func(t *testing.T) {
		env := newRoomsEnv(rooms, allLevels)
		_, err := env.svc.ListRooms(ctx, caller(access.RoleReceptionist), "bl_3", nil)
		if !errors.Is(err, access.ErrBuildingForbidden) {
			t.Fatalf("listRooms err = %v", err)
		}
		_, err = env.svc.GetRoom(ctx, caller(access.RoleReceptionist), "r3")
		if !errors.Is(err, access.ErrBuildingForbidden) {
			t.Fatalf("getRoom err = %v", err)
		}
	})
	t.Run("VIEW is enough", func(t *testing.T) {
		env := newRoomsEnv([]RoomRow{vacant("r2", "bl_2")}, allLevels)
		if _, err := env.svc.GetRoom(ctx, caller(access.RoleReceptionist), "r2"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("role first, before any repo call", func(t *testing.T) {
		env := newRoomsEnv(rooms, allLevels)
		_, err := env.svc.ListRooms(ctx, Caller{TenantID: tenantA, Role: access.Role("GUEST")}, "bl_1", nil)
		if !errors.Is(err, access.ErrRoleForbidden) {
			t.Fatalf("err = %v", err)
		}
		_, err = env.svc.GetRoom(ctx, Caller{TenantID: tenantA, Role: access.Role("GUEST")}, "r1")
		if !errors.Is(err, access.ErrRoleForbidden) || env.repo.calls != 0 || env.levels.calls != 0 {
			t.Fatalf("err = %v, repo calls %d", err, env.repo.calls)
		}
	})
	t.Run("missing and foreign ids are not found, even for NONE", func(t *testing.T) {
		env := newRoomsEnv(rooms, map[string]access.Level{})
		c := caller(access.RoleReceptionist)
		for _, id := range []string{"bl_nope", "bl_x"} {
			if _, err := env.svc.ListRooms(ctx, c, id, nil); !errors.Is(err, ErrNotFound) {
				t.Fatalf("building %s err = %v", id, err)
			}
		}
		for _, id := range []string{"rm_nope", "rm_x"} {
			if _, err := env.svc.GetRoom(ctx, c, id); !errors.Is(err, ErrNotFound) {
				t.Fatalf("room %s err = %v", id, err)
			}
		}
	})
}

func TestRoomsPricingUnavailable_SG201_AC2(t *testing.T) {
	env := newRoomsEnv([]RoomRow{stayed("r1", "bl_1", "HOURLY", roomsNow.Add(-time.Hour))}, allLevels)
	env.quoter.err = ErrPricingUnavailable
	c := caller(access.RoleOwner)
	if _, err := env.svc.ListRooms(context.Background(), c, "bl_1", nil); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatalf("listRooms err = %v", err)
	}
	if _, err := env.svc.GetRoom(context.Background(), c, "r1"); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatalf("getRoom err = %v", err)
	}
}

func TestRoomsFailClosed_SG201_AC3(t *testing.T) {
	badStatus := vacant("r1", "bl_1")
	badStatus.StoredStatus = "OVERDUE"
	badSnap := stayed("r1", "bl_1", "OVERNIGHT", roomsNow.Add(-time.Hour))
	badSnap.Stay.RatePlanSnapshot = []byte(`{`)
	badRental := stayed("r1", "bl_1", "WEEKLY", roomsNow.Add(-time.Hour))
	for name, row := range map[string]RoomRow{"status": badStatus, "snapshot": badSnap, "rental type": badRental} {
		t.Run(name, func(t *testing.T) {
			env := newRoomsEnv([]RoomRow{row}, allLevels)
			c := caller(access.RoleOwner)
			if _, err := env.svc.ListRooms(context.Background(), c, "bl_1", nil); err == nil {
				t.Fatal("listRooms: want error")
			}
			if _, err := env.svc.GetRoom(context.Background(), c, "r1"); err == nil {
				t.Fatal("getRoom: want error")
			}
			if _, err := env.svc.ListBuildings(context.Background(), c); err == nil {
				t.Fatal("listBuildings: want error")
			}
		})
	}
	t.Run("zone", func(t *testing.T) {
		env := newRoomsEnv([]RoomRow{vacant("r1", "bl_1")}, allLevels)
		env.repo.tz = "Mars/Olympus"
		if _, err := env.svc.GetRoom(context.Background(), caller(access.RoleOwner), "r1"); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestRoomsHardening_SG201_AC4(t *testing.T) {
	ctx := context.Background()
	c := caller(access.RoleOwner)
	rooms := []RoomRow{vacant("r1", "bl_1"), vacant("r2", "bl_2")}
	t.Run("empty room id is not found", func(t *testing.T) {
		env := newRoomsEnv(rooms, allLevels)
		if _, err := env.svc.GetRoom(ctx, c, ""); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("adapter returning a different room is not found", func(t *testing.T) {
		env := newRoomsEnv(rooms, allLevels)
		env.repo.ignoreFilter = true
		if _, err := env.svc.GetRoom(ctx, c, "r2"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("adapter ignoring the building filter cannot leak rooms", func(t *testing.T) {
		env := newRoomsEnv(rooms, allLevels)
		env.repo.ignoreFilter = true
		got, err := env.svc.ListRooms(ctx, c, "bl_1", nil)
		if err != nil || len(got) != 1 || got[0].ID != "r1" {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("building missing from levels fails closed", func(t *testing.T) {
		env := newRoomsEnv(rooms, map[string]access.Level{})
		rc := caller(access.RoleReceptionist)
		if _, err := env.svc.ListRooms(ctx, rc, "bl_1", nil); !errors.Is(err, access.ErrBuildingForbidden) {
			t.Fatalf("listRooms err = %v", err)
		}
		if _, err := env.svc.GetRoom(ctx, rc, "r1"); !errors.Is(err, access.ErrBuildingForbidden) {
			t.Fatalf("getRoom err = %v", err)
		}
	})
}

func TestListBuildingsLevelRange_SG201_AC1(t *testing.T) {
	env := newRoomsEnv([]RoomRow{vacant("r1", "bl_1")}, map[string]access.Level{"bl_1": access.Level(9), "bl_2": access.Level(-1), "bl_3": access.VIEW})
	got, err := env.svc.ListBuildings(context.Background(), caller(access.RoleOwner))
	if err != nil || len(got) != 1 || got[0].ID != "bl_3" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestRoomsStayOnlyWhenOccupied_SG201_AC2(t *testing.T) {
	for _, stored := range []string{"VACANT", "TO_CLEAN", "MAINTENANCE"} {
		t.Run(stored, func(t *testing.T) {
			row := stayed("r1", "bl_1", "HOURLY", roomsNow.Add(-time.Hour))
			row.StoredStatus = stored
			env := newRoomsEnv([]RoomRow{row}, allLevels)
			got, err := env.svc.GetRoom(context.Background(), caller(access.RoleOwner), "r1")
			if err != nil || got.ActiveStay != nil || len(env.quoter.calls) != 0 {
				t.Fatalf("stay %+v quotes %d err %v", got.ActiveStay, len(env.quoter.calls), err)
			}
		})
	}
}

func TestRoomsLevelsCallCount_SG201_AC5(t *testing.T) {
	counts := map[int]int{}
	for _, n := range []int{1, 50} {
		var rows []RoomRow
		for i := range n {
			rows = append(rows, vacant(fmt.Sprintf("r%d", i), "bl_1"))
		}
		env := newRoomsEnv(rows, allLevels)
		if _, err := env.svc.ListRooms(context.Background(), caller(access.RoleOwner), "bl_1", nil); err != nil {
			t.Fatal(err)
		}
		counts[n] = env.levels.calls
	}
	if counts[1] != counts[50] {
		t.Fatalf("levels calls vary with rooms: %v", counts)
	}
}

// Building.floors lists each building's floors in display order with names (the level number when unnamed).
func TestListBuildings_Floors_Integration(t *testing.T) {
	env := newRoomsEnv(nil, map[string]access.Level{"bl_1": access.VIEW})
	env.repo.floors = []FloorRow{{ID: "f2", BuildingID: "bl_1", Name: "Rooftop", Level: 2}, {ID: "f1", BuildingID: "bl_1", Name: "1", Level: 1}}
	got, err := env.svc.ListBuildings(context.Background(), caller(access.RoleReceptionist))
	if err != nil || len(got) == 0 || len(got[0].Floors) != 2 || got[0].Floors[0].ID != "f2" || got[0].Floors[1].Order != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}
