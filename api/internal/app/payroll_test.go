package app

import (
	"testing"
	"time"
)

func oct(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

// F-A4: shifts under paid leave are paid, shifts under sick or unpaid leave are not, and leave days are counted for any kind.
func TestLineFor_LeaveKindsAndPay_FA4(t *testing.T) {
	staff := PayrollStaff{UserID: "u1", Name: "Lan", Position: "FRONT_DESK", PayType: "PER_SHIFT", Rate: 250_000, Allowance: 100_000}
	var cells []RosterCell
	for d := 1; d <= 6; d++ {
		cells = append(cells, RosterCell{"u1", oct(d), "MORNING"})
	}
	cells = append(cells, RosterCell{"u2", oct(1), "MORNING"}) // somebody else's shift is not hers
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
	// worked: the 1st and the 6th (pending leave does not count); paid leave: the 2nd and 3rd; sick and unpaid leave earn nothing.
	if got.ShiftsWorked != 2 || got.LeaveDays != 4 || got.EarnedPay != 250_000*4 || got.Net != 1_000_000+100_000+50_000-20_000 || got.Status != "UNPAID" ||
		got.Note == nil || *got.Note != "ok" {
		t.Fatalf("line: %+v", got)
	}
}
