package vascula

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// Lazy is a host value computed the first time a render uses it. A host puts
// a *Lazy anywhere in Data, Settings or Globals (a map value, a slice element,
// a root) for a value that is costly to compute and that most templates never
// read: the template sees the computed value wherever it reads one, and a
// render that never reaches it never computes it.
//
// Every use of a value honours it the same way:
//   - reading a variable, a property, an index, first/last or a loop item
//     computes that value;
//   - whatever consumes a whole value (output, json and every other filter
//     except size, comparisons and contains, case, host tag arguments, form
//     attributes) receives it with every Lazy inside it computed, at any depth.
//
// A load error fails the render. The load runs at most once; later uses, also
// in other renders, return its first result, so a host creates its Lazy values
// per request. A load may return another *Lazy or data holding them. It must
// not use the same Lazy itself.
type Lazy struct {
	once  sync.Once
	load  func() (interface{}, error)
	value interface{}
	err   error
}

// NewLazy returns a Lazy computed by load.
func NewLazy(load func() (interface{}, error)) *Lazy {
	return &Lazy{load: load}
}

// Value computes the value on the first call and returns that result on every
// call. A load returning another Lazy is followed, so Value never returns a
// *Lazy; the value may hold Lazy values inside it, which Resolve computes.
// Host code reading a path through data computes each step with Value, so the
// rest of the data stays uncomputed.
func (l *Lazy) Value() (interface{}, error) {
	var v interface{} = l
	for depth := 0; ; depth++ {
		next, ok := v.(*Lazy)
		if !ok {
			return v, nil
		}
		if next == nil {
			return nil, nil
		}
		if depth >= maxValueDepth {
			return nil, fmt.Errorf("lazy value nesting too deep")
		}
		next.once.Do(func() {
			if next.load != nil {
				next.value, next.err = next.load()
			}
		})
		if next.err != nil {
			return nil, next.err
		}
		v = next.value
	}
}

// MarshalJSON encodes the computed value, so host data holding Lazy values
// encodes completely with encoding/json.
func (l *Lazy) MarshalJSON() ([]byte, error) {
	v, err := Resolve(l)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// Resolve returns v with every Lazy computed, at any depth: what a template
// consuming v as a whole sees. A host tag reading a variable (TagCall.Lookup)
// or a host observing output receives values as the host supplied them and
// uses Resolve to read them completely. v itself is not modified; containers
// holding a Lazy are copied.
func Resolve(v interface{}) (interface{}, error) {
	return (&lazyState{}).deep(v, 0)
}

// lazyState is one render's view of the Lazy values in its data: whole values
// already resolved, by identity, so a collection reaching several consumers
// (a filter chain, a loop printing it) is walked once. Like the value budget,
// this relies on data not changing during a render.
type lazyState struct {
	resolved map[valueIdentity]resolvedValue
	budget   *budget // nil: unbounded (Resolve)
}

// shallow computes v if it is a Lazy: the value a read produces.
func shallow(v interface{}) (interface{}, error) {
	if l, ok := v.(*Lazy); ok {
		return l.Value()
	}
	return v, nil
}

var (
	lazyType = reflect.TypeOf((*Lazy)(nil))
	anyType  = reflect.TypeOf((*interface{})(nil)).Elem()
)

// deep returns v with every Lazy inside it computed. Values holding no Lazy
// are returned as they are; a container holding one is copied with the
// computed values, keeping its type where the computed values fit it.
func (s *lazyState) deep(v interface{}, depth int) (interface{}, error) {
	out, _, err := s.walk(v, depth)
	return out, err
}

// walk is deep, also reporting whether the result differs from v.
func (s *lazyState) walk(v interface{}, depth int) (interface{}, bool, error) {
	_, isLazy := v.(*Lazy)
	v, err := shallow(v)
	if err != nil {
		return nil, false, err
	}
	switch v.(type) {
	case nil, bool, string, float64, int, int64, safeString, jsonString, SafeHTML, HTMLAttr, emptyMarker, *loopInfo:
		return v, isLazy, nil
	}
	switch x := v.(type) {
	case map[string]interface{}:
		out, changed, err := s.walkJSON(x, nil, depth)
		return out, isLazy || changed, err
	case []interface{}:
		out, changed, err := s.walkJSON(nil, x, depth)
		return out, isLazy || changed, err
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
	default:
		// Other scalars and host objects: the evaluator reads no fields of them.
		return v, isLazy, nil
	}
	if rv.Len() == 0 || !mayHoldLazy(rv.Type().Elem()) {
		return v, isLazy, nil
	}
	if depth > maxValueDepth {
		return nil, false, fmt.Errorf("render budget exceeded: value nesting too deep")
	}
	var key valueIdentity
	keyed := false
	if rv.Kind() != reflect.Array {
		key, keyed = identityOf(rv)
	}
	if keyed {
		if r, ok := s.resolved[key]; ok {
			return r.value, isLazy || r.changed, nil
		}
	}
	out, changed, err := s.walkContainer(rv, depth)
	if err != nil {
		return nil, false, err
	}
	if keyed {
		if s.resolved == nil {
			s.resolved = map[valueIdentity]resolvedValue{}
		}
		s.resolved[key] = resolvedValue{value: out, changed: changed}
	}
	return out, isLazy || changed, nil
}

// walkJSON is walk for the JSON shapes hosts pass almost exclusively, without
// reflection: a map (m) or a slice (l).
func (s *lazyState) walkJSON(m map[string]interface{}, l []interface{}, depth int) (interface{}, bool, error) {
	var v interface{} = l
	n := len(l)
	if m != nil {
		v, n = m, len(m)
	}
	if n == 0 {
		return v, false, nil
	}
	if depth > maxValueDepth {
		return nil, false, fmt.Errorf("render budget exceeded: value nesting too deep")
	}
	key, keyed := identityOf(reflect.ValueOf(v))
	if keyed {
		if r, ok := s.resolved[key]; ok {
			return r.value, r.changed, nil
		}
	}
	step := func() error {
		if s.budget != nil {
			return s.budget.step()
		}
		return nil
	}
	var out interface{} = v
	changed := false
	if m != nil {
		var copied map[string]interface{}
		for k, item := range m {
			if err := step(); err != nil {
				return nil, false, err
			}
			computed, itemChanged, err := s.walk(item, depth+1)
			if err != nil {
				return nil, false, err
			}
			if itemChanged {
				if copied == nil {
					copied = make(map[string]interface{}, len(m))
					for k2, v2 := range m {
						copied[k2] = v2
					}
				}
				copied[k] = computed
			}
		}
		if copied != nil {
			out, changed = copied, true
		}
	} else {
		var copied []interface{}
		for i, item := range l {
			if err := step(); err != nil {
				return nil, false, err
			}
			computed, itemChanged, err := s.walk(item, depth+1)
			if err != nil {
				return nil, false, err
			}
			if itemChanged && copied == nil {
				copied = append(make([]interface{}, 0, len(l)), l...)
			}
			if copied != nil {
				copied[i] = computed
			}
		}
		if copied != nil {
			out, changed = copied, true
		}
	}
	if keyed {
		if s.resolved == nil {
			s.resolved = map[valueIdentity]resolvedValue{}
		}
		s.resolved[key] = resolvedValue{value: out, changed: changed}
	}
	return out, changed, nil
}

type resolvedValue struct {
	value   interface{}
	changed bool
}

func (s *lazyState) walkContainer(rv reflect.Value, depth int) (interface{}, bool, error) {
	elemType := rv.Type().Elem()
	var keys []reflect.Value
	n := rv.Len()
	if rv.Kind() == reflect.Map {
		keys = rv.MapKeys()
	}
	items := make([]reflect.Value, n)
	changed := false
	for i := 0; i < n; i++ {
		if s.budget != nil {
			if err := s.budget.step(); err != nil {
				return nil, false, err
			}
		}
		var item reflect.Value
		if keys != nil {
			item = rv.MapIndex(keys[i])
		} else {
			item = rv.Index(i)
		}
		out, itemChanged, err := s.walk(item.Interface(), depth+1)
		if err != nil {
			return nil, false, err
		}
		if itemChanged {
			changed = true
			item = valueOf(out, elemType)
		}
		items[i] = item
	}
	if !changed {
		return rv.Interface(), false, nil
	}
	fits := true
	for _, item := range items {
		if !item.Type().AssignableTo(elemType) {
			fits = false
			break
		}
	}
	if !fits {
		// The copy holds interface{} elements: a computed nil is untyped.
		for i, item := range items {
			if item.Kind() == reflect.Ptr && item.IsNil() && item.Type() == lazyType {
				items[i] = reflect.Zero(anyType)
			}
		}
	}
	if rv.Kind() == reflect.Map {
		target := rv.Type()
		if !fits {
			target = reflect.MapOf(rv.Type().Key(), anyType)
		}
		out := reflect.MakeMapWithSize(target, n)
		for i, k := range keys {
			out.SetMapIndex(k, items[i])
		}
		return out.Interface(), true, nil
	}
	var out reflect.Value
	switch {
	case !fits:
		out = reflect.MakeSlice(reflect.TypeOf([]interface{}{}), n, n)
	case rv.Kind() == reflect.Array:
		out = reflect.New(rv.Type()).Elem()
	default:
		out = reflect.MakeSlice(rv.Type(), n, n)
	}
	for i, item := range items {
		out.Index(i).Set(item)
	}
	return out.Interface(), true, nil
}

// mayHoldLazy reports whether a container with elements of type t can hold a
// Lazy at some depth.
func mayHoldLazy(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Interface:
		return true
	case reflect.Map, reflect.Slice, reflect.Array:
		return mayHoldLazy(t.Elem())
	}
	return t == lazyType
}

// valueOf is v as an element of a container of elemType: an untyped nil
// becomes elemType's zero value.
func valueOf(v interface{}, elemType reflect.Type) reflect.Value {
	if v == nil {
		return reflect.Zero(elemType)
	}
	return reflect.ValueOf(v)
}

// entries is v as a filter reading only its entries (and the field key of
// each, when key is set) sees it: a sequence's entries computed, and the key of
// each entry that is an object. Anything else is computed whole.
func (s *lazyState) entries(v interface{}, key string) (interface{}, error) {
	items, ok := v.([]interface{})
	if !ok {
		rv := reflect.ValueOf(v)
		if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
			return s.deep(v, 0)
		}
		items = iterate(v)
	}
	// Bounded by the collection's length, which the filter's budget charges.
	var out []interface{}
	for i, item := range items {
		computed, changed := item, false
		if l, ok := item.(*Lazy); ok {
			v, err := l.Value()
			if err != nil {
				return nil, err
			}
			computed, changed = v, true
		}
		if key != "" {
			withKey, keyChanged, err := s.withField(computed, key)
			if err != nil {
				return nil, err
			}
			computed, changed = withKey, changed || keyChanged
		}
		if out == nil && changed {
			out = make([]interface{}, len(items))
			copy(out, items[:i])
		}
		if out != nil {
			out[i] = computed
		}
	}
	if out == nil {
		return v, nil
	}
	return out, nil
}

// withField is entry with its field key computed (a copy when it was a Lazy),
// and whether it differs from entry.
func (s *lazyState) withField(entry interface{}, key string) (interface{}, bool, error) {
	switch m := entry.(type) {
	case map[string]interface{}:
		field, ok := m[key].(*Lazy)
		if !ok {
			return entry, false, nil
		}
		computed, err := field.Value()
		if err != nil {
			return nil, false, err
		}
		copied := make(map[string]interface{}, len(m))
		for k, item := range m {
			copied[k] = item
		}
		copied[key] = computed
		return copied, true, nil
	case nil, bool, string, float64, int, int64, safeString, jsonString, SafeHTML, HTMLAttr:
		return entry, false, nil
	}
	// Other host maps: computed whole.
	return s.walk(entry, 0)
}

// markComputed records a map or slice built from computed values only (a
// built-in filter's output over a computed input), so consumers do not walk it.
func (s *lazyState) markComputed(v interface{}) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) {
		return
	}
	if key, ok := identityOf(rv); ok {
		if s.resolved == nil {
			s.resolved = map[valueIdentity]resolvedValue{}
		}
		s.resolved[key] = resolvedValue{value: v}
	}
}

// whole is v as a consumer of the whole value sees it: every Lazy computed.
func (e *evalCtx) whole(v interface{}) (interface{}, error) {
	switch v.(type) {
	case nil, string, safeString, jsonString, SafeHTML, HTMLAttr, float64, int64, int, bool:
		return v, nil
	}
	return e.lazy.deep(v, 0)
}
