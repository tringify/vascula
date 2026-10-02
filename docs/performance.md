---
title: Performance
description: Benchmarks against Liquid, and how to keep templates fast.
section: Guides
order: 43
---

# Performance

## Benchmarks

The same templates and data, rendered by Vascula, the Go Liquid
implementation (`github.com/osteele/liquid`) and Shopify's Ruby Liquid (with
YJIT), on one Linux machine. Lower is better.

| Workload | Vascula | Go Liquid | Ruby Liquid |
|---|---:|---:|---:|
| Product grid, 48 products | **100 µs** | 320 µs | 352 µs |
| `where \| sort \| map \| join` over 48 products | **8.5 µs** | 27 µs | 37 µs |
| The same, products with 150 fields each | **10.5 µs** | 30 µs | 38 µs |
| Compiling the grid template | **9 µs** | 32 µs | 89 µs |

Measured on 2 October 2026. The grid template prints
titles, prices and tags with conditions and nested loops; it is in the
engine's benchmark suite.

## Keeping templates fast

- **Filter before you loop.** `{% assign visible = items | where: "visible" %}`
  then loop over `visible`, rather than an `if` around every iteration.
- **Use `limit`.** `{% for p in products limit: 8 %}` stops early.
- **Avoid loops inside loops over large collections.** They multiply work and
  are the usual cause of `too many steps`.
- **Give templates only the data they need.** Smaller data is faster to load and
  to check.
- **Compile once.** Keep compiled templates and reuse them; see
  [Embedding](/embedding#cache-compiled-templates).
- **Cache children.** Your resolver runs for every `render` tag; return
  compiled templates from a cache.
