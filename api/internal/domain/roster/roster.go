// Package roster holds the pure rules of the duty roster and leave requests (SG-1102, SG-1103): the leave status
// machine, which shifts are left uncovered, and what a leave request must say. Dates are calendar days at midnight UTC.
package roster

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Shift codes (docs/15 rule 14): MORNING 06-14, AFTERNOON 14-22, NIGHT 22-06.
const (
	Morning   = "MORNING"
	Afternoon = "AFTERNOON"
	Night     = "NIGHT"
)

// Shifts lists the codes in day order.
var Shifts = []string{Morning, Afternoon, Night}

// Leave kinds and statuses. TAKEN is derived (approved leave whose last day has passed), never stored.
const (
	KindPaid, KindSick, KindUnpaid = "PAID", "SICK", "UNPAID"

	Pending         = "PENDING"
	Approved        = "APPROVED"
	Declined        = "DECLINED"
	Cancelled       = "CANCELLED"
	CancelRequested = "CANCEL_REQUESTED"
	Taken           = "TAKEN"
)

// Actions on a leave request.
const (
	Approve = "APPROVE"
	Decline = "DECLINE"
	Cancel  = "CANCEL" // by the requester
)

const (
	// MaxLeaveDays bounds one request.
	MaxLeaveDays   = 60
	maxReasonRunes = 300
)

// FieldError carries a field path and a code, never a value.
type FieldError struct{ Path, Code string }

type Assignment struct {
	UserID string
	Date   time.Time
	Shift  string
}

// Leave is a leave request as far as coverage is concerned; Shift is empty for the whole day.
type Leave struct {
	UserID, Shift, Status, Cover string
	From, To                     time.Time
}

// Cell is one shift of one day.
type Cell struct {
	Date  time.Time
	Shift string
}

func ValidShift(s string) bool { return s == Morning || s == Afternoon || s == Night }

// Next is the status a request moves to when an action is taken on it; ok is false when the action does not apply.
func Next(action, from string) (string, bool) {
	switch action {
	case Approve:
		switch from {
		case Pending:
			return Approved, true
		case CancelRequested:
			return Cancelled, true
		}
	case Decline:
		switch from {
		case Pending:
			return Declined, true
		case CancelRequested:
			return Approved, true // the owner refused to let the leave go: it stands
		}
	case Cancel:
		switch from {
		case Pending:
			return Cancelled, true
		case Approved:
			return CancelRequested, true
		}
	}
	return "", false
}

// Days counts the days from first to last, both included.
func Days(first, last time.Time) int { return int(last.Sub(first)/(24*time.Hour)) + 1 }

// IsMonday: a roster week starts on Monday.
func IsMonday(d time.Time) bool { return d.Weekday() == time.Monday }

// Covers says whether a leave takes its person off the shift of a day: only leave that stands does.
func (l Leave) Covers(c Cell) bool {
	standing := l.Status == Approved || l.Status == CancelRequested || l.Status == Taken
	return standing && !c.Date.Before(l.From) && !c.Date.After(l.To) && (l.Shift == "" || l.Shift == c.Shift)
}

// Gaps lists the shifts from first to last (not before today) with nobody on duty: a person on standing leave is off,
// and the cover named on their leave takes their place.
func Gaps(first, last, today time.Time, assignments []Assignment, leave []Leave) []Cell {
	var out []Cell
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if day.Before(today) {
			continue
		}
		for _, code := range Shifts {
			if !staffed(Cell{day, code}, assignments, leave) {
				out = append(out, Cell{day, code})
			}
		}
	}
	return out
}

func staffed(c Cell, assignments []Assignment, leave []Leave) bool {
	assigned := map[string]bool{}
	for _, a := range assignments {
		if a.Date.Equal(c.Date) && a.Shift == c.Shift {
			assigned[a.UserID] = true
		}
	}
	off, cover := map[string]bool{}, map[string]bool{}
	for _, l := range leave {
		if l.Covers(c) {
			off[l.UserID] = true
			if l.Cover != "" && assigned[l.UserID] { // the cover takes the place of the person who would have worked
				cover[l.Cover] = true
			}
		}
	}
	for id := range assigned {
		if !off[id] {
			return true
		}
	}
	for id := range cover {
		if !off[id] {
			return true
		}
	}
	return false
}

// CheckLeave lists what is wrong with a leave request: dates in order, not in the past, at most MaxLeaveDays, a known
// kind, an empty or known shift, a short reason, and a cover who is somebody else.
func CheckLeave(from, to, today time.Time, kind, shift, reason, userID, cover string) []FieldError {
	var errs []FieldError
	if from.Before(today) {
		errs = append(errs, FieldError{"fromDate", "MIN"})
	}
	switch {
	case to.Before(from):
		errs = append(errs, FieldError{"toDate", "MIN"})
	case Days(from, to) > MaxLeaveDays:
		errs = append(errs, FieldError{"toDate", "MAX"})
	}
	if kind != KindPaid && kind != KindSick && kind != KindUnpaid {
		errs = append(errs, FieldError{"kind", "PATTERN"})
	}
	if shift != "" && !ValidShift(shift) {
		errs = append(errs, FieldError{"shift", "PATTERN"})
	}
	if utf8.RuneCountInString(strings.TrimSpace(reason)) > maxReasonRunes {
		errs = append(errs, FieldError{"reason", "TOO_LONG"})
	}
	if cover != "" && cover == userID {
		errs = append(errs, FieldError{"coverUserId", "PATTERN"})
	}
	return errs
}
