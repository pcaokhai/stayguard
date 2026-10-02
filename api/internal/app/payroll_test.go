package app

import (
	"testing"
	"time"
)

func oct(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

// F-A4: shifts under paid or sick leave are paid like worked shifts, shifts under unpaid leave are not, and leave days are counted for any kind.
func TestLineFor_LeaveKindsAndPay_FA4(t *testing.T) {
	staff := PayrollStaff{UserID: "u1", Name: "Lan", Position: "FRONT_DESK", PayType: "PER_SHIFT", Rate: 250_000, Allowance: 100_000}
	var cells []RosterCell
	for d := 1; d <= 6; d++ {
		cells = append(cells, RosterCell{UserID: "u1", Date: oct(d), Shift: "MORNING"})
	}
	cells = append(cells, RosterCell{UserID: "u2", Date: oct(1), Shift: "MORNING"}) // somebody else's shift is not hers
	leave := []LeaveRow{
		{ID: "l1", UserID: "u1", Kind: "PAID", Status: "APPROVED", From: oct(2), To: oct(3)},
		{ID: "l2", UserID: "u1", Kind: "SICK", Status: "APPROVED", From: oct(4), To: oct(4)},
		{ID: "l3", UserID: "u1", Kind: "UNPAID", Status: "CANCEL_REQUESTED", From: oct(5), To: oct(5)},
		{ID: "l4", UserID: "u1", Kind: "PAID", Status: "PENDING", From: oct(6), To: oct(6)},
	}
	got, err := lineFor(staff, PayrollStored{Bonus: 50_000, Deduction: 20_000, Note: "ok"}, oct(1), oct(31), cells, leave)
	if err != nil {
		t.Fatal(err)
	}
	// worked: the 1st and the 6th (pending leave does not count); paid leave: the 2nd and 3rd; sick leave the 4th (paid too);
	// unpaid leave the 5th earns nothing.
	if got.ShiftsWorked != 2 || got.LeaveDays != 4 || got.EarnedPay != 250_000*5 || got.Net != 1_250_000+100_000+50_000-20_000 || got.Status != "UNPAID" ||
		got.Note == nil || *got.Note != "ok" {
		t.Fatalf("line: %+v", got)
	}
}

// Owner decision (docs/15 Q-02): a day of sick leave is paid like a worked shift, for a monthly and a per-shift employee alike.
func TestLineFor_OneSickDayIsPaid_FA4(t *testing.T) {
	var cells []RosterCell
	for d := 1; d <= 13; d++ {
		cells = append(cells, RosterCell{UserID: "u1", Date: oct(d), Shift: "MORNING"})
	}
	sick := []LeaveRow{{ID: "l1", UserID: "u1", Kind: "SICK", Status: "APPROVED", From: oct(13), To: oct(13)}}
	unpaid := []LeaveRow{{ID: "l1", UserID: "u1", Kind: "UNPAID", Status: "APPROVED", From: oct(13), To: oct(13)}}

	monthly := PayrollStaff{UserID: "u1", PayType: "MONTHLY", Rate: 7_800_000, StandardShifts: 26}
	for name, c := range map[string]struct {
		leave        []LeaveRow
		worked, earn int64
	}{
		"sick":   {sick, 12, 3_900_000},   // 12 worked + 1 sick = 13 of 26 shifts: half the salary
		"unpaid": {unpaid, 12, 3_600_000}, // the same day unpaid: 12 of 26
		"none":   {nil, 13, 3_900_000},
	} {
		got, err := lineFor(monthly, PayrollStored{}, oct(1), oct(31), cells, c.leave)
		if err != nil || int64(got.ShiftsWorked) != c.worked || got.EarnedPay != c.earn {
			t.Errorf("monthly, %s: worked %d earned %d (%v), want %d and %d", name, got.ShiftsWorked, got.EarnedPay, err, c.worked, c.earn)
		}
	}

	perShift := PayrollStaff{UserID: "u1", PayType: "PER_SHIFT", Rate: 250_000}
	got, err := lineFor(perShift, PayrollStored{}, oct(1), oct(31), cells, sick)
	if err != nil || got.ShiftsWorked != 12 || got.EarnedPay != 13*250_000 || got.LeaveDays != 1 {
		t.Errorf("per shift with a sick day: worked %d earned %d leave days %d (%v), want 12, %d, 1", got.ShiftsWorked, got.EarnedPay, got.LeaveDays, err, 13*250_000)
	}
	got, _ = lineFor(perShift, PayrollStored{}, oct(1), oct(31), cells, unpaid)
	if got.EarnedPay != 12*250_000 {
		t.Errorf("per shift with an unpaid day: earned %d, want %d", got.EarnedPay, 12*250_000)
	}
}
