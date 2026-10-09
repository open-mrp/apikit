package endpoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/include"
	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/sensitive"
)

// hookIdentity is a fixture app identity: its permissions, and whether it is the include-reads copy.
type hookIdentity struct {
	Permissions  []string
	IncludeReads bool
}

func (i *hookIdentity) ForIncludeReads() any {
	if i.IncludeReads {
		return i
	}
	cp := *i
	cp.IncludeReads = true
	return &cp
}

// requirePermission is a fixture access policy, in whatever shape the app's Authorizer understands.
type requirePermission string

// The app's hooks are process-wide and set before any test runs, as an app sets them at startup. Endpoints without an Auth policy, which every other test uses, are unaffected.
func init() {
	SetAuthorizer(func(r *http.Request, policy any) *apierror.APIError {
		need, ok := policy.(requirePermission)
		if !ok {
			return nil
		}
		identity, ok := appctx.Identity[*hookIdentity](r.Context())
		if !ok || identity == nil {
			return apierror.NewAuthenticationError("Sign in to use this endpoint.")
		}
		if !slices.Contains(identity.Permissions, string(need)) {
			return apierror.NewAuthorizationError("You need " + string(need) + ".")
		}
		return nil
	})
	sensitive.Register(sensitive.Class{
		Tag: "hook_cost",
		Visible: func(ctx context.Context) bool {
			identity, ok := appctx.Identity[*hookIdentity](ctx)
			return ok && identity != nil && slices.Contains(identity.Permissions, "costs:read")
		},
	})
}

func TestExecute_Authorizer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		identity *hookIdentity
		want     int
	}{
		{"no caller", nil, http.StatusUnauthorized},
		{"caller without the permission", &hookIdentity{Permissions: []string{"items:read"}}, http.StatusForbidden},
		{"caller with the permission", &hookIdentity{Permissions: []string{"orders:read"}}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ran := false
			ep := &APIEndpoint[*stubRequest, *stubResponse]{
				Method:            http.MethodGet,
				Route:             "/v1/orders",
				SuccessStatusCode: http.StatusOK,
				Auth:              requirePermission("orders:read"),
				ServiceHandler: func(any) ServiceHandler[*stubRequest, *stubResponse] {
					return func(context.Context, *stubRequest) (*stubResponse, *apierror.APIError) {
						ran = true
						return &stubResponse{ID: "or_1"}, nil
					}
				},
			}
			bindHandler(ep)

			r := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
			if tc.identity != nil {
				r = r.WithContext(appctx.WithIdentity(r.Context(), tc.identity))
			}
			w := httptest.NewRecorder()
			ep.Execute(w, r)

			if w.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if ran != (tc.want == http.StatusOK) {
				t.Errorf("handler ran = %v", ran)
			}
		})
	}
}

type pricedResponse struct {
	ID       string  `json:"id"`
	UnitCost *string `json:"unit_cost" sensitive:"hook_cost"`
}

func TestExecute_RedactsSensitiveFieldsForTheCaller(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		identity *hookIdentity
		wantCost bool
	}{
		{"caller who may see costs", &hookIdentity{Permissions: []string{"costs:read"}}, true},
		{"caller who may not", &hookIdentity{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ep := &APIEndpoint[*stubRequest, *pricedResponse]{
				Method:            http.MethodGet,
				Route:             "/v1/items/{id}",
				SuccessStatusCode: http.StatusOK,
				ServiceHandler: func(any) ServiceHandler[*stubRequest, *pricedResponse] {
					return func(context.Context, *stubRequest) (*pricedResponse, *apierror.APIError) {
						cost := "4.10"
						return &pricedResponse{ID: "it_1", UnitCost: &cost}, nil
					}
				},
			}
			bindHandler(ep)

			r := httptest.NewRequest(http.MethodGet, "/v1/items/it_1", nil)
			r = r.WithContext(appctx.WithIdentity(r.Context(), tc.identity))
			w := httptest.NewRecorder()
			ep.Execute(w, r)

			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := body["unit_cost"] != nil; got != tc.wantCost {
				t.Errorf("unit_cost = %v, want present=%v", body["unit_cost"], tc.wantCost)
			}
		})
	}
}

const (
	otHookOrder object.Type = "hook_order"
	otHookBuyer object.Type = "hook_buyer"
)

type hookOrder struct {
	ID      string     `json:"id"`
	BuyerID string     `json:"-"`
	Buyer   *hookBuyer `json:"buyer"`
}

type hookBuyer struct {
	ID string `json:"id"`
}

// The handler runs as the caller; includes load as the caller's include-reads identity.
func TestExecute_IncludesLoadAsIncludeReads(t *testing.T) {
	// Not parallel: it registers a resource definition in the process-wide include registry.
	var handledAs, buyerLoadedAs *hookIdentity
	include.Register(&include.Definition{
		ObjectType: otHookOrder,
		Load: func(context.Context, []string) (map[string]any, *apierror.APIError) {
			return nil, apierror.NewInvariantViolationError("roots are never loaded")
		},
		Subs: []include.SubField{{
			Key: "buyer", Target: otHookBuyer, Cardinality: include.CardinalityOnePtr,
			ExtractIDs: func(_ context.Context, p any) []string { return []string{p.(*hookOrder).BuyerID} },
			Populate: func(_ context.Context, p any, loaded map[string]any) {
				o := p.(*hookOrder)
				if v, ok := loaded[o.BuyerID]; ok {
					o.Buyer = v.(*hookBuyer)
				}
			},
		}},
	})
	include.Register(&include.Definition{
		ObjectType: otHookBuyer,
		Load: func(ctx context.Context, ids []string) (map[string]any, *apierror.APIError) {
			buyerLoadedAs, _ = appctx.Identity[*hookIdentity](ctx)
			out := map[string]any{}
			for _, id := range ids {
				out[id] = &hookBuyer{ID: id}
			}
			return out, nil
		},
	})

	ep := &APIEndpoint[*stubRequest, *hookOrder]{
		Method:            http.MethodGet,
		Route:             "/v1/orders/{id}",
		SuccessStatusCode: http.StatusOK,
		ObjectType:        otHookOrder,
		IncludeConfig:     &IncludeConfig{Fields: []IncludeField{{Key: "buyer", ObjectType: otHookBuyer, JSONPaths: []string{"buyer"}}}},
		ServiceHandler: func(any) ServiceHandler[*stubRequest, *hookOrder] {
			return func(ctx context.Context, _ *stubRequest) (*hookOrder, *apierror.APIError) {
				handledAs, _ = appctx.Identity[*hookIdentity](ctx)
				return &hookOrder{ID: "or_1", BuyerID: "cu_1"}, nil
			}
		},
	}
	bindHandler(ep)

	caller := &hookIdentity{Permissions: []string{"orders:read"}}
	r := httptest.NewRequest(http.MethodGet, "/v1/orders/or_1?include[]=buyer", nil)
	r = r.WithContext(appctx.WithIdentity(r.Context(), caller))
	w := httptest.NewRecorder()
	ep.Execute(w, r)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"buyer":{"id":"cu_1"}`) {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if handledAs != caller {
		t.Errorf("handler ran as %+v, want the caller", handledAs)
	}
	if buyerLoadedAs == nil || !buyerLoadedAs.IncludeReads {
		t.Errorf("include loaded as %+v, want the include-reads copy", buyerLoadedAs)
	}
}
