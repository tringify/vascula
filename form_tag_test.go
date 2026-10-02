package vascula

import (
	"strings"
	"testing"
)

func renderForTest(t *testing.T, src string, ctx map[string]interface{}, allowed []string) (string, error) {
	t.Helper()
	tmpl, err := compileFixture(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return tmpl.Render(Options{Data: ctx, Allow: allowed})
}

func TestFormTagEmitsActionCSRFAndAttrs(t *testing.T) {
	out, err := renderForTest(t,
		`{% form "save", class: "buy-form" %}<button>Add</button>{% endform %}`,
		map[string]interface{}{"token": `tok"1`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form method="post" action="/save" data-action="save" class="buy-form">`,
		`<input type="hidden" name="authenticity_token" value="tok&#34;1">`,
		`<button>Add</button>`,
		`</form>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestFormTagMultipartForUploads(t *testing.T) {
	out, err := renderForTest(t, `{% form "upload" %}{% endform %}`,
		map[string]interface{}{"token": "tok"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `enctype="multipart/form-data"`) {
		t.Fatalf("multipart enctype missing: %q", out)
	}
}

func TestFormTagUnknownKindFailsCompile(t *testing.T) {
	if _, err := compileFixture(`{% form "teleport" %}{% endform %}`); err == nil {
		t.Fatal("unknown action kind must fail at compile")
	}
}

func TestFormTagDynamicKindFailsCompile(t *testing.T) {
	if _, err := compileFixture(`{% form settings.kind %}{% endform %}`); err == nil {
		t.Fatal("dynamic action kind must fail at compile")
	}
}

func TestFormTagMissingCSRFFailsRender(t *testing.T) {
	if _, err := renderForTest(t, `{% form "save" %}{% endform %}`, map[string]interface{}{}, nil); err == nil {
		t.Fatal("missing token must fail loud, not emit a tokenless form")
	}
}

func TestUsesFormDetection(t *testing.T) {
	with, _ := compileFixture(`{% if a %}{% form "login" %}x{% endform %}{% endif %}`)
	if !with.UsesForm() {
		t.Fatal("UsesForm must see nested form tags")
	}
	without, _ := compileFixture(`{{ shop.name }}`)
	if without.UsesForm() {
		t.Fatal("false positive")
	}
}

func TestFormBodyRenderTargetsWalked(t *testing.T) {
	tmpl, err := compileFixture(`{% form "save" %}{% render "child-snippet" %}{% endform %}`)
	if err != nil {
		t.Fatal(err)
	}
	targets := tmpl.RenderTargets()
	if len(targets) != 1 || targets[0] != "child-snippet" {
		t.Fatalf("form body render targets not walked: %v", targets)
	}
}
