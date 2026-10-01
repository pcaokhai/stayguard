package money

import (
	"errors"
	"math"
	"testing"
)

func TestSubChecked_SG101_AC5(t *testing.T) {
	if v, err := Sub(10, 4); err != nil || v != 6 {
		t.Fatalf("10-4 = %v, %v", v, err)
	}
	if v, err := Sub(5, 5); err != nil || v != 0 {
		t.Fatalf("5-5 = %v, %v", v, err)
	}
	for _, c := range [][2]Vnd{{4, 10}, {-1, 0}, {0, -1}, {0, math.MinInt64}} {
		if _, err := Sub(c[0], c[1]); !errors.Is(err, ErrNegative) {
			t.Fatalf("Sub(%d,%d) err = %v", c[0], c[1], err)
		}
	}
}
