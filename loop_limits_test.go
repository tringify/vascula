package vascula

import (
	"strings"
	"testing"
)

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
