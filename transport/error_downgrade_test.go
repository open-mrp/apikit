package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/version"
)

// errorDowngrade serves testVersionOld callers validation failures as the 400 they used to be.
type errorDowngrade struct{}

func (errorDowngrade) FromVersion() version.APIVersion { return testVersionNew }
func (errorDowngrade) ToVersion() version.APIVersion   { return testVersionOld }
func (errorDowngrade) ObjectTypes() []object.Type      { return nil }
func (errorDowngrade) Transform(_ object.Type, d map[string]any) map[string]any {
	return d
}
func (errorDowngrade) TransformRequest(_ object.Type, d map[string]any) map[string]any {
	return d
}
func (errorDowngrade) TransformError(status int, body map[string]any) (int, map[string]any) {
	if status == http.StatusUnprocessableEntity {
		status = http.StatusBadRequest
	}
	body["error"].(map[string]any)["downgraded"] = true
	return status, body
}

func init() {
	version.RegisterTransformer(errorDowngrade{})
}

func TestRespondWithAPIError_downgradesForAnOlderVersion(t *testing.T) {
	t.Parallel()

	apiErr := apierror.NewMissingFieldError("name", "name is required.")

	w := httptest.NewRecorder()
	RespondWithAPIError(context.Background(), w, apiErr)
	if w.Code != http.StatusUnprocessableEntity || containsKey(t, w.Body.Bytes(), "downgraded") {
		t.Errorf("latest version: status %d body %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	RespondWithAPIError(appctx.WithAPIVersion(context.Background(), testVersionOld), w, apiErr)
	if w.Code != http.StatusBadRequest || !containsKey(t, w.Body.Bytes(), "downgraded") {
		t.Errorf("older version: status %d body %s", w.Code, w.Body.String())
	}
}

func containsKey(t *testing.T, body []byte, key string) bool {
	t.Helper()
	var resp struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	_, ok := resp.Error[key]
	return ok
}
