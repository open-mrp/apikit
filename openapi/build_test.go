package openapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
)

// The whole fixture API, end to end: what an app gets back from Build.
func TestBuild_FixtureAPI(t *testing.T) {
	t.Parallel()

	doc, err := Build(testConfig(testGroups(), false))
	if err != nil {
		t.Fatal(err)
	}
	info := doc["info"].(map[string]any)
	if info["title"] != "Example API" || info["version"] != "1.0.0" {
		t.Errorf("info = %v", info)
	}

	paths := doc["paths"].(map[string]any)
	list := paths["/v1/catalog/widgets"].(map[string]any)["get"].(map[string]any)
	var paramNames []string
	for _, p := range list["parameters"].([]any) {
		paramNames = append(paramNames, p.(map[string]any)["name"].(string))
	}
	for _, want := range []string{"cursor", "limit", "q", "statuses[]", "include[]"} {
		if !slices.Contains(paramNames, want) {
			t.Errorf("list parameters %v missing %s", paramNames, want)
		}
	}
	if !strings.HasPrefix(list["description"].(string), "Returns a list of widgets, newest first.") {
		t.Errorf("list description %q does not come from the request's doc comment", list["description"])
	}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	spec := string(out)
	for _, want := range []string{
		`"$ref": "#/components/schemas/APIErrorResponse"`,
		`"previous_page_url"`,
		`?cursor=`,
		string(apierror.CodeIdempotencyKeyReused),
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("spec missing %s", want)
		}
	}

	public, err := Build(testConfig(testGroups(), true))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := public["paths"].(map[string]any)["/v1/catalog/widgets/{id}"].(map[string]any)["delete"]; ok {
		t.Error("public spec has the internal-only delete")
	}
}

func TestErrorCodes(t *testing.T) {
	t.Parallel()
	codes := ErrorCodes()
	i := slices.IndexFunc(codes, func(c ErrorCode) bool { return c.Code == apierror.CodeRateLimited })
	if i < 0 {
		t.Fatal("rate_limited missing")
	}
	if c := codes[i]; c.Status != 429 || !c.Transient || c.Description == "" {
		t.Errorf("rate_limited = %+v", c)
	}
	if !slices.IsSortedFunc(codes, func(a, b ErrorCode) int { return strings.Compare(string(a.Code), string(b.Code)) }) {
		t.Error("codes not sorted")
	}
	if _, err := json.Marshal(codes); err != nil {
		t.Error(err)
	}
}
