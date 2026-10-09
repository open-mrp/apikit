package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/version"
)

var (
	testVersionOld = func() version.APIVersion {
		v := version.MustNew("1.0.test")
		v.DeprecatedAt = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
		v.SunsetAt = time.Date(2028, time.March, 1, 0, 0, 0, 0, time.UTC)
		return v
	}()
	testVersionNew = version.MustNew("2.0.test")
)

func init() {
	version.RegisterVersions(testVersionOld, testVersionNew)
}

func TestRespondWithJSON_versionHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	RespondWithJSON(context.Background(), w, http.StatusOK, map[string]string{})
	if got := w.Header().Get(version.Header()); got != "2.0.test" {
		t.Errorf("no version in context: header = %q, want the latest", got)
	}
	if w.Header().Get(DeprecationHeader) != "" || w.Header().Get(SunsetHeader) != "" {
		t.Error("latest version announced a deprecation")
	}

	w = httptest.NewRecorder()
	ctx := appctx.WithAPIVersion(context.Background(), testVersionOld)
	RespondWithJSON(ctx, w, http.StatusOK, map[string]string{})
	if got := w.Header().Get(version.Header()); got != "1.0.test" {
		t.Errorf("header = %q", got)
	}
	if got := w.Header().Get(DeprecationHeader); got != "@1772323200" {
		t.Errorf("Deprecation = %q", got)
	}
	if got := w.Header().Get(SunsetHeader); got != "Wed, 01 Mar 2028 00:00:00 GMT" {
		t.Errorf("Sunset = %q", got)
	}
}
