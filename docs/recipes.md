---
title: Recipes
description: Short, tested solutions to common template tasks.
section: Guides
order: 41
---

# Recipes

Each recipe is a complete, tested example.

## Show something only if it is there

`if` alone is not enough, because `""` and `[]` are truthy. Compare with
`blank`:

```vascula
{% if settings.subtitle != blank %}<p class="subtitle">{{ settings.subtitle }}</p>{% endif %}
```

```json data
{"settings": {"subtitle": ""}}
```

```output

```

## Fall back to a default

`default` treats `nil`, `false`, `""` and empty collections as missing:

```vascula
<h2>{{ settings.heading | default: "Featured" }}</h2>
```

```json data
{"settings": {"heading": ""}}
```

```output
<h2>Featured</h2>
```

## Join a list with commas and "and"

```vascula
{% for name in settings.names %}{% if forloop.last and forloop.length > 1 %} and {% elsif forloop.index > 1 %}, {% endif %}{{ name }}{% endfor %}
```

```json data
{"settings": {"names": ["Ada", "Grace", "Hedy"]}}
```

```output
Ada, Grace and Hedy
```

## Plural words

```vascula
{{ settings.count }} {% if settings.count == 1 %}item{% else %}items{% endif %}
```

```json data
{"settings": {"count": 1}}
```

```output
1 item
```

## A grid with a fixed number of columns

`forloop.index0` with `modulo` finds the first item of each row:

```vascula
{% for item in (1..5) %}{% assign column = forloop.index0 | modulo: 2 %}{% if column == 0 %}<div class="row">{% endif %}<span>{{ item }}</span>{% if column == 1 or forloop.last %}</div>{% endif %}{% endfor %}
```

```output
<div class="row"><span>1</span><span>2</span></div><div class="row"><span>3</span><span>4</span></div><div class="row"><span>5</span></div>
```

## The first few items, then "and more"

```vascula
{% for tag in settings.tags limit: 2 %}{{ tag }} {% endfor %}{% if settings.tags.size > 2 %}and {{ settings.tags.size | minus: 2 }} more{% endif %}
```

```json data
{"settings": {"tags": ["new", "sale", "tea", "gift"]}}
```

```output
new sale and 2 more
```

## Filter, sort and pick

```vascula
{% assign cheapest = settings.products | where: "available" | sort: "price" | first %}Cheapest available: {{ cheapest.title }}
```

```json data
{"settings": {"products": [
  {"title": "Pot", "price": 30, "available": true},
  {"title": "Cup", "price": 8, "available": false},
  {"title": "Jug", "price": 12, "available": true}
]}}
```

```output
Cheapest available: Jug
```

## Put data in an attribute or a script

`json` is safe in both places:

```vascula
<div data-config="{{ settings.config | json }}"></div><script>const config = {{ settings.config | json }};</script>
```

```json data
{"settings": {"config": {"autoplay": true, "delay": 3}}}
```

```output
<div data-config="{&#34;autoplay&#34;:true,&#34;delay&#34;:3}"></div><script>const config = {"autoplay":true,"delay":3};</script>
```

## A short excerpt from rich text

```vascula
{{ settings.body | strip_html | truncatewords: 5 }}
```

```json data
{"settings": {"body": "<p>Our <b>first flush</b> Darjeeling has arrived this week.</p>"}}
```

```output
Our first flush Darjeeling has...
```

## Mark the current item

```vascula
{% for link in settings.links %}<a href="{{ link.url }}"{% if link.url == settings.current %} aria-current="page"{% endif %}>{{ link.title }}</a>{% endfor %}
```

```json data
{"settings": {"current": "/teas", "links": [{"title": "Home", "url": "/"}, {"title": "Teas", "url": "/teas"}]}}
```

```output
<a href="/">Home</a><a href="/teas" aria-current="page">Teas</a>
```

## Group items under headings

There is no `group_by`; sort by the group, then print a heading whenever it
changes:

```vascula
{% assign last = nil %}{% for t in settings.teas | sort: "kind" %}{% if t.kind != last %}<h3>{{ t.kind }}</h3>{% assign last = t.kind %}{% endif %}<p>{{ t.name }}</p>{% endfor %}
```

```json data
{"settings": {"teas": [{"name": "Sencha", "kind": "green"}, {"name": "Assam", "kind": "black"}, {"name": "Matcha", "kind": "green"}]}}
```

```error
expected "%}"
```

A loop's collection cannot take filters directly. Assign the sorted list first:

```vascula
{% assign sorted = settings.teas | sort: "kind" %}{% assign last = nil %}{% for t in sorted %}{% if t.kind != last %}<h3>{{ t.kind }}</h3>{% assign last = t.kind %}{% endif %}<p>{{ t.name }}</p>{% endfor %}
```

```json data
{"settings": {"teas": [{"name": "Sencha", "kind": "green"}, {"name": "Assam", "kind": "black"}, {"name": "Matcha", "kind": "green"}]}}
```

```output
<h3>black</h3><p>Assam</p><h3>green</h3><p>Sencha</p><p>Matcha</p>
```
