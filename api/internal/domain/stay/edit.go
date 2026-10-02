package stay

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	pathNewCheckIn = "newCheckInAt"
	pathReason     = "reasonCode"
	pathNote       = "note"

	// MaxCheckInDelay is how much later than the originally recorded time a correction may move check-in.
	MaxCheckInDelay = 60 * time.Minute
	minNoteRunes    = 3
	maxNoteRunes    = 300
)

// CheckInReasons are the reason codes the contract accepts for a check-in correction.
var CheckInReasons = map[string]bool{"WRONG_TIME": true, "LATE_ARRIVAL": true, "OTHER": true}

// ValidateCheckInEdit applies docs/15 rule 3: the new time is at most MaxCheckInDelay after the time first
// recorded and never after now (the server clock), and a reason code and note are required.
func ValidateCheckInEdit(original, newAt, now time.Time, reason, note string) error {
	var errs []FieldError
	if newAt.After(now) || newAt.After(original.Add(MaxCheckInDelay)) {
		errs = append(errs, FieldError{pathNewCheckIn, CodeMax})
	}
	if !CheckInReasons[reason] {
		errs = append(errs, FieldError{pathReason, CodePattern})
	}
	switch n := utf8.RuneCountInString(strings.TrimSpace(note)); {
	case n < minNoteRunes:
		errs = append(errs, FieldError{pathNote, CodeTooShort})
	case utf8.RuneCountInString(note) > maxNoteRunes:
		errs = append(errs, FieldError{pathNote, CodeTooLong})
	}
	if len(errs) == 0 {
		return nil
	}
	return newValidationError(errs)
}
