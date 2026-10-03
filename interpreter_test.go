package vascula

import (
	"fmt"
	"strings"
	"sync"
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

// Assign and capture bind in the template's top-level frame unless a
// nearer binding exists, so values set inside branches and loops survive them.
func TestAssignmentsOutliveTheirBlock(t *testing.T) {
	opts := Options{Settings: map[string]interface{}{"on": true, "items": []interface{}{"a", "b", "c"}}}
	// Loop variables stay local to their loop: reading one afterwards is an
	// unknown name, as it always was.
	tmpl, _ := Compile(`{% for i in settings.items %}{% endfor %}{{ i }}`)
	if _, err := tmpl.Render(opts); err == nil {
		t.Error("a loop variable outlived its loop")
	}
	for src, want := range map[string]string{
		`{% if settings.on %}{% assign label = "yes" %}{% endif %}{{ label }}`:                                                              "yes",
		`{% for i in settings.items %}{% assign last = i %}{% endfor %}{{ last }}`:                                                          "c",
		`{% for i in settings.items %}{% if forloop.first %}{% capture first %}<{{ i }}>{% endcapture %}{% endif %}{% endfor %}{{ first }}`: "&lt;a&gt;",
		`{% case "x" %}{% when "x" %}{% assign hit = 1 %}{% endcase %}{{ hit }}`:                                                            "1",
		`{% assign total = 0 %}{% for i in settings.items %}{% assign total = total | plus: 1 %}{% endfor %}{{ total }}`:                    "3",
		// An assignment to the loop variable changes this iteration only.
		`{% for i in settings.items %}{% assign i = "z" %}{{ i }}{% endfor %}`: "zzz",
	} {
		if got := renderWith(t, src, opts); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestChildRendersKeepTheirOwnVariables(t *testing.T) {
	child, _ := Compile(`{% assign secret = "child" %}{{ secret }}`)
	resolve := Options{Resolve: func(string) (Child, error) { return Child{Template: child}, nil }}
	if out := renderWith(t, `{% render "c" %}`, resolve); out != "child" {
		t.Errorf("got %q", out)
	}
	tmpl, _ := Compile(`{% render "c" %}{{ secret }}`)
	if _, err := tmpl.Render(resolve); err == nil {
		t.Error("a child's assignment reached the caller")
	}
}

func TestParentloop(t *testing.T) {
	opts := Options{Settings: map[string]interface{}{"rows": []interface{}{"r1", "r2"}, "cols": []interface{}{"c1", "c2"}}}
	got := renderWith(t, `{% for r in settings.rows %}{% for c in settings.cols %}{{ forloop.parentloop.index }}.{{ forloop.index }} {% endfor %}{% endfor %}`, opts)
	if got != "1.1 1.2 2.1 2.2 " {
		t.Errorf("got %q", got)
	}
	if got := renderWith(t, `{% for r in settings.rows %}{{ forloop.parentloop }}{% endfor %}`, opts); got != "" {
		t.Errorf("outermost parentloop must be nil, got %q", got)
	}
}

func TestRanges(t *testing.T) {
	opts := Options{Settings: map[string]interface{}{"n": float64(3)}}
	for src, want := range map[string]string{
		`{% for i in (1..5) %}{{ i }}{% endfor %}`:               "12345",
		`{% for i in (1..settings.n) %}{{ i }}{% endfor %}`:      "123",
		`{% for i in (3..1) %}{{ i }}{% else %}none{% endfor %}`: "none",
		`{% assign r = (-2..2) %}{{ r | join: "," }}`:            "-2,-1,0,1,2",
		`{% for i in (1..3) reversed %}{{ i }}{% endfor %}`:      "321",
		`{{ 1.5 | plus: 1 }}`:                                    "2.5",
	} {
		if got := renderWith(t, src, opts); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
	tmpl, _ := Compile(`{% for i in (1..100000) %}{% endfor %}`)
	if _, err := tmpl.Render(Options{}); err == nil {
		t.Error("an oversized range was allowed")
	}
	for _, bad := range []string{`{% for i in (1..) %}{% endfor %}`, `{% for i in ("a"..2) %}{% endfor %}`, `{% for i in (1 2) %}{% endfor %}`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%s compiled", bad)
		}
	}
}

func TestIncludeIsNotATag(t *testing.T) {
	if _, err := Compile(`{% include "footer" %}`); err == nil {
		t.Error("include compiled; render is the only way to compose templates")
	}
}

func TestForloopAsData(t *testing.T) {
	opts := Options{Settings: map[string]interface{}{"items": []interface{}{"a", "b"}}}
	got := renderWith(t, `{% for i in settings.items %}{{ forloop | json }};{% endfor %}`, opts)
	want := `{"first":true,"index":1,"index0":0,"last":false,"length":2,"parentloop":null,"rindex":2,"rindex0":1};{"first":false,"index":2,"index0":1,"last":true,"length":2,"parentloop":null,"rindex":1,"rindex0":0};`
	if got != want {
		t.Errorf("got %s", got)
	}
	if got := renderWith(t, `{% for i in settings.items %}{% if forloop.last %}{{ forloop.rindex0 }}{% endif %}{% endfor %}`, opts); got != "0" {
		t.Errorf("got %q", got)
	}
}

func TestConcurrentRendersKeepRequestDataIsolated(t *testing.T) {
	template, err := CompileWithOptions(`{% form "save" %}{{ account.name }}|{{ settings.label }}|{{ site.settings.title }}{% endform %}`, CompileOptions{
		Forms: map[string]FormDefinition{"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := fmt.Sprintf("request-%d", i)
			name := fmt.Sprintf("Customer %d", i)
			html, err := template.Render(Options{
				Data:  map[string]interface{}{"token": token, "account": map[string]interface{}{"name": name}},
				Allow: []string{"account"}, Settings: map[string]interface{}{"label": token},
				Globals: siteGlobals(map[string]interface{}{"title": name}),
			})
			if err != nil {
				t.Error(err)
				return
			}
			want := fmt.Sprintf(`<form method="post" action="/save"><input type="hidden" name="csrf" value="%s">%s|%s|%s</form>`, token, name, token, name)
			if html != want {
				t.Errorf("request %d: got %q, want %q", i, html, want)
			}
		}(i)
	}
	wg.Wait()
}

func TestFormTokenDoesNotGrantTemplateReadAccess(t *testing.T) {
	template, err := CompileWithOptions(`{% form "save" %}{{ token }}{% endform %}`, CompileOptions{
		Forms: map[string]FormDefinition{"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := template.Render(Options{Data: map[string]interface{}{"token": "not-for-template-expressions"}})
	if err == nil || !strings.Contains(err.Error(), "undeclared name") || html != "" {
		t.Fatalf("token capability leaked: %q, %v", html, err)
	}
}
