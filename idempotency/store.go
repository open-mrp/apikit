// Package idempotency lets a client safely retry a write: the first request with an Idempotency-Key runs, and a retry with the same key gets the first response back instead of running again. The middleware package applies it to requests; an app provides the Store.
package idempotency

import (
	"context"
	"net/http"
)

// MaxKeyLength is the longest Idempotency-Key accepted.
const MaxKeyLength = 255

// Outcome is what Begin found for a key.
type Outcome int

const (
	// OutcomeNew means the key is unused: the request runs, and its response is stored with Complete.
	OutcomeNew Outcome = iota
	// OutcomeReplay means the key already has a stored response, which is returned as is.
	OutcomeReplay
	// OutcomeInProgress means a request with the key is still running.
	OutcomeInProgress
	// OutcomeMismatch means the key was used with a different request.
	OutcomeMismatch
)

// BeginRequest describes a request claiming a key.
type BeginRequest struct {
	// Key is the client's Idempotency-Key.
	Key string
	// ScopeHash binds the key to the caller, method and route (ComputeHTTPScopeHash), so another caller's or another endpoint's use of the same key is a different key.
	ScopeHash string
	// RequestHash fingerprints the body and parameters (ComputeRequestBodyHash); a stored key with a different one is a mismatch.
	RequestHash string
	// Method and Route are the request's method and route pattern.
	Method, Route string
	// RequestID is the request log's ID.
	RequestID string
	// CallerID and TenantID are the caller and the account it acts in, when known.
	CallerID, TenantID *string
}

// StoredResponse is a response kept for replay.
type StoredResponse struct {
	StatusCode int
	Body       []byte
	// Cookies are the response's Set-Cookie headers, replayed only within CookieTTLSeconds since they may carry short-lived sessions.
	Cookies          []*http.Cookie
	CookieTTLSeconds int
}

// BeginResult is what Begin found.
type BeginResult struct {
	Outcome Outcome
	// KeyID identifies the stored key, for Complete and Release, and for the request log.
	KeyID string
	// Response is the stored response for OutcomeReplay.
	Response *StoredResponse
}

// Store persists idempotency keys. Keys are kept for 24 hours. An implementation must make Begin atomic: of two concurrent requests with one key, exactly one sees OutcomeNew.
type Store interface {
	// Begin claims the key for a new request, or reports a replay, a request in flight, or a mismatch.
	Begin(ctx context.Context, req BeginRequest) (BeginResult, error)
	// Complete stores the response of the request that claimed keyID and releases its lock.
	Complete(ctx context.Context, keyID string, resp StoredResponse) error
	// Release frees keyID without storing a response, after a transient failure, so a retry runs again.
	Release(ctx context.Context, keyID string) error
}
