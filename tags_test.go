package vascula

import (
	"strings"
	"testing"
)

func lpRender(t *testing.T, tpl string, ctx map[string]interface{}) string {
	t.Helper()
	c, err := compileFixture(tpl)
	if err != nil {
		t.Fatalf("compile %q: %v", tpl, err)
	}
	allow := make([]string, 0, len(ctx))
	for k := range ctx {
		allow = append(allow, k)
	}
	out, err := c.Render(Options{Data: ctx, Allow: allow})
	if err != nil {
		t.Fatalf("render %q: %v", tpl, err)
	}
	return out
}

func lpRenderErr(t *testing.T, tpl string, ctx map[string]interface{}) error {
	t.Helper()
	c, err := compileFixture(tpl)
	if err != nil {
		return err
	}
	allow := make([]string, 0, len(ctx))
	for k := range ctx {
		allow = append(allow, k)
	}
	_, err = c.Render(Options{Data: ctx, Allow: allow})
	return err
}

// ---- tags ----

func TestCaseWhen(t *testing.T) {
	tpl := `{% case x %}{% when 'a' %}A{% when 'b', 'c' %}BC{% when 'd' or 'e' %}DE{% else %}other{% endcase %}`
	for in, want := range map[string]string{"a": "A", "b": "BC", "c": "BC", "d": "DE", "e": "DE", "z": "other"} {
		if got := lpRender(t, tpl, map[string]interface{}{"x": in}); got != want {
			t.Fatalf("case %q: want %q got %q", in, want, got)
		}
	}
	// numeric subjects compare numerically
	if got := lpRender(t, `{% case n %}{% when 2 %}two{% endcase %}`, map[string]interface{}{"n": float64(2)}); got != "two" {
		t.Fatalf("numeric when: got %q", got)
	}
	// no else + no match → empty
	if got := lpRender(t, `{% case x %}{% when 'q' %}Q{% endcase %}`, map[string]interface{}{"x": "z"}); got != "" {
		t.Fatalf("no-match case must be empty, got %q", got)
	}
}

func TestCaseRequiresWhen(t *testing.T) {
	if _, err := compileFixture(`{% case x %}{% else %}o{% endcase %}`); err == nil {
		t.Fatal("case without when must fail compile")
	}
}

func TestCapture(t *testing.T) {
	got := lpRender(t, `{% capture greet %}Hi {{ name }}!{% endcapture %}[{{ greet }}]`, map[string]interface{}{"name": "Ada"})
	if got != "[Hi Ada!]" {
		t.Fatalf("capture: got %q", got)
	}
	// reserved names refuse
	if err := lpRenderErr(t, `{% capture settings %}x{% endcapture %}`, nil); err == nil {
		t.Fatal("capture into reserved name must error")
	}
}

func TestBreakContinue(t *testing.T) {
	ctx := map[string]interface{}{"xs": []interface{}{1.0, 2.0, 3.0, 4.0}}
	if got := lpRender(t, `{% for x in xs %}{% if x > 2 %}{% break %}{% endif %}{{ x }}{% endfor %}`, ctx); got != "12" {
		t.Fatalf("break: got %q", got)
	}
	if got := lpRender(t, `{% for x in xs %}{% if x == 2 %}{% continue %}{% endif %}{{ x }}{% endfor %}`, ctx); got != "134" {
		t.Fatalf("continue: got %q", got)
	}
	// break only unwinds the INNER loop
	nested := `{% for x in xs %}{% for y in xs %}{% break %}{{ y }}{% endfor %}{{ x }}{% endfor %}`
	if got := lpRender(t, nested, ctx); got != "1234" {
		t.Fatalf("nested break: got %q", got)
	}
	// outside a loop → error
	if err := lpRenderErr(t, `{% break %}`, nil); err == nil {
		t.Fatal("bare break must error")
	}
}

func TestCycle(t *testing.T) {
	ctx := map[string]interface{}{"xs": []interface{}{1.0, 2.0, 3.0}}
	if got := lpRender(t, `{% for x in xs %}{% cycle 'odd', 'even' %} {% endfor %}`, ctx); got != "odd even odd " {
		t.Fatalf("cycle: got %q", got)
	}
	// two cycle tags rotate independently
	got := lpRender(t, `{% for x in xs %}{% cycle 'a', 'b' %}{% cycle '1', '2' %}{% endfor %}`, ctx)
	if got != "a1b2a1" {
		t.Fatalf("independent cycles: got %q", got)
	}
}

// ---- filters ----

func TestArrayFilters(t *testing.T) {
	ctx := map[string]interface{}{
		"csv": "a,b,c",
		"items": []interface{}{
			map[string]interface{}{"name": "b", "on": true, "n": 2.0},
			map[string]interface{}{"name": "a", "on": false, "n": 1.0},
			map[string]interface{}{"name": "c", "on": true, "n": 3.0},
		},
		"dupes": []interface{}{"x", "y", "x"},
		"holes": []interface{}{"x", nil, "y"},
	}
	cases := map[string]string{
		`{{ csv | split: "," | size }}`:                       "3",
		`{{ csv | split: "," | first }}`:                      "a",
		`{{ csv | split: "," | last }}`:                       "c",
		`{{ csv | split: "" | size }}`:                        "5",
		`{{ items | map: "name" | join: "" }}`:                "bac",
		`{{ items | where: "on" | size }}`:                    "2",
		`{{ items | where: "name", "a" | size }}`:             "1",
		`{{ items | sort: "name" | map: "name" | join: "" }}`: "abc",
		`{{ items | sort: "n" | first | fetchless }}`:         "", // placeholder replaced below
		`{{ dupes | uniq | join: "" }}`:                       "xy",
		`{{ dupes | concat: holes | size }}`:                  "6",
		`{{ holes | compact | size }}`:                        "2",
		`{{ dupes | reverse | join: "" }}`:                    "xyx",
		`{{ items | sum: "n" }}`:                              "6",
		`{{ csv | split: "," | slice: 1, 2 | join: "" }}`:     "bc",
		`{{ csv | split: "," | slice: -1 | join: "" }}`:       "c",
	}
	delete(cases, `{{ items | sort: "n" | first | fetchless }}`)
	for tpl, want := range cases {
		if got := lpRender(t, tpl, ctx); got != want {
			t.Fatalf("%s: want %q got %q", tpl, want, got)
		}
	}
}

func TestStringAndNumberFilters(t *testing.T) {
	cases := map[string]string{
		`{{ "one two three four" | truncatewords: 2 }}`: "one two...",
		`{{ "one two" | truncatewords: 5 }}`:            "one two",
		`{{ "aXbXc" | replace_first: "X", "-" }}`:       "a-bXc",
		`{{ "aXbXc" | remove_first: "X" }}`:             "abXc",
		`{{ "a&amp;b" | escape_once }}`:                 "a&amp;b",
		`{{ "hello world" | url_encode }}`:              "hello+world",
		`{{ "hello+world" | url_decode }}`:              "hello world",
		`{{ 4 | at_least: 5 }}`:                         "5",
		`{{ 4 | at_most: 3 }}`:                          "3",
		`{{ "hello" | slice: 1, 3 }}`:                   "ell",
		`{{ "hello" | slice: -2, 2 }}`:                  "lo",
	}
	for tpl, want := range cases {
		if got := lpRender(t, tpl, nil); got != want {
			t.Fatalf("%s: want %q got %q", tpl, want, got)
		}
	}
	// strip_newlines (built outside the map: literal newlines in template strings)
	got := lpRender(t, "{{ s | strip_newlines }}", map[string]interface{}{"s": "a\nb\r\nc"})
	if got != "abc" {
		t.Fatalf("strip_newlines: got %q", got)
	}
}

func TestDateFilter(t *testing.T) {
	ctx := map[string]interface{}{"d": "2026-07-10T14:30:05Z", "day": "2026-07-10"}
	cases := map[string]string{
		`{{ d | date: "%Y-%m-%d" }}`:   "2026-07-10",
		`{{ d | date: "%B %e, %Y" }}`:  "July 10, 2026",
		`{{ d | date: "%H:%M" }}`:      "14:30",
		`{{ d | date: "%a %b %d" }}`:   "Fri Jul 10",
		`{{ day | date: "%d/%m/%y" }}`: "10/07/26",
		`{{ d | date: "100%% %Y" }}`:   "100% 2026",
	}
	for tpl, want := range cases {
		if got := lpRender(t, tpl, ctx); got != want {
			t.Fatalf("%s: want %q got %q", tpl, want, got)
		}
	}
	// unparseable input ERRORS (no silent wrong dates)
	if err := lpRenderErr(t, `{{ x | date: "%Y" }}`, map[string]interface{}{"x": "not-a-date"}); err == nil || !strings.Contains(err.Error(), "date") {
		t.Fatalf("bad date input must error, got %v", err)
	}
	// missing format errors
	if err := lpRenderErr(t, `{{ d | date }}`, ctx); err == nil {
		t.Fatal("date without format must error")
	}
}
