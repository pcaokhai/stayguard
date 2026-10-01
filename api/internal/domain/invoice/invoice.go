// Package invoice holds the pure rules of an invoice: its status and the bill code.
package invoice

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusOpen Status = "OPEN"
	StatusPaid Status = "PAID"
)

// ParseStatus fails closed on anything but the two stored statuses.
func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case StatusOpen, StatusPaid:
		return st, nil
	}
	return "", errors.New("invoice: unknown status")
}

// ErrBillCode: no bill code can be built (empty room code once reduced, or attempt below 1).
var ErrBillCode = errors.New("invoice: cannot build bill code")

const (
	billPrefix = "PH"
	// MaxBillCodeLen is the cap the plan promises (Ruling 3): PH + MMDD + 8 + a 2 digit attempt fits.
	MaxBillCodeLen = 20
	maxRoomPart    = 8
)

// BillCode is PH, month and day of day (already in the tenant zone), the room code reduced to
// ASCII uppercase letters and digits (at most 8), and from the second attempt on the attempt number.
func BillCode(day time.Time, roomCode string, attempt int) (string, error) {
	if attempt < 1 {
		return "", fmt.Errorf("%w: attempt", ErrBillCode)
	}
	room := reduceRoom(roomCode)
	if room == "" {
		return "", fmt.Errorf("%w: room code", ErrBillCode)
	}
	code := billPrefix + day.Format("0102") + room
	if attempt > 1 {
		code += fmt.Sprint(attempt)
	}
	if len(code) > MaxBillCodeLen {
		return "", fmt.Errorf("%w: too long", ErrBillCode)
	}
	return code, nil
}

func reduceRoom(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > maxRoomPart {
		out = out[:maxRoomPart]
	}
	return out
}
