package pricing

import (
	"reflect"
	"testing"
	"time"
)

func TestQuoteRunning_SG203_AC6(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	plan := planWith(15, 80_000, 20_000, 250_000, 350_000,
		Clock{Hour: 21}, Clock{Hour: 12}, Clock{Hour: 14}, Clock{Hour: 12})
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, loc)
	min := func(rt RentalType) Quote {
		q, err := Price(plan, rt, in, in.Add(MinBillableInterval), loc)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	for _, rt := range []RentalType{RentalHourly, RentalOvernight, RentalDaily} {
		for name, tc := range map[string]struct {
			at   time.Time
			want func() (Quote, error)
		}{
			"equal instants":   {in, func() (Quote, error) { return min(rt), nil }},
			"one second after": {in.Add(time.Second), func() (Quote, error) { return Price(plan, rt, in, in.Add(time.Second), loc) }},
			"before check-in":  {in.Add(-time.Hour), func() (Quote, error) { return min(rt), nil }},
			"normal":           {in.Add(3 * time.Hour), func() (Quote, error) { return Price(plan, rt, in, in.Add(3*time.Hour), loc) }},
		} {
			got, err := QuoteRunning(plan, rt, in, tc.at, loc)
			want, werr := tc.want()
			if err != nil || werr != nil {
				t.Fatalf("%s/%s: %v %v", rt, name, err, werr)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s/%s: got %+v want %+v", rt, name, got, want)
			}
		}
	}
}
