package vascula

import (
	"fmt"
	"math"
	"reflect"
)

const (
	defaultMaxSourceBytes     = 1 << 20
	defaultMaxTokens          = 100_000
	defaultMaxNesting         = 128
	defaultMaxExpressionDepth = 256
	defaultMaxValueBytes      = 2 << 20
	defaultMaxValueNodes      = 50_000
	maxValueDepth             = 64
)

func (p *parser) enter() error {
	if p.nesting >= p.maxNesting {
		return atOffset(p.peek().pos, "compile budget exceeded: nesting too deep")
	}
	p.nesting++
	return nil
}

func (p *parser) leave() { p.nesting-- }

// value bounds JSON-shaped values before conversion or processing. It counts
// repeated references too: serializing them can expand their contents repeatedly.
// The depth bound also terminates cyclic maps/slices without recursive overflow.
// Custom methods on host objects remain trusted code, not engine-controlled work.
//
// A filter chain hands the same collections from step to step, so each map and
// slice is walked once per render: its size and height are remembered by
// identity and reused, keeping the totals and limits exactly as if the value
// were walked again. Values are never mutated during a render (built-in filters
// return new collections; host filters are trusted code), so an identity keeps
// describing the same contents.
func (b *budget) value(value interface{}) error {
	nodes, bytes := 0, 0
	addBytes := func(n int) error {
		if n > b.maxValueBytes-bytes {
			return fmt.Errorf("render budget exceeded: value too large")
		}
		bytes += n
		return b.spend(n / 64)
	}
	var visit func(reflect.Value, int) (int, error)
	visit = func(v reflect.Value, depth int) (int, error) {
		if depth > maxValueDepth {
			return 0, fmt.Errorf("render budget exceeded: value nesting too deep")
		}
		if nodes >= b.maxValueNodes {
			return 0, fmt.Errorf("render budget exceeded: too many value nodes")
		}
		nodes++
		if err := b.step(); err != nil {
			return 0, err
		}
		for v.IsValid() && v.Kind() == reflect.Interface {
			if v.IsNil() {
				return 0, nil
			}
			v = v.Elem()
		}
		if !v.IsValid() {
			return 0, nil
		}
		switch v.Kind() {
		case reflect.Float32, reflect.Float64:
			if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
				return 0, fmt.Errorf("non-finite numbers are not supported")
			}
		case reflect.String:
			return 0, addBytes(v.Len())
		case reflect.Ptr:
			if !v.IsNil() {
				h, err := visit(v.Elem(), depth+1)
				return h + 1, err
			}
		case reflect.Slice, reflect.Map:
			if v.Len() > b.maxValueNodes-nodes {
				return 0, fmt.Errorf("render budget exceeded: too many value nodes")
			}
			key, keyed := identityOf(v)
			if keyed {
				if seen, ok := b.validated[key]; ok {
					if depth+seen.height > maxValueDepth {
						return 0, fmt.Errorf("render budget exceeded: value nesting too deep")
					}
					if seen.nodes > b.maxValueNodes-nodes {
						return 0, fmt.Errorf("render budget exceeded: too many value nodes")
					}
					if seen.bytes > b.maxValueBytes-bytes {
						return 0, fmt.Errorf("render budget exceeded: value too large")
					}
					// Charge the walk's steps without repeating it, so the
					// render budget bounds work exactly as before.
					if err := b.spend(seen.steps); err != nil {
						return 0, err
					}
					nodes += seen.nodes
					bytes += seen.bytes
					return seen.height, nil
				}
			}
			startNodes, startBytes, startSteps, height := nodes, bytes, b.steps, 0
			grow := func(h int) {
				if h+1 > height {
					height = h + 1
				}
			}
			if v.Kind() == reflect.Slice {
				for i := 0; i < v.Len(); i++ {
					h, err := visit(v.Index(i), depth+1)
					if err != nil {
						return 0, err
					}
					grow(h)
				}
			} else {
				iter := v.MapRange()
				for iter.Next() {
					if iter.Key().Kind() == reflect.String {
						if err := addBytes(iter.Key().Len()); err != nil {
							return 0, err
						}
					}
					h, err := visit(iter.Value(), depth+1)
					if err != nil {
						return 0, err
					}
					grow(h)
				}
			}
			if keyed {
				if b.validated == nil {
					b.validated = map[valueIdentity]validatedValue{}
				}
				b.validated[key] = validatedValue{nodes: nodes - startNodes, bytes: bytes - startBytes, steps: b.steps - startSteps, height: height}
			}
			return height, nil
		case reflect.Array:
			if v.Len() > b.maxValueNodes-nodes {
				return 0, fmt.Errorf("render budget exceeded: too many value nodes")
			}
			height := 0
			for i := 0; i < v.Len(); i++ {
				h, err := visit(v.Index(i), depth+1)
				if err != nil {
					return 0, err
				}
				if h+1 > height {
					height = h + 1
				}
			}
			return height, nil
		}
		return 0, nil
	}
	// fast walks the JSON shapes hosts pass almost exclusively without
	// reflection; anything else takes the general walk above.
	var fast func(interface{}, int) (int, error)
	fast = func(x interface{}, depth int) (int, error) {
		switch x.(type) {
		case nil, bool, int, int64, string, float64, map[string]interface{}, []interface{}:
		default:
			return visit(reflect.ValueOf(x), depth)
		}
		if depth > maxValueDepth {
			return 0, fmt.Errorf("render budget exceeded: value nesting too deep")
		}
		if nodes >= b.maxValueNodes {
			return 0, fmt.Errorf("render budget exceeded: too many value nodes")
		}
		nodes++
		if err := b.step(); err != nil {
			return 0, err
		}
		switch t := x.(type) {
		case string:
			return 0, addBytes(len(t))
		case float64:
			if math.IsNaN(t) || math.IsInf(t, 0) {
				return 0, fmt.Errorf("non-finite numbers are not supported")
			}
			return 0, nil
		case map[string]interface{}, []interface{}:
			rv := reflect.ValueOf(t)
			if rv.Len() > b.maxValueNodes-nodes {
				return 0, fmt.Errorf("render budget exceeded: too many value nodes")
			}
			key, keyed := identityOf(rv)
			if keyed {
				if seen, ok := b.validated[key]; ok {
					if depth+seen.height > maxValueDepth {
						return 0, fmt.Errorf("render budget exceeded: value nesting too deep")
					}
					if seen.nodes > b.maxValueNodes-nodes {
						return 0, fmt.Errorf("render budget exceeded: too many value nodes")
					}
					if seen.bytes > b.maxValueBytes-bytes {
						return 0, fmt.Errorf("render budget exceeded: value too large")
					}
					// Charge the walk's steps without repeating it, so the
					// render budget bounds work exactly as before.
					if err := b.spend(seen.steps); err != nil {
						return 0, err
					}
					nodes += seen.nodes
					bytes += seen.bytes
					return seen.height, nil
				}
			}
			startNodes, startBytes, startSteps, height := nodes, bytes, b.steps, 0
			if m, ok := t.(map[string]interface{}); ok {
				for k, item := range m {
					if err := addBytes(len(k)); err != nil {
						return 0, err
					}
					h, err := fast(item, depth+1)
					if err != nil {
						return 0, err
					}
					if h+1 > height {
						height = h + 1
					}
				}
			} else {
				for _, item := range t.([]interface{}) {
					h, err := fast(item, depth+1)
					if err != nil {
						return 0, err
					}
					if h+1 > height {
						height = h + 1
					}
				}
			}
			if keyed {
				if b.validated == nil {
					b.validated = map[valueIdentity]validatedValue{}
				}
				b.validated[key] = validatedValue{nodes: nodes - startNodes, bytes: bytes - startBytes, steps: b.steps - startSteps, height: height}
			}
			return height, nil
		}
		return 0, nil
	}
	_, err := fast(value, 0)
	return err
}

// valueIdentity names one map or slice by its storage, so a value validated
// earlier in the render is recognised wherever it reappears.
type valueIdentity struct {
	typ reflect.Type
	ptr uintptr
	len int
}

// validatedValue is what the walk below an identity added: nodes beneath it
// (excluding itself), string bytes, budget steps, and its height.
type validatedValue struct {
	nodes, bytes, steps, height int
}

func identityOf(v reflect.Value) (valueIdentity, bool) {
	if v.IsNil() || v.Len() == 0 {
		return valueIdentity{}, false
	}
	return valueIdentity{typ: v.Type(), ptr: v.Pointer(), len: v.Len()}, true
}

// A sequence loop copies element references rather than recursively processing
// each element. Bound that allocation and work here; output, comparisons and
// filters still validate any nested values they actually consume. This permits
// rich host records to contain fields the loop never reads.
func (b *budget) loopCollection(value interface{}) error {
	v := reflect.ValueOf(value)
	if v.IsValid() && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) {
		if v.Len() >= b.maxValueNodes {
			return fmt.Errorf("render budget exceeded: too many collection entries")
		}
		return b.spend(v.Len() + 1)
	}
	// Map iteration additionally stringifies and sorts keys. Retain the full
	// bound before those conversions; non-iterables remain harmless.
	return b.value(value)
}

// collection bounds a value passing through a shallow filter: a sequence by
// its length, like a loop; anything else fully.
func (b *budget) collection(value interface{}) error {
	return b.loopCollection(value)
}

func (b *budget) stringSize(size int) error {
	if size < 0 || size > b.maxValueBytes {
		return fmt.Errorf("render budget exceeded: value too large")
	}
	return nil
}

func (b *budget) growString(current, extra int) error {
	if err := b.stringSize(current); err != nil {
		return err
	}
	if extra < 0 || extra > b.maxValueBytes-current {
		return fmt.Errorf("render budget exceeded: value too large")
	}
	return nil
}
