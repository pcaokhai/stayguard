//go:build integration

package main

import (
	"fmt"
	"testing"
	"time"
)

// seedRoomMap gives a tenant two buildings: bld has a vacant room and an overnight stay that ended
// long before the injected clock (so it is OVERDUE); bld_empty has one vacant room, no stays.
func (e *env) seedRoomMap(tenant, bld, bldEmpty string) {
	e.t.Helper()
	e.seedBuildings(tenant, bld, bldEmpty)
	e.exec(`INSERT INTO app.floors (id, tenant_id, building_id, level) VALUES ($1, $2, $3, 1), ($4, $2, $5, 1)`,
		bld+"_f", tenant, bld, bldEmpty+"_f", bldEmpty)
	e.exec(`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version)
		VALUES ($1, $2, 'STD', '{"vi":"Tieu chuan","en":"Standard"}', '{}', 1)`, bld+"_ut", tenant)
	e.exec(`INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status, attributes) VALUES
		($1, $2, $3, $4, $5, '101', 'OCCUPIED', '{}'),
		($6, $2, $3, $4, $5, '102', 'VACANT', '{"note":"sea view"}'),
		($7, $2, $8, $9, $5, '201', 'VACANT', '{}')`,
		bld+"_u1", tenant, bld, bld+"_f", bld+"_ut", bld+"_u2", bldEmpty+"_u1", bldEmpty, bldEmpty+"_f")
	e.exec(`INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, guest_name, check_in_at, rate_plan_snapshot)
		VALUES ($1, $2, $3, 'OVERNIGHT', 'ACTIVE', 'Guest', $4, '{"graceMinutes":15,"overnight":{"windowEnd":"12:00"}}')`,
		bld+"_s1", tenant, bld+"_u1", e.start.Add(-48*time.Hour))
}

func TestRoomMapE2E_SG201_AC4(t *testing.T) {
	e := newEnv(t)
	a, b := e.demo("OWNER", "vi", ""), e.demo("OWNER", "vi", "")
	ta, tb := a.str("tenantId"), b.str("tenantId")
	e.seedRoomMap(ta, "bld_a", "bld_a2")
	e.seedRoomMap(tb, "bld_b", "bld_b2")
	owner, rec := a.str("accessToken"), e.demo("RECEPTIONIST", "vi", ta).str("accessToken")

	// Owner A: every building at EDIT, OVERDUE derived from the stored OCCUPIED status.
	bs := e.call("GET", "/v1/buildings", owner, nil)
	items, _ := bs.body["items"].([]any)
	if bs.status != 200 || len(items) != 2 {
		t.Fatalf("listBuildings status=%d body=%v", bs.status, bs.body)
	}
	for _, it := range items {
		m := it.(map[string]any)
		if m["level"] != "EDIT" {
			t.Errorf("owner level for %v = %v, want EDIT", m["id"], m["level"])
		}
		if m["id"] == "bld_a" {
			if c, _ := m["counts"].(map[string]any); c["overdue"] != float64(1) || c["vacant"] != float64(1) || c["occupied"] != float64(0) {
				t.Errorf("bld_a counts = %v", m["counts"])
			}
		}
	}
	// A room list without stays works; with an active stay the placeholder quoter answers 503.
	rs := e.call("GET", "/v1/buildings/bld_a2/rooms", owner, nil)
	if rooms, _ := rs.body["items"].([]any); rs.status != 200 || len(rooms) != 1 {
		t.Fatalf("listRooms bld_a2 status=%d body=%v", rs.status, rs.body)
	}
	if r := e.call("GET", "/v1/buildings/bld_a2/rooms?status=VACANT", owner, nil); r.status != 200 {
		t.Errorf("status filter: %d %v", r.status, r.body)
	}
	if r := e.call("GET", "/v1/buildings/bld_a2/rooms?status=NOPE", owner, nil); r.status != 400 {
		t.Errorf("bad status: %d", r.status)
	}
	for _, path := range []string{"/v1/buildings/bld_a/rooms", "/v1/rooms/bld_a_u1"} {
		if r := e.call("GET", path, owner, nil); r.status != 503 || r.str("code") != "PRICING_UNAVAILABLE" {
			t.Errorf("%s: status=%d body=%v (documented limit until SG-101)", path, r.status, r.body)
		}
	}
	if r := e.call("GET", "/v1/rooms/bld_a_u2", owner, nil); r.status != 200 || r.str("note") != "sea view" || r.str("status") != "VACANT" {
		t.Errorf("vacant room: %d %v", r.status, r.body)
	}

	// Receptionist A: derived NONE everywhere, so no buildings and 403 on an existing one.
	if r := e.call("GET", "/v1/buildings", rec, nil); r.status != 200 || len(r.body["items"].([]any)) != 0 {
		t.Errorf("receptionist buildings: %d %v", r.status, r.body)
	}
	if r := e.call("GET", "/v1/buildings/bld_a/rooms", rec, nil); r.status != 403 || r.str("code") != "BUILDING_FORBIDDEN" {
		t.Errorf("receptionist rooms: %d %v", r.status, r.body)
	}

	// Tenant B's ids are indistinguishable from random ids.
	for _, pair := range [][2]string{
		{"/v1/buildings/bld_b/rooms", "/v1/buildings/bld_nope/rooms"},
		{"/v1/rooms/bld_b_u2", "/v1/rooms/bld_nope_u"},
	} {
		foreign, random := e.call("GET", pair[0], owner, nil), e.call("GET", pair[1], owner, nil)
		if foreign.status != 404 || foreign.str("code") != "NOT_FOUND" || fmt.Sprint(foreign.body) != fmt.Sprint(random.body) || random.status != 404 {
			t.Errorf("%s: foreign=%d %v random=%d %v", pair[0], foreign.status, foreign.body, random.status, random.body)
		}
	}
}
