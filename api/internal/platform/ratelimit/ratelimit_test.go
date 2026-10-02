package ratelimit

import (
	"fmt"
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

// Hardening 7: with 10,000 active keys the map stays bounded, the oldest keys go first, newer keys keep their count,
// and sweeping does not happen on every request.
func TestLimiter_FullMapEvictsOldestInBatches_Hardening7(t *testing.T) {
	now := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	l := New(3, time.Hour, func() time.Time { return now })
	for i := 0; i < maxKeys; i++ {
		now = now.Add(time.Millisecond) // every window is active and older keys have earlier starts
		l.Allow(fmt.Sprintf("k%05d", i))
	}
	if len(l.keys) != maxKeys {
		t.Fatalf("keys = %d", len(l.keys))
	}
	l.Allow("k00000") // a recent hit on an old key is only a count: it stays and does not add a key
	now = now.Add(time.Millisecond)
	l.Allow("new-1")
	if len(l.keys) != maxKeys-evictBatch+1 {
		t.Fatalf("one batch of the oldest must go: keys = %d, want %d", len(l.keys), maxKeys-evictBatch+1)
	}
	if _, ok := l.keys["k00005"]; ok {
		t.Fatal("the oldest keys are evicted first")
	}
	if _, ok := l.keys[fmt.Sprintf("k%05d", maxKeys-1)]; !ok {
		t.Fatal("the newest keys stay")
	}
	// Room was made, so the next new keys do not sort again.
	before := l.lastPrune
	for i := 0; i < 100; i++ {
		l.Allow(fmt.Sprintf("later-%d", i))
	}
	if len(l.keys) != maxKeys-evictBatch+1+100 || !l.lastPrune.Equal(before) {
		t.Fatalf("no work while there is room: keys=%d", len(l.keys))
	}
}

func TestLimiter_SweepsExpiredWindowsAtMostOncePerSecond_Hardening7(t *testing.T) {
	now := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	l := New(3, time.Minute, func() time.Time { return now })
	for i := 0; i < maxKeys; i++ {
		l.Allow(fmt.Sprintf("k%05d", i))
	}
	now = now.Add(2 * time.Minute) // every window has ended
	l.Allow("fresh")
	if len(l.keys) != 1 {
		t.Fatalf("expired windows are swept when the map is full, keys = %d", len(l.keys))
	}
}

func BenchmarkAllowFullMap(b *testing.B) {
	now := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	l := New(3, time.Hour, func() time.Time { return now })
	for i := 0; i < maxKeys; i++ {
		now = now.Add(time.Millisecond)
		l.Allow(fmt.Sprintf("k%05d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now = now.Add(time.Millisecond)
		l.Allow(fmt.Sprintf("n%d", i))
	}
}
