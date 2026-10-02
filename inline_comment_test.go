package vascula

import "testing"

func TestInlineComments(t *testing.T) {
	c, err := compileFixture(`a{# hidden #}b{#- ws-trim left #}c
	{#- also trims -#}   d`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Render(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if out != "abcd" {
		t.Fatalf("comments must vanish (with -trims): got %q", out)
	}
	if _, err := compileFixture(`x {# never closed`); err == nil {
		t.Fatal("unterminated {# must be a compile error")
	}
	// {# inside literal text only starts a comment at the opener — content with
	// single braces is untouched.
	out2, _ := mustRender(t, `{ "a": 1 } {# gone #}ok`)
	if out2 != `{ "a": 1 } ok` {
		t.Fatalf("plain braces untouched, got %q", out2)
	}
}

func mustRender(t *testing.T, tpl string) (string, error) {
	t.Helper()
	c, err := compileFixture(tpl)
	if err != nil {
		t.Fatal(err)
	}
	return c.Render(Options{})
}
