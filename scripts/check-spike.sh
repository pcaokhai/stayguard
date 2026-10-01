#!/usr/bin/env bash
# SG002_AC6: the ADR-003 amendment records every OpenAPI 3.1 feature the spec uses
# and the result per generator; no feature may be marked unsupported.
set -u
cd "$(dirname "$0")/.."
adr=docs/adr/ADR-003-contract-first-openapi.md
fail=0
bad() { echo "FAIL: $*" >&2; fail=1; }

grep -q '^## Amendment: SG-002 generator spike' "$adr" || bad "ADR-003 has no spike amendment"
sect=$(sed -n '/^## Amendment: SG-002 generator spike/,$p' "$adr")
for feat in 'type: \[x, null\]' 'oneOf' 'const' 'webhook'; do
  echo "$sect" | grep -q "^| .*$feat" || bad "spike table lacks a row for: $feat"
done
for tool in oapi-codegen openapi-typescript orval Spectral; do
  echo "$sect" | grep -q "$tool" || bad "spike table lacks column or note for: $tool"
done
if echo "$sect" | grep -qi 'unsupported'; then bad "a 3.1 feature is marked unsupported: contract downgrade needed, stop"; fi
[ "$fail" = 0 ] && echo "PASS: spike recorded"
exit "$fail"
