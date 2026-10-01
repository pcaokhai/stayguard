#!/usr/bin/env bash
# Contracts_SG002_AC1: `make contracts` passes on a clean tree and each sabotage case fails
# naming its step. Every case runs on a temporary copy of contracts/; the real tree is never
# edited, and the script asserts that at the end.
set -u
cd "$(dirname "$0")/.."
root=$PWD
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0
bad() { echo "FAIL: $*" >&2; fail=1; }

fresh_copy() { rm -rf "$tmp/c"; rsync -a --exclude node_modules contracts/ "$tmp/c/"; }
# edit <file> <regex> <replacement>: replace the first match in the temp copy; fail if none.
edit() {
  python3 - "$@" <<'PY' || bad "sabotage edit found no match in $1"
import re, sys
p, pat, rep = sys.argv[1:4]
t = open(p).read()
n, c = re.subn(pat, rep, t, count=1, flags=re.M)
if c == 0:
    sys.exit(1)
open(p, "w").write(n)
PY
}
# run_case <step> <description>: contracts.sh on the copy must exit non-zero and name <step>.
run_case() {
  local out rc
  out=$(CONTRACTS_DIR="$tmp/c" BASE_FILE="$root/contracts/openapi.yaml" bash scripts/contracts.sh 2>&1); rc=$?
  if [ "$rc" -eq 0 ] || ! grep -q "FAIL step: $1" <<<"$out"; then
    bad "$2: expected exit != 0 naming step '$1', got rc=$rc"; echo "$out" | tail -n 5 >&2
  fi
}

# Clean tree passes and reports all four steps.
out=$(BASE_FILE="$root/contracts/openapi.yaml" bash scripts/contracts.sh 2>&1); rc=$?
[ "$rc" -eq 0 ] || { bad "clean tree failed (rc=$rc)"; echo "$out" | tail -n 8 >&2; }
for s in spectral oasdiff schemas vectors; do grep -q "ok step: $s" <<<"$out" || bad "clean run lacks 'ok step: $s'"; done

fresh_copy; edit "$tmp/c/openapi.yaml" '^ +operationId: .*\n' ''
run_case spectral "missing operationId"
fresh_copy; edit "$tmp/c/openapi.yaml" '^  /readyz:' '  /readyz-renamed:'
run_case oasdiff "removed path"
fresh_copy; edit "$tmp/c/events/payment-events.schema.json" '"type": "object"' '"type": "objectx"'
run_case schemas "invalid JSON Schema"
fresh_copy; edit "$tmp/c/pricing/golden-cases.json" '"graceMinutes": [0-9]+' '"graceMinutes": 999'
run_case vectors "stale pricing vectors"

# The real contract must be untouched.
[ -z "$(git status --porcelain contracts/)" ] || { bad "git status --porcelain contracts/ is not empty"; git status --porcelain contracts/ >&2; }

[ "$fail" -eq 0 ] && echo "Contracts_SG002_AC1 PASS"
exit "$fail"
