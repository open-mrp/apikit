package metadata

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-mrp/apikit/field"
)

func ptr(s string) *string { return &s }

func TestEncode(t *testing.T) {
	assert.Nil(t, Encode(nil))
	assert.Nil(t, Encode(map[string]string{}))
	assert.JSONEq(t, `{"a":"1","blank":""}`, string(Encode(map[string]string{"a": "1", "blank": ""})), "an empty string is stored as a value")
}

func TestPatch(t *testing.T) {
	assert.False(t, Patch(Update{}).Valid, "no change leaves the column alone")
	assert.False(t, Patch(Update{Set: map[string]string{}}).Valid, "an empty set is no change")

	p := Patch(Update{Set: map[string]string{"keep": "v", "blank": ""}, Remove: []string{"drop"}})
	require.True(t, p.Valid)
	assert.JSONEq(t, `{"keep":"v","blank":"","drop":null}`, p.String)

	clear := Patch(Update{Clear: true})
	require.True(t, clear.Valid, "a clear alone still writes, merging {} into an empty object")
	assert.JSONEq(t, `{}`, clear.String)
}

func TestDecode(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want map[string]string
	}{
		{"null", nil, map[string]string{}},
		{"bytes", []byte(`{"a":"1"}`), map[string]string{"a": "1"}},
		{"string", `{"a":"1","blank":""}`, map[string]string{"a": "1", "blank": ""}},
		{"non-string value kept as JSON text", `{"n":5,"o":{"x":true}}`, map[string]string{"n": "5", "o": `{"x":true}`}},
		{"not an object", `[1]`, map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Decode(tc.in)
			require.NotNil(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCheckLimit(t *testing.T) {
	m := map[string]string{}
	for i := range MaxKeys {
		m[fmt.Sprint(i)] = "v"
	}
	assert.Nil(t, CheckLimit(m, "metadata"))

	m["one_more"] = "v"
	apiErr := CheckLimit(m, "metadata")
	require.NotNil(t, apiErr)
	assert.Equal(t, "metadata", apiErr.Param)
}

func TestFromCreateRequest(t *testing.T) {
	assert.Nil(t, FromCreateRequest(nil))
	assert.Equal(t,
		map[string]string{"a": "1", "blank": ""},
		FromCreateRequest(map[string]*string{"a": ptr("1"), "blank": ptr(""), "skip": nil}),
		"a null value has nothing to remove on a new record",
	)
}

// The request → Update conversion each update path takes, decoded from JSON as the endpoint does.
func TestUpdateFrom(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want Update
	}{
		{name: "omitted", body: `{}`},
		{name: "null clears", body: `{"metadata": null}`, want: Update{Clear: true}},
		{name: "empty object", body: `{"metadata": {}}`},
		{
			name: "set, remove and empty string",
			body: `{"metadata": {"a": "1", "b": null, "blank": ""}}`,
			want: Update{Set: map[string]string{"a": "1", "blank": ""}, Remove: []string{"b"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req struct {
				Metadata field.Clearable[map[string]*string] `json:"metadata,omitzero"`
			}
			require.NoError(t, json.Unmarshal([]byte(tc.body), &req))
			assert.Equal(t, tc.want, UpdateFrom(req.Metadata))
		})
	}
}

func TestCheckEntries(t *testing.T) {
	long := strings.Repeat("k", MaxKeyLength+1)
	assert.Nil(t, CheckEntries(map[string]*string{"a": ptr(strings.Repeat("v", MaxValueLength)), "gone": nil}, "metadata"))

	apiErr := CheckEntries(map[string]*string{long: ptr("v"), "big": ptr(strings.Repeat("v", MaxValueLength+1)), "ok": ptr("v")}, "metadata")
	require.NotNil(t, apiErr)
	assert.Equal(t, 422, apiErr.Status())
	require.Len(t, apiErr.Errors, 2)
	assert.Equal(t, "metadata.big", apiErr.Errors[0].Param)
	assert.Equal(t, "metadata."+long, apiErr.Errors[1].Param)
}
