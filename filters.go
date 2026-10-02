package vascula

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// FilterArgs is the uniform argument shape every filter receives — positional
// args (truncate: 20) and named args (image_url: width: 200) in one struct, so
// the evaluator represents a filter call the same way regardless of which forms
// it uses. Positional-only filters ignore Named; arg-aware filters read it.
type FilterArgs struct {
	Pos   []interface{}
	Named map[string]interface{}
}

// Arg returns the i-th positional argument, or nil if absent.
func (a FilterArgs) Arg(i int) interface{} {
	if i >= 0 && i < len(a.Pos) {
		return a.Pos[i]
	}
	return nil
}

// NamedArg returns the named argument, or nil if absent.
func (a FilterArgs) NamedArg(name string) interface{} {
	if a.Named == nil {
		return nil
	}
	return a.Named[name]
}

// HasNamed reports whether a named argument was supplied (distinguishes an
// explicit nil from an absent arg, for filters that care).
func (a FilterArgs) HasNamed(name string) bool {
	if a.Named == nil {
		return false
	}
	_, ok := a.Named[name]
	return ok
}

// FilterFunc is a template filter: it transforms a piped value. Host filters
// (url_for, image_url, money, t) are injected via Options.Filters and override or
// extend these built-ins. Every filter receives the same FilterArgs shape;
// positional-only filters simply never read Named.
type FilterFunc func(input interface{}, args FilterArgs) (interface{}, error)

// builtinNamedArgs lists the named arguments each built-in accepts.
var builtinNamedArgs = map[string]map[string]bool{
	"default": {"allow_false": true},
}

// builtinFilters is the always-available set. The host layer adds application
// filters (image_url, money, t, …) on top.
func builtinFilters() map[string]FilterFunc {
	return map[string]FilterFunc{
		// default falls back when the input is nil, false, OR empty (empty string, empty array/map).
		// This is deliberately MORE aggressive than truthiness: `{{ settings.heading | default: shop.name }}`
		// must use the fallback for an empty-string setting (the most common template pattern), even though "" is
		// truthy in an `if`. A numeric 0 is KEPT (it's a real value, not "missing").
		// With allow_false: true an explicit false is kept (so a checkbox defaulting to true can still be
		// turned off).
		"default": func(in interface{}, a FilterArgs) (interface{}, error) {
			if in == false {
				if truthy(a.NamedArg("allow_false")) {
					return in, nil
				}
				return a.Arg(0), nil
			}
			// 0 is a real value — keep it (isEmptyValue treats only nil/""/empty-collection as empty).
			if in == nil || isEmptyValue(in) {
				return a.Arg(0), nil
			}
			return in, nil
		},
		// NOTE: there is deliberately NO `raw` filter. Output is auto-escaped, universally, with no
		// author-facing escape hatch. Trusted HTML reaches output only when the HOST hands the interpreter
		// a SafeHTML value (sanitize-on-write rich-text fields, host-generated markers). `raw` is rejected
		// at compile (see parser.go) so a template using it fails compilation, not at render.
		// escape produces the final escaped string — marked safe so the output stage does NOT escape it
		// AGAIN (without this, `{{ x | escape }}` double-escapes: < → &lt; → &amp;lt;).
		"escape": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return safeString(html.EscapeString(toString(in))), nil
		},
		"upcase":     strFilter(strings.ToUpper),
		"downcase":   strFilter(strings.ToLower),
		"capitalize": strFilter(capitalizeFirst),
		"strip":      strFilter(strings.TrimSpace),
		"lstrip":     strFilter(func(s string) string { return strings.TrimLeft(s, " \t\n\r") }),
		"rstrip":     strFilter(func(s string) string { return strings.TrimRight(s, " \t\n\r") }),
		"strip_html": strFilter(stripHTML),
		"newline_to_br": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return safeString(strings.ReplaceAll(html.EscapeString(toString(in)), "\n", "<br>")), nil
		},
		"append": func(in interface{}, a FilterArgs) (interface{}, error) {
			return toString(in) + toString(a.Arg(0)), nil
		},
		"prepend": func(in interface{}, a FilterArgs) (interface{}, error) {
			return toString(a.Arg(0)) + toString(in), nil
		},
		"replace": func(in interface{}, a FilterArgs) (interface{}, error) {
			return strings.ReplaceAll(toString(in), toString(a.Arg(0)), toString(a.Arg(1))), nil
		},
		"remove": func(in interface{}, a FilterArgs) (interface{}, error) {
			return strings.ReplaceAll(toString(in), toString(a.Arg(0)), ""), nil
		},
		// truncate is RUNE-aware: it counts and cuts on character boundaries (never splits a multibyte
		// rune). n is the total length including the ellipsis. Default n=50, ell="...".
		"truncate": func(in interface{}, a FilterArgs) (interface{}, error) {
			r := []rune(toString(in))
			n := 50
			if f, ok := toFloat(a.Arg(0)); ok {
				var err error
				n, err = intFromFloat(f)
				if err != nil {
					return nil, fmt.Errorf("truncate: %w", err)
				}
			}
			ell := "..."
			if a.Arg(1) != nil {
				ell = toString(a.Arg(1))
			}
			if len(r) <= n {
				return string(r), nil
			}
			cut := 0
			if ellLength := utf8.RuneCountInString(ell); n > ellLength {
				cut = n - ellLength
			}
			return string(r[:cut]) + ell, nil
		},
		// size: rune count for strings; element/key count for arrays AND maps (consistent with .size).
		"size": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return float64(sizeOf(in)), nil
		},
		"join": func(in interface{}, a FilterArgs) (interface{}, error) {
			sep := toString(a.Arg(0))
			items := iterate(in)
			parts := make([]string, len(items))
			for i, v := range items {
				parts[i] = toString(v)
			}
			return strings.Join(parts, sep), nil
		},

		// ---- array workhorses ----
		"split": func(in interface{}, a FilterArgs) (interface{}, error) {
			sep := toString(a.Arg(0))
			str := toString(in)
			if str == "" {
				return []interface{}{}, nil
			}
			var parts []string
			if sep == "" {
				for _, r := range str {
					parts = append(parts, string(r))
				}
			} else {
				parts = strings.Split(str, sep)
			}
			out := make([]interface{}, len(parts))
			for i, p := range parts {
				out[i] = p
			}
			return out, nil
		},
		"first": func(in interface{}, _ FilterArgs) (interface{}, error) {
			items := iterate(in)
			if len(items) == 0 {
				return nil, nil
			}
			return items[0], nil
		},
		"last": func(in interface{}, _ FilterArgs) (interface{}, error) {
			items := iterate(in)
			if len(items) == 0 {
				return nil, nil
			}
			return items[len(items)-1], nil
		},
		"map": func(in interface{}, a FilterArgs) (interface{}, error) {
			key := toString(a.Arg(0))
			items := iterate(in)
			out := make([]interface{}, len(items))
			for i, it := range items {
				out[i] = fieldValue(it, key)
			}
			return out, nil
		},
		// where: one arg = keep items whose property is truthy; two args = keep
		// items whose property equals the value.
		"where": func(in interface{}, a FilterArgs) (interface{}, error) {
			key := toString(a.Arg(0))
			hasVal := len(a.Pos) > 1
			want := a.Arg(1)
			out := []interface{}{}
			for _, it := range iterate(in) {
				got := fieldValue(it, key)
				if hasVal {
					if equalValues(got, want) {
						out = append(out, it)
					}
				} else if truthy(got) {
					out = append(out, it)
				}
			}
			return out, nil
		},
		// sort: optional property key. Values that don't compare keep input order
		// among themselves (stable sort on comparable pairs).
		"sort": func(in interface{}, a FilterArgs) (interface{}, error) {
			return sortItems(in, toString(a.Arg(0)), false), nil
		},
		"sort_natural": func(in interface{}, a FilterArgs) (interface{}, error) {
			return sortItems(in, toString(a.Arg(0)), true), nil
		},
		"uniq": func(in interface{}, _ FilterArgs) (interface{}, error) {
			items := iterate(in)
			out := make([]interface{}, 0, len(items))
			for _, it := range items {
				dup := false
				for _, seen := range out {
					if equalValues(it, seen) {
						dup = true
						break
					}
				}
				if !dup {
					out = append(out, it)
				}
			}
			return out, nil
		},
		"concat": func(in interface{}, a FilterArgs) (interface{}, error) {
			out := append([]interface{}{}, iterate(in)...)
			return append(out, iterate(a.Arg(0))...), nil
		},
		"compact": func(in interface{}, _ FilterArgs) (interface{}, error) {
			out := []interface{}{}
			for _, it := range iterate(in) {
				if it != nil {
					out = append(out, it)
				}
			}
			return out, nil
		},
		"reverse": func(in interface{}, _ FilterArgs) (interface{}, error) {
			items := iterate(in)
			out := make([]interface{}, len(items))
			for i, it := range items {
				out[len(items)-1-i] = it
			}
			return out, nil
		},
		// sum adds numbers (and numeric strings), skipping anything else. All
		// integers give an exact integer, unless the total overflows.
		"sum": func(in interface{}, a FilterArgs) (interface{}, error) {
			key := toString(a.Arg(0))
			total, exact := 0.0, int64(0)
			allInts := true
			for _, it := range iterate(in) {
				v := it
				if key != "" {
					v = fieldValue(it, key)
				}
				if n, ok := toInt64(v); ok && allInts {
					if next, ok := addInt(exact, n); ok {
						exact = next
						total += float64(n)
						continue
					}
					allInts = false
				}
				if f, ok := toFloat(v); ok {
					if _, isInt := toInt64(v); !isInt {
						allInts = false
					}
					total += f
				}
			}
			if allInts {
				return exact, nil
			}
			return total, nil
		},
		// slice works on strings (rune-safe) AND arrays: offset [, length].
		// Negative offset counts from the end.
		"slice": func(in interface{}, a FilterArgs) (interface{}, error) {
			off, okOff := toFloat(a.Arg(0))
			if !okOff {
				return nil, fmt.Errorf("slice: offset must be a number")
			}
			length := 1.0
			if len(a.Pos) > 1 {
				l, ok := toFloat(a.Arg(1))
				if !ok {
					return nil, fmt.Errorf("slice: length must be a number")
				}
				length = l
			}
			o, err := intFromFloat(off)
			if err != nil {
				return nil, fmt.Errorf("slice offset: %w", err)
			}
			l, err := intFromFloat(length)
			if err != nil {
				return nil, fmt.Errorf("slice length: %w", err)
			}
			if l < 0 {
				l = 0
			}
			if str, isStr := in.(string); isStr {
				runes := []rune(str)
				start := sliceStart(o, len(runes))
				if start >= len(runes) {
					return "", nil
				}
				end := len(runes)
				if l < end-start {
					end = start + l
				}
				return string(runes[start:end]), nil
			}
			items := iterate(in)
			start := sliceStart(o, len(items))
			if start >= len(items) {
				return []interface{}{}, nil
			}
			end := len(items)
			if l < end-start {
				end = start + l
			}
			return items[start:end], nil
		},

		// ---- string additions ----
		"truncatewords": func(in interface{}, a FilterArgs) (interface{}, error) {
			n, ok := toFloat(a.Arg(0))
			if !ok || n < 1 {
				return nil, fmt.Errorf("truncatewords: word count must be a positive number")
			}
			count, err := intFromFloat(n)
			if err != nil {
				return nil, fmt.Errorf("truncatewords: %w", err)
			}
			ellipsis := "..."
			if len(a.Pos) > 1 {
				ellipsis = toString(a.Arg(1))
			}
			words := strings.Fields(toString(in))
			if len(words) <= count {
				return strings.Join(words, " "), nil
			}
			return strings.Join(words[:count], " ") + ellipsis, nil
		},
		"replace_first": func(in interface{}, a FilterArgs) (interface{}, error) {
			return strings.Replace(toString(in), toString(a.Arg(0)), toString(a.Arg(1)), 1), nil
		},
		"remove_first": func(in interface{}, a FilterArgs) (interface{}, error) {
			return strings.Replace(toString(in), toString(a.Arg(0)), "", 1), nil
		},
		"strip_newlines": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return strings.NewReplacer("\r\n", "", "\n", "", "\r", "").Replace(toString(in)), nil
		},
		"escape_once": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return safeString(html.EscapeString(html.UnescapeString(toString(in)))), nil
		},
		"url_encode": func(in interface{}, _ FilterArgs) (interface{}, error) {
			return url.QueryEscape(toString(in)), nil
		},
		"url_decode": func(in interface{}, _ FilterArgs) (interface{}, error) {
			out, err := url.QueryUnescape(toString(in))
			if err != nil {
				return nil, fmt.Errorf("url_decode: %w", err)
			}
			return out, nil
		},

		// ---- numeric clamps ----
		"at_least": clamp(true),
		"at_most":  clamp(false),

		// date formats a time value with strftime directives.
		// Accepts RFC3339/date-only strings and time.Time; "now"/"today" use
		// render time. Unparseable input errors (silent wrong dates are worse).
		"date": func(in interface{}, a FilterArgs) (interface{}, error) {
			t, err := parseDateValue(in)
			if err != nil {
				return nil, err
			}
			format := toString(a.Arg(0))
			if format == "" {
				return nil, fmt.Errorf("date: format argument is required")
			}
			return strftime(t, format), nil
		},

		"plus":  arithmetic(addInt, func(a, b float64) float64 { return a + b }),
		"minus": arithmetic(subInt, func(a, b float64) float64 { return a - b }),
		"times": arithmetic(mulInt, func(a, b float64) float64 { return a * b }),
		// divide/modulo by zero is an ERROR — a real division bug must surface, not become a silent 0.
		"divided_by": dividedBy,
		"modulo":     modulo,
		// round/ceil/floor take an OPTIONAL precision (number of decimal places). No arg = integer.
		"round": precisionFilter(math.Round),
		"ceil":  precisionFilter(math.Ceil),
		"floor": precisionFilter(math.Floor),
		"abs":   absolute,
		"json": func(in interface{}, _ FilterArgs) (interface{}, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return nil, err
			}
			return jsonString(b), nil
		},
	}
}

func strFilter(fn func(string) string) FilterFunc {
	return func(in interface{}, _ FilterArgs) (interface{}, error) {
		return fn(toString(in)), nil
	}
}

// precisionFilter builds round/ceil/floor with an optional decimal-places arg. No arg → integer.
func precisionFilter(rounder func(float64) float64) FilterFunc {
	return func(in interface{}, a FilterArgs) (interface{}, error) {
		if x, ok := toInt64(in); ok && a.Arg(0) == nil {
			return x, nil
		}
		f, _ := toFloat(in)
		if a.Arg(0) == nil {
			return rounder(f), nil
		}
		p, _ := toFloat(a.Arg(0))
		scale := math.Pow(10, p)
		return rounder(f*scale) / scale, nil
	}
}

// capitalizeFirst upper-cases the first rune and lower-cases the rest (rune-aware).
func capitalizeFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(unicode.ToUpper(r[0])) + strings.ToLower(string(r[1:]))
}

// sizeOf returns the size used by both `| size` and `.size`: rune count for strings, len for arrays,
// key count for maps.
func sizeOf(v interface{}) int {
	if s, ok := v.(string); ok {
		return utf8.RuneCountInString(s)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len()
	}
	return 0
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sliceStart resolves a possibly-negative slice offset against a length.
func sliceStart(off, length int) int {
	if off < 0 {
		off = length + off
		if off < 0 {
			off = 0
		}
	}
	return off
}

// sortItems sorts by the items themselves or a property key. Natural mode
// compares strings case-insensitively. Incomparable pairs keep input order.
func sortItems(in interface{}, key string, natural bool) []interface{} {
	items := append([]interface{}{}, iterate(in)...)
	valOf := func(it interface{}) interface{} {
		if key == "" {
			return it
		}
		return fieldValue(it, key)
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := valOf(items[i]), valOf(items[j])
		if natural {
			return strings.ToLower(toString(a)) < strings.ToLower(toString(b))
		}
		if c, ok := compareValues(a, b); ok {
			return c < 0
		}
		return false
	})
	return items
}

// parseDateValue coerces a template value to a time for the date filter.
func parseDateValue(in interface{}) (time.Time, error) {
	switch v := in.(type) {
	case time.Time:
		return v, nil
	case *time.Time:
		if v != nil {
			return *v, nil
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "now" || s == "today" {
			return time.Now().UTC(), nil
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("date: cannot parse %q as a date", toString(in))
}

// strftime renders the common strftime directives. Unknown
// directives pass through literally (matching Ruby's lenient behavior).
func strftime(t time.Time, format string) string {
	var b strings.Builder
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' || i == len(runes)-1 {
			b.WriteRune(runes[i])
			continue
		}
		i++
		switch runes[i] {
		case 'Y':
			fmt.Fprintf(&b, "%04d", t.Year())
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'e':
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'B':
			b.WriteString(t.Month().String())
		case 'b', 'h':
			b.WriteString(t.Format("Jan"))
		case 'A':
			b.WriteString(t.Weekday().String())
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			b.WriteString(t.Format("03"))
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 'p':
			b.WriteString(t.Format("PM"))
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'Z':
			b.WriteString(t.Format("MST"))
		case 'z':
			b.WriteString(t.Format("-0700"))
		case '%':
			b.WriteRune('%')
		default:
			b.WriteRune('%')
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}
