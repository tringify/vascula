package vascula

import (
	"strings"
	"testing"
)

func FuzzCompileAndRender(f *testing.F) {
	for _, source := range []string{
		`<h1>{{ settings.title }}</h1>`,
		`{% if product %}{{ product.title | escape }}{% endif %}`,
		`{% for item in settings.items limit: 2 %}{{ item }}{% endfor %}`,
		`{% assign n = 1 %}{% capture title %}{{ n }}{% endcapture %}{{ title }}`,
		`{% form "subscribe" %}{{ settings.title }}{% endform %}`,
		`{% render "missing" %}`,
		`{% blocks %}`,
		`{{ settings.title | json }}`,
		`{# comment #}{{ settings.absent | default: "none" }}`,
		`{{ (((settings.title))) }}`,
		`{{ "unterminated`,
		"\x00\xff{%",
		`{% for i in (1..3) %}{% for j in (i..3) %}{{ forloop.parentloop.index }}{% endfor %}{% endfor %}`,
		`{% if true %}{% assign kept = 9007199254740993 | plus: 1 %}{% endif %}{{ kept | divided_by: 3 }}`,
		`{{ settings.items | where: "x" | sort | map: "y" | reverse | join: "," }}`,
		`{% for i in (settings.n..2) %}{{ i | modulo: 0 }}{% endfor %}`,
		`{{ product | json }}<a title="{{ product | json }}">`,
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 16*1024 {
			t.Skip()
		}
		template, err := CompileWithOptions(source, CompileOptions{Forms: map[string]FormDefinition{
			"subscribe": {Path: "/subscribe", TokenContextKey: "token", TokenField: "csrf"},
		}, Tags: map[string]TagDefinition{"blocks": {Argument: TagOptionalArgument}}, Reserved: []string{"site"}})
		if err != nil {
			return
		}
		_ = template.ExternalNames()
		out, _ := template.Render(Options{
			Data:     map[string]interface{}{"token": "sample", "product": map[string]interface{}{"title": "<script>&"}},
			Allow:    []string{"product"},
			Settings: map[string]interface{}{"title": "<sample>", "items": []interface{}{0, "x", nil}},
			Tags: map[string]TagFunc{"blocks": func(call TagCall) (SafeHTML, error) {
				v, _ := call.Lookup("settings")
				return SafeHTML(toString(call.Arg) + toString(v)), nil
			}},
			MaxSteps: 1_000, MaxBytes: 8_192,
		})
		// Host data is escaped wherever a template prints it.
		if !strings.Contains(source, "<script>") && strings.Contains(out, "<script>&") {
			t.Fatalf("unescaped host data in %q", out)
		}
	})
}
