package pricing

import (
	"testing"
	"time"
)

func TestPriceUsesSnapshotNotTenantPlan_SG101_AC6(t *testing.T) {
	loc := time.FixedZone("ICT", 7*60*60)
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, loc)
	out := in.Add(45 * time.Minute)
	v1, err := ParseRatePlan([]byte(edit(t, map[string]any{"version": 1})))
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ParseRatePlan([]byte(edit(t, map[string]any{"version": 2, "hourly.firstHour": 90000})))
	if err != nil {
		t.Fatal(err)
	}

	stored := v1.Snapshot()
	before := mustPrice(t, stored, in, out, loc)
	// The tenant plan moves to v2; the stay still prices from its stored snapshot.
	after := mustPrice(t, stored, in, out, loc)
	if before.Total != 80000 || after.Total != before.Total {
		t.Fatalf("stored v1: before %d, after %d", before.Total, after.Total)
	}
	if got := mustPrice(t, v2.Snapshot(), in, out, loc); got.Total != 90000 {
		t.Fatalf("new stay on v2 = %d", got.Total)
	}
}

func mustPrice(t *testing.T, snapshot []byte, in, out time.Time, loc *time.Location) Quote {
	t.Helper()
	plan, err := ParseRatePlan(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Price(plan, RentalHourly, in, out, loc)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
