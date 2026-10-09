package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_OPTIONS(t *testing.T) {
	t.Parallel()
	r := NewRouter()

	// Add a dummy middleware that sets a header
	r.AddMiddleware(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-Applied", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		}
	})

	// Register a POST route
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Test OPTIONS request to the POST route
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	if rr.Header().Get("X-Middleware-Applied") != "true" {
		t.Error("expected middleware to be applied to OPTIONS request")
	}

	// Test OPTIONS request to a non-existent route
	req = httptest.NewRequest(http.MethodOptions, "/not-found", nil)
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status NotFound for non-existent route OPTIONS, got %v", rr.Code)
	}
}

func TestRouter_PathPattern_OPTIONS(t *testing.T) {
	t.Parallel()
	r := NewRouter()

	r.AddMiddleware(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-Applied", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		}
	})

	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test OPTIONS request to the pattern route
	req := httptest.NewRequest(http.MethodOptions, "/users/123", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	if rr.Header().Get("X-Middleware-Applied") != "true" {
		t.Error("expected middleware to be applied to OPTIONS request for pattern route")
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()
	r := NewRouter()
	noop := func(http.ResponseWriter, *http.Request) {}
	r.HandleEndpoint(http.MethodGet, "/v1/orders/{id}", noop, false)
	r.HandleEndpoint(http.MethodGet, "/v1/orders/current", noop, true)

	if pattern, public, ok := r.Match(http.MethodGet, "/v1/orders/or_1"); !ok || pattern != "/v1/orders/{id}" || public {
		t.Errorf("param route: %q %v %v", pattern, public, ok)
	}
	if pattern, public, ok := r.Match(http.MethodGet, "/v1/orders/current"); !ok || pattern != "/v1/orders/current" || !public {
		t.Errorf("literal route: %q %v %v", pattern, public, ok)
	}
	if _, _, ok := r.Match(http.MethodPost, "/v1/orders/or_1"); ok {
		t.Error("matched the wrong method")
	}
}
