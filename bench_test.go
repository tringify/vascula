package vascula

import (
	"fmt"
	"testing"
)

// Benchmarks for the workloads a shop front renders most: a product grid, a
// filter chain over a collection, and compilation. The wide variant gives each
// product 150 fields, as host records with metafields and variants do; filter
// cost must not grow with fields the template never reads.

func benchProducts(n, fields int) []interface{} {
	products := make([]interface{}, n)
	for i := range products {
		p := map[string]interface{}{
			"title":     fmt.Sprintf("Product %d", i),
			"url":       fmt.Sprintf("/products/p-%d", i),
			"price":     float64(1000 + i*7),
			"available": i%3 != 0,
			"vendor":    fmt.Sprintf("Vendor %d", i%5),
			"tags":      []interface{}{"new", "sale", fmt.Sprintf("t%d", i%4)},
		}
		for f := len(p); f < fields; f++ {
			p[fmt.Sprintf("field_%d", f)] = fmt.Sprintf("value %d of product %d", f, i)
		}
		products[i] = p
	}
	return products
}

const benchGrid = `<ul class="grid">{% for product in collection.products %}
<li class="card{% if forloop.first %} first{% endif %}">
  <a href="{{ product.url }}">{{ product.title | upcase }}</a>
  {% if product.available %}<span class="price">{{ product.price | divided_by: 100 }}</span>{% else %}<span>Sold out</span>{% endif %}
  {% for tag in product.tags %}<em>{{ tag }}</em>{% endfor %}
  <small>{{ product.vendor | default: "Acme" }}</small>
</li>{% endfor %}</ul>`

const benchChain = `{{ collection.products | where: "available" | sort: "price" | map: "title" | join: ", " }}`

func benchRender(b *testing.B, src string, products []interface{}) {
	tmpl, err := Compile(src)
	if err != nil {
		b.Fatal(err)
	}
	opts := Options{
		Data:  map[string]interface{}{"collection": map[string]interface{}{"products": products}},
		Allow: []string{"collection"},
	}
	if _, err := tmpl.Render(opts); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tmpl.Render(opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGrid48(b *testing.B)          { benchRender(b, benchGrid, benchProducts(48, 6)) }
func BenchmarkFilterChain(b *testing.B)     { benchRender(b, benchChain, benchProducts(48, 6)) }
func BenchmarkFilterChainWide(b *testing.B) { benchRender(b, benchChain, benchProducts(48, 150)) }

func BenchmarkCompileGrid(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Compile(benchGrid); err != nil {
			b.Fatal(err)
		}
	}
}
