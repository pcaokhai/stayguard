package room

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func at(loc *time.Location, day, hour, min int) time.Time {
	return time.Date(2026, time.March, day, hour, min, 0, 0, loc)
}

func TestOverdueStatus_SG201_AC3(t *testing.T) {
	zones := []string{"Asia/Ho_Chi_Minh", "America/New_York"}
	for _, zone := range zones {
		loc := mustLoc(t, zone)
		checkIn := at(loc, 10, 20, 0)
		end := at(loc, 11, 12, 0)
		overnight := &StayTiming{RentalType: Overnight, CheckInAt: checkIn, WindowEnd: "12:00", GraceMinutes: 15}
		daily := &StayTiming{RentalType: Daily, CheckInAt: checkIn, WindowEnd: "12:00", GraceMinutes: 15}
		hourly := &StayTiming{RentalType: Hourly, CheckInAt: checkIn, GraceMinutes: 15}
		grace := 15 * time.Minute

		cases := []struct {
			name   string
			stored Status
			stay   *StayTiming
			now    time.Time
			want   Status
		}{
			{"overnight 1m before end", StatusOccupied, overnight, end.Add(-time.Minute), StatusOccupied},
			{"overnight at end", StatusOccupied, overnight, end, StatusOccupied},
			{"overnight at end+grace", StatusOccupied, overnight, end.Add(grace), StatusOccupied},
			{"overnight 1m after end+grace", StatusOccupied, overnight, end.Add(grace + time.Minute), StatusOverdue},
			{"daily 1m before end", StatusOccupied, daily, end.Add(-time.Minute), StatusOccupied},
			{"daily at end", StatusOccupied, daily, end, StatusOccupied},
			{"daily at end+grace", StatusOccupied, daily, end.Add(grace), StatusOccupied},
			{"daily 1m after end+grace", StatusOccupied, daily, end.Add(grace + time.Minute), StatusOverdue},
			{"hourly 30 days later", StatusOccupied, hourly, checkIn.AddDate(0, 0, 30), StatusOccupied},
			{"to clean never overdue", StatusToClean, overnight, end.AddDate(0, 0, 5), StatusToClean},
			{"maintenance never overdue", StatusMaintenance, overnight, end.AddDate(0, 0, 5), StatusMaintenance},
			{"vacant never overdue", StatusVacant, nil, end.AddDate(0, 0, 5), StatusVacant},
			{"occupied nil stay", StatusOccupied, nil, end.AddDate(0, 0, 5), StatusOccupied},
		}
		for _, tc := range cases {
			t.Run(zone+"/"+tc.name, func(t *testing.T) {
				got, err := Derive(tc.stored, tc.stay, tc.now, loc)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %s want %s", got, tc.want)
				}
			})
		}
	}
}

func TestOverdueStatus_ZoneMatters_SG201_AC3(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	stay := &StayTiming{RentalType: Overnight, CheckInAt: at(hcm, 10, 20, 0), WindowEnd: "12:00", GraceMinutes: 0}
	now := at(hcm, 11, 12, 1)
	if got, _ := Derive(StatusOccupied, stay, now, hcm); got != StatusOverdue {
		t.Fatalf("hcm: got %s want OVERDUE", got)
	}
	if got, _ := Derive(StatusOccupied, stay, now, time.UTC); got != StatusOccupied {
		t.Fatalf("utc: got %s want OCCUPIED (12:00 UTC is 19:00 +07)", got)
	}
}

func TestOverdueStatus_Rejects_SG201_AC3(t *testing.T) {
	for _, s := range []Status{StatusOverdue, Status("BOGUS"), Status("")} {
		if _, err := Derive(s, nil, time.Unix(0, 0), time.UTC); err == nil {
			t.Fatalf("stored %q: want error", s)
		}
	}
	bad := &StayTiming{RentalType: Overnight, WindowEnd: "nope"}
	if _, err := Derive(StatusOccupied, bad, time.Unix(0, 0), time.UTC); err == nil {
		t.Fatal("bad window: want error, not 'not overdue'")
	}
}

func TestParseStored_SG201_AC3(t *testing.T) {
	for _, s := range []string{"VACANT", "OCCUPIED", "TO_CLEAN", "MAINTENANCE"} {
		if got, err := ParseStored(s); err != nil || string(got) != s {
			t.Fatalf("%s: got %v %v", s, got, err)
		}
	}
	for _, s := range []string{"OVERDUE", "vacant", "", "X"} {
		if _, err := ParseStored(s); err == nil {
			t.Fatalf("%q: want error", s)
		}
	}
}

func TestParseRentalType_SG201_AC3(t *testing.T) {
	for _, s := range []string{"HOURLY", "OVERNIGHT", "DAILY"} {
		if got, err := ParseRentalType(s); err != nil || string(got) != s {
			t.Fatalf("%s: got %v %v", s, got, err)
		}
	}
	if _, err := ParseRentalType("WEEKLY"); err == nil {
		t.Fatal("want error")
	}
}

func TestExpectedEnd_SG201_AC3(t *testing.T) {
	loc := mustLoc(t, "Asia/Ho_Chi_Minh")
	cases := []struct {
		name    string
		rt      RentalType
		checkIn time.Time
		want    time.Time
	}{
		{"08:00 same day", Daily, at(loc, 10, 8, 0), at(loc, 10, 12, 0)},
		{"11:59 same day", Daily, at(loc, 10, 11, 59), at(loc, 10, 12, 0)},
		{"12:00 exactly next day", Daily, at(loc, 10, 12, 0), at(loc, 11, 12, 0)},
		{"13:00 next day", Daily, at(loc, 10, 13, 0), at(loc, 11, 12, 0)},
		{"23:30 next day", Daily, at(loc, 10, 23, 30), at(loc, 11, 12, 0)},
		{"overnight crossing midnight", Overnight, at(loc, 31, 22, 0), time.Date(2026, time.April, 1, 12, 0, 0, 0, loc)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stay := StayTiming{RentalType: tc.rt, CheckInAt: tc.checkIn, WindowEnd: "12:00"}
			got, err := ExpectedEnd(stay, loc)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	if _, err := ExpectedEnd(StayTiming{RentalType: Hourly}, loc); err == nil {
		t.Fatal("hourly: want error")
	}
}

func TestExpectedEnd_DST_SG201_AC3(t *testing.T) {
	ny := mustLoc(t, "America/New_York")
	stay := StayTiming{RentalType: Overnight, CheckInAt: time.Date(2026, time.March, 7, 22, 0, 0, 0, ny), WindowEnd: "12:00"}
	got, err := ExpectedEnd(stay, ny)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.March, 8, 12, 0, 0, 0, ny); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestParseTiming_SG201_AC3(t *testing.T) {
	checkIn := time.Date(2026, time.March, 10, 20, 0, 0, 0, time.UTC)
	const ok = `{"version":1,"currency":"VND","graceMinutes":15,"hourly":{},"overnight":{"windowStart":"20:00","windowEnd":"12:00"},"daily":{"windowStart":"14:00","windowEnd":"11:00"}}`
	valid := []struct {
		name string
		rt   RentalType
		want StayTiming
	}{
		{"overnight", Overnight, StayTiming{Overnight, checkIn, "12:00", 15}},
		{"daily", Daily, StayTiming{Daily, checkIn, "11:00", 15}},
		{"hourly", Hourly, StayTiming{Hourly, checkIn, "", 15}},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTiming(tc.rt, checkIn, []byte(ok))
			if err != nil || got != tc.want {
				t.Fatalf("got %+v %v want %+v", got, err, tc.want)
			}
		})
	}
	invalid := []struct {
		name string
		rt   RentalType
		json string
	}{
		{"malformed", Overnight, `{`},
		{"empty", Overnight, ``},
		{"missing window", Overnight, `{"graceMinutes":15,"overnight":{}}`},
		{"missing section", Daily, `{"graceMinutes":15,"overnight":{"windowEnd":"12:00"}}`},
		{"bad clock", Overnight, `{"graceMinutes":15,"overnight":{"windowEnd":"25:00"}}`},
		{"negative grace", Overnight, `{"graceMinutes":-1,"overnight":{"windowEnd":"12:00"}}`},
		{"missing grace", Overnight, `{"overnight":{"windowEnd":"12:00"}}`},
		{"hourly missing grace", Hourly, `{}`},
		{"unknown rental type", RentalType("X"), ok},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseTiming(tc.rt, checkIn, []byte(tc.json)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestElapsedMinutes_SG201_AC2(t *testing.T) {
	base := time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		now  time.Time
		want int
	}{
		{"zero", base, 0},
		{"59s truncates", base.Add(59 * time.Second), 0},
		{"90s truncates", base.Add(90 * time.Second), 1},
		{"two hours", base.Add(2 * time.Hour), 120},
		{"negative skew", base.Add(-5 * time.Minute), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ElapsedMinutes(base, tc.now); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestCount_SG201_AC1(t *testing.T) {
	got := Count([]Status{StatusVacant, StatusOccupied, StatusOccupied, StatusOverdue, StatusToClean, StatusMaintenance, StatusVacant})
	want := Counts{Vacant: 2, Occupied: 2, Overdue: 1, ToClean: 1, Maintenance: 1}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if got := Count(nil); got != (Counts{}) {
		t.Fatalf("nil: got %+v", got)
	}
}
