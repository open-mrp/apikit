package openapi

import (
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/open-mrp/apikit/endpoint"
	"github.com/open-mrp/apikit/field"
	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/openapi/testdata"
)

// A fixture API, standing in for an app's endpoint groups: widgets with CRUD endpoints, an include, an access policy and agent-tool flags.

const (
	sampleWidgetID    = "wd_0f3k2m9a1b7c"
	sampleAPIKeyID    = "apke_7d2c9e1f0a3b"
	sampleAttributeID = "at_4b8e1c0d9f2a"
	sampleOwnerID     = "us_9c1d7e3f5a2b"
)

var sampleCreatedAt = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

// requirePermission is the fixture's access policy.
type requirePermission string

// widgetStatus is a fixture enum.
type widgetStatus string

func (s widgetStatus) IsValid() bool { return s == "active" || s == "archived" }

func (widgetStatus) EnumValues() []string { return []string{"active", "archived"} }

// A widget, the fixture's resource.
type Widget struct {
	// Widget ID.
	ID string `json:"id"`
	// Always "widget".
	Object object.Type `json:"object"`
	// The widget's name.
	Name string `json:"name"`
	// Whether the widget can be used.
	Status widgetStatus `json:"status"`
	// Instructions for handling the widget, or null.
	Instructions *string `json:"instructions"`
	// The widget's owner. Null unless included.
	Owner *Owner `json:"owner" expandable:"true"`
	// When the widget was created.
	CreatedAt time.Time `json:"created_at"`
}

func (*Widget) SchemaExample() any {
	return &Widget{ID: sampleWidgetID, Object: "widget", Name: "Sprocket", Status: "active", CreatedAt: sampleCreatedAt}
}

// The user who owns a widget.
type Owner struct {
	// User ID.
	ID string `json:"id"`
	// The user's name.
	Name string `json:"name"`
}

func (*Owner) SchemaExample() any {
	return &Owner{ID: sampleOwnerID, Name: "Ada"}
}

// Creates a widget.
type CreateWidgetRequest struct {
	// The widget's name.
	Name string `json:"name" validate:"required"`
	// Whether the widget can be used. Defaults to active().
	Status field.Optional[widgetStatus] `json:"status,omitzero"`
}

func (*CreateWidgetRequest) SchemaExample() any {
	return map[string]any{"name": "Sprocket", "status": "active"}
}

// Updates a widget. Only the fields sent change.
type UpdateWidgetRequest struct {
	ID string `path:"id" validate:"required"`
	// The widget's name.
	Name field.Optional[string] `json:"name,omitzero"`
	// Instructions for handling the widget. Send null to remove them.
	Instructions field.Clearable[string] `json:"instructions,omitzero"`
	// Whether the widget is featured.
	Featured field.Optional[bool] `json:"featured,omitzero"`
}

func (*UpdateWidgetRequest) SchemaExample() any {
	return map[string]any{"id": sampleWidgetID, "name": "Sprocket", "instructions": nil, "featured": false}
}

type RetrieveWidgetRequest struct {
	WidgetID string `path:"id" validate:"required"`
}

// Lists widgets.
type ListWidgetsRequest struct {
	object.ListRequest
	// Matches the start of a widget's name, case-insensitive.
	Query *string `query:"q"`
	// Only widgets with one of these statuses.
	Statuses []widgetStatus `query:"statuses"`
}

type DeleteWidgetRequest struct {
	ID string `path:"id" validate:"required"`
}

type widgetEndpoint[TReq, TResp any] struct {
	endpoint.APIEndpoint[TReq, TResp]
}

func (e *widgetEndpoint[TReq, TResp]) GetHandler() http.HandlerFunc { return nil }

func testGroups() []endpoint.APIEndpointGroup {
	return []endpoint.APIEndpointGroup{{
		Title:       "Widgets",
		Description: "Widgets are the fixture's resource.",
		Endpoints: []endpoint.APIEndpointer{
			&widgetEndpoint[*CreateWidgetRequest, *Widget]{endpoint.APIEndpoint[*CreateWidgetRequest, *Widget]{
				Title: "Create Widget", Method: http.MethodPost, Route: "/v1/catalog/widgets",
				SuccessStatusCode: http.StatusCreated, Public: true, AgentTool: true, Auth: requirePermission("widgets:write"),
			}},
			&widgetEndpoint[*ListWidgetsRequest, *object.List[Widget]]{endpoint.APIEndpoint[*ListWidgetsRequest, *object.List[Widget]]{
				Title: "List Widgets", Method: http.MethodGet, Route: "/v1/catalog/widgets",
				SuccessStatusCode: http.StatusOK, Public: true, AgentTool: true, Auth: requirePermission("widgets:read"),
				IncludeConfig: &endpoint.IncludeConfig{Fields: []endpoint.IncludeField{{Key: "data.owner", ObjectType: "user", JSONPaths: []string{"data.owner"}}}},
				EndpointType:  reflect.TypeFor[testdata.ListWidgetsEndpoint](),
			}},
			&widgetEndpoint[*RetrieveWidgetRequest, *Widget]{endpoint.APIEndpoint[*RetrieveWidgetRequest, *Widget]{
				Title: "Retrieve Widget", Method: http.MethodGet, Route: "/v1/catalog/widgets/{id}",
				SuccessStatusCode: http.StatusOK, Public: true, Auth: requirePermission("widgets:read"),
				IncludeConfig: &endpoint.IncludeConfig{Fields: []endpoint.IncludeField{{Key: "owner", ObjectType: "user", JSONPaths: []string{"owner"}}}},
			}},
			&widgetEndpoint[*UpdateWidgetRequest, *Widget]{endpoint.APIEndpoint[*UpdateWidgetRequest, *Widget]{
				Title: "Update Widget", Method: http.MethodPatch, Route: "/v1/catalog/widgets/{id}",
				SuccessStatusCode: http.StatusOK, Public: true, AgentTool: true, Auth: requirePermission("widgets:write"),
			}},
			&widgetEndpoint[*DeleteWidgetRequest, *object.Deleted]{endpoint.APIEndpoint[*DeleteWidgetRequest, *object.Deleted]{
				Title: "Delete Widget", Method: http.MethodDelete, Route: "/v1/catalog/widgets/{id}",
				SuccessStatusCode: http.StatusOK, Public: false, Auth: requirePermission("widgets:write"),
			}},
		},
	}}
}

// testConfig is the Config an app would pass, around the given groups.
func testConfig(groups []endpoint.APIEndpointGroup, publicOnly bool) Config {
	return Config{
		Groups:      groups,
		Title:       "Example API",
		Description: "The fixture API.",
		Version:     "1.0.0",
		PublicOnly:  publicOnly,
		Servers:     []Server{{URL: "https://api.example.com", Description: "Production server"}},
		SecuritySchemes: map[string]SecuritySchemeSpec{
			"BearerAuth": {Type: "http", Scheme: "bearer", Description: "API key as a Bearer token."},
		},
		Security: []map[string][]string{{"BearerAuth": {}}, {}},
		DescribeAuth: func(policy any) string {
			if p, ok := policy.(requirePermission); ok {
				return "This endpoint requires the permission: `" + string(p) + "`."
			}
			return ""
		},
		Examples: Examples{
			PathParam: func(param, route, fieldName string) string {
				switch {
				case fieldName == "APIKeyID":
					return sampleAPIKeyID
				case param == "attribute_id":
					return sampleAttributeID
				case param == "id" && strings.Contains(route, "/widgets/"):
					return sampleWidgetID
				}
				return ""
			},
			QueryParam: func(param string) any {
				if param == "q" {
					return "spro"
				}
				return nil
			},
		},
	}
}

// withActive runs fn with cfg active, for tests that call the generator's internals directly.
func withActive(cfg Config, fn func()) {
	genMu.Lock()
	defer genMu.Unlock()
	setActive(cfg)
	fn()
}
