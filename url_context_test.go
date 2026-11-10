package vascula

import (
	"html"
	"strings"
	"testing"
)

func renderLink(t *testing.T, src string, link interface{}) string {
	t.Helper()
	tpl, err := Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.Render(Options{Settings: map[string]interface{}{"link": link}})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A value that starts a URL attribute cannot choose a scripting scheme,
// however it is spelled.
func TestURLAttributeRejectsUnsafeSchemes(t *testing.T) {
	for _, link := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		" \t javascript:alert(1)",
		"\x01javascript:alert(1)",
		"java\tscript:alert(1)",
		"java\nscript:alert(1)",
		"vbscript:msgbox(1)",
		"data:text/html,<script>alert(1)</script>",
		"data:image/svg+xml,<svg onload=alert(1)>",
		"livescript:x",
		"jar:file:x",
		":x",
	} {
		for _, src := range []string{
			`<a href="{{ settings.link }}">`,
			`<a href='{{ settings.link }}'>`,
			`<a href={{ settings.link }}>`,
			`<a HREF = " {{ settings.link }}">`,
			`<a class="x" href="{{ settings.link }}" download>`,
			`<form action="{{ settings.link }}">`,
			`<button formaction="{{ settings.link }}">`,
			`<iframe src="{{ settings.link }}">`,
			`<video poster="{{ settings.link }}">`,
			`<object data="{{ settings.link }}">`,
			`<svg><use xlink:href="{{ settings.link }}"/></svg>`,
			`<blockquote cite="{{ settings.link }}">`,
			`<td background="{{ settings.link }}">`,
			`<a ping="{{ settings.link }}">`,
			`<html manifest="{{ settings.link }}">`,
			`<div data-src="{{ settings.link }}">`,
			`<a href="{{ "" }}{{ settings.link }}">`,
		} {
			out := renderLink(t, src, link)
			want := strings.NewReplacer(`" {{ settings.link }}`, `"#`, `{{ settings.link }}`, "#", `{{ "" }}`, "").Replace(src)
			if out != want {
				t.Errorf("%s with %q rendered %s", src, link, out)
			}
		}
	}
}

func TestURLAttributeKeepsSafeURLs(t *testing.T) {
	for _, link := range []string{
		"https://shop.example/a?b=1&c=2#d",
		"http://shop.example",
		"HTTPS://SHOP.EXAMPLE",
		"//cdn.example/x.png",
		"/products/tea",
		"products/tea",
		"./a:b",
		"?page=2",
		"#reviews",
		"mailto:hello@shop.example",
		"tel:+441234",
		"",
	} {
		out := renderLink(t, `<a href="{{ settings.link }}">`, link)
		want := `<a href="` + escapeForTest(link) + `">`
		if out != want {
			t.Errorf("%q rendered %s, want %s", link, out, want)
		}
	}
}

func escapeForTest(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&#34;").Replace(s)
}

// Only an image may arrive as a data URL, and only where an image loads.
func TestDataImageOnlyInImageSources(t *testing.T) {
	img := "data:image/png;base64,iVBORw0KGgo="
	if out := renderLink(t, `<img src="{{ settings.link }}">`, img); out != `<img src="`+img+`">` {
		t.Errorf("img src: %s", out)
	}
	if out := renderLink(t, `<source src="{{ settings.link }}">`, img); out != `<source src="`+img+`">` {
		t.Errorf("source src: %s", out)
	}
	for _, src := range []string{`<a href="{{ settings.link }}">`, `<iframe src="{{ settings.link }}">`, `<embed src="{{ settings.link }}">`} {
		if out := renderLink(t, src, img); !strings.Contains(out, `"#"`) {
			t.Errorf("%s: %s", src, out)
		}
	}
}

// Only the value that starts the URL picks its scheme; later parts are
// paths, queries and fragments and render as before.
func TestURLAttributeLaterPartsRenderAsBefore(t *testing.T) {
	for src, want := range map[string]string{
		`<a href="/search?q={{ settings.link }}">`:         `<a href="/search?q=javascript:alert(1)">`,
		`<a href="https://x.example/{{ settings.link }}">`: `<a href="https://x.example/javascript:alert(1)">`,
		`<a title="{{ settings.link }}">`:                  `<a title="javascript:alert(1)">`,
		`<a data-note="{{ settings.link }}">`:              `<a data-note="javascript:alert(1)">`,
		`<p>{{ settings.link }}</p>`:                       `<p>javascript:alert(1)</p>`,
		`<a href="#">{{ settings.link }}</a>`:              `<a href="#">javascript:alert(1)</a>`,
	} {
		if out := renderLink(t, src, "javascript:alert(1)"); out != want {
			t.Errorf("%s rendered %s, want %s", src, out, want)
		}
	}
}

// Each candidate in a srcset is checked; descriptors are kept.
func TestSrcsetChecksEveryCandidate(t *testing.T) {
	out := renderLink(t, `<img srcset="{{ settings.link }}">`, "https://cdn.example/a.jpg 1x, javascript:alert(1) 2x")
	if out != `<img srcset="https://cdn.example/a.jpg 1x, # 2x">` {
		t.Fatalf("srcset: %s", out)
	}
	out = renderLink(t, `<img srcset="{{ settings.link }} 400w, {{ settings.link }} 800w">`, "/a.jpg")
	if out != `<img srcset="/a.jpg 400w, /a.jpg 800w">` {
		t.Fatalf("srcset: %s", out)
	}
	out = renderLink(t, `<img srcset="/a.jpg 400w, {{ settings.link }} 800w">`, "javascript:alert(1)")
	if out != `<img srcset="/a.jpg 400w, # 800w">` {
		t.Fatalf("srcset: %s", out)
	}
}

// Character references in a value the application vouched for are decoded
// before the scheme is read, as the browser decodes them.
func TestURLAttributeDecodesTrustedReferences(t *testing.T) {
	for _, link := range []SafeHTML{"javascript&#58;alert(1)", "&#106;avascript:alert(1)", "java&Tab;script:alert(1)"} {
		if out := renderLink(t, `<a href="{{ settings.link }}">`, link); out != `<a href="#">` {
			t.Errorf("%q rendered %s", link, out)
		}
	}
	if out := renderLink(t, `<a href="{{ settings.link }}">`, SafeHTML("/a?b=1&amp;c=2")); out != `<a href="/a?b=1&amp;c=2">` {
		t.Errorf("safe url changed: %s", out)
	}
}

func TestCSSURLInStyleAttribute(t *testing.T) {
	out0 := renderLink(t, `<div style="background: url(/a.png), url({{ settings.link }})">`, "javascript:alert(1)")
	if out0 != `<div style="background: url(/a.png), url(#)">` {
		t.Errorf("css url: %s", out0)
	}
	// A printed value that ends url() early cannot add an unsafe one.
	out0 = renderLink(t, `<div style="color: {{ settings.link }}">`, "red; background: URL( 'javascript:alert(1)' )")
	if out0 != `<div style="color: red; background: URL( &#39;#&#39; )">` {
		t.Errorf("css url: %s", out0)
	}
	out0 = renderLink(t, `<div style="color: {{ settings.link }}">`, `red; background: u\72l(javascript:alert(1))`)
	if out0 != `<div style="">` {
		t.Errorf("css escapes: %s", out0)
	}
	out0 = renderLink(t, `<div style="background: url({{ settings.link }})">`, "data:image/png;base64,AAAA")
	if out0 != `<div style="background: url(data:image/png;base64,AAAA)">` {
		t.Errorf("css data image: %s", out0)
	}
	out := renderLink(t, `<div style="background-image: url({{ settings.link }})">`, "javascript:alert(1)")
	if out != `<div style="background-image: url(#)">` {
		t.Errorf("css url: %s", out)
	}
	out = renderLink(t, `<div style="background:URL('{{ settings.link }}')">`, "https://cdn.example/a b.jpg')")
	if out != `<div style="background:URL('https://cdn.example/a%20b.jpg%27%29')">` {
		t.Errorf("css url: %s", out)
	}
	out = renderLink(t, `<div style="color: {{ settings.link }}">`, "red")
	if out != `<div style="color: red">` {
		t.Errorf("non-url style: %s", out)
	}
}

// The check sees the printed value whatever produced it.
func TestURLAttributeChecksFilteredAndJSONValues(t *testing.T) {
	out := renderLink(t, `<a href="{{ settings.link | strip }}">`, "  javascript:alert(1)")
	if out != `<a href="#">` {
		t.Errorf("filtered: %s", out)
	}
	out = renderLink(t, `<a href="{{ settings.link | json }}">`, "javascript:alert(1)")
	if out != `<a href="#">` {
		t.Errorf("json: %s", out)
	}
}

func compileError(t *testing.T, src string) string {
	t.Helper()
	_, err := Compile(src)
	if err == nil {
		t.Fatalf("%s compiled", src)
	}
	return err.Error()
}

// The whole value of a URL attribute is checked as the browser reads it,
// whatever literal text comes before the printed part.
func TestURLAttributeChecksTheAssembledValue(t *testing.T) {
	for _, c := range []struct{ src, link string }{
		{`<a href="&#32;{{ settings.link }}">`, "javascript:alert(1)"},
		{`<a href="&#9;&#10;{{ settings.link }}">`, "javascript:alert(1)"},
		{`<a href=" &NewLine;{{ settings.link }}">`, "javascript:alert(1)"},
		{`<a href="java{{ settings.link }}">`, "script:alert(1)"},
		{`<a href="&#106;ava{{ settings.link }}">`, "script:alert(1)"},
		{`<a href="{{ settings.link }}script:alert(1)">`, "java"},
		{`<a href="{% if true %}java{% endif %}{{ settings.link }}">`, "script:alert(1)"},
		{`<a href="{% if true %}javascript:{% else %}/{% endif %}{{ settings.link }}">`, "alert(1)"},
		{`<a href="{% case 1 %}{% when 1 %}javascript{% endcase %}{{ settings.link }}">`, ":alert(1)"},
		{`<a href="{% for i in (1..2) %}{{ settings.link }}{% endfor %}">`, "javascript:void(0)//"},
		{`<a href="{% cycle settings.link, "/" %}">`, "javascript:alert(1)"},
		{`<a href={{ settings.link }}>`, " javascript:alert(1)"},
	} {
		out := renderLink(t, c.src, c.link)
		if !strings.Contains(out, `href="#"`) && !strings.Contains(out, `href=#>`) {
			t.Errorf("%s with %q rendered %s", c.src, c.link, out)
		}
	}
	// A safe assembled value prints exactly as written.
	if out := renderLink(t, `<a href="&#32;/a?{{ settings.link }}">`, "q=1&r=2"); out != `<a href="&#32;/a?q=1&amp;r=2">` {
		t.Errorf("safe: %s", out)
	}
	// Values written entirely in the template are the author's.
	if out := renderLink(t, `<a href="javascript:void(0)">{{ settings.link }}</a>`, "x"); out != `<a href="javascript:void(0)">x</a>` {
		t.Errorf("literal: %s", out)
	}
}

// Element and attribute names cannot be printed; control flow must leave
// every later output in one context.
func TestAmbiguousOrDynamicMarkupFailsToCompile(t *testing.T) {
	for src, want := range map[string]string{
		`<a {{ settings.n }}="{{ settings.link }}">`:                                       "a printed value in <a> is followed by '='",
		`<a data-{{ settings.n }}="x">`:                                                    "cannot print part of an attribute name in <a>",
		`<a {{ settings.n }}ref="x">`:                                                      "a printed value in <a> is followed by 'r'",
		`<a {{ settings.n }}{{ settings.m }}>`:                                             "follows another printed value in <a> with no space",
		`<a {% if settings.c %}{{ settings.n }}{% endif %}{{ settings.m }}>`:               "follows another printed value in <a> with no space",
		`<a h{{ settings.n }}="{{ settings.link }}">`:                                      "cannot print part of an attribute name in <a>",
		`<div {% render "x" %}>`:                                                           "{% render %} cannot print attributes in <div>",
		`<h{{ settings.n }}>`:                                                              "cannot print an element name",
		`</{{ settings.n }}>`:                                                              "cannot print an element name",
		`<a {% if settings.c %}href{% else %}title{% endif %}="{{ settings.link }}">`:      "depends on control flow",
		`<a href="{% if settings.c %}x" title="{% endif %}{{ settings.link }}">`:           "{% if %} branches end in different HTML contexts",
		`{% if settings.c %}<a href="{% else %}<a title="{% endif %}{{ settings.link }}">`: "{% if %} branches end in different HTML contexts",
		`{% if settings.c %}<a href="{% else %}<a href="{% endif %}{{ settings.link }}">`:  "{% if %} branches end in different HTML contexts",
		`{% if settings.c %}<script>{% endif %}{{ settings.link }}`:                        "{% if %} branches end in different HTML contexts",
		`{% case settings.c %}{% when 1 %}<a href="{% endcase %}{{ settings.link }}">`:     "{% case %} branches end in different HTML contexts",
		`{% for i in settings.l %}<a href="{% endfor %}{{ settings.link }}">`:              "the {% for %} body end",
		`{% for i in settings.l %}<a href="{{ i }}{% break %}">{% endfor %}`:               "{% break %} stands in the href attribute of <a>",
		`{% for i in settings.l %}<a href="{% continue %}{{ i }}">{% endfor %}`:            "{% continue %} stands in the href attribute",
		`<a href="{{ settings.link }}`:                                                     "the template ends inside the <a> tag",
		`{% capture c %}<a href="{% endcapture %}{{ settings.link }}">`:                    "the {% capture %} body ends inside the <a> tag",
		`<a href={% render "x" %}>`:                                                        "{% render %} cannot print the unquoted href attribute value; quote the value",
	} {
		if got := compileError(t, src); !strings.Contains(got, want) {
			t.Errorf("%s: %s, want %q", src, got, want)
		}
	}
	// Branches that agree compile.
	for _, src := range []string{
		`<a href="{% if settings.c %}/a{% elsif settings.d %}/b{% endif %}{{ settings.link }}">`,
		`<div class="a{% if settings.c %} b{% endif %}"{% if settings.c %} hidden{% endif %}>`,
		`<{% if settings.c %}ul{% else %}ol{% endif %} class="x">`,
		`<a {% if settings.c %}href="/a"{% else %}href="/b"{% endif %}>`,
		`<a href="/{% if settings.c %}a"{% else %}b"{% endif %}>`,
		`{% for i in settings.l %}<a href="{{ i }}">{% if forloop.last %}{% break %}{% endif %}</a>{% endfor %}`,
		`{% for i in (1..3) %}<{{ i }}>{% endfor %}`,
		`<p>{% capture c %}<a href="{{ settings.link }}">{% endcapture %}</p>`,
		`<img src="/a.png" {{ settings.n }} {{ settings.m }}/>`,
		`<input data-srch{% if settings.c %} autofocus{% endif %}>`,
		`<input {% if settings.c %}src{% endif %} title="x">`,
	} {
		if _, err := Compile(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// Output in an unquoted attribute value cannot end the value or add
// attributes.
func TestUnquotedAttributeValuesEscapeDelimiters(t *testing.T) {
	for src, want := range map[string]string{
		`<img src={{ settings.link }}>`:          `<img src=&#47;missing&#32;onerror&#61;alert(1)>`,
		`<img alt={{ settings.link }} src=/x>`:   `<img alt=&#47;missing&#32;onerror&#61;alert(1) src=/x>`,
		`<img alt=a{{ settings.link }}>`:         `<img alt=a&#47;missing&#32;onerror&#61;alert(1)>`,
		`<img alt={% cycle settings.link %}>`:    `<img alt=&#47;missing&#32;onerror&#61;alert(1)>`,
		`<img alt={{ settings.link | json }}>`:   `<img alt=&#34;&#47;missing&#32;onerror&#61;alert(1)&#34;>`,
		`<img alt={{ settings.link | escape }}>`: `<img alt=&#47;missing&#32;onerror&#61;alert(1)>`,
	} {
		if out := renderLink(t, src, "/missing onerror=alert(1)"); out != want {
			t.Errorf("%s rendered %s, want %s", src, out, want)
		}
	}
	for _, link := range []string{"a\tb", "a\nb", "a\rb", "a\fb", `a"b`, "a'b", "a`b", "a>b", "a<b", "a=b", "a/b", "a b", "a&b"} {
		out := renderLink(t, `<p title={{ settings.link }}>`, link)
		value := strings.TrimSuffix(strings.TrimPrefix(out, "<p title="), ">")
		if strings.ContainsAny(value, " \t\n\r\f\"'`<>=/") || html.UnescapeString(value) != link {
			t.Errorf("%q rendered %s", link, out)
		}
	}
	if out := renderLink(t, `<p title={{ settings.link }}>`, SafeHTML("a b")); out != `<p title=a&#32;b>` {
		t.Errorf("safe html: %s", out)
	}
}

// srcset is read as browsers read it: a URL runs to whitespace, so commas
// inside it are part of it.
func TestSrcsetFollowsTheHTMLParser(t *testing.T) {
	for link, want := range map[string]string{
		"/image?crop=1,2:3 1x":                                 "/image?crop=1,2:3 1x",
		"/a.jpg?w=1,2 1x, /b.jpg?w=3,4 2x":                     "/a.jpg?w=1,2 1x, /b.jpg?w=3,4 2x",
		"/a.jpg,javascript:alert(1) 2x":                        "/a.jpg,javascript:alert(1) 2x",
		"/a.jpg, javascript:alert(1) 2x":                       "/a.jpg, # 2x",
		"javascript:alert(1),, /b.jpg 2x":                      "#,, /b.jpg 2x",
		",,javascript:alert(1) 1x":                             ",,# 1x",
		"/a.jpg (a,b) 1x, javascript:alert(1)":                 "/a.jpg (a,b) 1x, #",
		"data:image/png;base64,AAAA 1x":                        "# 1x",
		"https://cdn.example/a.jpg 1x, javascript:alert(1) 2x": "https://cdn.example/a.jpg 1x, # 2x",
	} {
		out := renderLink(t, `<img srcset="{{ settings.link }}">`, link)
		if want := `<img srcset="` + html.EscapeString(want) + `">`; out != want {
			t.Errorf("%q rendered %s, want %s", link, out, want)
		}
	}
	if out := renderLink(t, `<img srcset="/a.jpg 1x, {{ settings.link }} 2x">`, "javascript:alert(1)"); out != `<img srcset="/a.jpg 1x, # 2x">` {
		t.Errorf("continued: %s", out)
	}
	if out := renderLink(t, `<img srcset="/a.jpg?c={{ settings.link }} 2x">`, "1,javascript:alert(1)"); out != `<img srcset="/a.jpg?c=1,javascript:alert(1) 2x">` {
		t.Errorf("inside url: %s", out)
	}
}

// Where an attribute goes, only host-vouched markup or a single plain
// attribute name prints.
func TestPrintedAttributesAreHostMarkupOrPlainNames(t *testing.T) {
	for link, want := range map[interface{}]string{
		HTMLAttr(`loading="lazy"`):   `<img src="/a.png" loading="lazy">`,
		SafeHTML(`loading="lazy"`):   `<img src="/a.png" >`,
		SafeHTML("onerror=alert(1)"): `<img src="/a.png" >`,
		SafeHTML("hidden"):           `<img src="/a.png" hidden>`,
		"hidden":                     `<img src="/a.png" hidden>`,
		"data-x_1":                   `<img src="/a.png" data-x_1>`,
		"":                           `<img src="/a.png" >`,
		"onerror=alert(1)":           `<img src="/a.png" >`,
		"onerror":                    `<img src="/a.png" >`,
		"OnLoad":                     `<img src="/a.png" >`,
		"style":                      `<img src="/a.png" >`,
		"href":                       `<img src="/a.png" >`,
		"srcset":                     `<img src="/a.png" >`,
		"data-src":                   `<img src="/a.png" >`,
		"a b":                        `<img src="/a.png" >`,
		"xlink:href":                 `<img src="/a.png" >`,
	} {
		if out := renderLink(t, `<img src="/a.png" {{ settings.link }}>`, link); out != want {
			t.Errorf("%q rendered %s, want %s", link, out, want)
		}
	}
}

// A name is never split across control flow: whatever the branches, the
// attribute a value belongs to is the one the scan sees. These are the forms
// that once printed javascript: into an unchecked href.
func TestNamesSplitByControlFlowFailToCompile(t *testing.T) {
	for _, src := range []string{
		`<a {% if settings.c %}h{% endif %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}hr{% else %}ti{% endif %}ef="{{ settings.link }}">`,
		`<a h{% if settings.c %}r{% endif %}ef="{{ settings.link }}">`,
		`<a h{% if settings.c %}{% endif %}ref="{{ settings.link }}">`,
		`<a {% unless settings.c %}h{% endunless %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}x{% elsif settings.d %}h{% endif %}ref="{{ settings.link }}">`,
		`<a {% case settings.c %}{% when 1 %}h{% endcase %}ref="{{ settings.link }}">`,
		`<a {% case settings.c %}{% when 1 %}h{% else %}x{% endcase %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}{% if settings.d %}h{% endif %}{% endif %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}{% case settings.d %}{% when 1 %}h{% endcase %}{% endif %}ref="{{ settings.link }}">`,
		`<a {% for i in settings.l %}h{% endfor %}ref="{{ settings.link }}">`,
		`<a {% for i in settings.l %}{% if forloop.first %}h{% else %}ref{% endif %}{% endfor %}="{{ settings.link }}">`,
		`<a {% for i in settings.l %} h{% break %}{% endfor %}ref="{{ settings.link }}">`,
		`<a {% for i in settings.l %} h{% if i %}{% continue %}{% endif %} x{% endfor %}ref="{{ settings.link }}">`,
		`<a h{% for i in settings.l %}{% endfor %}ref="{{ settings.link }}">`,
		`<a {% for i in settings.l %}{% else %}h{% endfor %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}h{% endif %}{% assign y = 1 %}ref="{{ settings.link }}">`,
		`<a {% if settings.c %}h{% endif %}{% if settings.d %}ref{% endif %}="{{ settings.link }}">`,
		`<a {% if settings.c %}h{% endif %}{{ settings.n }}>`,
		`<{% if settings.c %}scr{% endif %}ipt>{{ settings.link }}`,
		`<{% if settings.c %}{% endif %}a href="{{ settings.link }}">`,
	} {
		_, err := Compile(src)
		if err == nil {
			t.Errorf("%s compiled", src)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, "name") {
			t.Errorf("%s: %s, want an error about a split name", src, msg)
		}
	}
	// Names ended before or by the next character still compile.
	for src, want := range map[string]string{
		`<input {% if settings.c %}checked{% endif %}>`:                      `<input checked>`,
		`<input {% if settings.c %}checked{% endif %} title="x">`:            `<input checked title="x">`,
		`<input {% if settings.c %}checked{% endif %}/>`:                     `<input checked/>`,
		`<a {% if settings.c %}href{% endif %} title="{{ settings.link }}">`: `<a href title="javascript:alert(1)">`,
		`<{% if settings.c %}ul{% else %}ol{% endif %} class="x">`:           `<ul class="x">`,
	} {
		if out := renderC(t, src); out != want {
			t.Errorf("%s rendered %s, want %s", src, out, want)
		}
	}
}

func renderC(t *testing.T, src string) string {
	t.Helper()
	tpl, err := Compile(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	out, err := tpl.Render(Options{Settings: map[string]interface{}{"c": true, "link": "javascript:alert(1)"}})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// SafeHTML is markup for the page body. Inside a tag it is text: it cannot add
// attributes or end the value it stands in. Only HTMLAttr adds attributes.
func TestSafeHTMLInsideATagIsText(t *testing.T) {
	for _, c := range []struct {
		src  string
		link interface{}
		want string
	}{
		{`<img src=/missing {{ settings.link }}>`, SafeHTML("onerror=alert(1)"), `<img src=/missing >`},
		{`<img src="/a.png" {{ settings.link }}>`, SafeHTML(`<b>x</b> onerror="alert(1)"`), `<img src="/a.png" >`},
		{`<img alt="{{ settings.link }}">`, SafeHTML(`<p>Tom &amp; "Jerry"</p><a title="x" onclick="y">z</a>`), `<img alt="Tom &amp; &#34;Jerry&#34;z">`},
		{`<img alt='{{ settings.link }}'>`, SafeHTML(`it's`), `<img alt='it&#39;s'>`},
		{`<img alt={{ settings.link }}>`, SafeHTML(`<i>a</i> onerror=alert(1)`), `<img alt=a&#32;onerror&#61;alert(1)>`},
		{`<a href="{{ settings.link }}">`, SafeHTML(`javascript:alert(1)`), `<a href="#">`},
		{`<a href="{{ settings.link }}">`, SafeHTML(`<b>/x?a=1&amp;b=2</b>`), `<a href="/x?a=1&amp;b=2">`},
		{`<p>{{ settings.link }}</p>`, SafeHTML(`<b>x</b>`), `<p><b>x</b></p>`},
		{`<p>{{ settings.link }}</p>`, HTMLAttr(`loading="lazy"`), `<p>loading=&#34;lazy&#34;</p>`},
		{`<img alt="{{ settings.link }}">`, HTMLAttr(`a" onerror="x`), `<img alt="a&#34; onerror=&#34;x">`},
	} {
		if out := renderLink(t, c.src, c.link); out != c.want {
			t.Errorf("%s with %q rendered %s, want %s", c.src, c.link, out, c.want)
		}
	}
}
