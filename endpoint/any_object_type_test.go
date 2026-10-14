package endpoint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/version"
)

var olderVersion = version.MustNew("1.0.forge-preview.7")

// markEveryResponse downgrades every response to olderVersion, whatever its endpoint's object type.
type markEveryResponse struct{}

func (markEveryResponse) FromVersion() version.APIVersion { return version.MustParse("1.0.forge-preview.8") }
func (markEveryResponse) ToVersion() version.APIVersion   { return olderVersion }
func (markEveryResponse) ObjectTypes() []object.Type      { return []object.Type{version.AnyObjectType} }
func (markEveryResponse) Transform(_ object.Type, d map[string]any) map[string]any {
	d["downgraded"] = true
	return d
}
func (markEveryResponse) TransformRequest(_ object.Type, d map[string]any) map[string]any {
	return d
}

func init() {
	version.RegisterVersions(olderVersion)
	version.RegisterTransformer(markEveryResponse{})
}

func TestExecute_AnyObjectTypeTransformerReachesEndpointsWithoutOne(t *testing.T) {
	t.Parallel()

	ep := &APIEndpoint[*stubRequest, *stubResponse]{
		Method:            http.MethodGet,
		Route:             "/v1/things",
		SuccessStatusCode: http.StatusOK,
		ServiceHandler: func(any) ServiceHandler[*stubRequest, *stubResponse] {
			return func(context.Context, *stubRequest) (*stubResponse, *apierror.APIError) {
				return &stubResponse{ID: "th_1"}, nil
			}
		},
	}
	bindHandler(ep)

	for _, tc := range []struct {
		v    version.APIVersion
		want bool
	}{{version.Latest(), false}, {olderVersion, true}} {
		r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
		r = r.WithContext(appctx.WithAPIVersion(r.Context(), tc.v))
		w := httptest.NewRecorder()
		ep.Execute(w, r)
		if got := strings.Contains(w.Body.String(), `"downgraded":true`); got != tc.want {
			t.Errorf("%s: downgraded = %v, body %s", tc.v, got, w.Body.String())
		}
	}
}
