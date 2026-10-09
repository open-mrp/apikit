package middleware

import (
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/ratelimit"
	"github.com/open-mrp/apikit/transport"
)

// RateLimitConfig configures RateLimit.
type RateLimitConfig struct {
	// Limiter (required) counts requests.
	Limiter ratelimit.Limiter
	// Key (optional; default: the client IP) names whose requests are counted together, such as an API key.
	Key func(r *http.Request) string
	// TrustedProxyHops (optional; default: 0) is how many proxies in front of the server append to X-Forwarded-For, for the default Key.
	TrustedProxyHops int
	// SkipPaths (optional) are paths never limited, such as a health check.
	SkipPaths []string
	// Skip (optional) exempts a request, such as every request in a development environment, where all traffic shares one IP.
	Skip func(r *http.Request) bool
}

// RateLimit counts each request against its key and refuses those over the limit with 429 rate_limited and a Retry-After header. Every counted response carries the RateLimit-Limit, RateLimit-Remaining and RateLimit-Reset headers. If the limiter fails, the request is let through: an outage of the counter should not take the API down with it.
func RateLimit(cfg RateLimitConfig) func(http.HandlerFunc) http.HandlerFunc {
	if cfg.Limiter == nil {
		panic("middleware: RateLimit needs a Limiter")
	}
	key := cfg.Key
	if key == nil {
		key = func(r *http.Request) string {
			if ip := transport.GetClientIP(r, cfg.TrustedProxyHops); ip != nil {
				return ip.String()
			}
			return r.RemoteAddr
		}
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if slices.Contains(cfg.SkipPaths, r.URL.Path) || (cfg.Skip != nil && cfg.Skip(r)) {
				next.ServeHTTP(w, r)
				return
			}
			d, err := cfg.Limiter.Allow(r.Context(), key(r))
			if err != nil {
				slog.WarnContext(r.Context(), "rate limiter failed; allowing the request", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set(transport.RateLimitLimitHeader, strconv.Itoa(d.Limit))
			h.Set(transport.RateLimitRemainingHeader, strconv.Itoa(d.Remaining))
			h.Set(transport.RateLimitResetHeader, strconv.Itoa(ceilSeconds(d.Reset)))
			if !d.Allowed {
				h.Set(transport.RetryAfterHeader, strconv.Itoa(ceilSeconds(d.RetryAfter)))
				transport.RespondWithAPIError(r.Context(), w, apierror.NewRateLimitedError("Too many requests. Wait for the time in the Retry-After header, then retry."))
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}

// ceilSeconds rounds up, so a caller that waits the advertised time is never early.
func ceilSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Seconds()))
}
