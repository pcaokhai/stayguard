//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func (e *env) roomIDByCode(tenant, code string) string {
	e.t.Helper()
	var id string
	if err := e.owner.QueryRow(context.Background(), `SELECT id FROM app.units WHERE tenant_id = $1 AND code = $2`, tenant, code).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func methodShares(t *testing.T, rep map[string]any) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, it := range rep["revenueByMethod"].([]any) {
		m := it.(map[string]any)
		out[m["key"].(string)] = int64(m["amount"].(float64))
	}
	return out
}

func (e *env) monthOf() string {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	return e.clock.Now().In(loc).Format("2006-01")
}

// Follow-up 1: cash taken by the owner (no drawer, so no shift) is recorded, listed with no shift, and counted in revenue by method.
func TestOwnerCash_NoShiftButNeverDropped_FU1(t *testing.T) {
	r := newPayRig(t, "A102") // the owner checked in with a 100,000 cash deposit and checks out
	e := r.e
	if st, p := r.pay("CASH"); st != 201 || p["status"] != "PAID" {
		t.Fatalf("cash: %d %v", st, p)
	}
	if n := e.count(`SELECT count(*) FROM app.shifts WHERE tenant_id = $1`, r.tenant); n != 0 {
		t.Fatalf("the owner opened a shift: %d", n)
	}
	month := e.monthOf()
	day := e.clock.Now().In(time.UTC).Format("2006-01-02")
	st, raw := e.send("GET", fmt.Sprintf("/v1/owner/transactions?from=%s&to=%s&filter=CASH", day, day), r.token, "", nil)
	items, _ := parse(raw)["items"].([]any)
	if st != 200 || len(items) != 1 {
		t.Fatalf("transactions: %d %s", st, raw)
	}
	tx := items[0].(map[string]any)
	if tx["reconciliation"] != "CASH" || tx["shiftId"] != nil || tx["amount"].(float64) <= 0 {
		t.Fatalf("a cash payment with no shift must show as CASH with no shift id: %v", tx)
	}
	st, raw = e.send("GET", fmt.Sprintf("/v1/owner/reports/income-costs?from=%s&to=%s", month, month), r.token, "", nil)
	rep := parse(raw)
	var total int64
	_ = e.owner.QueryRow(context.Background(), `SELECT total FROM app.invoices WHERE id = $1`, r.invoice).Scan(&total)
	if st != 200 || methodShares(t, rep)["CASH"] != total || rep["revenue"] != float64(total) {
		t.Fatalf("revenue by method %v, invoice total %d: %s", methodShares(t, rep), total, raw)
	}
}

// Follow-up 2: revenue by method adds up to revenue with a deposit refund, a deposit plus balance in cash, and a transfer.
func TestRevenueByMethod_Reconciles_FU2(t *testing.T) {
	cash := newPayRig(t, "A102")
	e := cash.e
	xfer := payRig{e: e, token: cash.token, tenant: cash.tenant, room: "A106"}.checkedOut()
	if st, _ := cash.pay("CASH"); st != 201 {
		t.Fatal("cash payment")
	}
	_, p := xfer.pay("TRANSFER")
	if sim := e.call("POST", "/v1/demo/payments/"+p["id"].(string)+"/simulate", xfer.token, nil); sim.str("status") != "PAID" {
		t.Fatalf("simulate: %v", sim.body)
	}
	// A third stay whose deposit is more than the bill: the rest is refunded in cash.
	e.clock.set(e.start)
	st, raw := e.send("POST", "/v1/rooms/"+e.roomIDByCode(cash.tenant, "A105")+"/stays", cash.token, newKey(),
		stayBody(map[string]any{"rentalType": "HOURLY", "deposit": 5000000}))
	stay, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	e.clock.set(e.start.Add(40 * time.Minute))
	st, raw = e.checkout(cash.token, stay, newKey())
	inv := parse(raw)
	if q := inv["quote"].(map[string]any); st != 201 || q["refundDue"].(float64) <= 0 {
		t.Fatalf("check-out with a refund: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/invoices/"+inv["id"].(string)+"/payments", cash.token, newKey(), map[string]any{"method": "CASH"}); st != 201 {
		t.Fatalf("refund payment: %d %s", st, raw)
	}
	month := e.monthOf()
	st, raw = e.send("GET", fmt.Sprintf("/v1/owner/reports/income-costs?from=%s&to=%s", month, month), cash.token, "", nil)
	rep := parse(raw)
	var invoices int64
	_ = e.owner.QueryRow(context.Background(), `SELECT coalesce(sum(total), 0) FROM app.invoices WHERE tenant_id = $1 AND status = 'PAID'`, cash.tenant).Scan(&invoices)
	var sum int64
	shares := methodShares(t, rep)
	for _, v := range shares {
		sum += v
	}
	if st != 200 || invoices == 0 || sum != invoices || rep["revenue"] != float64(invoices) || shares["TRANSFER"] <= 0 || shares["CASH"] <= 0 {
		t.Fatalf("methods %v add up to %d, revenue %v, paid invoices %d", shares, sum, rep["revenue"], invoices)
	}
}
