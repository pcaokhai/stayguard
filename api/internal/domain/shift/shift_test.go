package shift

import (
	"testing"
	"time"
)

// SG-503: expected cash is float plus cash taken (deposits, balances) minus cash paid out (refunds, payouts).
func TestExpectedCash_DepositsRefundsAndPayouts_SG503(t *testing.T) {
	cases := []struct {
		name                         string
		float, deposit, paid, refund int64
		payout, want                 int64
	}{
		{"nothing happened", 500_000, 0, 0, 0, 0, 500_000},
		{"deposit and balance", 500_000, 200_000, 300_000, 0, 0, 1_000_000},
		{"refund of a deposit", 500_000, 200_000, 0, 50_000, 0, 650_000},
		{"payout for ice", 500_000, 0, 100_000, 0, 30_000, 570_000},
		{"all of it", 100_000, 300_000, 150_000, 100_000, 20_000, 430_000},
	}
	for _, c := range cases {
		got, err := ExpectedCash(c.float, c.deposit+c.paid, c.refund+c.payout)
		if err != nil || got != c.want {
			t.Errorf("%s: %d %v, want %d", c.name, got, err, c.want)
		}
	}
	if _, err := ExpectedCash(1<<62, 1<<62, 0); err == nil {
		t.Error("overflow accepted")
	}
}

func TestCountedCash_SG503(t *testing.T) {
	got, err := CountedCash([]Count{{500_000, 2}, {20_000, 3}, {10_000, 0}})
	if err != nil || got != 1_060_000 {
		t.Fatalf("%d %v", got, err)
	}
	for name, c := range map[string][]Count{"unknown denomination": {{1000, 1}}, "negative": {{10_000, -1}}, "huge": {{500_000, 1 << 40}}} {
		if _, err := CountedCash(c); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCheckClose_ReasonAndFloat_SG503(t *testing.T) {
	has := func(errs []FieldError, path string) bool {
		for _, e := range errs {
			if e.Path == path {
				return true
			}
		}
		return false
	}
	if errs := CheckClose(1_000_000, 1_000_000, 200_000, ""); len(errs) != 0 {
		t.Errorf("exact count needs no reason: %v", errs)
	}
	if errs := CheckClose(990_000, 1_000_000, 200_000, "  "); !has(errs, "reason") {
		t.Errorf("a difference needs a reason: %v", errs)
	}
	if errs := CheckClose(990_000, 1_000_000, 200_000, "dropped a note"); len(errs) != 0 {
		t.Errorf("reason given: %v", errs)
	}
	if errs := CheckClose(100_000, 100_000, 200_000, ""); !has(errs, "floatLeft") {
		t.Errorf("float above the counted cash: %v", errs)
	}
}

func TestCodeFor_LocalHour_SG503(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	at := func(h int) time.Time { return time.Date(2026, 10, 2, h, 0, 0, 0, loc) }
	for h, want := range map[int]string{5: "NIGHT", 6: "MORNING", 13: "MORNING", 14: "AFTERNOON", 21: "AFTERNOON", 22: "NIGHT", 0: "NIGHT"} {
		if got := CodeFor(at(h)); got != want {
			t.Errorf("hour %d: %s want %s", h, got, want)
		}
	}
}
