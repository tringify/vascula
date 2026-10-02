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
