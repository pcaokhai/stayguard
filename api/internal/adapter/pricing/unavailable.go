// Package pricing binds the StayQuoter port.
package pricing

import (
	"context"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// Unavailable is the temporary StayQuoter until SG-101 merges the pricing engine: it always fails,
// so the room map stays behind its flag instead of showing invented totals.
type Unavailable struct{}

var _ app.StayQuoter = Unavailable{}

func (Unavailable) RunningTotal(context.Context, []byte, room.RentalType, time.Time, time.Time, *time.Location) (int64, error) {
	return 0, app.ErrPricingUnavailable
}
