//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// deskRig checks a guest in at the front desk (a 100,000 deposit opens the desk's shift), checks out two hours later and
// asks for a transfer payment, so the invoice is open with a balance.
func deskRig(t *testing.T) payRig {
	t.Helper()
	e := newSeededEnv(t)
	tenant := e.demo("OWNER", "vi", "").str("tenantId")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	var roomID string
	if err := e.owner.QueryRow(context.Background(), `SELECT id FROM app.units WHERE tenant_id = $1 AND code = 'A102'`, tenant).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+roomID+"/stays", desk, newKey(), stayBody(nil))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(2 * time.Hour))
	st, raw = e.checkout(desk, stay, newKey())
	inv := parse(raw)
	if st != 201 {
		t.Fatalf("check-out: %d %s", st, raw)
	}
	r := payRig{e: e, token: desk, tenant: tenant, room: "A102"}
	r.invoice, _ = inv["id"].(string)
	r.code, _ = inv["billCode"].(string)
	r.balance = num(inv["quote"].(map[string]any)["balanceDue"])
	if r.balance <= 0 {
		t.Fatalf("balance %d", r.balance)
	}
	if st, p := r.pay("TRANSFER"); st != 201 {
		t.Fatalf("transfer payment: %d %v", st, p)
	}
	return r
}

func (r payRig) runJobsAt(at time.Time) {
	var buf bytes.Buffer
	if err := runJobs(context.Background(), r.e.jobs(), []string{r.tenant}, at, &buf); err != nil {
		r.e.t.Fatalf("jobs: %v\n%s", err, buf.String())
	}
}

// Anti-loss: bank money on an invoice that is still not fully paid 15 minutes after the first partial event raises one
// PAYMENT_PARTIAL alert for the owner, with the remaining amount.
func TestPartialTransfer_AlertAfter15Minutes_FU(t *testing.T) {
	r := deskRig(t)
	first := r.balance * 3 / 10
	at := r.e.clock.Now()
	if res, err := r.handler().Settle(context.Background(), r.event("bank-a1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	r.runJobsAt(at.Add(14*time.Minute + 59*time.Second))
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 0 {
		t.Fatalf("an alert before 15 minutes: %d", n)
	}
	r.runJobsAt(at.Add(15 * time.Minute))
	got := r.alerts("PAYMENT_PARTIAL")
	if len(got) != 1 || got[0] != r.balance-first {
		t.Fatalf("one alert for the remaining amount %d, got %v", r.balance-first, got)
	}
	r.runJobsAt(at.Add(time.Hour))
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 1 {
		t.Fatalf("the job must not raise the same alert twice: %d", n)
	}
	st, raw := r.e.send("GET", "/v1/owner/alerts?kind=PAYMENT_PARTIAL", r.e.demo("OWNER", "vi", r.tenant).str("accessToken"), "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 1 {
		t.Fatalf("the owner sees it: %d %s", st, raw)
	}
}

// An invoice that is paid in time never raises it.
func TestPartialTransfer_PaidInTimeRaisesNoAlert_FU(t *testing.T) {
	r := deskRig(t)
	at := r.e.clock.Now()
	h := r.handler()
	if res, err := h.Settle(context.Background(), r.event("bank-b1", r.balance/2, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	if res, err := h.Settle(context.Background(), r.event("bank-b2", r.balance-r.balance/2, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("rest: %+v %v", res, err)
	}
	r.runJobsAt(at.Add(time.Hour))
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 0 {
		t.Fatalf("a paid invoice raised an alert: %d", n)
	}
}

// Anti-loss at shift close: the shift lists its invoices that are not fully paid with the amounts left, and closing needs a
// reason while the list is not empty, even when the cash matches.
func TestShiftClose_ListsUnpaidInvoicesAndNeedsReason_FU(t *testing.T) {
	r := deskRig(t)
	first := r.balance / 4
	if res, err := r.handler().Settle(context.Background(), r.event("bank-c1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	st, raw := r.e.send("GET", "/v1/shifts/current", r.token, "", nil)
	list, _ := parse(raw)["unpaidInvoices"].([]any)
	if st != 200 || len(list) != 1 {
		t.Fatalf("shift: %d %s", st, raw)
	}
	row := list[0].(map[string]any)
	if row["invoiceId"] != r.invoice || row["billCode"] != r.code || num(row["balance"]) != r.balance-first {
		t.Fatalf("the unpaid invoice with what is left (%d): %v", r.balance-first, row)
	}
	body := map[string]any{"counts": []map[string]any{{"denomination": 100000, "quantity": 1}}, "floatLeft": 0}
	if st, raw = r.e.send("POST", "/v1/shifts/current/close", r.token, newKey(), body); st != 422 {
		t.Fatalf("closing with an unpaid invoice and no reason: %d %s", st, raw)
	}
	body["reason"] = "guest sends the rest tomorrow"
	st, raw = r.e.send("POST", "/v1/shifts/current/close", r.token, newKey(), body)
	review := parse(raw)
	closed, _ := review["shift"].(map[string]any)["unpaidInvoices"].([]any)
	if st != 200 || review["shift"].(map[string]any)["status"] != "CLOSED" || len(closed) != 1 {
		t.Fatalf("closing with a reason: %d %s", st, raw)
	}
}

// The 15-minute timer runs on the server clock at the moment the event arrived, never on SePay's own transaction date (Vietnam
// time without a zone, here 7 hours off).
func TestPartialTransfer_TimerUsesServerReceiveTime_FU(t *testing.T) {
	r := newPayRig(t, "A102")
	r.connectHook(hookSecret)
	if st, p := r.pay("TRANSFER"); st != 201 {
		t.Fatalf("transfer: %d %v", st, p)
	}
	at := r.e.clock.Now()
	body := r.sepayBody(t, 2001, "in", r.balance/2, "CK "+r.code)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	m["transactionDate"] = at.UTC().Format("2006-01-02 15:04:05") // read as Vietnam time this is 7 hours in the past
	body, _ = json.Marshal(m)
	if st, out := r.webhook(hookID, hookSecret, at, body, ""); st != 200 {
		t.Fatalf("webhook: %d %s", st, out)
	}
	if n := r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND result = 'PARTIAL' AND received_at = $2`, r.tenant, at.UTC()); n != 1 {
		t.Fatalf("received_at must be the server clock %s", at.UTC())
	}
	r.runJobsAt(at.Add(14*time.Minute + 59*time.Second))
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 0 {
		t.Fatalf("an alert before 15 minutes of server time: %d", n)
	}
	r.runJobsAt(at.Add(15 * time.Minute))
	if n := len(r.alerts("PAYMENT_PARTIAL")); n != 1 {
		t.Fatalf("one alert at 15 minutes: %d", n)
	}
}
