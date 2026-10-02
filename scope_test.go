package vascula

import "testing"

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
