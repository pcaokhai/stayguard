// Package shift holds the pure cash rules of a front-desk shift (SG-503): expected cash, the counted
// total, what closing requires, and which roster shift a time belongs to.
package shift

import (
	"errors"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// Entry kinds of the cash ledger: the first two come into the drawer, the last two go out.
const (
	Deposit = "DEPOSIT"
	Payment = "PAYMENT"
	Refund  = "REFUND"
	Payout  = "PAYOUT"
)

// Shift codes (docs/15 rule 14).
const (
	Morning   = "MORNING"
	Afternoon = "AFTERNOON"
	Night     = "NIGHT"
)

// Denominations are the notes a drawer is counted in.
var Denominations = []int64{500_000, 200_000, 100_000, 50_000, 20_000, 10_000}

// maxNotes bounds one denomination's count, far above any real drawer, so counting cannot overflow.
const maxNotes = 1_000_000

var ErrInvalidCount = errors.New("shift: invalid cash count")

// FieldError carries a field path and a code, never a value.
type FieldError struct{ Path, Code string }

type Count struct{ Denomination, Quantity int64 }

// ExpectedCash is the opening float plus cash taken minus cash paid out. It can be negative when more
// went out than was there: that is a finding for the owner, not an error here.
func ExpectedCash(openingFloat, cashIn, cashOut int64) (int64, error) {
	f, err := money.NewVnd(openingFloat)
	if err != nil {
		return 0, err
	}
	i, err := money.NewVnd(cashIn)
	if err != nil {
		return 0, err
	}
	if _, err := money.NewVnd(cashOut); err != nil {
		return 0, err
	}
	total, err := money.Add(f, i)
	if err != nil {
		return 0, err
	}
	return total.Int64() - cashOut, nil
}

// CountedCash adds up the notes counted per denomination.
func CountedCash(counts []Count) (int64, error) {
	var sum money.Vnd
	for _, c := range counts {
		if !validDenomination(c.Denomination) || c.Quantity < 0 || c.Quantity > maxNotes {
			return 0, ErrInvalidCount
		}
		line, err := money.Mul(money.Vnd(c.Denomination), c.Quantity)
		if err != nil {
			return 0, err
		}
		if sum, err = money.Add(sum, line); err != nil {
			return 0, err
		}
	}
	return sum.Int64(), nil
}

func validDenomination(d int64) bool {
	for _, x := range Denominations {
		if x == d {
			return true
		}
	}
	return false
}

// CheckClose lists what stops a close: a difference needs a reason, and the float left for the next shift
// cannot be more than the cash counted.
func CheckClose(counted, expected, floatLeft int64, reason string) []FieldError {
	var errs []FieldError
	if counted != expected && strings.TrimSpace(reason) == "" {
		errs = append(errs, FieldError{"reason", "REQUIRED"})
	}
	if floatLeft < 0 {
		errs = append(errs, FieldError{"floatLeft", "MIN"})
	} else if floatLeft > counted {
		errs = append(errs, FieldError{"floatLeft", "MAX"})
	}
	return errs
}

// CodeFor names the roster shift an instant falls in, by the hour in the instant's own zone.
func CodeFor(at time.Time) string {
	switch h := at.Hour(); {
	case h >= 6 && h < 14:
		return Morning
	case h >= 14 && h < 22:
		return Afternoon
	}
	return Night
}
