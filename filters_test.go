package vascula

import (
	"fmt"
	"strings"
	"testing"
)

// probeFilter echoes its positional + named args so a test can assert exactly what
// the evaluator delivered through FilterArgs (the image_url: width: 200 shape).
func probeFilter(_ interface{}, a FilterArgs) (interface{}, error) {
	out := fmt.Sprintf("pos=%v", a.Pos)
	if w := a.NamedArg("width"); w != nil {
		out += fmt.Sprintf(" width=%v", w)
	}
	if a.HasNamed("crop") {
		out += fmt.Sprintf(" crop=%v", a.NamedArg("crop"))
	}
	return out, nil
}

func compileErr(t *testing.T, src string) error {
	t.Helper()
	_, err := compileFixture(src)
	return err
}

// TestNamedFilterArgs proves the image_url: width: 200 call shape parses and the
// named arg reaches the filter via FilterArgs.NamedArg.
func TestNamedFilterArgs(t *testing.T) {
	opts := Options{Filters: map[string]FilterFunc{"probe": probeFilter}}

	// named only
	if got := render(t, `{{ "x" | probe: width: 200 }}`, opts); !strings.Contains(got, "width=200") {
		t.Fatalf("named arg not delivered: %q", got)
	}
	// named value as an expression, not just a literal
	o2 := Options{
		Filters:  map[string]FilterFunc{"probe": probeFilter},
		Settings: map[string]interface{}{"w": 300.0},
	}
	if got := render(t, `{{ "x" | probe: width: settings.w }}`, o2); !strings.Contains(got, "width=300") {
		t.Fatalf("expression-valued named arg not delivered: %q", got)
	}
	// mixed: positional precedes named
	if got := render(t, `{{ "x" | probe: 1, width: 200, crop: "center" }}`, opts); !strings.Contains(got, "pos=[1]") || !strings.Contains(got, "width=200") || !strings.Contains(got, "crop=center") {
		t.Fatalf("mixed positional+named not delivered: %q", got)
	}
}

// TestPositionalFiltersUnchanged proves the existing positional path is untouched
// by the FilterArgs refactor (backward compatibility).
func TestPositionalFiltersUnchanged(t *testing.T) {
	if got := render(t, `{{ "hello" | truncate: 4, ".." }}`, Options{}); got != "he.." {
		t.Fatalf("positional truncate broke: %q", got)
	}
	if got := render(t, `{{ "a" | append: "b" | upcase }}`, Options{}); got != "AB" {
		t.Fatalf("positional append/upcase broke: %q", got)
	}
}

// TestDefaultAllowFalse proves a builtin (default) reads a named arg: with
// allow_false:true an explicit false is kept rather than replaced by the fallback
// rather than replaced by the fallback.
func TestDefaultAllowFalse(t *testing.T) {
	o := Options{Settings: map[string]interface{}{"flag": false}}
	// without allow_false, false falls through to the fallback
	if got := render(t, `{{ settings.flag | default: 'F' }}`, o); got != "F" {
		t.Fatalf("default without allow_false should use fallback for false: %q", got)
	}
	// with allow_false, false is preserved
	if got := render(t, `{{ settings.flag | default: 'F', allow_false: true }}`, o); got != "false" {
		t.Fatalf("default with allow_false should keep false: %q", got)
	}
}

// TestDuplicateNamedArgRejected proves a duplicate named arg is a parse error.
func TestDuplicateNamedArgRejected(t *testing.T) {
	err := compileErr(t, `{{ "x" | probe: width: 1, width: 2 }}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate named argument") {
		t.Fatalf("duplicate named arg should fail to parse, got %v", err)
	}
}

// TestPositionalAfterNamedRejected proves positional-after-named is a parse error
// (positional args must precede named ones).
func TestPositionalAfterNamedRejected(t *testing.T) {
	err := compileErr(t, `{{ "x" | probe: width: 1, 5 }}`)
	if err == nil || !strings.Contains(err.Error(), "positional argument after a named argument") {
		t.Fatalf("positional-after-named should fail to parse, got %v", err)
	}
}

func TestBuiltinsRefuseUnknownNamedArguments(t *testing.T) {
	child, _ := Compile(`{{ amount }} {{ currency }}`)
	tmpl, _ := Compile(`{% render "price", amount: 1250 | divided_by: 100, currency: "EUR" %}`)
	_, err := tmpl.Render(Options{Resolve: func(string) (Child, error) { return Child{Template: child}, nil }})
	if err == nil || !strings.Contains(err.Error(), `filter "divided_by" does not take a named argument "currency"`) {
		t.Fatalf("got %v", err)
	}
	if out := render(t, `{{ false | default: true, allow_false: true }}`, Options{}); out != "false" {
		t.Fatalf("default allow_false: %q", out)
	}
	host, _ := Compile(`{{ 1 | custom: width: 2 }}`)
	out, err := host.Render(Options{Filters: map[string]FilterFunc{"custom": func(in interface{}, a FilterArgs) (interface{}, error) { return a.NamedArg("width"), nil }}})
	if err != nil || out != "2" {
		t.Fatalf("host filters keep named arguments: %q %v", out, err)
	}
}
