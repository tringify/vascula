# Contributing to Vascula

Small, focused changes are easiest to review. Describe the behavior you want,
include the smallest template that demonstrates it, and show the expected output.
Discuss new syntax or changes to existing semantics before implementing them.

## Development

Install Go 1.25 or later, clone the repository, and run:

```sh
./scripts/verify.sh
go test -run '^$' -fuzz FuzzCompileAndRender -fuzztime 30s .
go run ./cmd/vascula-docs -out dist/docs
```

The engine and documentation builder use only Go's standard library. Tests need
no accounts, credentials, database or network connection. Supply an installed Go
toolchain; verification does not download one.

Add a regression test for a bug fix. Include empty, malformed and limit-boundary
cases when they matter. Documentation examples are executable tests; update
the relevant page in `docs/` with the implementation.
Run the race detector when changing compiled-template or render state.

## Pull requests

Explain the problem, resulting behavior and verification. Keep unrelated edits
separate. Do not include credentials, private templates or personal customer data
in examples, screenshots or test fixtures. Use fictional names and `.example`
or `.test` domains. Do not post potential vulnerabilities as public issues; follow
[Security](SECURITY.md).

Contributions are provided under the repository's MIT licence. By contributing,
you confirm that you can submit your changes under that licence. Reviewers check
correctness, compatibility, documentation and resource use before merging.

## Compatibility

Until 1.0, a release can change the language or the Go API. A release must describe observable changes, including earlier rejection
of invalid templates. New syntax requires language tests, documentation and an
explicit decision about conflicts with existing templates. Similar syntax in
another template language does not establish Vascula compatibility.
