package openapi

import (
	"reflect"
	"testing"
)

type planningSection struct {
	// Weeks of supply to hold.
	BufferWeeks int `json:"buffer_weeks"`
}

type itemWithBeta struct {
	// Item name.
	Name string `json:"name"`
	// Planning settings, in private beta.
	Planning *planningSection `json:"planning" visibility:"internal"`
}

func (*itemWithBeta) SchemaExample() any {
	return map[string]any{"name": "Sprocket", "planning": map[string]any{"buffer_weeks": 2}}
}

// A private-beta field is served to everyone but appears only in the internal reference.
func TestGenerateSchema_InternalFieldOnlyInInternalSpec(t *testing.T) {
	genMu.Lock()
	defer genMu.Unlock()
	defer activeConfig.Store(nil)

	setActive(Config{PublicOnly: true, Version: "1.0.0"})
	public := generateSchema(reflect.TypeFor[itemWithBeta](), &Components{Schemas: map[string]Schema{}}, NewDocReader())
	if _, ok := public.Properties["planning"]; ok {
		t.Error("public schema documents the internal planning field")
	}
	if ex, _ := public.Example.(map[string]any); ex == nil || ex["planning"] != nil {
		t.Errorf("public example = %v, want it without planning", public.Example)
	}

	setActive(Config{Version: "1.0.0"})
	internal := generateSchema(reflect.TypeFor[itemWithBeta](), &Components{Schemas: map[string]Schema{}}, NewDocReader())
	if _, ok := internal.Properties["planning"]; !ok {
		t.Error("internal schema leaves out the planning field")
	}
}
