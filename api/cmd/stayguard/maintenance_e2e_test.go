//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// SG-1201 over HTTP: a receptionist reports damage and locks a room, a guest in the room blocks the lock, the owner prices and
// closes the ticket, which opens the room again, and a receptionist with EDIT can mark a room clean.
func TestMaintenanceE2E_SG1201(t *testing.T) {
	e := newEnv(t)
	owner := e.demo("OWNER", "vi", "")
	tenant, boss := owner.str("tenantId"), owner.str("accessToken")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.seedStayTenant(tenant, 3)
	e.exec(`INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
		SELECT tenant_id, id, $2, 'EDIT' FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, tenant, stayBuilding)
	e.clock.set(e.start)
	ctx := context.Background()
	roomStatus := func(n int) string {
		var s string
		if err := e.owner.QueryRow(ctx, `SELECT status FROM app.units WHERE id = $1`, roomID(n)).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	report := map[string]any{"category": "AIR_CONDITIONER", "description": "does not cool", "severity": "LOCK_ROOM"}

	st, raw := e.send("POST", "/v1/rooms/"+roomID(1)+"/damage-reports", desk, newKey(), report)
	ticket := parse(raw)
	if st != 201 || ticket["code"] != "BT-001" || ticket["roomLocked"] != true || ticket["status"] != "NEW" || roomStatus(1) != "MAINTENANCE" {
		t.Fatalf("report: %d %s room=%s", st, raw, roomStatus(1))
	}
	id, _ := ticket["id"].(string)

	if st, _ := e.checkIn(desk, 2, newKey(), stayBody(nil)); st != 201 {
		t.Fatalf("check-in: %d", st)
	}
	if st, raw = e.send("POST", "/v1/rooms/"+roomID(2)+"/damage-reports", desk, newKey(), report); st != 409 || parse(raw)["code"] != "ROOM_OCCUPIED" {
		t.Fatalf("lock with a guest: %d %s", st, raw)
	}
	if st, _ = e.send("POST", "/v1/rooms/"+roomID(2)+"/damage-reports", desk, newKey(), map[string]any{"category": "TV", "description": "no remote", "severity": "STILL_RENTABLE"}); st != 201 {
		t.Fatalf("report on an occupied room: %d", st)
	}
	if st, _ = e.send("POST", "/v1/rooms/"+roomID(3)+"/usage-reports", desk, newKey(), map[string]any{"note": "bed looks slept in"}); st != 201 {
		t.Fatalf("usage report: %d", st)
	}

	// Only the owner and manager read tickets; the overview points at the open ones.
	if st, _ = e.send("GET", "/v1/owner/maintenance-tickets", desk, "", nil); st != 403 {
		t.Fatalf("receptionist lists tickets: %d", st)
	}
	st, raw = e.send("GET", "/v1/owner/maintenance-tickets?status=NEW", boss, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 2 {
		t.Fatalf("tickets: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/owner/overview", boss, "", nil)
	kinds := ""
	for _, a := range parse(raw)["attention"].([]any) {
		kinds += a.(map[string]any)["kind"].(string) + " "
	}
	if st != 200 || !strings.Contains(kinds, "TICKET_OPEN") {
		t.Fatalf("overview attention: %d %s", st, kinds)
	}

	// Costs are the owner's; DONE posts completion and opens the room again.
	costs := map[string]any{"partsCost": 300000, "labourCost": 200000}
	if st, _ = e.send("PATCH", "/v1/owner/maintenance-tickets/"+id, desk, "", costs); st != 403 {
		t.Fatalf("receptionist patches: %d", st)
	}
	if st, raw = e.send("PATCH", "/v1/owner/maintenance-tickets/"+id, boss, "", costs); st != 200 || parse(raw)["totalCost"] != float64(500000) {
		t.Fatalf("costs: %d %s", st, raw)
	}
	if st, raw = e.send("PATCH", "/v1/owner/maintenance-tickets/"+id, boss, "", map[string]any{"status": "DONE"}); st != 200 || parse(raw)["completedAt"] == nil || parse(raw)["roomLocked"] != false {
		t.Fatalf("done: %d %s", st, raw)
	}
	if roomStatus(1) != "VACANT" {
		t.Fatalf("room after DONE: %s", roomStatus(1))
	}
	if st, raw = e.send("PATCH", "/v1/owner/maintenance-tickets/"+id, boss, "", map[string]any{"status": "IN_REPAIR"}); st != 409 || parse(raw)["code"] != "TICKET_DONE" {
		t.Fatalf("reopen a done ticket: %d %s", st, raw)
	}

	// A receptionist with EDIT cleans a room after the guest paid.
	e.clock.set(e.start.Add(2 * time.Hour))
	var stayID string
	_ = e.owner.QueryRow(ctx, `SELECT id FROM app.stays WHERE unit_id = $1`, roomID(2)).Scan(&stayID)
	st, raw = e.checkout(desk, stayID, newKey())
	inv, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	if st, _ = e.send("POST", "/v1/invoices/"+inv+"/payments", desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("cash: %d", st)
	}
	if st, raw = e.send("POST", "/v1/housekeeping/tasks/"+roomID(2)+"/complete", desk, newKey(), nil); st != 200 || roomStatus(2) != "VACANT" {
		t.Fatalf("receptionist cleans: %d %s room=%s", st, raw, roomStatus(2))
	}
}
