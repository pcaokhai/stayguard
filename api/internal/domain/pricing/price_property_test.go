package pricing

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// propertySeed is fixed so a failure reproduces; Fatalf prints it with the iteration inputs.
const (
	propertySeed       uint64 = 20261001
	propertyIterations        = 3000
	maxPriceVnd               = 2_000_000
	maxStaySeconds            = 4 * 24 * 60 * 60
	checkInSpreadSecs         = 400 * 24 * 60 * 60
)

var rentalTypes = []RentalType{RentalHourly, RentalOvernight, RentalDaily}

// randomClock covers crossing and non-crossing windows alike, minutes included.
func randomClock(r *rand.Rand) Clock {
	return Clock{Hour: r.IntN(24), Minute: r.IntN(60)}
}

func randomPlan(r *rand.Rand) RatePlan {
	price := func() money.Vnd { return money.Vnd(r.Int64N(maxPriceVnd + 1)) }
	return RatePlan{
		Version:      goldenPlanVersion,
		Currency:     CurrencyVND,
		GraceMinutes: r.IntN(MaxGraceMinutes + 1),
		Hourly:       Hourly{FirstHour: price(), ExtraHour: price()},
		Overnight:    Window{Price: price(), Start: randomClock(r), End: randomClock(r)},
		Daily:        Window{Price: price(), Start: randomClock(r), End: randomClock(r)},
	}
}

func sumLines(lines []Line) money.Vnd {
	var s money.Vnd
	for _, l := range lines {
		s += l.Amount
	}
	return s
}

func TestPriceProperties_SG101_AC3(t *testing.T) {
	loc, err := time.LoadLocation(goldenZone)
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(propertySeed, propertySeed)) //nolint:gosec // reproducible test data, not security
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < propertyIterations; i++ {
		plan := randomPlan(r)
		in := base.Add(time.Duration(r.Int64N(checkInSpreadSecs)) * time.Second)
		s1, s2 := 1+r.Int64N(maxStaySeconds), 1+r.Int64N(maxStaySeconds)
		if s1 > s2 {
			s1, s2 = s2, s1
		}
		early, late := in.Add(time.Duration(s1)*time.Second), in.Add(time.Duration(s2)*time.Second)
		for _, rt := range rentalTypes {
			fail := func(msg string, args ...any) {
				t.Helper()
				t.Fatalf("seed %d iter %d %s plan %+v in %s out %s..%s: "+msg, append([]any{propertySeed, i, rt, plan, in, early, late}, args...)...)
			}
			qe, err1 := Price(plan, rt, in, early, loc)
			ql, err2 := Price(plan, rt, in, late, loc)
			if err1 != nil || err2 != nil {
				fail("unexpected error %v / %v", err1, err2)
			}
			if qe.Total > ql.Total {
				fail("price decreased: %d then %d", qe.Total, ql.Total)
			}
			for _, q := range []Quote{qe, ql} {
				checkQuote(fail, plan, rt, q)
			}
		}
	}
}

func checkQuote(fail func(string, ...any), plan RatePlan, rt RentalType, q Quote) {
	uncapped := sumLines(q.Lines)
	for _, l := range q.Lines {
		if l.Amount != l.UnitAmount*money.Vnd(l.Quantity) {
			fail("line %+v amount != quantity * unit", l)
		}
	}
	if rt == RentalDaily {
		if q.Capped || q.Total != uncapped {
			fail("daily must not cap: %+v", q)
		}
		return
	}
	if q.Total > plan.Daily.Price {
		fail("total %d exceeds daily %d", q.Total, plan.Daily.Price)
	}
	if q.Capped != (uncapped > plan.Daily.Price) {
		fail("capped=%v but uncapped sum %d, daily %d", q.Capped, uncapped, plan.Daily.Price)
	}
	if !q.Capped && q.Total != uncapped {
		fail("total %d != line sum %d", q.Total, uncapped)
	}
}

func TestPriceDeterministic_SG101_AC4(t *testing.T) {
	loc, err := time.LoadLocation(goldenZone)
	if err != nil {
		t.Fatal(err)
	}
	plan := goldenFile{GraceMinutes: 15, RatePlans: map[string]goldenPlan{"S": {80000, 20000, 200000, 300000}}}.plan(t, "S")
	in := time.Date(2026, 10, 5, 22, 0, 0, 0, loc)
	out := time.Date(2026, 10, 6, 13, 10, 0, 0, loc)
	other := time.FixedZone("UTC-5", -5*60*60)
	for _, rt := range rentalTypes {
		a, errA := Price(plan, rt, in, out, loc)
		b, errB := Price(plan, rt, in, out, loc)
		c, errC := Price(plan, rt, in.In(other), out.In(other), loc)
		if errA != nil || errB != nil || errC != nil {
			t.Fatal(errA, errB, errC)
		}
		if a.Total != b.Total || a.Total != c.Total || len(a.Lines) != len(c.Lines) || a.Capped != c.Capped {
			t.Fatalf("%s: %+v vs %+v vs %+v", rt, a, b, c)
		}
		for i := range a.Lines {
			if a.Lines[i] != b.Lines[i] || a.Lines[i] != c.Lines[i] {
				t.Fatalf("%s line %d differs", rt, i)
			}
		}
	}
}
