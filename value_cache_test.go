package vascula

import (
	"strings"
	"testing"
)

// The value walk is cached per render . These pin that caching changes
// only speed: every limit and the step charge stay what a fresh walk gives.

func TestCachedValuesStillChargeTheirSteps(t *testing.T) {
	big := benchProducts(200, 40)
	tmpl, err := Compile(`{% for i in n %}{% assign s = data | where: "available" | map: "title" | join: "," %}{% endfor %}done`)
	if err != nil {
		t.Fatal(err)
	}
	n := make([]interface{}, 40)
	opts := Options{Data: map[string]interface{}{"data": big, "n": n}, Allow: []string{"data", "n"}, MaxSteps: 5_000_000}
	out, err := tmpl.Render(opts)
	if err != nil || out != "done" {
		t.Fatalf("within a large budget: %q, %v", out, err)
	}
	// One pass fits a small budget; forty passes must not, cached or not.
	opts.MaxSteps = 30_000
	if _, err := tmpl.Render(opts); err == nil || !strings.Contains(err.Error(), "too many steps") {
		t.Fatalf("repeated use of a cached value escaped the step budget: %v", err)
	}
}

func TestCachedValuesKeepNodeAndByteLimits(t *testing.T) {
	products := benchProducts(10, 10)
	tmpl, err := Compile(`{{ a | json | size }}{{ a | concat: a | concat: a | json | size }}`)
	if err != nil {
		t.Fatal(err)
	}
	// a alone fits; three copies of it in one value do not.
	_, err = tmpl.Render(Options{Data: map[string]interface{}{"a": products}, Allow: []string{"a"}, MaxValueNodes: 200})
	if err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("a cached subtree must still count toward the node limit: %v", err)
	}
	long := strings.Repeat("x", 600)
	strs := []interface{}{long, long}
	tmpl, _ = Compile(`{{ s | json | size }}{{ s | concat: s | json | size }}`)
	_, err = tmpl.Render(Options{Data: map[string]interface{}{"s": strs}, Allow: []string{"s"}, MaxValueBytes: 2000})
	if err == nil || !strings.Contains(err.Error(), "value too large") {
		t.Fatalf("a cached subtree must still count toward the byte limit: %v", err)
	}
}

func TestCachedValuesKeepTheDepthLimit(t *testing.T) {
	// A value 60 deep is fine at the top and too deep inside 5 more levels.
	var deep interface{} = "leaf"
	for i := 0; i < 60; i++ {
		deep = []interface{}{deep}
	}
	var wrapped interface{} = deep
	for i := 0; i < 5; i++ {
		wrapped = []interface{}{wrapped}
	}
	tmpl, _ := Compile(`{{ d | json | size }}{{ w | json | size }}`)
	_, err := tmpl.Render(Options{Data: map[string]interface{}{"d": deep, "w": wrapped}, Allow: []string{"d", "w"}})
	if err == nil || !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("a cached subtree reused deeper must still hit the depth limit: %v", err)
	}
}

// Shallow filters bound a collection by length; whatever they hand on is
// still fully checked by the first deep consumer, and where/sort check the
// fields they compare.
func TestShallowFiltersStillBoundWhatIsConsumed(t *testing.T) {
	huge := strings.Repeat("x", 3000)
	items := []interface{}{map[string]interface{}{"k": huge, "ok": true}}
	opts := Options{Data: map[string]interface{}{"items": items}, Allow: []string{"items"}, MaxValueBytes: 1000}
	for _, src := range []string{
		`{{ items | where: "ok" | first | json }}`, // json consumes the whole entry
		`{{ items | where: "k", "y" | size }}`,     // where compares the oversized field
		`{{ items | sort: "k" | size }}`,           // sort compares the oversized field
		`{{ items | map: "k" | join: "," }}`,       // join consumes the mapped field
	} {
		tmpl, err := Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpl.Render(opts); err == nil || !strings.Contains(err.Error(), "value too large") {
			t.Errorf("%s: %v", src, err)
		}
	}
	// Reordering and counting never read the oversized field, so they pass.
	if out := renderWith(t, `{{ items | reverse | size }}`, opts); out != "1" {
		t.Errorf("got %q", out)
	}
}
