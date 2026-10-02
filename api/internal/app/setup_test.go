package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

func TestCodeSpan_SG1002(t *testing.T) {
	for _, c := range []struct {
		from, to string
		want     []string
		bad      bool
	}{
		{"A107", "A109", []string{"A107", "A108", "A109"}, false},
		{"A107", "A107", []string{"A107"}, false},
		{"A098", "A101", []string{"A098", "A099", "A100", "A101"}, false},
		{"R1", "R3", []string{"R1", "R2", "R3"}, false},
		{"A107", "A106", nil, true}, // backwards
		{"A107", "B108", nil, true}, // other letters
		{"ROOM", "ROOM", nil, true}, // no number
		{"A1", "A500", nil, true},   // more than 200 rooms
		{"A 1", "A 2", nil, true},   // spaces
	} {
		got, err := codeSpan(c.from, c.to)
		if c.bad != (err != nil) || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s..%s: %v %v", c.from, c.to, got, err)
		}
	}
}

func TestServiceCodeFrom_SG1004(t *testing.T) {
	for in, want := range map[string]string{"Spring water": "SPRING_WATER", "  Bia 333 (lon) ": "BIA_333_LON", "???": "ITEM", "Nước suối": "N_C_SU_I"} {
		if got := serviceCodeFrom(in); got != want {
			t.Errorf("%q: %q want %q", in, got, want)
		}
	}
	if len(serviceCodeFrom(string(make([]byte, 200)))) > maxServiceCodeLen-4 {
		t.Error("code too long")
	}
}

func TestRoomUpdateRules_SG1002(t *testing.T) {
	s := &Setup{}
	cur := RoomSetupRow{ID: "r", Code: "A101", UnitTypeCode: "STD", UnitTypeID: "ut1", Status: "OCCUPIED", HasGuest: true}
	vip, yes := "VIP", true
	for name, in := range map[string]RoomUpdateInput{
		"type": {UnitTypeCode: &vip}, "retire": {Retired: &yes}, "maintenance": {Maintenance: &MaintenanceInput{On: true}},
	} {
		if _, err := s.applyRoomUpdate(nil, nil, cur, in); !errors.Is(err, ErrRoomOccupied) { //nolint:staticcheck // no database call on these paths
			t.Errorf("%s: %v", name, err)
		}
	}
	same := "STD"
	if _, err := s.applyRoomUpdate(nil, nil, cur, RoomUpdateInput{UnitTypeCode: &same}); err != nil { //nolint:staticcheck // as above
		t.Errorf("the same type is no change: %v", err)
	}
}

func TestSetupRoles_SG1002(t *testing.T) {
	mgr := Caller{TenantID: "t", UserID: "u", Role: access.RoleManager}
	s := &Setup{}
	for name, err := range map[string]error{
		"createBuilding": func() error { _, err := s.CreateBuilding(nil, mgr, "k", BuildingInput{}); return err }(),   //nolint:staticcheck // role check comes first
		"updateRatePlan": func() error { _, err := s.UpdateRatePlan(nil, mgr, "STD", RatePlanInput{}); return err }(), //nolint:staticcheck // as above
		"createFloor":    func() error { _, err := s.CreateFloor(nil, mgr, "k", "b", FloorInput{}); return err }(),    //nolint:staticcheck // as above
	} {
		if !errors.Is(err, access.ErrRoleForbidden) {
			t.Errorf("manager %s: %v", name, err)
		}
	}
}
