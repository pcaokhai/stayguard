#!/usr/bin/env bash
# Makefile_SG001_AC1: every named target exists; unimplemented ones fail and name their story.
set -u
cd "$(dirname "$0")/.."
fail=0
for t in gen:SG-002 migrate:SG-003 test-api-int:SG-003 e2e:SG-603; do
  out=$(make "${t%%:*}" 2>&1); rc=$?
  if [ $rc -eq 0 ] || ! grep -q "${t##*:}" <<<"$out"; then echo "FAIL ${t%%:*}: rc=$rc out=$out"; fail=1; fi
done
for t in up down test test-api test-web lint fmt contracts; do
  make -n "$t" >/dev/null 2>&1 || { echo "FAIL make -n $t"; fail=1; }
done
[ $fail -eq 0 ] && echo "TestMakefileTargets_SG001_AC1 PASS"
exit $fail
