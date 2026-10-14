package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/router"
)

type captureSaver struct{ saved []*appctx.RequestLog }

func (s *captureSaver) Save(_ context.Context, rl *appctx.RequestLog) error {
	s.saved = append(s.saved, rl)
	return nil
}

func TestRequestLog(t *testing.T) {
	t.Parallel()

	rt := router.NewRouter()
	rt.HandleEndpoint(http.MethodPost, "/v1/keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		rl, _ := appctx.GetRequestLog(r.Context())
		rl.SensitiveResponseFields = map[string]bool{"secret": true}
		if id, _ := appctx.GetRequestID(r.Context()); id != rl.ID {
			t.Errorf("request ID on context %q, log %q", id, rl.ID)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"ak_1","secret":"not-a-real-secret"}`))
	}, true)

	saver := &captureSaver{}
	var logs bytes.Buffer
	h := RequestLog(RequestLogConfig{
		Saver:     saver,
		Routes:    rt,
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		NewID:     func() string { return "rq_test" },
		SkipPaths: []string{"/healthz"},
	})(rt)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/keys/ak_1?expand=true", nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d", w.Code)
	}
	if len(saver.saved) != 1 {
		t.Fatalf("saved %d logs", len(saver.saved))
	}
	rl := saver.saved[0]
	if rl.ID != "rq_test" || rl.NormalizedRoute != "/v1/keys/{id}" || rl.StatusCode != http.StatusCreated || !rl.PublicEndpoint {
		t.Errorf("log %+v", rl)
	}
	if rl.ResponseJSON == nil || strings.Contains(*rl.ResponseJSON, "not-a-real-secret") {
		t.Errorf("response not redacted: %v", rl.ResponseJSON)
	}
	if rl.QueryJSON == nil || *rl.QueryJSON != `{"expand":"true"}` {
		t.Errorf("query %v", rl.QueryJSON)
	}

	var line map[string]any
	if err := json.Unmarshal(logs.Bytes(), &line); err != nil {
		t.Fatalf("canonical line %q: %v", logs.String(), err)
	}
	if line["type"] != "canonical-log-line" || line["http_route"] != "/v1/keys/{id}" || line["request_id"] != "rq_test" {
		t.Errorf("canonical line %v", line)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodOptions, "/v1/keys/ak_1", nil))
	if len(saver.saved) != 1 {
		t.Errorf("skipped requests were saved: %d", len(saver.saved))
	}
}

func TestRequestLog_MaxBodyBytes(t *testing.T) {
	t.Parallel()

	body := `{"data":"` + strings.Repeat("x", 300<<10) + `"}`
	for _, tc := range []struct {
		name      string
		max       int
		truncated bool
	}{
		{"default cap", 0, true},
		{"raised cap", 1 << 20, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			saver := &captureSaver{}
			h := RequestLog(RequestLogConfig{Saver: saver, MaxBodyBytes: tc.max, Logger: slog.New(slog.DiscardHandler)})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rl, _ := appctx.GetRequestLog(r.Context())
				if want := max(tc.max, DefaultMaxLoggedBodyBytes); rl.MaxBodyBytes != want {
					t.Errorf("MaxBodyBytes %d, want %d", rl.MaxBodyBytes, want)
				}
				_, _ = w.Write([]byte(body))
			}))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/things", nil))
			got := saver.saved[0].ResponseJSON
			if got == nil || strings.Contains(*got, "_truncated") != tc.truncated {
				t.Errorf("truncated = %v, want %v", got != nil && strings.Contains(*got, "_truncated"), tc.truncated)
			}
		})
	}
}
