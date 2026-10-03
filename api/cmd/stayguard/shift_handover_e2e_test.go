//go:build integration

package main

import "testing"

// CA-08: the review of a closed shift says who closed it (name and role) and the float left in the drawer. There is no "handed to":
// the system has no recipient.
func TestShiftReview_SaysWhoClosedAndTheFloatLeft_FU(t *testing.T) {
	r := newRefundRig(t) // the desk took a deposit, so it has an open shift
	st, raw := r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	cur := parse(raw)
	if st != 200 {
		t.Fatalf("shift: %d %s", st, raw)
	}
	body := map[string]any{"counts": []map[string]any{{"denomination": 500000, "quantity": 2}}, "floatLeft": 150_000, "reason": "refund still open"}
	st, raw = r.e.send("POST", "/v1/shifts/current/close", r.desk, newKey(), body)
	closed := parse(raw)
	if st != 200 || num(closed["floatLeft"]) != 150_000 || closed["closedByName"] != cur["userName"] || closed["closedByRole"] != "RECEPTIONIST" {
		t.Fatalf("close answer: %d %s", st, raw)
	}
	id := cur["id"].(string)
	st, raw = r.e.send("GET", "/v1/owner/shifts/"+id, r.own, "", nil)
	review := parse(raw)
	if st != 200 || review["closedByName"] != cur["userName"] || review["closedByRole"] != "RECEPTIONIST" || num(review["floatLeft"]) != 150_000 {
		t.Fatalf("owner review: %d %s", st, raw)
	}
	if _, has := review["handedTo"]; has {
		t.Fatalf("the system has no recipient: %s", raw)
	}
}
