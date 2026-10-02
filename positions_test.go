package vascula

import (
	"fmt"
	"strings"
	"testing"
)

// positions_test.go pins that RUNTIME errors carry a source position (line:col), so template authors can find their mistakes.
// Parse errors already had offsets; these are the eval-time errors.

func wantErrAt(t *testing.T, src string, opts Options, line, col int, substr string) {
	t.Helper()
	_, err := renderErr(src, opts)
	if err == nil {
		t.Fatalf("expected error for %q, got nil", src)
	}
	msg := err.Error()
	if !strings.Contains(msg, substr) {
		t.Fatalf("error %q should contain %q", msg, substr)
	}
	loc := fmt.Sprintf("line %d, col %d", line, col)
	if !strings.Contains(msg, loc) {
		t.Fatalf("error %q should contain position %q (src=%q)", msg, loc, src)
	}
}

func TestRuntimeErrorPositions(t *testing.T) {
	// undeclared name — line 1, the var starts at col 4 ({{ widget)
	wantErrAt(t, "{{ widget.title }}", Options{}, 1, 4, "undeclared name")

	// unknown filter
	wantErrAt(t, "{{ settings.x | nope }}", Options{Settings: m("x", "y")}, 1, 17, "unknown filter")

	// divide by zero — points at the FILTER (divided_by), not the input
	wantErrAt(t, "{{ settings.x | divided_by: 0 }}", Options{Settings: m("x", 10.0)}, 1, 17, "division by zero")

	// cross-type comparison — points at the operator
	wantErrAt(t, "{% if settings.n > settings.s %}Y{% endif %}",
		Options{Settings: mm("n", 5.0, "s", "x")}, 1, 18, "cannot compare")

	// position on a LATER line: error should report line 3
	src := "line one\nline two\n{{ widget.x }}"
	_, err := renderErr(src, Options{})
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("multiline error should report line 3; got %v", err)
	}
}
