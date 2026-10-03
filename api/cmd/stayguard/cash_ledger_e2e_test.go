//go:build integration

package main

import (
	"fmt"
	"testing"
	"time"
)

func (r refundRig) overviewCash() int64 {
	day := r.e.clock.Now().In(time.UTC).Format("2006-01-02")
	st, raw := r.e.send("GET", "/v1/owner/overview?date="+day, r.own, "", nil)
	if st != 200 {
		r.e.t.Fatalf("overview: %d %s", st, raw)
	}
	return num(parse(raw)["cashExpected"])
}

func (r refundRig) shiftExpected() int64 {
	st, raw := r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	if st != 200 {
		r.e.t.Fatalf("shift: %d %s", st, raw)
	}
	return num(parse(raw)["expectedCash"])
}

// Item 4, reconstructed: the receptionist took a 100,000 deposit (it is in her drawer), the bill is 80,000, and the OWNER recorded the
// 20,000 cash refund. The refund had no ledger line, so the shift screen kept expecting the deposit it never gave back (too high by the
// refund), while the owner overview counted what the bill really was. Both figures now come from one ledger: a cash movement by the
// owner while a receptionist shift is open in that building is on that shift, marked "by owner".
func TestCashDrawer_OwnerRefundWhileAShiftIsOpen_ScreensAgree_FU(t *testing.T) {
	r := newRefundRig(t) // deposit bigDeposit (1,000,000), bill 80,000, refund 920,000: left open
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.own, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("the owner's refund: %d %s", st, raw)
	}
	want := int64(bigDeposit) - r.refund // the drawer holds the bill: deposit minus what went back out
	if got := r.shiftExpected(); got != want {
		t.Fatalf("the shift screen expects %d, want %d (deposit %d, owner's refund %d)", got, want, bigDeposit, r.refund)
	}
	if got := r.overviewCash(); got != want {
		t.Fatalf("the owner overview expects %d, the shift screen %d", got, want)
	}
	st, raw := r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	if sh := parse(raw); st != 200 || num(sh["cashOut"]) != r.refund {
		t.Fatalf("the refund is on the open shift: %d %s", st, raw)
	}
	if n := r.e.count(`SELECT count(*) FROM app.cash_entries WHERE tenant_id = $1 AND kind = 'REFUND' AND by_owner AND shift_id IS NOT NULL`, r.tenant); n != 1 {
		t.Fatalf("the refund is marked by owner on the shift: %d", n)
	}
}

// With no receptionist shift open, owner cash is owner cash: a ledger line with no shift, in the overview, on no shift screen.
func TestCashDrawer_OwnerCashWithNoShiftIsOwnerCash_FU(t *testing.T) {
	r := newRefundRig(t)
	// The receptionist closes her shift (1,000,000 counted: her deposit), then the owner gives the refund back.
	body := map[string]any{"counts": []map[string]any{{"denomination": 500000, "quantity": 2}}, "floatLeft": 0, "reason": "refund left for the owner"}
	if st, raw := r.e.send("POST", "/v1/shifts/current/close", r.desk, newKey(), body); st != 200 {
		t.Fatalf("close: %d %s", st, raw)
	}
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.own, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("the owner's refund: %d %s", st, raw)
	}
	if n := r.e.count(`SELECT count(*) FROM app.cash_entries WHERE tenant_id = $1 AND kind = 'REFUND' AND by_owner AND shift_id IS NULL`, r.tenant); n != 1 {
		t.Fatalf("the refund is owner cash, no shift: %d", n)
	}
	// Overview: the closed shift's drawer (deposit counted) and the owner's refund out of his own pocket: net deposit minus refund.
	if got, want := r.overviewCash(), int64(bigDeposit)-r.refund; got != want {
		t.Fatalf("overview %d, want %d", got, want)
	}
}

// The screens always match: after any mix of desk and owner cash movements with a shift open, the one expected-cash figure is the same
// on the shift screen and the owner overview.
func TestCashDrawer_ScreensAlwaysMatch_FU(t *testing.T) {
	r := newRefundRig(t)
	check := func(step string) {
		t.Helper()
		if a, b := r.shiftExpected(), r.overviewCash(); a != b {
			t.Fatalf("%s: shift screen %d, owner overview %d", step, a, b)
		}
	}
	check("after the deposit and the open refund")
	// A second stay by the owner (deposit by owner while the desk shift is open), paid in cash by the desk.
	r.e.clock.set(r.outAt.Add(10 * time.Minute))
	st, raw := r.e.send("POST", "/v1/rooms/"+r.e.roomIDByCode(r.tenant, "A106")+"/stays", r.own, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 50_000}))
	stay2, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("owner check-in: %d %s", st, raw)
	}
	check("owner deposit on an open shift")
	r.e.clock.set(r.outAt.Add(80 * time.Minute))
	st, raw = r.e.checkout(r.desk, stay2, newKey())
	inv2, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("checkout: %d %s", st, raw)
	}
	if st, raw = r.e.send("POST", "/v1/invoices/"+inv2+"/payments", r.desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("cash: %d %s", st, raw)
	}
	check("desk cash payment")
	if st, raw = r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.own, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("owner refund: %d %s", st, raw)
	}
	check("owner refund")
	if st, raw = r.e.send("POST", "/v1/shifts/current/payouts", r.desk, newKey(), map[string]any{"amount": 10_000, "description": fmt.Sprint("ice")}); st != 200 {
		t.Fatalf("payout: %d %s", st, raw)
	}
	check("payout")
}

// QA SH-08: shift 1 closes leaving a float, shift 2 opens with it. The float is the same cash, so the owner overview counts it once:
// a new 10,000 deposit moves the day's cash by 10,000, not by the 10,000 plus the float.
func TestCashDrawer_AFloatLeftInTheDrawerIsCountedOnce_FU(t *testing.T) {
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	own := e.demo("OWNER", "vi", tenant).str("accessToken")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	r := refundRig{e: e, tenant: tenant, desk: desk, own: own}
	e.clock.set(e.start)
	if st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 50_000})); st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	// Shift 1 expects 50,000; the desk counts 40,000 (short, with a reason) and leaves 20,000 in the drawer.
	body := map[string]any{"counts": []map[string]any{{"denomination": 20000, "quantity": 2}}, "floatLeft": 20_000, "reason": "a 10,000 note went as change"}
	if st, raw := e.send("POST", "/v1/shifts/current/close", desk, newKey(), body); st != 200 {
		t.Fatalf("close: %d %s", st, raw)
	}
	before := r.overviewCash()
	e.clock.set(e.start.Add(20 * time.Minute))
	if st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A106")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 10_000})); st != 201 {
		t.Fatalf("second check-in: %d %s", st, raw)
	}
	st, raw := e.send("GET", "/v1/shifts/current", desk, "", nil)
	if sh := parse(raw); st != 200 || num(sh["openingFloat"]) != 20_000 {
		t.Fatalf("shift 2 opens with the float: %d %s", st, raw)
	}
	if after := r.overviewCash(); after-before != 10_000 {
		t.Fatalf("the overview moved by %d (from %d to %d), want only the new 10,000 deposit", after-before, before, after)
	}
}
