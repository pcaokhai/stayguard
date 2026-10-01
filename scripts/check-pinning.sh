#!/usr/bin/env bash
# SG001_AC5: no floating dependency ranges; licence and update files present.
set -u
cd "$(dirname "$0")/.."
fail=0
bad() { echo "FAIL: $*" >&2; fail=1; }

for f in LICENSE NOTICE .github/dependabot.yml; do
  [ -f "$f" ] || bad "missing $f"
done

# go.mod: versions must be exact vX.Y.Z (no "latest", no pseudo ranges); toolchain line exact.
if grep -nE '^\s*go\s+[0-9]+\.[0-9]+\s*$|latest' api/go.mod; then bad "api/go.mod has a floating go version or 'latest'"; fi

# package.json: every dependency value must be an exact version (digits and dots, optional prerelease).
if ! node -e '
const p = require("./web/package.json");
const exact = /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/;
let bad = 0;
for (const k of ["dependencies","devDependencies","optionalDependencies","peerDependencies"])
  for (const [n, v] of Object.entries(p[k] || {}))
    if (!exact.test(v)) { console.error(`web/package.json ${k}.${n} = ${v}`); bad = 1; }
process.exit(bad);
'; then bad "web/package.json has non-exact versions (^, ~, *, tags, ranges)"; fi

# Workflow actions: pinned by 40-hex SHA. Local (./) actions are exempt.
if [ -d .github/workflows ]; then
  while IFS= read -r line; do
    bad "unpinned action: $line"
  done < <(grep -rhnE '^\s*-?\s*uses:' .github/workflows | grep -vE 'uses:\s*(\./|[^@]+@[0-9a-f]{40}(\s|$))')
fi

[ "$fail" -eq 0 ] && echo "PASS check-pinning"
exit "$fail"
