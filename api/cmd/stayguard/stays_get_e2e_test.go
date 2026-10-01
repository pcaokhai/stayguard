//go:build integration

package main

import (
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

const getAfter = 3*time.Hour + 20*time.Minute

// wantQuote prices the seeded plan for a stay open since checkIn, independently of the use case.
func wantQuote(t *testing.T, tz string, checkIn, asOf time.Time, deposit money.Vnd) (pricing.Quote, pricing.Bill) {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatal(err)
	}
	if !asOf.After(checkIn) {
		asOf = checkIn.Add(time.Minute) // a stay that just started owes the minimum billable interval
	}
	q, err := pricing.Price(seedPlan(), pricing.RentalOvernight, checkIn, asOf, loc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := pricing.Assemble(q, nil, deposit)
	if err != nil {
		t.Fatal(err)
	}
	return q, bill
}

func assertQuote(t *testing.T, label string, got map[string]any, q pricing.Quote, b pricing.Bill, deposit int64) {
	t.Helper()
	quote, _ := got["quote"].(map[string]any)
	lines, _ := quote["lines"].([]any)
	if quote["total"] != float64(b.Total.Int64()) || quote["stayAmount"] != float64(b.StayTotal.Int64()) ||
		quote["capped"] != q.Capped || quote["balanceDue"] != float64(b.BalanceDue.Int64()) ||
		quote["refundDue"] != float64(b.RefundDue.Int64()) || quote["depositPaid"] != float64(deposit) || len(lines) != len(q.Lines) {
		t.Errorf("%s quote = %v, want total %d lines %v capped %v", label, quote, b.Total.Int64(), q.Lines, q.Capped)
	}
	for i, l := range q.Lines {
		m, _ := lines[i].(map[string]any)
		if m["code"] != l.Code || m["quantity"] != float64(l.Quantity) || m["amount"] != float64(l.Amount.Int64()) {
			t.Errorf("%s line %d = %v, want %+v", label, i, m, l)
		}
	}
}

func TestGetStayE2E_SG203_AC6(t *testing.T) {
	e := newEnv(t)
	e.clock.set(fixedCheckIn)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 1)
	tablesBefore := e.count(`SELECT count(*) FROM app.stay_extras`) + e.count(`SELECT count(*) FROM app.invoices`) +
		e.count(`SELECT count(*) FROM app.payments`) + e.count(`SELECT count(*) FROM app.payment_events`)

	st, raw := e.checkIn(token, 1, newKey(), stayBody(nil))
	created := parse(raw)
	if st != 201 || created["checkInAt"] != fixedCheckIn.Format(time.RFC3339) || created["pricingVersion"] != float64(1) {
		t.Fatalf("create: %d %s", st, raw)
	}
	const deposit = 100_000 // TestStayDeposit_SG203_AC5: present on create and get
	q0, b0 := wantQuote(t, "Asia/Ho_Chi_Minh", fixedCheckIn, fixedCheckIn, deposit)
	assertQuote(t, "create", created, q0, b0, deposit)
	id, _ := created["id"].(string)

	e.clock.set(fixedCheckIn.Add(getAfter))
	gst, graw := e.send("GET", "/v1/stays/"+id, token, "", nil)
	got := parse(graw)
	q1, b1 := wantQuote(t, "Asia/Ho_Chi_Minh", fixedCheckIn, fixedCheckIn.Add(getAfter), deposit)
	if gst != 200 || got["deposit"] != float64(deposit) || got["status"] != "ACTIVE" || got["roomCode"] != "R1" {
		t.Fatalf("get: %d %s", gst, graw)
	}
	assertQuote(t, "get", got, q1, b1, deposit)
	if asOf, _ := got["quote"].(map[string]any)["asOf"].(string); asOf != fixedCheckIn.Add(getAfter).Format(time.RFC3339) {
		t.Errorf("asOf = %s", asOf)
	}

	// Check-in writes the stay, the room update, one audit row and one idempotency key, nothing else.
	after := e.count(`SELECT count(*) FROM app.stay_extras`) + e.count(`SELECT count(*) FROM app.invoices`) +
		e.count(`SELECT count(*) FROM app.payments`) + e.count(`SELECT count(*) FROM app.payment_events`)
	if after != tablesBefore || e.count(`SELECT count(*) FROM app.stays`) != 1 ||
		e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'stay.check_in'`) != 1 ||
		e.count(`SELECT count(*) FROM app.idempotency_keys`) != 1 {
		t.Errorf("unexpected rows written: others %d->%d", tablesBefore, after)
	}
}
