// Package money holds the whole-VND amount type with checked arithmetic.
package money

import (
	"errors"
	"math"
)

var (
	ErrOverflow = errors.New("money: overflow")
	ErrNegative = errors.New("money: negative amount")
)

// Vnd is a whole-VND amount; floats and decimals are never used for money.
type Vnd int64

func NewVnd(n int64) (Vnd, error) {
	if n < 0 {
		return 0, ErrNegative
	}
	return Vnd(n), nil
}

func (v Vnd) Int64() int64 { return int64(v) }

// Add never wraps; amounts here are never negative, so only the upper bound is checked.
func Add(a, b Vnd) (Vnd, error) {
	if a < 0 || b < 0 {
		return 0, ErrNegative
	}
	if a > math.MaxInt64-b {
		return 0, ErrOverflow
	}
	return a + b, nil
}

func Mul(a Vnd, qty int64) (Vnd, error) {
	if a < 0 || qty < 0 {
		return 0, ErrNegative
	}
	if a == 0 || qty == 0 {
		return 0, nil
	}
	if int64(a) > math.MaxInt64/qty {
		return 0, ErrOverflow
	}
	return a * Vnd(qty), nil
}

func Sum(vs ...Vnd) (Vnd, error) {
	var total Vnd
	for _, v := range vs {
		var err error
		if total, err = Add(total, v); err != nil {
			return 0, err
		}
	}
	return total, nil
}

// Sub fails rather than going below zero, so settlement never produces a negative amount.
func Sub(a, b Vnd) (Vnd, error) {
	if a < 0 || b < 0 || b > a {
		return 0, ErrNegative
	}
	return a - b, nil
}
