package vascula

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentRendersKeepRequestDataIsolated(t *testing.T) {
	template, err := CompileWithOptions(`{% form "save" %}{{ account.name }}|{{ settings.label }}|{{ site.settings.title }}{% endform %}`, CompileOptions{
		Forms: map[string]FormDefinition{"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := fmt.Sprintf("request-%d", i)
			name := fmt.Sprintf("Customer %d", i)
			html, err := template.Render(Options{
				Data:  map[string]interface{}{"token": token, "account": map[string]interface{}{"name": name}},
				Allow: []string{"account"}, Settings: map[string]interface{}{"label": token},
				Globals: siteGlobals(map[string]interface{}{"title": name}),
			})
			if err != nil {
				t.Error(err)
				return
			}
			want := fmt.Sprintf(`<form method="post" action="/save"><input type="hidden" name="csrf" value="%s">%s|%s|%s</form>`, token, name, token, name)
			if html != want {
				t.Errorf("request %d: got %q, want %q", i, html, want)
			}
		}(i)
	}
	wg.Wait()
}

func TestFormTokenDoesNotGrantTemplateReadAccess(t *testing.T) {
	template, err := CompileWithOptions(`{% form "save" %}{{ token }}{% endform %}`, CompileOptions{
		Forms: map[string]FormDefinition{"save": {Path: "/save", TokenContextKey: "token", TokenField: "csrf"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := template.Render(Options{Data: map[string]interface{}{"token": "not-for-template-expressions"}})
	if err == nil || !strings.Contains(err.Error(), "undeclared name") || html != "" {
		t.Fatalf("token capability leaked: %q, %v", html, err)
	}
}
