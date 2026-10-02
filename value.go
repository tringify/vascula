package vascula

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// intFromFloat rejects values whose conversion to int would overflow or produce
// a platform-dependent result. Fractional finite values retain truncation toward zero.
func intFromFloat(value float64) (int, error) {
	limit := math.Ldexp(1, strconv.IntSize-1)
	if math.IsNaN(value) || math.IsInf(value, 0) || value >= limit || value < -limit {
		return 0, fmt.Errorf("number is outside the supported integer range")
	}
	return int(value), nil
}

// Data values are JSON-shaped (map[string]interface{}, []interface{}, string,
// float64, bool, nil). Hosts may also supply documented Go value types.
// These helpers give the evaluator Vascula's truthiness and iteration rules over that shape.

// safeString marks already-escaped/trusted HTML so output does not re-escape it (the result of escape/
// json, literal markup, and host-vouched SafeHTML values).
type safeString string

// jsonString is the json filter's output: safe inside <script> (the encoder
// escapes <, > and &) but not inside an HTML tag, where it is escaped on write.
type jsonString string

// SafeHTML is the ONLY way trusted HTML reaches output unescaped. The HOST wraps a Data/settings value in
// SafeHTML to vouch that it is already safe — e.g. rich-text fields that were sanitized on write
// (product.description, page/blog content) and host-generated editor markers. There is NO author-facing
// escape hatch: `raw` does not exist, output is auto-escaped, and only the host can emit unescaped HTML
// by handing the interpreter a SafeHTML value.
type SafeHTML string

// truthy: only nil, false, and the empty/absent marker are falsy. 0, "", and
// empty-but-present arrays are TRUTHY. Use `.size > 0` when "has items" is the question.
func truthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case emptyMarker:
		return false
	case map[string]interface{}:
		// A TYPED nil map inside an interface is not `nil` to a type switch, but
		// to a template author `{% if product.price %}` on an absent price must be
		// false — hosts build data maps via helpers that return nil maps.
		return x != nil
	case []interface{}:
		return x != nil
	default:
		return true
	}
}

// toString renders a value for output / string filters.
func toString(v interface{}) string {
	switch x := v.(type) {
	case nil, emptyMarker:
		return ""
	case safeString:
		return string(x)
	case jsonString:
		return string(x)
	case SafeHTML:
		return string(x)
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		// The shortest exact decimal: integral values print without ".0", and
		// values beyond the int64 range print their digits rather than wrapping.
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case *loopInfo:
		return toString(x.asMap())
	case fmt.Stringer:
		return x.String()
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			parts := make([]string, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				parts[i] = toString(rv.Index(i).Interface())
			}
			return strings.Join(parts, "")
		}
		return fmt.Sprintf("%v", v)
	}
}

// typeName gives a human label for a value, for comparison error messages.
func typeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "nil"
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	default:
		return reflect.TypeOf(v).Kind().String()
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func isEmptyValue(v interface{}) bool {
	switch x := v.(type) {
	case nil, emptyMarker:
		return true
	case SafeHTML:
		return x == ""
	case string:
		return x == ""
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len() == 0
		}
		return false
	}
}

// equalValues handles the `empty`/`blank` marker on either side, numeric compare,
// and falls back to string compare.
func equalValues(a, b interface{}) bool {
	if _, ok := a.(emptyMarker); ok {
		return isEmptyValue(b)
	}
	if _, ok := b.(emptyMarker); ok {
		return isEmptyValue(a)
	}
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if x, y, ok := intPair(a, b); ok {
		return x == y
	}
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}
	return toString(a) == toString(b)
}

// compareValues returns -1/0/1 for a<b/a==b/a>b. ok=false when the two are NOT meaningfully orderable
// (number-vs-string, or any non-string/non-number), so the caller errors rather than silently
// string-comparing nonsense. Both numbers compare numerically; both strings compare lexically.
func compareValues(a, b interface{}) (int, bool) {
	if x, y, ok := intPair(a, b); ok {
		switch {
		case x < y:
			return -1, true
		case x > y:
			return 1, true
		}
		return 0, true
	}
	af, aIsNum := toFloatStrict(a)
	bf, bIsNum := toFloatStrict(b)
	if aIsNum && bIsNum {
		switch {
		case af < bf:
			return -1, true
		case af > bf:
			return 1, true
		default:
			return 0, true
		}
	}
	as, aIsStr := a.(string)
	bs, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return strings.Compare(as, bs), true
	}
	return 0, false // cross-type / unorderable → caller errors
}

// toFloatStrict is toFloat WITHOUT string coercion — a string is not a number for ordering purposes
// (so "5" > 3 is a cross-type error, not a silent string compare).
func toFloatStrict(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

// containsValue implements `a contains b` — substring for strings, membership for
// arrays.
func containsValue(a, b interface{}) bool {
	switch x := a.(type) {
	case string:
		return strings.Contains(x, toString(b))
	case nil:
		return false
	}
	rv := reflect.ValueOf(a)
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			if equalValues(rv.Index(i).Interface(), b) {
				return true
			}
		}
	}
	return false
}

// fieldValue resolves a named property. Supports map access plus Vascula
// collection properties (size/first/last) on arrays and strings. A missing field is nil.
func fieldValue(base interface{}, name string) interface{} {
	switch m := base.(type) {
	case *loopInfo:
		return m.field(name)
	case map[string]interface{}:
		if v, ok := m[name]; ok {
			return v
		}
		return pseudoProp(base, name) // e.g. map.size when there is no "size" key
	case map[string]string:
		if v, ok := m[name]; ok {
			return v
		}
		return pseudoProp(base, name)
	case nil:
		// A missing collection has zero elements: nil.size is 0 (not nil), so the universal idiom
		// `{% if X.items.size > 0 %}` is safe when X.items is absent — no nil-vs-number compare error.
		// .first/.last on nil stay nil.
		if name == "size" {
			return float64(0)
		}
		return nil
	}
	rv := reflect.ValueOf(base)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return pseudoProp(base, name)
		}
		key := reflect.ValueOf(name).Convert(rv.Type().Key())
		v := rv.MapIndex(key)
		if v.IsValid() {
			return v.Interface()
		}
		return pseudoProp(base, name)
	case reflect.Slice, reflect.Array, reflect.String:
		return pseudoProp(base, name)
	}
	return nil
}

func pseudoProp(base interface{}, name string) interface{} {
	rv := reflect.ValueOf(base)
	switch name {
	case "size":
		// Consistent with the `| size` filter: rune count for strings, len for arrays/maps.
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
			return float64(sizeOf(base))
		}
	case "first":
		if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) && rv.Len() > 0 {
			return rv.Index(0).Interface()
		}
	case "last":
		if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) && rv.Len() > 0 {
			return rv.Index(rv.Len() - 1).Interface()
		}
	}
	return nil
}

// indexValue resolves base[idx]: numeric index into arrays, key into maps.
func indexValue(base, idx interface{}) interface{} {
	if s, ok := idx.(string); ok {
		return fieldValue(base, s)
	}
	if f, ok := toFloat(idx); ok {
		rv := reflect.ValueOf(base)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			i := int(f)
			if i < 0 {
				i += rv.Len()
			}
			if i >= 0 && i < rv.Len() {
				return rv.Index(i).Interface()
			}
		}
	}
	return nil
}

// iterate returns base as a slice of values for {% for %}. Arrays/slices yield their elements. Maps
// yield [key, value] 2-element pairs (item[0]=key, item[1]=value), with keys sorted for
// deterministic output (Go maps have no stable order). Non-iterables yield nil.
func iterate(base interface{}) []interface{} {
	switch x := base.(type) {
	case *loopInfo:
		return iterate(x.asMap())
	case []interface{}:
		return x
	case nil, emptyMarker:
		return nil
	}
	rv := reflect.ValueOf(base)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out
	case reflect.Map:
		keys := rv.MapKeys()
		strs := make([]string, len(keys))
		byKey := make(map[string]reflect.Value, len(keys))
		for i, k := range keys {
			s := toString(k.Interface())
			strs[i] = s
			byKey[s] = k
		}
		sort.Strings(strs)
		out := make([]interface{}, len(strs))
		for i, s := range strs {
			out[i] = []interface{}{s, rv.MapIndex(byKey[s]).Interface()}
		}
		return out
	}
	return nil
}
