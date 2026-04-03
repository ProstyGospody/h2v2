package middleware

import (
	"net/http"
	"sync"
	"time"
)

const (
	apiRateLimitWindow = time.Minute
	apiRateLimitBurst  = 300
)

// APIRateLimiter limits API requests per IP address.
type APIRateLimiter struct {
	window  time.Duration
	burst   int
	mu      sync.Mutex
	attempt map[string][]time.Time
}

func NewAPIRateLimiter() *APIRateLimiter {
	return &APIRateLimiter{
		window:  apiRateLimitWindow,
		burst:   apiRateLimitBurst,
		attempt: make(map[string][]time.Time),
	}
}

func (l *APIRateLimiter) Allow(key string) bool {
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

// RateLimit returns a middleware that limits requests per IP.
func RateLimit(limiter *APIRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if !limiter.Allow(ip) {
				http.Error(w, `{"error":"rate_limited","message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
