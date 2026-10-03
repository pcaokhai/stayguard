//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

func (r payRig) invoiceTotalAndCreated() (int64, time.Time) {
	var total int64
	var at time.Time
	if err := r.e.owner.QueryRow(context.Background(), `SELECT total, created_at FROM app.invoices WHERE id = $1`, r.invoice).Scan(&total, &at); err != nil {
		r.e.t.Fatal(err)
	}
	return total, at.UTC()
}

// roomMapStay is the active stay summary the room map reads for the rig's room.
func (r payRig) roomMapStay() map[string]any {
	var building string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT building_id FROM app.units WHERE tenant_id = $1 AND code = $2`, r.tenant, r.room).Scan(&building); err != nil {
		r.e.t.Fatal(err)
	}
	st, raw := r.e.send("GET", "/v1/buildings/"+building+"/rooms", r.token, "", nil)
	if st != 200 {
		r.e.t.Fatalf("listRooms: %d %s", st, raw)
	}
	items, _ := parse(raw)["items"].([]any)
	for _, it := range items {
		m := it.(map[string]any)
		if m["code"] == r.room {
			s, _ := m["activeStay"].(map[string]any)
			return s
		}
	}
	r.e.t.Fatalf("room %s not in %s", r.room, raw)
	return nil
}

func (r payRig) stayID() string {
	var id string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT stay_id FROM app.invoices WHERE id = $1`, r.invoice).Scan(&id); err != nil {
		r.e.t.Fatal(err)
	}
	return id
}

// A checked-out stay with an unpaid invoice shows pendingPayment on the room map and on getStay, with the money so far.
func TestPendingPayment_OnRoomMapAndGetStay_FU(t *testing.T) {
	r := deskRig(t)
	_, p := r.pay("TRANSFER")
	payID := p["id"].(string)
	total, _ := r.invoiceTotalAndCreated()
	deposit := total - r.balance

	check := func(step string, received, remaining int64) {
		t.Helper()
		s := r.roomMapStay()
		pp, _ := s["pendingPayment"].(map[string]any)
		if pp == nil || pp["paymentId"] != payID || num(pp["total"]) != total || num(pp["received"]) != received || num(pp["remaining"]) != remaining {
			t.Fatalf("%s: room map pendingPayment %v (want %s %d %d %d)", step, s, payID, total, received, remaining)
		}
		st, raw := r.e.send("GET", "/v1/stays/"+r.stayID(), r.token, "", nil)
		gp, _ := parse(raw)["pendingPayment"].(map[string]any)
		if st != 200 || gp == nil || gp["paymentId"] != payID || num(gp["received"]) != received || num(gp["remaining"]) != remaining {
			t.Fatalf("%s: getStay pendingPayment %d %s", step, st, raw)
		}
	}
	check("nothing paid yet", deposit, r.balance)
	first := r.balance * 3 / 10
	if res, err := r.handler().Settle(context.Background(), r.event("bank-pp1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	check("after a partial transfer", deposit+first, r.balance-first)
	if res, err := r.handler().Settle(context.Background(), r.event("bank-pp2", r.balance-first, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("rest: %+v %v", res, err)
	}
	if s := r.roomMapStay(); s != nil {
		t.Fatalf("a paid room has no stay on the map: %v", s)
	}
	_, raw := r.e.send("GET", "/v1/stays/"+r.stayID(), r.token, "", nil)
	if parse(raw)["pendingPayment"] != nil {
		t.Fatalf("a paid stay has no pending payment: %s", raw)
	}
}

// Anti-loss: no bank or cash money on an unpaid invoice 30 minutes after check-out raises one PAYMENT_UNPAID alert (server time).
func TestUnpaidAlert_After30Minutes_FU(t *testing.T) {
	r := deskRig(t)
	_, at := r.invoiceTotalAndCreated()
	r.runJobsAt(at.Add(29*time.Minute + 59*time.Second))
	if n := len(r.alerts("PAYMENT_UNPAID")); n != 0 {
		t.Fatalf("an alert at 29:59: %d", n)
	}
	r.runJobsAt(at.Add(30 * time.Minute))
	got := r.alerts("PAYMENT_UNPAID")
	if len(got) != 1 || got[0] != r.balance {
		t.Fatalf("one alert for the unpaid balance %d at 30:00, got %v", r.balance, got)
	}
	r.runJobsAt(at.Add(3 * time.Hour))
	if n := len(r.alerts("PAYMENT_UNPAID")); n != 1 {
		t.Fatalf("once per invoice: %d", n)
	}
}

// Money on the invoice (a partial transfer, or a paid invoice) never raises PAYMENT_UNPAID: PAYMENT_PARTIAL covers the first.
func TestUnpaidAlert_NotWhenMoneyArrived_FU(t *testing.T) {
	r := deskRig(t)
	_, at := r.invoiceTotalAndCreated()
	if res, err := r.handler().Settle(context.Background(), r.event("bank-u1", r.balance/2, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	r.runJobsAt(at.Add(time.Hour))
	if n := len(r.alerts("PAYMENT_UNPAID")); n != 0 {
		t.Fatalf("an invoice with bank money raised PAYMENT_UNPAID: %d", n)
	}
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 1 {
		t.Fatalf("the partial alert covers it: %d", n)
	}
}
