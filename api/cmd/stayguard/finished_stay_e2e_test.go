//go:build integration

package main

import (
	"testing"
	"time"
)

// Item 7 probe: the stay's real deposit shows in every view of a checked-out stay.
func TestCheckedOutStay_ShowsTheRealDeposit_FU(t *testing.T) {
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 100_000}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(70 * time.Minute))
	key := newKey()
	st, raw = e.checkout(desk, stay, key)
	first := parse(raw)
	if st != 201 || num(first["quote"].(map[string]any)["depositPaid"]) != 100_000 {
		t.Fatalf("checkout: %d %s", st, raw)
	}
	st, raw = e.checkout(desk, stay, key) // the same key: the same answer
	if st != 201 || num(parse(raw)["quote"].(map[string]any)["depositPaid"]) != 100_000 {
		t.Fatalf("replay: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(5 * time.Hour))
	st, raw = e.send("GET", "/v1/stays/"+stay, desk, "", nil)
	got := parse(raw)
	if q, _ := got["quote"].(map[string]any); st != 200 || num(got["deposit"]) != 100_000 || num(q["depositPaid"]) != 100_000 {
		t.Fatalf("getStay of a checked-out stay: %d %s", st, raw)
	}
}

type finishedStay struct {
	name, stay, invoice, bill, state string
	paid                             bool
}

// finishedStays checks out three stays in one tenant: paid, awaiting payment, and a deposit refund still pending.
func finishedStays(t *testing.T) (*env, string, string, []finishedStay) {
	t.Helper()
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	var out []finishedStay
	for _, c := range []struct {
		name, room, state string
		body              map[string]any
		pay               bool
	}{
		{"paid", "A102", "PAID", stayBody(nil), true},
		{"awaiting payment", "A106", "AWAITING_PAYMENT", stayBody(nil), false},
		{"refund pending", "A105", "REFUND_PENDING", stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 1_000_000}), false},
	} {
		e.clock.set(e.start)
		st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, c.room)+"/stays", desk, newKey(), c.body)
		stay, _ := parse(raw)["id"].(string)
		if st != 201 {
			t.Fatalf("%s check-in: %d %s", c.name, st, raw)
		}
		e.clock.set(e.start.Add(40 * time.Minute))
		st, raw = e.checkout(desk, stay, newKey())
		inv := parse(raw)
		if st != 201 {
			t.Fatalf("%s checkout: %d %s", c.name, st, raw)
		}
		if c.pay {
			if st, raw = e.send("POST", "/v1/invoices/"+inv["id"].(string)+"/payments", desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
				t.Fatalf("%s payment: %d %s", c.name, st, raw)
			}
		}
		out = append(out, finishedStay{name: c.name, stay: stay, invoice: inv["id"].(string), bill: inv["billCode"].(string), state: c.state, paid: c.pay})
	}
	e.clock.set(e.start.Add(3 * time.Hour))
	return e, tenant, desk, out
}

// Item 1: a finished stay (paid or not) is read-only. Check-out again, extras, check-in time and move return 409 STAY_NOT_ACTIVE and
// change nothing; getStay returns the frozen invoice and the settled state with paidAt.
func TestFinishedStay_IsReadOnly_FU(t *testing.T) {
	e, tenant, desk, stays := finishedStays(t)
	var service string
	if err := e.owner.QueryRow(t.Context(), `SELECT code FROM app.services WHERE tenant_id = $1 ORDER BY code LIMIT 1`, tenant).Scan(&service); err != nil {
		t.Fatal(err)
	}
	vacant := e.roomIDByCode(tenant, "A101")
	for _, s := range stays {
		invoices := e.count(`SELECT count(*) FROM app.invoices WHERE tenant_id = $1`, tenant)
		extras := e.count(`SELECT count(*) FROM app.stay_extras WHERE tenant_id = $1`, tenant)
		var checkIn time.Time
		var unit string
		_ = e.owner.QueryRow(t.Context(), `SELECT check_in_at, unit_id FROM app.stays WHERE id = $1`, s.stay).Scan(&checkIn, &unit)
		for name, call := range map[string]func() (int, []byte){
			"checkout": func() (int, []byte) { return e.checkout(desk, s.stay, newKey()) },
			"extras": func() (int, []byte) {
				return e.send("POST", "/v1/stays/"+s.stay+"/extras", desk, newKey(), map[string]any{"items": []map[string]any{{"serviceCode": service, "quantity": 1}}})
			},
			"check-in time": func() (int, []byte) {
				return e.send("POST", "/v1/stays/"+s.stay+"/check-in-time", desk, newKey(), map[string]any{"newCheckInAt": e.start.Add(10 * time.Minute).Format(time.RFC3339), "reasonCode": "LATE_ARRIVAL", "note": "x"})
			},
			"move": func() (int, []byte) {
				return e.send("POST", "/v1/stays/"+s.stay+"/move", desk, newKey(), map[string]any{"toRoomId": vacant, "rentalType": "HOURLY"})
			},
		} {
			if st, raw := call(); st != 409 || parse(raw)["code"] != "STAY_NOT_ACTIVE" {
				t.Errorf("%s / %s: %d %s, want 409 STAY_NOT_ACTIVE", s.name, name, st, raw)
			}
		}
		var checkIn2 time.Time
		var unit2 string
		_ = e.owner.QueryRow(t.Context(), `SELECT check_in_at, unit_id FROM app.stays WHERE id = $1`, s.stay).Scan(&checkIn2, &unit2)
		if e.count(`SELECT count(*) FROM app.invoices WHERE tenant_id = $1`, tenant) != invoices || e.count(`SELECT count(*) FROM app.stay_extras WHERE tenant_id = $1`, tenant) != extras ||
			!checkIn.Equal(checkIn2) || unit != unit2 {
			t.Errorf("%s: a refused change changed something", s.name)
		}
		st, raw := e.send("GET", "/v1/stays/"+s.stay, desk, "", nil)
		got := parse(raw)
		inv, _ := got["invoice"].(map[string]any)
		if st != 200 || got["status"] != "CHECKED_OUT" || inv == nil || inv["id"] != s.invoice || inv["billCode"] != s.bill || got["paymentState"] != s.state ||
			(s.paid != (got["paidAt"] != nil)) {
			t.Errorf("%s: getStay %d %s (want invoice %s, state %s, paidAt set: %v)", s.name, st, raw, s.invoice, s.state, s.paid)
		}
	}
}

// Item 7: a stay with a 100,000 deposit whose open invoice was frozen showing no deposit (what Khai's stack showed: "Đã cọc −0đ,
// Phải thu 80.000đ") reads with its real deposit everywhere: getStay, the room map and the payment created from it.
func TestOpenInvoice_ShowsTheStaysRealDeposit_FU(t *testing.T) {
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 100_000}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(40 * time.Minute))
	st, raw = e.checkout(desk, stay, newKey())
	inv := parse(raw)
	total := num(inv["quote"].(map[string]any)["total"])
	if st != 201 || total != 80_000 || num(inv["quote"].(map[string]any)["refundDue"]) != 20_000 {
		t.Fatalf("checkout (80,000 bill, 100,000 deposit): %d %s", st, raw)
	}
	// The stack's state: the frozen quote lost the deposit.
	e.exec(`UPDATE app.invoices SET quote = quote || jsonb_build_object('depositPaid', 0, 'balanceDue', total, 'refundDue', 0) WHERE stay_id = $1`, stay)
	st, raw = e.send("GET", "/v1/stays/"+stay, desk, "", nil)
	got := parse(raw)
	q, _ := got["quote"].(map[string]any)
	pp, _ := got["pendingPayment"].(map[string]any)
	if st != 200 || num(q["depositPaid"]) != 100_000 || num(q["balanceDue"]) != 0 || num(q["refundDue"]) != 20_000 || got["paymentState"] != "REFUND_PENDING" ||
		pp == nil || num(pp["deposit"]) != 100_000 || num(pp["refundDue"]) != 20_000 || num(pp["remaining"]) != 0 {
		t.Fatalf("getStay of the stack's state: %d %s", st, raw)
	}
	r := payRig{e: e, token: desk, tenant: tenant, room: "A102"}
	if tile, _ := r.roomMapStay()["pendingPayment"].(map[string]any); tile == nil || num(tile["deposit"]) != 100_000 || num(tile["refundDue"]) != 20_000 {
		t.Fatalf("room map: %v", tile)
	}
	// A transfer is refused (nothing to collect) and the cash payment is the refund: it must not ask the guest for 80,000.
	if st, raw = e.send("POST", "/v1/invoices/"+inv["id"].(string)+"/payments", desk, newKey(), map[string]any{"method": "TRANSFER"}); st != 422 {
		t.Fatalf("a transfer for a refund: %d %s", st, raw)
	}
	st, raw = e.send("POST", "/v1/invoices/"+inv["id"].(string)+"/payments", desk, newKey(), map[string]any{"method": "CASH"})
	if p := parse(raw); st != 201 || num(p["amount"]) != 0 {
		t.Fatalf("the cash payment is the refund, amount 0: %d %s", st, raw)
	}
}
