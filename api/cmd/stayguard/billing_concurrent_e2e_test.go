//go:build integration

package main

import (
	"sync/atomic"
	"testing"
)

const (
	stockRacers = 12
	stockAmount = 5
	billRounds  = 3
)

func TestAddExtrasConcurrentStock_SG205_AC2(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, billRounds)
	e.seedServices(tenant, stockAmount)

	for round := 1; round <= billRounds; round++ {
		e.exec(`UPDATE app.services SET stock = $2 WHERE tenant_id = $1`, tenant, stockAmount)
		id := e.openStay(token, round, nil)
		var ok, short atomic.Int64
		race(stockRacers, func(int) {
			st, raw := e.addExtras(token, id, newKey(), extrasBody("WATER", 1))
			switch {
			case st == 200:
				ok.Add(1)
			case st == 409 && problemCode(raw) == "INSUFFICIENT_STOCK":
				short.Add(1)
			default:
				t.Errorf("round %d: unexpected %s", round, describe(st, raw))
			}
		})
		if ok.Load() != stockAmount || short.Load() != stockRacers-stockAmount {
			t.Errorf("round %d: %d ok, %d short", round, ok.Load(), short.Load())
		}
		if n := e.stock(tenant, "WATER"); n != 0 {
			t.Errorf("round %d: stock %d, want 0", round, n)
		}
		if n := e.count(`SELECT count(*) FROM app.stay_extras WHERE stay_id = $1`, id); n != stockAmount {
			t.Errorf("round %d: %d extras rows, want %d", round, n, stockAmount)
		}
	}
}

// A request whose second line fails leaves the first line's stock untouched (the whole unit rolls back).
func TestAddExtrasRollback_SG205_AC2(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.seedServices(tenant, stockAmount)
	id := e.openStay(token, 1, nil)

	// Lines run in code order: BEER first (fits), then WATER (more than the stock).
	st, raw := e.addExtras(token, id, newKey(), extrasBody("WATER", stockAmount+1, "BEER", 1))
	if st != 409 || problemCode(raw) != "INSUFFICIENT_STOCK" {
		t.Fatalf("second line short: %s", describe(st, raw))
	}
	if e.stock(tenant, "BEER") != stockAmount || e.stock(tenant, "WATER") != stockAmount ||
		e.count(`SELECT count(*) FROM app.stay_extras`) != 0 || e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'stay.extras_added'`) != 0 {
		t.Error("a failed request changed stock, extras or audit rows")
	}
}

func TestCheckoutConcurrent_SG205_AC3(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	first, second := e.openStay(token, 1, nil), e.openStay(token, 2, nil)
	e.clock.set(fixedCheckIn.Add(billDay))

	// Distinct keys: every racer gets the same invoice; one row; the stay is checked out once.
	bodies := make([]string, 10)
	race(10, func(i int) {
		st, raw := e.checkout(token, first, newKey())
		if st != 201 {
			t.Errorf("racer %d: %s", i, describe(st, raw))
		}
		bodies[i] = string(raw)
	})
	for i, b := range bodies {
		if parse([]byte(b))["id"] != parse([]byte(bodies[0]))["id"] || parse([]byte(b))["billCode"] != parse([]byte(bodies[0]))["billCode"] {
			t.Errorf("racer %d saw another invoice: %s vs %s", i, b, bodies[0])
		}
	}
	if e.count(`SELECT count(*) FROM app.invoices WHERE stay_id = $1`, first) != 1 ||
		e.count(`SELECT count(*) FROM app.stays WHERE id = $1 AND status = 'CHECKED_OUT'`, first) != 1 ||
		e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'stay.checked_out'`) != 1 {
		t.Error("check-out wrote more or fewer rows than one")
	}

	// Same key from many goroutines: byte-identical answers, one invoice.
	key := newKey()
	same := make([]string, 10)
	race(10, func(i int) {
		st, raw := e.checkout(token, second, key)
		if st != 201 {
			t.Errorf("same-key racer %d: %s", i, describe(st, raw))
		}
		same[i] = string(raw)
	})
	for i, b := range same {
		if b != same[0] {
			t.Errorf("same-key racer %d differs: %s vs %s", i, b, same[0])
		}
	}
	if e.count(`SELECT count(*) FROM app.invoices WHERE stay_id = $1`, second) != 1 {
		t.Error("same-key burst made more than one invoice")
	}
}

// Extras racing a check-out: afterwards the frozen quote equals the stay's final extras, and every
// extras request either landed before the invoice (200) or was refused with STAY_NOT_ACTIVE.
func TestExtrasVsCheckoutRace_SG205_AC4(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, billRounds)
	e.seedServices(tenant, 100)

	for round := 1; round <= billRounds; round++ {
		id := e.openStay(token, round, nil)
		e.clock.set(fixedCheckIn.Add(billDay))
		var added atomic.Int64
		race(9, func(i int) {
			if i%3 == 0 {
				if st, raw := e.checkout(token, id, newKey()); st != 201 {
					t.Errorf("round %d checkout: %s", round, describe(st, raw))
				}
				return
			}
			switch st, raw := e.addExtras(token, id, newKey(), extrasBody("WATER", 1)); {
			case st == 200:
				added.Add(1)
			case st == 409 && problemCode(raw) == "STAY_NOT_ACTIVE":
			default:
				t.Errorf("round %d extras: %s", round, describe(st, raw))
			}
		})
		rows := e.count(`SELECT count(*) FROM app.stay_extras WHERE stay_id = $1`, id)
		sum := e.count(`SELECT coalesce(sum(amount), 0) FROM app.stay_extras WHERE stay_id = $1`, id)
		frozen := e.count(`SELECT (quote->>'ExtrasAmount')::int FROM app.invoices WHERE stay_id = $1`, id)
		if int64(rows) != added.Load() || frozen != sum {
			t.Errorf("round %d: %d rows vs %d added, frozen extras %d vs rows %d", round, rows, added.Load(), frozen, sum)
		}
		if n := e.count(`SELECT count(*) FROM app.stay_extras x JOIN app.invoices i ON i.stay_id = x.stay_id WHERE x.stay_id = $1 AND x.created_at > i.created_at`, id); n != 0 {
			t.Errorf("round %d: %d extras created after the invoice", round, n)
		}
	}
}

// Rooms whose bill code candidates overlap (R1 second try is R12) check out at the same instant and both
// end with distinct codes: the unique violation is retried inside the use case, never a 500.
func TestCheckoutBillCodeCollision_SG205_AC3(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 24)
	// Old checked-out stays already hold PH1001R1 and PH1001R2, so R1 and R2 start at their second candidate.
	for _, code := range []string{"R1", "R2"} {
		old := tenant + "_old" + code
		e.exec(`INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, guest_name, guest_phone, check_in_at, check_out_at, rate_plan_snapshot, rate_plan_schema)
			VALUES ($1, $2, $3, 'OVERNIGHT', 'CHECKED_OUT', 'Old', '0900000000', $4, $4, '{}', 1)`, old, tenant, roomID(24), fixedCheckIn)
		e.exec(`INSERT INTO app.invoices (id, tenant_id, stay_id, bill_code, quote, total) VALUES ($1, $2, $3, $4, '{}', 0)`,
			"iv_old"+code, tenant, old, "PH1001"+code)
	}
	pairs := [][2]int{{1, 12}, {2, 22}}
	var ids [][2]string
	for _, p := range pairs {
		ids = append(ids, [2]string{e.openStay(token, p[0], nil), e.openStay(token, p[1], nil)})
	}
	e.clock.set(fixedCheckIn.Add(billDay))
	race(4, func(i int) {
		pair := ids[i/2]
		if st, raw := e.checkout(token, pair[i%2], newKey()); st != 201 {
			t.Errorf("racer %d: %s", i, describe(st, raw))
		}
	})
	if n := e.count(`SELECT count(DISTINCT bill_code) FROM app.invoices WHERE tenant_id = $1`, tenant); n != 6 {
		t.Errorf("%d distinct bill codes, want 6", n)
	}
	if n := e.count(`SELECT count(*) FROM app.invoices WHERE tenant_id = $1 AND created_at IS NOT NULL`, tenant); n != 6 {
		t.Errorf("%d invoices, want 6", n)
	}
	if e.logs.String() != "" && containsLevel(e.logs.String(), "ERROR") {
		t.Errorf("a retried bill code conflict must not log an error: %s", e.logs.String())
	}
}
