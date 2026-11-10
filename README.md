<p align="center">
  <a href="https://vascula.dev"><img src=".github/assets/vascula-greenhouse.webp" width="100%" alt="Pixel-art greenhouse at night: glowing green vines connect a developer at a desk to products, a shop front, carts and parcels."></a>
</p>

<h1 align="center">Vascula</h1>

<p align="center">Safe, fast HTML templates for Go.</p>

<p align="center">
  <a href="https://vascula.dev">Documentation</a> ·
  <a href="https://vascula.dev/getting-started">Getting started</a> ·
  <a href="https://vascula.dev/tutorial">Tutorial</a> ·
  <a href="https://vascula.dev/changelog">Changelog</a>
</p>

<p align="center">
  <a href="https://pkg.go.dev/vascula.dev/vascula"><img src="https://pkg.go.dev/badge/vascula.dev/vascula.svg" alt="Go reference"></a>
  <a href="https://github.com/tringify/vascula/actions/workflows/verify.yml"><img src="https://github.com/tringify/vascula/actions/workflows/verify.yml/badge.svg" alt="Verify"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/licence-MIT-a6f87b" alt="MIT licence"></a>
</p>

Vascula is a template language for HTML and a Go library with no third-party
dependencies. You write markup with `{{ output }}` and `{% tags %}`; your
application compiles the template once and renders it with data.

- **Safe by default.** Everything a template prints is HTML-escaped, and a
  template cannot switch that off.
- **Explicit about data.** A template reads only the data your application
  allows; anything else is an error, not a silent blank.
- **Bounded.** Compilation and rendering have limits, so a template cannot loop
  forever or build a gigabyte string.
- **Fast.** See the [benchmarks](https://vascula.dev/performance).
- **Familiar.** If you know Liquid or Jinja you can already read it; see
  [Coming from Liquid](https://vascula.dev/from-liquid).

## Install

```sh
go get vascula.dev/vascula@latest
```

Use Go 1.25 or later. Vascula follows semantic versioning; read
[the changelog](CHANGELOG.md) before upgrading.

## A first template

```go
tmpl, err := vascula.Compile(`<h1>{{ settings.title }}</h1>`)
if err != nil {
    return err
}
output, err := tmpl.Render(vascula.Options{
    Settings: map[string]interface{}{"title": "Tea & coffee"},
})
// output: <h1>Tea &amp; coffee</h1>
```

## Learn the language

The documentation lives at [vascula.dev](https://vascula.dev). Its source is
the Markdown in [`docs/`](docs), and every page is also published as plain
Markdown (`/<page>.md`) together with `/llms.txt` and `/llms-full.txt` for
agents. Every template example in `docs/` is rendered by
`TestDocumentationExamples`, every built-in filter must have a section in
`docs/filters.md`, and every Go program in the docs is compiled and run by
`scripts/verify_examples.py`.

## Embed Vascula

Compile once and render with request-specific options. Supply application data
in `Data`, and grant each readable root in `Allow`. A dotted allowance grants
the whole root; it is not field-level authorization. `Settings` is always
readable as `settings`; `Globals` adds further names every template can read.

Use `Resolve` to supply child templates, `Filters` for trusted filters,
`CompileOptions.Forms` and `CompileOptions.Tags` to declare form kinds and host
tags, and `CompileOptions.Reserved` for names templates may not assign.
`ExternalNames`, `ExternalPaths`, `RenderTargets`, `RenderArguments`, `UsesForm` and `UsesTag`
support inspection before rendering.

Vascula does not supply a commerce API, identity system, route table, theme bundle
format or editor schema. Those are responsibilities of the embedding application.
Similar syntax does not imply compatibility with another template language.

## Safety

Ordinary output is HTML-escaped. A URL attribute such as `href` or `src` with
printed parts must, as a whole, be relative or use `http`, `https`, `mailto`
or `tel`; anything else prints as `#`. Literal template HTML, `SafeHTML` in the
page body, `HTMLAttr` where an attribute goes and host callbacks remain
trusted. Vascula is not an HTML sanitizer and does not perform contextual
JavaScript or CSS escaping. Never evaluate rendered data as a
second template or return source code when rendering fails.

Compilation and rendering enforce source, token, nesting, work, output and value
limits. These are not hard process-memory or callback-time quotas. The host must
authorize data, validate submitted forms and bound its own extensions. Read
[Security](SECURITY.md) and [Safety and limits](https://vascula.dev/safety).

## Verify and contribute

```sh
./scripts/verify.sh
go test -run '^$' -fuzz FuzzCompileAndRender -fuzztime 30s .
```

Verification checks formatting, vets the packages, runs the race detector and
executes documentation examples. It requires an installed Go toolchain and Python
3.10 or later for the standalone-consumer and release checks. No credentials or
external services are needed. See [Contributing](CONTRIBUTING.md).

## Licence

[MIT](LICENSE). Copyright 2026 Tringify.
