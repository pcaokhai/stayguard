//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func (e *env) ownerSetup() string {
	e.t.Helper()
	e.seedOwner()
	e.seedStayTenant(staffTenant, 2)
	// A second room type to change to; its plan is the seed plan.
	e.exec(`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version)
		VALUES ('ut_vip', $1, 'VIP', '{"vi":"VIP","en":"VIP"}', $2, 1)`, staffTenant, seedPlanSnapshot())
	r := e.signIn("owner1", ownerPIN)
	if r.status != 200 {
		e.t.Fatalf("owner sign-in: %d %v", r.status, r.body)
	}
	return r.str("accessToken")
}

func (e *env) put(path, token string, body any) reply   { return e.call("PUT", path, token, body) }
func (e *env) patch(path, token string, body any) reply { return e.call("PATCH", path, token, body) }
func (e *env) post(path, token string, body any) (int, reply) {
	st, raw := e.send("POST", path, token, newKey(), body)
	return st, reply{status: st, body: parse(raw)}
}

func items(r reply) []map[string]any {
	var out []map[string]any
	for _, it := range r.body["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

// SG-1003 AC2: previewPrice runs the same pricing engine as check-out; every golden case goes through the endpoint.
func TestPreviewPrice_GoldenCasesThroughEndpoint_SG1003(t *testing.T) {
	e := newEnv(t)
	token := e.ownerSetup()
	raw, err := os.ReadFile("../../../contracts/pricing/golden-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		GraceMinutes int                       `json:"graceMinutes"`
		RatePlans    map[string]map[string]int `json:"ratePlans"`
		Cases        []struct {
			ID, RoomType, RentalType, CheckIn, CheckOut string
			Expected                                    struct {
				Total  int64
				Capped bool
				Lines  []struct {
					Code                         string
					Quantity, UnitAmount, Amount int64
				}
			}
		}
	}
	if err = json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, c := range golden.Cases {
		p := golden.RatePlans[c.RoomType]
		plan := map[string]any{"graceMinutes": golden.GraceMinutes,
			"hourly":    map[string]any{"firstHour": p["firstHour"], "extraHour": p["extraHour"]},
			"overnight": map[string]any{"price": p["overnight"], "windowStart": "21:00", "windowEnd": "12:00"},
			"daily":     map[string]any{"price": p["daily"], "windowStart": "14:00", "windowEnd": "12:00"}}
		r := e.call("POST", "/v1/owner/rate-plans/preview", token, map[string]any{"rentalType": c.RentalType, "checkIn": c.CheckIn, "checkOut": c.CheckOut, "ratePlan": plan})
		if r.status != 200 || int64(r.body["total"].(float64)) != c.Expected.Total || r.body["capped"] != c.Expected.Capped {
			t.Errorf("%s: %d %v want total %d capped %v", c.ID, r.status, r.body, c.Expected.Total, c.Expected.Capped)
			continue
		}
		lines := r.body["lines"].([]any)
		if len(lines) != len(c.Expected.Lines) {
			t.Errorf("%s: %d lines, want %d", c.ID, len(lines), len(c.Expected.Lines))
			continue
		}
		for i, l := range c.Expected.Lines {
			g := lines[i].(map[string]any)
			if g["code"] != l.Code || int64(g["amount"].(float64)) != l.Amount {
				t.Errorf("%s line %d: %v want %+v", c.ID, i, g, l)
			}
		}
	}
	// A bad draft is a 422, and the manager may preview but the front desk may not.
	bad := map[string]any{"rentalType": "HOURLY", "checkIn": "2026-10-05T10:00:00+07:00", "checkOut": "2026-10-05T11:00:00+07:00",
		"ratePlan": map[string]any{"graceMinutes": 99, "hourly": map[string]any{"firstHour": 1, "extraHour": 1},
			"overnight": map[string]any{"price": 1, "windowStart": "21:00", "windowEnd": "12:00"}, "daily": map[string]any{"price": 1, "windowStart": "14:00", "windowEnd": "12:00"}}}
	if r := e.call("POST", "/v1/owner/rate-plans/preview", token, bad); r.status != 422 {
		t.Fatalf("invalid draft: %d %v", r.status, r.body)
	}
}

// SG-1002: generated rooms, no staff access for a new building, rooms by range, and the guest rules of updateRoom.
func TestSetupRooms_SG1002(t *testing.T) {
	e := newEnv(t)
	token := e.ownerSetup()
	key := newKey()
	body := map[string]any{"code": "C", "name": "Building C", "floors": 2, "roomsPerFloor": 3, "unitTypeCode": "STD"}
	st, raw := e.send("POST", "/v1/owner/buildings", token, key, body)
	b := parse(raw)
	bid, _ := b["id"].(string)
	if st != 201 || bid == "" || b["counts"].(map[string]any)["vacant"] != float64(6) || b["level"] != "EDIT" {
		t.Fatalf("createBuilding: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/owner/buildings", token, key, body); st != 201 || parse(raw)["id"] != bid || e.count(`SELECT count(*) FROM app.units WHERE building_id = $1`, bid) != 6 {
		t.Fatalf("replay: %d %s", st, raw)
	}
	if st, _ = e.post("/v1/owner/buildings", token, body); st != 409 {
		t.Fatalf("duplicate code: %d", st)
	}
	if n := e.count(`SELECT count(*) FROM app.building_permissions WHERE building_id = $1`, bid); n != 0 {
		t.Fatalf("a new building starts with no staff access, got %d rows", n)
	}
	for _, bad := range []map[string]any{{"code": "c", "name": "x", "floors": 1, "roomsPerFloor": 1, "unitTypeCode": "STD"}, {"code": "D", "name": "x", "floors": 1, "roomsPerFloor": 1, "unitTypeCode": "NOPE"}, {"code": "D", "name": "x", "floors": 31, "roomsPerFloor": 1, "unitTypeCode": "STD"}} {
		if st, _ = e.post("/v1/owner/buildings", token, bad); st != 422 {
			t.Errorf("%v: %d", bad, st)
		}
	}
	if r := e.patch("/v1/owner/buildings/"+bid, token, map[string]any{"name": "Block C"}); r.status != 200 || r.str("name") != "Block C" {
		t.Fatalf("rename: %d %v", r.status, r.body)
	}

	// A floor with rooms, then a range with features that is not ready yet.
	st, r := e.post("/v1/owner/buildings/"+bid+"/floors", token, map[string]any{"name": "Floor 3", "rooms": map[string]any{"count": 2, "startCode": "C301", "unitTypeCode": "STD"}})
	rooms := items(r)
	if st != 201 || len(rooms) != 2 || rooms[0]["code"] != "C301" || rooms[1]["code"] != "C302" || rooms[0]["floor"] != float64(3) {
		t.Fatalf("createFloor: %d %v", st, r.body)
	}
	floor := e.owner.QueryRow(context.Background(), `SELECT id FROM app.floors WHERE building_id = $1 AND level = 3`, bid)
	var fid string
	if err := floor.Scan(&fid); err != nil {
		t.Fatal(err)
	}
	rng := map[string]any{"buildingId": bid, "floorId": fid, "fromCode": "C401", "toCode": "C403", "unitTypeCode": "VIP", "features": []string{"WINDOW"}, "availableNow": false}
	st, r = e.post("/v1/owner/rooms", token, rng)
	got := items(r)
	if st != 201 || len(got) != 3 || got[2]["code"] != "C403" || got[0]["status"] != "TO_CLEAN" || got[0]["unitType"].(map[string]any)["code"] != "VIP" {
		t.Fatalf("createRooms: %d %v", st, r.body)
	}
	if st, _ = e.post("/v1/owner/rooms", token, rng); st != 409 {
		t.Fatalf("existing codes: %d", st)
	}
	rng["toCode"] = "C400"
	if st, _ = e.post("/v1/owner/rooms", token, rng); st != 422 {
		t.Fatalf("backwards range: %d", st)
	}

	// updateRoom: a guest blocks type change, maintenance and retiring; a vacant room can do all three.
	guest := roomID(1)
	if st, _ := e.checkIn(token, 1, newKey(), stayBody(nil)); st != 201 {
		t.Fatalf("check-in: %d", st)
	}
	for name, patch := range map[string]map[string]any{
		"type": {"unitTypeCode": "VIP"}, "retire": {"retired": true}, "maintenance": {"maintenance": map[string]any{"on": true, "reason": "leak"}},
	} {
		if r = e.patch("/v1/owner/rooms/"+guest, token, patch); r.status != 409 || r.str("code") != "ROOM_OCCUPIED" {
			t.Errorf("%s with a guest: %d %v", name, r.status, r.body)
		}
	}
	if r = e.patch("/v1/owner/rooms/"+guest, token, map[string]any{"features": []string{"BATHTUB"}}); r.status != 200 {
		t.Errorf("features with a guest are fine: %d %v", r.status, r.body)
	}
	free := roomID(2)
	if r = e.patch("/v1/owner/rooms/"+free, token, map[string]any{"unitTypeCode": "VIP", "code": "R2X"}); r.status != 200 || r.str("code") != "R2X" || r.str("unitType", "code") != "VIP" {
		t.Fatalf("type and code of a vacant room: %d %v", r.status, r.body)
	}
	if r = e.patch("/v1/owner/rooms/"+free, token, map[string]any{"maintenance": map[string]any{"on": true, "reason": "leak", "expectedBackOn": "2026-10-09"}}); r.status != 200 || r.str("status") != "MAINTENANCE" || r.str("note") != "leak" {
		t.Fatalf("maintenance on: %d %v", r.status, r.body)
	}
	if r = e.patch("/v1/owner/rooms/"+free, token, map[string]any{"maintenance": map[string]any{"on": false}}); r.str("status") != "VACANT" || r.status != 200 {
		t.Fatalf("maintenance off: %d %v", r.status, r.body)
	}
	if r = e.patch("/v1/owner/rooms/"+free, token, map[string]any{"code": "C301"}); r.status != 409 {
		t.Fatalf("taken code: %d", r.status)
	}
	if r = e.patch("/v1/owner/rooms/"+free, token, map[string]any{"retired": true}); r.status != 200 {
		t.Fatalf("retire: %d %v", r.status, r.body)
	}
	list := e.call("GET", "/v1/buildings/"+stayBuilding+"/rooms", token, nil)
	for _, it := range items(list) {
		if it["id"] == free {
			t.Fatal("a retired room leaves the map")
		}
	}
	if st, _ := e.checkIn(token, 2, newKey(), stayBody(nil)); st != 404 {
		t.Fatalf("a retired room cannot be checked in: %d", st)
	}
	if r = e.patch("/v1/owner/rooms/nope", token, map[string]any{"code": "Z"}); r.status != 404 {
		t.Fatalf("unknown room: %d", r.status)
	}
}

// SG-1003 AC1: a new rate plan version applies to later check-ins; stays keep the snapshot they were priced with.
func TestSetupRatePlan_VersionsKeepStaySnapshots_SG1003(t *testing.T) {
	e := newEnv(t)
	token := e.ownerSetup()
	st, raw := e.checkIn(token, 1, newKey(), stayBody(nil))
	oldStay := parse(raw)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	plans := e.call("GET", "/v1/owner/rate-plans", token, nil)
	var std map[string]any
	for _, it := range items(plans) {
		if it["code"] == "STD" {
			std = it
		}
	}
	if plans.status != 200 || std == nil || std["ratePlan"].(map[string]any)["version"] != float64(1) {
		t.Fatalf("list: %d %v", plans.status, plans.body)
	}
	plan := std["ratePlan"].(map[string]any)
	plan["hourly"] = map[string]any{"firstHour": 99000, "extraHour": 25000}
	delete(plan, "version")
	r := e.put("/v1/owner/unit-types/STD/rate-plan", token, plan)
	if r.status != 200 || r.body["ratePlan"].(map[string]any)["version"] != float64(2) {
		t.Fatalf("update: %d %v", r.status, r.body)
	}
	if got := e.call("GET", "/v1/stays/"+oldStay["id"].(string), token, nil); got.body["pricingVersion"] != float64(1) {
		t.Fatalf("the running stay keeps its snapshot: %v", got.body["pricingVersion"])
	}
	st, raw = e.checkIn(token, 2, newKey(), stayBody(nil))
	if st != 201 || parse(raw)["pricingVersion"] != float64(2) {
		t.Fatalf("a later check-in uses version 2: %d %s", st, raw)
	}
	plan["graceMinutes"] = 99
	if r = e.put("/v1/owner/unit-types/STD/rate-plan", token, plan); r.status != 422 {
		t.Fatalf("invalid plan: %d", r.status)
	}
	if r = e.put("/v1/owner/unit-types/NOPE/rate-plan", token, plan); r.status != 404 {
		t.Fatalf("unknown type: %d", r.status)
	}
}

// SG-1004 AC1-2: stock changes only through movements; editing an item never edits stock.
func TestSetupItemsAndStock_SG1004(t *testing.T) {
	e := newEnv(t)
	token := e.ownerSetup()
	key := newKey()
	body := map[string]any{"name": map[string]any{"vi": "Nuoc suoi", "en": "Spring water"}, "price": 10000, "unitCost": 6000, "openingQuantity": 24, "unit": "bottle", "lowStockAt": 6}
	st, raw := e.send("POST", "/v1/owner/services", token, key, body)
	item := parse(raw)
	if st != 201 || item["code"] != "SPRING_WATER" || item["stock"] != float64(24) || item["latestUnitCost"] != float64(6000) || item["onSale"] != true {
		t.Fatalf("createService: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/owner/services", token, key, body); st != 201 || parse(raw)["code"] != "SPRING_WATER" || e.count(`SELECT count(*) FROM app.stock_movements WHERE kind = 'OPENING' AND quantity = 24`) != 1 {
		t.Fatalf("replay: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/owner/services", token, newKey(), body); st != 201 || parse(raw)["code"] != "SPRING_WATER_2" {
		t.Fatalf("a second item with the same English name gets another code: %d %s", st, raw)
	}
	// Editing never touches stock, even if a stock field is sent.
	r := e.patch("/v1/owner/services/SPRING_WATER", token, map[string]any{"price": 12000, "lowStockAt": 10, "onSale": false, "stock": 999})
	if r.status == 200 && r.body["stock"] != float64(24) {
		t.Fatalf("stock changed by an edit: %v", r.body)
	}
	if r.status != 200 && r.status != 400 {
		t.Fatalf("edit: %d %v", r.status, r.body)
	}
	if r = e.patch("/v1/owner/services/SPRING_WATER", token, map[string]any{"price": 12000, "lowStockAt": 10, "onSale": false}); r.status != 200 || r.body["price"] != float64(12000) || r.body["onSale"] != false || r.body["stock"] != float64(24) {
		t.Fatalf("edit: %d %v", r.status, r.body)
	}
	st, raw = e.send("POST", "/v1/owner/services/SPRING_WATER/restock", token, newKey(), map[string]any{"quantity": 12, "unitCost": 6500})
	if st != 200 || parse(raw)["stock"] != float64(36) || parse(raw)["latestUnitCost"] != float64(6500) {
		t.Fatalf("restock: %d %s", st, raw)
	}
	if st, _ = e.send("POST", "/v1/owner/services/SPRING_WATER/restock", token, newKey(), map[string]any{"quantity": 0, "unitCost": 1}); st != 400 && st != 422 {
		t.Fatalf("zero quantity: %d", st)
	}
	if st, _ = e.send("POST", "/v1/owner/services/NOPE/restock", token, newKey(), map[string]any{"quantity": 1, "unitCost": 1}); st != 404 {
		t.Fatalf("unknown item: %d", st)
	}
	// The books balance: stock equals the sum of its movements.
	if n := e.count(`SELECT count(*) FROM app.services s WHERE s.code = 'SPRING_WATER' AND s.stock = (SELECT sum(quantity) FROM app.stock_movements m WHERE m.service_id = s.id)`); n != 1 {
		t.Fatal("stock must equal the sum of its movements")
	}
	if n := e.count(`SELECT count(*) FROM app.stock_movements WHERE kind = 'IN' AND unit_cost = 6500 AND actor_id = 'us_owner'`); n != 1 {
		t.Fatal("the IN movement keeps cost and actor")
	}
	// The front desk no longer sees an item that is not on sale.
	for _, it := range items(e.call("GET", "/v1/services", token, nil)) {
		if it["code"] == "SPRING_WATER" {
			t.Fatal("an item that is not on sale must not be offered at the desk")
		}
	}
}
