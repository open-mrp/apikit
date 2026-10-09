// Package ratelimit decides whether a caller may make another request. A Limiter counts requests per key; the middleware package turns its Decision into RateLimit-* headers and 429 rate_limited responses.
package ratelimit

import (
	"context"
	"time"
)

// Decision is a Limiter's answer for one request.
type Decision struct {
	// Allowed reports whether the request may proceed.
	Allowed bool
	// Limit is the number of requests allowed per window.
	Limit int
	// Remaining is how many more requests the key may make in the current window.
	Remaining int
	// Reset is how long until the current window ends.
	Reset time.Duration
	// RetryAfter is how long a refused caller should wait before retrying. Zero when allowed.
	RetryAfter time.Duration
}

// Limiter counts requests per key, such as a client IP or an API key.
type Limiter interface {
	// Allow records a request for key and reports whether it is within the limit.
	Allow(ctx context.Context, key string) (Decision, error)
}
