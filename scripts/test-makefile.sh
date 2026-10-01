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
[ $fail -eq 0 ] && echo "TestMakefileTargets_SG001_AC1 PASS"
exit $fail
