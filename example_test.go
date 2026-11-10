package vascula_test

import (
	"fmt"
	"log"

	"vascula.dev/vascula"
)

func ExampleCompile() {
	template, err := vascula.Compile(`<h1>{{ settings.title }}</h1>{% for item in catalog.items %}<p>{{ item.name }}</p>{% endfor %}`)
	if err != nil {
		log.Fatal(err)
	}
	html, err := template.Render(vascula.Options{
		Allow:    []string{"catalog"},
		Settings: map[string]interface{}{"title": "Tea & coffee"},
		Data: map[string]interface{}{
			"catalog": map[string]interface{}{
				"items": []interface{}{map[string]interface{}{"name": "Ceramic cup"}},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(html)
	// Output: <h1>Tea &amp; coffee</h1><p>Ceramic cup</p>
}

func ExampleCompileWithOptions() {
	template, err := vascula.CompileWithOptions(`{% form "subscribe" %}<button>Subscribe</button>{% endform %}`, vascula.CompileOptions{
		Forms: map[string]vascula.FormDefinition{
			"subscribe": {Path: "/subscribe", TokenContextKey: "token", TokenField: "authenticity_token"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	html, err := template.Render(vascula.Options{Data: map[string]interface{}{"token": "example-token"}})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(html)
	// Output: <form method="post" action="/subscribe"><input type="hidden" name="authenticity_token" value="example-token"><button>Subscribe</button></form>
}
