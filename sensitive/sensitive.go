// Package sensitive keeps response fields from callers who may not see them.
//
// An app registers each class of restricted data (Register): the value of the `sensitive` struct tag that marks it, optionally a tag for maps whose matching keys belong to it, and a function saying whether the caller may see it. Redact then nulls every field of a hidden class reachable from a response, through nested resources, lists, maps, interfaces and resolved includes.
//
// The walk is planned once per type and class set and cached, so a response type that can never carry a restricted field costs one map lookup.
package sensitive

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// TagKey is the struct tag key that marks a field as restricted.
const TagKey = "sensitive"

// Class is one kind of restricted data.
type Class struct {
	// Tag (required) is the `sensitive` tag value that marks a field of this class, e.g. "cost".
	Tag string
	// KeyedTag (optional) marks a map[string]any whose keys name what they hold: the entries whose key satisfies MatchKey belong to this class, and nested maps are checked too.
	KeyedTag string
	// MatchKey (required with KeyedTag) reports whether a map key names data of this class.
	MatchKey func(key string) bool
	// Visible (required) reports whether the caller in ctx may see this class.
	Visible func(ctx context.Context) bool
}

// Set is a set of classes, as bits.
type Set uint64

type registered struct {
	Class
	bit Set
}

var (
	registryMu sync.RWMutex
	classes    []registered
	byTag      = map[string]Set{}
	byKeyedTag = map[string]int{}
)

// Register adds a class. Call it during startup, before any response is redacted: plans are cached per type and would not see a class added later. It panics on an invalid or duplicate class.
func Register(c Class) {
	switch {
	case c.Tag == "" || c.Tag == "true":
		panic(fmt.Sprintf("sensitive: class tag %q is empty or reserved for secrets", c.Tag))
	case c.Visible == nil:
		panic(fmt.Sprintf("sensitive: class %q needs Visible", c.Tag))
	case (c.KeyedTag == "") != (c.MatchKey == nil):
		panic(fmt.Sprintf("sensitive: class %q needs both KeyedTag and MatchKey or neither", c.Tag))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if len(classes) == 64 {
		panic("sensitive: at most 64 classes")
	}
	if _, dup := byTag[c.Tag]; dup {
		panic(fmt.Sprintf("sensitive: class %q is already registered", c.Tag))
	}
	if _, dup := byKeyedTag[c.KeyedTag]; c.KeyedTag != "" && dup {
		panic(fmt.Sprintf("sensitive: keyed tag %q is already registered", c.KeyedTag))
	}
	bit := Set(1) << len(classes)
	classes = append(classes, registered{Class: c, bit: bit})
	byTag[c.Tag] = bit
	if c.KeyedTag != "" {
		byKeyedTag[c.KeyedTag] = len(classes) - 1
	}
}

// Of returns the set holding the named classes; unknown tags are ignored.
func Of(tags ...string) Set {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var s Set
	for _, t := range tags {
		s |= byTag[t]
	}
	return s
}

// Hides reports whether the set includes the class with tag.
func (s Set) Hides(tag string) bool {
	return s&Of(tag) != 0
}

// Redactor is implemented by a type whose restricted data cannot be marked field by field, such as an audited change whose field name says what it holds. Redact calls it, through a pointer, on every such value it reaches while any class is hidden.
type Redactor interface {
	RedactSensitive(hidden Set)
}

// Hidden returns the classes the caller in ctx may not see.
func Hidden(ctx context.Context) Set {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var s Set
	for _, c := range classes {
		if !c.Visible(ctx) {
			s |= c.bit
		}
	}
	return s
}

// Visible reports whether the caller in ctx may see the class with tag.
func Visible(ctx context.Context, tag string) bool {
	return !Hidden(ctx).Hides(tag)
}

// Redact nulls every field reachable from v that the caller in ctx may not see, and returns the value to serialize. Pointers, slices and maps are redacted in place; a bare struct value comes back as a redacted copy.
func Redact(ctx context.Context, v any) any {
	return Strip(v, Hidden(ctx))
}

// Strip nulls every field of the given classes reachable from v, whoever the caller. See Redact for what is changed in place.
func Strip(v any, hidden Set) any {
	if v == nil || hidden == 0 {
		return v
	}
	rv := reflect.ValueOf(v)
	p := planFor(rv.Type(), hidden)
	if p == nil {
		return v
	}
	if rv.Kind() == reflect.Struct || rv.Kind() == reflect.Array {
		cp := reflect.New(rv.Type()).Elem()
		cp.Set(rv)
		p.apply(cp)
		return cp.Interface()
	}
	p.apply(rv)
	return v
}

// HasFields reports whether a value of type t can carry a field of the class with tag.
func HasFields(t reflect.Type, tag string) bool {
	set := Of(tag)
	return set != 0 && planFor(t, set) != nil
}

// classOf is the class a field's tag places it in, or 0.
func classOf(sf reflect.StructField) Set {
	return Of(sf.Tag.Get(TagKey))
}

// keyedMatcher returns the key matcher of a keyed-map field whose class is in hidden.
func keyedMatcher(sf reflect.StructField, hidden Set) (func(string) bool, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	i, ok := byKeyedTag[sf.Tag.Get(TagKey)]
	if !ok || classes[i].bit&hidden == 0 {
		return nil, false
	}
	return classes[i].MatchKey, true
}

// plan is what to visit in a value of one type: the hidden fields to clear, and the children that can lead to more. A nil plan means the type can never carry a hidden field, so the walk skips it without reflecting over it.
type plan struct {
	hidden   Set
	kind     reflect.Kind
	clear    []int
	keyed    []keyedField
	fields   []fieldPlan
	elem     *plan
	dynamic  bool
	redactor bool
}

type keyedField struct {
	index int
	match func(string) bool
}

type fieldPlan struct {
	index int
	plan  *plan
}

func (p *plan) apply(v reflect.Value) {
	switch p.kind {
	case reflect.Pointer:
		if !v.IsNil() {
			p.elem.apply(v.Elem())
		}
	case reflect.Interface:
		if !v.IsNil() {
			applyDynamic(v, p.hidden)
		}
	case reflect.Struct:
		for _, i := range p.clear {
			v.Field(i).SetZero()
		}
		for _, k := range p.keyed {
			if m, ok := v.Field(k.index).Interface().(map[string]any); ok {
				clearKeys(m, k.match)
			}
		}
		for _, f := range p.fields {
			f.plan.apply(v.Field(f.index))
		}
		if p.redactor && v.CanAddr() {
			v.Addr().Interface().(Redactor).RedactSensitive(p.hidden)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			p.elem.apply(v.Index(i))
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			val := iter.Value()
			switch val.Kind() {
			case reflect.Pointer, reflect.Slice, reflect.Map:
				p.elem.apply(val)
			default:
				cp := reflect.New(val.Type()).Elem()
				cp.Set(val)
				p.elem.apply(cp)
				v.SetMapIndex(iter.Key(), cp)
			}
		}
	}
}

// clearKeys nulls every entry of m, and of the maps nested in it, whose key match accepts.
func clearKeys(m map[string]any, match func(string) bool) {
	for key, value := range m {
		if match(key) {
			m[key] = nil
			continue
		}
		switch nested := value.(type) {
		case map[string]any:
			clearKeys(nested, match)
		case []any:
			for _, elem := range nested {
				if inner, ok := elem.(map[string]any); ok {
					clearKeys(inner, match)
				}
			}
		}
	}
}

// applyDynamic redacts the concrete value held by an interface, writing a copy back when that value is not addressable.
func applyDynamic(v reflect.Value, hidden Set) {
	inner := v.Elem()
	p := planFor(inner.Type(), hidden)
	if p == nil {
		return
	}
	switch inner.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map:
		p.apply(inner)
	default:
		if !v.CanSet() {
			return
		}
		cp := reflect.New(inner.Type()).Elem()
		cp.Set(inner)
		p.apply(cp)
		v.Set(cp)
	}
}

type planKey struct {
	t      reflect.Type
	hidden Set
}

var (
	plans   sync.Map // planKey -> *plan (nil when the type carries nothing hidden)
	buildMu sync.Mutex
)

func planFor(t reflect.Type, hidden Set) *plan {
	key := planKey{t: t, hidden: hidden}
	if p, ok := plans.Load(key); ok {
		return p.(*plan)
	}
	buildMu.Lock()
	defer buildMu.Unlock()
	if p, ok := plans.Load(key); ok {
		return p.(*plan)
	}
	b := &builder{hidden: hidden, nodes: map[reflect.Type]*node{}}
	root := b.node(t)
	b.finish()
	return root.plan
}

// node is a type under construction. Types refer to each other in cycles (an account's child accounts are accounts), so whether a type leads to a hidden field is settled for the whole graph at once in finish, not while descending.
type node struct {
	t        reflect.Type
	kind     reflect.Kind
	clear    []int
	keyed    []keyedField
	fields   []fieldNode
	elem     *node
	dynamic  bool
	redactor bool
	live     bool
	plan     *plan
	cached   bool
}

type fieldNode struct {
	index int
	node  *node
}

type builder struct {
	hidden Set
	nodes  map[reflect.Type]*node
}

var redactorType = reflect.TypeFor[Redactor]()

func (b *builder) node(t reflect.Type) *node {
	if n, ok := b.nodes[t]; ok {
		return n
	}
	n := &node{t: t, kind: t.Kind()}
	b.nodes[t] = n
	if p, ok := plans.Load(planKey{t: t, hidden: b.hidden}); ok {
		n.cached = true
		n.plan = p.(*plan)
		n.live = n.plan != nil
		return n
	}
	switch n.kind {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		n.elem = b.node(t.Elem())
	case reflect.Interface:
		n.dynamic = true
	case reflect.Struct:
		n.redactor = reflect.PointerTo(t).Implements(redactorType)
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() || sf.Tag.Get("json") == "-" {
				continue
			}
			if classOf(sf)&b.hidden != 0 {
				n.clear = append(n.clear, i)
				continue
			}
			if match, ok := keyedMatcher(sf, b.hidden); ok {
				n.keyed = append(n.keyed, keyedField{index: i, match: match})
				continue
			}
			if canNest(sf.Type) {
				n.fields = append(n.fields, fieldNode{index: i, node: b.node(sf.Type)})
			}
		}
	}
	return n
}

// canNest reports whether a value of type t can hold another value, so a hidden field could sit below it.
func canNest(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface, reflect.Struct:
		return true
	default:
		return false
	}
}

func (b *builder) finish() {
	for _, n := range b.nodes {
		if !n.cached && (len(n.clear) > 0 || len(n.keyed) > 0 || n.dynamic || n.redactor) {
			n.live = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range b.nodes {
			if n.live || n.cached {
				continue
			}
			if (n.elem != nil && n.elem.live) || anyLive(n.fields) {
				n.live = true
				changed = true
			}
		}
	}
	for _, n := range b.nodes {
		if !n.cached && n.live {
			n.plan = &plan{hidden: b.hidden, kind: n.kind, clear: n.clear, keyed: n.keyed, dynamic: n.dynamic, redactor: n.redactor}
		}
	}
	for _, n := range b.nodes {
		if n.cached || !n.live {
			continue
		}
		if n.elem != nil {
			n.plan.elem = n.elem.plan
		}
		for _, f := range n.fields {
			if f.node.live {
				n.plan.fields = append(n.plan.fields, fieldPlan{index: f.index, plan: f.node.plan})
			}
		}
	}
	for t, n := range b.nodes {
		if !n.cached {
			plans.Store(planKey{t: t, hidden: b.hidden}, n.plan)
		}
	}
}

func anyLive(fields []fieldNode) bool {
	for _, f := range fields {
		if f.node.live {
			return true
		}
	}
	return false
}
