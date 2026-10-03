//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

// attention lists the owner overview's attention items of a kind.
func (r payRig) attention(owner, kind string) []map[string]any {
	st, raw := r.e.send("GET", "/v1/owner/overview", owner, "", nil)
	if st != 200 {
		r.e.t.Fatalf("overview: %d %s", st, raw)
	}
	var out []map[string]any
	for _, it := range parse(raw)["attention"].([]any) {
		if m := it.(map[string]any); m["kind"] == kind {
			out = append(out, m)
		}
	}
	return out
}

// alertRow reads the one alert of a kind for the rig's tenant: its resolution and resolved time.
func (r payRig) alertRow(kind string) (resolution *string, resolved bool, n int) {
	rows, err := r.e.owner.Query(context.Background(), `SELECT resolution, resolved_at IS NOT NULL FROM app.alerts WHERE tenant_id = $1 AND kind = $2`, r.tenant, kind)
	if err != nil {
		r.e.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		n++
		if err := rows.Scan(&resolution, &resolved); err != nil {
			r.e.t.Fatal(err)
		}
	}
	return
}

func (r payRig) ownerToken() string { return r.e.demo("OWNER", "vi", r.tenant).str("accessToken") }

// Item 2: PAYMENT_UNPAID resolves when the invoice is paid; the owner overview then no longer lists it, and the alert keeps its history.
func TestAlertLifecycle_UnpaidResolvesWhenPaid_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	_, at := r.invoiceTotalAndCreated()
	r.runJobsAt(at.Add(30 * time.Minute))
	if n := len(r.attention(owner, "PAYMENT_UNPAID")); n != 1 {
		t.Fatalf("an unpaid invoice needs action: %d", n)
	}
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.token, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("cash: %d %s", st, raw)
	}
	res, resolved, n := r.alertRow("PAYMENT_UNPAID")
	if n != 1 || !resolved || res == nil || *res != "PAID" {
		t.Fatalf("the alert is kept, resolved as PAID: n=%d resolved=%v %v", n, resolved, res)
	}
	if n := len(r.attention(owner, "PAYMENT_UNPAID")); n != 0 {
		t.Fatalf("a paid invoice is still in the overview: %d", n)
	}
	st, raw := r.e.send("GET", "/v1/owner/alerts?kind=PAYMENT_UNPAID", owner, "", nil)
	items, _ := parse(raw)["items"].([]any)
	if st != 200 || len(items) != 1 || items[0].(map[string]any)["resolvedAt"] == nil || items[0].(map[string]any)["resolution"] != "PAID" {
		t.Fatalf("the alert list keeps it with resolvedAt and resolution: %d %s", st, raw)
	}
}

func TestAlertLifecycle_PartialResolvesWhenPaid_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	at := r.e.clock.Now()
	h := r.handler()
	if res, err := h.Settle(context.Background(), r.event("bank-l1", r.balance/2, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	r.runJobsAt(at.Add(15 * time.Minute))
	if n := len(r.attention(owner, "PAYMENT_PARTIAL")); n != 1 {
		t.Fatalf("a short transfer needs action: %d", n)
	}
	if res, err := h.Settle(context.Background(), r.event("bank-l2", r.balance-r.balance/2, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("rest: %+v %v", res, err)
	}
	if _, resolved, _ := r.alertRow("PAYMENT_PARTIAL"); !resolved || len(r.attention(owner, "PAYMENT_PARTIAL")) != 0 {
		t.Fatal("paying the rest resolves the partial alert")
	}
}

// An unmatched transfer is resolved when the owner links it to an invoice.
func TestAlertLifecycle_UnmatchedResolvesWhenLinked_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	if res, err := r.handler().Settle(context.Background(), r.event("bank-u9", r.balance, "no bill code here")); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("unmatched: %+v %v", res, err)
	}
	if n := len(r.attention(owner, "UNMATCHED_TRANSFER")); n != 1 {
		t.Fatalf("an unmatched transfer needs action: %d", n)
	}
	var eventID string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payment_events WHERE tenant_id = $1 AND external_id = 'bank-u9'`, r.tenant).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+eventID+"/link", owner, newKey(), map[string]any{"invoiceId": r.invoice}); st != 200 {
		t.Fatalf("link: %d %s", st, raw)
	}
	res, resolved, _ := r.alertRow("UNMATCHED_TRANSFER")
	if !resolved || res == nil || *res != "LINKED" || len(r.attention(owner, "UNMATCHED_TRANSFER")) != 0 {
		t.Fatalf("linking resolves it: resolved=%v %v", resolved, res)
	}
}

// Item 3: an open deposit refund is REFUND_PENDING (data refundDue), never PAYMENT_UNPAID, under the same 30-minute rule; recording the refund resolves it.
func TestRefundPendingAlert_After30Minutes_FU(t *testing.T) {
	r := newRefundRig(t)
	rig := payRig{e: r.e, token: r.desk, tenant: r.tenant, invoice: r.invoice, code: r.code}
	rig.runJobsAt(r.outAt.Add(29*time.Minute + 59*time.Second))
	if len(rig.alerts("REFUND_PENDING"))+len(rig.alerts("PAYMENT_UNPAID")) != 0 {
		t.Fatal("an alert at 29:59")
	}
	rig.runJobsAt(r.outAt.Add(30 * time.Minute))
	got := rig.alerts("REFUND_PENDING")
	if len(got) != 1 || got[0] != r.refund || len(rig.alerts("PAYMENT_UNPAID")) != 0 {
		t.Fatalf("one REFUND_PENDING for %d and no PAYMENT_UNPAID: %v / %v", r.refund, got, rig.alerts("PAYMENT_UNPAID"))
	}
	var due string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT details->>'refundDue' FROM app.alerts WHERE tenant_id = $1 AND kind = 'REFUND_PENDING'`, r.tenant).Scan(&due); err != nil || due == "" {
		t.Fatalf("the alert carries refundDue: %q %v", due, err)
	}
	rig.runJobsAt(r.outAt.Add(6 * time.Hour))
	if n := len(rig.alerts("REFUND_PENDING")); n != 1 {
		t.Fatalf("once per stay: %d", n)
	}
	if n := len(rig.attention(r.own, "REFUND_PENDING")); n != 1 {
		t.Fatalf("it needs action: %d", n)
	}
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("refund: %d %s", st, raw)
	}
	if res, resolved, _ := rig.alertRow("REFUND_PENDING"); !resolved || res == nil || *res != "REFUNDED" || len(rig.attention(r.own, "REFUND_PENDING")) != 0 {
		t.Fatalf("recording the refund resolves it: %v %v", resolved, res)
	}
}
