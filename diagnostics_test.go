package vascula

import (
	"strings"
	"testing"
)

// VN5: compile errors read like an editor: line and character column, token
// names instead of internal numbers, and unclosed blocks point at their opening.
func TestCompileErrorsNameLineAndColumn(t *testing.T) {
	for src, want := range map[string]string{
		"<p>\n{{ product.title | }}":                     `expected name, found "}}" (line 2, col 20)`,
		"{% for item in items %}\n<li>\n":                `{% for %} is never closed with {% endfor %} (line 1, col 1)`,
		"ok\n  {% if a %}\n{% for x in y %}{% endfor %}": `{% if %} is never closed with {% endif %} (line 2, col 3)`,
		"{% assign x 5 %}":                               `expected "=", found number "5" (line 1, col 13)`,
		"{% unknown %}":                                  `unknown tag "unknown" (line 1, col 4)`,
		"héllo wörld {{ ! }}":                            `unexpected '!' (line 1, col 16)`,
		"{% endif %}":                                    `unexpected {% endif %} with no opening tag (line 1, col 1)`,
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", src, err, want)
		}
	}
}
