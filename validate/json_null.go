package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	apierror "github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/field"
)

// RejectExplicitJSONNulls returns a 422 listing every field the JSON body sets to an explicit null or a blank string on an optional pointer field (json omitempty), or to a blank string on a field.Optional field. Absent keys are allowed (PATCH semantics).
//
// field.Clearable values accept null (clear) and are not checked here. field.Optional values reject explicit null at unmarshal time; this pass only rejects a present-but-blank string for them. Response-style pointers without omitempty are not checked here.
func RejectExplicitJSONNulls(body []byte, v any) *apierror.APIError {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	var fields []apierror.FieldError
	collectExplicitNulls(rv, rv.Type(), raw, &fields)
	if len(fields) == 0 {
		return nil
	}
	return newValidationError(fields)
}

func collectExplicitNulls(rv reflect.Value, rt reflect.Type, raw map[string]json.RawMessage, fields *[]apierror.FieldError) {
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.PkgPath != "" {
			continue
		}

		if sf.Anonymous {
			switch {
			case sf.Type.Kind() == reflect.Struct:
				collectExplicitNulls(rv.Field(i), sf.Type, raw, fields)
			case sf.Type.Kind() == reflect.Pointer && sf.Type.Elem().Kind() == reflect.Struct:
				fv := rv.Field(i)
				if fv.IsNil() {
					continue
				}
				collectExplicitNulls(fv.Elem(), sf.Type.Elem(), raw, fields)
			}
			continue
		}

		if field.IsClearableType(sf.Type) {
			// Clearable accepts null (clear) by design.
			continue
		}
		if field.IsOptionalType(sf.Type) {
			// Optional rejects an explicit null at unmarshal time; here we additionally reject a present-but-blank string so an empty value is a 422 rather than silently set to "". Non-string Optionals never carry a blank string here (it would have failed to unmarshal into the inner type).
			if fe, ok := blankStringError(sf, raw); ok {
				*fields = append(*fields, fe)
			}
			continue
		}

		if sf.Type.Kind() != reflect.Pointer || !jsonTagHasOmitempty(sf.Tag.Get("json")) {
			continue
		}

		jsonName := jsonFieldNameFromTag(sf.Tag.Get("json"))
		if jsonName == "" || jsonName == "-" {
			continue
		}

		rm, ok := raw[jsonName]
		if !ok {
			continue
		}
		var parsed any
		if err := json.Unmarshal(rm, &parsed); err != nil {
			continue
		}
		if parsed == nil {
			*fields = append(*fields, apierror.Field(jsonName, apierror.CodeInvalidFormat, fmt.Sprintf("Field '%s' cannot be null.", jsonName)))
			continue
		}
		if sf.Type.Elem().Kind() == reflect.String {
			if str, ok := parsed.(string); ok && strings.TrimSpace(str) == "" {
				*fields = append(*fields, apierror.Field(jsonName, apierror.CodeInvalidFormat, fmt.Sprintf("Field '%s' must not be blank.", jsonName)))
			}
		}
	}
}

// ApplySlicePresenceFlags sets boolean "Has" companion fields to true when the corresponding slice field's JSON key is present in the raw body. This lets downstream code distinguish "field absent" (Has=false) from "field explicitly sent" (Has=true), including empty arrays to clear the collection.
//
// Convention: a slice field `FooIDs []string` with json tag "foo_ids" has a companion `HasFooIDs bool` with json:"-". When "foo_ids" appears in the JSON body, HasFooIDs is set to true.
func ApplySlicePresenceFlags(body []byte, v any) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for sf := range rt.Fields() {
		if sf.PkgPath != "" || sf.Type.Kind() != reflect.Slice {
			continue
		}

		jsonName := jsonFieldNameFromTag(sf.Tag.Get("json"))
		if jsonName == "" || jsonName == "-" {
			continue
		}

		if _, ok := raw[jsonName]; !ok {
			continue
		}

		hasFld, ok := rt.FieldByName("Has" + sf.Name)
		if !ok || hasFld.Type.Kind() != reflect.Bool {
			continue
		}
		rv.FieldByName("Has" + sf.Name).SetBool(true)
	}
}

// blankStringError reports an invalid_format failure when sf's JSON key is present and its value is a blank string. Absent keys and non-string values pass.
func blankStringError(sf reflect.StructField, raw map[string]json.RawMessage) (apierror.FieldError, bool) {
	jsonName := jsonFieldNameFromTag(sf.Tag.Get("json"))
	if jsonName == "" || jsonName == "-" {
		return apierror.FieldError{}, false
	}
	rm, ok := raw[jsonName]
	if !ok {
		return apierror.FieldError{}, false
	}
	var parsed any
	if err := json.Unmarshal(rm, &parsed); err != nil {
		return apierror.FieldError{}, false
	}
	if str, ok := parsed.(string); ok && strings.TrimSpace(str) == "" {
		return apierror.Field(jsonName, apierror.CodeInvalidFormat, fmt.Sprintf("Field '%s' must not be blank.", jsonName)), true
	}
	return apierror.FieldError{}, false
}

func jsonTagHasOmitempty(tag string) bool {
	if tag == "" || tag == "-" {
		return false
	}
	return slices.Contains(strings.Split(tag, ",")[1:], "omitempty")
}

func jsonFieldNameFromTag(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	before, _, ok := strings.Cut(tag, ",")
	if ok {
		return before
	}
	return tag
}
