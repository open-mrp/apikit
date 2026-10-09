package conformance

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/conformance/testdata"
	"github.com/open-mrp/apikit/endpoint"
	"github.com/open-mrp/apikit/field"
	"github.com/open-mrp/apikit/object"
)

type ep[TReq, TResp any] struct {
	endpoint.APIEndpoint[TReq, TResp]
}

func (*ep[TReq, TResp]) GetHandler() http.HandlerFunc { return nil }

type widget struct {
	ID   string  `json:"id"`
	Note *string `json:"note"`
}

type listWidgetsRequest struct {
	object.ListRequest
	Statuses []string `query:"statuses"`
}

type widgetPathRequest struct {
	ID string `path:"id" validate:"required"`
}

type updateWidgetRequest struct {
	ID   string                 `path:"id" validate:"required"`
	Name field.Optional[string] `json:"name,omitzero"`
}

func goodGroups() []endpoint.APIEndpointGroup {
	return []endpoint.APIEndpointGroup{{Title: "Widgets", Endpoints: []endpoint.APIEndpointer{
		&ep[*listWidgetsRequest, *object.List[widget]]{endpoint.APIEndpoint[*listWidgetsRequest, *object.List[widget]]{
			Title: "List Widgets", Method: "GET", Route: "/v1/catalog/widgets", Public: true, SuccessStatusCode: 200,
			EndpointType: reflect.TypeFor[testdata.ListWidgets](),
		}},
		&ep[*updateWidgetRequest, *widget]{endpoint.APIEndpoint[*updateWidgetRequest, *widget]{
			Title: "Update Widget", Method: "PATCH", Route: "/v1/catalog/widgets/{id}", SuccessStatusCode: 200,
		}},
		&ep[*widgetPathRequest, *object.Deleted]{endpoint.APIEndpoint[*widgetPathRequest, *object.Deleted]{
			Title: "Delete Widget", Method: "DELETE", Route: "/v1/catalog/widgets/{id}", Public: true, SuccessStatusCode: 200,
			EndpointType: reflect.TypeFor[testdata.DeleteWidget](),
		}},
		&ep[*widgetPathRequest, *object.AsyncJob[widget]]{endpoint.APIEndpoint[*widgetPathRequest, *object.AsyncJob[widget]]{
			Title: "Rebuild Widget", Method: "POST", Route: "/v1/catalog/widgets/{id}/actions/rebuild", SuccessStatusCode: 202,
		}},
		&ep[struct{}, struct{}]{endpoint.APIEndpoint[struct{}, struct{}]{Title: "Health", Method: "GET", Route: "/healthz"}},
	}}}
}

func TestCheck_ConformingAPIPasses(t *testing.T) {
	t.Parallel()
	Assert(t, goodGroups())
}

type omittingResponse struct {
	ID    string   `json:"id"`
	Inner *innerRS `json:"inner"`
}

type innerRS struct {
	Note string `json:"note,omitempty"`
}

type badRequest struct {
	ID       string                  `path:"id"`
	Name     field.Optional[string]  `json:"name" validate:"required"`
	Clearing field.Clearable[string] `json:"clearing,omitzero"`
	Filters  []string                `query:"filters"`
}

func badGroups() []endpoint.APIEndpointGroup {
	return []endpoint.APIEndpointGroup{
		{Title: "Gadgets", Endpoints: []endpoint.APIEndpointer{
			&ep[*badRequest, *omittingResponse]{endpoint.APIEndpoint[*badRequest, *omittingResponse]{
				Title: "Update Gadget", Method: "PUT", Route: "/gadgets/{gadgetId}/actions/sync", Public: true,
				EndpointType: reflect.TypeFor[testdata.BadSummary](),
			}},
			&ep[*widgetPathRequest, *widget]{endpoint.APIEndpoint[*widgetPathRequest, *widget]{
				Title: "Delete Gadget", Method: "DELETE", Route: "/v1/catalog/gadgets/{id}", SuccessStatusCode: 204,
			}},
			&ep[*widgetPathRequest, *object.List[widget]]{endpoint.APIEndpoint[*widgetPathRequest, *object.List[widget]]{
				Title: "List Gadgets", Method: "GET", Route: "/v1/catalog/gadgets", Public: true,
			}},
			&ep[*widgetPathRequest, *widget]{endpoint.APIEndpoint[*widgetPathRequest, *widget]{
				Title: "Export Gadgets", Method: "POST", Route: "/v1/catalog/gadgets/actions/export", SuccessStatusCode: 202,
			}},
		}},
		{Title: "Duplicates", Endpoints: []endpoint.APIEndpointer{
			&ep[*widgetPathRequest, *object.List[widget]]{endpoint.APIEndpoint[*widgetPathRequest, *object.List[widget]]{
				Title: "List Gadgets Again", Method: "GET", Route: "/v1/catalog/gadgets",
			}},
		}},
	}
}

func TestCheck_ReportsEachRule(t *testing.T) {
	t.Parallel()
	violations := Check(badGroups())
	byRule := map[string][]string{}
	for _, v := range violations {
		byRule[v.Rule] = append(byRule[v.Rule], v.String())
	}
	for _, rule := range []string{
		"route_shape", "actions_are_post", "delete_returns_stub", "lists_page_with_list_request",
		"long_running_returns_async_job", "path_params_required", "optional_fields_omitzero",
		"responses_never_omit", "public_docstrings", "unique_routes",
	} {
		if len(byRule[rule]) == 0 {
			t.Errorf("no %s violation reported; got %v", rule, violations)
		}
	}
	all := strings.Join(slices.Collect(func(yield func(string) bool) {
		for _, v := range violations {
			if !yield(v.String()) {
				return
			}
		}
	}), "\n")
	for _, want := range []string{"inner.note omits empty values", "Name is a presence type without ,omitzero", "Name is a presence type and must not be validate", "succeeds with 204", "does not start with a verb"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "Clearing") {
		t.Errorf("a well-formed Clearable field was reported:\n%s", all)
	}
}

func TestCheck_SubsetOfRules(t *testing.T) {
	t.Parallel()
	for _, v := range Check(badGroups(), RouteShape) {
		if v.Rule != "route_shape" && v.Rule != "unique_routes" {
			t.Errorf("rule %s ran though only route_shape was asked for", v.Rule)
		}
	}
}
