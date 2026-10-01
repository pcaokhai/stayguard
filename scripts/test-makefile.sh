#!/usr/bin/env bash
# Makefile_SG001_AC1: every named target exists; unimplemented ones fail and name their story.
set -u
cd "$(dirname "$0")/.."
fail=0
for t in e2e:SG-603; do
  out=$(make "${t%%:*}" 2>&1); rc=$?
  if [ $rc -eq 0 ] || ! grep -q "${t##*:}" <<<"$out"; then echo "FAIL ${t%%:*}: rc=$rc out=$out"; fail=1; fi
done
for t in up down test test-api test-api-int migrate test-web lint fmt contracts gen; do
  make -n "$t" >/dev/null 2>&1 || { echo "FAIL make -n $t"; fail=1; }
done
# SG-203: the local key rule fails loudly, leaves no file behind, then creates a private key once.
tmp=$(mktemp -d); cp Makefile "$tmp/"; mkdir "$tmp/deploy" "$tmp/bin"
printf '#!/bin/sh\nexit 0\n' > "$tmp/bin/openssl"; chmod +x "$tmp/bin/openssl"
if (cd "$tmp" && PATH="$tmp/bin:$PATH" make deploy/.env.local >/dev/null 2>&1) || [ -e "$tmp/deploy/.env.local" ] || [ -e "$tmp/deploy/.env.local.tmp" ]; then
  echo "FAIL empty key must fail and leave no file"; fail=1
fi
(cd "$tmp" && make deploy/.env.local >/dev/null 2>&1)
key=$(sed -n 's/^DATA_ENCRYPTION_KEY=//p' "$tmp/deploy/.env.local")
[ -n "$key" ] || { echo "FAIL key file empty"; fail=1; }
[ -n "$(find "$tmp/deploy/.env.local" -perm 600)" ] || { echo "FAIL key file mode"; fail=1; }
(cd "$tmp" && make deploy/.env.local >/dev/null 2>&1)
[ "$key" = "$(sed -n 's/^DATA_ENCRYPTION_KEY=//p' "$tmp/deploy/.env.local")" ] || { echo "FAIL key not idempotent"; fail=1; }
rm -rf "$tmp"
[ $fail -eq 0 ] && echo "TestMakefileTargets_SG001_AC1 PASS"
exit $fail
