package vascula

import (
	"strings"
	"testing"
)

func TestOutputSpansDescribeOnlyDirectVisibleOutputs(t *testing.T) {
	source := `<p title="{{ settings.title }}">{{ settings.title }}</p><script>{{ settings.title }}</script><style>{{ settings.title }}</style><textarea>{{ settings.title }}</textarea><h2>{{ settings.title | upcase }}</h2>{% capture copy %}{{ settings.title }}{% endcapture %}{{ copy }}`
	tmpl, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	var spans []OutputSpan
	result, err := tmpl.Render(Options{Settings: map[string]interface{}{"title": "A & 雨"}, ObserveOutput: func(span OutputSpan) { spans = append(spans, span) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 {
		t.Fatalf("unexpected bindings: %+v", spans)
	}
	span := spans[0]
	if strings.Join(span.Path, ".") != "settings.title" || result[span.Start:span.End] != "A &amp; 雨" {
		t.Fatalf("bad byte span: %+v %s", span, result)
	}
	plain, err := tmpl.Render(Options{Settings: map[string]interface{}{"title": "A & 雨"}})
	if err != nil || plain != result {
		t.Fatal("observer changed rendering")
	}
}
func TestOutputSpansExcludeChildSettingsAndIdentifyLoopOwner(t *testing.T) {
	tmpl, _ := Compile(`{% for item in settings.blocks %}{{ item.settings.text }}{% endfor %}{% render 'child' %}`)
	child, _ := Compile(`{{ settings.text }}`)
	var spans []OutputSpan
	result, err := tmpl.Render(Options{Settings: map[string]interface{}{"blocks": []interface{}{map[string]interface{}{"id": "block-one", "settings": map[string]interface{}{"text": "One"}}}}, Resolve: func(string) (Child, error) {
		return Child{Template: child, Settings: map[string]interface{}{"text": "Child"}}, nil
	}, ObserveOutput: func(span OutputSpan) { spans = append(spans, span) }})
	if err != nil || result != "OneChild" || len(spans) != 1 {
		t.Fatalf("%s %+v %v", result, spans, err)
	}
	if spans[0].RootValue.(map[string]interface{})["id"] != "block-one" {
		t.Fatal("lost block identity")
	}
}

func TestOutputSpansIncludeDefaultWhenTheValueIsPrinted(t *testing.T) {
	tmpl, err := Compile(`<h1>{{ settings.heading | default: shop }}</h1><p>{{ settings.empty | default: 'Fallback' }}</p><p>{{ settings.heading | default: 'x' | upcase }}</p>`)
	if err != nil {
		t.Fatal(err)
	}
	var spans []OutputSpan
	result, err := tmpl.Render(Options{
		Settings: map[string]interface{}{"heading": "Useful objects", "empty": ""},
		Data:     map[string]interface{}{"shop": "Store"}, Allow: []string{"shop"},
		ObserveOutput: func(span OutputSpan) { spans = append(spans, span) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || strings.Join(spans[0].Path, ".") != "settings.heading" || result[spans[0].Start:spans[0].End] != "Useful objects" {
		t.Fatalf("spans %+v in %s", spans, result)
	}
}
