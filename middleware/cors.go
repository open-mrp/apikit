package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/transport"
	"github.com/open-mrp/apikit/version"
)

// CORSConfig configures CORS.
type CORSConfig struct {
	// AllowedOrigins (optional; default: any origin) lists the origins allowed to call the API with credentials. Empty reflects any origin.
	AllowedOrigins []string
	// AllowHeaders (optional) adds request headers to the built-in ones (content type, authorization, idempotency key, the API version header, trace context and the Stainless SDK headers), e.g. an app's account header.
	AllowHeaders []string
	// ExposeHeaders (optional) adds response headers to the built-in ones (rate limit, request ID, the API version header, deprecation, sunset, content disposition).
	ExposeHeaders []string
}

var baseAllowHeaders = []string{
	"Content-Type", "Authorization", "X-Requested-With", "X-Retry", transport.IdempotencyKeyHeader, "Accept", "Origin",
	"User-Agent", "Cache-Control", "Pragma", "traceparent", "tracestate",
	"X-Stainless-Arch", "X-Stainless-Lang", "X-Stainless-OS", "X-Stainless-Package-Version", "X-Stainless-Read-Timeout",
	"X-Stainless-Retry-Count", "X-Stainless-Runtime", "X-Stainless-Runtime-Version", "X-Stainless-Timeout",
}

// Content-Disposition is not a CORS-safelisted response header, so a browser drops it unless it is named: file downloads put the filename there.
var baseExposeHeaders = []string{
	transport.RateLimitLimitHeader, transport.RateLimitRemainingHeader, transport.RateLimitResetHeader,
	transport.RequestIDHeader, transport.DeprecationHeader, transport.SunsetHeader, "Content-Disposition",
}

// CORS answers preflight requests and sets the CORS headers on every response. The allowed methods are the matched route's, so it must run inside the router.
func CORS(cfg CORSConfig) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			origin := r.Header.Get("Origin")
			switch {
			case origin != "" && (len(cfg.AllowedOrigins) == 0 || slices.Contains(cfg.AllowedOrigins, origin)):
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Add("Vary", "Origin")
			case origin == "" && len(cfg.AllowedOrigins) == 0:
				h.Set("Access-Control-Allow-Origin", "*")
			default:
				h.Add("Vary", "Origin")
			}

			if methods, ok := appctx.GetAllowedMethods(r.Context()); ok {
				h.Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
			}
			allow := append(append(slices.Clone(baseAllowHeaders), version.Header()), cfg.AllowHeaders...)
			h.Set("Access-Control-Allow-Headers", strings.Join(allow, ", "))
			expose := append(append(slices.Clone(baseExposeHeaders), version.Header()), cfg.ExposeHeaders...)
			h.Set("Access-Control-Expose-Headers", strings.Join(expose, ", "))
			h.Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
