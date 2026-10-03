//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
)

// dismissUnmatchedTransfer: the owner closes an unmatched inbound transfer with a note (a typo, a refund, money that is not ours).
func (r payRig) eventsOf(result string) int {
	return r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND provider = 'test' AND result = $2`, r.tenant, result)
}

func TestDismissUnmatchedTransfer_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	if res, err := r.handler().Settle(context.Background(), r.event("bank-d1", 77_000, "chuyen nham tien")); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("unmatched: %+v %v", res, err)
	}
	id := r.eventID("test", "bank-d1")
	url := "/v1/owner/payment-events/" + id + "/dismiss"
	note := map[string]any{"note": "wrong account, the guest asked for it back"}

	if st, raw := r.e.send("POST", url, r.token, newKey(), note); st != 403 {
		t.Fatalf("a receptionist dismissed money: %d %s", st, raw)
	}
	for name, body := range map[string]map[string]any{"no note": {}, "blank note": {"note": "   "}, "too long": {"note": strings.Repeat("x", 501)}} {
		if st, raw := r.e.send("POST", url, owner, newKey(), body); st != 422 {
			t.Fatalf("%s: %d %s, want 422", name, st, raw)
		}
	}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/nope/dismiss", owner, newKey(), note); st != 404 {
		t.Fatalf("unknown event: %d %s", st, raw)
	}
	if r.eventsOf("UNMATCHED") != 1 {
		t.Fatal("a refused dismissal changed the event")
	}

	key := newKey()
	st, first := r.e.send("POST", url, owner, key, note)
	if st != 200 || parse(first)["eventId"] != id || parse(first)["result"] != "DISMISSED" {
		t.Fatalf("dismiss: %d %s", st, first)
	}
	if st, again := r.e.send("POST", url, owner, key, note); st != 200 || string(again) != string(first) {
		t.Fatalf("the same key replays the same answer: %d %s", st, again)
	}
	if st, raw := r.e.send("POST", url, owner, newKey(), note); st != 409 {
		t.Fatalf("dismissing twice with a new key: %d %s", st, raw)
	}
	if r.eventsOf("DISMISSED") != 1 || r.eventsOf("UNMATCHED") != 0 {
		t.Fatalf("the event is DISMISSED (kept for audit): dismissed %d unmatched %d", r.eventsOf("DISMISSED"), r.eventsOf("UNMATCHED"))
	}
	if res, resolved, _ := r.alertRow("UNMATCHED_TRANSFER"); !resolved || res == nil || *res != "DISMISSED" || len(r.attention(owner, "UNMATCHED_TRANSFER")) != 0 {
		t.Fatalf("the alert is resolved as DISMISSED and leaves the overview: %v %v", resolved, res)
	}
	st, raw := r.e.send("GET", r.needsActionURL(), owner, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 0 {
		t.Fatalf("needs-action: %d %s", st, raw)
	}
	if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+id+"/link", owner, newKey(), map[string]any{"invoiceId": r.invoice}); st != 409 {
		t.Fatalf("a dismissed transfer cannot be linked: %d %s", st, raw)
	}
	if n := r.e.count(`SELECT count(*) FROM app.audit_logs WHERE tenant_id = $1 AND action = 'payment.dismissed' AND entity_id = $2`, r.tenant, id); n != 1 {
		t.Fatalf("audit rows payment.dismissed: %d, want 1", n)
	}
	if r.status("invoices", r.invoice) != "OPEN" {
		t.Fatal("dismissing money must not touch any invoice")
	}
}

// Only an unmatched inbound transfer can be dismissed: ignored (outgoing, other account) and settled events cannot.
func TestDismissUnmatchedTransfer_OnlyUnmatched_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	note := map[string]any{"note": "x"}
	r.e.exec(`INSERT INTO app.payment_events (id, tenant_id, provider, external_id, amount, result, received_at)
		VALUES ('pe_ign', $1, 'sepay', 'ign-1', 5000, 'IGNORED', now()), ('pe_set', $1, 'sepay', 'set-1', 5000, 'SETTLED', now())`, r.tenant)
	for _, id := range []string{"pe_ign", "pe_set"} {
		if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+id+"/dismiss", owner, newKey(), note); st != 409 {
			t.Fatalf("%s: %d %s, want 409", id, st, raw)
		}
	}
}
