//go:build integration

package main

import (
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

const (
	tenantZone    = "Asia/Ho_Chi_Minh"
	flowDeposit   = 100_000
	flowStayTotal = 350_000 // the seeded overnight window price
	flowExtras    = 2*waterPrice + beerPrice
	refundDeposit = 500_000
)

// wantInvoiceQuote prices the seeded plan independently of the use case: QuoteRunning plus Assemble.
func wantInvoiceQuote(t *testing.T, checkOut time.Time, deposit money.Vnd, extras []pricing.Extra) (pricing.Quote, pricing.Bill) {
	t.Helper()
	loc, err := time.LoadLocation(tenantZone)
	if err != nil {
		t.Fatal(err)
	}
	q, err := pricing.QuoteRunning(seedPlan(), pricing.RentalOvernight, fixedCheckIn, checkOut, loc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := pricing.Assemble(q, extras, deposit)
	if err != nil {
		t.Fatal(err)
	}
	return q, bill
}

func TestBillingFlowE2E_SG205_AC5(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	e.seedServices(tenant, 10)
	id := e.openStay(token, 1, nil)

	st, raw := e.send("GET", "/v1/services", token, "", nil)
	items, _ := parse(raw)["items"].([]any)
	if first, _ := items[0].(map[string]any); st != 200 || len(items) != 2 || first["code"] != "BEER" || first["stock"] != float64(10) || first["price"] != float64(beerPrice) {
		t.Fatalf("list services: %s", describe(st, raw))
	}

	st, raw = e.addExtras(token, id, newKey(), extrasBody("WATER", 2, "BEER", 1))
	got := parse(raw)
	if st != 200 || got["status"] != "ACTIVE" || got["quote"].(map[string]any)["extrasAmount"] != float64(flowExtras) {
		t.Fatalf("add extras: %s", describe(st, raw))
	}
	if e.stock(tenant, "WATER") != 8 || e.stock(tenant, "BEER") != 9 {
		t.Errorf("stock after extras: water %d beer %d", e.stock(tenant, "WATER"), e.stock(tenant, "BEER"))
	}
	gst, graw := e.send("GET", "/v1/stays/"+id, token, "", nil)
	if gst != 200 || parse(graw)["quote"].(map[string]any)["extrasAmount"] != float64(flowExtras) || len(parse(graw)["extras"].([]any)) != 2 {
		t.Fatalf("get stay: %s", describe(gst, graw))
	}

	// A nanosecond component: the stored instants must come back identical on a repeat (database keeps microseconds).
	out := fixedCheckIn.Add(billDay + 123456789*time.Nanosecond)
	e.clock.set(out)
	st, raw = e.checkout(token, id, newKey())
	inv := parse(raw)
	extras := []pricing.Extra{{ServiceCode: "WATER", Quantity: 2, UnitAmount: waterPrice}, {ServiceCode: "BEER", Quantity: 1, UnitAmount: beerPrice}}
	q, bill := wantInvoiceQuote(t, out, flowDeposit, extras)
	if st != 201 || inv["billCode"] != "PH1001R1" || inv["status"] != "OPEN" || inv["roomCode"] != "R1" || inv["stayId"] != id || inv["createdAt"] != out.Truncate(time.Microsecond).Format(time.RFC3339Nano) {
		t.Fatalf("checkout: %s", describe(st, raw))
	}
	if bill.Total.Int64() != flowStayTotal+flowExtras || bill.BalanceDue.Int64() != flowStayTotal+flowExtras-flowDeposit {
		t.Fatalf("independent bill = %+v", bill)
	}
	assertQuote(t, "invoice", inv, q, bill, flowDeposit)
	if inv["quote"].(map[string]any)["extrasAmount"] != float64(flowExtras) {
		t.Errorf("invoice extras = %v", inv["quote"])
	}

	// Repeat with a different key: the identical invoice. Extras on the checked-out stay: 409.
	if st2, raw2 := e.checkout(token, id, newKey()); st2 != 201 || string(raw2) != string(raw) {
		t.Errorf("repeat checkout: %s", describe(st2, raw2))
	}
	if st2, raw2 := e.addExtras(token, id, newKey(), extrasBody("WATER", 1)); st2 != 409 || problemCode(raw2) != "STAY_NOT_ACTIVE" {
		t.Errorf("extras after checkout: %s", describe(st2, raw2))
	}
	assertRoomStaysOccupied(t, e, token, 1)
}

// AC6: the room is not released by check-out; only the payment of the invoice (SG-302) does that.
func assertRoomStaysOccupied(t *testing.T, e *env, token string, room int) {
	t.Helper()
	if n := e.count(`SELECT count(*) FROM app.units WHERE id = $1 AND status = 'OCCUPIED'`, roomID(room)); n != 1 {
		t.Error("room is not OCCUPIED after check-out")
	}
	if st, raw := e.checkIn(token, room, newKey(), stayBody(nil)); st != 409 || problemCode(raw) != "ROOM_NOT_VACANT" {
		t.Errorf("new check-in on the checked-out room: %s", describe(st, raw))
	}
}

func TestBillingRefundE2E_SG205_AC5(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 1)
	id := e.openStay(token, 1, map[string]any{"deposit": refundDeposit})
	out := fixedCheckIn.Add(billDay)
	e.clock.set(out)

	st, raw := e.checkout(token, id, newKey())
	q, bill := wantInvoiceQuote(t, out, refundDeposit, nil)
	quote, _ := parse(raw)["quote"].(map[string]any)
	if st != 201 || quote["refundDue"] != float64(refundDeposit-flowStayTotal) || quote["balanceDue"] != float64(0) || bill.RefundDue.Int64() != refundDeposit-flowStayTotal {
		t.Fatalf("refund invoice: %s", describe(st, raw))
	}
	assertQuote(t, "refund", parse(raw), q, bill, refundDeposit)
	if total := e.count(`SELECT total FROM app.invoices WHERE stay_id = $1`, id); total != flowStayTotal {
		t.Errorf("invoices.total = %d, want the quote total %d", total, flowStayTotal)
	}
}

func TestBillingIdempotencyE2E_SG205_AC2(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.seedServices(tenant, 3)
	id := e.openStay(token, 1, nil)
	key := newKey()

	st1, first := e.addExtras(token, id, key, extrasBody("WATER", 2))
	st2, second := e.addExtras(token, id, key, extrasBody("WATER", 2))
	if st1 != 200 || st2 != 200 || string(first) != string(second) {
		t.Fatalf("replay: %d %d\n%s\n%s", st1, st2, first, second)
	}
	if e.stock(tenant, "WATER") != 1 || e.count(`SELECT count(*) FROM app.stay_extras`) != 1 {
		t.Error("a replay changed stock or rows")
	}
	if st, raw := e.addExtras(token, id, key, extrasBody("WATER", 1)); st != 409 || problemCode(raw) != "IDEMPOTENCY_KEY_REUSED" {
		t.Errorf("different body: %s", describe(st, raw))
	}

	// A failed attempt (not enough stock) leaves no key row: the transaction rolled back.
	failKey, keysBefore := newKey(), e.count(`SELECT count(*) FROM app.idempotency_keys`)
	if st, raw := e.addExtras(token, id, failKey, extrasBody("WATER", 2)); st != 409 || problemCode(raw) != "INSUFFICIENT_STOCK" {
		t.Errorf("short stock: %s", describe(st, raw))
	}
	if e.count(`SELECT count(*) FROM app.idempotency_keys WHERE key = $1`, failKey) != 0 || e.count(`SELECT count(*) FROM app.idempotency_keys`) != keysBefore {
		t.Error("failed attempt left a key row")
	}
}
