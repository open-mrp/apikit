package openapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/pagination"
)

func TestBuildListSchemaExample_WithoutRouteUsesNullPageURLs(t *testing.T) {
	t.Parallel()
	components := &Components{Schemas: make(map[string]Schema)}
	reader := NewDocReader()
	components.Schemas["Widget"] = generateSchema(reflect.TypeOf(Widget{}), components, reader)

	listType := reflect.TypeOf(object.List[Widget]{})
	var ex map[string]any
	withActive(Config{Version: "1.0.0"}, func() {
		ex = buildListSchemaExample(components, reader, listType, "", nil)
	})

	pageInfo, ok := ex["page_info"].(map[string]any)
	if !ok {
		t.Fatalf("page_info type = %T", ex["page_info"])
	}
	if pageInfo["next_page_url"] != nil {
		t.Errorf("next_page_url = %v, want nil for nested list schemas", pageInfo["next_page_url"])
	}
	if _, ok := pageInfo["has_next_page"]; ok {
		t.Errorf("page_info has has_next_page; it is the two URLs only")
	}
}

func TestBuildListSchemaExample_PaginatedListURL(t *testing.T) {
	t.Parallel()
	components := &Components{Schemas: make(map[string]Schema)}
	reader := NewDocReader()
	components.Schemas["Widget"] = generateSchema(reflect.TypeOf(Widget{}), components, reader)

	listType := reflect.TypeOf(object.List[Widget]{})
	reqType := reflect.TypeOf(ListWidgetsRequest{})
	var ex map[string]any
	withActive(Config{Version: "1.0.0"}, func() {
		ex = buildListSchemaExample(components, reader, listType, "/v1/catalog/widgets", reqType)
	})

	pageInfo, ok := ex["page_info"].(map[string]any)
	if !ok {
		t.Fatalf("page_info type = %T", ex["page_info"])
	}
	nextURL, ok := pageInfo["next_page_url"].(string)
	if !ok || nextURL == "" {
		t.Fatalf("next_page_url = %v, want relative URL string", pageInfo["next_page_url"])
	}
	if !strings.HasPrefix(nextURL, "/v1/catalog/widgets?") {
		t.Errorf("next_page_url = %q, want path prefix /v1/catalog/widgets?", nextURL)
	}
	wantCursor := pagination.EncodeDocumentationStringCursor(sampleCreatedAt, sampleWidgetID)
	if wantCursor == sampleWidgetID {
		t.Fatal("cursor must be a signed pagination token, not a bare type ID")
	}
	if !isSignedPaginationCursor(wantCursor) {
		t.Fatalf("cursor = %q, want payload.signature form", wantCursor)
	}
	wantURL := buildDocumentationPageURL("/v1/catalog/widgets", wantCursor)
	if nextURL != wantURL {
		t.Errorf("next_page_url = %q, want %q", nextURL, wantURL)
	}
	if len(pageInfo) != 2 {
		t.Errorf("page_info = %v, want next_page_url and previous_page_url only", pageInfo)
	}
}

func TestExpandDocumentationRoute_SubstitutesPathParams(t *testing.T) {
	t.Parallel()
	reqType := reflect.TypeOf(retrieveItemPathRequest{})
	var got string
	withActive(testConfig(nil, false), func() {
		got = expandDocumentationRoute("/v1/catalog/widgets/{id}/attributes", reqType)
	})
	want := "/v1/catalog/widgets/" + sampleWidgetID + "/attributes"
	if got != want {
		t.Errorf("expandDocumentationRoute() = %q, want %q", got, want)
	}
}

func TestRouteSegmentBefore(t *testing.T) {
	t.Parallel()
	for route, want := range map[string]string{
		"/v1/catalog/widgets/{id}":                   "widgets",
		"/v1/catalog/widgets/{id}/attributes/{id}":   "widgets",
		"/v1/catalog/widgets/{widget_id}/parts/{id}": "parts",
		"/{id}": "",
	} {
		if got := RouteSegmentBefore(route, "{id}"); got != want {
			t.Errorf("RouteSegmentBefore(%q) = %q, want %q", route, got, want)
		}
	}
}
