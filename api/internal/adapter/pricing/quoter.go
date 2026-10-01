// Package pricing binds the StayQuoter port to the pricing engine.
package pricing

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// Quoter prices a running stay from the rate plan snapshot copied onto it at check-in, so a later
// tenant plan change never moves an open stay's total.
//
// Fail closed: an engine error (a stay beyond pricing.MaxStayDays, an unreadable snapshot) is
// returned, and the room list answers a generic 500 rather than showing an invented total. A stay
// left open past MaxStayDays therefore breaks the room list until it is checked out.
type Quoter struct{}

var _ app.StayQuoter = Quoter{}

func (Quoter) RunningTotal(_ context.Context, snapshot []byte, rt room.RentalType, checkIn, now time.Time, loc *time.Location) (int64, error) {
	plan, err := pricing.ParseRatePlan(snapshot)
	if err != nil {
		// The validation error lists field paths only; the snapshot bytes stay out of it.
		return 0, fmt.Errorf("running total: rate plan snapshot: %w", err)
	}
	rental, err := pricing.ParseRentalType(string(rt))
	if err != nil {
		return 0, fmt.Errorf("running total: %w", err)
	}
	q, err := pricing.QuoteRunning(plan, rental, checkIn, now, loc)
	if err != nil {
		return 0, fmt.Errorf("running total: %w", err)
	}
	return q.Total.Int64(), nil
}
