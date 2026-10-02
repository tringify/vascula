package vascula

import (
	"strings"
	"testing"
)

// A value whose strings would break out of an attribute if json were left raw.
var hostile = map[string]interface{}{"t": `x" onmouseover=alert(1) y='z'`}

func renderJSON(t *testing.T, src string) string {
	t.Helper()
	tpl, err := Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.Render(Options{Settings: map[string]interface{}{"x": hostile}})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestJSONIsEscapedInsideTags(t *testing.T) {
	const escaped = `{&#34;t&#34;:&#34;x\&#34; onmouseover=alert(1) y=&#39;z&#39;&#34;}`
	for _, src := range []string{
		`<div data-x="{{ settings.x | json }}"></div>`,
		`<div data-x='{{ settings.x | json }}'></div>`,
		`<div data-x={{ settings.x | json }}></div>`,
		`<div {{ settings.x | json }}></div>`,
		`<div class="a" data-x="pre {{ settings.x | json }} post"></div>`,
		"<div\n  data-x=\"{{ settings.x | json }}\">",
		`{% if true %}<div data-x="{% endif %}{{ settings.x | json }}">`,
	} {
		out := renderJSON(t, src)
		if !strings.Contains(out, escaped) || strings.Contains(out, `" onmouseover`) {
			t.Errorf("%s\n rendered %s", src, out)
		}
	}
}

func TestJSONStaysRawOutsideTags(t *testing.T) {
	const raw = `{"t":"x\" onmouseover=alert(1) y='z'"}`
	for _, src := range []string{
		`<script>var x = {{ settings.x | json }};</script>`,
		`<script type="application/json">{{ settings.x | json }}</script>`,
		`<div data-a="1">{{ settings.x | json }}</div>`,
		`<p>a < b {{ settings.x | json }}</p>`,
		`<!-- <div data-x=" --> {{ settings.x | json }}`,
		`<!DOCTYPE html><p>{{ settings.x | json }}</p>`,
		`<script>if (a<b) { x = "<div data-x='"; }</script>{{ settings.x | json }}`,
		`{{ settings.x | json }}`,
	} {
		if out := renderJSON(t, src); !strings.Contains(out, raw) {
			t.Errorf("%s\n rendered %s", src, out)
		}
	}
}

// Templates already write `| json | escape` in attributes; that output must not
// change (escape yields escaped HTML, which is never escaped twice).
func TestJSONEscapeInAttributeRendersAsBefore(t *testing.T) {
	a := renderJSON(t, `<div data-x="{{ settings.x | json | escape }}">`)
	b := renderJSON(t, `<div data-x="{{ settings.x | json }}">`)
	if a != b {
		t.Fatalf("json | escape %s\njson        %s", a, b)
	}
	if strings.Contains(a, "&amp;#34;") {
		t.Fatalf("escaped twice: %s", a)
	}
}

// An output tag standing where an attribute value starts is that value; the
// tag reads on in its attribute area afterwards.
func TestOutputAsUnquotedValueKeepsTheTagOpen(t *testing.T) {
	out := renderJSON(t, `<div data-a={{ settings.x | json }} data-b="{{ settings.x | json }}">{{ settings.x | json }}</div>`)
	if strings.Count(out, `{"t"`) != 1 {
		t.Fatalf("want only the text-content json raw: %s", out)
	}
}
