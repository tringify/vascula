package vascula

import (
	"math"
	"strings"
	"testing"
)

func TestOversizedFilterCountsReturnErrors(t *testing.T) {
	for _, source := range []string{
		`{{ settings.seed | truncate: settings.count }}`,
		`{{ settings.seed | truncatewords: settings.count }}`,
		`{{ settings.seed | slice: 2048, settings.count }}`,
	} {
		for _, count := range []float64{9223372036854775808, math.Inf(1), math.NaN()} {
			t.Run(source+":"+toString(count), func(t *testing.T) {
				defer func() {
					if failure := recover(); failure != nil {
						t.Errorf("author-controlled count panicked: %v", failure)
					}
				}()
				tmpl, err := Compile(source)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tmpl.Render(Options{Settings: map[string]interface{}{"seed": strings.Repeat("x ", 4096), "count": count}}); err == nil {
					t.Fatal("out-of-range count must return an error")
				}
			})
		}
	}
}

func TestSliceLengthCannotOverflowEndIndex(t *testing.T) {
	defer func() {
		if failure := recover(); failure != nil {
			t.Fatalf("slice length overflow panicked: %v", failure)
		}
	}()
	tmpl, err := Compile(`{{ settings.seed | slice: 2048, 9223372036854774784 }}`)
	if err != nil {
		t.Fatal(err)
	}
	output, err := tmpl.Render(Options{Settings: map[string]interface{}{"seed": strings.Repeat("x", 4096)}})
	if err != nil {
		t.Fatal(err)
	}
	if output != strings.Repeat("x", 2048) {
		t.Fatal("large finite length should clamp to the remaining string")
	}
}

func TestCompileRejectsPathologicalStructure(t *testing.T) {
	for name, source := range map[string]string{
		"source bytes":   strings.Repeat("x", (1<<20)+1),
		"nested blocks":  strings.Repeat("{% if true %}", 140) + "ok" + strings.Repeat("{% endif %}", 140),
		"nested indexes": "{{ " + strings.Repeat("settings[", 140) + "'x'" + strings.Repeat("]", 140) + " }}",
		"boolean chain":  "{{ " + strings.Repeat("false or ", 300) + "true }}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(source); err == nil || !strings.Contains(err.Error(), "budget") {
				t.Fatalf("expected compilation budget error, got %v", err)
			}
		})
	}
}

func TestUnprintedTemporaryValuesAreBounded(t *testing.T) {
	cases := []struct {
		name, source string
		settings     map[string]interface{}
	}{
		{"append", "{% assign s = settings.seed %}{% for item in settings.items %}{% assign s = s | append: s %}{% endfor %}ok", map[string]interface{}{"seed": strings.Repeat("x", 128*1024), "items": []interface{}{1, 2, 3, 4, 5}}},
		{"replace", "{% assign s = settings.seed | replace: 'x', settings.replacement %}ok", map[string]interface{}{"seed": strings.Repeat("x", 512), "replacement": strings.Repeat("y", 8192)}},
		{"concat", "{% assign a = settings.items | concat: settings.items %}ok", map[string]interface{}{"items": make([]interface{}, 30000)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := Compile(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tmpl.Render(Options{Settings: tc.settings, MaxBytes: 64})
			if err == nil || !strings.Contains(err.Error(), "budget") {
				t.Fatalf("unprinted temporary escaped limits: %v", err)
			}
		})
	}
}

func TestFiltersConsumeEvaluationBudget(t *testing.T) {
	tmpl, err := Compile("{{ 0" + strings.Repeat(" | plus: 1", 200) + " }}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tmpl.Render(Options{MaxSteps: 10}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("filter pipeline escaped the evaluation budget: %v", err)
	}
}

func TestCompileLimitsCanBeConfiguredIndependently(t *testing.T) {
	cases := []struct {
		name, source string
		options      CompileOptions
		allowed      bool
	}{
		{"source at boundary", strings.Repeat("x", 64), CompileOptions{MaxSourceBytes: 64}, true},
		{"source over boundary", strings.Repeat("x", 65), CompileOptions{MaxSourceBytes: 64}, false},
		{"tokens at boundary", "{{ 1 }}", CompileOptions{MaxTokens: 4}, true},
		{"tokens over boundary", "{{ 1 }}", CompileOptions{MaxTokens: 3}, false},
		{"nesting at boundary", "{% if true %}x{% endif %}", CompileOptions{MaxNesting: 2}, true},
		{"nesting over boundary", "{% if true %}{% if true %}x{% endif %}{% endif %}", CompileOptions{MaxNesting: 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileWithOptions(tc.source, tc.options)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, got %v", tc.allowed, err)
			}
		})
	}
}

func TestValueLimitIsSeparateFromOutputLimit(t *testing.T) {
	tmpl, err := Compile(`{% assign doubled = settings.seed | append: settings.seed %}ok`)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{Settings: map[string]interface{}{"seed": strings.Repeat("x", 16)}, MaxBytes: 2, MaxValueBytes: 32}
	if output, err := tmpl.Render(options); err != nil || output != "ok" {
		t.Fatalf("valid intermediate: %q, %v", output, err)
	}
	options.MaxValueBytes = 31
	if _, err := tmpl.Render(options); err == nil {
		t.Fatal("temporary value exceeded its configured limit")
	}
}

func TestCyclicValuesReturnErrorsBeforeConversion(t *testing.T) {
	cyclic := map[string]interface{}{}
	cyclic["self"] = cyclic
	for _, source := range []string{`{{ settings.cyclic }}`, `{{ settings.cyclic | json }}`, `{{ settings.cyclic | join: ',' }}`} {
		tmpl, err := Compile(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpl.Render(Options{Settings: map[string]interface{}{"cyclic": cyclic}}); err == nil || !strings.Contains(err.Error(), "budget") {
			t.Fatalf("cyclic value: %v", err)
		}
	}
}

func TestCollectionFiltersBudgetTheirComparisons(t *testing.T) {
	items := make([]interface{}, 128)
	for i := range items {
		items[i] = 128 - i
	}
	for _, filter := range []string{"uniq", "sort", "sort_natural"} {
		tmpl, err := Compile(`{% assign sorted = settings.items | ` + filter + ` %}ok`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpl.Render(Options{Settings: map[string]interface{}{"items": items}, MaxSteps: 300}); err == nil || !strings.Contains(err.Error(), "budget") {
			t.Fatalf("%s comparisons escaped budget: %v", filter, err)
		}
	}
}

func TestFilterArgsOutOfBounds(t *testing.T) {
	args := FilterArgs{Pos: []interface{}{"first"}}
	if args.Arg(-1) != nil || args.Arg(1) != nil || args.Arg(0) != "first" {
		t.Fatal("arguments outside bounds should be absent")
	}
}

func TestSequenceLoopBudgetsConsumedFields(t *testing.T) {
	record := map[string]interface{}{"title": "Print", "unused": strings.Repeat("x", 100)}
	record["cycle"] = record
	items := []interface{}{record, record, record}
	for _, collection := range []interface{}{items, [3]interface{}{record, record, record}} {
		tmpl, err := Compile(`{% for item in settings.items %}{{ item.title }}{% endfor %}`)
		if err != nil {
			t.Fatal(err)
		}
		opts := Options{Settings: map[string]interface{}{"items": collection}, MaxValueNodes: 4, MaxValueBytes: 16}
		out, err := tmpl.Render(opts)
		if err != nil || out != "PrintPrintPrint" {
			t.Fatalf("selected fields: %q, %v", out, err)
		}
		for _, source := range []string{
			`{% for item in settings.items %}{{ item.unused }}{% endfor %}`,
			`{% for item in settings.items %}{{ item | json }}{% endfor %}`,
			`{% for item in settings.items %}{% assign data = item | json %}{% endfor %}`,
		} {
			tmpl, err := Compile(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tmpl.Render(opts); err == nil || !strings.Contains(err.Error(), "budget") {
				t.Fatalf("consumed value escaped budget: %s: %v", source, err)
			}
		}
	}
}

func TestSequenceLoopStillBoundsCollectionWork(t *testing.T) {
	items := make([]interface{}, 100)
	tmpl, err := Compile(`{% for item in settings.items limit: 1 %}ok{% endfor %}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []Options{
		{Settings: map[string]interface{}{"items": items}, MaxValueNodes: 100},
		{Settings: map[string]interface{}{"items": items}, MaxSteps: 50},
	} {
		if _, err := tmpl.Render(opts); err == nil || !strings.Contains(err.Error(), "budget") {
			t.Fatalf("collection work escaped budget: %v", err)
		}
	}
}

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
