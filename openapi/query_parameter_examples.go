package openapi

import (
	"fmt"
	"reflect"
	"strings"

	apiexample "github.com/open-mrp/apikit/example"
)

func queryParameterExample(reqType reflect.Type, field reflect.StructField, paramSchema Schema) any {
	if v := queryValueFromSchemaExample(reqType, field); v != nil && queryExampleSatisfiesEnum(v, paramSchema) {
		return v
	}
	queryTag := field.Tag.Get("query")
	if queryTag != "" {
		paramName := queryTag
		if paramSchema.Type == "array" && !strings.HasSuffix(paramName, "[]") {
			paramName = paramName + "[]"
		}
		if v := sampleQueryExampleForOpenAPIName(paramName); v != nil && queryExampleSatisfiesEnum(v, paramSchema) {
			return v
		}
	}
	return parameterExample(paramSchema)
}

// queryExampleSatisfiesEnum reports whether v is consistent with paramSchema's enum constraint, if it has one. A generic documented/hard-coded sample that is not a member of the parameter's enum (e.g. a `type` sample of "payment" applied to an endpoint whose `type` enum is [material_category, product_category]) must be rejected so the enum-aware fallback can substitute a valid value.
func queryExampleSatisfiesEnum(v any, paramSchema Schema) bool {
	if paramSchema.Type == "array" && paramSchema.Items != nil && len(paramSchema.Items.Enum) > 0 {
		arr, ok := v.([]any)
		if !ok {
			return false
		}
		for _, e := range arr {
			if !enumContains(paramSchema.Items.Enum, e) {
				return false
			}
		}
		return true
	}
	if len(paramSchema.Enum) > 0 {
		return enumContains(paramSchema.Enum, v)
	}
	return true
}

func enumContains(enum []any, v any) bool {
	for _, e := range enum {
		if fmt.Sprint(e) == fmt.Sprint(v) {
			return true
		}
	}
	return false
}

func queryValueFromSchemaExample(reqType reflect.Type, field reflect.StructField) any {
	queryTag := field.Tag.Get("query")
	if queryTag == "" {
		return nil
	}
	key := strings.TrimSuffix(queryTag, "[]")
	return lookupQueryInDocumentedTypes(reqType, key)
}

func lookupQueryInDocumentedTypes(reqType reflect.Type, queryKey string) any {
	if reqType.Kind() == reflect.Pointer {
		reqType = reqType.Elem()
	}
	if reqType.Kind() != reflect.Struct {
		return nil
	}
	if v := tryQueryFromDocumentedType(reqType, queryKey); v != nil {
		return v
	}
	t := reqType
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous {
			continue
		}
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() != reflect.Struct {
			continue
		}
		if v := lookupQueryInDocumentedTypes(ft, queryKey); v != nil {
			return v
		}
	}
	return nil
}

func tryQueryFromDocumentedType(t reflect.Type, queryKey string) any {
	ptr := reflect.PointerTo(t)
	if !ptr.Implements(reflect.TypeFor[apiexample.Documented]()) {
		return nil
	}
	ex := reflect.New(t).Interface().(apiexample.Documented).SchemaExample()
	if ex == nil {
		return nil
	}
	m, ok := ex.(map[string]any)
	if !ok {
		return nil
	}
	if v, ok := m[queryKey]; ok && !isEmptyQueryExampleValue(v) {
		return v
	}
	return nil
}

func isEmptyQueryExampleValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok && s == "" {
		return true
	}
	if arr, ok := v.([]any); ok && len(arr) == 0 {
		return true
	}
	return false
}

// sampleQueryExampleForOpenAPIName asks the app's Examples.QueryParam for a sample once the request type's own SchemaExample has none.
func sampleQueryExampleForOpenAPIName(openAPIParam string) any {
	if active().Examples.QueryParam == nil {
		return nil
	}
	return active().Examples.QueryParam(strings.TrimSuffix(openAPIParam, "[]"))
}
