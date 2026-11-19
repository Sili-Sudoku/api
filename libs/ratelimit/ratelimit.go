// Package ratelimit is a small in-process limiter: at most N events per key in a sliding window.
// State lives in this process only, so with several API instances each one counts alone.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu   sync.Mutex
	hits map[string][]time.Time
}

func New(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, now: time.Now, hits: map[string][]time.Time{}}
}

// SetClock replaces the clock. For tests.
func (l *Limiter) SetClock(now func() time.Time) { l.now = now }

// Allow records an event for key and reports whether it is within the limit.
// A refused event is not recorded, so a client that waits is let in again.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	if len(l.hits) > 10_000 { // bound memory: drop idle keys
		for k, ts := range l.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(l.hits, k)
			}
		}
	}

	return true
}
