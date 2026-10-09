package endpoint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/include"
	"github.com/open-mrp/apikit/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fixture include graph, registered the way an app registers its own: an order has a buyer and lines; a line has a product; a product has a category and links back to its order (a cycle).
func init() {
	RegisterIncludes(&ObjectIncludes{ObjectType: "test_order", Fields: []IncludeFieldDef{
		{Key: "buyer", ObjectType: "test_account"},
		{Key: "lines", ObjectType: "test_line"},
		{Key: "notes", ObjectType: "test_order", JSONPath: "internal_notes"},
	}})
	RegisterIncludes(&ObjectIncludes{ObjectType: "test_line", Fields: []IncludeFieldDef{
		{Key: "product", ObjectType: "test_product"},
	}})
	RegisterIncludes(&ObjectIncludes{ObjectType: "test_product", Fields: []IncludeFieldDef{
		{Key: "category", ObjectType: "test_category"},
		{Key: "last_order", ObjectType: "test_order"},
	}})
	RegisterIncludes(&ObjectIncludes{ObjectType: "test_account", Fields: []IncludeFieldDef{
		{Key: "owner", ObjectType: "test_user", Children: []IncludeFieldDef{{Key: "role", ObjectType: "test_role"}}},
	}})
}

func TestIncludesFor_DirectFields(t *testing.T) {
	t.Parallel()
	cfg := IncludesFor(IncludesParams{ObjectType: "test_order", Fields: []string{"buyer", "lines", "notes"}})
	require.Len(t, cfg.Fields, 3)
	assert.Equal(t, IncludeField{Key: "buyer", ObjectType: "test_account", JSONPaths: []string{"buyer"}}, cfg.Fields[0])
	assert.Equal(t, IncludeField{Key: "lines", ObjectType: "test_line", JSONPaths: []string{"lines"}}, cfg.Fields[1])
	assert.Equal(t, IncludeField{Key: "notes", ObjectType: "test_order", JSONPaths: []string{"internal_notes"}}, cfg.Fields[2], "JSONPath overrides the key")
}

func TestIncludesFor_NestedThroughTheRegistry(t *testing.T) {
	t.Parallel()
	cfg := IncludesFor(IncludesParams{ObjectType: "test_order", Fields: []string{"lines.product.category", "buyer.owner.role"}})
	require.Len(t, cfg.Fields, 2)
	assert.Equal(t, IncludeField{Key: "lines.product.category", ObjectType: "test_category", JSONPaths: []string{"lines.product.category"}}, cfg.Fields[0])
	assert.Equal(t, IncludeField{Key: "buyer.owner.role", ObjectType: "test_role", JSONPaths: []string{"buyer.owner.role"}}, cfg.Fields[1], "inline Children")
}

func TestIncludesFor_WithPathPrefix(t *testing.T) {
	t.Parallel()
	cfg := IncludesFor(IncludesParams{ObjectType: "test_line", Fields: []string{"product"}, PathPrefix: "line_info"})
	require.Len(t, cfg.Fields, 1)
	assert.Equal(t, []string{"line_info.product"}, cfg.Fields[0].JSONPaths)
}

// A type already being expanded along a branch is not expanded again below itself: product.last_order resolves, and order's fields do too, but the walk ends instead of recursing forever.
func TestIncludesFor_BreaksCycles(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() {
		IncludesFor(IncludesParams{ObjectType: "test_product", Fields: []string{"last_order", "last_order.buyer"}})
	})
}

func TestIncludesFor_PanicsOnUnknownField(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		IncludesFor(IncludesParams{
			ObjectType: "test_order",
			Fields:     []string{"nonexistent"},
		})
	})
}

func TestIncludesFor_PanicsOnEmptyFields(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		IncludesFor(IncludesParams{
			ObjectType: "test_order",
			Fields:     []string{},
		})
	})
}

// TestIncludesFor_PanicsOnKeyDeeperThanResolver pins that a key the resolver would reject at request time is caught at startup instead. The chain is synthetic because no real resource graph is currently this deep.
func TestIncludesFor_PanicsOnKeyDeeperThanResolver(t *testing.T) {
	// Deliberately not parallel: this is the only test that writes to the package-global registry, which every other test reads. Go runs serial tests to completion before resuming parallel ones, so staying serial is what keeps the write off the same clock as those reads.

	// A chain of distinct types deep enough to overshoot the resolver cap by one: link0 -> link1 -> ... The cycle-breaking in walkFields means a self-referencing type would not expand far enough.
	depth := include.DefaultMaxIncludeDepth + 1
	linkType := func(i int) object.Type {
		return object.Type(fmt.Sprintf("test_deep_link_%d", i))
	}
	for i := 0; i <= depth; i++ {
		oi := &ObjectIncludes{ObjectType: linkType(i)}
		if i < depth {
			oi.Fields = []IncludeFieldDef{{Key: "next", ObjectType: linkType(i + 1)}}
		}
		RegisterIncludes(oi)
		// The synthetic types exist only for this test, so they are removed again rather than left in a package-global other tests walk. Without this, a second run in the same process panics on duplicate registration.
		t.Cleanup(func() { delete(registry, oi.ObjectType) })
	}

	segments := make([]string, depth)
	for i := range segments {
		segments[i] = "next"
	}

	// One segment short of the cap resolves fine.
	require.NotPanics(t, func() {
		IncludesFor(IncludesParams{ObjectType: linkType(0), Fields: []string{strings.Join(segments[:depth-1], ".")}})
	})

	assert.Panics(t, func() {
		IncludesFor(IncludesParams{ObjectType: linkType(0), Fields: []string{strings.Join(segments, ".")}})
	})
}

func TestIncludesFor_PanicsOnUnregisteredType(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		IncludesFor(IncludesParams{
			ObjectType: object.Type("nonexistent"),
			Fields:     []string{"anything"},
		})
	})
}
