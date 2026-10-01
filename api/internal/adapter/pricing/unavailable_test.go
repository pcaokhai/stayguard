package pricing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

func TestUnavailableQuoter_SG201_AC2(t *testing.T) {
	_, err := Unavailable{}.RunningTotal(context.Background(), nil, room.Overnight, time.Time{}, time.Time{}, time.UTC)
	if !errors.Is(err, app.ErrPricingUnavailable) {
		t.Fatalf("err = %v, want ErrPricingUnavailable", err)
	}
}
