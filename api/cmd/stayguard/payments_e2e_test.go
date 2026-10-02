//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

// payRig is a seeded tenant with one checked-out stay and an open invoice.
type payRig struct {
	e       *env
	token   string
	tenant  string
	room    string
	invoice string
	code    string
	balance int64
}

func newPayRig(t *testing.T, roomCode string) payRig {
	t.Helper()
	e := newSeededEnv(t)
	s := e.demo("OWNER", "vi", "")
	r := payRig{e: e, token: s.str("accessToken"), tenant: s.str("tenantId"), room: roomCode}
	return r.checkedOut()
}

func (r payRig) checkedOut() payRig {
	e := r.e
	e.clock.set(e.start)
	var roomID string
	if err := e.owner.QueryRow(context.Background(), `SELECT id FROM app.units WHERE tenant_id = $1 AND code = $2`, r.tenant, r.room).Scan(&roomID); err != nil {
		e.t.Fatal(err)
	}
	st, raw := e.send("POST", "/v1/rooms/"+roomID+"/stays", r.token, newKey(), stayBody(nil))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 || stay == "" {
		e.t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(2 * time.Hour))
	st, raw = e.checkout(r.token, stay, newKey())
	inv := parse(raw)
	if st != 201 {
		e.t.Fatalf("checkout: %d %s", st, raw)
	}
	r.invoice, _ = inv["id"].(string)
	r.code, _ = inv["billCode"].(string)
	q, _ := inv["quote"].(map[string]any)
	bal, _ := q["balanceDue"].(float64)
	r.balance = int64(bal)
	if r.balance <= 0 {
		e.t.Fatalf("balance %d, want > 0", r.balance)
	}
	return r
}

func (r payRig) pay(method string) (int, map[string]any) {
	st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.token, newKey(), map[string]any{"method": method})
	return st, parse(raw)
}

func (r payRig) status(table, id string) string {
	var s string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT status FROM app.`+table+` WHERE id = $1`, id).Scan(&s); err != nil {
		r.e.t.Fatal(err)
	}
	return s
}

func (r payRig) roomStatus() string {
	var s string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT status FROM app.units WHERE tenant_id = $1 AND code = $2`, r.tenant, r.room).Scan(&s); err != nil {
		r.e.t.Fatal(err)
	}
	return s
}

func (r payRig) handler() *app.Payments {
	cfg := config.Config{DataEncryptionKey: testDataKey}
	p, err := newPayments(cfg, postgres.NewUnitOfWork(r.e.pool), postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), r.e.clock)
	if err != nil {
		r.e.t.Fatal(err)
	}
	return p
}

func (r payRig) event(id string, amount int64, content string) app.PaymentEvent {
	return app.PaymentEvent{TenantID: r.tenant, Provider: "test", ExternalID: id, Amount: amount, Content: content, ReceivedAt: r.e.clock.Now()}
}

func TestPaymentTransferSimulate_A2(t *testing.T) {
	r := newPayRig(t, "A102")
	st, p := r.pay("TRANSFER")
	qr, _ := p["qr"].(map[string]any)
	if st != 201 || p["status"] != "PENDING" || qr == nil || qr["transferNote"] != r.code || int64(qr["amount"].(float64)) != r.balance {
		t.Fatalf("create: %d %v (code %s balance %d)", st, p, r.code, r.balance)
	}
	if payload, _ := qr["payload"].(string); !strings.Contains(payload, "0010A000000727") || !strings.Contains(payload, "0000000000") {
		t.Fatalf("QR is not for the tenant account: %v", qr)
	}
	id := p["id"].(string)
	if r.roomStatus() != "OCCUPIED" {
		t.Fatal("room must stay OCCUPIED until paid")
	}
	if got := r.e.call("GET", "/v1/payments/"+id, r.token, nil); got.str("status") != "PENDING" {
		t.Fatalf("get pending: %v", got.body)
	}
	sim := r.e.call("POST", "/v1/demo/payments/"+id+"/simulate", r.token, nil)
	if sim.status != 200 || sim.str("status") != "PAID" || sim.str("paidAt") == "" {
		t.Fatalf("simulate: %d %v", sim.status, sim.body)
	}
	if got := r.e.call("GET", "/v1/payments/"+id, r.token, nil); got.str("status") != "PAID" {
		t.Fatalf("get paid: %v", got.body)
	}
	if r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" {
		t.Fatalf("invoice %s room %s", r.status("invoices", r.invoice), r.roomStatus())
	}
	if again := r.e.call("POST", "/v1/demo/payments/"+id+"/simulate", r.token, nil); again.str("status") != "PAID" {
		t.Fatalf("second simulate: %v", again.body)
	}
	if n := r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND result = 'SETTLED'`, r.tenant); n != 1 {
		t.Errorf("settled events = %d, want 1", n)
	}
}

func TestPaymentDuplicateEvent_A2(t *testing.T) {
	r := newPayRig(t, "A102")
	_, p := r.pay("TRANSFER")
	h := r.handler()
	ev := r.event("bank-1", r.balance, "CK "+strings.ToLower(r.code[:4])+"-"+r.code[4:])
	first, err := h.Settle(context.Background(), ev)
	if err != nil || first.Result != "SETTLED" {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := h.Settle(context.Background(), ev)
	if err != nil || second.Result != "DUPLICATE_IGNORED" {
		t.Fatalf("second: %+v %v", second, err)
	}
	if r.status("payments", p["id"].(string)) != "PAID" || r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1`, r.tenant) != 1 {
		t.Fatal("duplicate changed state")
	}
}

func TestPaymentOverpaidAndUnmatched_A2(t *testing.T) {
	r := newPayRig(t, "A102")
	_, p := r.pay("TRANSFER")
	id := p["id"].(string)
	h := r.handler()
	if res, err := h.Settle(context.Background(), r.event("bank-3", r.balance, "no code here")); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("unmatched: %+v %v", res, err)
	}
	if r.status("payments", id) != "PENDING" {
		t.Fatal("an unmatched event must leave the payment pending")
	}
	res, err := h.Settle(context.Background(), r.event("bank-2", r.balance+1000, r.code))
	if err != nil || res.Result != "SETTLED" {
		t.Fatalf("overpaid: %+v %v", res, err)
	}
	if r.status("payments", id) != "PAID" || r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" {
		t.Fatal("an overpayment still closes the invoice")
	}
	if got := r.alerts("OVERPAID"); len(got) != 1 {
		t.Fatalf("overpaid alert: %v", got)
	}
}

func TestPaymentCashAndRules_A2(t *testing.T) {
	r := newPayRig(t, "A102")
	st, p := r.pay("CASH")
	if st != 201 || p["status"] != "PAID" || p["qr"] != nil {
		t.Fatalf("cash: %d %v", st, p)
	}
	if r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" {
		t.Fatal("cash must close the invoice and release the room")
	}
	if st, _ := r.pay("TRANSFER"); st != 409 {
		t.Errorf("paying a paid invoice = %d, want 409", st)
	}
	if st, _ := r.pay("BITCOIN"); st != 422 {
		t.Errorf("unknown method = %d, want 422", st)
	}
	hk := r.e.demo("HOUSEKEEPING", "vi", r.tenant).str("accessToken")
	if got := r.e.call("GET", "/v1/payments/"+p["id"].(string), hk, nil); got.status != 403 {
		t.Errorf("housekeeping read = %d, want 403", got.status)
	}
}

func TestPaymentIdempotency_A2(t *testing.T) {
	r := newPayRig(t, "A102")
	key := newKey()
	var ids []string
	for range 2 {
		st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.token, key, map[string]any{"method": "TRANSFER"})
		if st != 201 {
			t.Fatalf("status %d %s", st, raw)
		}
		ids = append(ids, parse(raw)["id"].(string))
	}
	if ids[0] != ids[1] || r.e.count(`SELECT count(*) FROM app.payments WHERE tenant_id = $1`, r.tenant) != 1 {
		t.Fatalf("replay created a second payment: %v", ids)
	}
}

func TestHousekeepingAfterPayment_A3(t *testing.T) {
	r := newPayRig(t, "A102")
	r.pay("CASH") // closes the invoice and puts A102 in TO_CLEAN
	hk := r.e.demo("HOUSEKEEPING", "vi", r.tenant).str("accessToken")
	list := r.e.call("GET", "/v1/housekeeping/tasks", hk, nil)
	items, _ := list.body["items"].([]any)
	if list.status != 200 || len(items) != 5 { // 4 seeded TO_CLEAN rooms plus A102
		t.Fatalf("list: %d %v", list.status, list.body)
	}
	var id string
	for _, it := range items {
		if m := it.(map[string]any); m["roomCode"] == "A102" {
			id = m["id"].(string)
		}
	}
	for range 2 {
		st, raw := r.e.send("POST", "/v1/housekeeping/tasks/"+id+"/complete", hk, newKey(), nil)
		if got := parse(raw); st != 200 || got["status"] != "DONE" {
			t.Fatalf("complete: %d %s", st, raw)
		}
	}
	if r.roomStatus() != "VACANT" {
		t.Errorf("room = %s, want VACANT", r.roomStatus())
	}
	rec := r.e.demo("RECEPTIONIST", "vi", r.tenant).str("accessToken")
	// Since L-B4 any role with EDIT on the building can mark a room clean; on a clean room it only answers DONE again.
	if st, _ := r.e.send("POST", "/v1/housekeeping/tasks/"+id+"/complete", rec, newKey(), nil); st != 200 {
		t.Errorf("receptionist complete = %d, want 200", st)
	}
}

func TestOwnerOverview_A4(t *testing.T) {
	cash := newPayRig(t, "A102")
	cash.pay("CASH")
	xfer := cash
	xfer.room = "A105"
	xfer = xfer.checkedOut()
	_, p := xfer.pay("TRANSFER")
	xfer.e.clock.set(xfer.e.start.Add(3 * time.Hour)) // the transfer is paid an hour after the cash
	if sim := xfer.e.call("POST", "/v1/demo/payments/"+p["id"].(string)+"/simulate", xfer.token, nil); sim.str("status") != "PAID" {
		t.Fatalf("simulate: %v", sim.body)
	}
	e := cash.e
	num := func(r reply, path ...string) int64 {
		var cur any = r.body
		for _, k := range path {
			cur = cur.(map[string]any)[k]
		}
		return int64(cur.(float64))
	}
	// Revenue is what the invoices were for (docs 15 rule 18, like the report): each stay's 100,000 cash deposit counts too.
	const deposits = 2 * 100000
	ov := e.call("GET", "/v1/owner/overview", cash.token, nil)
	if ov.status != 200 {
		t.Fatalf("overview: %d %v", ov.status, ov.body)
	}
	if got := num(ov, "revenueTotal"); got != cash.balance+xfer.balance+deposits {
		t.Errorf("revenueTotal = %d, want %d", got, cash.balance+xfer.balance+deposits)
	}
	if got := num(ov, "transfersReceived"); got != xfer.balance {
		t.Errorf("transfersReceived = %d, want %d", got, xfer.balance)
	}
	if got := num(ov, "cashExpected"); got != cash.balance+deposits {
		t.Errorf("cashExpected = %d, want %d", got, cash.balance+deposits)
	}
	byB, _ := ov.body["byBuilding"].([]any)
	if len(byB) != 2 || byB[0].(map[string]any)["revenue"].(float64) != float64(cash.balance+xfer.balance+deposits) || byB[1].(map[string]any)["revenue"].(float64) != 0 {
		t.Errorf("byBuilding = %v", byB)
	}
	if num(ov, "occupancy", "totalRooms") != 35 || num(ov, "occupancy", "occupiedRooms") != 11 || num(ov, "occupancy", "overdueRooms") < 1 {
		t.Errorf("occupancy = %v", ov.body["occupancy"])
	}
	latest, _ := ov.body["latestPayments"].([]any)
	if len(latest) != 2 || latest[0].(map[string]any)["roomCode"] != "A105" || latest[1].(map[string]any)["method"] != "CASH" {
		t.Errorf("latestPayments = %v", latest)
	}
	if alerts, ok := ov.body["alerts"].([]any); !ok || len(alerts) != 0 {
		t.Errorf("alerts = %v", ov.body["alerts"])
	}
	if past := e.call("GET", "/v1/owner/overview?date=2000-01-01", cash.token, nil); num(past, "revenueTotal") != 0 || past.str("date") != "2000-01-01" {
		t.Errorf("past day = %v", past.body)
	}
	rec := e.demo("RECEPTIONIST", "vi", cash.tenant).str("accessToken")
	if r := e.call("GET", "/v1/owner/overview", rec, nil); r.status != 403 {
		t.Errorf("receptionist = %d, want 403", r.status)
	}
}
