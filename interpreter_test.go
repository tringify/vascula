package vascula

import (
	"strings"
	"testing"
)

func render(t *testing.T, src string, opts Options) string {
	t.Helper()
	tmpl, err := compileFixture(src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	out, err := tmpl.Render(opts)
	if err != nil {
		t.Fatalf("render %q: %v", src, err)
	}
	return out
}

func TestRendering(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"text", "<h1>hi</h1>", Options{}, "<h1>hi</h1>"},
		{"setting", "{{ settings.heading }}", Options{Settings: map[string]interface{}{"heading": "Welcome"}}, "Welcome"},
		{"autoescape", "{{ settings.x }}", Options{Settings: map[string]interface{}{"x": "<b>&"}}, "&lt;b&gt;&amp;"},
		{"SafeHTML unescaped", "{{ settings.x }}", Options{Settings: map[string]interface{}{"x": SafeHTML("<b>")}}, "<b>"},
		{"upcase", `{{ "hello" | upcase }}`, Options{}, "HELLO"},
		{"default", "{{ settings.missing | default: 'fallback' }}", Options{Settings: map[string]interface{}{}}, "fallback"},
		{"append", `{{ "a" | append: "b" | upcase }}`, Options{}, "AB"},
		{"if true", "{% if settings.on %}Y{% else %}N{% endif %}", Options{Settings: map[string]interface{}{"on": true}}, "Y"},
		{"if false", "{% if settings.on %}Y{% else %}N{% endif %}", Options{Settings: map[string]interface{}{"on": false}}, "N"},
		{"elsif", "{% if settings.n == 1 %}one{% elsif settings.n == 2 %}two{% else %}many{% endif %}", Options{Settings: map[string]interface{}{"n": 2.0}}, "two"},
		{"unless", "{% unless settings.hide %}shown{% endunless %}", Options{Settings: map[string]interface{}{"hide": false}}, "shown"},
		{"comparison", "{% if settings.n >= 3 %}big{% endif %}", Options{Settings: map[string]interface{}{"n": 5.0}}, "big"},
		{"contains", `{% if settings.s contains "ll" %}yes{% endif %}`, Options{Settings: map[string]interface{}{"s": "hello"}}, "yes"},
		{"and/or", "{% if settings.a and settings.b %}both{% endif %}", Options{Settings: map[string]interface{}{"a": true, "b": true}}, "both"},
		{
			"for", "{% for item in settings.items %}{{ forloop.index }}:{{ item }} {% endfor %}",
			Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b", "c"}}}, "1:a 2:b 3:c ",
		},
		{
			"for last", "{% for x in settings.items %}{{ x }}{% unless forloop.last %},{% endunless %}{% endfor %}",
			Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b", "c"}}}, "a,b,c",
		},
		{
			"for empty else", "{% for x in settings.items %}{{ x }}{% else %}none{% endfor %}",
			Options{Settings: map[string]interface{}{"items": []interface{}{}}}, "none",
		},
		{
			"for limit offset", "{% for x in settings.items offset: 1 limit: 2 %}{{ x }}{% endfor %}",
			Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b", "c", "d"}}}, "bc",
		},
		{
			"for reversed", "{% for x in settings.items reversed %}{{ x }}{% endfor %}",
			Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b", "c"}}}, "cba",
		},
		{"assign", "{% assign g = 'hi' | upcase %}{{ g }}", Options{}, "HI"},
		{"nested path", "{{ settings.obj.name }}", Options{Settings: map[string]interface{}{"obj": map[string]interface{}{"name": "x"}}}, "x"},
		{"nested type path", "{% for b in settings.blocks %}{% if b.type == 'auth_form' %}yes{% endif %}{% endfor %}", Options{Settings: map[string]interface{}{"blocks": []interface{}{map[string]interface{}{"type": "auth_form"}}}}, "yes"},
		{"loop assign updates predeclared variable", "{% assign render_form = false %}{% for b in settings.blocks %}{% if b.type == 'auth_form' %}{% assign render_form = true %}{% endif %}{% endfor %}{% if render_form %}yes{% endif %}", Options{Settings: map[string]interface{}{"blocks": []interface{}{map[string]interface{}{"type": "auth_form"}}}}, "yes"},
		{"missing path empty", "[{{ settings.obj.nope }}]", Options{Settings: map[string]interface{}{"obj": map[string]interface{}{}}}, "[]"},
		{"index", "{{ settings.items[1] }}", Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b"}}}, "b"},
		{"size", "{{ settings.items.size }}", Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b"}}}, "2"},
		{"comment", "a{% comment %}hidden{% endcomment %}b", Options{}, "ab"},
		{"whitespace control", "{% assign x = 'v' -%}   {{ x }}", Options{}, "v"},
		{
			"declared data", "{{ product.title }}",
			Options{Data: map[string]interface{}{"product": map[string]interface{}{"title": "Shirt"}}, Allow: []string{"product"}}, "Shirt",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := render(t, c.src, c.opts); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestSandbox: a template can read only the data roots it is allowed.
func TestSandbox(t *testing.T) {
	tmpl, err := compileFixture("{{ cart.item_count }}")
	if err != nil {
		t.Fatal(err)
	}
	// cart is NOT declared → render must fail closed.
	_, err = tmpl.Render(Options{
		Data:  map[string]interface{}{"cart": map[string]interface{}{"item_count": 3.0}},
		Allow: []string{"product"}, // declared product, not cart
	})
	if err == nil || !strings.Contains(err.Error(), "undeclared name") {
		t.Fatalf("expected undeclared-name error, got %v", err)
	}

	// Declaring it makes it reachable.
	out, err := tmpl.Render(Options{
		Data:  map[string]interface{}{"cart": map[string]interface{}{"item_count": 3.0}},
		Allow: []string{"cart"},
	})
	if err != nil || out != "3" {
		t.Fatalf("declared cart: got %q err %v", out, err)
	}
}

func TestRenderBudget(t *testing.T) {
	// A loop that would emit far more than the budget allows must abort, not hang.
	tmpl, err := compileFixture("{% for x in settings.items %}{{ x }}{% endfor %}")
	if err != nil {
		t.Fatal(err)
	}
	big := make([]interface{}, 100000)
	for i := range big {
		big[i] = "xxxxxxxxxx"
	}
	_, err = tmpl.Render(Options{
		Settings: map[string]interface{}{"items": big},
		MaxSteps: 1000,
	})
	if err == nil || !strings.Contains(err.Error(), "render budget exceeded") {
		t.Fatalf("expected budget error, got %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"{% if x %}no end",
		"{{ unclosed",
		"{% for x of items %}{% endfor %}", // 'of' not 'in'
		"{% bogus %}",
	}
	for _, src := range bad {
		if _, err := compileFixture(src); err == nil {
			t.Fatalf("expected compile error for %q", src)
		}
	}
}
