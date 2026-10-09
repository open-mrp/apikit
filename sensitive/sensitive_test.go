package sensitive

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-mrp/apikit/appctx"
)

// caller is a fixture identity: which classes it may see.
type caller struct {
	seesCost, seesInternal bool
}

func callerFrom(ctx context.Context) caller {
	c, _ := appctx.Identity[caller](ctx)
	return c
}

func isCostName(key string) bool {
	return strings.Contains(key, "cost") || strings.Contains(key, "margin")
}

// Classes are registered process-wide before any test runs, the same way an app registers them at startup.
func TestMain(m *testing.M) {
	Register(Class{
		Tag:      "cost",
		KeyedTag: "cost_keys",
		MatchKey: isCostName,
		Visible:  func(ctx context.Context) bool { return callerFrom(ctx).seesCost },
	})
	Register(Class{
		Tag:     "internal",
		Visible: func(ctx context.Context) bool { return callerFrom(ctx).seesInternal },
	})
	os.Exit(m.Run())
}

func ctxFor(c caller) context.Context {
	return appctx.WithIdentity(context.Background(), c)
}

type rate struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type item struct {
	ID        string `json:"id"`
	UnitValue *rate  `json:"unit_value"`
	UnitCost  *rate  `json:"unit_cost" sensitive:"cost"`
}

type product struct {
	ID   string `json:"id"`
	Item *item  `json:"item"`
}

type line struct {
	ID       string   `json:"id"`
	Product  *product `json:"product"`
	UnitCost *rate    `json:"unit_cost" sensitive:"cost"`
}

type list[T any] struct {
	Data []T `json:"data"`
}

type order struct {
	ID          string         `json:"id"`
	Lines       *list[line]    `json:"lines"`
	OtherOrders []string       `json:"other_orders" sensitive:"internal"`
	Settings    map[string]any `json:"settings" sensitive:"cost_keys"`
}

type account struct {
	ID       string          `json:"id"`
	Children []*account      `json:"children"`
	Margin   *string         `json:"margin" sensitive:"cost"`
	ByCode   map[string]item `json:"by_code"`
	Extra    any             `json:"extra"`
}

type plain struct {
	ID    string    `json:"id"`
	Inner *struct{} `json:"inner"`
	Rates []rate    `json:"rates"`
}

func ptr[T any](v T) *T { return &v }

func sampleOrder() *order {
	shared := &item{ID: "itm_1", UnitValue: &rate{ID: "rt_v", Value: "5"}, UnitCost: &rate{ID: "rt_c", Value: "2"}}
	return &order{
		ID: "so_1",
		Lines: &list[line]{Data: []line{
			{ID: "sol_1", Product: &product{ID: "pd_1", Item: shared}, UnitCost: &rate{ID: "rt_l1", Value: "3"}},
			{ID: "sol_2"},
		}},
		OtherOrders: []string{"so_9"},
		Settings:    map[string]any{"unit_cost": 3, "hours": 7, "nested": map[string]any{"gross_margin": 0.3, "shift": 2}},
	}
}

func TestRedact_ByCaller(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                 string
		caller               caller
		wantCost, wantIntern bool
	}{
		{"sees everything", caller{seesCost: true, seesInternal: true}, true, true},
		{"internal but no costs", caller{seesInternal: true}, false, true},
		{"sees nothing", caller{}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := Redact(ctxFor(tc.caller), sampleOrder()).(*order)
			assert.Equal(t, tc.wantCost, o.Lines.Data[0].UnitCost != nil, "line cost")
			assert.Equal(t, tc.wantCost, o.Lines.Data[0].Product.Item.UnitCost != nil, "nested item cost")
			assert.Equal(t, tc.wantCost, o.Settings["unit_cost"] != nil, "cost-named setting")
			assert.Equal(t, tc.wantIntern, o.OtherOrders != nil, "internal field")
			assert.Equal(t, "5", o.Lines.Data[0].Product.Item.UnitValue.Value, "unrestricted field kept")
			assert.Equal(t, 7, o.Settings["hours"], "other setting kept")
		})
	}
}

func TestRedact_NoIdentityHidesEverything(t *testing.T) {
	t.Parallel()
	o := Redact(context.Background(), sampleOrder()).(*order)
	assert.Nil(t, o.Lines.Data[0].UnitCost)
	assert.Nil(t, o.OtherOrders)
}

func TestStrip_SerializesNullAndClearsNestedKeys(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Strip(sampleOrder(), Of("cost")))
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"so_1","lines":{"data":[
		{"id":"sol_1","product":{"id":"pd_1","item":{"id":"itm_1","unit_value":{"id":"rt_v","value":"5"},"unit_cost":null}},"unit_cost":null},
		{"id":"sol_2","product":null,"unit_cost":null}]},
		"other_orders":["so_9"],
		"settings":{"unit_cost":null,"hours":7,"nested":{"gross_margin":null,"shift":2}}}`, string(body))
}

func TestStrip_RecursiveTypesMapsAndInterfaces(t *testing.T) {
	t.Parallel()

	root := &account{
		ID:     "ac_1",
		Margin: ptr("0.4"),
		Children: []*account{
			{ID: "ac_2", Margin: ptr("0.3"), Children: []*account{{ID: "ac_3", Margin: ptr("0.2")}}},
			nil,
		},
		ByCode: map[string]item{"a": {ID: "itm_a", UnitCost: &rate{Value: "1"}}},
		Extra:  &item{ID: "itm_x", UnitCost: &rate{Value: "9"}},
	}
	Strip(root, Of("cost"))

	assert.Nil(t, root.Margin)
	assert.Nil(t, root.Children[0].Margin)
	assert.Nil(t, root.Children[0].Children[0].Margin)
	assert.Nil(t, root.ByCode["a"].UnitCost)
	assert.Equal(t, "itm_a", root.ByCode["a"].ID)
	assert.Nil(t, root.Extra.(*item).UnitCost)
}

func TestStrip_ValueRootAndInterfaceHoldingAValue(t *testing.T) {
	t.Parallel()

	got := Strip(item{ID: "itm_v", UnitCost: &rate{Value: "1"}}, Of("cost")).(item)
	assert.Nil(t, got.UnitCost)

	holder := &account{Extra: item{ID: "itm_i", UnitCost: &rate{Value: "1"}}}
	Strip(holder, Of("cost"))
	assert.Nil(t, holder.Extra.(item).UnitCost)

	inMap := map[string]any{"x": item{UnitCost: &rate{Value: "1"}}, "y": &item{UnitCost: &rate{Value: "2"}}}
	Strip(inMap, Of("cost"))
	assert.Nil(t, inMap["x"].(item).UnitCost)
	assert.Nil(t, inMap["y"].(*item).UnitCost)
}

func TestStrip_NilSafety(t *testing.T) {
	t.Parallel()

	assert.Nil(t, Strip(nil, Of("cost")))
	assert.NotPanics(t, func() {
		Strip((*order)(nil), Of("cost"))
		Strip(&order{}, Of("cost", "internal"))
		Strip(&order{Lines: &list[line]{}}, Of("cost"))
		Strip([]*item{nil, {}}, Of("cost"))
		Strip(&account{Extra: (*item)(nil)}, Of("cost"))
		Redact(context.Background(), (*item)(nil))
	})
}

func TestHasFields(t *testing.T) {
	t.Parallel()

	assert.True(t, HasFields(reflect.TypeFor[*order](), "cost"))
	assert.True(t, HasFields(reflect.TypeFor[*order](), "internal"))
	assert.True(t, HasFields(reflect.TypeFor[[]line](), "cost"))
	assert.False(t, HasFields(reflect.TypeFor[[]line](), "internal"))
	assert.True(t, HasFields(reflect.TypeFor[*account](), "cost"), "an any field may hold restricted data")
	assert.False(t, HasFields(reflect.TypeFor[*plain](), "cost"))
	assert.False(t, HasFields(reflect.TypeFor[*order](), "unregistered"))
}

type change struct {
	Field    string          `json:"field"`
	NewValue json.RawMessage `json:"new_value"`
}

func (c *change) RedactSensitive(hidden Set) {
	if hidden.Hides("cost") && isCostName(c.Field) {
		c.NewValue = nil
	}
}

type event struct {
	ID      string            `json:"id"`
	Changes *list[change]     `json:"changes"`
	Pinned  map[string]change `json:"pinned"`
}

func TestStrip_CallsRedactorsWhereverTheyAppear(t *testing.T) {
	t.Parallel()

	newEvent := func() *event {
		return &event{
			Changes: &list[change]{Data: []change{
				{Field: "unit_cost_value", NewValue: json.RawMessage(`"4.10"`)},
				{Field: "name", NewValue: json.RawMessage(`"Sock"`)},
			}},
			Pinned: map[string]change{"x": {Field: "gross_margin", NewValue: json.RawMessage(`"0.3"`)}},
		}
	}

	ev := newEvent()
	Strip(ev, Of("cost"))
	assert.Nil(t, ev.Changes.Data[0].NewValue)
	assert.JSONEq(t, `"Sock"`, string(ev.Changes.Data[1].NewValue))
	assert.Nil(t, ev.Pinned["x"].NewValue)

	ev = newEvent()
	Strip(ev, Of("internal"))
	assert.NotNil(t, ev.Changes.Data[0].NewValue, "a redactor decides from the hidden set what to clear")
}

func TestRegister_RejectsBadClasses(t *testing.T) {
	t.Parallel()
	visible := func(context.Context) bool { return true }
	for name, c := range map[string]Class{
		"duplicate":       {Tag: "cost", Visible: visible},
		"secret tag":      {Tag: "true", Visible: visible},
		"empty tag":       {Visible: visible},
		"no visible":      {Tag: "x1"},
		"keyed tag alone": {Tag: "x2", KeyedTag: "x2_keys", Visible: visible},
		"duplicate keyed": {Tag: "x3", KeyedTag: "cost_keys", MatchKey: isCostName, Visible: visible},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Register did not panic", name)
				}
			}()
			Register(c)
		}()
	}
}

func BenchmarkRedact_List(b *testing.B) {
	ctx := ctxFor(caller{seesInternal: true})
	items := make([]item, 100)
	for i := range items {
		items[i] = item{ID: "itm", UnitValue: &rate{Value: "1"}, UnitCost: &rate{Value: "1"}}
	}
	resp := &list[item]{Data: items}
	b.ResetTimer()
	for range b.N {
		Redact(ctx, resp)
	}
}
