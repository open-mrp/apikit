package openapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func findDescriptor(t *testing.T, descriptors []AgentTool, slug string) AgentTool {
	t.Helper()
	for _, d := range descriptors {
		if d.Slug == slug {
			return d
		}
	}
	t.Fatalf("no agent-tool descriptor with slug %q (got %d descriptors)", slug, len(descriptors))
	return AgentTool{}
}

func paramByName(d AgentTool, name string) (AgentToolParam, bool) {
	for _, p := range d.Params {
		if p.Name == name {
			return p, true
		}
	}
	return AgentToolParam{}, false
}

func testAgentTools(t *testing.T) []AgentTool {
	t.Helper()
	tools, err := AgentTools(testConfig(testGroups(), false))
	if err != nil {
		t.Fatal(err)
	}
	return tools
}

// TestAgentToolDescriptors verifies that endpoints flagged AgentTool are turned into tool descriptors with self-contained schemas and a correct param-location map, and that others are not.
func TestAgentToolDescriptors(t *testing.T) {
	descriptors := testAgentTools(t)
	slugs := make([]string, len(descriptors))
	for i, d := range descriptors {
		slugs[i] = d.Slug
	}
	if want := []string{"create_widget", "list_widgets", "update_widget"}; !slices.Equal(slugs, want) {
		t.Fatalf("slugs = %v, want %v (only endpoints flagged AgentTool, sorted)", slugs, want)
	}

	// Schemas handed to the LLM must be self-contained: no component refs or
	// OpenAPI-only noise that the model cannot resolve.
	for _, d := range descriptors {
		s := string(d.InputSchema)
		for _, banned := range []string{"$ref", "allOf", "x-stainless", "x-expandable"} {
			if strings.Contains(s, banned) {
				t.Errorf("%s: input schema must not contain %q (not self-contained):\n%s", d.Slug, banned, s)
			}
		}
		var obj map[string]any
		if err := json.Unmarshal(d.InputSchema, &obj); err != nil {
			t.Errorf("%s: input schema is not valid JSON: %v", d.Slug, err)
			continue
		}
		if obj["type"] != "object" {
			t.Errorf("%s: input schema root type = %v, want object", d.Slug, obj["type"])
		}
	}

	t.Run("create_widget body params", func(t *testing.T) {
		d := findDescriptor(t, descriptors, "create_widget")
		if d.Method != "POST" || d.Group != "Widgets" || d.Auth != requirePermission("widgets:write") {
			t.Errorf("method %s, group %q, auth %v", d.Method, d.Group, d.Auth)
		}
		p, ok := paramByName(d, "name")
		if !ok || p.In != "body" {
			t.Fatalf("name param = %+v, %v", p, ok)
		}
		var obj struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(d.InputSchema, &obj); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(obj.Required, "name") {
			t.Errorf("required = %v, want it to include name", obj.Required)
		}
	})

	t.Run("list_widgets query params and includes", func(t *testing.T) {
		d := findDescriptor(t, descriptors, "list_widgets")
		p, ok := paramByName(d, "statuses")
		if !ok || p.In != "query" || !p.Array {
			t.Fatalf("statuses param = %+v, %v", p, ok)
		}
		if p, ok := paramByName(d, "include"); !ok || p.In != "query" || !p.Array {
			t.Errorf("include param = %+v, %v", p, ok)
		}
	})

	t.Run("update_widget path param", func(t *testing.T) {
		d := findDescriptor(t, descriptors, "update_widget")
		if p, ok := paramByName(d, "id"); !ok || p.In != "path" {
			t.Errorf("id param = %+v, %v", p, ok)
		}
	})
}

func TestToSnakeSlug(t *testing.T) {
	cases := map[string]string{
		"List Widgets":      "list_widgets",
		"Create Widget":     "create_widget",
		"  Spaced  Title  ": "spaced_title",
	}
	for in, want := range cases {
		if got := toSnakeSlug(in); got != want {
			t.Errorf("toSnakeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEndpointToolSchemasAreEmbedded(t *testing.T) {
	descriptors := testAgentTools(t)
	for _, d := range descriptors {
		if len(d.InputSchema) == 0 {
			t.Errorf("%s: InputSchema must be embedded in the catalog (no DB seed exists)", d.Slug)
		}
	}
}

func TestRewriteNullableToTypeUnion(t *testing.T) {
	in := json.RawMessage(`{
		"type":"object",
		"properties":{
			"note":{"type":"string","nullable":true},
			"name":{"type":"string"},
			"credit_limit":{"type":"object","nullable":true,"properties":{"value":{"type":"string"}}},
			"tags":{"type":"array","nullable":true,"items":{"type":"string"}},
			"already":{"type":["string","null"],"nullable":true}
		}
	}`)
	out, err := rewriteNullableToTypeUnion(in)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Properties map[string]struct {
			Type     any   `json:"type"`
			Nullable *bool `json:"nullable"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	for name, p := range got.Properties {
		if p.Nullable != nil {
			t.Errorf("%s: nullable keyword should be removed, still present", name)
		}
	}
	assertTypeUnion := func(name string, want string) {
		got, ok := got.Properties[name].Type.([]any)
		if !ok {
			t.Errorf("%s: type should be an array, got %#v", name, got)
			return
		}
		if len(got) != 2 || got[0] != want || got[1] != "null" {
			t.Errorf("%s: type = %v, want [%q null]", name, got, want)
		}
	}
	assertTypeUnion("note", "string")
	assertTypeUnion("credit_limit", "object")
	assertTypeUnion("tags", "array")
	// A non-nullable field keeps its scalar type.
	if got.Properties["name"].Type != "string" {
		t.Errorf("name: type = %#v, want scalar \"string\"", got.Properties["name"].Type)
	}
	// An already-unioned type does not get a duplicate "null".
	if arr, ok := got.Properties["already"].Type.([]any); !ok || len(arr) != 2 {
		t.Errorf("already: type = %#v, want no duplicate null", got.Properties["already"].Type)
	}
}

// TestAgentToolNullableFieldsUseTypeUnion guards the end-to-end output: a clearable field comes
// through as a JSON-Schema null union (so models send a real null to clear it), never OpenAPI `nullable`.
func TestAgentToolNullableFieldsUseTypeUnion(t *testing.T) {
	descriptors := testAgentTools(t)
	for _, d := range descriptors {
		if strings.Contains(string(d.InputSchema), `"nullable"`) {
			t.Errorf("%s: InputSchema still contains OpenAPI \"nullable\"; models ignore it, use a \"null\" type member", d.Slug)
		}
	}

	d := findDescriptor(t, descriptors, "update_widget")
	var schema struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(d.InputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	arr, ok := schema.Properties["instructions"].Type.([]any)
	if !ok || !slices.Contains(arr, any("null")) {
		t.Errorf("instructions type = %#v, want a union containing \"null\"", schema.Properties["instructions"].Type)
	}
}
