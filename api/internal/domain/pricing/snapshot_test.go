package pricing

import (
	"bytes"
	"testing"
)

func TestSnapshotStable_SG101_AC6(t *testing.T) {
	v1Doc := edit(t, map[string]any{"version": 1})
	v2Doc := edit(t, map[string]any{"version": 2, "hourly.firstHour": 90000})
	v1, err := ParseRatePlan([]byte(v1Doc))
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ParseRatePlan([]byte(v2Doc))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("round trip byte stable", func(t *testing.T) {
		snap := v1.Snapshot()
		back, err := ParseRatePlan(snap)
		if err != nil {
			t.Fatal(err)
		}
		if back != v1 {
			t.Fatalf("round trip changed plan: %+v vs %+v", back, v1)
		}
		if !bytes.Equal(back.Snapshot(), snap) {
			t.Fatalf("snapshot not byte stable:\n%s\n%s", snap, back.Snapshot())
		}
	})

	t.Run("canonical form", func(t *testing.T) {
		const want = `{"version":1,"currency":"VND","graceMinutes":15,"hourly":{"firstHour":80000,"extraHour":20000},` +
			`"overnight":{"price":250000,"windowStart":"21:00","windowEnd":"12:00"},` +
			`"daily":{"price":350000,"windowStart":"14:00","windowEnd":"12:00"}}`
		if got := string(v1.Snapshot()); got != want {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("input formatting does not change the snapshot", func(t *testing.T) {
		spaced := "  " + string(bytes.ReplaceAll(v1.Snapshot(), []byte(","), []byte(",\n  "))) + "\n"
		p, err := ParseRatePlan([]byte(spaced))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(p.Snapshot(), v1.Snapshot()) {
			t.Fatal("formatting leaked into snapshot")
		}
	})

	t.Run("versions differ and a stored v1 survives v2", func(t *testing.T) {
		stored := v1.Snapshot()
		if bytes.Equal(stored, v2.Snapshot()) {
			t.Fatal("v1 and v2 snapshots must differ")
		}
		got, err := ParseRatePlan(stored)
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != 1 || got.Hourly.FirstHour != 80000 {
			t.Fatalf("stored v1 changed: %+v", got)
		}
		if v2.Version != 2 || v2.Hourly.FirstHour != 90000 {
			t.Fatalf("v2 wrong: %+v", v2)
		}
	})
}
