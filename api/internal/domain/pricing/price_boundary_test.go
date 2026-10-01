package pricing

import (
	"testing"
	"time"
)

// Boundaries the golden file does not reach; found by the mutation review (Ruling 4).
func TestPriceBoundaries_SG101_AC1(t *testing.T) {
	g := loadGolden(t)
	loc := goldenZoneLoc(t, g)
	std := g.plan(t, "STANDARD")
	day := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, loc) }

	t.Run("hourly total equal to the daily price is not capped", func(t *testing.T) {
		// 12h: 80000 + 11 * 20000 = 300000, exactly the daily price.
		q, err := Price(std, RentalHourly, day(5, 10, 0), day(5, 22, 0), loc)
		if err != nil || q.Total != std.Daily.Price || q.Capped {
			t.Fatalf("got %+v, %v", q, err)
		}
	})
	t.Run("check-in exactly at the overnight window end starts that day's night", func(t *testing.T) {
		// 12:00 is not before 12:00, so the night is today's 21:00 start: 9h early, then capped.
		q, err := Price(std, RentalOvernight, day(5, 12, 0), day(5, 12, 30), loc)
		if err != nil || q.Total != std.Daily.Price || !q.Capped {
			t.Fatalf("got %+v, %v", q, err)
		}
	})
}
