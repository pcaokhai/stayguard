package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_AllowsLimitThenBlocksUntilWindowEnds_SG701_AC5(t *testing.T) {
	now := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	l := New(2, time.Minute, func() time.Time { return now })
	first, second, third := l.Allow("a"), l.Allow("a"), l.Allow("a")
	if !first || !second || third {
		t.Fatal("want two allowed then blocked")
	}
	if !l.Allow("b") {
		t.Fatal("keys are independent")
	}
	now = now.Add(time.Minute)
	if !l.Allow("a") {
		t.Fatal("new window")
	}
}
