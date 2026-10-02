// Package ticket holds the pure rules of a maintenance ticket (SG-1201): its states, what a damage report must say,
// and its total cost.
package ticket

import (
	"strings"
	"unicode/utf8"
)

// Ticket statuses; a ticket only moves forward.
const (
	New      = "NEW"
	InRepair = "IN_REPAIR"
	Done     = "DONE"
)

// Severities of a damage report: LockRoom takes the room out of service at once.
const (
	StillRentable = "STILL_RENTABLE"
	LockRoom      = "LOCK_ROOM"
)

const maxDescriptionRunes = 500

var categories = map[string]bool{"AIR_CONDITIONER": true, "HOT_WATER": true, "PLUMBING": true, "POWER_LIGHTS": true, "TV": true,
	"DOOR_LOCK": true, "MISSING_ITEMS": true, "OTHER": true}

var rank = map[string]int{New: 0, InRepair: 1, Done: 2}

// FieldError carries a field path and a code, never a value.
type FieldError struct{ Path, Code string }

// ValidStatus reports whether s is a ticket status.
func ValidStatus(s string) bool { _, ok := rank[s]; return ok }

// CanMove says whether a ticket may go from one status to another (the same status is a no-op).
func CanMove(from, to string) bool {
	f, okF := rank[from]
	t, okT := rank[to]
	return okF && okT && t >= f
}

// CheckReport lists what is wrong with a damage report.
func CheckReport(category, description, severity string) []FieldError {
	var errs []FieldError
	if !categories[category] {
		errs = append(errs, FieldError{"category", "PATTERN"})
	}
	switch n := utf8.RuneCountInString(strings.TrimSpace(description)); {
	case n == 0:
		errs = append(errs, FieldError{"description", "REQUIRED"})
	case utf8.RuneCountInString(description) > maxDescriptionRunes:
		errs = append(errs, FieldError{"description", "TOO_LONG"})
	}
	if severity != StillRentable && severity != LockRoom {
		errs = append(errs, FieldError{"severity", "PATTERN"})
	}
	return errs
}

// Total is parts plus labour; nil while neither cost is known.
func Total(parts, labour *int64) *int64 {
	if parts == nil && labour == nil {
		return nil
	}
	var sum int64
	if parts != nil {
		sum += *parts
	}
	if labour != nil {
		sum += *labour
	}
	return &sum
}
