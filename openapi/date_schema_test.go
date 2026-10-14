package openapi

import (
	"reflect"
	"testing"

	"github.com/open-mrp/apikit/field"
	"github.com/open-mrp/apikit/timeutil"
)

type dateSchemaStruct struct {
	ShipOn     timeutil.Date                  `json:"ship_on"`
	ExpiresOn  *timeutil.Date                 `json:"expires_on"`
	PromisedOn field.Clearable[timeutil.Date] `json:"promised_on,omitzero"`
	DueOn      field.Optional[timeutil.Date]  `json:"due_on,omitzero"`
}

// A timeutil.Date is a YYYY-MM-DD string in the spec, never an object with year, month and day.
func TestGenerateSchema_DateIsAStringDate(t *testing.T) {
	t.Parallel()
	components := &Components{Schemas: make(map[string]Schema)}

	if got := generateSchema(reflect.TypeFor[timeutil.Date](), components, NewDocReader()); got.Type != "string" || got.Format != "date" {
		t.Errorf("bare Date schema = %s/%s, want string/date", got.Type, got.Format)
	}

	schema := generateSchema(reflect.TypeFor[dateSchemaStruct](), components, NewDocReader())
	for _, name := range []string{"ship_on", "expires_on", "promised_on", "due_on"} {
		prop := schema.Properties[name]
		if prop.Type != "string" || prop.Format != "date" || len(prop.AllOf) != 0 || prop.Properties != nil {
			t.Errorf("%s = %+v, want type string format date", name, prop)
		}
	}
	if _, ok := components.Schemas["Date"]; ok {
		t.Error("Date was registered as a component schema")
	}
}
