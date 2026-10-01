package pricing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const (
	hourlyFirst   = money.Vnd(80_000)
	staleStayDays = 400
)

func testPlan(version int64, firstHour money.Vnd) pricing.RatePlan {
	return pricing.RatePlan{
		Version: version, Currency: pricing.CurrencyVND, GraceMinutes: 15,
		Hourly:    pricing.Hourly{FirstHour: firstHour, ExtraHour: 20_000},
		Overnight: pricing.Window{Price: 350_000, Start: pricing.Clock{Hour: 21}, End: pricing.Clock{Hour: 12}},
		Daily:     pricing.Window{Price: 500_000, Start: pricing.Clock{Hour: 14}, End: pricing.Clock{Hour: 12}},
	}
}

func TestQuoter_SG201_AC2(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, loc)
	snap := testPlan(1, hourlyFirst).Snapshot()
	q := Quoter{}

	for name, tc := range map[string]struct {
		rt  room.RentalType
		now time.Time
	}{
		"hourly 45 minutes": {room.Hourly, in.Add(45 * time.Minute)},
		"hourly 2h35m":      {room.Hourly, in.Add(2*time.Hour + 35*time.Minute)},
		"overnight":         {room.Overnight, in.Add(26 * time.Hour)},
		"daily":             {room.Daily, in.Add(50 * time.Hour)},
	} {
		t.Run(name, func(t *testing.T) {
			want := mustPrice(t, snap, pricing.RentalType(tc.rt), in, tc.now, loc)
			got, err := q.RunningTotal(context.Background(), snap, tc.rt, in, tc.now, loc)
			if err != nil || got != want {
				t.Fatalf("got %d, %v; want %d", got, err, want)
			}
		})
	}

	t.Run("now at or before check-in bills the minimum", func(t *testing.T) {
		for _, now := range []time.Time{in, in.Add(-time.Hour)} {
			got, err := q.RunningTotal(context.Background(), snap, room.Hourly, in, now, loc)
			if err != nil || got != int64(hourlyFirst) {
				t.Fatalf("now=%v got %d, %v; want first hour", now, got, err)
			}
		}
	})

	t.Run("stored v1 snapshot keeps v1 prices", func(t *testing.T) {
		_ = testPlan(2, 90_000).Snapshot() // the tenant moved on; the stay does not look at it
		got, err := q.RunningTotal(context.Background(), snap, room.Hourly, in, in.Add(45*time.Minute), loc)
		if err != nil || got != int64(hourlyFirst) {
			t.Fatalf("got %d, %v; want %d", got, err, hourlyFirst)
		}
	})

	t.Run("fails closed", func(t *testing.T) {
		if _, err := q.RunningTotal(context.Background(), []byte(`{}`), room.Hourly, in, in.Add(time.Hour), loc); err == nil {
			t.Error("garbage snapshot: want error")
		}
		if _, err := q.RunningTotal(context.Background(), snap, room.RentalType("WEEKLY"), in, in.Add(time.Hour), loc); err == nil {
			t.Error("unknown rental type: want error")
		}
		_, err := q.RunningTotal(context.Background(), snap, room.Hourly, in, in.AddDate(0, 0, staleStayDays), loc)
		if !errors.Is(err, pricing.ErrStayTooLong) {
			t.Errorf("400-day stay: err = %v, want ErrStayTooLong", err)
		}
	})
}

func mustPrice(t *testing.T, snap []byte, rt pricing.RentalType, in, now time.Time, loc *time.Location) int64 {
	t.Helper()
	plan, err := pricing.ParseRatePlan(snap)
	if err != nil {
		t.Fatal(err)
	}
	q, err := pricing.Price(plan, rt, in, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	return q.Total.Int64()
}
