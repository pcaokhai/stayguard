package pricing

import (
	"errors"
	"math"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

func TestBillArithmetic_SG101_AC5(t *testing.T) {
	stay := Quote{Total: 100000}
	water := []Extra{{ServiceCode: "WATER", Quantity: 2, UnitAmount: 10000}}

	t.Run("deposit equal to total leaves nothing either way", func(t *testing.T) {
		b, err := Assemble(stay, water, 120000)
		if err != nil || b.BalanceDue != 0 || b.RefundDue != 0 || b.Total != 120000 || b.ExtrasTotal != 20000 {
			t.Fatalf("got %+v, %v", b, err)
		}
	})
	t.Run("larger deposit is refunded", func(t *testing.T) {
		b, err := Assemble(stay, water, 150000)
		if err != nil || b.BalanceDue != 0 || b.RefundDue != 30000 {
			t.Fatalf("got %+v, %v", b, err)
		}
	})
	t.Run("smaller deposit leaves a balance", func(t *testing.T) {
		b, err := Assemble(stay, water, 50000)
		if err != nil || b.BalanceDue != 70000 || b.RefundDue != 0 {
			t.Fatalf("got %+v, %v", b, err)
		}
	})
	for _, q := range []int64{0, -1} {
		t.Run("rejects extra quantity", func(t *testing.T) {
			_, err := Assemble(stay, []Extra{{ServiceCode: "WATER", Quantity: q, UnitAmount: 10}}, 0)
			if !errors.Is(err, ErrInvalidExtra) {
				t.Fatalf("quantity %d: err = %v", q, err)
			}
		})
	}
	t.Run("extra multiplication overflow", func(t *testing.T) {
		_, err := Assemble(stay, []Extra{{ServiceCode: "X", Quantity: 2, UnitAmount: math.MaxInt64}}, 0)
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("extras sum overflow", func(t *testing.T) {
		big := Extra{ServiceCode: "X", Quantity: 1, UnitAmount: math.MaxInt64}
		if _, err := Assemble(stay, []Extra{big, big}, 0); !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("stay plus extras overflow", func(t *testing.T) {
		_, err := Assemble(Quote{Total: math.MaxInt64}, water, 0)
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("never both positive", func(t *testing.T) {
		for _, dep := range []money.Vnd{0, 1, 99999, 100000, 100001, 500000} {
			b, err := Assemble(stay, nil, dep)
			if err != nil || (b.BalanceDue > 0 && b.RefundDue > 0) {
				t.Fatalf("deposit %d: %+v, %v", dep, b, err)
			}
		}
	})
}
