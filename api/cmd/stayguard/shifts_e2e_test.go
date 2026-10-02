//go:build integration

package main

import (
	"fmt"
	"testing"
	"time"
)

// The cash path of a receptionist over HTTP: the first cash action opens the shift, deposit, balance, refund-free
// check-out and payout feed expected cash, closing locks it, and the database refuses to change what was closed.
func TestShiftsE2E_SG503(t *testing.T) {
	e := newEnv(t)
	owner := e.demo("OWNER", "vi", "")
	tenant, boss := owner.str("tenantId"), owner.str("accessToken")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.exec(`INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
		SELECT tenant_id, id, $2, 'EDIT' FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, tenant, stayBuilding)
	e.clock.set(e.start)

	if st, raw := e.send("GET", "/v1/shifts/current", desk, "", nil); st != 404 {
		t.Fatalf("before any cash action: %d %s", st, raw)
	}
	st, raw := e.checkIn(desk, 1, newKey(), stayBody(map[string]any{"deposit": 100000}))
	stayID, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/shifts/current", desk, "", nil)
	if sh := parse(raw); st != 200 || sh["expectedCash"] != float64(100000) || sh["cashIn"] != float64(100000) || sh["status"] != "OPEN" {
		t.Fatalf("shift after the deposit: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(2 * time.Hour))
	st, raw = e.checkout(desk, stayID, newKey())
	inv := parse(raw)
	invoiceID, _ := inv["id"].(string)
	balance := int64(inv["quote"].(map[string]any)["balanceDue"].(float64))
	if st != 201 || balance <= 0 {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/invoices/"+invoiceID+"/payments", desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("cash payment: %d %s", st, raw)
	}
	payout := map[string]any{"amount": 20000, "description": "ice"}
	if st, raw = e.send("POST", "/v1/shifts/current/payouts", desk, newKey(), payout); st != 200 {
		t.Fatalf("payout: %d %s", st, raw)
	}
	expected := 100000 + balance - 20000
	st, raw = e.send("GET", "/v1/shifts/current", desk, "", nil)
	if sh := parse(raw); st != 200 || int64(sh["expectedCash"].(float64)) != expected || sh["cashOut"] != float64(20000) {
		t.Fatalf("expected cash %d: %d %s", expected, st, raw)
	}
	// the owner has no drawer
	if st, _ = e.send("GET", "/v1/shifts/current", boss, "", nil); st != 404 {
		t.Fatalf("owner shift: %d", st)
	}

	// A count that does not match needs a reason (422); with it the shift closes and an alert is raised.
	short := map[string]any{"counts": []map[string]any{{"denomination": 10000, "quantity": 1}}, "floatLeft": 0}
	if st, raw = e.send("POST", "/v1/shifts/current/close", desk, newKey(), short); st != 422 {
		t.Fatalf("close without a reason: %d %s", st, raw)
	}
	short["reason"] = "counted in a hurry"
	st, raw = e.send("POST", "/v1/shifts/current/close", desk, newKey(), short)
	review := parse(raw)
	shiftID, _ := review["shift"].(map[string]any)["id"].(string)
	if st != 200 || int64(review["difference"].(float64)) != 10000-expected || shiftID == "" {
		t.Fatalf("close: %d %s", st, raw)
	}
	var alerts int
	if err := e.owner.QueryRow(t.Context(), `SELECT count(*) FROM app.alerts WHERE tenant_id = $1 AND kind = 'CASH_SHORT' AND shift_id = $2`, tenant, shiftID).Scan(&alerts); err != nil || alerts != 1 {
		t.Fatalf("CASH_SHORT alerts: %d %v", alerts, err)
	}
	if st, raw = e.send("GET", "/v1/owner/shifts/"+shiftID, boss, "", nil); st != 200 || len(parse(raw)["cashPayments"].([]any)) != 2 {
		t.Fatalf("review: %d %s", st, raw)
	}
	if st, _ = e.send("GET", "/v1/owner/shifts/"+shiftID, desk, "", nil); st != 403 {
		t.Fatalf("receptionist review: %d", st)
	}
	st, raw = e.send("GET", fmt.Sprintf("/v1/owner/shifts?month=%s&onlyDifferences=true", e.start.Format("2006-01")), boss, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 1 {
		t.Fatalf("closed shifts: %d %s", st, raw)
	}

	// The database itself refuses to change a closed shift or add to its ledger.
	if _, err := e.owner.Exec(t.Context(), `UPDATE app.shifts SET difference = 0 WHERE id = $1`, shiftID); err == nil {
		t.Fatal("a closed shift was updated")
	}
	if _, err := e.owner.Exec(t.Context(), `INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, created_at) VALUES ('ce_late', $1, $2, 'PAYOUT', 1, now())`, tenant, shiftID); err == nil {
		t.Fatal("an entry was added to a closed shift")
	}
}
