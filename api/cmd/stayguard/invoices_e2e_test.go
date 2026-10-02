//go:build integration

package main

import (
	"fmt"
	"testing"
	"time"
)

func invoiceIDs(t *testing.T, raw []byte) (ids []string, items []map[string]any) {
	t.Helper()
	for _, it := range parse(raw)["items"].([]any) {
		m := it.(map[string]any)
		items = append(items, m)
		ids = append(ids, m["invoiceId"].(string))
	}
	return ids, items
}

// listInvoices: owner only, tenant-scoped, the balance follows money the bank reported, and the amount puts the match first.
func TestListInvoices_Candidates_SG903(t *testing.T) {
	a := newPayRig(t, "A102")
	e := a.e
	b := payRig{e: e, token: a.token, tenant: a.tenant, room: "A106"}.checkedOut()
	if st, _ := a.pay("TRANSFER"); st != 201 {
		t.Fatal("transfer payment")
	}
	// A partial payment on A: the bank reported 40,000 under A's bill code, which does not settle the invoice.
	if res, err := a.handler().Settle(t.Context(), a.event("bank-partial", 40_000, a.code)); err != nil || res.Result != "MISMATCH" {
		t.Fatalf("partial transfer: %+v %v", res, err)
	}

	st, raw := e.send("GET", "/v1/owner/invoices?status=UNPAID", a.token, "", nil)
	ids, items := invoiceIDs(t, raw)
	if st != 200 || len(ids) != 2 {
		t.Fatalf("unpaid invoices: %d %s", st, raw)
	}
	byID := map[string]map[string]any{}
	for _, m := range items {
		byID[m["invoiceId"].(string)] = m
	}
	ma, mb := byID[a.invoice], byID[b.invoice]
	// Both stays had a 100,000 deposit; A also has 40,000 reported, so its balance is 40,000 lower than the bill after the deposit.
	if ma["billCode"] != a.code || ma["roomCode"] != "A102" || ma["guestName"] == "" || ma["checkedOutAt"] == nil ||
		ma["paid"] != float64(140_000) || ma["balance"] != ma["total"].(float64)-140_000 || ma["balance"] != float64(a.balance-40_000) {
		t.Fatalf("invoice with a partial payment: %v (balance due was %d)", ma, a.balance)
	}
	if mb["paid"] != float64(100_000) || mb["balance"] != float64(b.balance) {
		t.Fatalf("invoice with nothing reported: %v (balance due %d)", mb, b.balance)
	}

	// The amount puts the invoices whose balance equals it first.
	first := func(amount int64) string {
		_, raw := e.send("GET", fmt.Sprintf("/v1/owner/invoices?amount=%d", amount), a.token, "", nil)
		ids, _ := invoiceIDs(t, raw)
		return ids[0]
	}
	if first(int64(b.balance)) != b.invoice {
		t.Error("the invoice whose balance equals the amount must come first (B)")
	}
	if first(int64(a.balance-40_000)) != a.invoice {
		t.Error("the partial payment's remaining balance must match A")
	}

	// The stay history carries the invoice id and bill code of a checked-out stay.
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	day := e.clock.Now().In(loc).Format("2006-01-02")
	st, raw = e.send("GET", "/v1/stays?date="+day+"&q=A102", a.token, "", nil)
	if list, _ := parse(raw)["items"].([]any); st != 200 || len(list) != 1 || list[0].(map[string]any)["invoiceId"] != a.invoice || list[0].(map[string]any)["billCode"] != a.code {
		t.Fatalf("stay list invoice fields: %d %s", st, raw)
	}

	// A paid invoice leaves the list; roles and tenants are respected.
	if st, _ := b.pay("CASH"); st != 201 {
		t.Fatal("cash payment")
	}
	if _, raw = e.send("GET", "/v1/owner/invoices", a.token, "", nil); len(parse(raw)["items"].([]any)) != 1 {
		t.Fatalf("a paid invoice is still listed: %s", raw)
	}
	desk := e.demo("RECEPTIONIST", "vi", a.tenant).str("accessToken")
	if st, _ := e.send("GET", "/v1/owner/invoices", desk, "", nil); st != 403 {
		t.Fatalf("a receptionist lists invoices: %d", st)
	}
	other := e.demo("OWNER", "vi", "").str("accessToken") // another guesthouse
	if st, raw := e.send("GET", "/v1/owner/invoices", other, "", nil); st != 200 || len(parse(raw)["items"].([]any)) != 0 {
		t.Fatalf("another tenant sees invoices: %d %s", st, raw)
	}
	if st, _ := e.send("GET", "/v1/owner/invoices?amount=0", a.token, "", nil); st != 422 {
		t.Fatalf("amount 0: %d", st)
	}
}
