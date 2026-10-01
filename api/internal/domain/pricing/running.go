package pricing

import "time"

// MinBillableInterval is what a stay owes the moment it starts: Price rejects an empty interval,
// but a guest who just checked in already owes the first billable amount.
const MinBillableInterval = time.Minute

// QuoteRunning prices a stay from check-in to at. When at is not after check-in (the same
// instant, or a clock that is a little behind) it quotes check-in plus MinBillableInterval.
func QuoteRunning(plan RatePlan, rental RentalType, checkIn, at time.Time, loc *time.Location) (Quote, error) {
	if !at.After(checkIn) {
		at = checkIn.Add(MinBillableInterval)
	}
	return Price(plan, rental, checkIn, at, loc)
}
