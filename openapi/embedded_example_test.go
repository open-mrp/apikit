package openapi

import (
	"reflect"
	"testing"
)

type embeddedBase struct {
	// The base ID.
	ID string `json:"id"`
}

type embeddingResource struct {
	embeddedBase
	// The extra field.
	Extra string `json:"extra"`
}

func (embeddingResource) SchemaExample() any {
	return map[string]any{"id": "b_1", "extra": "x"}
}

// An example keeps the fields an embedded struct contributes, which the schema lists through allOf.
func TestGenerateSchema_ExampleKeepsEmbeddedFields(t *testing.T) {
	t.Parallel()
	components := &Components{Schemas: make(map[string]Schema)}

	schema := generateSchema(reflect.TypeOf(embeddingResource{}), components, NewDocReader())

	example, ok := schema.Example.(map[string]any)
	if !ok || example["id"] != "b_1" || example["extra"] != "x" {
		t.Errorf("example %#v, want both the embedded and the own field", schema.Example)
	}
	if len(schema.AllOf) != 1 {
		t.Errorf("allOf %v, want the embedded struct's schema", schema.AllOf)
	}
}
