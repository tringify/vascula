---
title: Extending Vascula
description: Add application filters, form actions, host tags and globals, and hand templates trusted HTML.
section: Go
order: 32
---

# Extending Vascula

The language stays small; applications add what they need. There are four
ways to extend it:

| Extension | Template sees | Declared |
|---|---|---|
| Filters | `{{ price \| money }}` | at render, `Options.Filters` |
| Globals | `{{ site.name }}` | at render, `Options.Globals` |
| Forms | `{% form "subscribe" %}...{% endform %}` | at compile, `CompileOptions.Forms` |
| Host tags | `{% badge product %}` | at compile, `CompileOptions.Tags`, implemented in `Options.Tags` |

Extensions are trusted code: they run with your application's permissions and
outside Vascula's limits, so keep them fast and bounded.

## Filters

A filter is a function from the piped value and its arguments to a new value.

```go run
package main

import (
	"fmt"
	"strings"

	"vascula.dev/vascula"
)

func main() {
	money := func(in interface{}, args vascula.FilterArgs) (interface{}, error) {
		cents, ok := in.(float64)
		if !ok {
			return nil, fmt.Errorf("expected a number of cents, got %T", in)
		}
		symbol := "€"
		if s, ok := args.NamedArg("symbol").(string); ok {
			symbol = s
		}
		return fmt.Sprintf("%s%.2f", symbol, cents/100), nil
	}
	shout := func(in interface{}, args vascula.FilterArgs) (interface{}, error) {
		return strings.ToUpper(fmt.Sprint(in)) + strings.Repeat("!", int(args.Arg(0).(float64))), nil
	}
	tmpl, _ := vascula.Compile(`{{ 1250 | money }} {{ 999 | money: symbol: "£" }} {{ "hi" | shout: 3 }}`)
	html, err := tmpl.Render(vascula.Options{
		Filters: map[string]vascula.FilterFunc{"money": money, "shout": shout},
	})
	fmt.Println(html, err)
}
```

```output
€12.50 £9.99 HI!!! <nil>
```

Things to know:

- **Numbers arrive as `float64`** when they fit exactly, so `1250` arrives as
  `float64(1250)`. Integers too large for a `float64` arrive as `int64`.
- **Return a value, not markup.** What you return is escaped when printed.
  Return `vascula.SafeHTML` only for markup you built safely yourself.
- **Errors fail the render** with your message, the filter's name and its
  position. Use them for misuse, not for missing data: return `nil` or `""`
  when a value is simply absent.
- An application filter **replaces a built-in** of the same name, except
  `escape`, `json` and `default`, which cannot be replaced.
- Unlike built-ins, application filters receive any named arguments the
  template passes; ignore the ones you do not use, or reject them.

## Globals

`Options.Globals` makes values readable everywhere, children included, without
an allowance. Reserve their names when compiling so templates cannot hide
them.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.CompileWithOptions(`{{ site.name }} — {{ settings.title }}`, vascula.CompileOptions{
		Reserved: []string{"site"},
	})
	if err != nil {
		panic(err)
	}
	html, _ := tmpl.Render(vascula.Options{
		Globals:  map[string]interface{}{"site": map[string]interface{}{"name": "Teahouse"}},
		Settings: map[string]interface{}{"title": "Menu"},
	})
	fmt.Println(html)

	_, err = vascula.CompileWithOptions(`{% assign site = "x" %}`, vascula.CompileOptions{Reserved: []string{"site"}})
	fmt.Println(err)
}
```

```output
Teahouse — Menu
assignment name "site" is reserved (line 1, col 11)
```

## Forms

`{% form "kind" %}` writes a `<form>` that posts to an action the application
registered. Templates cannot choose the URL; they choose one of the kinds you
registered. Each form carries a token from the data, for your
cross-site-request-forgery protection.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.CompileWithOptions(
		`{% form "subscribe", class: "signup" %}<input name="email"><button>Join</button>{% endform %}`,
		vascula.CompileOptions{Forms: map[string]vascula.FormDefinition{
			"subscribe": {
				Path:            "/newsletter",
				TokenContextKey: "csrf_token",
				TokenField:      "_csrf",
				ActionAttribute: "data-action",
			},
		}},
	)
	if err != nil {
		panic(err)
	}
	html, err := tmpl.Render(vascula.Options{
		Data: map[string]interface{}{"csrf_token": "t0k3n"},
	})
	fmt.Println(html, err)
}
```

```output
<form method="post" action="/newsletter" data-action="subscribe" class="signup"><input type="hidden" name="_csrf" value="t0k3n"><input name="email"><button>Join</button></form> <nil>
```

| Field | Meaning |
|---|---|
| `Path` | Where the form posts. Must be a local path starting with `/`. |
| `Multipart` | Adds `enctype="multipart/form-data"` for file uploads. |
| `TokenContextKey` | The `Data` root holding the token. The template does not need to be allowed to read it; the tag reads it directly. A missing token fails the render. |
| `TokenField` | The name of the hidden token field. |
| `ActionAttribute` | Optional. A `data-` attribute that receives the kind, for your scripts. |

A template may add other attributes (`class`, `id`, `data-*`), written in
alphabetical order. It cannot set `action`, `method`, `enctype` or the action
attribute. An unknown kind fails compilation. Your server must still verify the
token and validate everything posted.

## Host tags

A host tag adds a tag of your own: `{% name %}` or `{% name expression %}`.
Declare it when compiling, so a template using an unknown tag fails to compile,
and implement it when rendering.

```go run
package main

import (
	"fmt"
	"html"
	"strings"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.CompileWithOptions(
		`{% stars settings.rating %} {% byline %}`,
		vascula.CompileOptions{Tags: map[string]vascula.TagDefinition{
			"stars":  {Argument: vascula.TagRequiredArgument},
			"byline": {Argument: vascula.TagNoArgument},
		}},
	)
	if err != nil {
		panic(err)
	}
	out, err := tmpl.Render(vascula.Options{
		Settings: map[string]interface{}{"rating": 3, "author": "Ada & co"},
		Tags: map[string]vascula.TagFunc{
			"stars": func(call vascula.TagCall) (vascula.SafeHTML, error) {
				n, _ := call.Arg.(int)
				return vascula.SafeHTML(`<span aria-label="` + fmt.Sprint(n) + ` stars">` + strings.Repeat("★", n) + `</span>`), nil
			},
			"byline": func(call vascula.TagCall) (vascula.SafeHTML, error) {
				settings, _ := call.Lookup("settings")
				author := settings.(map[string]interface{})["author"]
				return vascula.SafeHTML("<em>by " + html.EscapeString(fmt.Sprint(author)) + "</em>"), nil
			},
		},
	})
	fmt.Println(out, err)
}
```

```output
<span aria-label="3 stars">★★★</span> <em>by Ada &amp; co</em> <nil>
```

| Argument rule | Accepts |
|---|---|
| `TagNoArgument` | `{% name %}` only |
| `TagOptionalArgument` | `{% name %}` or `{% name expression %}` |
| `TagRequiredArgument` | `{% name expression %}` only |

What the function receives:

- `call.Arg`: the evaluated argument, and `call.HasArg` whether one was written.
  A value from `Settings` keeps its Go type (`3` above is an `int`); values
  from `Data` decoded from JSON are `float64`.
- `call.Lookup(name)`: any name the template could read at that point, such
  as a loop variable or `settings`.

The returned `SafeHTML` is printed as-is: **escape everything you insert into
it**, as `byline` does. Tag names are lower case and cannot replace a built-in
tag. Children inherit the implementations, and `UsesTag(name)` tells you
whether a template uses a tag.

## Trusted HTML

Sometimes a template must print HTML: a sanitized product description, a
rendered Markdown page. Wrap such values in `vascula.SafeHTML` before putting
them in `Data`, `Settings` or `Globals`. Templates print them unescaped but
cannot create them; every other value is escaped.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, _ := vascula.Compile(`{{ settings.description }} {{ settings.note }}`)
	html, _ := tmpl.Render(vascula.Options{Settings: map[string]interface{}{
		"description": vascula.SafeHTML("<p>Hand-thrown <em>porcelain</em>.</p>"),
		"note":        "<em>escaped</em>",
	}})
	fmt.Println(html)
}
```

```output
<p>Hand-thrown <em>porcelain</em>.</p> &lt;em&gt;escaped&lt;/em&gt;
```

Mark a value as `SafeHTML` only when you produced it or ran it through an HTML
sanitizer. Vascula does not sanitize.
