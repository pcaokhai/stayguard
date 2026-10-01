package room

import (
	"errors"
	"testing"
)

func TestRoomCheckIn_SG203_AC1(t *testing.T) {
	for _, s := range []Status{StatusVacant, StatusOccupied, StatusOverdue, StatusToClean, StatusMaintenance} {
		got, err := CheckIn(s)
		if s == StatusVacant {
			if err != nil || got != StatusOccupied {
				t.Errorf("VACANT: got %s err=%v", got, err)
			}
			continue
		}
		if !errors.Is(err, ErrNotVacant) || got != "" {
			t.Errorf("%s: got %q err=%v, want ErrNotVacant", s, got, err)
		}
	}
}
