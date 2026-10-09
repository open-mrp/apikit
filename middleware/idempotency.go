package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/idempotency"
	"github.com/open-mrp/apikit/transport"
)

// IdempotencyConfig configures Idempotency.
type IdempotencyConfig struct {
	// Store (optional; default: nil) persists keys. Without one, the key is still put on the request context but requests are not deduplicated.
	Store idempotency.Store
	// Scope (optional) returns the caller and the account it acts in, so one caller's key never replays another's response. Without it every caller shares one scope per route.
	Scope func(r *http.Request) (callerID, tenantID *string)
	// StoreTimeout (optional; default: 30s) bounds storing a response. A timeout leaves the key locked until the store expires it, so it is generous.
	StoreTimeout time.Duration
}

// cookieTTLSeconds is how long a stored Set-Cookie may be replayed.
const cookieTTLSeconds = 300

// Idempotency makes POST and PATCH requests with an Idempotency-Key safe to retry. The first request runs and its response is stored before the client receives it; a retry with the same key and request gets that response back with Idempotent-Replayed: true; a retry while the first still runs is a 409 idempotency_in_progress; the same key with a different request is a 422 idempotency_key_reused. A response with a transient status (429, 5xx) is not stored, so the retry runs again.
func Idempotency(cfg IdempotencyConfig) func(http.HandlerFunc) http.HandlerFunc {
	storeTimeout := cfg.StoreTimeout
	if storeTimeout <= 0 {
		storeTimeout = 30 * time.Second
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(transport.IdempotencyKeyHeader)
			if key == "" || (r.Method != http.MethodPost && r.Method != http.MethodPatch) {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			if len(key) > idempotency.MaxKeyLength {
				transport.RespondWithAPIError(ctx, w, apierror.NewParameterInvalidError(transport.IdempotencyKeyHeader,
					fmt.Sprintf("The Idempotency-Key header must be at most %d characters.", idempotency.MaxKeyLength)))
				return
			}
			if cfg.Store == nil {
				next.ServeHTTP(w, r.WithContext(appctx.WithIdempotencyKey(ctx, key)))
				return
			}

			body, r, apiErr := readAndRestoreBody(w, r)
			if apiErr != nil {
				transport.RespondWithAPIError(ctx, w, apiErr)
				return
			}
			route := r.URL.Path
			if pattern, ok := appctx.GetRoutePattern(ctx); ok && pattern != "" {
				route = pattern
			}
			var callerID, tenantID *string
			if cfg.Scope != nil {
				callerID, tenantID = cfg.Scope(r)
			}
			query := map[string]string{}
			for k, vs := range r.URL.Query() {
				if len(vs) > 0 {
					query[k] = vs[0]
				}
			}
			pathParams, _ := appctx.GetPathParams(ctx)
			var requestID string
			rl, hasRL := appctx.GetRequestLog(ctx)
			if hasRL {
				requestID = rl.ID
			}

			beginCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			res, err := cfg.Store.Begin(beginCtx, idempotency.BeginRequest{
				Key:         key,
				ScopeHash:   idempotency.ComputeHTTPScopeHash(callerID, tenantID, r.Method, route, key),
				RequestHash: idempotency.ComputeRequestBodyHash(body, query, pathParams),
				Method:      r.Method,
				Route:       route,
				RequestID:   requestID,
				CallerID:    callerID,
				TenantID:    tenantID,
			})
			cancel()
			if err != nil {
				transport.RespondWithAPIError(ctx, w, asAPIError(err, "beginning idempotent request"))
				return
			}
			if hasRL && res.KeyID != "" {
				rl.IdempotencyKeyID = &res.KeyID
			}

			switch res.Outcome {
			case idempotency.OutcomeMismatch:
				transport.RespondWithAPIError(ctx, w, apierror.NewIdempotencyKeyReusedError(key))
			case idempotency.OutcomeInProgress:
				transport.RespondWithAPIError(ctx, w, apierror.NewIdempotencyInProgressError(key))
			case idempotency.OutcomeReplay:
				replay(w, res.Response)
			default:
				ctx = appctx.WithIdempotencyKey(ctx, key)
				ctx = appctx.WithIdempotencyKeyID(ctx, res.KeyID)
				rec := newResponseRecorder(w)
				next.ServeHTTP(rec, r.WithContext(ctx))

				// Stored before the client sees the response, so a retry fired the moment it arrives finds the key complete rather than in progress.
				storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
				if isTransientStatus(rec.statusCode) {
					err = cfg.Store.Release(storeCtx, res.KeyID)
				} else {
					stored := idempotency.StoredResponse{StatusCode: rec.statusCode, Body: rec.body.Bytes(), Cookies: rec.cookies()}
					if len(stored.Cookies) > 0 {
						stored.CookieTTLSeconds = cookieTTLSeconds
					}
					err = cfg.Store.Complete(storeCtx, res.KeyID, stored)
				}
				cancel()
				if err != nil {
					slog.ErrorContext(ctx, "storing idempotent response failed", "key_id", res.KeyID, "error", err)
				}
				rec.flush(w)
			}
		}
	}
}

func replay(w http.ResponseWriter, resp *idempotency.StoredResponse) {
	code, body := http.StatusOK, []byte("{}")
	if resp != nil {
		for _, c := range resp.Cookies {
			http.SetCookie(w, c)
		}
		if resp.StatusCode != 0 {
			code = resp.StatusCode
		}
		if resp.Body != nil {
			body = resp.Body
		}
	}
	w.Header().Set(transport.IdempotentReplayedHeader, "true")
	w.Header().Set(transport.ContentTypeHeader, "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body) // #nosec G705 - the stored response of the original request
}

func asAPIError(err error, msg string) *apierror.APIError {
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return apierror.NewServiceUnavailableError(err, msg)
}

// readAndRestoreBody buffers the body for hashing and restores it for the handler, refusing one over the largest any endpoint accepts.
func readAndRestoreBody(w http.ResponseWriter, r *http.Request) ([]byte, *http.Request, *apierror.APIError) {
	if r.Body == nil {
		return nil, r, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, transport.MaxJSONBodyBytes)
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, r, transport.NewBodyTooLargeError(transport.MaxJSONBodyBytes)
		}
		body = nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, r, nil
}

func isTransientStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// responseRecorder buffers the handler's response so it can be stored before the client receives it.
type responseRecorder struct {
	headers    http.Header
	statusCode int
	body       bytes.Buffer
	written    bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{headers: w.Header().Clone(), statusCode: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header { return r.headers }

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.body.Write(b)
}

func (r *responseRecorder) cookies() []*http.Cookie {
	return (&http.Response{Header: http.Header{"Set-Cookie": r.headers["Set-Cookie"]}}).Cookies()
}

// flush writes the buffered response to the client, once, after it has been stored.
func (r *responseRecorder) flush(w http.ResponseWriter) {
	maps.Copy(w.Header(), r.headers)
	if r.written {
		w.WriteHeader(r.statusCode)
	}
	if r.body.Len() > 0 {
		_, _ = w.Write(r.body.Bytes())
	}
}
