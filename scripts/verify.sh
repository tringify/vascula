#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
export GOMAXPROCS="${GOMAXPROCS:-2}"

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "Go files need formatting:" >&2
  echo "$unformatted" >&2
  exit 1
fi
go vet ./...
go test -race -p 2 -timeout 90s ./...
python3 scripts/verify_examples.py
