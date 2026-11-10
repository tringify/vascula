package vascula

import (
	"reflect"
	"testing"
)

// A host can learn at compile time every name a template reads but never
// defines, and refuse it before anyone renders it.
func TestExternalNames(t *testing.T) {
	tmpl, err := Compile(`{% assign total = 0 %}
{% for item in cart.items %}{% assign total = total | plus: item.price %}{{ forloop.index }}{% endfor %}
{% capture label %}{{ settings.title }} {{ site.settings.accent }}{% endcapture %}
<form><input name="token" value="{{ csrf_token }}"></form>
{% if customer and cart.items.size > 0 %}{{ products[product_key] | img_url: size }}{% endif %}
{% for i in (1..limit) %}{{ i }}{% endfor %}{{ forloop }}{{ total }}{{ label }}{{ cart }}`)
	if err != nil {
		t.Fatal(err)
	}
	got := tmpl.ExternalNames("site")
	want := []NameUse{
		{"cart", 2, 16},
		{"csrf_token", 4, 37},
		{"customer", 5, 7},
		{"products", 5, 45},
		{"product_key", 5, 54},
		{"size", 5, 78},
		{"limit", 6, 17},
		{"forloop", 6, 48},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestExternalNamesSeeRenderArgumentsAndBlocks(t *testing.T) {
	tmpl, _ := CompileWithOptions(`{% render "badge", label: heading, size: 2 %}{% blocks block_ref %}{% cycle first, "b" %}`, CompileOptions{Tags: map[string]TagDefinition{"blocks": {Argument: TagOptionalArgument}}})
	var names []string
	for _, use := range tmpl.ExternalNames() {
		names = append(names, use.Name)
	}
	if !reflect.DeepEqual(names, []string{"heading", "block_ref", "first"}) {
		t.Errorf("got %v", names)
	}
}

func TestRenderArguments(t *testing.T) {
	tmpl, _ := Compile(`{% render "a", x: 1 %}{% if c %}{% render "a", y: 2, x: 3 %}{% endif %}{% for i in (1..2) %}{% render "b" %}{% endfor %}`)
	got := tmpl.RenderArguments()
	want := map[string][]string{"a": {"x", "y"}, "b": {}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}
