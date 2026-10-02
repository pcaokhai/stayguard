package roster

import (
	"testing"
	"time"
)

func d(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }

func TestTransitions_SG1102(t *testing.T) {
	cases := []struct {
		action, from, want string
		ok                 bool
	}{
		{Approve, Pending, Approved, true},
		{Approve, CancelRequested, Cancelled, true},
		{Approve, Approved, "", false},
		{Approve, Declined, "", false},
		{Decline, Pending, Declined, true},
		{Decline, CancelRequested, Approved, true},
		{Decline, Approved, "", false},
		{Cancel, Pending, Cancelled, true},
		{Cancel, Approved, CancelRequested, true},
		{Cancel, CancelRequested, "", false},
		{Cancel, Cancelled, "", false},
		{Cancel, Declined, "", false},
	}
	for _, c := range cases {
		got, ok := Next(c.action, c.from)
		if ok != c.ok || got != c.want {
			t.Errorf("%s from %s: %q %v, want %q %v", c.action, c.from, got, ok, c.want, c.ok)
		}
	}
}

func TestGaps_UncoveredShiftsAreFlagged_SG1102(t *testing.T) {
	assign := []Assignment{
		{"lan", d(5), Morning}, {"lan", d(5), Afternoon}, {"ba", d(5), Night},
		{"lan", d(6), Morning}, {"ba", d(6), Afternoon},
	}
	leave := []Leave{
		{UserID: "lan", From: d(6), To: d(6), Status: Approved},              // morning of the 6th loses its only person
		{UserID: "ba", From: d(6), To: d(6), Status: Approved, Cover: "chi"}, // afternoon of the 6th is covered by chi
		{UserID: "ba", From: d(5), To: d(5), Shift: Night, Status: Pending},  // pending leave does not remove anyone
	}
	got := Gaps(d(5), d(6), d(5), assign, leave)
	want := map[Cell]bool{{d(6), Morning}: true, {d(6), Night}: true}
	if len(got) != len(want) {
		t.Fatalf("gaps %v, want %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected gap %v", g)
		}
	}
	// Days before today are history, not gaps.
	if past := Gaps(d(5), d(6), d(7), assign, leave); len(past) != 0 {
		t.Errorf("past gaps: %v", past)
	}
}

func TestDaysAndMonday_SG1102(t *testing.T) {
	if Days(d(5), d(5)) != 1 || Days(d(5), d(9)) != 5 {
		t.Error("inclusive day count")
	}
	if !IsMonday(d(5)) || IsMonday(d(6)) { // 5 October 2026 is a Monday
		t.Error("monday check")
	}
}

func TestCheckLeave_SG1103(t *testing.T) {
	has := func(errs []FieldError, path string) bool {
		for _, e := range errs {
			if e.Path == path {
				return true
			}
		}
		return false
	}
	if errs := CheckLeave(d(7), d(9), d(5), "PAID", "", "wedding", "u1", "u2"); len(errs) != 0 {
		t.Fatalf("valid: %v", errs)
	}
	if errs := CheckLeave(d(9), d(7), d(5), "PAID", "", "", "u1", ""); !has(errs, "toDate") {
		t.Errorf("to before from: %v", errs)
	}
	if errs := CheckLeave(d(3), d(4), d(5), "PAID", "", "", "u1", ""); !has(errs, "fromDate") {
		t.Errorf("in the past: %v", errs)
	}
	if errs := CheckLeave(d(7), d(8), d(5), "HOLIDAY", "EVENING", "", "u1", "u1"); !has(errs, "kind") || !has(errs, "shift") || !has(errs, "coverUserId") {
		t.Errorf("bad kind, shift and cover: %v", errs)
	}
	if errs := CheckLeave(d(7), d(7).AddDate(0, 0, MaxLeaveDays+1), d(5), "SICK", "", "", "u1", ""); !has(errs, "toDate") {
		t.Errorf("too long: %v", errs)
	}
}
