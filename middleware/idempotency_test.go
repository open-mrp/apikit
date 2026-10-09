package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/idempotency"
)

// fakeStore keeps keys in memory, as a test double for an app's Store.
type fakeStore struct {
	mu       sync.Mutex
	keys     map[string]*fakeKey
	released []string
}

type fakeKey struct {
	id, requestHash string
	done            bool
	resp            idempotency.StoredResponse
}

func newFakeStore() *fakeStore { return &fakeStore{keys: map[string]*fakeKey{}} }

func (s *fakeStore) Begin(_ context.Context, req idempotency.BeginRequest) (idempotency.BeginResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[req.ScopeHash]
	switch {
	case !ok:
		k = &fakeKey{id: "ik_" + req.Key, requestHash: req.RequestHash}
		s.keys[req.ScopeHash] = k
		return idempotency.BeginResult{Outcome: idempotency.OutcomeNew, KeyID: k.id}, nil
	case k.requestHash != req.RequestHash:
		return idempotency.BeginResult{Outcome: idempotency.OutcomeMismatch, KeyID: k.id}, nil
	case !k.done:
		return idempotency.BeginResult{Outcome: idempotency.OutcomeInProgress, KeyID: k.id}, nil
	default:
		resp := k.resp
		return idempotency.BeginResult{Outcome: idempotency.OutcomeReplay, KeyID: k.id, Response: &resp}, nil
	}
}

func (s *fakeStore) Complete(_ context.Context, keyID string, resp idempotency.StoredResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if k.id == keyID {
			k.done, k.resp = true, resp
		}
	}
	return nil
}

func (s *fakeStore) Release(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for scope, k := range s.keys {
		if k.id == keyID {
			delete(s.keys, scope)
		}
	}
	s.released = append(s.released, keyID)
	return nil
}

func idemRequest(key, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	r.Header.Set("Idempotency-Key", key)
	return r.WithContext(appctx.WithRequestLog(r.Context(), &appctx.RequestLog{ID: "rq_1"}))
}

func TestIdempotency_RunsOnceThenReplays(t *testing.T) {
	t.Parallel()
	runs := 0
	h := Idempotency(IdempotencyConfig{Store: newFakeStore()})(func(w http.ResponseWriter, r *http.Request) {
		runs++
		if key, _ := appctx.GetIdempotencyKey(r.Context()); key != "k1" {
			t.Errorf("key on context = %q", key)
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1"})
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"or_1"}`))
	})

	first := httptest.NewRecorder()
	h(first, idemRequest("k1", `{"a":1}`))
	second := httptest.NewRecorder()
	h(second, idemRequest("k1", `{ "a" : 1 }`))

	if runs != 1 {
		t.Fatalf("handler ran %d times", runs)
	}
	if first.Code != http.StatusCreated || first.Header().Get("Idempotent-Replayed") != "" {
		t.Errorf("first: %d %v", first.Code, first.Header())
	}
	if second.Code != http.StatusCreated || second.Body.String() != `{"id":"or_1"}` || second.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay: %d %v %s", second.Code, second.Header(), second.Body.String())
	}
	if !strings.Contains(second.Header().Get("Set-Cookie"), "session=s1") {
		t.Errorf("cookie not replayed: %v", second.Header())
	}
}

func TestIdempotency_KeyReusedWithADifferentRequest(t *testing.T) {
	t.Parallel()
	h := Idempotency(IdempotencyConfig{Store: newFakeStore()})(ok)
	h(httptest.NewRecorder(), idemRequest("k1", `{"a":1}`))
	w := httptest.NewRecorder()
	h(w, idemRequest("k1", `{"a":2}`))
	if w.Code != http.StatusUnprocessableEntity || errorCode(t, w) != apierror.CodeIdempotencyKeyReused {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
}

func TestIdempotency_InProgress(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	var inner *httptest.ResponseRecorder
	var h http.HandlerFunc
	h = Idempotency(IdempotencyConfig{Store: store})(func(w http.ResponseWriter, r *http.Request) {
		inner = httptest.NewRecorder()
		h(inner, idemRequest("k1", `{}`))
		w.WriteHeader(http.StatusOK)
	})
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	if inner.Code != http.StatusConflict || errorCode(t, inner) != apierror.CodeIdempotencyInProgress {
		t.Errorf("concurrent retry: %d %s", inner.Code, inner.Body.String())
	}
}

func TestIdempotency_TransientFailureIsReleased(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	runs := 0
	h := Idempotency(IdempotencyConfig{Store: store})(func(w http.ResponseWriter, _ *http.Request) {
		runs++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	if runs != 2 || len(store.released) != 2 {
		t.Errorf("runs %d, released %v", runs, store.released)
	}
}

func TestIdempotency_KeyTooLong(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	Idempotency(IdempotencyConfig{Store: newFakeStore()})(ok)(w, idemRequest(strings.Repeat("k", 256), `{}`))
	if w.Code != http.StatusBadRequest || errorCode(t, w) != apierror.CodeParameterInvalid {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
}

func TestIdempotency_ScopesKeysByCaller(t *testing.T) {
	t.Parallel()
	runs := 0
	caller := "us_1"
	h := Idempotency(IdempotencyConfig{
		Store: newFakeStore(),
		Scope: func(*http.Request) (*string, *string) { c := caller; return &c, nil },
	})(func(w http.ResponseWriter, _ *http.Request) { runs++; w.WriteHeader(http.StatusOK) })
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	caller = "us_2"
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	if runs != 2 {
		t.Errorf("another caller's key replayed: runs %d", runs)
	}
}

func TestIdempotency_WithoutStoreOrKey(t *testing.T) {
	t.Parallel()
	var key string
	h := Idempotency(IdempotencyConfig{})(func(w http.ResponseWriter, r *http.Request) {
		key, _ = appctx.GetIdempotencyKey(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h(httptest.NewRecorder(), idemRequest("k1", `{}`))
	if key != "k1" {
		t.Errorf("key not propagated without a store: %q", key)
	}
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/v1/orders", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET: %d", w.Code)
	}
}
