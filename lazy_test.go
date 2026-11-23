package vascula

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// lazyProduct is a product whose vendor and count are Lazy, counting loads.
func lazyProduct(loads *int) map[string]interface{} {
	count := NewLazy(func() (interface{}, error) {
		*loads++
		return int64(7), nil
	})
	vendor := NewLazy(func() (interface{}, error) {
		*loads++
		return map[string]interface{}{"name": "Acme", "product_count": count}, nil
	})
	return map[string]interface{}{"title": "Mug", "vendor_ref": vendor}
}

func renderLazy(t *testing.T, src string, opts Options) string {
	t.Helper()
	tmpl, err := Compile(src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	out, err := tmpl.Render(opts)
	if err != nil {
		t.Fatalf("render %q: %v", src, err)
	}
	return out
}

func TestLazyComputedOnlyWhenUsed(t *testing.T) {
	loads := 0
	out := renderLazy(t, `{{ product.title }}{% if product.title == "Mug" %}!{% endif %}{{ product | size }}`,
		Options{Data: map[string]interface{}{"product": lazyProduct(&loads)}, Allow: []string{"product"}})
	if out != "Mug!2" || loads != 0 {
		t.Fatalf("out=%q loads=%d, want Mug!2 and no load", out, loads)
	}
}

// Every way a template reaches a value sees the computed value.
func TestLazyEveryAccessPath(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"property", `{{ product.vendor_ref.name }}`, "Acme"},
		{"nested lazy", `{{ product.vendor_ref.product_count | plus: 1 }}`, "8"},
		{"quoted index", `{{ product["vendor_ref"]["product_count"] }}`, "7"},
		{"computed index", `{% assign k = "vendor_ref" %}{{ product[k].name }}`, "Acme"},
		{"setting index", `{{ product[settings.field].name }}`, "Acme"},
		{"quoted delimiter", `{{ '}}' | append: product.vendor_ref.name }}`, "}}Acme"},
		{"alias", `{% assign countries = product %}{{ countries.vendor_ref.name }}`, "Acme"},
		{"whole object", `{{ product.vendor_ref }}`, "map[name:Acme product_count:7]"},
		{"json", `{{ product | json }}`, `{"title":"Mug","vendor_ref":{"name":"Acme","product_count":7}}`},
		{"json alias", `{% assign countries = product %}{{ countries | json }}`, `{"title":"Mug","vendor_ref":{"name":"Acme","product_count":7}}`},
		{"pair last", `{% for pair in product %}{% if pair.first == settings.field %}{{ pair.last.product_count }}{% endif %}{% endfor %}`, "7"},
		{"pair index", `{% for pair in product.vendor_ref %}{{ pair[0] }}={{ pair[1] }};{% endfor %}`, "name=Acme;product_count=7;"},
		{"pair whole", `{% for pair in product.vendor_ref %}{{ pair | json }}{% endfor %}`, `["name","Acme"]["product_count",7]`},
		{"truthy", `{% if product.vendor_ref %}yes{% endif %}{% unless product.vendor_ref.product_count %}no{% endunless %}`, "yes"},
		{"compare", `{% if product.vendor_ref.product_count > 5 %}many{% endif %}`, "many"},
		{"equal whole", `{% assign v = product.vendor_ref %}{% if v == product.vendor_ref %}same{% endif %}`, "same"},
		{"case", `{% case product.vendor_ref.product_count %}{% when 7 %}seven{% endcase %}`, "seven"},
		{"default", `{{ product.vendor_ref.product_count | default: 0 }}`, "7"},
		{"map filter", `{{ list | map: settings.count | join: "," }}`, "7,7"},
		{"where filter", `{{ list | where: "product_count", 7 | size }}`, "2"},
		{"sort filter", `{{ list | sort: "product_count" | first | json }}`, `{"name":"Acme","product_count":7}`},
		{"loop item", `{% for v in list %}{{ v.name }}{{ v.product_count }}{% endfor %}`, "Acme7Acme7"},
		{"loop whole", `{% for v in list %}{{ v | json }}{% endfor %}`, `{"name":"Acme","product_count":7}{"name":"Acme","product_count":7}`},
		{"first last", `{{ list.first.product_count }}{{ list.last.name }}{{ list[0].product_count }}`, "7Acme7"},
		{"contains", `{% if list contains product.vendor_ref %}in{% endif %}`, "in"},
		{"capture", `{% capture s %}{{ product.vendor_ref.name }}{% endcapture %}{{ s }}`, "Acme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loads := 0
			product := lazyProduct(&loads)
			list := []interface{}{product["vendor_ref"], product["vendor_ref"]}
			out := renderLazy(t, tc.src, Options{
				Data:     map[string]interface{}{"product": product, "list": list},
				Allow:    []string{"product", "list"},
				Settings: map[string]interface{}{"field": "vendor_ref", "count": "product_count"},
			})
			if out != tc.want {
				t.Fatalf("got %q, want %q", out, tc.want)
			}
			if loads > 2 {
				t.Fatalf("loads=%d: each Lazy computes at most once", loads)
			}
		})
	}
}

func TestLazyInSettingsGlobalsChildrenAndRoots(t *testing.T) {
	loads := 0
	child, err := Compile(`{{ v.name }}/{{ settings.vendor.product_count }}`)
	if err != nil {
		t.Fatal(err)
	}
	product := lazyProduct(&loads)
	out := renderLazy(t, `{{ settings.vendor.name }}|{{ shop.vendor.product_count }}|{{ root.title }}|{% render "c", v: product.vendor_ref %}`, Options{
		Data:     map[string]interface{}{"product": product, "root": NewLazy(func() (interface{}, error) { return map[string]interface{}{"title": "T"}, nil })},
		Allow:    []string{"product", "root"},
		Settings: map[string]interface{}{"vendor": product["vendor_ref"]},
		Globals:  map[string]interface{}{"shop": map[string]interface{}{"vendor": product["vendor_ref"]}},
		Resolve: func(string) (Child, error) {
			return Child{Template: child, Settings: map[string]interface{}{"vendor": product["vendor_ref"]}}, nil
		},
	})
	if out != "Acme|7|T|Acme/7" {
		t.Fatalf("got %q", out)
	}
}

// Host filters and host tags receive values with every Lazy computed.
func TestLazyReachesHostCodeComputed(t *testing.T) {
	loads := 0
	product := lazyProduct(&loads)
	var filtered, tagged interface{}
	tmpl, err := CompileWithOptions(`{{ product | inspect }}{% probe product %}`, CompileOptions{Tags: map[string]TagDefinition{"probe": {Argument: TagRequiredArgument}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tmpl.Render(Options{
		Data:    map[string]interface{}{"product": product},
		Allow:   []string{"product"},
		Filters: map[string]FilterFunc{"inspect": func(in interface{}, _ FilterArgs) (interface{}, error) { filtered = in; return "", nil }},
		Tags:    map[string]TagFunc{"probe": func(c TagCall) (SafeHTML, error) { tagged = c.Arg; return "", nil }},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]interface{}{"filter": filtered, "tag": tagged} {
		b, _ := json.Marshal(got)
		if string(b) != `{"title":"Mug","vendor_ref":{"name":"Acme","product_count":7}}` {
			t.Fatalf("%s received %s", name, b)
		}
		if strings.Contains(stringOf(got), "Lazy") {
			t.Fatalf("%s received a Lazy: %#v", name, got)
		}
	}
	if product["vendor_ref"].(*Lazy) == nil {
		t.Fatal("host data was modified")
	}
}

func stringOf(v interface{}) string {
	b := &strings.Builder{}
	var walk func(interface{})
	walk = func(v interface{}) {
		switch x := v.(type) {
		case *Lazy:
			b.WriteString("Lazy")
		case map[string]interface{}:
			for _, item := range x {
				walk(item)
			}
		case []interface{}:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(v)
	return b.String()
}

func TestLazyLoadErrorFailsRender(t *testing.T) {
	boom := errors.New("count unavailable")
	data := map[string]interface{}{"e": map[string]interface{}{"n": NewLazy(func() (interface{}, error) { return nil, boom })}}
	for _, src := range []string{`{{ e.n }}`, "\n{{ e | json }}", `{% for p in e %}{{ p.last }}{% endfor %}`, `{% if e == 1 %}{% endif %}`} {
		tmpl, err := Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tmpl.Render(Options{Data: data, Allow: []string{"e"}})
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "line ") {
			t.Fatalf("%q: err=%v, want the load error with a position", src, err)
		}
	}
}

func TestResolveTypedContainersAndJSON(t *testing.T) {
	n := NewLazy(func() (interface{}, error) { return 3.0, nil })
	none := NewLazy(func() (interface{}, error) { return nil, nil })
	in := map[string]interface{}{
		"typed":  []map[string]interface{}{{"n": n}},
		"lazies": map[string]*Lazy{"n": n, "none": none},
		"array":  [1]interface{}{n},
		"chain":  NewLazy(func() (interface{}, error) { return n, nil }),
	}
	out, err := Resolve(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"array":[3],"chain":3,"lazies":{"n":3,"none":null},"typed":[{"n":3}]}`; string(b) != want {
		t.Fatalf("Resolve = %s, want %s", b, want)
	}
	if strings.Contains(stringOf(out), "Lazy") || stringOf(in) == "" {
		t.Fatal("Resolve must compute a copy and leave its input as it was")
	}
	if v, err := in["chain"].(*Lazy).Value(); err != nil || v != 3.0 {
		t.Fatalf("Value follows a chain: %v, %v", v, err)
	}
	if v, err := (*Lazy)(nil).Value(); err != nil || v != nil {
		t.Fatalf("nil Lazy: %v, %v", v, err)
	}
	direct, err := json.Marshal(in)
	if err != nil || string(direct) != string(b) {
		t.Fatalf("json.Marshal of host data = %s, %v", direct, err)
	}
}

func TestLazySelfReferenceIsAnError(t *testing.T) {
	var l *Lazy
	l = NewLazy(func() (interface{}, error) { return l, nil })
	tmpl, _ := Compile(`{{ x }}`)
	if _, err := tmpl.Render(Options{Data: map[string]interface{}{"x": l}, Allow: []string{"x"}}); err == nil {
		t.Fatal("a Lazy resolving to itself must fail the render")
	}
}

// Filters that read entries compute only the entries and the field they read:
// their cost does not grow with fields a template never reads.
func TestLazyEntryFiltersComputeOnlyWhatTheyRead(t *testing.T) {
	for _, src := range []string{
		`{{ list | map: "name" | join: "," }}`,
		`{{ list | where: "name", "Acme" | size }}`,
		`{% assign v = list | sort: "name" | first %}{{ v.name }}`,
		`{{ list | reverse | concat: list | size }}`,
	} {
		loads := 0
		product := lazyProduct(&loads)
		list := []interface{}{product["vendor_ref"], product["vendor_ref"]}
		renderLazy(t, src, Options{Data: map[string]interface{}{"list": list}, Allow: []string{"list"}})
		if loads != 1 {
			t.Fatalf("%s: loads=%d, want the entries only (not their product_count)", src, loads)
		}
	}
}
