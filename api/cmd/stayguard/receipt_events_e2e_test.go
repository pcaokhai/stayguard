//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TT-08 (a): the receipt shows one line per bank event that settled money on the invoice, each with its own amount and time, and the
// lines add up to what was paid (not one cumulative line for a short transfer and its top-up).
func TestReceipt_OneLinePerBankEvent_FU(t *testing.T) {
	r := deskRig(t)
	h := r.handler()
	first := r.balance * 4 / 10
	t1 := r.e.clock.Now()
	if res, err := h.Settle(context.Background(), r.event("bank-r1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	t2 := t1.Add(25 * time.Minute)
	r.e.clock.set(t2)
	if res, err := h.Settle(context.Background(), r.event("bank-r2", r.balance-first, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("top-up: %+v %v", res, err)
	}
	st, raw := r.e.send("GET", "/v1/invoices/"+r.invoice+"/receipt", r.token, "", nil)
	lines, _ := parse(raw)["payments"].([]any)
	if st != 200 || len(lines) != 2 {
		t.Fatalf("receipt: %d %s", st, raw)
	}
	var sum int64
	for i, want := range []struct {
		amount int64
		at     time.Time
	}{{first, t1}, {r.balance - first, t2}} {
		l := lines[i].(map[string]any)
		at, _ := time.Parse(time.RFC3339, fmt.Sprint(l["at"]))
		if num(l["amount"]) != want.amount || !at.Equal(want.at.UTC()) || l["method"] != "TRANSFER" {
			t.Fatalf("line %d: %v, want %d at %s", i, l, want.amount, want.at.UTC())
		}
		sum += num(l["amount"])
	}
	if sum != r.balance {
		t.Fatalf("lines add up to %d, paid %d", sum, r.balance)
	}
}

// TT-08 (b): once the invoice is paid, its earlier partial (and mismatch) events are resolved: needs-action has no row for a paid bill.
func TestPaidInvoice_LeavesNothingInNeedsAction_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	h := r.handler()
	if res, err := h.Settle(context.Background(), r.event("bank-n1", r.balance/2, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	needs := func() int {
		st, raw := r.e.send("GET", r.needsActionURL(), owner, "", nil)
		items, _ := parse(raw)["items"].([]any)
		if st != 200 {
			t.Fatalf("needs action: %d %s", st, raw)
		}
		return len(items)
	}
	if needs() != 1 {
		t.Fatalf("the short transfer needs action: %d", needs())
	}
	// A transfer-era mismatch row that the old flow left behind (payment MISMATCH with its event), on the same invoice.
	r.e.exec(`INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount, received_amount, reference_code, created_at, transaction_id)
		VALUES ('pm_old', $1, $2, 'TRANSFER', 'MISMATCH', 1000, 500, $3, now(), 'bank-old')`, r.tenant, r.invoice, r.code)
	r.e.exec(`INSERT INTO app.payment_events (id, tenant_id, provider, external_id, amount, reference_code, result, received_at)
		VALUES ('pe_old', $1, 'test', 'bank-old', 500, $2, 'MISMATCH', now())`, r.tenant, r.code)
	if needs() != 2 {
		t.Fatalf("both rows need action: %d", needs())
	}
	if res, err := h.Settle(context.Background(), r.event("bank-n2", r.balance-r.balance/2, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("top-up: %+v %v", res, err)
	}
	if n := needs(); n != 0 {
		t.Fatalf("a paid bill has %d rows in needs-action", n)
	}
}
