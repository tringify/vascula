package vascula

import (
	"errors"
	"strings"
	"testing"
)

func TestHostTags(t *testing.T) {
	opts := CompileOptions{Tags: map[string]TagDefinition{
		"badge":   {},
		"zone":    {Argument: TagOptionalArgument},
		"require": {Argument: TagRequiredArgument},
	}}
	tmpl, err := CompileWithOptions(`<i>{% badge %}</i>{% zone %}{% for b in settings.list %}{% zone b %}{% endfor %}{% require settings.title | upcase %}`, opts)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	out, err := tmpl.Render(Options{
		Settings: map[string]interface{}{"title": "hi", "list": []interface{}{"x", "y"}},
		Tags: map[string]TagFunc{
			"badge": func(TagCall) (SafeHTML, error) { return "<b>new</b>", nil },
			"zone": func(c TagCall) (SafeHTML, error) {
				b, ok := c.Lookup("b")
				seen = append(seen, toString(c.Arg)+"/"+toString(b)+"/"+map[bool]string{true: "arg", false: "bare"}[c.HasArg]+"/"+map[bool]string{true: "b", false: "-"}[ok])
				return "", nil
			},
			"require": func(c TagCall) (SafeHTML, error) { return SafeHTML("[" + toString(c.Arg) + "]"), nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "<i><b>new</b></i>[HI]" {
		t.Errorf("out = %q", out)
	}
	if strings.Join(seen, " ") != "//bare/- x/x/arg/b y/y/arg/b" {
		t.Errorf("calls = %v", seen)
	}
	if !tmpl.UsesTag("zone") || tmpl.UsesTag("other") {
		t.Error("UsesTag")
	}
}

func TestHostTagRules(t *testing.T) {
	for src, tags := range map[string]map[string]TagDefinition{
		`{% badge %}`:         nil,                                          // undeclared
		`{% badge x %}`:       {"badge": {}},                                // takes no argument
		`{% require %}`:       {"require": {Argument: TagRequiredArgument}}, // needs one
		`{% if %}{% endif %}`: {"if": {}},                                   // cannot replace a built-in
		`{% Bad %}`:           {"Bad": {}},                                  // invalid name
	} {
		if _, err := CompileWithOptions(src, CompileOptions{Tags: tags}); err == nil {
			t.Errorf("%s with %v compiled", src, tags)
		}
	}
	tmpl, _ := CompileWithOptions(`{% badge %}`, CompileOptions{Tags: map[string]TagDefinition{"badge": {}}})
	if _, err := tmpl.Render(Options{}); err == nil {
		t.Error("a declared tag without an implementation rendered")
	}
	failing, _ := CompileWithOptions("\n {% badge %}", CompileOptions{Tags: map[string]TagDefinition{"badge": {}}})
	cause := errors.New("boom")
	_, err := failing.Render(Options{Tags: map[string]TagFunc{"badge": func(TagCall) (SafeHTML, error) { return "", cause }}})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "line 2, col 2") {
		t.Errorf("host tag error lost its cause or position: %v", err)
	}
}

func TestGlobalsAndReservedNames(t *testing.T) {
	child, _ := Compile(`{{ site.settings.brand }}`)
	parent, err := CompileWithOptions(`{{ site.settings.brand }}|{% render "c" %}`, CompileOptions{Reserved: []string{"site"}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := parent.Render(Options{Globals: siteGlobals(map[string]interface{}{"brand": "Acme"}), Resolve: func(string) (Child, error) { return Child{Template: child}, nil }})
	if err != nil || out != "Acme|Acme" {
		t.Fatalf("globals must reach children: %q, %v", out, err)
	}
	for _, src := range []string{
		`{% assign site = 1 %}`, `{% capture site %}x{% endcapture %}`, `{% for site in settings.l %}{% endfor %}`,
		`{% render "c", site: 1 %}`, `{% assign settings = 1 %}`, `{% capture forloop %}{% endcapture %}`,
	} {
		if _, err := CompileWithOptions(src, CompileOptions{Reserved: []string{"site"}}); err == nil {
			t.Errorf("%s compiled", src)
		}
	}
	var undeclared *UndeclaredNameError
	tmpl, _ := Compile(`{{ missing }}`)
	if _, err := tmpl.Render(Options{}); !errors.As(err, &undeclared) || undeclared.Name != "missing" {
		t.Errorf("want UndeclaredNameError, got %v", err)
	}
}
