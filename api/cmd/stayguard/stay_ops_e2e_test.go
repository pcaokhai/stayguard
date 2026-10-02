//go:build integration

package main

import (
	"fmt"
	"testing"
	"time"
)

// The five stay operations of L-B1 over HTTP: the edit window, the move, the history, the timeline and the receipt.
func TestStayOpsE2E_SG801(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	e.clock.set(e.start)

	st, raw := e.checkIn(token, 1, newKey(), stayBody(map[string]any{"rentalType": "HOURLY"}))
	stayID, _ := parse(raw)["id"].(string)
	if st != 201 || stayID == "" {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(2 * time.Hour))

	// 61 minutes after the recorded time is a 422 naming the field; 30 minutes later is accepted.
	late := map[string]any{"newCheckInAt": e.start.Add(61 * time.Minute).Format(time.RFC3339), "reasonCode": "WRONG_TIME", "note": "typed wrong"}
	if st, raw = e.send("POST", "/v1/stays/"+stayID+"/check-in-time", token, newKey(), late); st != 422 || parse(raw)["code"] != "VALIDATION_FAILED" {
		t.Fatalf("out of range: %d %s", st, raw)
	}
	ok := map[string]any{"newCheckInAt": e.start.Add(30 * time.Minute).Format(time.RFC3339), "reasonCode": "LATE_ARRIVAL", "note": "came late"}
	if st, raw = e.send("POST", "/v1/stays/"+stayID+"/check-in-time", token, newKey(), ok); st != 200 {
		t.Fatalf("edit: %d %s", st, raw)
	}
	var alerts int
	if err := e.owner.QueryRow(t.Context(), `SELECT count(*) FROM app.alerts WHERE tenant_id = $1 AND kind = 'STAY_TIME_EDITED'`, tenant).Scan(&alerts); err != nil || alerts != 1 {
		t.Fatalf("STAY_TIME_EDITED alerts: %d %v", alerts, err)
	}

	// Move to room 2, then the history, the timeline and (after check-out and cash) the receipt.
	if st, raw = e.send("POST", "/v1/stays/"+stayID+"/move", token, newKey(), map[string]any{"toRoomId": roomID(2), "rentalType": "HOURLY"}); st != 200 || parse(raw)["roomCode"] != "R2" {
		t.Fatalf("move: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/stays/"+stayID+"/move", token, newKey(), map[string]any{"toRoomId": roomID(2), "rentalType": "HOURLY"}); st != 422 {
		t.Fatalf("move to the same room: %d %s", st, raw)
	}
	day := e.start.Format("2006-01-02")
	st, raw = e.send("GET", fmt.Sprintf("/v1/stays?date=%s&q=r2", day), token, "", nil)
	items, _ := parse(raw)["items"].([]any)
	if st != 200 || len(items) != 1 || items[0].(map[string]any)["state"] != "TIME_EDITED" {
		t.Fatalf("history: %d %s", st, raw)
	}
	if st, raw = e.send("GET", "/v1/owner/stays/"+stayID+"/timeline", token, "", nil); st != 200 || len(parse(raw)["items"].([]any)) != 3 {
		t.Fatalf("timeline: %d %s", st, raw)
	}
	if st, raw = e.send("GET", "/v1/stays?date=2000-01-01", token, "", nil); st != 200 {
		t.Fatalf("an owner may look at any day: %d %s", st, raw)
	}

	st, raw = e.checkout(token, stayID, newKey())
	inv, _ := parse(raw)["id"].(string)
	if st != 201 || inv == "" {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/invoices/"+inv+"/payments", token, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("cash: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/invoices/"+inv+"/receipt", token, "", nil)
	r := parse(raw)
	if pays, _ := r["payments"].([]any); st != 200 || r["roomCode"] != "R2" || len(pays) != 1 {
		t.Fatalf("receipt: %d %s", st, raw)
	}
	if st, _ = e.send("GET", "/v1/invoices/nope/receipt", token, "", nil); st != 404 {
		t.Fatalf("unknown invoice: %d", st)
	}
}
