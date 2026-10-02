// Package ratelimit is a small in-memory fixed-window limiter, per process.
package ratelimit

import (
	"sync"
	"time"
)

// maxKeys bounds memory: past it, expired windows are dropped before a new key is added.
const maxKeys = 10_000

type window struct {
	start time.Time
	count int
}

// Limiter allows at most limit calls per key in each window.
// ponytail: per process and fixed window; a shared store or sliding window if the API runs on several instances.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	keys   map[string]window
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
			l.prune(now)
		}
		w = window{start: now}
	}
	w.count++
	l.keys[key] = w
	return w.count <= l.limit
}

func (l *Limiter) prune(now time.Time) {
	for k, w := range l.keys {
		if !now.Before(w.start.Add(l.window)) {
			delete(l.keys, k)
		}
	}
}
