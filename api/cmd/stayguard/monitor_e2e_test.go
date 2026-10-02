//go:build integration

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// SG-903: an owner can link an unmatched bank transfer to an unpaid invoice, once, only for bank-reported money and only
// as the owner; the invoice is settled by the settlement code and the whole story is visible in the monitoring reads.
func TestMonitorE2E_LinkUnmatchedTransfer_SG903(t *testing.T) {
	r := newPayRig(t, "A102")
	e, ctx, h := r.e, context.Background(), r.handler()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	day := e.clock.Now().In(loc).Format("2006-01-02")
	desk := e.demo("RECEPTIONIST", "vi", r.tenant).str("accessToken")

	// Three bank events nobody matched: the right amount, the wrong amount, and one from the demo bank.
	for id, amount := range map[string]int64{"bank-ok": r.balance, "bank-low": r.balance - 1000} {
		if res, err := h.Settle(ctx, r.event(id, amount, "no code here")); err != nil || res.Result != "UNMATCHED" {
			t.Fatalf("%s: %+v %v", id, res, err)
		}
	}
	sim := r.event("sim-1", r.balance, "no code here")
	sim.Provider = "simulator"
	if res, err := h.Settle(ctx, sim); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("simulator event: %+v %v", res, err)
	}
	ids := map[string]string{} // external id by the event row id
	rows, err := e.owner.Query(ctx, `SELECT external_id, id FROM app.payment_events WHERE tenant_id = $1`, r.tenant)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var ext, id string
		if err := rows.Scan(&ext, &id); err != nil {
			t.Fatal(err)
		}
		ids[ext] = id
	}
	rows.Close()

	// The transactions list asks for action on all three, and the overview and alerts say so too.
	st, raw := e.send("GET", fmt.Sprintf("/v1/owner/transactions?from=%s&to=%s&filter=NEEDS_ACTION", day, day), r.token, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 3 || items[0].(map[string]any)["reconciliation"] != "UNMATCHED" {
		t.Fatalf("transactions: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/owner/alerts?unread=true&kind=UNMATCHED_TRANSFER", r.token, "", nil)
	alerts, _ := parse(raw)["items"].([]any)
	if st != 200 || len(alerts) != 3 {
		t.Fatalf("alerts: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/owner/overview", r.token, "", nil)
	ov := parse(raw)
	if att, _ := ov["attention"].([]any); st != 200 || len(att) == 0 || len(ov["buildings"].([]any)) == 0 {
		t.Fatalf("overview: %d %s", st, raw)
	}

	link := func(token, event, key string, body map[string]any) (int, map[string]any) {
		st, raw := e.send("POST", "/v1/owner/payment-events/"+event+"/link", token, key, body)
		return st, parse(raw)
	}
	body := map[string]any{"invoiceId": r.invoice}
	if st, _ := link(desk, ids["bank-ok"], newKey(), body); st != 403 {
		t.Fatalf("a receptionist linked a transfer: %d", st)
	}
	if st, b := link(r.token, ids["sim-1"], newKey(), body); st != 422 {
		t.Fatalf("a demo-bank event was linked: %d %v", st, b)
	}
	if st, b := link(r.token, ids["bank-low"], newKey(), body); st != 422 {
		t.Fatalf("an event of another amount was linked: %d %v", st, b)
	}
	if st, _ := link(r.token, "nope", newKey(), body); st != 404 {
		t.Fatalf("unknown event: %d", st)
	}
	if st, _ := link(r.token, ids["bank-ok"], newKey(), map[string]any{"invoiceId": "nope"}); st != 404 {
		t.Fatalf("unknown invoice: %d", st)
	}
	if r.status("invoices", r.invoice) != "OPEN" {
		t.Fatal("a refused link changed the invoice")
	}

	key := newKey()
	st, got := link(r.token, ids["bank-ok"], key, body)
	if st != 200 || got["reconciliation"] != "MATCHED" || got["method"] != "TRANSFER" || got["paymentEventId"] != ids["bank-ok"] || got["billCode"] != r.code {
		t.Fatalf("link: %d %v", st, got)
	}
	if r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" {
		t.Fatalf("the invoice was not settled: %s, room %s", r.status("invoices", r.invoice), r.roomStatus())
	}
	var eventResult, payStatus string
	_ = e.owner.QueryRow(ctx, `SELECT result FROM app.payment_events WHERE id = $1`, ids["bank-ok"]).Scan(&eventResult)
	_ = e.owner.QueryRow(ctx, `SELECT status FROM app.payments WHERE invoice_id = $1 AND method = 'TRANSFER'`, r.invoice).Scan(&payStatus)
	if eventResult != "SETTLED" || payStatus != "PAID" {
		t.Fatalf("event %s payment %s", eventResult, payStatus)
	}

	// Once: the same event again is a conflict under a new key and a replay under the old one.
	if st, b := link(r.token, ids["bank-ok"], newKey(), body); st != 409 || b["code"] != "EVENT_NOT_LINKABLE" {
		t.Fatalf("second link: %d %v", st, b)
	}
	if st, again := link(r.token, ids["bank-ok"], key, body); st != 200 || again["id"] != got["id"] {
		t.Fatalf("replay: %d %v", st, again)
	}

	// The audit log, the timeline and the transactions list tell the story; alerts can be marked read.
	st, raw = e.send("GET", fmt.Sprintf("/v1/owner/audit-logs?from=%s&to=%s&category=MONEY&q=%s", day, day, r.invoice), r.token, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 1 || items[0].(map[string]any)["action"] != "payment.linked" {
		t.Fatalf("audit log: %d %s", st, raw)
	}
	if st, _ = e.send("GET", fmt.Sprintf("/v1/owner/audit-logs?from=%s&to=%s&category=NOPE", day, day), r.token, "", nil); st != 422 {
		t.Fatalf("unknown category: %d", st)
	}
	if st, _ = e.send("GET", fmt.Sprintf("/v1/owner/audit-logs?from=%s&to=%s", day, day), desk, "", nil); st != 403 {
		t.Fatalf("a receptionist read the activity log: %d", st)
	}
	var stayID string
	_ = e.owner.QueryRow(ctx, `SELECT stay_id FROM app.invoices WHERE id = $1`, r.invoice).Scan(&stayID)
	st, raw = e.send("GET", "/v1/owner/stays/"+stayID+"/timeline", r.token, "", nil)
	var kinds string
	for _, it := range parse(raw)["items"].([]any) {
		kinds += it.(map[string]any)["kind"].(string) + " "
	}
	if st != 200 || !strings.Contains(kinds, "LINKED_BY_OWNER") || !strings.Contains(kinds, "PAYMENT_RECEIVED") {
		t.Fatalf("timeline: %d %s", st, kinds)
	}
	alertID := alerts[0].(map[string]any)["id"].(string)
	if st, _ = e.send("POST", "/v1/owner/alerts/"+alertID+"/read", r.token, "", nil); st != 204 {
		t.Fatalf("mark read: %d", st)
	}
	if st, _ = e.send("POST", "/v1/owner/alerts/"+alertID+"/read", r.token, "", nil); st != 204 {
		t.Fatalf("mark read again: %d", st)
	}
	if st, _ = e.send("POST", "/v1/owner/alerts/nope/read", r.token, "", nil); st != 404 {
		t.Fatalf("unknown alert: %d", st)
	}
	st, raw = e.send("GET", "/v1/owner/alerts?unread=true&kind=UNMATCHED_TRANSFER", r.token, "", nil)
	if left, _ := parse(raw)["items"].([]any); st != 200 || len(left) != 2 {
		t.Fatalf("unread after one read: %d %s", st, raw)
	}
}
