package include

import (
	"context"
	"sync"
	"testing"

	apierror "github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/object"
)

const (
	otOrder    object.Type = "test_order"
	otLine     object.Type = "test_line"
	otProduct  object.Type = "test_product"
	otCategory object.Type = "test_category"
	otBuyer    object.Type = "test_buyer"
)

type testOrder struct {
	BuyerID string
	Lines   []*testLine
	Buyer   *testRef
}

type testLine struct {
	ProductID string
	Product   *testProduct
}

type testProduct struct {
	ID         string
	CategoryID string
	Category   *testRef
}

type testRef struct{ ID string }

// testIdentity is a fixture app identity that loads includes as a flagged copy of itself.
type testIdentity struct {
	ActorID      string
	IncludeReads bool
}

func (i *testIdentity) ForIncludeReads() any {
	if i.IncludeReads {
		return i
	}
	cp := *i
	cp.IncludeReads = true
	return &cp
}

// identitySeen records the identity every loader ran under.
type identitySeen struct {
	mu   sync.Mutex
	seen map[object.Type][]*testIdentity
}

func (s *identitySeen) record(ctx context.Context, ot object.Type) {
	identity, _ := appctx.Identity[*testIdentity](ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[ot] = append(s.seen[ot], identity)
}

func (s *identitySeen) loader(ot object.Type, build func(id string) any) Loader {
	return func(ctx context.Context, ids []string) (map[string]any, *apierror.APIError) {
		s.record(ctx, ot)
		out := map[string]any{}
		for _, id := range ids {
			out[id] = build(id)
		}
		return out, nil
	}
}

func registerOrderGraph(seen *identitySeen) {
	Register(&Definition{
		ObjectType: otOrder,
		Load:       seen.loader(otOrder, func(string) any { return &testOrder{} }),
		Subs: []SubField{
			{
				Key: "buyer", Target: otBuyer, Cardinality: CardinalityOnePtr,
				ExtractIDs: func(_ context.Context, p any) []string { return []string{p.(*testOrder).BuyerID} },
				Populate: func(_ context.Context, p any, loaded map[string]any) {
					o := p.(*testOrder)
					if v, ok := loaded[o.BuyerID]; ok {
						o.Buyer = v.(*testRef)
					}
				},
			},
			{
				Key: "lines", Target: otLine, Cardinality: CardinalityList,
				ExtractRefs: func(_ context.Context, p any) []any {
					var refs []any
					for _, l := range p.(*testOrder).Lines {
						refs = append(refs, l)
					}
					return refs
				},
			},
		},
	})
	Register(&Definition{
		ObjectType: otLine,
		Load: func(context.Context, []string) (map[string]any, *apierror.APIError) {
			return nil, apierror.NewInvariantViolationError("lines are traversed, never loaded")
		},
		Subs: []SubField{{
			Key: "product", Target: otProduct, Cardinality: CardinalityOnePtr,
			ExtractIDs: func(_ context.Context, p any) []string { return []string{p.(*testLine).ProductID} },
			Populate: func(_ context.Context, p any, loaded map[string]any) {
				l := p.(*testLine)
				if v, ok := loaded[l.ProductID]; ok {
					l.Product = v.(*testProduct)
				}
			},
		}},
	})
	Register(&Definition{
		ObjectType: otProduct,
		Load:       seen.loader(otProduct, func(id string) any { return &testProduct{ID: id, CategoryID: "cat_" + id} }),
		Subs: []SubField{{
			Key: "category", Target: otCategory, Cardinality: CardinalityOnePtr,
			ExtractIDs: func(_ context.Context, p any) []string { return []string{p.(*testProduct).CategoryID} },
			Populate: func(_ context.Context, p any, loaded map[string]any) {
				pr := p.(*testProduct)
				if v, ok := loaded[pr.CategoryID]; ok {
					pr.Category = v.(*testRef)
				}
			},
		}},
	})
	Register(&Definition{ObjectType: otCategory, Load: seen.loader(otCategory, func(id string) any { return &testRef{ID: id} })})
	Register(&Definition{ObjectType: otBuyer, Load: seen.loader(otBuyer, func(id string) any { return &testRef{ID: id} })})
}

// Every include load, at every depth and through traversed children, runs as a flagged copy of the caller; the request's own identity is untouched.
func TestResolveIncludes_LoadsEveryIncludeAsAnIncludeRead(t *testing.T) {
	ResetForTest()
	seen := &identitySeen{seen: map[object.Type][]*testIdentity{}}
	registerOrderGraph(seen)

	caller := &testIdentity{ActorID: "usr_1"}
	ctx := appctx.WithIdentity(context.Background(), caller)

	order := &testOrder{BuyerID: "buyer_1", Lines: []*testLine{{ProductID: "prd_1"}, {ProductID: "prd_2"}}}
	tree := ParseIncludeTree([]string{"buyer", "lines.product.category"})
	if apiErr := ResolveIncludes(ctx, []any{order}, otOrder, tree); apiErr != nil {
		t.Fatalf("ResolveIncludes() error = %v", apiErr)
	}
	if order.Buyer == nil || order.Lines[1].Product == nil || order.Lines[1].Product.Category == nil {
		t.Fatalf("includes not stitched: %+v", order)
	}

	for _, ot := range []object.Type{otBuyer, otProduct, otCategory} {
		identities := seen.seen[ot]
		if len(identities) == 0 {
			t.Fatalf("%s was never loaded", ot)
		}
		for _, identity := range identities {
			if identity == nil || !identity.IncludeReads {
				t.Errorf("%s loaded without the include-reads flag: %+v", ot, identity)
				continue
			}
			if identity == caller || identity.ActorID != caller.ActorID {
				t.Errorf("%s loaded as a different caller: %+v", ot, identity)
			}
		}
	}
	if len(seen.seen[otOrder]) != 0 {
		t.Error("the root records were reloaded")
	}
	if caller.IncludeReads {
		t.Error("ResolveIncludes flagged the request's own identity")
	}
	if got, _ := appctx.Identity[*testIdentity](ctx); got != caller || got.IncludeReads {
		t.Error("the caller's context now carries a flagged identity")
	}
}

func TestWithIncludeReads(t *testing.T) {
	t.Parallel()

	if got := WithIncludeReads(context.Background()); got == nil {
		t.Fatal("a context without an identity must stay usable")
	} else if _, ok := appctx.AnyIdentity(got); ok {
		t.Fatal("WithIncludeReads invented an identity")
	}

	caller := &testIdentity{ActorID: "apke_1"}
	flaggedCtx := WithIncludeReads(appctx.WithIdentity(context.Background(), caller))
	flagged, _ := appctx.Identity[*testIdentity](flaggedCtx)
	if flagged == nil || !flagged.IncludeReads || flagged.ActorID != caller.ActorID {
		t.Fatalf("got %+v, want a flagged copy of the caller", flagged)
	}
	if caller.IncludeReads {
		t.Fatal("WithIncludeReads mutated the caller's identity")
	}
	if again, _ := appctx.Identity[*testIdentity](WithIncludeReads(flaggedCtx)); again != flagged {
		t.Error("an already flagged context should be returned as is")
	}
}
