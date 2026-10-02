package vascula

import (
	"strings"
	"testing"
)

// correctness_test.go pins the engine's behavior in cases that are easy to get wrong.

// renderErr renders and returns (output, error) without t.Fatal, for tests that assert an error.
func renderErr(src string, opts Options) (string, error) {
	tmpl, err := compileFixture(src)
	if err != nil {
		return "", err
	}
	return tmpl.Render(opts)
}

// compileErr returns the compile error (nil if it compiled).
func mustCompileErr(src string) error {
	_, err := compileFixture(src)
	return err
}

// --- unless must be !truthy(cond), not cond == false ---
func TestUnlessTruthiness(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		// nil is falsy → unless renders
		{"unless nil", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": nil}}, "R"},
		// empty marker is falsy → unless renders
		{"unless empty literal", "{% unless empty %}R{% endunless %}", Options{}, "R"},
		// false → unless renders
		{"unless false", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": false}}, "R"},
		// missing setting (nil) → renders
		{"unless missing", "{% unless settings.missing %}R{% endunless %}", Options{}, "R"},
		// truthy values → unless does NOT render
		{"unless true", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": true}}, ""},
		{"unless zero (truthy)", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": 0.0}}, ""},
		{"unless empty string (truthy)", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": ""}}, ""},
		{"unless nonempty", "{% unless settings.x %}R{% endunless %}", Options{Settings: map[string]interface{}{"x": "hi"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := render(t, c.src, c.opts); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

// --- site.settings must survive {% render %} into a child template ---
func TestGlobalsAcrossRender(t *testing.T) {
	child, _ := compileFixture("[{{ site.settings.brand }}]")
	resolve := func(name string) (Child, error) {
		return Child{Template: child, Allow: nil, Settings: map[string]interface{}{}}, nil
	}
	out := render(t, "{% render 'child' %}", Options{
		Globals: siteGlobals(map[string]interface{}{"brand": "Acme"}),
		Resolve: resolve,
	})
	if out != "[Acme]" {
		t.Fatalf("child should read site.settings; got %q", out)
	}
}

func TestSettingsIsAReservedRenderArg(t *testing.T) {
	if _, err := Compile("{% render 'child', settings: 1 %}"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("passing settings as a render argument must fail compilation; got %v", err)
	}
}

// --- for limit/offset must NOT swallow expression errors (sandbox leak) ---
func TestForLimitOffsetSurfacesErrors(t *testing.T) {
	// "widget" is not an allowed root → reading it in limit: must error, not be ignored.
	_, err := renderErr("{% for x in settings.items limit: widget.count %}{{ x }}{% endfor %}",
		Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b"}}})
	if err == nil || !strings.Contains(err.Error(), "undeclared name") {
		t.Fatalf("undeclared access in limit: must surface; got %v", err)
	}
	_, err = renderErr("{% for x in settings.items offset: widget.n %}{{ x }}{% endfor %}",
		Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b"}}})
	if err == nil || !strings.Contains(err.Error(), "undeclared name") {
		t.Fatalf("undeclared access in offset: must surface; got %v", err)
	}
}

// --- and/or return the OPERAND value, not a bool ---
func TestAndOrReturnValue(t *testing.T) {
	// or returns the first truthy operand; here the fallback string.
	out := render(t, "{% assign name = settings.name or 'Guest' %}{{ name }}",
		Options{Settings: map[string]interface{}{"name": nil}})
	if out != "Guest" {
		t.Fatalf("or should return the fallback value; got %q", out)
	}
	// or returns the left when it's truthy.
	out = render(t, "{% assign name = settings.name or 'Guest' %}{{ name }}",
		Options{Settings: map[string]interface{}{"name": "Alice"}})
	if out != "Alice" {
		t.Fatalf("or should return the truthy left; got %q", out)
	}
	// and returns the right when left is truthy.
	out = render(t, "{% assign v = settings.a and settings.b %}{{ v }}",
		Options{Settings: map[string]interface{}{"a": "x", "b": "y"}})
	if out != "y" {
		t.Fatalf("and should return the right operand when left truthy; got %q", out)
	}
	// and returns the (falsy) left when left is falsy.
	out = render(t, "{% assign v = settings.a and settings.b %}[{{ v }}]",
		Options{Settings: map[string]interface{}{"a": nil, "b": "y"}})
	if out != "[]" {
		t.Fatalf("and should return falsy left; got %q", out)
	}
	// still works in if-conditions (truthiness of the returned value).
	out = render(t, "{% if settings.a and settings.b %}both{% endif %}",
		Options{Settings: map[string]interface{}{"a": "x", "b": "y"}})
	if out != "both" {
		t.Fatalf("and/or must still drive if; got %q", out)
	}
}

// --- default falls back on nil/false/EMPTY; keeps 0 ---
func TestDefaultEmptyFallback(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want string
	}{
		{"zero KEPT", Options{Settings: map[string]interface{}{"x": 0.0}}, "0"},
		{"empty string FALLS BACK", Options{Settings: map[string]interface{}{"x": ""}}, "fb"}, // a common pattern
		{"empty array falls back", Options{Settings: map[string]interface{}{"x": []interface{}{}}}, "fb"},
		{"nil falls back", Options{Settings: map[string]interface{}{"x": nil}}, "fb"},
		{"false falls back", Options{Settings: map[string]interface{}{"x": false}}, "fb"},
		{"value kept", Options{Settings: map[string]interface{}{"x": "v"}}, "v"},
		{"nonzero number kept", Options{Settings: map[string]interface{}{"x": 5.0}}, "5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(t, "{{ settings.x | default: 'fb' }}", c.opts)
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
	// A common pattern: empty heading → shop name.
	got := render(t, "{{ settings.heading | default: shop.name }}",
		Options{Settings: map[string]interface{}{"heading": ""}, Data: map[string]interface{}{"shop": map[string]interface{}{"name": "Acme"}}, Allow: []string{"shop"}})
	if got != "Acme" {
		t.Fatalf("empty heading must fall back to shop.name; got %q", got)
	}
	// allow_false keeps an explicit false.
	got = render(t, "{{ settings.x | default: 'fb', allow_false: true }}", Options{Settings: map[string]interface{}{"x": false}})
	if got != "false" {
		t.Fatalf("allow_false should keep false; got %q", got)
	}
}

// --- filters work inside if / for / render args, not only output/assign ---
func TestFiltersInExpressions(t *testing.T) {
	// filter in an if-condition
	out := render(t, "{% if settings.s | upcase == 'HI' %}Y{% endif %}", Options{Settings: map[string]interface{}{"s": "hi"}})
	if out != "Y" {
		t.Fatalf("filter in if condition; got %q", out)
	}
	// filter in a for collection (split-like via a host filter is overkill; use size guard)
	out = render(t, "{% for x in settings.items limit: settings.n | plus: 1 %}{{ x }}{% endfor %}",
		Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b", "c", "d"}, "n": 1.0}})
	if out != "ab" {
		t.Fatalf("filter in for limit; got %q", out)
	}
}

// --- unicode-aware truncate/size/capitalize (rune, not byte) ---
func TestUnicodeFilters(t *testing.T) {
	// truncate must not split a multibyte rune.
	out := render(t, "{{ settings.s | truncate: 5, '' }}", Options{Settings: map[string]interface{}{"s": "café ☕ latte"}})
	if out != strings.ToValidUTF8(out, "?") || strings.Contains(out, "�") {
		t.Fatalf("truncate produced invalid UTF-8: %q", out)
	}
	if len([]rune(out)) != 5 {
		t.Fatalf("truncate should count runes; got %q (%d runes)", out, len([]rune(out)))
	}
	// size on a string = rune count.
	out = render(t, "{{ settings.s | size }}", Options{Settings: map[string]interface{}{"s": "café"}})
	if out != "4" {
		t.Fatalf("size should be rune count 4; got %q", out)
	}
	// capitalize: first rune upper, REST lowercased.
	out = render(t, "{{ settings.s | capitalize }}", Options{Settings: map[string]interface{}{"s": "hELLO"}})
	if out != "Hello" {
		t.Fatalf("capitalize should be 'Hello'; got %q", out)
	}
}

// --- round/ceil/floor with precision ---
func TestNumberPrecision(t *testing.T) {
	out := render(t, "{{ settings.x | round: 2 }}", Options{Settings: map[string]interface{}{"x": 3.14159}})
	if out != "3.14" {
		t.Fatalf("round: 2; got %q", out)
	}
	out = render(t, "{{ settings.x | round }}", Options{Settings: map[string]interface{}{"x": 3.7}})
	if out != "4" {
		t.Fatalf("round no-arg = integer; got %q", out)
	}
	out = render(t, "{{ settings.x | ceil: 1 }}", Options{Settings: map[string]interface{}{"x": 3.141}})
	if out != "3.2" {
		t.Fatalf("ceil: 1; got %q", out)
	}
}

// --- | size and .size agree for maps ---
func TestSizeMapConsistency(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": 2, "c": 3}
	out := render(t, "{{ settings.m.size }}-{{ settings.m | size }}", Options{Settings: map[string]interface{}{"m": m}})
	if out != "3-3" {
		t.Fatalf(".size and | size must agree for maps; got %q", out)
	}
}

// --- divide/modulo by zero is an error, not a silent 0 ---
func TestDivideByZeroErrors(t *testing.T) {
	if _, err := renderErr("{{ settings.x | divided_by: 0 }}", Options{Settings: map[string]interface{}{"x": 10.0}}); err == nil {
		t.Fatal("divided_by: 0 must error")
	}
	if _, err := renderErr("{{ settings.x | modulo: 0 }}", Options{Settings: map[string]interface{}{"x": 10.0}}); err == nil {
		t.Fatal("modulo: 0 must error")
	}
}

// --- slot is rejected at COMPILE time (not wired) ---
func TestSlotRejectedAtCompile(t *testing.T) {
	if err := mustCompileErr("{% slot 'main' %}"); err == nil {
		t.Fatal("slot must be rejected at compile (not wired)")
	}
}

// --- dynamic render target rejected at COMPILE time ---
func TestDynamicRenderRejectedAtCompile(t *testing.T) {
	if err := mustCompileErr("{% render settings.x %}"); err == nil {
		t.Fatal("dynamic render target must be rejected at compile")
	}
	// literal still compiles
	if err := mustCompileErr("{% render 'child' %}"); err != nil {
		t.Fatalf("literal render must compile; got %v", err)
	}
}

// --- maps iterate as [key, value] pairs ---
func TestMapIteration(t *testing.T) {
	m := map[string]interface{}{"a": "1", "b": "2"}
	// item[0]=key, item[1]=value. Sort to make output deterministic via a known-key map of size 1 first.
	one := map[string]interface{}{"k": "v"}
	out := render(t, "{% for kv in settings.m %}{{ kv[0] }}={{ kv[1] }}{% endfor %}",
		Options{Settings: map[string]interface{}{"m": one}})
	if out != "k=v" {
		t.Fatalf("map iteration should yield [key,value]; got %q", out)
	}
	// non-empty map → loop runs (was silently empty before)
	out = render(t, "{% for kv in settings.m %}X{% else %}EMPTY{% endfor %}",
		Options{Settings: map[string]interface{}{"m": m}})
	if out == "EMPTY" {
		t.Fatal("non-empty map must iterate, not hit else")
	}
	if len(out) != 2 { // two entries → "XX"
		t.Fatalf("map of 2 should iterate twice; got %q", out)
	}
}

// --- cross-type < / > is an ERROR, not a silent string compare ---
func TestCrossTypeComparisonErrors(t *testing.T) {
	// number vs string with > → error
	if _, err := renderErr("{% if settings.n > settings.s %}Y{% endif %}",
		Options{Settings: map[string]interface{}{"n": 5.0, "s": "banana"}}); err == nil {
		t.Fatal("number > string must error")
	}
	if _, err := renderErr("{% if settings.s < settings.n %}Y{% endif %}",
		Options{Settings: map[string]interface{}{"n": 5.0, "s": "banana"}}); err == nil {
		t.Fatal("string < number must error")
	}
	// same-type comparisons still work
	out := render(t, "{% if settings.a > settings.b %}Y{% else %}N{% endif %}",
		Options{Settings: map[string]interface{}{"a": 5.0, "b": 3.0}})
	if out != "Y" {
		t.Fatalf("number > number must work; got %q", out)
	}
	out = render(t, "{% if settings.a < settings.b %}Y{% else %}N{% endif %}",
		Options{Settings: map[string]interface{}{"a": "apple", "b": "banana"}})
	if out != "Y" {
		t.Fatalf("string < string must work; got %q", out)
	}
	// == stays loose (numeric/string coercion) — NOT affected by #7
	out = render(t, "{% if settings.x == 5 %}Y{% endif %}", Options{Settings: map[string]interface{}{"x": "5"}})
	if out != "Y" {
		t.Fatalf("loose == should still coerce; got %q", out)
	}
}
