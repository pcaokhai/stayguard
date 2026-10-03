//go:build integration

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func (r payRig) eventID(provider, external string) string {
	var id string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payment_events WHERE tenant_id = $1 AND provider = $2 AND external_id = $3`, r.tenant, provider, external).Scan(&id); err != nil {
		r.e.t.Fatal(err)
	}
	return id
}

// Outgoing money and money to another account are kept for audit and dedupe (result IGNORED) but are never "unmatched money": not in
// the list of unmatched transfers, not in needs-action, no alert, never linkable, and the owner's figures do not move.
func TestWebhook_OutgoingAndOtherAccountMoneyIsIgnored_FU(t *testing.T) {
	r := deskRig(t)
	r.connectHook(hookSecret)
	owner := r.ownerToken()
	now := r.e.clock.Now()
	alerts := r.e.count(`SELECT count(*) FROM app.alerts WHERE tenant_id = $1`, r.tenant)
	cashBefore := r.ownerCash(owner)

	out := r.sepayBody(t, 3001, "out", r.balance, "CK "+r.code)
	other := bytes.Replace(r.sepayBody(t, 3002, "in", r.balance, "CK "+r.code), []byte(`"0000000000"`), []byte(`"9999999999"`), 1)
	for name, body := range map[string][]byte{"outgoing": out, "other account": other} {
		for i := 0; i < 2; i++ { // the second delivery is a duplicate
			if st, resp := r.webhook(hookID, hookSecret, now, body, ""); st != 200 || resp != `{"success": true}` {
				t.Fatalf("%s delivery %d: %d %s", name, i, st, resp)
			}
		}
	}
	if r.events("IGNORED") != 2 || r.events("UNMATCHED") != 0 {
		t.Fatalf("ignored %d, unmatched %d: the two events are kept as IGNORED, nothing else", r.events("IGNORED"), r.events("UNMATCHED"))
	}
	if n := r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND external_id IN ('3001', '3002')`, r.tenant); n != 2 {
		t.Fatalf("a duplicate delivery must not store twice: %d rows", n)
	}
	if r.e.count(`SELECT count(*) FROM app.alerts WHERE tenant_id = $1`, r.tenant) != alerts {
		t.Fatal("ignored money raises no alert")
	}
	if len(r.transactions(owner)) != 0 {
		t.Fatalf("ignored money is not in the transactions list: %v", r.transactions(owner))
	}
	st, raw := r.e.send("GET", r.needsActionURL(), owner, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 0 {
		t.Fatalf("needs-action must not count ignored money: %d %s", st, raw)
	}
	if got := r.ownerCash(owner); got != cashBefore || r.status("invoices", r.invoice) != "OPEN" {
		t.Fatalf("ignored money moved the owner's cash (%d to %d) or the invoice", cashBefore, got)
	}
	for name, id := range map[string]string{"outgoing": r.eventID("sepay", "3001"), "other account": r.eventID("sepay", "3002")} {
		if st, raw := r.e.send("POST", "/v1/owner/payment-events/"+id+"/link", owner, newKey(), map[string]any{"invoiceId": r.invoice}); st != 409 {
			t.Fatalf("linking %s money: %d %s, want 409", name, st, raw)
		}
	}
	if r.status("invoices", r.invoice) != "OPEN" {
		t.Fatal("a refused link changed the invoice")
	}
}

// An inbound transfer to the tenant's own account with no bill code is still UNMATCHED, with its alert, in the list and in needs-action.
func TestWebhook_InboundWithNoBillCodeStaysUnmatched_FU(t *testing.T) {
	r := deskRig(t)
	r.connectHook(hookSecret)
	owner := r.ownerToken()
	body := r.sepayBody(t, 3101, "in", r.balance, "chuyen tien khong ro "+strings.Repeat("x", 3))
	if st, resp := r.webhook(hookID, hookSecret, r.e.clock.Now(), body, ""); st != 200 {
		t.Fatalf("webhook: %d %s", st, resp)
	}
	if r.events("UNMATCHED") != 1 || len(r.alerts("UNMATCHED_TRANSFER")) != 1 || len(r.attention(owner, "UNMATCHED_TRANSFER")) != 1 {
		t.Fatal("an inbound transfer nobody can match is UNMATCHED, with its alert")
	}
	st, raw := r.e.send("GET", r.needsActionURL(), owner, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 1 {
		t.Fatalf("needs-action: %d %s", st, raw)
	}
}

func (r payRig) needsActionURL() string {
	day := r.e.clock.Now().In(time.UTC).Format("2006-01-02")
	next := r.e.clock.Now().In(time.UTC).Add(24 * time.Hour).Format("2006-01-02")
	return "/v1/owner/transactions?from=" + day + "&to=" + next + "&filter=NEEDS_ACTION"
}

func (r payRig) ownerCash(owner string) int64 {
	day := r.e.clock.Now().In(time.UTC).Format("2006-01-02")
	st, raw := r.e.send("GET", "/v1/owner/overview?date="+day, owner, "", nil)
	if st != 200 {
		r.e.t.Fatalf("overview: %d %s", st, raw)
	}
	return num(parse(raw)["cashExpected"])
}

// A transfer whose note holds the pending bill's code followed by a digit (another bill's code, not pending) is not that bill's money:
// it stays UNMATCHED, with its alert, and the invoice stays open.
func TestSettle_ContentWithALongerCodeDoesNotLandOnThePendingBill_FU(t *testing.T) {
	r := deskRig(t)
	res, err := r.handler().Settle(context.Background(), r.event("bank-char1", r.balance, "CK "+r.code+"2 thanh toan"))
	if err != nil || res.Result != "UNMATCHED" || r.status("invoices", r.invoice) != "OPEN" || len(r.alerts("UNMATCHED_TRANSFER")) != 1 {
		t.Fatalf("the money must not land on the pending bill: %+v %v (invoice %s)", res, err, r.status("invoices", r.invoice))
	}
}
