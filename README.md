# Vascula

Readable HTML templates with explicit data access, reusable components and
bounded rendering. Vascula is a Go library with no third-party runtime dependencies.

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

```sh
go get vascula.dev/vascula@latest
```

The module is `vascula.dev/vascula`. Use Go 1.25 or later. Documentation:
[vascula.dev](https://vascula.dev).
Until 1.0, a release can change the language or the Go API; pin a version and
read [the changelog](CHANGELOG.md) before upgrading.

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
`ExternalNames`, `RenderTargets`, `RenderArguments`, `UsesForm` and `UsesTag`
support inspection before rendering.

Vascula does not supply a commerce API, identity system, route table, theme bundle
format or editor schema. Those are responsibilities of the embedding application.
Similar syntax does not imply compatibility with another template language.

## Safety

Ordinary output is HTML-escaped. Literal template HTML, `SafeHTML` and host
callbacks remain trusted. Vascula is not an HTML sanitizer and does not perform
contextual JavaScript, CSS or URL escaping. Never evaluate rendered data as a
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
