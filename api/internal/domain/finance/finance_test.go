package finance

import (
	"testing"
	"time"
)

// F-A4: payroll per pay type, with the rounding rule (whole VND, half up).
func TestEarned_PerPayType_FA4(t *testing.T) {
	cases := []struct {
		name string
		c    Contract
		w    Work
		want int64
	}{
		{"monthly, full month", Contract{PayType: Monthly, Rate: 7_000_000, StandardShifts: 26}, Work{Shifts: 26}, 7_000_000},
		{"monthly, more than standard is not paid extra", Contract{PayType: Monthly, Rate: 7_000_000, StandardShifts: 26}, Work{Shifts: 30}, 7_000_000},
		{"monthly, half the shifts", Contract{PayType: Monthly, Rate: 7_000_000, StandardShifts: 26}, Work{Shifts: 13}, 3_500_000},
		{"monthly, rounds down below half", Contract{PayType: Monthly, Rate: 7_000_000, StandardShifts: 26}, Work{Shifts: 25}, 6_730_769},
		{"monthly, rounds half up", Contract{PayType: Monthly, Rate: 1, StandardShifts: 2}, Work{Shifts: 1}, 1},
		{"monthly, paid leave counts as worked", Contract{PayType: Monthly, Rate: 6_000_000, StandardShifts: 30}, Work{Shifts: 20, PaidLeaveShifts: 5}, 5_000_000},
		{"monthly, nothing worked", Contract{PayType: Monthly, Rate: 6_000_000, StandardShifts: 30}, Work{}, 0},
		{"monthly, no standard shifts is a flat salary", Contract{PayType: Monthly, Rate: 5_000_000}, Work{Shifts: 3}, 5_000_000},
		{"per shift", Contract{PayType: PerShift, Rate: 250_000}, Work{Shifts: 20, PaidLeaveShifts: 2}, 5_500_000},
		{"hourly, eight hours a shift", Contract{PayType: Hourly, Rate: 30_000}, Work{Shifts: 10}, 2_400_000},
	}
	for _, c := range cases {
		got, err := Earned(c.c, c.w)
		if err != nil || got != c.want {
			t.Errorf("%s: %d %v, want %d", c.name, got, err, c.want)
		}
	}
	if _, err := Earned(Contract{PayType: "WEEKLY", Rate: 1}, Work{}); err == nil {
		t.Error("unknown pay type accepted")
	}
	if _, err := Earned(Contract{PayType: PerShift, Rate: 1 << 62}, Work{Shifts: 10}); err == nil {
		t.Error("overflow accepted")
	}
}

func TestNet_AllowanceBonusAndDeduction_FA4(t *testing.T) {
	if got := Net(5_000_000, 300_000, 200_000, 100_000); got != 5_400_000 {
		t.Errorf("net %d", got)
	}
	if got := Net(100, 0, 0, 500); got != 0 {
		t.Errorf("a deduction cannot make the net negative: %d", got)
	}
	if MaxDeduction(5_000_000, 300_000, 200_000) != 5_500_000 {
		t.Error("max deduction is the gross pay")
	}
}

func TestMonths_FA4(t *testing.T) {
	first, last, err := MonthDays("2026-02")
	if err != nil || !first.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) || !last.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("february: %v %v %v", first, last, err)
	}
	for _, bad := range []string{"2026-13", "2026-1", "26-01", "", "2026-00"} {
		if _, _, err := MonthDays(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	got, err := MonthsBetween("2026-11", "2027-02")
	if err != nil || len(got) != 4 || got[0] != "2026-11" || got[3] != "2027-02" {
		t.Fatalf("months %v %v", got, err)
	}
	if _, err := MonthsBetween("2026-05", "2026-04"); err == nil {
		t.Error("to before from accepted")
	}
	if _, err := MonthsBetween("2020-01", "2026-01"); err == nil {
		t.Error("a range longer than 36 months accepted")
	}
	if MonthOf(time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)) != "2026-10" || NextMonth("2026-12") != "2027-01" || PrevMonth("2027-01") != "2026-12" {
		t.Error("month arithmetic")
	}
}

func TestPercentages_FA4(t *testing.T) {
	if MarginPct(1_000_000, 750_000) != 25 || MarginPct(0, 5) != 0 || MarginPct(3, 1) != 66.7 || MarginPct(100, 150) != -50 {
		t.Errorf("margin: %v %v %v %v", MarginPct(1_000_000, 750_000), MarginPct(0, 5), MarginPct(3, 1), MarginPct(100, 150))
	}
	if OccupancyPct(15, 20) != 75 || OccupancyPct(1, 0) != 0 || OccupancyPct(30, 20) != 100 {
		t.Error("occupancy")
	}
}

func TestExpenseRules_FA4(t *testing.T) {
	if !IsAutomatic(Payroll) || !IsAutomatic(MaintenanceSrc) || !IsAutomatic(Stock) || IsAutomatic(Manual) || IsAutomatic(Recurring) {
		t.Error("automatic sources")
	}
	ok := func(cat string, amount int64, month, paidOn, note string) []FieldError {
		return CheckExpense(cat, amount, month, paidOn, note)
	}
	if errs := ok("RENT", 5_000_000, "2026-10", "2026-10-05", "october rent"); len(errs) != 0 {
		t.Fatalf("valid: %v", errs)
	}
	for name, errs := range map[string][]FieldError{
		"category": ok("FUN", 1, "2026-10", "", ""), "amount": ok("RENT", -1, "2026-10", "", ""), "month": ok("RENT", 1, "2026-13", "", ""),
		"paidOn": ok("RENT", 1, "2026-10", "2026-11-01", ""), "note": ok("RENT", 1, "2026-10", "", string(make([]byte, 400))),
	} {
		found := false
		for _, e := range errs {
			found = found || e.Path == name
		}
		if !found {
			t.Errorf("%s not reported: %v", name, errs)
		}
	}
}
