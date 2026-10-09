package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/version"
)

var (
	testVersionOld = version.MustNew("1.0.test")
	testVersionNew = version.MustNew("2.0.test")
)

func init() {
	version.RegisterVersions(testVersionOld, testVersionNew)
}

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

func errorCode(t *testing.T, w *httptest.ResponseRecorder) apierror.Code {
	t.Helper()
	var resp apierror.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return resp.Error.Code
}

func TestVersion(t *testing.T) {
	t.Parallel()

	var seen version.APIVersion
	h := Version("/healthz")(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = appctx.GetAPIVersionFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/v1/things", nil))
	if w.Code != http.StatusBadRequest || errorCode(t, w) != apierror.CodeAPIVersionRequired {
		t.Errorf("missing header: %d %s", w.Code, w.Body.String())
	}

	r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r.Header.Set(version.Header(), "9.9.nope")
	w = httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest || errorCode(t, w) != apierror.CodeAPIVersionInvalid {
		t.Errorf("unknown version: %d %s", w.Code, w.Body.String())
	}

	r = httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r.Header.Set(version.Header(), "1.0.test")
	w = httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusNoContent || !seen.Equal(testVersionOld) {
		t.Errorf("known version: %d, context %v", w.Code, seen)
	}

	w = httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("skipped path: %d", w.Code)
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	h := CORS(CORSConfig{AllowedOrigins: []string{"https://app.example.com"}, AllowHeaders: []string{"Example-Account"}})(ok)

	r := httptest.NewRequest(http.MethodOptions, "/v1/things", nil)
	r.Header.Set("Origin", "https://app.example.com")
	r = r.WithContext(appctx.WithAllowedMethods(r.Context(), []string{"GET", "POST"}))
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("preflight status %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allow origin %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("allow methods %q", got)
	}
	allow := w.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"Idempotency-Key", version.Header(), "Example-Account", "traceparent"} {
		if !strings.Contains(allow, want) {
			t.Errorf("allow headers missing %s: %s", want, allow)
		}
	}
	if expose := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "Sunset") || !strings.Contains(expose, version.Header()) {
		t.Errorf("expose headers %s", expose)
	}

	r = httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w = httptest.NewRecorder()
	h(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin got %q", got)
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("a disallowed origin still reaches the handler; the browser enforces CORS: %d", w.Code)
	}
}

func TestCORS_AnyOriginByDefault(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r.Header.Set("Origin", "https://anywhere.example.com")
	w := httptest.NewRecorder()
	CORS(CORSConfig{})(ok)(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://anywhere.example.com" {
		t.Errorf("allow origin %q", got)
	}
}

func TestRecover(t *testing.T) {
	t.Parallel()
	rl := &appctx.RequestLog{ID: "rq_1"}
	r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r = r.WithContext(appctx.WithRequestLog(r.Context(), rl))
	w := httptest.NewRecorder()
	Recover()(func(http.ResponseWriter, *http.Request) { panic("boom") })(w, r)
	if w.Code != http.StatusInternalServerError || errorCode(t, w) != apierror.CodeInternalError {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
	if rl.ErrorMessage == nil {
		t.Error("panic not recorded on the request log")
	}
}

func TestIPBlock(t *testing.T) {
	t.Parallel()
	h := IPBlock([]string{"203.0.113.9"}, 0)(ok)

	r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	r.RemoteAddr = "203.0.113.9:4000"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("blocked IP got %d", w.Code)
	}

	r.RemoteAddr = "198.51.100.1:4000"
	w = httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("other IP got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SecurityHeaders()(ok)(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("headers %v", w.Header())
	}
}

