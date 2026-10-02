package room

import "errors"

// ErrNotVacant: only a VACANT room can be checked into.
var ErrNotVacant = errors.New("room: not vacant")

// Status transitions live in this file; later stories add checkout and cleaning here.

// CheckIn moves a VACANT room to OCCUPIED.
func CheckIn(stored Status) (Status, error) {
	if stored != StatusVacant {
		return "", ErrNotVacant
	}
	return StatusOccupied, nil
}

// ErrNotToClean: only a TO_CLEAN room can be marked clean.
var ErrNotToClean = errors.New("room: not waiting to be cleaned")

// Clean moves a TO_CLEAN room to VACANT. A VACANT room stays VACANT (completing twice is not an error).
func Clean(stored Status) (Status, error) {
	if stored != StatusToClean && stored != StatusVacant {
		return "", ErrNotToClean
	}
	return StatusVacant, nil
}

// ErrNotOccupied: only an OCCUPIED room can be left by its guest.
var ErrNotOccupied = errors.New("room: not occupied")

// MoveOut moves an OCCUPIED room to TO_CLEAN when its guest moves to another room.
func MoveOut(stored Status) (Status, error) {
	if stored != StatusOccupied {
		return "", ErrNotOccupied
	}
	return StatusToClean, nil
}

// ErrOccupied: a room with a guest in it cannot be locked for maintenance.
var ErrOccupied = errors.New("room: occupied")

// LockForMaintenance takes a room out of service. A VACANT or TO_CLEAN room is locked at once; one with a guest is refused.
func LockForMaintenance(stored Status) (Status, error) {
	if stored == StatusOccupied {
		return "", ErrOccupied
	}
	return StatusMaintenance, nil
}

// Reopen puts a room under maintenance back in service as VACANT; any other status is left as it is.
func Reopen(stored Status) Status {
	if stored == StatusMaintenance {
		return StatusVacant
	}
	return stored
}
