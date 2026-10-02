package vascula

import (
	"sort"
	"testing"
)

func targetsOf(t *testing.T, src string) []string {
	t.Helper()
	tmpl, err := compileFixture(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out := tmpl.RenderTargets()
	sort.Strings(out)
	return out
}

func TestRenderTargets_FlatAndNested(t *testing.T) {
	// renders inside text, if, and for bodies all collected; deduped.
	src := `{% render 'header' %}
{% if x %}{% render 'badge' %}{% else %}{% render 'badge' %}{% endif %}
{% for i in items %}{% render 'line' %}{% endfor %}`
	got := targetsOf(t, src)
	want := []string{"badge", "header", "line"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestRenderTargets_None(t *testing.T) {
	if got := targetsOf(t, `{{ shop.name }}`); len(got) != 0 {
		t.Fatalf("no renders → empty, got %v", got)
	}
}
