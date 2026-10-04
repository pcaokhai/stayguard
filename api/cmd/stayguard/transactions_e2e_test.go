//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func (r payRig) transactions(owner string) []map[string]any {
	day := r.e.clock.Now().In(time.UTC).Add(-24 * time.Hour).Format("2006-01-02")
	next := r.e.clock.Now().In(time.UTC).Add(48 * time.Hour).Format("2006-01-02")
	st, raw := r.e.send("GET", fmt.Sprintf("/v1/owner/transactions?from=%s&to=%s&filter=ALL", day, next), owner, "", nil)
	if st != 200 {
		r.e.t.Fatalf("transactions: %d %s", st, raw)
	}
	var out []map[string]any
	for _, it := range parse(raw)["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

// Item 5: a transfer received at one time and linked later shows both times (and sorts by when the bank money arrived).
func TestTransactions_ReceivedAndSettledTimes_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	received := r.e.clock.Now()
	if res, err := r.handler().Settle(context.Background(), r.event("bank-t1", r.balance, "no bill code here")); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("unmatched: %+v %v", res, err)
	}
	linkedAt := received.Add(7*time.Hour + 23*time.Minute)
	r.e.clock.set(linkedAt)
	var eventID string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payment_events WHERE tenant_id = $1 AND external_id = 'bank-t1'`, r.tenant).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+eventID+"/link", owner, newKey(), map[string]any{"invoiceId": r.invoice}); st != 200 {
		t.Fatalf("link: %d %s", st, raw)
	}
	var row map[string]any
	for _, tx := range r.transactions(owner) {
		if tx["method"] == "TRANSFER" && num(tx["amount"]) == r.balance {
			row = tx
		}
	}
	rt, _ := time.Parse(time.RFC3339, fmt.Sprint(row["receivedAt"]))
	st, _ := time.Parse(time.RFC3339, fmt.Sprint(row["settledAt"]))
	at, _ := time.Parse(time.RFC3339, fmt.Sprint(row["at"]))
	if row == nil || !rt.Equal(received.UTC()) || !st.Equal(linkedAt.UTC()) || !at.Equal(received.UTC()) || row["kind"] != "PAYMENT" {
		t.Fatalf("the transfer row (received %s, linked %s): %v", received.UTC(), linkedAt.UTC(), row)
	}
}

// Item 5: a deposit refund is a negative CASH_REFUND line, and no longer a "0 đ cash" payment.
func TestTransactions_RefundIsANegativeLine_FU(t *testing.T) {
	r := newRefundRig(t)
	if st, raw := r.e.send("POST", "/v1/invoices/"+r.invoice+"/payments", r.desk, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("refund: %d %s", st, raw)
	}
	rig := payRig{e: r.e, tenant: r.tenant}
	var refunds int
	for _, tx := range rig.transactions(r.own) {
		if num(tx["amount"]) == 0 {
			t.Fatalf("a zero-amount line: %v", tx)
		}
		if tx["kind"] == "CASH_REFUND" {
			refunds++
			if num(tx["amount"]) != -r.refund || tx["method"] != "CASH" || tx["roomCode"] != "A102" || tx["settledAt"] == nil || tx["shiftId"] == nil {
				t.Fatalf("the refund line (want -%d): %v", r.refund, tx)
			}
		}
	}
	if refunds != 1 {
		t.Fatalf("%d refund lines, want 1", refunds)
	}
}
