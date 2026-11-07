#!/usr/bin/env bash
# Runs CI checks for one plugin module: dependency sync, lint, build, an info
# probe, and every Go test package (nested packages included).
# Usage: scripts/ci/test-plugin.sh <plugin-directory-name>
# DEVEX_CI_ROOT overrides the repository root; the tests use it for fixtures.
set -euo pipefail

plugin=${1:-}
case "$plugin" in
  "" | . | .. | */*)
    echo "usage: test-plugin.sh <plugin-directory-name>" >&2
    exit 2
    ;;
esac
root=${DEVEX_CI_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}
cd "$root/packages/$plugin"

fail() {
  echo "::endgroup::"
  echo "::error title=$1::packages/$plugin: $2"
  exit 1
}

echo "::group::Updating dependencies for $plugin"
if [ -f go.mod ]; then
  go mod tidy
  go mod download
fi
echo "::endgroup::"

echo "::group::Linting $plugin"
if [ -f go.mod ]; then
  golangci-lint run || fail "golangci-lint failed" "run 'golangci-lint run' in that directory"
fi
echo "::endgroup::"

echo "::group::Building $plugin"
if [ -f main.go ]; then
  go build -v -o "devex-plugin-$plugin" . || fail "go build failed" "run 'go build .' in that directory"
  echo "::endgroup::"

  echo "::group::Testing plugin functionality"
  "./devex-plugin-$plugin" --plugin-info || echo "Plugin info command not available"
  echo "::endgroup::"
elif [ -f package.json ]; then
  echo "Node.js plugin detected"
  if command -v npm >/dev/null 2>&1; then
    npm install || echo "npm install failed"
    npm test || echo "npm test failed"
  fi
  echo "::endgroup::"
else
  echo "::endgroup::"
fi

# Run every package's tests; a module whose tests live only in nested packages
# must not be skipped.
echo "::group::Running tests"
if [ -f go.mod ]; then
  go test -v ./... || fail "go test failed" "run 'go test -v ./...' in that directory; failing specs are listed under [FAIL] above"
else
  echo "No Go module for $plugin"
fi
echo "::endgroup::"
