#!/usr/bin/env bash
# MockClient_SG002_AC3 (build part): NEXT_PUBLIC_MOCK=1 ships the MSW worker code, the default build does not.
set -u
cd "$(dirname "$0")/../web"
fail=0
# The default worker URL is a string only the MSW browser runtime contains.
marker='mockServiceWorker.js'
shipped() { grep -rl --include='*.js' "$marker" out/_next 2>/dev/null | head -n 1; }

rm -rf out .next; NEXT_PUBLIC_MOCK=1 npm run build >/dev/null 2>&1 || { echo "FAIL: mock build failed" >&2; exit 1; }
[ -n "$(shipped)" ] || { echo "FAIL: mock build lacks the MSW worker" >&2; fail=1; }

rm -rf out .next; npm run build >/dev/null 2>&1 || { echo "FAIL: default build failed" >&2; exit 1; }
[ -z "$(shipped)" ] || { echo "FAIL: default build ships mock code: $(shipped)" >&2; fail=1; }

[ "$fail" -eq 0 ] && echo "PASS check-mock-switch"
exit "$fail"
