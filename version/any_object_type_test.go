package version

import (
	"net/http"
	"testing"

	"github.com/open-mrp/apikit/object"
)

type errorDowngrader struct {
	mockTransformer
	calls *int
}

func (d errorDowngrader) TransformError(status int, body map[string]any) (int, map[string]any) {
	*d.calls++
	return http.StatusBadRequest, body
}

func TestTransformerRegistry_AnyObjectTypeHandlesEveryType(t *testing.T) {
	t.Parallel()
	registry := NewTransformerRegistry()
	registry.Register(&mockTransformer{
		from:        V1_0_Forge_Preview2,
		to:          V1_0_Forge_Preview1,
		objectTypes: []object.Type{AnyObjectType},
		transformFunc: func(_ object.Type, data map[string]any) map[string]any {
			data["seen"] = true
			return data
		},
	})
	registry.Register(&mockTransformer{
		from:        V1_0_Forge_Preview2,
		to:          V1_0_Forge_Preview1,
		objectTypes: []object.Type{testObjectUser},
		transformFunc: func(_ object.Type, data map[string]any) map[string]any {
			data["user_only"] = true
			return data
		},
	})

	for _, ot := range []object.Type{testObjectAccount, ""} {
		got := registry.Transform(V1_0_Forge_Preview2, V1_0_Forge_Preview1, ot, map[string]any{})
		if got["seen"] != true || got["user_only"] != nil {
			t.Errorf("object type %q: got %v", ot, got)
		}
	}
}

func TestTransformerRegistry_TransformErrorRunsInRange(t *testing.T) {
	t.Parallel()
	registry := NewTransformerRegistry()
	calls := 0
	registry.Register(&errorDowngrader{
		mockTransformer: mockTransformer{from: V1_0_Forge_Preview3, to: V1_0_Forge_Preview2},
		calls:           &calls,
	})

	status, _ := registry.TransformError(V1_0_Forge_Preview3, V1_0_Forge_Preview2, http.StatusUnprocessableEntity, map[string]any{})
	if status != http.StatusBadRequest || calls != 1 {
		t.Errorf("in range: status %d calls %d", status, calls)
	}
	status, _ = registry.TransformError(V1_0_Forge_Preview3, V1_0_Forge_Preview3, http.StatusUnprocessableEntity, map[string]any{})
	if status != http.StatusUnprocessableEntity || calls != 1 {
		t.Errorf("same version: status %d calls %d", status, calls)
	}
}
