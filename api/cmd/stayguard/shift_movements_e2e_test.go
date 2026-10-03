//go:build integration

package main

import (
	"testing"
	"time"
)

func movementsSum(t *testing.T, sh map[string]any) (sum int64, kinds map[string]int) {
	t.Helper()
	kinds = map[string]int{}
	ms, ok := sh["movements"].([]any)
	if !ok {
		t.Fatalf("no movements in %v", sh)
	}
	for _, m := range ms {
		mv := m.(map[string]any)
		sum += num(mv["amount"])
		kinds[mv["kind"].(string)]++
	}
	return
}

// The shift's movements come from the same ledger as its expected cash: their signed sum is the expected cash, with the opening float,
// deposits, payments, refunds (also an owner's refund on the receptionist's open shift) and payouts.
func TestShiftMovements_SumToExpectedCash_FU(t *testing.T) {
	r := newRefundRig(t) // desk deposit 1,000,000 on the first shift, refund open
	// The first shift closes leaving a 100,000 float; the next shift opens with it.
	body := map[string]any{"counts": []map[string]any{{"denomination": 500000, "quantity": 2}}, "floatLeft": 100_000, "reason": "refund still open"}
	if st, raw := r.e.send("POST", "/v1/shifts/current/close", r.desk, newKey(), body); st != 200 {
		t.Fatalf("close: %d %s", st, raw)
	}
	r.e.clock.set(r.outAt.Add(30 * time.Minute))
	// Second shift: a deposit by the desk, and the owner's refund of the first stay lands on this open shift.
	st, raw := r.e.send("POST", "/v1/rooms/"+r.e.roomIDByCode(r.tenant, "A106")+"/stays", r.desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 50_000}))
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	if st, raw = r.e.send("POST", "/v1/shifts/current/payouts", r.desk, newKey(), map[string]any{"amount": 10_000, "description": "ice"}); st != 200 {
		t.Fatalf("payout: %d %s", st, raw)
	}
	if st, raw = r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.own, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("owner refund: %d %s", st, raw)
	}
	st, raw = r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	sh := parse(raw)
	sum, kinds := movementsSum(t, sh)
	if st != 200 || sum != num(sh["expectedCash"]) || kinds["OPENING_FLOAT"] != 1 || kinds["DEPOSIT"] != 1 || kinds["REFUND"] != 1 || kinds["PAYOUT"] != 1 {
		t.Fatalf("movements %v sum %d, expected cash %d: %s", kinds, sum, num(sh["expectedCash"]), raw)
	}
	var refund map[string]any
	for _, m := range sh["movements"].([]any) {
		if mv := m.(map[string]any); mv["kind"] == "REFUND" {
			refund = mv
		}
	}
	if num(refund["amount"]) != -r.refund || refund["byOwner"] != true || refund["billCode"] != r.code || refund["roomCode"] != "A102" || refund["at"] == nil {
		t.Fatalf("the owner's refund movement: %v", refund)
	}
	// The owner's review of the closed shift reads the same ledger.
	if st, raw = r.e.send("POST", "/v1/shifts/current/close", r.desk, newKey(), map[string]any{"counts": []map[string]any{{"denomination": 100000, "quantity": 1}}, "floatLeft": 0, "reason": "test"}); st != 200 {
		t.Fatalf("close: %d %s", st, raw)
	}
	review := parse(raw)
	rs, _ := review["shift"].(map[string]any)
	if sum, _ = movementsSum(t, rs); sum != num(rs["expectedCash"]) {
		t.Fatalf("review movements sum %d, expected cash %d", sum, num(rs["expectedCash"]))
	}
}
