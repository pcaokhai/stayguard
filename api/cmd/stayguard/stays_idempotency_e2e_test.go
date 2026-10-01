//go:build integration

package main

import "testing"

func TestIdempotencyE2E_SG203_AC3(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	key := newKey()

	st1, first := e.checkIn(token, 1, key, stayBody(nil))
	st2, second := e.checkIn(token, 1, key, stayBody(nil))
	if st1 != 201 || st2 != 201 || string(first) != string(second) {
		t.Fatalf("replay: %d %d\n%s\n%s", st1, st2, first, second)
	}
	if n := e.count(`SELECT count(*) FROM app.stays`); n != 1 {
		t.Errorf("%d stay rows after a replay", n)
	}

	st, raw := e.checkIn(token, 1, key, stayBody(map[string]any{"deposit": 200000}))
	if p := parse(raw); st != 409 || p["code"] != "IDEMPOTENCY_KEY_REUSED" {
		t.Errorf("different body: %d %s", st, raw)
	}

	// A failed attempt (room already occupied) leaves no key row: the transaction rolled back.
	failKey := newKey()
	st, raw = e.checkIn(token, 1, failKey, stayBody(map[string]any{"guestName": "Another"}))
	if p := parse(raw); st != 409 || p["code"] != "ROOM_NOT_VACANT" {
		t.Errorf("occupied room: %d %s", st, raw)
	}
	if n := e.count(`SELECT count(*) FROM app.idempotency_keys WHERE key = $1`, failKey); n != 0 {
		t.Errorf("failed attempt left %d key rows", n)
	}
	if n := e.count(`SELECT count(*) FROM app.idempotency_keys`); n != 1 {
		t.Errorf("%d key rows, want 1", n)
	}
}
