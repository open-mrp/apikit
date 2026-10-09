package openapi

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"

	apiexample "github.com/open-mrp/apikit/example"
)

// buildListSchemaExample constructs the OpenAPI example for object.List[T].
// When route is non-empty, next_page_url is the list path plus a cursor query param only.
func buildListSchemaExample(components *Components, docReader *DocReader, listType reflect.Type, route string, reqType reflect.Type) map[string]any {
	if listType.Kind() == reflect.Pointer {
		listType = listType.Elem()
	}

	dataArray, nextCursor := listItemExampleData(components, docReader, listType)

	var nextPageURL, prevPageURL any
	hasNextPage := false
	hasPrevPage := false

	if nextCursor != "" && strings.TrimSpace(route) != "" {
		expandedRoute := expandDocumentationRoute(route, reqType)
		if expandedRoute != "" {
			nextPageURL = buildDocumentationPageURL(expandedRoute, nextCursor)
			hasNextPage = true
		}
	}

	return map[string]any{
		"object": "list",
		"page_info": map[string]any{
			"next_page_url":     nextPageURL,
			"previous_page_url": prevPageURL,
			"has_next_page":     hasNextPage,
			"has_previous_page": hasPrevPage,
		},
		"data": dataArray,
	}
}

func listItemExampleData(components *Components, docReader *DocReader, listType reflect.Type) ([]any, string) {
	var itemExample any
	var itemTypeName string
	if listType.Kind() != reflect.Struct {
		return nil, ""
	}

	for i := 0; i < listType.NumField(); i++ {
		f := listType.Field(i)
		jsonTag := f.Tag.Get("json")
		if jsonTag != "data" && !strings.HasPrefix(jsonTag, "data,") {
			continue
		}
		elemType := f.Type
		if elemType.Kind() != reflect.Slice {
			break
		}
		elemType = elemType.Elem()
		if elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}
		itemTypeName = getCleanTypeName(elemType)
		if itemTypeName != "" {
			if existingSchema, ok := components.Schemas[itemTypeName]; ok && existingSchema.Example != nil {
				itemExample = existingSchema.Example
			}
		}
		if itemExample == nil && reflect.PointerTo(elemType).Implements(reflect.TypeFor[apiexample.Documented]()) {
			v := reflect.New(elemType).Interface().(apiexample.Documented)
			func() {
				defer func() { recover() }()
				itemExample = v.SchemaExample()
			}()
		}
		break
	}

	if itemExample == nil {
		return []any{}, ""
	}

	if itemMap, ok := exampleAsMap(itemExample); ok {
		return []any{itemMap}, documentationNextCursor(itemTypeName, itemMap)
	}
	return []any{itemExample}, ""
}

// expandDocumentationRoute substitutes {path_param} placeholders using the same sample
// IDs as OpenAPI path parameter examples.
func expandDocumentationRoute(route string, reqType reflect.Type) string {
	if reqType.Kind() == reflect.Pointer {
		reqType = reqType.Elem()
	}
	if reqType.Kind() != reflect.Struct {
		return route
	}

	expanded := route
	for _, f := range flattenStructFields(reqType) {
		pathParam := f.Tag.Get("path")
		if pathParam == "" {
			continue
		}
		placeholder := "{" + pathParam + "}"
		if !strings.Contains(expanded, placeholder) {
			continue
		}
		ex := pathParameterExample(reqType, f, pathParam, route, Schema{Type: "string"})
		if s, ok := ex.(string); ok && s != "" {
			expanded = strings.ReplaceAll(expanded, placeholder, s)
		}
	}
	return expanded
}

// buildDocumentationPageURL returns a relative next-page URL with only the cursor param.
func buildDocumentationPageURL(route string, nextCursor string) string {
	return route + "?cursor=" + url.QueryEscape(nextCursor)
}

// exampleAsMap reads an example as a JSON object, whatever Go value supplied it: a map, or a struct as it serializes.
func exampleAsMap(example any) (map[string]any, bool) {
	if m, ok := example.(map[string]any); ok {
		return m, true
	}
	b, err := json.Marshal(example)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return nil, false
	}
	return m, true
}
