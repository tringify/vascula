package vascula

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestInspectionVisitsEveryControlBody(t *testing.T) {
	for _, wrapper := range []string{
		`{% capture text %}%s{% endcapture %}`,
		`{% case settings.kind %}{% when "a" %}%s{% else %}other{% endcase %}`,
		`{% case settings.kind %}{% when "a" %}first{% else %}%s{% endcase %}`,
		`{% capture text %}{% if settings.ok %}{% case settings.kind %}{% when "a" %}%s{% endcase %}{% endif %}{% endcapture %}`,
	} {
		t.Run(wrapper, func(t *testing.T) {
			tmpl, err := CompileWithOptions(strings.Replace(wrapper, "%s", `{% render "child" %}{% form "save" %}{% blocks %}{% endform %}`, 1), CompileOptions{Forms: map[string]FormDefinition{
				"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf"},
			}, Tags: map[string]TagDefinition{"blocks": {Argument: TagOptionalArgument}}})
			if err != nil {
				t.Fatal(err)
			}
			if got := tmpl.RenderTargets(); len(got) != 1 || got[0] != "child" {
				t.Errorf("targets = %v", got)
			}
			if !tmpl.UsesForm() {
				t.Error("nested form was missed")
			}
			if !tmpl.UsesTag("blocks") {
				t.Error("nested host tag was missed")
			}
		})
	}
}

func TestChildRenderingBoundary(t *testing.T) {
	t.Run("missing template returns error", func(t *testing.T) {
		defer func() {
			if p := recover(); p != nil {
				t.Errorf("resolver result panicked: %v", p)
			}
		}()
		tmpl, _ := Compile(`{% render "missing" %}`)
		out, err := tmpl.Render(Options{Resolve: func(string) (Child, error) { return Child{}, nil }})
		if err == nil || out != "" {
			t.Errorf("got %q, %v", out, err)
		}
	})
	t.Run("host tags reach the child", func(t *testing.T) {
		child, _ := CompileWithOptions(`{% note %}`, CompileOptions{Tags: map[string]TagDefinition{"note": {}}})
		parent, _ := Compile(`{% render "child" %}`)
		out, err := parent.Render(Options{
			Resolve: func(string) (Child, error) { return Child{Template: child}, nil },
			Tags:    map[string]TagFunc{"note": func(TagCall) (SafeHTML, error) { return "<p>Note</p>", nil }},
		})
		if err != nil || out != "<p>Note</p>" {
			t.Fatalf("got %q, %v", out, err)
		}
	})
	t.Run("child error uses child source", func(t *testing.T) {
		child, _ := Compile("line one\nline two\n{{ forbidden }}")
		parent, _ := Compile(`{% render "child" %}`)
		_, err := parent.Render(Options{Resolve: func(string) (Child, error) { return Child{Template: child}, nil }})
		if err == nil || !strings.Contains(err.Error(), "line 3, col 4") || !strings.Contains(err.Error(), `"child"`) {
			t.Fatalf("wrong source: %v", err)
		}
	})
	t.Run("child cannot break caller loop", func(t *testing.T) {
		child, _ := Compile(`{% break %}`)
		parent, _ := Compile(`{% for item in settings.items %}{% render "child" %}{% endfor %}`)
		out, err := parent.Render(Options{Settings: map[string]interface{}{"items": []interface{}{1, 2}}, Resolve: func(string) (Child, error) { return Child{Template: child}, nil }})
		if err == nil || out != "" {
			t.Fatalf("child escaped its scope: %q, %v", out, err)
		}
	})
}

func TestHostFilterErrorsRemainInspectable(t *testing.T) {
	sentinel := errors.New("service unavailable")
	tmpl, _ := Compile(`{{ "input" | host_filter }}`)
	_, err := tmpl.Render(Options{Filters: map[string]FilterFunc{"host_filter": func(interface{}, FilterArgs) (interface{}, error) { return nil, sentinel }}})
	if !errors.Is(err, sentinel) {
		t.Fatalf("cause lost: %v", err)
	}
}

func TestInvalidHostValuesDoNotPanic(t *testing.T) {
	for _, val := range []interface{}{map[int]string{1: "one"}, map[uint]interface{}{1: "one"}} {
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("map access panicked: %v", p)
				}
			}()
			tmpl, _ := Compile(`{{ settings.value.name }}`)
			_, _ = tmpl.Render(Options{Settings: map[string]interface{}{"value": val}})
		})
	}
	t.Run("nil filter", func(t *testing.T) {
		defer func() {
			if p := recover(); p != nil {
				t.Errorf("nil filter panicked: %v", p)
			}
		}()
		tmpl, _ := Compile(`{{ "x" | custom }}`)
		if _, err := tmpl.Render(Options{Filters: map[string]FilterFunc{"custom": nil}}); err == nil {
			t.Fatal("nil filter accepted")
		}
	})
}

func TestCompileRejectsAmbiguousOrReservedBindings(t *testing.T) {
	for _, src := range []string{
		`{% assign settings = "oops" %}`,
		`{% for site in settings.items %}{{ site }}{% endfor %}`,
		`{% render "child", value: 1, value: 2 %}`,
		`{% form "save", class: "one", class: "two" %}{% endform %}`,
		`{% form "save", action: "/other" %}{% endform %}`,
		`{% form "save", METHOD: "get" %}{% endform %}`,
		`{% form "save", data-action: "other" %}{% endform %}`,
	} {
		if _, err := CompileWithOptions(src, CompileOptions{Reserved: []string{"site"}, Forms: map[string]FormDefinition{"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf", ActionAttribute: "data-action"}}}); err == nil {
			t.Errorf("accepted %s", src)
		}
	}
}

func TestNonFiniteNumbersReturnErrors(t *testing.T) {
	for _, source := range []string{`{{ settings.value }}`, `{{ settings.value | plus: 1 }}`, `{% if settings.value > 0 %}yes{% endif %}`} {
		for _, number := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			tmpl, err := Compile(source)
			if err != nil {
				t.Fatal(err)
			}
			if out, err := tmpl.Render(Options{Settings: map[string]interface{}{"value": number}}); err == nil || out != "" {
				t.Errorf("non-finite result %q: %v", out, err)
			}
		}
	}
	tmpl, _ := Compile(`{{ settings.value | times: 2 }}`)
	if _, err := tmpl.Render(Options{Settings: map[string]interface{}{"value": math.MaxFloat64}}); err == nil {
		t.Fatal("numeric overflow must fail")
	}
}
