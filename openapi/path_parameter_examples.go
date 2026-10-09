package openapi

import (
	"reflect"
	"strings"

	apiexample "github.com/open-mrp/apikit/example"
)

func pathParameterExample(reqType reflect.Type, field reflect.StructField, pathParamName, route string, paramSchema Schema) any {
	if v := pathValueFromSchemaExample(reqType, field); v != nil {
		return v
	}
	if active().Examples.PathParam != nil {
		if v := active().Examples.PathParam(pathParamName, route, field.Name); v != "" {
			return v
		}
	}
	return parameterExample(paramSchema)
}

func pathValueFromSchemaExample(reqType reflect.Type, field reflect.StructField) any {
	ptrType := reflect.PointerTo(reqType)
	if !ptrType.Implements(reflect.TypeFor[apiexample.Documented]()) {
		return nil
	}

	example := reflect.New(reqType).Interface().(apiexample.Documented).SchemaExample()
	if example == nil {
		return nil
	}

	v := reflect.ValueOf(example)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		fv := v.FieldByName(field.Name)
		if fv.IsValid() && fv.Kind() == reflect.String && fv.String() != "" {
			return fv.String()
		}
		return nil
	}

	m, ok := example.(map[string]any)
	if !ok {
		return nil
	}
	if val, ok := m[field.Name]; ok {
		return normalizePathExampleValue(val)
	}
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		key := strings.Split(jsonTag, ",")[0]
		if key != "" && key != "-" {
			if val, ok := m[key]; ok {
				return normalizePathExampleValue(val)
			}
		}
	}
	return nil
}

func normalizePathExampleValue(val any) any {
	if val == nil {
		return nil
	}
	if s, ok := val.(string); ok {
		if s == "" {
			return nil
		}
		return s
	}
	return val
}

// RouteSegmentBefore returns the path segment just before param in route, such as "widgets" for "{id}" in "/v1/catalog/widgets/{id}": the resource a generic {id} names, for an Examples.PathParam hook to pick a sample ID by.
func RouteSegmentBefore(route, param string) string {
	idx := strings.Index(route, param)
	if idx <= 0 {
		return ""
	}
	prefix := strings.TrimSuffix(route[:idx], "/")
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		return prefix[i+1:]
	}
	return prefix
}
