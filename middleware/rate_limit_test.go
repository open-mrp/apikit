package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/ratelimit"
)

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string) (ratelimit.Decision, error) {
	return ratelimit.Decision{}, errors.New("redis down")
}

func TestRateLimit(t *testing.T) {
	t.Parallel()
	h := RateLimit(RateLimitConfig{Limiter: ratelimit.NewMemory(1, time.Minute), SkipPaths: []string{"/healthz"}})(ok)

	req := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "198.51.100.1:5000"
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}

	w := req("/v1/things")
	if w.Code != http.StatusNoContent || w.Header().Get("RateLimit-Limit") != "1" || w.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("first request: %d %v", w.Code, w.Header())
	}
	w = req("/v1/things")
	if w.Code != http.StatusTooManyRequests || errorCode(t, w) != apierror.CodeRateLimited || w.Header().Get("Retry-After") == "" {
		t.Fatalf("second request: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	if w := req("/healthz"); w.Code != http.StatusNoContent {
		t.Errorf("skipped path limited: %d", w.Code)
	}
}

func TestRateLimit_FailsOpen(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	RateLimit(RateLimitConfig{Limiter: failingLimiter{}})(ok)(w, httptest.NewRequest(http.MethodGet, "/v1/things", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("got %d", w.Code)
	}
}
