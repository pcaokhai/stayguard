package pricing

import (
	"errors"
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

var ErrInvalidExtra = errors.New("pricing: extra quantity must be at least 1")

type Extra struct {
	ServiceCode string
	Quantity    int64
	UnitAmount  money.Vnd
}

type Bill struct {
	StayTotal, ExtrasTotal, Total, BalanceDue, RefundDue money.Vnd
}

// Assemble adds extras to the stay and settles against the deposit; at most one of
// BalanceDue and RefundDue is positive.
func Assemble(stay Quote, extras []Extra, deposit money.Vnd) (Bill, error) {
	if deposit < 0 {
		return Bill{}, fmt.Errorf("deposit: %w", money.ErrNegative)
	}
	amounts := make([]money.Vnd, len(extras))
	for i, e := range extras {
		if e.Quantity < 1 {
			return Bill{}, fmt.Errorf("%w: %s", ErrInvalidExtra, e.ServiceCode)
		}
		a, err := money.Mul(e.UnitAmount, e.Quantity)
		if err != nil {
			return Bill{}, fmt.Errorf("extra %s: %w", e.ServiceCode, err)
		}
		amounts[i] = a
	}
	extrasTotal, err := money.Sum(amounts...)
	if err != nil {
		return Bill{}, fmt.Errorf("extras total: %w", err)
	}
	total, err := money.Add(stay.Total, extrasTotal)
	if err != nil {
		return Bill{}, fmt.Errorf("bill total: %w", err)
	}
	b := Bill{StayTotal: stay.Total, ExtrasTotal: extrasTotal, Total: total}
	if total > deposit {
		b.BalanceDue, err = money.Sub(total, deposit)
	} else {
		b.RefundDue, err = money.Sub(deposit, total)
	}
	if err != nil {
		return Bill{}, fmt.Errorf("settlement: %w", err)
	}
	return b, nil
}
