#!/usr/bin/env bash
# make contracts: Spectral lint, oasdiff against main, JSON Schema compile, pricing vector check.
# Runs all four steps in order and stops at the first failure, printing "FAIL step: <name>".
# Env: CONTRACTS_DIR (default contracts), BASE_FILE (oasdiff base; default origin/main copy).
set -u
cd "$(dirname "$0")/.."
root=$PWD
dir=${CONTRACTS_DIR:-contracts}
OASDIFF_VERSION=v1.32.1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

step() { # step <name> <command...>
  local name=$1; shift
  if "$@"; then echo "ok step: $name"; else echo "FAIL step: $name" >&2; exit 1; fi
}

ensure_tools() {
  [ -x scripts/node_modules/.bin/spectral ] || npm ci --silent --no-audit --no-fund --prefix scripts
}

lint() { scripts/node_modules/.bin/spectral lint "$dir/openapi.yaml" --ruleset "$dir/.spectral.yaml" --fail-severity=error; }

# The base is the published contract on main. No base yet (first commit of the file) is a notice, not a failure.
base_file() {
  if [ -n "${BASE_FILE:-}" ]; then echo "$BASE_FILE"; return; fi
  local ref
  for ref in origin/main main; do
    if git cat-file -e "$ref:contracts/openapi.yaml" 2>/dev/null; then
      git show "$ref:contracts/openapi.yaml" >"$tmp/base.yaml"; echo "$tmp/base.yaml"; return
    fi
  done
}
breaking() {
  local base; base=$(base_file)
  if [ -z "$base" ]; then echo "notice: no base contract on main, skipping oasdiff"; return 0; fi
  go run "github.com/oasdiff/oasdiff@$OASDIFF_VERSION" breaking "$base" "$dir/openapi.yaml" --fail-on ERR
}

schemas() { node scripts/schema-compile.mjs "$dir"; }
vectors() { python3 "$dir/pricing/generate_vectors.py" --check; }

ensure_tools
step spectral lint
step oasdiff breaking
step schemas schemas
step vectors vectors
