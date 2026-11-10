package vascula

import (
	"html"
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

// A URL attribute with a printed part is safe as the browser reads it, however
// the literal text before the printed value and the value are spelled, and an
// unquoted value cannot add attributes.
func FuzzURLAttributes(f *testing.F) {
	for _, seed := range [][2]string{
		{"", "javascript:alert(1)"},
		{"&#32;", "javascript:alert(1)"},
		{"java", "script:alert(1)"},
		{"&#106;ava", "script:x"},
		{"/a?", "q=1&r=2"},
		{"", "/missing onerror=alert(1)"},
		{"", "/image?crop=1,2:3 1x, javascript:x 2x"},
		{" \t", "\x01JaVa\tScRiPt:x"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, prefix, value string) {
		if len(prefix) > 256 || len(value) > 1024 || strings.ContainsAny(prefix, "{}%\"'<>`= \t\n\r\f/") {
			t.Skip()
		}
		render := func(src string) string {
			tpl, err := Compile(src)
			if err != nil {
				t.Fatalf("%q: %v", src, err)
			}
			out, err := tpl.Render(Options{Settings: map[string]interface{}{"v": value}})
			if err != nil {
				t.Fatalf("%q: %v", src, err)
			}
			return out
		}
		quoted := render(`<a href="` + prefix + `{{ settings.v }}">x</a>`)
		rest, ok := strings.CutPrefix(quoted, `<a href="`)
		v, tail, _ := strings.Cut(rest, `"`)
		if !ok || tail != ">x</a>" || !safeURL(html.UnescapeString(v), false) {
			t.Fatalf("quoted %q %q: %s", prefix, value, quoted)
		}
		unquoted := render(`<a href=` + prefix + `{{ settings.v }}>x</a>`)
		rest, ok = strings.CutPrefix(unquoted, `<a href=`)
		end := strings.IndexAny(rest, " \t\n\r\f>")
		if !ok || end < 0 || rest[end:] != ">x</a>" || !safeURL(html.UnescapeString(rest[:end]), false) {
			t.Fatalf("unquoted %q %q: %s", prefix, value, unquoted)
		}
		srcset := render(`<img srcset="` + prefix + `{{ settings.v }}">`)
		rest, ok = strings.CutPrefix(srcset, `<img srcset="`)
		v, tail, _ = strings.Cut(rest, `"`)
		if _, unsafe := checkSrcset(html.UnescapeString(v)); !ok || tail != ">" || unsafe {
			t.Fatalf("srcset %q %q: %s", prefix, value, srcset)
		}
	})
}

// FuzzAttributeNamesAcrossControlFlow: however literal name fragments are
// split by if, case or for, a template either fails to compile or every
// attribute in its output is one the scan saw: a URL attribute's value is
// checked, and printed body HTML adds no attribute.
func FuzzAttributeNamesAcrossControlFlow(f *testing.F) {
	for _, seed := range [][4]string{
		{"", "h", "ref", "if"},
		{"h", "r", "ef", "if"},
		{"", "hr", "ef", "case"},
		{"", "h", "ref", "for"},
		{"x ", "src", " ", "if"},
		{"", "on", "click", "if"},
		{"", "", "href", "for"},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3])
	}
	f.Fuzz(func(t *testing.T, a, b, c, flow string) {
		for _, s := range []string{a, b, c} {
			if len(s) > 12 || strings.Trim(s, "abcdefhilnorstx-: ") != "" {
				t.Skip()
			}
		}
		var open, end string
		switch flow {
		case "if":
			open, end = "{% if settings.c %}", "{% endif %}"
		case "case":
			open, end = "{% case settings.c %}{% when true %}", "{% endcase %}"
		case "for":
			open, end = "{% for i in settings.l %}", "{% endfor %}"
		default:
			t.Skip()
		}
		src := `<a ` + a + open + b + end + c + `="{{ settings.v }}" {{ settings.h }}>x</a>`
		tpl, err := Compile(src)
		if err != nil {
			return
		}
		for _, on := range []bool{true, false} {
			out, err := tpl.Render(Options{Settings: map[string]interface{}{
				"c": on, "l": []interface{}{1, 2}, "v": "javascript:alert(1)", "h": SafeHTML(`onclick="alert(2)" href="javascript:alert(3)"`),
			}})
			if err != nil {
				t.Fatalf("%q: %v", src, err)
			}
			for name, value := range tagAttributes(t, out) {
				// The literal fragments cannot spell onclick; only the printed
				// HTML could add it.
				if name == "onclick" {
					t.Fatalf("%q (c=%v) rendered handler %s: %s", src, on, name, out)
				}
				if kind := classifyAttr("a", name); kind != attrPlain && strings.Contains(strings.ToLower(value), "javascript:") {
					t.Fatalf("%q (c=%v) rendered %s=%q: %s", src, on, name, value, out)
				}
			}
		}
	})
}

// tagAttributes reads the attributes of the first tag in s as an HTML
// tokenizer does: names lowercased, values unquoted and unescaped.
func tagAttributes(t *testing.T, s string) map[string]string {
	t.Helper()
	i := strings.IndexByte(s, ' ')
	if !strings.HasPrefix(s, "<a") || i < 0 {
		t.Fatalf("not a tag: %s", s)
	}
	attrs := map[string]string{}
	for i < len(s) {
		for i < len(s) && (isHTMLSpace(s[i]) || s[i] == '/') {
			i++
		}
		if i >= len(s) || s[i] == '>' {
			break
		}
		start := i
		for i++; i < len(s) && !isHTMLSpace(s[i]) && s[i] != '/' && s[i] != '>' && s[i] != '='; i++ {
		}
		name := strings.ToLower(s[start:i])
		for i < len(s) && isHTMLSpace(s[i]) {
			i++
		}
		value := ""
		if i < len(s) && s[i] == '=' {
			i++
			for i < len(s) && isHTMLSpace(s[i]) {
				i++
			}
			if i < len(s) && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				j := strings.IndexByte(s[i+1:], q)
				if j < 0 {
					t.Fatalf("unterminated value: %s", s)
				}
				value = s[i+1 : i+1+j]
				i += j + 2
			} else {
				start := i
				for i < len(s) && !isHTMLSpace(s[i]) && s[i] != '>' {
					i++
				}
				value = s[start:i]
			}
		}
		if _, dup := attrs[name]; !dup {
			attrs[name] = html.UnescapeString(value)
		}
	}
	return attrs
}
