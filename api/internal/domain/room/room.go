// Package room derives the visible room status from stored facts. It is pure:
// instants are parameters and the clock is never read.
package room

import (
	"encoding/json"
	"fmt"
	"time"
)

type Status string

const (
	StatusVacant      Status = "VACANT"
	StatusOccupied    Status = "OCCUPIED"
	StatusOverdue     Status = "OVERDUE"
	StatusToClean     Status = "TO_CLEAN"
	StatusMaintenance Status = "MAINTENANCE"
)

type RentalType string

const (
	Hourly    RentalType = "HOURLY"
	Overnight RentalType = "OVERNIGHT"
	Daily     RentalType = "DAILY"
)

const clockLayout = "15:04"

// ParseStored rejects OVERDUE because it is derived, never stored.
func ParseStored(s string) (Status, error) {
	switch st := Status(s); st {
	case StatusVacant, StatusOccupied, StatusToClean, StatusMaintenance:
		return st, nil
	}
	return "", fmt.Errorf("room: unknown stored status %q", s)
}

func ParseRentalType(s string) (RentalType, error) {
	switch rt := RentalType(s); rt {
	case Hourly, Overnight, Daily:
		return rt, nil
	}
	return "", fmt.Errorf("room: unknown rental type %q", s)
}

// StayTiming is the part of a stay and its rate-plan snapshot that overdue
// derivation needs. WindowEnd is empty for HOURLY.
type StayTiming struct {
	RentalType   RentalType
	CheckInAt    time.Time
	WindowEnd    string
	GraceMinutes int
}

type snapshotWindow struct {
	WindowEnd *string `json:"windowEnd"`
}

type snapshot struct {
	GraceMinutes *int           `json:"graceMinutes"`
	Overnight    snapshotWindow `json:"overnight"`
	Daily        snapshotWindow `json:"daily"`
}

// ParseTiming reads the rate-plan snapshot; anything unreadable is an error so
// a broken snapshot can never pass as "not overdue".
func ParseTiming(rt RentalType, checkIn time.Time, raw []byte) (StayTiming, error) {
	if _, err := ParseRentalType(string(rt)); err != nil {
		return StayTiming{}, err
	}
	var s snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return StayTiming{}, fmt.Errorf("room: rate plan snapshot: %w", err)
	}
	if s.GraceMinutes == nil || *s.GraceMinutes < 0 {
		return StayTiming{}, fmt.Errorf("room: rate plan snapshot: invalid graceMinutes")
	}
	t := StayTiming{RentalType: rt, CheckInAt: checkIn, GraceMinutes: *s.GraceMinutes}
	var w *string
	switch rt {
	case Overnight:
		w = s.Overnight.WindowEnd
	case Daily:
		w = s.Daily.WindowEnd
	case Hourly:
		return t, nil
	}
	if w == nil {
		return StayTiming{}, fmt.Errorf("room: rate plan snapshot: missing %s windowEnd", rt)
	}
	if _, err := time.Parse(clockLayout, *w); err != nil {
		return StayTiming{}, fmt.Errorf("room: rate plan snapshot: windowEnd %q: %w", *w, err)
	}
	t.WindowEnd = *w
	return t, nil
}

// ExpectedEnd is the first WindowEnd wall-clock instant in loc strictly after
// check-in, so a check-in exactly at the window end expects the next day.
func ExpectedEnd(t StayTiming, loc *time.Location) (time.Time, error) {
	if t.RentalType != Overnight && t.RentalType != Daily {
		return time.Time{}, fmt.Errorf("room: %q has no expected end", t.RentalType)
	}
	clock, err := time.Parse(clockLayout, t.WindowEnd)
	if err != nil {
		return time.Time{}, fmt.Errorf("room: windowEnd %q: %w", t.WindowEnd, err)
	}
	y, m, d := t.CheckInAt.In(loc).Date()
	end := time.Date(y, m, d, clock.Hour(), clock.Minute(), 0, 0, loc)
	if !end.After(t.CheckInAt) {
		end = time.Date(y, m, d+1, clock.Hour(), clock.Minute(), 0, 0, loc)
	}
	return end, nil
}

// Derive turns the stored status into the visible one: OCCUPIED overnight or
// daily stays become OVERDUE strictly after expected end plus grace.
func Derive(stored Status, stay *StayTiming, now time.Time, loc *time.Location) (Status, error) {
	if _, err := ParseStored(string(stored)); err != nil {
		return "", err
	}
	if stored != StatusOccupied || stay == nil || stay.RentalType == Hourly {
		return stored, nil
	}
	end, err := ExpectedEnd(*stay, loc)
	if err != nil {
		return "", err
	}
	if now.After(end.Add(time.Duration(stay.GraceMinutes) * time.Minute)) {
		return StatusOverdue, nil
	}
	return stored, nil
}

// ElapsedMinutes is whole minutes since check-in, never negative.
func ElapsedMinutes(checkIn, now time.Time) int {
	return max(0, int(now.Sub(checkIn)/time.Minute))
}

type Counts struct {
	Vacant, Occupied, Overdue, ToClean, Maintenance int
}

func Count(statuses []Status) Counts {
	var c Counts
	for _, s := range statuses {
		switch s {
		case StatusVacant:
			c.Vacant++
		case StatusOccupied:
			c.Occupied++
		case StatusOverdue:
			c.Overdue++
		case StatusToClean:
			c.ToClean++
		case StatusMaintenance:
			c.Maintenance++
		}
	}
	return c
}
