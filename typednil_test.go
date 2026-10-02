package vascula

import "testing"

// A typed-nil map/slice in an interface must be FALSY: hosts build data values
// via helpers returning map[string]interface{} — absent data comes through as
// a typed nil, and {% if product.price %} must guard it (the AED listing 500).
func TestTypedNilFalsy(t *testing.T) {
	tmpl, err := compileFixture(`{% if price %}{{ price.current | upcase }}{% else %}none{% endif %}`)
	if err != nil {
		t.Fatal(err)
	}
	var nilMap map[string]interface{}
	out, err := tmpl.Render(Options{Data: map[string]interface{}{"price": nilMap}, Allow: []string{"price"}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "none" {
		t.Fatalf("typed-nil map must be falsy, got %q", out)
	}
	var nilSlice []interface{}
	tmpl2, _ := compileFixture(`{% if items %}yes{% else %}no{% endif %}`)
	out2, err := tmpl2.Render(Options{Data: map[string]interface{}{"items": nilSlice}, Allow: []string{"items"}})
	if err != nil {
		t.Fatal(err)
	}
	if out2 != "no" {
		t.Fatalf("typed-nil slice must be falsy, got %q", out2)
	}
	// Empty-but-present containers stay TRUTHY.
	out3, _ := tmpl2.Render(Options{Data: map[string]interface{}{"items": []interface{}{}}, Allow: []string{"items"}})
	if out3 != "yes" {
		t.Fatalf("empty non-nil slice must stay truthy, got %q", out3)
	}
}
