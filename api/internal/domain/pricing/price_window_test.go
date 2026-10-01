package pricing

import (
	"errors"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

func planWith(grace int, first, extra, night, daily money.Vnd, nStart, nEnd, dStart, dEnd Clock) RatePlan {
	return RatePlan{
		Version: 1, Currency: CurrencyVND, GraceMinutes: grace,
		Hourly:    Hourly{FirstHour: first, ExtraHour: extra},
		Overnight: Window{Price: night, Start: nStart, End: nEnd},
		Daily:     Window{Price: daily, Start: dStart, End: dEnd},
	}
}

func clk(h, m int) Clock { return Clock{Hour: h, Minute: m} }

func TestPriceFollowsPlanWindowsAndGrace_SG101_AC1(t *testing.T) {
	loc := time.FixedZone("ICT", 7*60*60)
	at := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, loc) }
	// Plans differ from the generator's 21/12/14/15 so a hardcoded engine fails.
	// Rates: first hour 50000, extra hour 10000, overnight 150000, daily 250000.
	g0 := planWith(0, 50000, 10000, 150000, 250000, clk(22, 0), clk(11, 0), clk(13, 0), clk(11, 0))
	g60 := planWith(60, 50000, 10000, 150000, 250000, clk(22, 0), clk(11, 0), clk(13, 0), clk(11, 0))
	// Non-crossing overnight window 00:00-12:00 with the standard-like rates.
	flat := planWith(15, 80000, 20000, 200000, 300000, clk(0, 0), clk(12, 0), clk(14, 0), clk(12, 0))

	// Start equal to End is a 24h window: 12:00 to 12:00 next day.
	full := planWith(15, 80000, 20000, 200000, 300000, clk(12, 0), clk(12, 0), clk(14, 0), clk(12, 0))

	cases := []struct {
		name    string
		plan    RatePlan
		rental  RentalType
		in, out time.Time
		total   money.Vnd
		capped  bool
	}{
		{"hourly grace 0, 61 min: 1 minute past the hour bills 1 block = 50000+10000", g0, RentalHourly, at(5, 10, 0), at(5, 11, 1), 60000, false},
		{"hourly grace 0, exactly 60 min", g0, RentalHourly, at(5, 10, 0), at(5, 11, 0), 50000, false},
		{"hourly grace 60, 119 min: remainder 59 <= 60, no extra", g60, RentalHourly, at(5, 10, 0), at(5, 11, 59), 50000, false},
		{"overnight 22-11, in 21:00 is 60 min early: 150000+10000", g0, RentalOvernight, at(5, 21, 0), at(6, 11, 0), 160000, false},
		{"overnight 22-11, out 11:30 is 30 min late, grace 0: 150000+10000", g0, RentalOvernight, at(5, 22, 0), at(6, 11, 30), 160000, false},
		{"overnight 22-11, grace 60, out 11:59 is inside grace", g60, RentalOvernight, at(5, 22, 0), at(6, 11, 59), 150000, false},
		{"daily 13-11, in 13:00 out 11:00 next day: one day", g0, RentalDaily, at(5, 13, 0), at(6, 11, 0), 250000, false},
		{"daily 13-11, in 12:00 is 60 min early: 250000+10000", g0, RentalDaily, at(5, 12, 0), at(6, 11, 0), 260000, false},
		{"daily 13-11, out 12:00 is 60 min late: 250000+10000", g0, RentalDaily, at(5, 13, 0), at(6, 12, 0), 260000, false},
		{"24h window 12:00-12:00, in 13:00 out 12:00 next day: no fees", full, RentalOvernight, at(5, 13, 0), at(6, 12, 0), 200000, false},
		// Non-crossing window: the night is day 5 00:00 to 12:00.
		{"non-crossing, inside the window, no fees: 200000", flat, RentalOvernight, at(5, 1, 0), at(5, 11, 0), 200000, false},
		// 34h stay: late = 12:00 day 5 to 11:00 day 6 = 23h = 23 blocks * 20000 = 460000;
		// 200000 + 460000 = 660000 exceeds daily 300000, so capped.
		{"non-crossing, in 01:00 out 11:00 next day: capped", flat, RentalOvernight, at(5, 1, 0), at(6, 11, 0), 300000, true},
		// After the window end the check-in is early for the next day's window (00:00 day 6, 11h
		// away): 200000 + 11 * 20000 = 420000, capped.
		{"non-crossing, in 13:00 is early for the next window", flat, RentalOvernight, at(5, 13, 0), at(6, 11, 0), 300000, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := Price(c.plan, c.rental, c.in, c.out, loc)
			if err != nil || q.Total != c.total || q.Capped != c.capped {
				t.Fatalf("got %+v, %v; want %d capped %v", q, err, c.total, c.capped)
			}
		})
	}
}

func TestPriceRejectsBadInput_SG101_AC4(t *testing.T) {
	loc := time.FixedZone("ICT", 7*60*60)
	plan := planWith(15, 80000, 20000, 200000, 300000, clk(21, 0), clk(12, 0), clk(14, 0), clk(12, 0))
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, loc)

	if _, err := Price(plan, RentalHourly, in, in.Add(time.Hour), nil); !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("nil zone: %v", err)
	}
	if _, err := Price(plan, RentalHourly, time.Time{}, in, loc); !errors.Is(err, ErrStayTooLong) {
		t.Fatalf("zero check-in: %v", err)
	}
	if _, err := Price(plan, RentalDaily, in, in.AddDate(0, 0, MaxStayDays+1), loc); !errors.Is(err, ErrStayTooLong) {
		t.Fatalf("367 days: %v", err)
	}
	if _, err := Price(plan, RentalDaily, in, in.AddDate(0, 0, MaxStayDays), loc); err != nil {
		t.Fatalf("exactly 366 days must be allowed: %v", err)
	}
}
