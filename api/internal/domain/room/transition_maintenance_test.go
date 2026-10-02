package room

import (
	"errors"
	"testing"
)

func TestLockForMaintenance_NotWithAGuest_SG1201(t *testing.T) {
	for _, s := range []Status{StatusVacant, StatusToClean, StatusMaintenance} {
		if got, err := LockForMaintenance(s); err != nil || got != StatusMaintenance {
			t.Errorf("%s: %v %v", s, got, err)
		}
	}
	if _, err := LockForMaintenance(StatusOccupied); !errors.Is(err, ErrOccupied) {
		t.Errorf("occupied: %v", err)
	}
}

func TestReopen_OnlyMaintenanceBecomesVacant_SG1201(t *testing.T) {
	if Reopen(StatusMaintenance) != StatusVacant {
		t.Error("maintenance must reopen as vacant")
	}
	for _, s := range []Status{StatusOccupied, StatusToClean, StatusVacant} {
		if Reopen(s) != s {
			t.Errorf("%s changed", s)
		}
	}
}
