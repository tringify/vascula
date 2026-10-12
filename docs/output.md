---
title: Output and escaping
description: What Vascula escapes, where JSON is safe, trusted HTML, and what remains the template author's job.
section: Language
order: 17
---

# Output and escaping

## Everything printed is escaped

`{{ }}` escapes its value for HTML: `&`, `<`, `>`, `"` and `'` become
character references. This applies to every value, whatever its source.

```vascula
<p title="{{ settings.text }}">{{ settings.text }}</p>
```

```json data
{"settings": {"text": "<script>alert(\"hi\")</script> & 'more'"}}
```

```output
<p title="&lt;script&gt;alert(&#34;hi&#34;)&lt;/script&gt; &amp; &#39;more&#39;">&lt;script&gt;alert(&#34;hi&#34;)&lt;/script&gt; &amp; &#39;more&#39;</p>
```

There is **no way for a template to turn escaping off**: no `raw` filter, no
`{% raw %}` block, no "safe" marker. Literal text in the template itself is
printed as written, because the author wrote it.

## Escaping never happens twice

The `escape` filter returns text that is already escaped, so printing it does
not escape it again. `escape_once` escapes text that may already contain
character references, without doubling them.

```vascula
{{ settings.text | escape }} | {{ settings.encoded | escape_once }}
```

```json data
{"settings": {"text": "a < b", "encoded": "a &lt; b & c"}}
```

```output
a &lt; b | a &lt; b &amp; c
```

`newline_to_br` escapes the text and turns each line break into `<br>`:

```vascula
{{ settings.address | newline_to_br }}
```

```json data
{"settings": {"address": "1 Tea Street\n<Leeds>"}}
```

```output
1 Tea Street<br>&lt;Leeds&gt;
```

## JSON

`json` turns a value into JSON. Its result is safe in two places:

- **Inside `<script>`**, it is printed as JSON, with `<`, `>` and `&` encoded
  so the data cannot close the script element.
- **Inside an HTML tag** (an attribute value), it is additionally escaped for
  HTML, because JSON's quotes would otherwise end the attribute.

```vascula
<script>window.product = {{ settings.product | json }};</script>
<div data-product="{{ settings.product | json }}"></div>
```

```json data
{"settings": {"product": {"name": "Cup <large>", "price": 12}}}
```

```output
<script>window.product = {"name":"Cup \u003clarge\u003e","price":12};</script>
<div data-product="{&#34;name&#34;:&#34;Cup \u003clarge\u003e&#34;,&#34;price&#34;:12}"></div>
```

Vascula decides which case applies by reading the literal markup of the
template in source order. If a tag is opened in one branch of an `if` and
closed in another, it reads them in the order written.

## Trusted HTML comes only from the application

Rich text that has already been sanitized, such as a product description, needs
to be printed as HTML. Only the **application** can do that, by handing the
template a trusted value (`vascula.SafeHTML` in Go). A template can print it
but cannot create one.

Trusted HTML is markup for the page body. Inside a tag it is only text: in an
attribute value its tags are removed and the rest is escaped like any string,
and where an attribute goes it prints only as a plain attribute name. Markup
that adds attributes is a separate trusted value (`vascula.HTMLAttr` in Go),
which the application should only produce itself, such as a filter returning
`loading="lazy"`; it prints as written only where an attribute goes.

## Links and other URLs

Escaping keeps a value inside its attribute, but a value in a URL attribute
can still choose the URL's scheme, and `javascript:` runs when followed. When
anything printed stands in the value of a URL attribute, Vascula checks the
**whole value** once the attribute is complete: the literal text and the
printed parts together. It must be a relative URL (a path, `?query` or
`#fragment`) or use `http`, `https`, `mailto` or `tel`. Otherwise the whole
value prints as `#`.

```vascula
<a href="{{ settings.link }}">Ours</a> <a href="{{ settings.bad }}">Theirs</a>
```

```json data
{"settings": {"link": "/pages/about?from=footer", "bad": "javascript:alert(1)"}}
```

```output
<a href="/pages/about?from=footer">Ours</a> <a href="#">Theirs</a>
```

The check reads the value as a browser would: character references are
decoded, leading and trailing spaces and control characters are ignored, tabs
and line breaks are removed, and case does not matter. So
`href="&#32;{{ x }}"` with `javascript:...`, or `href="java{{ x }}"` with
`script:...`, prints `href="#"` too. A value after a fixed start is part of a
path, query or fragment and prints as written:

```vascula
<a href="/search?q={{ settings.q }}">Search</a>
```

```json data
{"settings": {"q": "tea:green"}}
```

```output
<a href="/search?q=tea:green">Search</a>
```

A URL attribute written entirely in the template is the author's and is not
checked.

The check applies to these attributes, in HTML and SVG: `href`, `src`,
`srcset`, `action`, `formaction`, `poster`, `data`, `cite`, `background`,
`ping`, `manifest`, `xmlns`, `xlink:href` and the obsolete `archive`,
`classid`, `codebase`, `icon`, `longdesc`, `profile` and `usemap`. As in Go's
`html/template`, a `data-` prefix is ignored (`data-src` is a source), and any
other attribute whose name contains `src`, `uri` or `url` is treated as a URL.

- **Images:** the `src` of `<img>` and `<source>` and `url()` in `style` also
  accept an image data URL (`data:image/...`). No other data URL is accepted.
- **`srcset`:** each image candidate is checked, read as browsers read
  `srcset`: a URL runs to the next space, so a comma inside it
  (`/image?crop=1,2 1x`) is part of it. An unsafe candidate URL prints as `#`
  and keeps its width or density descriptor.
- **`style`:** each `url(...)` argument in the value is checked. A value
  printed as the start of a `url()` argument also has its spaces, quotes,
  parentheses and backslashes percent-encoded, so it cannot end the `url()`.

## Where output can stand in a tag

- **Quoted attribute values** (`class="{{ x }}"`) are the place for output.
- **Unquoted attribute values** (`class={{ x }}`) work too: spaces, quotes,
  `` ` ``, `=`, `/`, `<` and `>` in the value print as character references, so
  the value cannot end early or add attributes.
- **Where an attribute goes** (`<img src="..." {{ image_attributes }}>`),
  attribute markup the application vouched for (`HTMLAttr`) prints as written.
  Trusted HTML does not count. Any other value must
  be one attribute name that is not an event handler, `style` or a URL
  attribute (`hidden`), and otherwise prints nothing.
- **Names** cannot be printed: `<h{{ n }}>`, `<a {{ name }}="...">`,
  `<a data-{{ key }}="...">`, and two values printed with no space between
  them in a tag fail to compile. Write names in the template and print values.

As in Go's `html/template`, compilation follows control flow. Every branch of
an `if`, `unless` or `case` must end in the same place in the markup (an `if`
without `else` must end where it started), a loop body must end where it
started, and `break` and `continue` must stand where their loop starts. So a
branch cannot open a tag or attribute value that another branch does not:

```vascula
<a href="{% if settings.local %}/pages/{% else %}https://example.com/{% endif %}{{ settings.page }}">About</a>
```

```json data
{"settings": {"local": true, "page": "about"}}
```

```output
<a href="/pages/about">About</a>
```

A name is never split by control flow. A branch may end inside a name when
the text after the branch ends it with a space, `/`, `=` or `>`:

```vascula
<input type="checkbox" {% if settings.on %}checked{% endif %}>
```

But text or output that would continue the name on one path only fails to
compile, as does a loop body or `break`/`continue` ending inside a name, or a
loop starting inside one:

```vascula
<a {% if settings.c %}h{% endif %}ref="{{ settings.link }}">
```

```error
would continue a name that control flow may end inside
```

A template, and a `capture` body, must not end inside a tag.

## What escaping does not cover

HTML escaping makes text safe as text and as a quoted attribute value. It does
not make a value safe as:

- **a URL in other places.** The URL check above covers attribute values.
  A URL printed into a `<script>`, an event handler or a `<meta>` refresh is
  not checked. Validate URLs in the application as well.
- **CSS or JavaScript code.** Do not print values into `style="..."`,
  `onclick="..."` or the body of a `<script>`, other than through `json`.

Vascula is not an HTML sanitizer. Anything the application passes in as
trusted HTML is its responsibility.

Next: [Filters](/filters).
