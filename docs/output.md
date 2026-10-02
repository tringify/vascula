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

## What escaping does not cover

HTML escaping makes text safe as text and as a quoted attribute value. It does
not make a value safe as:

- **a URL.** `<a href="{{ settings.link }}">` still accepts `javascript:` links.
  Validate URLs in the application, or build them with an application filter.
- **CSS or JavaScript code.** Do not print values into `style="..."`,
  `onclick="..."` or the body of a `<script>`, other than through `json`.
- **an unquoted attribute.** Always quote attribute values: `class="{{ x }}"`,
  never `class={{ x }}`.

Vascula is not an HTML sanitizer. Anything the application passes in as
trusted HTML is its responsibility.

Next: [Filters](/filters).
