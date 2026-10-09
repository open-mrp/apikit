package appctx

import (
	"context"
	"testing"
)

type testIdentity struct {
	ActorID string
}

type otherIdentity struct{}

func TestIdentity_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := WithIdentity(context.Background(), &testIdentity{ActorID: "us_1"})

	got, ok := Identity[*testIdentity](ctx)
	if !ok || got.ActorID != "us_1" {
		t.Fatalf("got %+v, %v", got, ok)
	}
	if any, ok := AnyIdentity(ctx); !ok || any.(*testIdentity).ActorID != "us_1" {
		t.Errorf("AnyIdentity = %+v, %v", any, ok)
	}
}

func TestIdentity_Absent(t *testing.T) {
	t.Parallel()
	if got, ok := Identity[*testIdentity](context.Background()); ok || got != nil {
		t.Errorf("got %+v, %v on a context with no identity", got, ok)
	}
	if _, ok := AnyIdentity(context.Background()); ok {
		t.Error("AnyIdentity reported an identity on an empty context")
	}
}

func TestIdentity_WrongTypeIsAbsent(t *testing.T) {
	t.Parallel()
	ctx := WithIdentity(context.Background(), &testIdentity{ActorID: "us_1"})
	if _, ok := Identity[*otherIdentity](ctx); ok {
		t.Error("an identity of another type was returned")
	}
	if _, ok := Identity[testIdentity](ctx); ok {
		t.Error("a pointer identity was returned as a value")
	}
}

// ok reports only that a T is stored: a stored nil pointer round-trips as (nil, true), so callers must
// nil-check before dereferencing.
func TestIdentity_NilPointerReportsPresent(t *testing.T) {
	t.Parallel()
	ctx := WithIdentity[*testIdentity](context.Background(), nil)
	got, ok := Identity[*testIdentity](ctx)
	if !ok || got != nil {
		t.Errorf("got %+v, %v; want nil, true", got, ok)
	}
}

func TestIdentity_ChildShadowsParent(t *testing.T) {
	t.Parallel()
	parent := WithIdentity(context.Background(), &testIdentity{ActorID: "us_parent"})
	child := WithIdentity(parent, &testIdentity{ActorID: "us_child"})
	if got, _ := Identity[*testIdentity](child); got.ActorID != "us_child" {
		t.Errorf("got %s", got.ActorID)
	}
	if got, _ := Identity[*testIdentity](parent); got.ActorID != "us_parent" {
		t.Errorf("parent changed to %s", got.ActorID)
	}
}
