//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

// A stay checked out with no deposit and nothing recorded: pendingPayment.remaining is the whole bill, with no payment record yet.
func TestPendingPayment_NoDepositNoPayment_RemainingIsTheBill_FU(t *testing.T) {
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(tenant, "A102")+"/stays", desk, newKey(), stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 0}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(40 * time.Minute))
	st, raw = e.checkout(desk, stay, newKey())
	inv := parse(raw)
	q, _ := inv["quote"].(map[string]any)
	if st != 201 || num(q["balanceDue"]) <= 0 || num(q["depositPaid"]) != 0 {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	due := num(q["balanceDue"])
	r := payRig{e: e, token: desk, tenant: tenant, room: "A102", invoice: inv["id"].(string)}
	s := r.roomMapStay()
	pp, _ := s["pendingPayment"].(map[string]any)
	if pp == nil || pp["paymentId"] != nil || num(pp["total"]) != due || num(pp["deposit"]) != 0 || num(pp["remaining"]) != due {
		t.Fatalf("room map pendingPayment (want remaining %d): %v", due, s)
	}
	_, gr := e.send("GET", "/v1/stays/"+stay, desk, "", nil)
	if gp, _ := parse(gr)["pendingPayment"].(map[string]any); num(gp["remaining"]) != due {
		t.Fatalf("getStay pendingPayment: %s", gr)
	}
}

// The same cases against rows in the database: a real checked-out invoice is rewritten to each shape (as stays already in the
// database may have it), and the room map and getStay must agree on remaining, with no payment record anywhere.
func TestPendingPayment_RemainingTable_FU(t *testing.T) {
	type c struct {
		name                            string
		total, deposit, partial, refund int64
		legacyQuote                     bool // the frozen quote has no depositPaid or refundDue key
		wantRemaining                   int64
	}
	for _, tc := range []c{
		{name: "no payment, no deposit", total: 80_000, wantRemaining: 80_000},
		{name: "no payment, no deposit, legacy quote", total: 80_000, legacyQuote: true, wantRemaining: 80_000},
		{name: "deposit below total", total: 80_000, deposit: 30_000, wantRemaining: 50_000},
		{name: "deposit above total", total: 80_000, deposit: 100_000, refund: 20_000, wantRemaining: 0},
		{name: "partial transfer", total: 80_000, partial: 30_000, wantRemaining: 50_000},
		{name: "deposit and partial transfer", total: 80_000, deposit: 30_000, partial: 20_000, wantRemaining: 30_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := deskRig(t) // a checked-out stay with an invoice (and a pending transfer, removed below: no payment record)
			ctx := context.Background()
			r.e.exec(`DELETE FROM app.payments WHERE tenant_id = $1 AND invoice_id = $2`, r.tenant, r.invoice)
			balance := max(tc.total-tc.deposit, 0)
			r.e.exec(`UPDATE app.invoices SET total = $3, quote = quote || jsonb_build_object('total', $3::bigint, 'balanceDue', $4::bigint, 'depositPaid', $5::bigint, 'refundDue', $6::bigint)
				WHERE tenant_id = $1 AND id = $2`, r.tenant, r.invoice, tc.total, balance, tc.deposit, tc.refund)
			if tc.legacyQuote {
				r.e.exec(`UPDATE app.invoices SET quote = quote - 'depositPaid' - 'refundDue' WHERE tenant_id = $1 AND id = $2`, r.tenant, r.invoice)
			}
			if tc.partial > 0 {
				r.e.exec(`INSERT INTO app.payment_events (id, tenant_id, provider, external_id, amount, result, invoice_id, received_at)
					VALUES ('pe_tbl', $1, 'test', 'tbl-1', $2, 'PARTIAL', $3, now())`, r.tenant, tc.partial, r.invoice)
			}
			pp, _ := r.roomMapStay()["pendingPayment"].(map[string]any)
			_, raw := r.e.send("GET", "/v1/stays/"+r.stayID(), r.token, "", nil)
			gp, _ := parse(raw)["pendingPayment"].(map[string]any)
			for where, m := range map[string]map[string]any{"room map": pp, "getStay": gp} {
				if m == nil || m["paymentId"] != nil || num(m["total"]) != tc.total || num(m["received"]) != tc.partial || num(m["remaining"]) != tc.wantRemaining {
					t.Errorf("%s: %v, want total %d received %d remaining %d with no payment id", where, m, tc.total, tc.partial, tc.wantRemaining)
				}
			}
			_ = ctx
		})
	}
}
