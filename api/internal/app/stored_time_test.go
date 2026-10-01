package app

import (
	"testing"
	"time"
)

func TestStoredTime_SG205(t *testing.T) {
	in := time.Date(2026, 10, 1, 5, 0, 0, 123456789, time.FixedZone("x", 7*3600))
	got := storedTime(in)
	if got.Nanosecond() != 123456000 || got.Location() != time.UTC || !got.Equal(in.Truncate(time.Microsecond)) {
		t.Errorf("storedTime = %v", got)
	}
}
