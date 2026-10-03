//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func sumKeys(t *testing.T, raw any) (total int64) {
	t.Helper()
	for _, it := range raw.([]any) {
		total += int64(it.(map[string]any)["amount"].(float64))
	}
	return total
}

// F-A4 over HTTP and the database: payroll, tickets and manual lines all land in the month's expenses, and the report's
// totals reconcile with the paid invoices and the expense lines.
func TestFinanceE2E_ReportReconciles_FA4(t *testing.T) {
	cash := newPayRig(t, "A102")
	e, ctx := cash.e, context.Background()
	xfer := payRig{e: e, token: cash.token, tenant: cash.tenant, room: "A106"}.checkedOut()
	if st, p := cash.pay("CASH"); st != 201 || p["status"] != "PAID" {
		t.Fatalf("cash: %d %v", st, p)
	}
	_, p := xfer.pay("TRANSFER")
	if sim := e.call("POST", "/v1/demo/payments/"+p["id"].(string)+"/simulate", xfer.token, nil); sim.str("status") != "PAID" {
		t.Fatalf("simulate: %v", sim.body)
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := e.clock.Now().In(loc)
	month := now.Format("2006-01")
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	boss := cash.token

	// Manual lines: a recurring rent (last month's, which this month inherits) and one electricity bill (a replay under one key adds nothing).
	prev := now.AddDate(0, -1, 0).Format("2006-01")
	rent := map[string]any{"category": "RENT", "amount": 5000000, "month": prev, "recurring": true, "note": "rent"} // copied into this month when it is read
	key := newKey()
	if st, raw := e.send("POST", "/v1/owner/expenses", boss, key, rent); st != 201 || parse(raw)["source"] != "RECURRING" {
		t.Fatalf("rent: %d %s", st, raw)
	}
	if st, _ := e.send("POST", "/v1/owner/expenses", boss, key, rent); st != 201 {
		t.Fatalf("replay: %d", st)
	}
	power := map[string]any{"category": "ELECTRICITY", "amount": 300000, "month": month, "paidOn": now.Format("2006-01-02")}
	if st, raw := e.send("POST", "/v1/owner/expenses", boss, newKey(), power); st != 201 || parse(raw)["source"] != "MANUAL" {
		t.Fatalf("power: %d %s", st, raw)
	}
	if st, _ := e.send("POST", "/v1/owner/expenses", boss, newKey(), map[string]any{"category": "RENT", "amount": 1, "month": month, "paidOn": "2001-01-01"}); st != 422 {
		t.Fatalf("paid on a day outside the month: %d", st)
	}

	// A ticket finished with costs posts a MAINTENANCE line.
	var roomID string
	if err := e.owner.QueryRow(ctx, `SELECT id FROM app.units WHERE tenant_id = $1 AND code = 'A104'`, cash.tenant).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	st, raw := e.send("POST", "/v1/rooms/"+roomID+"/damage-reports", boss, newKey(), map[string]any{"category": "PLUMBING", "description": "leak", "severity": "STILL_RENTABLE"})
	ticket, _ := parse(raw)["id"].(string)
	if st != 201 {
		t.Fatalf("damage: %d %s", st, raw)
	}
	if st, raw = e.send("PATCH", "/v1/owner/maintenance-tickets/"+ticket, boss, "", map[string]any{"partsCost": 300000, "labourCost": 150000, "status": "DONE"}); st != 200 {
		t.Fatalf("ticket done: %d %s", st, raw)
	}

	// Buying stock in posts its cost as COST_OF_GOODS (10 x 20,000), once per restock.
	restock := map[string]any{"quantity": 10, "unitCost": 20000}
	rkey := newKey()
	for i := 0; i < 2; i++ { // the second call is the retry
		if st, raw := e.send("POST", "/v1/owner/services/WATER/restock", boss, rkey, restock); st != 200 && st != 201 {
			t.Fatalf("restock: %d %s", st, raw)
		}
	}

	// Payroll: 13 of 26 standard shifts on a 6,000,000 contract, 500,000 allowance, 200,000 bonus: 3,000,000 + 500,000 + 200,000.
	var lan string
	_ = e.owner.QueryRow(ctx, `SELECT id FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, cash.tenant).Scan(&lan)
	if lan == "" {
		e.demo("RECEPTIONIST", "vi", cash.tenant)
		_ = e.owner.QueryRow(ctx, `SELECT id FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, cash.tenant).Scan(&lan)
	}
	e.exec(`INSERT INTO app.staff_profiles (tenant_id, user_id, position, pay_type, rate, fixed_allowance, standard_shifts, start_date, annual_leave_days)
		VALUES ($1, $2, 'FRONT_DESK', 'MONTHLY', 6000000, 500000, 26, '2020-01-01', 12) ON CONFLICT (tenant_id, user_id) DO NOTHING`, cash.tenant, lan)
	e.exec(`INSERT INTO app.roster_assignments (tenant_id, user_id, work_date, shift)
		SELECT $1, $2, d::date, 'MORNING' FROM generate_series($3::date, $3::date + 12, interval '1 day') d`, cash.tenant, lan, first)
	st, raw = e.send("PATCH", "/v1/owner/payroll/"+month+"/lines/"+lan, boss, "", map[string]any{"bonus": 200000, "note": "good month"})
	line := parse(raw)
	if st != 200 || line["earnedPay"] != float64(3000000) || line["net"] != float64(3700000) || line["shiftsWorked"] != float64(13) {
		t.Fatalf("payroll line: %d %s", st, raw)
	}
	if st, _ = e.send("PATCH", "/v1/owner/payroll/"+month+"/lines/"+lan, boss, "", map[string]any{"deduction": 99000000}); st != 422 {
		t.Fatalf("a deduction above the gross pay: %d", st)
	}
	if st, raw = e.send("POST", "/v1/owner/payroll/"+month+"/mark-paid", boss, newKey(), map[string]any{}); st != 200 || parse(raw)["totalNet"] != float64(3700000) {
		t.Fatalf("mark paid: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/owner/payroll/"+month+"/mark-paid", boss, newKey(), map[string]any{}); st != 409 || parse(raw)["code"] != "PAYROLL_NOTHING_TO_PAY" {
		t.Fatalf("pay twice: %d %s", st, raw)
	}
	if st, raw = e.send("PATCH", "/v1/owner/payroll/"+month+"/lines/"+lan, boss, "", map[string]any{"bonus": 1}); st != 409 || parse(raw)["code"] != "PAYROLL_PAID" {
		t.Fatalf("change a paid line: %d %s", st, raw)
	}

	// The month's expenses: five lines, the system's lines cannot be edited, and the totals add up.
	st, raw = e.send("GET", "/v1/owner/expenses?month="+month, boss, "", nil)
	em := parse(raw)
	items, _ := em["items"].([]any)
	if st != 200 || len(items) != 5 || em["total"] != float64(5000000+300000+450000+3700000+200000) {
		t.Fatalf("expense month: %d %s", st, raw)
	}
	for _, it := range items {
		m := it.(map[string]any)
		if src := m["source"]; src == "PAYROLL" || src == "MAINTENANCE" {
			if st, r := e.send("PATCH", "/v1/owner/expenses/"+m["id"].(string), boss, "", power); st != 409 || parse(r)["code"] != "EXPENSE_AUTOMATIC" {
				t.Errorf("edit a %s line: %d %s", src, st, r)
			}
			if st, _ := e.send("DELETE", "/v1/owner/expenses/"+m["id"].(string), boss, "", nil); st != 409 {
				t.Errorf("delete a %s line: %d", src, st)
			}
		}
	}
	var invoiceTotal int64
	if err := e.owner.QueryRow(ctx, `SELECT coalesce(sum(total), 0) FROM app.invoices WHERE tenant_id = $1 AND status = 'PAID'`, cash.tenant).Scan(&invoiceTotal); err != nil || invoiceTotal == 0 {
		t.Fatalf("paid invoices: %d %v", invoiceTotal, err)
	}
	if em["revenue"] != float64(invoiceTotal) {
		t.Fatalf("expense month revenue %v, paid invoices %d", em["revenue"], invoiceTotal)
	}
	cats := int64(0)
	for _, c := range em["categories"].([]any) {
		cats += int64(c.(map[string]any)["amount"].(float64))
	}
	if cats != 9650000 {
		t.Fatalf("categories add up to %d", cats)
	}

	// The report reconciles with the invoices and the lines, and every revenue split adds up to the revenue.
	st, raw = e.send("GET", fmt.Sprintf("/v1/owner/reports/income-costs?from=%s&to=%s", month, month), boss, "", nil)
	rep := parse(raw)
	if st != 200 || rep["revenue"] != float64(invoiceTotal) || rep["expenses"] != float64(9650000) || rep["profit"] != float64(invoiceTotal-9650000) {
		t.Fatalf("report: %d %s", st, raw)
	}
	for _, split := range []string{"revenueByRentalType", "revenueByBuilding", "revenueByMethod"} {
		if got := sumKeys(t, rep[split]); got != invoiceTotal {
			t.Errorf("%s adds up to %d, revenue is %d", split, got, invoiceTotal)
		}
	}
	if got := sumKeys(t, rep["expensesByCategory"]); got != 9650000 {
		t.Errorf("expensesByCategory adds up to %d", got)
	}
	months := rep["months"].([]any)
	if m0 := months[0].(map[string]any); len(months) != 1 || m0["revenue"] != float64(invoiceTotal) || m0["expenses"] != float64(9650000) || m0["month"] != month {
		t.Errorf("months: %v", months)
	}
	if rep["occupancyPct"].(float64) <= 0 {
		t.Errorf("occupancy %v", rep["occupancyPct"])
	}
	if st, _ = e.send("GET", "/v1/owner/reports/income-costs?from=2026-05&to=2026-04", boss, "", nil); st != 422 {
		t.Fatalf("to before from: %d", st)
	}

	// The inherited rent is a RECURRING line; deleting this month's copy by hand keeps it deleted.
	var copyID string
	for _, it := range items {
		if m := it.(map[string]any); m["source"] == "RECURRING" {
			copyID, _ = m["id"].(string)
			if m["amount"] != float64(5000000) || m["category"] != "RENT" {
				t.Fatalf("copied line: %v", m)
			}
		}
	}
	if copyID == "" {
		t.Fatalf("the recurring rent was not copied: %s", raw)
	}
	if st, raw = e.send("PATCH", "/v1/owner/expenses/"+copyID, boss, "", map[string]any{"category": "RENT", "amount": 5000000, "month": month, "paidOn": now.Format("2006-01-02")}); st != 200 {
		t.Fatalf("edit a copy: %d %s", st, raw)
	}
	if st, _ = e.send("DELETE", "/v1/owner/expenses/"+copyID, boss, "", nil); st != 204 {
		t.Fatalf("delete a copy: %d", st)
	}
	st, raw = e.send("GET", "/v1/owner/expenses?month="+month, boss, "", nil)
	if left, _ := parse(raw)["items"].([]any); st != 200 || len(left) != 4 {
		t.Fatalf("a deleted copy came back: %d %s", st, raw)
	}
	desk := e.demo("RECEPTIONIST", "vi", cash.tenant).str("accessToken")
	if st, _ = e.send("GET", "/v1/owner/expenses?month="+month, desk, "", nil); st != 403 {
		t.Fatalf("a receptionist reads the expenses: %d", st)
	}
}
