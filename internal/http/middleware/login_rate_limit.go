package middleware

import (
	"sync"
	"time"
)

type LoginRateLimiter struct {
	window  time.Duration
	burst   int
	mu      sync.Mutex
	attempt map[string][]time.Time
}

func NewLoginRateLimiter(window time.Duration, burst int) *LoginRateLimiter {
	if burst <= 0 {
		burst = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &LoginRateLimiter{
		window:  window,
		burst:   burst,
		attempt: map[string][]time.Time{},
	}
}

func (l *LoginRateLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	items := l.attempt[key]
	filtered := items[:0]
	for _, ts := range items {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	if len(filtered) >= l.burst {
		l.attempt[key] = filtered
		return false
	}
	filtered = append(filtered, now)
	l.attempt[key] = filtered
	return true
}

func (l *LoginRateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempt, key)
}

