package vascula

import (
	"fmt"
	"sort"
	"strings"
)

// Most built-ins cannot amplify a validated input beyond a fixed small factor.
// These operations can multiply user-selected sizes, so check before allocating.
func boundedBuiltinFilters(work *budget) map[string]FilterFunc {
	filters := builtinFilters()
	for _, name := range []string{"append", "prepend"} {
		name := name
		filters[name] = func(in interface{}, args FilterArgs) (interface{}, error) {
			left, right := toString(in), toString(args.Arg(0))
			if err := work.growString(len(left), len(right)); err != nil {
				return nil, err
			}
			if name == "prepend" {
				return right + left, nil
			}
			return left + right, nil
		}
	}
	for _, name := range []string{"replace", "replace_first"} {
		name := name
		filters[name] = func(in interface{}, args FilterArgs) (interface{}, error) {
			s, old, replacement := toString(in), toString(args.Arg(0)), toString(args.Arg(1))
			if err := work.stringSize(len(s)); err != nil {
				return nil, err
			}
			count := strings.Count(s, old)
			if name == "replace_first" && count > 1 {
				count = 1
			}
			growth := len(replacement) - len(old)
			if growth > 0 && count > (work.maxValueBytes-len(s))/growth {
				return nil, fmt.Errorf("render budget exceeded: replacement too large")
			}
			return strings.Replace(s, old, replacement, count), nil
		}
	}
	filters["join"] = func(in interface{}, args FilterArgs) (interface{}, error) {
		items, separator := iterate(in), toString(args.Arg(0))
		parts := make([]string, len(items))
		size := 0
		for i, item := range items {
			parts[i] = toString(item)
			if err := work.growString(size, len(parts[i])); err != nil {
				return nil, err
			}
			size += len(parts[i])
			if i > 0 {
				if err := work.growString(size, len(separator)); err != nil {
					return nil, err
				}
				size += len(separator)
			}
		}
		return strings.Join(parts, separator), nil
	}
	filters["concat"] = func(in interface{}, args FilterArgs) (interface{}, error) {
		left, right := iterate(in), iterate(args.Arg(0))
		if len(left) >= work.maxValueNodes || len(right) >= work.maxValueNodes-len(left) {
			return nil, fmt.Errorf("render budget exceeded: concatenated collection too large")
		}
		out := make([]interface{}, 0, len(left)+len(right))
		out = append(out, left...)
		return append(out, right...), nil
	}
	filters["uniq"] = func(in interface{}, _ FilterArgs) (interface{}, error) {
		items := iterate(in)
		out := make([]interface{}, 0, len(items))
		for _, item := range items {
			duplicate := false
			for _, seen := range out {
				if err := work.step(); err != nil {
					return nil, err
				}
				if equalValues(item, seen) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				out = append(out, item)
			}
		}
		return out, nil
	}
	for _, name := range []string{"sort", "sort_natural"} {
		name := name
		filters[name] = func(in interface{}, args FilterArgs) (interface{}, error) {
			items := append([]interface{}{}, iterate(in)...)
			key := toString(args.Arg(0))
			value := func(item interface{}) interface{} {
				if key != "" {
					return fieldValue(item, key)
				}
				return item
			}
			// The sort keys are what this filter consumes; check them (the
			// collection itself is bounded by length, see shallowFilters).
			for _, item := range items {
				if err := work.value(value(item)); err != nil {
					return nil, err
				}
			}
			var budgetErr error
			sort.SliceStable(items, func(i, j int) bool {
				if budgetErr != nil {
					return false
				}
				if budgetErr = work.step(); budgetErr != nil {
					return false
				}
				a, b := value(items[i]), value(items[j])
				if name == "sort_natural" {
					return strings.ToLower(toString(a)) < strings.ToLower(toString(b))
				}
				if compared, ok := compareValues(a, b); ok {
					return compared < 0
				}
				return false
			})
			if budgetErr != nil {
				return nil, budgetErr
			}
			return items, nil
		}
	}
	filters["where"] = func(in interface{}, args FilterArgs) (interface{}, error) {
		key := toString(args.Arg(0))
		hasValue := len(args.Pos) > 1
		want := args.Arg(1)
		out := []interface{}{}
		for _, item := range iterate(in) {
			got := fieldValue(item, key)
			// The compared field is what this filter consumes; check it.
			if err := work.value(got); err != nil {
				return nil, err
			}
			if hasValue {
				if equalValues(got, want) {
					out = append(out, item)
				}
			} else if truthy(got) {
				out = append(out, item)
			}
		}
		return out, nil
	}
	return filters
}

// shallowFilters only select, reorder or count entries of a collection; they
// never serialize or deeply compare an entry. Like a for loop, a collection
// passing through them is bounded by its length, and fields are checked when
// something consumes them: where and sort check the field they compare, and
// output, json, join and the rest check whatever reaches them. A host filter
// of the same name is not shallow.
var shallowFilters = map[string]bool{
	"where": true, "sort": true, "sort_natural": true, "map": true, "first": true,
	"last": true, "reverse": true, "slice": true, "size": true, "concat": true,
	"compact": true,
}
