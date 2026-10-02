package vascula

import (
	"strings"
	"testing"
)

// coverage_test.go is the comprehensive behavior lock for the interpreter: every filter, every access
// rule, escaping, the render budget, render scope, whitespace control, forloop vars, operators, and the
// value semantics. Each expectation is the ACTUAL engine behavior — these pin it so it cannot drift.

// --- Output & auto-escaping -------------------------------------------------

func TestOutputAndEscaping(t *testing.T) {
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"literal markup", "<p>hi & bye</p>", "<p>hi & bye</p>", Options{}}, // text nodes are NOT escaped
		{"value escaped", "{{ settings.x }}", "&lt;b&gt;&amp;&#34;", Options{Settings: m("x", `<b>&"`)}},
		{"SafeHTML unescaped", "{{ settings.x }}", "<b>", Options{Settings: m("x", SafeHTML("<b>"))}},
		{"json safe", "{{ settings.x | json }}", `{"a":1}`, Options{Settings: m("x", map[string]interface{}{"a": 1})}},
		{"number prints int", "{{ settings.x }}", "5", Options{Settings: m("x", 5.0)}},
		{"number prints decimal", "{{ settings.x }}", "5.5", Options{Settings: m("x", 5.5)}},
		{"bool prints", "{{ settings.x }}", "true", Options{Settings: m("x", true)}},
		{"nil prints empty", "[{{ settings.x }}]", "[]", Options{Settings: m("x", nil)}},
	}
	runCases(t, cases)
}

// --- Truthiness (only nil/false/empty are falsy) ----------------------------

func TestTruthiness(t *testing.T) {
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"nil falsy", "{% if settings.x %}Y{% else %}N{% endif %}", "N", Options{Settings: m("x", nil)}},
		{"false falsy", "{% if settings.x %}Y{% else %}N{% endif %}", "N", Options{Settings: m("x", false)}},
		{"empty literal falsy", "{% if empty %}Y{% else %}N{% endif %}", "N", Options{}},
		{"zero TRUTHY", "{% if settings.x %}Y{% else %}N{% endif %}", "Y", Options{Settings: m("x", 0.0)}},
		{"empty string TRUTHY", "{% if settings.x %}Y{% else %}N{% endif %}", "Y", Options{Settings: m("x", "")}},
		{"empty array TRUTHY", "{% if settings.x %}Y{% else %}N{% endif %}", "Y", Options{Settings: m("x", []interface{}{})}},
		{"string truthy", "{% if settings.x %}Y{% endif %}", "Y", Options{Settings: m("x", "hi")}},
	}
	runCases(t, cases)
}

// --- Operators --------------------------------------------------------------

func TestOperators(t *testing.T) {
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"eq num", "{% if settings.x == 5 %}Y{% endif %}", "Y", Options{Settings: m("x", 5.0)}},
		{"eq loose str-num", "{% if settings.x == 5 %}Y{% endif %}", "Y", Options{Settings: m("x", "5")}},
		{"neq", "{% if settings.x != 5 %}Y{% endif %}", "Y", Options{Settings: m("x", 6.0)}},
		{"gt", "{% if settings.x > 3 %}Y{% endif %}", "Y", Options{Settings: m("x", 5.0)}},
		{"gte equal", "{% if settings.x >= 5 %}Y{% endif %}", "Y", Options{Settings: m("x", 5.0)}},
		{"lt str", "{% if settings.a < settings.b %}Y{% endif %}", "Y", Options{Settings: mm("a", "apple", "b", "banana")}},
		{"contains substr", `{% if settings.s contains "ell" %}Y{% endif %}`, "Y", Options{Settings: m("s", "hello")}},
		{"contains member", `{% if settings.l contains "b" %}Y{% endif %}`, "Y", Options{Settings: m("l", []interface{}{"a", "b"})}},
		{"contains absent", `{% if settings.s contains "z" %}Y{% else %}N{% endif %}`, "N", Options{Settings: m("s", "hello")}},
		{"and both", "{% if settings.a and settings.b %}Y{% endif %}", "Y", Options{Settings: mm("a", true, "b", true)}},
		{"or short", "{% if settings.a or settings.b %}Y{% endif %}", "Y", Options{Settings: mm("a", true, "b", false)}},
		{"precedence and over or", "{% if false and false or true %}Y{% else %}N{% endif %}", "Y", Options{}},
		{"eq empty marker", `{% if settings.s == empty %}Y{% endif %}`, "Y", Options{Settings: m("s", "")}},
	}
	runCases(t, cases)
}

// --- Variable access / sandbox ----------------------------------------------

func TestVariableAccess(t *testing.T) {
	// declared namespace reads
	out := render(t, "{{ product.title }}", Options{Data: m("product", m("title", "Widget")), Allow: []string{"product"}})
	if out != "Widget" {
		t.Fatalf("declared data read; got %q", out)
	}
	// undeclared root namespace is a HARD error
	if _, err := renderErr("{{ widget.title }}", Options{Data: m("widget", m("title", "x"))}); err == nil || !strings.Contains(err.Error(), "undeclared name") {
		t.Fatalf("undeclared root must error; got %v", err)
	}
	// missing sub-field is nil (not an error)
	out = render(t, "[{{ product.missing.deep }}]", Options{Data: m("product", m("title", "x")), Allow: []string{"product"}})
	if out != "[]" {
		t.Fatalf("missing subfield = nil; got %q", out)
	}
	// settings always available without declaration
	out = render(t, "{{ settings.x }}", Options{Settings: m("x", "v")})
	if out != "v" {
		t.Fatalf("settings ungated; got %q", out)
	}
	// site.settings always available without declaration
	out = render(t, "{{ site.settings.brand }}", Options{Globals: siteGlobals(m("brand", "Acme"))})
	if out != "Acme" {
		t.Fatalf("site.settings ungated; got %q", out)
	}
	// bracket access: index, negative index, map key
	out = render(t, "{{ settings.l[0] }}-{{ settings.l[-1] }}-{{ settings.mp['k'] }}",
		Options{Settings: mm("l", []interface{}{"a", "b", "c"}, "mp", map[string]interface{}{"k": "v"})})
	if out != "a-c-v" {
		t.Fatalf("bracket access; got %q", out)
	}
	// pseudo props
	out = render(t, "{{ settings.l.size }}/{{ settings.l.first }}/{{ settings.l.last }}",
		Options{Settings: m("l", []interface{}{"a", "b", "c"})})
	if out != "3/a/c" {
		t.Fatalf("pseudo props; got %q", out)
	}
}

// --- for loop semantics -----------------------------------------------------

func TestForLoop(t *testing.T) {
	items := []interface{}{"a", "b", "c"}
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"basic", "{% for x in settings.l %}{{ x }}{% endfor %}", "abc", Options{Settings: m("l", items)}},
		{"index", "{% for x in settings.l %}{{ forloop.index }}{% endfor %}", "123", Options{Settings: m("l", items)}},
		{"index0", "{% for x in settings.l %}{{ forloop.index0 }}{% endfor %}", "012", Options{Settings: m("l", items)}},
		{"rindex", "{% for x in settings.l %}{{ forloop.rindex }}{% endfor %}", "321", Options{Settings: m("l", items)}},
		{"rindex0", "{% for x in settings.l %}{{ forloop.rindex0 }}{% endfor %}", "210", Options{Settings: m("l", items)}},
		{"first/last", "{% for x in settings.l %}{% if forloop.first %}F{% endif %}{% if forloop.last %}L{% endif %}{% endfor %}", "FL", Options{Settings: m("l", items)}},
		{"length", "{% for x in settings.l %}{{ forloop.length }}{% endfor %}", "333", Options{Settings: m("l", items)}},
		{"limit", "{% for x in settings.l limit: 2 %}{{ x }}{% endfor %}", "ab", Options{Settings: m("l", items)}},
		{"offset", "{% for x in settings.l offset: 1 %}{{ x }}{% endfor %}", "bc", Options{Settings: m("l", items)}},
		{"reversed", "{% for x in settings.l reversed %}{{ x }}{% endfor %}", "cba", Options{Settings: m("l", items)}},
		{"else empty", "{% for x in settings.l %}{{ x }}{% else %}none{% endfor %}", "none", Options{Settings: m("l", []interface{}{})}},
		{"nil collection else", "{% for x in settings.l %}{{ x }}{% else %}none{% endfor %}", "none", Options{Settings: m("l", nil)}},
		{"offset beyond length", "[{% for x in settings.l offset: 10 %}{{ x }}{% endfor %}]", "[]", Options{Settings: m("l", items)}},
	}
	runCases(t, cases)
}

// --- assign + scope ---------------------------------------------------------

func TestAssignAndScope(t *testing.T) {
	out := render(t, "{% assign x = 'a' %}{{ x }}{% assign x = 'b' %}{{ x }}", Options{})
	if out != "ab" {
		t.Fatalf("reassign; got %q", out)
	}
	// the loop var does NOT leak out of the loop: after the loop, `i` is neither a local nor a declared
	// data root, so reading it is an undeclared-access error (proves it didn't leak as a local).
	if _, err := renderErr("{% for i in settings.l %}{{ i }}{% endfor %}{{ i }}", Options{Settings: m("l", []interface{}{"x"})}); err == nil || !strings.Contains(err.Error(), "undeclared name") {
		t.Fatalf("loop var must not leak (reading it after = undeclared error); got %v", err)
	}
	// inside the loop the var IS available
	out = render(t, "{% for i in settings.l %}{{ i }}{% endfor %}", Options{Settings: m("l", []interface{}{"x", "y"})})
	if out != "xy" {
		t.Fatalf("loop var available inside; got %q", out)
	}
	// assign with a filter pipeline
	out = render(t, "{% assign y = settings.s | upcase %}{{ y }}", Options{Settings: m("s", "hi")})
	if out != "HI" {
		t.Fatalf("assign with filter; got %q", out)
	}
}

// --- Every built-in filter --------------------------------------------------

func TestAllFilters(t *testing.T) {
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"default fallback", "{{ settings.x | default: 'D' }}", "D", Options{Settings: m("x", nil)}},
		{"default keep", "{{ settings.x | default: 'D' }}", "v", Options{Settings: m("x", "v")}},
		{"default keep zero", "{{ settings.x | default: 'D' }}", "0", Options{Settings: m("x", 0.0)}},
		{"upcase", `{{ "hi" | upcase }}`, "HI", Options{}},
		{"downcase", `{{ "HI" | downcase }}`, "hi", Options{}},
		{"capitalize", `{{ "hELLO" | capitalize }}`, "Hello", Options{}},
		{"strip", `{{ "  x  " | strip }}`, "x", Options{}},
		{"lstrip", `[{{ "  x  " | lstrip }}]`, "[x  ]", Options{}},
		{"rstrip", `[{{ "  x  " | rstrip }}]`, "[  x]", Options{}},
		{"strip_html", `{{ "<b>x</b>" | strip_html }}`, "x", Options{}},
		{"append", `{{ "a" | append: "b" }}`, "ab", Options{}},
		{"prepend", `{{ "a" | prepend: "b" }}`, "ba", Options{}},
		{"replace", `{{ "a-b" | replace: "-", "_" }}`, "a_b", Options{}},
		{"remove", `{{ "a-b-c" | remove: "-" }}`, "abc", Options{}},
		{"truncate", `{{ "abcdef" | truncate: 5, "" }}`, "abcde", Options{}},
		{"truncate ellipsis", `{{ "abcdef" | truncate: 4 }}`, "a...", Options{}},
		{"truncate short", `{{ "ab" | truncate: 5 }}`, "ab", Options{}},
		{"size string", `{{ "abc" | size }}`, "3", Options{}},
		{"size array", "{{ settings.l | size }}", "2", Options{Settings: m("l", []interface{}{"a", "b"})}},
		{"join", "{{ settings.l | join: ',' }}", "a,b,c", Options{Settings: m("l", []interface{}{"a", "b", "c"})}},
		{"plus", "{{ settings.x | plus: 2 }}", "5", Options{Settings: m("x", 3.0)}},
		{"minus", "{{ settings.x | minus: 1 }}", "2", Options{Settings: m("x", 3.0)}},
		{"times", "{{ settings.x | times: 4 }}", "12", Options{Settings: m("x", 3.0)}},
		{"divided_by", "{{ settings.x | divided_by: 2 }}", "5", Options{Settings: m("x", 10.0)}},
		{"modulo", "{{ settings.x | modulo: 3 }}", "1", Options{Settings: m("x", 10.0)}},
		{"abs", "{{ settings.x | abs }}", "5", Options{Settings: m("x", -5.0)}},
		{"round", "{{ settings.x | round }}", "4", Options{Settings: m("x", 3.6)}},
		{"round precision", "{{ settings.x | round: 2 }}", "3.14", Options{Settings: m("x", 3.14159)}},
		{"ceil", "{{ settings.x | ceil }}", "4", Options{Settings: m("x", 3.1)}},
		{"floor", "{{ settings.x | floor }}", "3", Options{Settings: m("x", 3.9)}},
		{"newline_to_br", "{{ settings.x | newline_to_br }}", "a<br>b", Options{Settings: m("x", "a\nb")}},
		{"escape", `{{ "<b>" | escape }}`, "&lt;b&gt;", Options{}},
		{"chained", `{{ "  hi  " | strip | upcase | append: "!" }}`, "HI!", Options{}},
	}
	runCases(t, cases)
}

// --- Host filters override / extend builtins --------------------------------

func TestHostFilters(t *testing.T) {
	hostFilters := map[string]FilterFunc{
		"money": func(in interface{}, _ FilterArgs) (interface{}, error) { return "$" + toString(in), nil },
		// override a builtin
		"upcase": func(in interface{}, _ FilterArgs) (interface{}, error) { return "X", nil },
	}
	out := render(t, "{{ settings.p | money }}", Options{Settings: m("p", "9.00"), Filters: hostFilters})
	if out != "$9.00" {
		t.Fatalf("host filter; got %q", out)
	}
	out = render(t, `{{ "hi" | upcase }}`, Options{Filters: hostFilters})
	if out != "X" {
		t.Fatalf("host filter should override builtin; got %q", out)
	}
}

// --- Unknown filter errors --------------------------------------------------

func TestUnknownFilterErrors(t *testing.T) {
	if _, err := renderErr("{{ settings.x | nope }}", Options{Settings: m("x", "y")}); err == nil || !strings.Contains(err.Error(), "unknown filter") {
		t.Fatalf("unknown filter must error; got %v", err)
	}
}

// --- Whitespace control -----------------------------------------------------

func TestWhitespaceControl(t *testing.T) {
	cases := []struct {
		name, src, want string
		opts            Options
	}{
		{"left trim", "a   {{- settings.x }}", "ax", Options{Settings: m("x", "x")}},
		{"right trim", "{{ settings.x -}}   b", "xb", Options{Settings: m("x", "x")}},
		{"both", "a  {{- settings.x -}}  b", "axb", Options{Settings: m("x", "x")}},
		{"tag trim", "a  {%- assign y = 'z' -%}  b", "ab", Options{}},
	}
	runCases(t, cases)
}

// --- comment / raw block ----------------------------------------------------

func TestCommentBlock(t *testing.T) {
	if got := render(t, "a{% comment %}hidden{{ settings.x }}{% endcomment %}b", Options{Settings: m("x", "X")}); got != "ab" {
		t.Fatalf("comment body must not render; got %q", got)
	}
}

// {% raw %} block is rejected at compile, same as the `raw` filter — no author escape hatch.
func TestRawBlockRejected(t *testing.T) {
	if err := mustCompileErr("{% raw %}{{ settings.x }}{% endraw %}"); err == nil || !strings.Contains(err.Error(), "{% raw %} block is not available") {
		t.Fatalf("{%% raw %%} block must be rejected at compile; got %v", err)
	}
}

// --- Render budget ----------------------------------------------------------

func TestRenderBudgetTrips(t *testing.T) {
	// huge output trips the byte budget
	big := strings.Repeat("x", 100)
	if _, err := renderErr("{% for i in settings.l %}"+big+"{% endfor %}",
		Options{Settings: m("l", makeList(100000)), MaxBytes: 1000}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("byte budget must trip; got %v", err)
	}
	// too many steps trips the step budget
	if _, err := renderErr("{% for i in settings.l %}{{ i }}{% endfor %}",
		Options{Settings: m("l", makeList(100000)), MaxSteps: 100}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("step budget must trip; got %v", err)
	}
}

// --- Render children --------------------------------------------------------

func TestRenderChildScope(t *testing.T) {
	child, _ := compileFixture("[{{ label }}|{{ settings.own }}]")
	resolve := func(name string) (Child, error) {
		return Child{Template: child, Settings: m("own", "childval")}, nil
	}
	// arg becomes a child LOCAL; child keeps its OWN settings (not the caller's).
	out := render(t, "{% render 'c', label: settings.x %}", Options{Settings: m("x", "fromparent"), Resolve: resolve})
	if out != "[fromparent|childval]" {
		t.Fatalf("render scope; got %q", out)
	}
	// child cannot read caller's undeclared data
	child2, _ := compileFixture("{{ product.title }}")
	resolve2 := func(name string) (Child, error) { return Child{Template: child2, Allow: nil}, nil }
	if _, err := renderErr("{% render 'c' %}", Options{Data: m("product", m("title", "x")), Allow: []string{"product"}, Resolve: resolve2}); err == nil {
		t.Fatal("child must NOT inherit the caller's allowed roots")
	}
	// no resolver wired → render errors
	if _, err := renderErr("{% render 'c' %}", Options{}); err == nil {
		t.Fatal("render with no resolver must error")
	}
}

// --- RenderTargets static analysis ------------------------------------------

func TestRenderTargets(t *testing.T) {
	tmpl, _ := compileFixture("{% render 'a' %}{% if settings.x %}{% render 'b' %}{% endif %}{% for i in settings.l %}{% render 'a' %}{% endfor %}")
	got := tmpl.RenderTargets()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("render targets (deduped, incl. nested): %v", got)
	}
}

// --- helpers ----------------------------------------------------------------

func m(k string, v interface{}) map[string]interface{} { return map[string]interface{}{k: v} }
func mm(k1 string, v1 interface{}, k2 string, v2 interface{}) map[string]interface{} {
	return map[string]interface{}{k1: v1, k2: v2}
}
func makeList(n int) []interface{} {
	out := make([]interface{}, n)
	for i := range out {
		out[i] = "x"
	}
	return out
}
func runCases(t *testing.T, cases []struct {
	name, src, want string
	opts            Options
}) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := render(t, c.src, c.opts); got != c.want {
				t.Fatalf("src %q: got %q want %q", c.src, got, c.want)
			}
		})
	}
}

// --- host filters cannot override safety-critical builtins ---
func TestProtectedFiltersNotOverridable(t *testing.T) {
	evil := map[string]FilterFunc{
		"escape": func(in interface{}, _ FilterArgs) (interface{}, error) { return safeString(toString(in)), nil }, // tries to neuter escape
		"json":   func(in interface{}, _ FilterArgs) (interface{}, error) { return safeString("PWNED"), nil },
	}
	// escape still escapes (host override ignored)
	if got := render(t, `{{ "<b>" | escape }}`, Options{Filters: evil}); got != "&lt;b&gt;" {
		t.Fatalf("escape must not be overridable; got %q", got)
	}
	// json still does the builtin behavior, not the host's
	if got := render(t, `{{ settings.x | json }}`, Options{Settings: m("x", "y"), Filters: evil}); got != `"y"` {
		t.Fatalf("json must not be overridable; got %q", got)
	}
}

// --- raw is REMOVED: rejected at compile, no escape hatch; SafeHTML is host-only ---
func TestRawRemoved(t *testing.T) {
	// raw rejected at COMPILE
	if err := mustCompileErr("{{ settings.x | raw }}"); err == nil || !strings.Contains(err.Error(), "'raw' filter is not available") {
		t.Fatalf("raw must be rejected at compile; got %v", err)
	}
	// a host `raw` filter does NOT resurrect it — still rejected at compile (parser-level)
	evil := map[string]FilterFunc{"raw": func(in interface{}, _ FilterArgs) (interface{}, error) { return safeString("PWNED"), nil }}
	if _, err := renderErr("{{ settings.x | raw }}", Options{Settings: m("x", "<b>"), Filters: evil}); err == nil {
		t.Fatal("a host raw filter must not bypass the compile rejection")
	}
	// SafeHTML from the host renders unescaped; a plain string with the same content is escaped.
	if got := render(t, "{{ settings.x }}", Options{Settings: m("x", SafeHTML("<b>bold</b>"))}); got != "<b>bold</b>" {
		t.Fatalf("SafeHTML must render unescaped; got %q", got)
	}
	if got := render(t, "{{ settings.x }}", Options{Settings: m("x", "<b>bold</b>")}); got != "&lt;b&gt;bold&lt;/b&gt;" {
		t.Fatalf("plain string must be escaped; got %q", got)
	}
	// empty SafeHTML is falsy-empty for default (so `description | default:` and `{% if %}` behave right)
	if got := render(t, "{{ settings.x | default: 'fb' }}", Options{Settings: m("x", SafeHTML(""))}); got != "fb" {
		t.Fatalf("empty SafeHTML should fall back in default; got %q", got)
	}
}

// --- end-tag scan respects word boundaries (now for comment, raw is gone) ---
func TestEndTagBoundary(t *testing.T) {
	// {% endcommentary %} must NOT close {% comment %}; the comment body runs to the real {% endcomment %}.
	out := render(t, "a{% comment %} x {% endcommentary %} y {% endcomment %}b", Options{})
	if out != "ab" {
		t.Fatalf("endcommentary must not close comment; got %q", out)
	}
}

// --- nil.size is 0 so `{% if X.size > 0 %}` is safe on a missing collection (no nil-vs-number error) ---
func TestNilSizeIsZero(t *testing.T) {
	// missing field → .size is 0 → comparison is number-vs-number, not an error
	out := render(t, "{% if settings.x.items.size > 0 %}Y{% else %}N{% endif %}", Options{Settings: m("x", nil)})
	if out != "N" {
		t.Fatalf("nil.items.size > 0 should be false (size 0), not error; got %q", out)
	}
	// directly: nil collection's size
	out = render(t, "{{ settings.missing.size }}", Options{})
	if out != "0" {
		t.Fatalf("missing.size should be 0; got %q", out)
	}
	// but a genuine nil > number (no .size) still errors — the #7 guard is intact
	if _, err := renderErr("{% if settings.x > 0 %}Y{% endif %}", Options{Settings: m("x", nil)}); err == nil {
		t.Fatal("nil > number (without .size) must still error")
	}
}
