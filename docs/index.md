---
title: Vascula
description: A template language for HTML that is safe by default, explicit about data, and fast. Embedded in Go.
section: Start
order: 1
---

# Vascula

Vascula is a template language for HTML. You write markup with `{{ output }}`
and `{% tags %}`; an application compiles the template once and renders it
with data.

```vascula
<h1>{{ settings.title }}</h1>
{% if settings.items.size > 0 %}
  <ul>{% for item in settings.items %}<li>{{ item | upcase }}</li>{% endfor %}</ul>
{% else %}
  <p>Nothing yet.</p>
{% endif %}
```

```json data
{"settings": {"title": "Tea & coffee", "items": ["sencha", "<b>espresso</b>"]}}
```

```output
<h1>Tea &amp; coffee</h1>

  <ul><li>SENCHA</li><li>&lt;B&gt;ESPRESSO&lt;/B&gt;</li></ul>

```

The output is escaped for HTML automatically, so the `&` and the markup
inside the data cannot break the page.

## What makes it different

**Safe by default.** Everything you print is HTML-escaped. There is no `raw`
filter and no way for a template to switch escaping off; only the application
can hand over trusted HTML. JSON printed inside an HTML attribute is escaped
for that position too.

**Explicit about data.** A template can only read the data roots the
application allows. Reading anything else is an error, not a silent blank, and
the application can list every name a template reads before it ever renders.

**Bounded.** Compilation and rendering have limits on source size, nesting,
work, output and value size. A template cannot loop forever or build a
gigabyte string.

**Fast.** A 48-product grid renders in about 100 microseconds, three times
faster than the popular Liquid implementations in Go and Ruby. See
[Performance](/performance).

**Familiar.** The syntax will look familiar if you know Liquid or Jinja, but
Vascula is its own language with documented behavior. See
[Coming from Liquid](/from-liquid).

## Where to start

| You want to | Read |
|---|---|
| Write templates | [Syntax](/syntax), then [the tutorial](/tutorial) |
| Look up a filter or tag | [Filters](/filters), [Conditionals](/conditionals), [Loops](/loops) |
| Embed Vascula in a Go program | [Getting started](/getting-started), then the [Go API](/go-api) |
| Extend it with your own filters and tags | [Extending Vascula](/extending) |
| Understand what it protects against | [Safety and limits](/safety) |
| Generate templates with an AI agent | [Vascula for agents](/agents) and [llms.txt](/llms.txt) |
| Build a theme for a Tringify store | [Tringify theme docs](https://dev-docs.tringify.com/themes/) |

## Made by Tringify

Vascula is made by [Tringify](https://tringify.com) and renders every Tringify
storefront. Building a theme for a Tringify store? See the
[Tringify theme docs](https://dev-docs.tringify.com/themes/).

## Versions

Vascula is at version 0.1. Until 1.0, a release can change the language or
the Go API; the [changelog](/changelog) lists every change.

Every page of this site is also available as Markdown: add `.md` to the
address, or read [llms-full.txt](/llms-full.txt) for the whole site in one file.
