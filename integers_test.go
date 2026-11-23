package vascula

import (
	"strings"
	"testing"
)

func renderWith(t *testing.T, src string, opts Options) string {
	t.Helper()
	tmpl, err := Compile(src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	out, err := tmpl.Render(opts)
	if err != nil {
		t.Fatalf("render %q: %v", src, err)
	}
	return out
}

func TestIntegersStayExact(t *testing.T) {
	ctx := Options{Data: map[string]interface{}{"product": map[string]interface{}{"id": int64(9007199254740993)}}, Allow: []string{"product"}}
	for src, want := range map[string]string{
		`{{ 9007199254740993 | plus: 0 }}`:           "9007199254740993",
		`{{ product.id | plus: 1 }}`:                 "9007199254740994",
		`{{ product.id | minus: 2 }}`:                "9007199254740991",
		`{{ 3037000499 | times: 3037000499 }}`:       "9223372030926249001",
		`{{ 9223372036854775807 | plus: 1 }}`:        "9223372036854776000", // overflow falls back to float
		`{{ 10 | divided_by: 4 }}`:                   "2.5",
		`{{ 10 | divided_by: 5 }}`:                   "2",
		`{{ 7 | modulo: 3 }}`:                        "1",
		`{{ -7 | abs }}`:                             "7",
		`{{ 5 | at_least: 9 }} {{ 5 | at_most: 9 }}`: "9 5",
		`{{ 1.5 | plus: 1 }}`:                        "2.5",
		`{% if 9007199254740993 == 9007199254740992 %}same{% else %}different{% endif %}`: "different",
		`{% if product.id > 9007199254740992 %}greater{% endif %}`:                        "greater",
		`{{ 7 | round }}`: "7",
	} {
		if got := renderWith(t, src, ctx); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestHostFiltersKeepJSONNumbers(t *testing.T) {
	var got []interface{}
	record := func(in interface{}, args FilterArgs) (interface{}, error) {
		got = append(got, in, args.Arg(0), args.NamedArg("width"))
		return "", nil
	}
	renderWith(t, `{{ 5 | probe: 6, width: 400 }}{{ 1152921504606846976 | probe: 1 }}`, Options{Filters: map[string]FilterFunc{"probe": record}})
	for i, want := range []interface{}{float64(5), float64(6), float64(400), int64(1152921504606846976), float64(1), nil} {
		if got[i] != want {
			t.Errorf("host filter value %d = %#v, want %#v", i, got[i], want)
		}
	}
}

func TestLargeFloatsPrintTheirDigits(t *testing.T) {
	out := renderWith(t, `{{ n }}`, Options{Data: map[string]interface{}{"n": 1e20}, Allow: []string{"n"}})
	if out != "100000000000000000000" {
		t.Errorf("got %q", out)
	}
}

func TestSumStaysExactForIntegers(t *testing.T) {
	big := []interface{}{int64(9007199254740993), int64(1)}
	if out := renderWith(t, `{{ n | sum }} {{ (1..4) | sum }} {{ f | sum }}`, Options{Settings: map[string]interface{}{"n": big}, Data: map[string]interface{}{"n": big, "f": []interface{}{1.5, int64(2), "x"}}, Allow: []string{"n", "f"}}); out != "9007199254740994 10 3.5" {
		t.Fatalf("got %q", out)
	}
}

func TestRawAndCommentErrorsHavePositions(t *testing.T) {
	for src, want := range map[string]string{
		"ok\n{% raw %}x{% endraw %}":     "line 2, col 4",
		"ok\n {% comment %}never closed": "line 2, col 2",
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v", src, err)
		}
	}
}
