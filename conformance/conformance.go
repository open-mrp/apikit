// Package conformance checks an app's endpoints against the forge.1 API conventions. An app runs it from its own tests:
//
//	func TestAPIConformance(t *testing.T) {
//		conformance.Assert(t, myapp.EndpointGroups())
//	}
//
// Each Rule inspects one endpoint by reflection over its definition and request and response types, so a violation fails the build rather than reaching a client. Pass a subset of rules, or add an app's own, to Check or Assert.
package conformance

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/endpoint"
	"github.com/open-mrp/apikit/field"
	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/openapi"
)

// Endpoint is what a Rule sees of one endpoint.
type Endpoint struct {
	Group        string
	Title        string
	Method       string
	Route        string
	Public       bool
	Preview      bool
	SuccessCode  int
	RequestType  reflect.Type
	ResponseType reflect.Type
	// Doc is the doc comment on the endpoint's own type, the operation description the reference publishes.
	Doc string
}

// Rule returns one message per way the endpoint breaks it.
type Rule struct {
	Name  string
	Check func(Endpoint) []string
}

// Violation is one broken rule on one endpoint.
type Violation struct {
	Rule, Method, Route, Message string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s %s: %s: %s", v.Method, v.Route, v.Rule, v.Message)
}

// Rules are the forge.1 conventions checked by default.
var Rules = []Rule{
	RouteShape,
	ActionsArePost,
	DeleteReturnsStub,
	ListsPageWithListRequest,
	LongRunningReturnsAsyncJob,
	PathParamsRequired,
	OptionalFieldsOmitZero,
	ResponsesNeverOmit,
	PublicDocstrings,
}

// Check runs rules (Rules when none are given) on every endpoint in groups, plus the cross-endpoint check that no two claim one method and route.
func Check(groups []endpoint.APIEndpointGroup, rules ...Rule) []Violation {
	if len(rules) == 0 {
		rules = Rules
	}
	docs := openapi.NewDocReader()
	var out []Violation
	seen := map[string]string{}
	for _, g := range groups {
		for _, e := range g.Endpoints {
			ep := describe(g.Title, e, docs)
			key := ep.Method + " " + ep.Route
			if other, dup := seen[key]; dup {
				out = append(out, Violation{"unique_routes", ep.Method, ep.Route, fmt.Sprintf("also defined in the %q group; only one would be served", other)})
			}
			seen[key] = g.Title
			for _, r := range rules {
				for _, msg := range r.Check(ep) {
					out = append(out, Violation{r.Name, ep.Method, ep.Route, msg})
				}
			}
		}
	}
	return out
}

// Assert fails t once per violation.
func Assert(t testing.TB, groups []endpoint.APIEndpointGroup, rules ...Rule) {
	t.Helper()
	for _, v := range Check(groups, rules...) {
		t.Error(v.String())
	}
}

func describe(group string, e endpoint.APIEndpointer, docs *openapi.DocReader) Endpoint {
	spec := reflect.ValueOf(e)
	for spec.Kind() == reflect.Pointer {
		spec = spec.Elem()
	}
	if inner := spec.FieldByName("APIEndpoint"); inner.IsValid() {
		spec = inner
	}
	ep := Endpoint{
		Group:        group,
		Title:        stringField(spec, "Title"),
		Method:       strings.ToUpper(e.GetMethod()),
		Route:        e.GetRoute(),
		Public:       e.IsPublic(),
		RequestType:  deref(e.GetRequestType()),
		ResponseType: deref(e.GetResponseType()),
	}
	if f := spec.FieldByName("Preview"); f.IsValid() {
		ep.Preview = f.Bool()
	}
	if f := spec.FieldByName("SuccessStatusCode"); f.IsValid() {
		ep.SuccessCode = int(f.Int())
	}
	if f := spec.FieldByName("EndpointType"); f.IsValid() && !f.IsNil() {
		ep.Doc = docs.GetTypeDoc(f.Interface().(reflect.Type)).Doc
	}
	return ep
}

func stringField(v reflect.Value, name string) string {
	if f := v.FieldByName(name); f.IsValid() && f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}

func deref(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

var routeRE = regexp.MustCompile(`^/v1/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*(/(\{[a-z][a-z0-9_]*\}|[a-z][a-z0-9-]*))*$`)

// RouteShape: every route is /v1/{namespace}/{resource}..., lowercase, hyphenated, with snake_case path parameters. Only /healthz is exempt.
var RouteShape = Rule{"route_shape", func(e Endpoint) []string {
	if e.Route == "/healthz" || routeRE.MatchString(e.Route) {
		return nil
	}
	return []string{"routes are /v1/{namespace}/{resource}..., lowercase and hyphenated, with snake_case {path_params}"}
}}

// ActionsArePost: a non-CRUD operation is POST .../actions/{verb}.
var ActionsArePost = Rule{"actions_are_post", func(e Endpoint) []string {
	if strings.Contains(e.Route, "/actions/") && e.Method != http.MethodPost {
		return []string{"actions are POST .../actions/{verb}"}
	}
	return nil
}}

var deletedType = reflect.TypeFor[object.Deleted]()

// DeleteReturnsStub: a DELETE answers 200 with {id, object, deleted}.
var DeleteReturnsStub = Rule{"delete_returns_stub", func(e Endpoint) []string {
	if e.Method != http.MethodDelete {
		return nil
	}
	var msgs []string
	if e.ResponseType != deletedType {
		msgs = append(msgs, fmt.Sprintf("returns %v; a delete returns object.Deleted", e.ResponseType))
	}
	if e.SuccessCode != 0 && e.SuccessCode != http.StatusOK {
		msgs = append(msgs, fmt.Sprintf("succeeds with %d; a delete succeeds with 200", e.SuccessCode))
	}
	return msgs
}}

var listRequestType = reflect.TypeFor[object.ListRequest]()

// ListsPageWithListRequest: an endpoint returning a page of resources accepts the standard paging parameters (limit 25 by default, 100 at most) by embedding object.ListRequest.
var ListsPageWithListRequest = Rule{"lists_page_with_list_request", func(e Endpoint) []string {
	if e.Method != http.MethodGet || !isGeneric(e.ResponseType, "List") || e.ResponseType.PkgPath() != listRequestType.PkgPath() {
		return nil
	}
	if !embeds(e.RequestType, listRequestType) {
		return []string{"returns a list but its request does not embed object.ListRequest"}
	}
	return nil
}}

// LongRunningReturnsAsyncJob: an endpoint that answers 202 hands back the async job to poll.
var LongRunningReturnsAsyncJob = Rule{"long_running_returns_async_job", func(e Endpoint) []string {
	if e.SuccessCode != http.StatusAccepted {
		return nil
	}
	if isGeneric(e.ResponseType, "AsyncJob") || embedsGeneric(e.ResponseType, "AsyncJob") {
		return nil
	}
	return []string{fmt.Sprintf("answers 202 with %v; long-running work returns an object.AsyncJob", e.ResponseType)}
}}

// PathParamsRequired: a path-bound request field is validate:"required", so a missing or misbound path parameter is refused rather than reaching the service empty.
var PathParamsRequired = Rule{"path_params_required", func(e Endpoint) []string {
	var msgs []string
	eachField(e.RequestType, func(sf reflect.StructField) {
		if sf.Tag.Get("path") != "" && !hasRule(sf.Tag.Get("validate"), "required") {
			msgs = append(msgs, fmt.Sprintf("path field %s is not validate:\"required\"", sf.Name))
		}
	})
	return msgs
}}

// OptionalFieldsOmitZero: a field.Optional or field.Clearable request field carries ,omitzero, so an unset value round-trips as absent, and is never validate:"required", since it is optional by type.
var OptionalFieldsOmitZero = Rule{"optional_fields_omitzero", func(e Endpoint) []string {
	var msgs []string
	eachField(e.RequestType, func(sf reflect.StructField) {
		if !field.IsOptionalType(sf.Type) && !field.IsClearableType(sf.Type) {
			return
		}
		if json := sf.Tag.Get("json"); json != "-" && !strings.Contains(json, ",omitzero") {
			msgs = append(msgs, fmt.Sprintf("%s is a presence type without ,omitzero in its json tag", sf.Name))
		}
		if v := sf.Tag.Get("validate"); hasRule(v, "required") || strings.Contains(v, "required_") {
			msgs = append(msgs, fmt.Sprintf("%s is a presence type and must not be validate:\"required\"", sf.Name))
		}
	})
	return msgs
}}

// ResponsesNeverOmit: response fields carry no omitempty or omitzero, so every member is always present, null when it has no value.
var ResponsesNeverOmit = Rule{"responses_never_omit", func(e Endpoint) []string {
	var msgs []string
	walkResponse(e.ResponseType, "", map[reflect.Type]bool{}, func(path string, sf reflect.StructField) {
		if json := sf.Tag.Get("json"); strings.Contains(json, ",omitempty") || strings.Contains(json, ",omitzero") {
			msgs = append(msgs, fmt.Sprintf("response field %s omits empty values; return null instead", path))
		}
	})
	return msgs
}}

// DocVerbs are the verbs a public endpoint's summary may start with. Add an app's own before running the checks.
var DocVerbs = []string{
	"Returns", "Lists", "Retrieves", "Creates", "Updates", "Deletes", "Archives", "Restores", "Cancels",
	"Starts", "Stops", "Pauses", "Resumes", "Completes", "Closes", "Reopens", "Sends", "Resends", "Runs",
	"Merges", "Searches", "Validates", "Verifies", "Previews", "Exports", "Imports", "Generates", "Calculates",
	"Sets", "Adds", "Removes", "Replaces", "Approves", "Rejects", "Issues", "Voids", "Refreshes", "Revokes",
	"Rotates", "Uploads", "Downloads", "Links", "Unlinks", "Assigns", "Unassigns", "Moves", "Copies",
	"Converts", "Counts", "Estimates", "Accepts", "Declines", "Invites", "Marks", "Records", "Registers",
	"Schedules", "Splits", "Picks", "Ships", "Receives", "Allocates", "Releases", "Reserves", "Applies",
}

// PublicDocstrings: a public endpoint is documented on its own type with a verb-first summary sentence ("Returns a list of...", "Creates a..."), since that becomes the reference page and the agent tool's description.
var PublicDocstrings = Rule{"public_docstrings", func(e Endpoint) []string {
	if !e.Public {
		return nil
	}
	if strings.TrimSpace(e.Doc) == "" {
		return []string{"public endpoint has no doc comment on its endpoint type"}
	}
	first, _, _ := strings.Cut(e.Doc, " ")
	if !slices.Contains(DocVerbs, first) {
		return []string{fmt.Sprintf("summary %q does not start with a verb such as Returns, Creates or Updates (see DocVerbs)", firstLine(e.Doc))}
	}
	return nil
}}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func hasRule(validate, rule string) bool {
	for _, r := range strings.Split(validate, ",") {
		if r == rule {
			return true
		}
	}
	return false
}

func isGeneric(t reflect.Type, name string) bool {
	return t != nil && strings.HasPrefix(t.Name(), name+"[")
}

func embeds(t, target reflect.Type) bool {
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}
	for sf := range t.Fields() {
		if sf.Anonymous && (deref(sf.Type) == target || embeds(deref(sf.Type), target)) {
			return true
		}
	}
	return false
}

func embedsGeneric(t reflect.Type, name string) bool {
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}
	for sf := range t.Fields() {
		if sf.Anonymous && (isGeneric(deref(sf.Type), name) || embedsGeneric(deref(sf.Type), name)) {
			return true
		}
	}
	return false
}

// eachField calls fn for every field of a struct, flattening embedded structs.
func eachField(t reflect.Type, fn func(reflect.StructField)) {
	if t == nil || t.Kind() != reflect.Struct {
		return
	}
	for sf := range t.Fields() {
		if sf.Anonymous && deref(sf.Type).Kind() == reflect.Struct && !field.IsOptionalType(sf.Type) && !field.IsClearableType(sf.Type) {
			eachField(deref(sf.Type), fn)
			continue
		}
		if sf.IsExported() {
			fn(sf)
		}
	}
}

// walkResponse visits every JSON field reachable from a response type, once per type.
func walkResponse(t reflect.Type, prefix string, seen map[reflect.Type]bool, fn func(string, reflect.StructField)) {
	t = deref(t)
	if t == nil {
		return
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		walkResponse(t.Elem(), prefix, seen, fn)
		return
	case reflect.Struct:
	default:
		return
	}
	if seen[t] || t.PkgPath() == "time" {
		return
	}
	seen[t] = true
	for sf := range t.Fields() {
		if !sf.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if sf.Anonymous && name == "" {
			walkResponse(sf.Type, prefix, seen, fn)
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		fn(path, sf)
		walkResponse(sf.Type, path, seen, fn)
	}
}
