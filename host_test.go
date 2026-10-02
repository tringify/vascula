package vascula

import (
	"strings"
	"testing"
)

func TestStandaloneHostContract(t *testing.T) {
	const source = `{% form "subscribe" %}{{ settings.title }}{% endform %}`
	if _, err := Compile(source); err == nil {
		t.Fatal("unconfigured form must fail compilation")
	}
	forms := map[string]FormDefinition{"subscribe": {Path: "/subscribe", TokenContextKey: "token", TokenField: "authenticity_token", ActionAttribute: "data-action"}}
	tmpl, err := CompileWithOptions(source, CompileOptions{Forms: forms})
	if err != nil {
		t.Fatal(err)
	}
	forms["subscribe"] = FormDefinition{Path: "/changed"}
	out, err := tmpl.Render(Options{Data: map[string]interface{}{"token": "a&b"}, Settings: map[string]interface{}{"title": "<hello>"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`action="/subscribe"`, `data-action="subscribe"`, `name="authenticity_token" value="a&amp;b"`, `&lt;hello&gt;`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if _, err := tmpl.Render(Options{}); err == nil {
		t.Fatal("missing host token must fail")
	}
}

func TestHostRejectsUnsafeFormProfiles(t *testing.T) {
	for _, path := range []string{"", "//evil.example/x", "https://evil.example/x", "/\\evil", "/x\n"} {
		_, err := CompileWithOptions(`{% form "x" %}{% endform %}`, CompileOptions{Forms: map[string]FormDefinition{"x": {Path: path, TokenContextKey: "token", TokenField: "csrf"}}})
		if err == nil {
			t.Errorf("accepted path %q", path)
		}
	}
}
