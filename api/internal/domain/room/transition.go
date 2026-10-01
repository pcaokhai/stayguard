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
