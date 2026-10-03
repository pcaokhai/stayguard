//go:build integration

package main

import "testing"

// LO-10: the server applies the web's phone rule on createStay: 9 to 11 digits, optional leading 0 or +84, spaces, dots and dashes
// stripped.
func TestCreateStay_GuestPhoneRule_FU(t *testing.T) {
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	room := e.roomIDByCode(tenant, "A102")
	before := e.count(`SELECT count(*) FROM app.stays WHERE tenant_id = $1`, tenant)
	st, raw := e.send("POST", "/v1/rooms/"+room+"/stays", desk, newKey(), stayBody(map[string]any{"guestPhone": "09012345"}))
	if errs, _ := parse(raw)["errors"].([]any); st != 422 || len(errs) != 1 || errs[0].(map[string]any)["field"] != "guestPhone" {
		t.Fatalf("an 8-digit phone: %d %s", st, raw)
	}
	if n := e.count(`SELECT count(*) FROM app.stays WHERE tenant_id = $1`, tenant); n != before {
		t.Fatalf("a refused check-in stored a stay: %d, was %d", n, before)
	}
	st, raw = e.send("POST", "/v1/rooms/"+room+"/stays", desk, newKey(), stayBody(map[string]any{"guestPhone": "+84 90-123.4567"}))
	if st != 201 || parse(raw)["guestPhone"] != "+84901234567" {
		t.Fatalf("a valid phone with separators is stored stripped: %d %s", st, raw)
	}
}
