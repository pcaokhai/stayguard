// Package ratelimit is a small in-memory fixed-window limiter, per process.
package ratelimit

import (
	"sort"
	"sync"
	"time"
)

const (
	// maxKeys bounds memory.
	maxKeys = 10_000
	// pruneEvery is how seldom expired windows are swept when the map is full.
	pruneEvery = time.Second
	// evictBatch is how many of the oldest windows go at once when the map is still full after a sweep, so the cost
	// of finding them is paid once per evictBatch new keys, not on every request.
	evictBatch = maxKeys / 10
)

type window struct {
	start time.Time
	count int
}

// Limiter allows at most limit calls per key in each window.
// ponytail: per process and fixed window; a shared store or sliding window if the API runs on several instances.
// Evicting a key forgets its count only; the sign-in lockout lives in the database, so eviction cannot reset it.
type Limiter struct {
	limit     int
	window    time.Duration
	now       func() time.Time
	mu        sync.Mutex
	keys      map[string]window
	lastPrune time.Time
}

func New(limit int, per time.Duration, now func() time.Time) *Limiter {
	return &Limiter{limit: limit, window: per, now: now, keys: map[string]window{}}
}

// Allow counts the call and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.keys[key]
	if !ok || !now.Before(w.start.Add(l.window)) {
		if !ok && len(l.keys) >= maxKeys {
			l.makeRoom(now)
		}
		w = window{start: now}
	}
	w.count++
	l.keys[key] = w
	return w.count <= l.limit
}

// makeRoom sweeps expired windows at most once per pruneEvery; if the map is still full it evicts the oldest batch.
func (l *Limiter) makeRoom(now time.Time) {
	if now.Sub(l.lastPrune) >= pruneEvery {
		l.lastPrune = now
		for k, w := range l.keys {
			if !now.Before(w.start.Add(l.window)) {
				delete(l.keys, k)
			}
		}
	}
	if len(l.keys) < maxKeys {
		return
	}
	type entry struct {
		key   string
		start time.Time
	}
	all := make([]entry, 0, len(l.keys))
	for k, w := range l.keys {
		all = append(all, entry{k, w.start})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].start.Before(all[j].start) })
	for _, e := range all[:evictBatch] {
		delete(l.keys, e.key)
	}
}
