//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

const bigDeposit = 1_000_000

// refundRig is a desk (receptionist) and an owner in one seeded tenant; a stay checked in with a deposit far above the bill is
// checked out 40 minutes later and left there: no payment, no refund recorded.
type refundRig struct {
	e                 *env
	tenant, desk, own string
	stay, invoice     string
	code              string
	refund, total     int64
	outAt             time.Time
}

func newRefundRig(t *testing.T) refundRig {
	t.Helper()
	e := newSeededEnv(t)
	own := e.demo("OWNER", "vi", "")
	tenant := own.str("tenantId")
	r := refundRig{e: e, tenant: tenant, own: own.str("accessToken"), desk: e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")}
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", r.desk, newKey(),
		stayBody(map[string]any{"rentalType": "HOURLY", "deposit": bigDeposit}))
	r.stay, _ = parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	r.outAt = e.start.Add(40 * time.Minute)
	e.clock.set(r.outAt)
	st, raw = e.checkout(r.desk, r.stay, newKey())
	inv := parse(raw)
	r.invoice, _ = inv["id"].(string)
	r.code, _ = inv["billCode"].(string)
	q, _ := inv["quote"].(map[string]any)
	r.refund, r.total = num(q["refundDue"]), num(q["total"])
	if st != 201 || r.refund <= 0 || num(q["balanceDue"]) != 0 {
		t.Fatalf("check-out with a refund: %d %s", st, raw)
	}
	return r
}

func (r refundRig) roomTile() map[string]any {
	var building string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT building_id FROM app.units WHERE tenant_id = $1 AND code = 'A102'`, r.tenant).Scan(&building); err != nil {
		r.e.t.Fatal(err)
	}
	st, raw := r.e.send("GET", "/v1/buildings/"+building+"/rooms", r.desk, "", nil)
	if st != 200 {
		r.e.t.Fatalf("listRooms: %d %s", st, raw)
	}
	for _, it := range parse(raw)["items"].([]any) {
		if m := it.(map[string]any); m["code"] == "A102" {
			return m
		}
	}
	r.e.t.Fatal("room A102 missing")
	return nil
}

// Item 1: the stay is only quoted while it is open (no state change, any number of times); the check-out time and the invoice
// are fixed by the call that confirms, on the server clock, and a repeated call changes nothing.
func TestCheckoutTime_FixedOnlyOnConfirmation_FU(t *testing.T) {
	e := newSeededEnv(t)
	own := e.demo("OWNER", "vi", "")
	tenant := own.str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": bigDeposit}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	for _, d := range []time.Duration{30 * time.Minute, 45 * time.Minute} { // the screen opens, twice: only a quote
		e.clock.set(e.start.Add(d))
		st, raw = e.send("GET", "/v1/stays/"+stay, desk, "", nil)
		got := parse(raw)
		if st != 200 || got["status"] != "ACTIVE" || got["checkOutAt"] != nil || got["pendingPayment"] != nil {
			t.Fatalf("a quote must not change the stay: %d %s", st, raw)
		}
	}
	if n := e.count(`SELECT count(*) FROM app.invoices WHERE tenant_id = $1`, tenant); n != 0 {
		t.Fatalf("opening the screen created an invoice: %d", n)
	}
	confirm := e.start.Add(time.Hour)
	e.clock.set(confirm)
	st, raw = e.checkout(desk, stay, newKey())
	inv := parse(raw)
	if st != 201 {
		t.Fatalf("checkout: %d %s", st, raw)
	}
	e.clock.set(confirm.Add(2 * time.Hour))
	st, raw = e.checkout(desk, stay, newKey()) // opened again later: the same frozen invoice
	again := parse(raw)
	if st != 201 || again["id"] != inv["id"] || again["billCode"] != inv["billCode"] {
		t.Fatalf("a second checkout must return the same invoice and bill code: %d %s", st, raw)
	}
	var outAt time.Time
	if err := e.owner.QueryRow(context.Background(), `SELECT check_out_at FROM app.stays WHERE id = $1`, stay).Scan(&outAt); err != nil || !outAt.Equal(confirm) {
		t.Fatalf("check-out time %v, want the confirmation time %v (%v)", outAt, confirm, err)
	}
}

// Item 2: a stay checked out with nothing recorded (an open refund, or a transfer chosen and not paid) stays visible and resumable.
func TestUnsettledCheckout_StaysOnTheMapAndResumes_FU(t *testing.T) {
	r := newRefundRig(t)
	r.e.clock.set(r.outAt.Add(5 * time.Hour)) // the receptionist left; hours go by
	tile := r.roomTile()
	s, _ := tile["activeStay"].(map[string]any)
	pp, _ := s["pendingPayment"].(map[string]any)
	if tile["status"] != "OCCUPIED" || s == nil || s["rentalType"] != "HOURLY" || s["guestName"] == "" || num(s["elapsedMinutes"]) != 40 || pp == nil {
		t.Fatalf("the unsettled stay on the map: %v", tile)
	}
	if pp["paymentId"] != nil || num(pp["total"]) != r.total || num(pp["deposit"]) != bigDeposit || num(pp["received"]) != 0 || num(pp["remaining"]) != 0 ||
		num(pp["refundDue"]) != r.refund || pp["createdAt"] == nil {
		t.Fatalf("pendingPayment of an open refund: %v", pp)
	}
	st, raw := r.e.send("GET", "/v1/stays/"+r.stay, r.desk, "", nil)
	gp, _ := parse(raw)["pendingPayment"].(map[string]any)
	if st != 200 || gp == nil || num(gp["refundDue"]) != r.refund || num(gp["deposit"]) != bigDeposit || gp["createdAt"] != pp["createdAt"] {
		t.Fatalf("getStay pendingPayment: %d %s", st, raw)
	}
	// Resuming: opening checkout again is the same invoice, and the refund can still be confirmed.
	st, raw = r.e.checkout(r.desk, r.stay, newKey())
	if again := parse(raw); st != 201 || again["id"] != r.invoice || again["billCode"] != r.code {
		t.Fatalf("resume: %d %s", st, raw)
	}
	if st, raw = r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("the refund after resuming: %d %s", st, raw)
	}
	if tile = r.roomTile(); tile["status"] != "TO_CLEAN" || tile["activeStay"] != nil {
		t.Fatalf("settled: %v", tile)
	}
}

// Same family: a transfer was chosen and never paid; pendingPayment carries the payment, the deposit and the check-out time.
func TestUnsettledCheckout_TransferChosen_FU(t *testing.T) {
	r := deskRig(t)
	_, p := r.pay("TRANSFER")
	total, created := r.invoiceTotalAndCreated()
	r.e.clock.set(created.Add(4 * time.Hour))
	s := r.roomMapStay()
	pp, _ := s["pendingPayment"].(map[string]any)
	if pp == nil || pp["paymentId"] != p["id"] || num(pp["total"]) != total || num(pp["deposit"]) != total-r.balance || num(pp["received"]) != 0 ||
		num(pp["remaining"]) != r.balance || num(pp["refundDue"]) != 0 || s["rentalType"] == nil {
		t.Fatalf("pendingPayment with a transfer chosen: %v", s)
	}
}

// Item 3: opening checkout again is the same invoice; the cash refund is recorded once on the receptionist's shift, whatever the retries.
func TestRefund_RecordedOnceWhateverTheRetries_FU(t *testing.T) {
	r := newRefundRig(t)
	key := newKey()
	var first []byte
	for i := 0; i < 3; i++ {
		st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.desk, key, map[string]any{"method": "CASH"})
		if st != 201 || (i > 0 && string(raw) != string(first)) {
			t.Fatalf("retry %d with the same key: %d %s", i, st, raw)
		}
		first = raw
	}
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.desk, newKey(), map[string]any{"method": "CASH"}); st != 409 {
		t.Fatalf("a new key on a paid invoice: %d %s", st, raw)
	}
	if n := r.e.count(`SELECT count(*) FROM app.cash_entries WHERE tenant_id = $1 AND kind = 'REFUND'`, r.tenant); n != 1 {
		t.Fatalf("refund entries: %d, want 1", n)
	}
	st, raw := r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	sh := parse(raw)
	if st != 200 || num(sh["cashOut"]) != r.refund || num(sh["expectedCash"]) != bigDeposit-r.refund {
		t.Fatalf("shift after the refund: %d %s", st, raw)
	}
}

func TestRefund_OwnerRecordsItOnNoShiftOnce_FU(t *testing.T) {
	r := newRefundRig(t)
	for i := 0; i < 2; i++ {
		st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.own, newKey(), map[string]any{"method": "CASH"})
		if want := map[int]int{0: 201, 1: 409}[i]; st != want {
			t.Fatalf("owner refund attempt %d: %d %s", i, st, raw)
		}
	}
	if n := r.e.count(`SELECT count(*) FROM app.payments WHERE tenant_id = $1 AND invoice_id = $2 AND status = 'PAID'`, r.tenant, r.invoice); n != 1 {
		t.Fatalf("payments: %d", n)
	}
	if n := r.e.count(`SELECT count(*) FROM app.cash_entries WHERE tenant_id = $1 AND kind = 'REFUND'`, r.tenant); n != 0 {
		t.Fatalf("the owner has no drawer, so no shift entry: %d", n)
	}
}

// Item 4: a stay checked out for 30 minutes with no payment and no recorded refund raises one PAYMENT_UNPAID alert; shift close lists it.
func TestUnpaidAlert_OpenRefund_FU(t *testing.T) {
	r := newRefundRig(t)
	rig := payRig{e: r.e, token: r.desk, tenant: r.tenant, invoice: r.invoice, code: r.code}
	rig.runJobsAt(r.outAt.Add(29*time.Minute + 59*time.Second))
	if n := len(rig.alerts("PAYMENT_UNPAID")); n != 0 {
		t.Fatalf("an alert at 29:59: %d", n)
	}
	rig.runJobsAt(r.outAt.Add(30 * time.Minute))
	got := rig.alerts("PAYMENT_UNPAID")
	if len(got) != 1 || got[0] != r.refund {
		t.Fatalf("one alert at 30:00 for the refund %d: %v", r.refund, got)
	}
	rig.runJobsAt(r.outAt.Add(5 * time.Hour))
	if n := len(rig.alerts("PAYMENT_UNPAID")); n != 1 {
		t.Fatalf("once per stay: %d", n)
	}

	st, raw := r.e.send("GET", "/v1/shifts/current", r.desk, "", nil)
	list, _ := parse(raw)["unpaidInvoices"].([]any)
	if st != 200 || len(list) != 1 || list[0].(map[string]any)["invoiceId"] != r.invoice || num(list[0].(map[string]any)["refundDue"]) != r.refund {
		t.Fatalf("shift close lists the open refund: %d %s", st, raw)
	}
	body := map[string]any{"counts": []map[string]any{{"denomination": 500000, "quantity": 2}}, "floatLeft": 0}
	if st, raw = r.e.send("POST", "/v1/shifts/current/close", r.desk, newKey(), body); st != 422 {
		t.Fatalf("closing with an open refund and no reason: %d %s", st, raw)
	}
}
