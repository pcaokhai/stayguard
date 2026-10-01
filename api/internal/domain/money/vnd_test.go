package money

import (
	"errors"
	"math"
	"testing"
)

func TestVndChecked_SG101_AC3(t *testing.T) {
	max := Vnd(math.MaxInt64)

	t.Run("new", func(t *testing.T) {
		if _, err := NewVnd(-1); !errors.Is(err, ErrNegative) {
			t.Fatalf("negative input: got %v", err)
		}
		v, err := NewVnd(math.MaxInt64)
		if err != nil || v.Int64() != math.MaxInt64 {
			t.Fatalf("exact max: got %v, %v", v, err)
		}
		if v, err := NewVnd(0); err != nil || v != 0 {
			t.Fatalf("zero: got %v, %v", v, err)
		}
	})

	t.Run("add", func(t *testing.T) {
		cases := []struct {
			name    string
			a, b    Vnd
			want    Vnd
			wantErr error
		}{
			{"small", 80000, 20000, 100000, nil},
			{"exact max", max - 1, 1, max, nil},
			{"max plus zero", max, 0, max, nil},
			{"max plus one", max, 1, 0, ErrOverflow},
			{"negative operand", -1, 5, 0, ErrNegative},
		}
		for _, c := range cases {
			got, err := Add(c.a, c.b)
			if !errors.Is(err, c.wantErr) || got != c.want {
				t.Errorf("%s: got %v, %v; want %v, %v", c.name, got, err, c.want, c.wantErr)
			}
		}
	})

	t.Run("mul", func(t *testing.T) {
		cases := []struct {
			name    string
			a       Vnd
			qty     int64
			want    Vnd
			wantErr error
		}{
			{"small", 50000, 3, 150000, nil},
			{"qty zero", max, 0, 0, nil},
			{"zero price huge qty", 0, math.MaxInt64, 0, nil},
			{"exact max", max, 1, max, nil},
			{"max times two", max, 2, 0, ErrOverflow},
			{"half boundary ok", Vnd(math.MaxInt64 / 2), 2, Vnd(math.MaxInt64/2) * 2, nil},
			{"half boundary over", Vnd(math.MaxInt64/2 + 1), 2, 0, ErrOverflow},
			{"negative qty", 10, -1, 0, ErrNegative},
			{"negative price", -10, 1, 0, ErrNegative},
		}
		for _, c := range cases {
			got, err := Mul(c.a, c.qty)
			if !errors.Is(err, c.wantErr) || got != c.want {
				t.Errorf("%s: got %v, %v; want %v, %v", c.name, got, err, c.want, c.wantErr)
			}
		}
	})

	t.Run("sum", func(t *testing.T) {
		if got, err := Sum(); err != nil || got != 0 {
			t.Errorf("empty: got %v, %v", got, err)
		}
		if got, err := Sum(1, 2, 3); err != nil || got != 6 {
			t.Errorf("small: got %v, %v", got, err)
		}
		if got, err := Sum(max-1, 1); err != nil || got != max {
			t.Errorf("exact max: got %v, %v", got, err)
		}
		if _, err := Sum(max, 1); !errors.Is(err, ErrOverflow) {
			t.Errorf("overflow: got %v", err)
		}
		if _, err := Sum(1, -1); !errors.Is(err, ErrNegative) {
			t.Errorf("negative: got %v", err)
		}
	})
}
