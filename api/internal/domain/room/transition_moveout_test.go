package room

import (
	"errors"
	"testing"
)

func TestMoveOut_OnlyOccupiedBecomesToClean_SG801_AC4(t *testing.T) {
	if got, err := MoveOut(StatusOccupied); err != nil || got != StatusToClean {
		t.Fatalf("occupied: %v %v", got, err)
	}
	for _, s := range []Status{StatusVacant, StatusToClean, StatusMaintenance} {
		if _, err := MoveOut(s); !errors.Is(err, ErrNotOccupied) {
			t.Errorf("%s: %v", s, err)
		}
	}
}
