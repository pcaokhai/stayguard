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
