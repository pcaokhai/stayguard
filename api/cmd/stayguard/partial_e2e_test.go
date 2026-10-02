//go:build integration

package main

import (
	"context"
	"testing"
)

func (r payRig) payment(id string) reply { return r.e.call("GET", "/v1/payments/"+id, r.token, nil) }

func (r payRig) alerts(kind string) []int64 {
	rows, err := r.e.owner.Query(context.Background(), `SELECT coalesce(amount, 0) FROM app.alerts WHERE tenant_id = $1 AND kind = $2 ORDER BY created_at`, r.tenant, kind)
	if err != nil {
		r.e.t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			r.e.t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

func num(v any) int64 { f, _ := v.(float64); return int64(f) }

// Partial transfers: bank money matched to the bill code accumulates; the QR asks for what is left; only a bank event closes the invoice.
func TestPartialTransfers_AccumulateUntilPaid_FU(t *testing.T) {
	r := newPayRig(t, "A102")
	h := r.handler()
	st, p := r.pay("TRANSFER")
	id := p["id"].(string)
	if st != 201 || num(p["remaining"]) != r.balance || num(p["qr"].(map[string]any)["amount"]) != r.balance {
		t.Fatalf("create: %d %v", st, p)
	}
	first := r.balance * 4 / 10
	res, err := h.Settle(context.Background(), r.event("bank-p1", first, r.code))
	if err != nil || res.Result != "PARTIAL" || res.PaymentID != id {
		t.Fatalf("partial: %+v %v", res, err)
	}
	if r.status("payments", id) != "PENDING" || r.status("invoices", r.invoice) != "OPEN" || r.roomStatus() != "OCCUPIED" {
		t.Fatal("a partial transfer must leave the payment pending, the invoice open and the room occupied")
	}
	left := r.balance - first
	got := r.payment(id)
	qr, _ := got.body["qr"].(map[string]any)
	if got.str("status") != "PENDING" || num(got.body["receivedAmount"]) != first || num(got.body["remaining"]) != left || qr == nil ||
		num(qr["amount"]) != left || qr["transferNote"] != r.code {
		t.Fatalf("payment after a partial transfer: %v", got.body)
	}
	if st, again := r.pay("TRANSFER"); st != 201 || again["id"] != id || num(again["remaining"]) != left || num(again["qr"].(map[string]any)["amount"]) != left {
		t.Fatalf("a new transfer payment must show the remaining QR with the same bill code: %d %v", st, again)
	}
	if n := len(r.alerts("PAYMENT_MISMATCH")) + len(r.alerts("PAYMENT_PARTIAL")); n != 0 {
		t.Fatalf("a short transfer raises no immediate alert (PAYMENT_PARTIAL comes after 15 minutes): %d alerts", n)
	}

	res, err = h.Settle(context.Background(), r.event("bank-p2", left, r.code))
	if err != nil || res.Result != "SETTLED" {
		t.Fatalf("the rest: %+v %v", res, err)
	}
	done := r.payment(id)
	if r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" || done.str("status") != "PAID" ||
		num(done.body["receivedAmount"]) != r.balance || num(done.body["remaining"]) != 0 || len(r.alerts("OVERPAID")) != 0 {
		t.Fatalf("after the rest: invoice %s room %s payment %v", r.status("invoices", r.invoice), r.roomStatus(), done.body)
	}
	if n := r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND result = 'SETTLED'`, r.tenant); n != 2 {
		t.Errorf("both events end SETTLED, got %d", n)
	}
}

func TestPartialTransfers_OverpaymentStillPaysAndAlerts_FU(t *testing.T) {
	r := newPayRig(t, "A102")
	h := r.handler()
	r.pay("TRANSFER")
	if res, err := h.Settle(context.Background(), r.event("bank-a", r.balance*7/10, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("first: %+v %v", res, err)
	}
	res, err := h.Settle(context.Background(), r.event("bank-b", r.balance*7/10, r.code))
	if err != nil || res.Result != "SETTLED" {
		t.Fatalf("second: %+v %v", res, err)
	}
	excess := r.balance*7/10*2 - r.balance
	if a := r.alerts("OVERPAID"); len(a) != 1 || a[0] != excess || r.status("invoices", r.invoice) != "PAID" {
		t.Fatalf("overpaid alerts %v want [%d]; invoice %s", a, excess, r.status("invoices", r.invoice))
	}
	// One transfer for too much does the same.
	o := newPayRig(t, "A102")
	o.pay("TRANSFER")
	if res, err := o.handler().Settle(context.Background(), o.event("bank-c", o.balance+5_000, o.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("one big transfer: %+v %v", res, err)
	}
	if a := o.alerts("OVERPAID"); len(a) != 1 || a[0] != 5_000 || o.status("invoices", o.invoice) != "PAID" {
		t.Fatalf("overpaid alerts %v", a)
	}
}

// Cash for the rest after the bank already sent part of it: the cash payment is for the remaining balance only.
func TestPartialTransfers_CashPaysTheRemainder_FU(t *testing.T) {
	r := newPayRig(t, "A102")
	r.pay("TRANSFER")
	part := r.balance / 2
	if res, err := r.handler().Settle(context.Background(), r.event("bank-h", part, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	st, c := r.pay("CASH")
	if st != 201 || c["status"] != "PAID" || num(c["amount"]) != r.balance-part || r.status("invoices", r.invoice) != "PAID" {
		t.Fatalf("cash for the remainder: %d %v", st, c)
	}
}

// linkTransferToInvoice: the event must equal the invoice's remaining balance, after partial money too.
func TestLinkTransfer_AmountIsTheRemainingBalance_FU(t *testing.T) {
	r := newPayRig(t, "A102")
	h := r.handler()
	r.pay("TRANSFER")
	part := r.balance / 4
	if res, err := h.Settle(context.Background(), r.event("bank-q", part, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	left := r.balance - part
	for id, amount := range map[string]int64{"bank-full": r.balance, "bank-left": left} { // no bill code: unmatched
		if res, err := h.Settle(context.Background(), r.event(id, amount, "no code")); err != nil || res.Result != "UNMATCHED" {
			t.Fatalf("%s: %+v %v", id, res, err)
		}
	}
	eventID := func(ext string) string {
		var id string
		_ = r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payment_events WHERE tenant_id = $1 AND external_id = $2`, r.tenant, ext).Scan(&id)
		return id
	}
	body := map[string]any{"invoiceId": r.invoice}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+eventID("bank-full")+"/link", r.token, newKey(), body); st != 409 {
		t.Fatalf("an event for the whole balance cannot be linked once part was paid: %d %s", st, raw)
	}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+eventID("bank-left")+"/link", r.token, newKey(), body); st != 200 {
		t.Fatalf("the event for the remaining balance links: %d %s", st, raw)
	}
	if r.status("invoices", r.invoice) != "PAID" {
		t.Fatal("linking the remainder pays the invoice")
	}
}

// A deposit larger than the bill: nothing to take by transfer; check-out returns the refund, which leaves the drawer as a
// cash payout of the shift, and the invoice is PAID.
func TestDepositOverBill_RefundIsACashPayout_FU(t *testing.T) {
	e := newEnv(t)
	owner := e.demo("OWNER", "vi", "")
	tenant, boss := owner.str("tenantId"), owner.str("accessToken")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.exec(`INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
		SELECT tenant_id, id, $2, 'EDIT' FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, tenant, stayBuilding)
	e.clock.set(e.start)
	st, raw := e.checkIn(desk, 1, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 1_000_000}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(30 * 60 * 1e9))
	st, raw = e.checkout(desk, stay, newKey())
	inv := parse(raw)
	refund := num(inv["quote"].(map[string]any)["refundDue"])
	if st != 201 || refund <= 0 || num(inv["quote"].(map[string]any)["balanceDue"]) != 0 {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	invID := inv["id"].(string)
	if st, raw = e.send("POST", "/v1/invoices/"+invID+"/payments", desk, newKey(), map[string]any{"method": "TRANSFER"}); st != 422 {
		t.Fatalf("a transfer for nothing: %d %s", st, raw)
	}
	var paid string
	_ = e.owner.QueryRow(context.Background(), `SELECT status FROM app.invoices WHERE id = $1`, invID).Scan(&paid)
	if paid != "OPEN" {
		t.Fatalf("a refused transfer changed the invoice: %s", paid)
	}
	if st, raw = e.send("POST", "/v1/invoices/"+invID+"/payments", desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("the refund payment: %d %s", st, raw)
	}
	_ = e.owner.QueryRow(context.Background(), `SELECT status FROM app.invoices WHERE id = $1`, invID).Scan(&paid)
	st, raw = e.send("GET", "/v1/shifts/current", desk, "", nil)
	sh := parse(raw)
	if paid != "PAID" || st != 200 || num(sh["cashOut"]) != refund || num(sh["cashIn"]) != 1_000_000 || num(sh["expectedCash"]) != 1_000_000-refund {
		t.Fatalf("invoice %s, shift %s (refund %d)", paid, raw, refund)
	}
	_ = boss
}

// Owner decision: the money overview is the owner's alone; a manager (even with EDIT everywhere) gets 403.
func TestOwnerOverview_ManagerForbidden_FU(t *testing.T) {
	e := newEnv(t)
	owner := e.ownerSetupRooms(1)
	mgr := e.roleToken(owner, "mai3", "MANAGER", "MANAGER", "EDIT")
	if st, _ := e.send("GET", "/v1/owner/overview", mgr, "", nil); st != 403 {
		t.Fatalf("a manager read the owner overview: %d", st)
	}
	if st, _ := e.send("GET", "/v1/owner/overview", owner, "", nil); st != 200 {
		t.Fatalf("the owner must read it: %d", st)
	}
}
