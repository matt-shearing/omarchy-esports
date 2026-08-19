#!/usr/bin/env bash
# Run every test: the Go suites and the shared QML/JavaScript model.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "== go =="
go vet ./...
go test ./...

echo
echo "== shared/Model.js =="
if command -v node >/dev/null; then
  node tests/model.test.mjs
else
  echo "  node not found; skipping" >&2
fi

echo
echo "== plugin manifest =="
if command -v omarchy-plugin-validate >/dev/null; then
  omarchy-plugin-validate ./plugin/contra.esports && echo "  manifest valid"
else
  echo "  omarchy-plugin-validate not found; skipping" >&2
fi
