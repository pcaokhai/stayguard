#!/usr/bin/env bash
# demo-check's table and exit status, with fake steps (DEMO_CHECK_STEPS): every step runs even after a failure, the table has one
# PASS/FAIL row per step, and the exit status is non-zero when any step failed.
set -u
cd "$(dirname "$0")/.."
fail=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# first reads stdin like make and npx can: it must not swallow the steps after it
printf 'first|cat >/dev/null; true\nsecond|echo boom >&2; false\nthird|true\n' >"$tmp/mixed"
out=$(DEMO_CHECK_STEPS="$tmp/mixed" scripts/demo-check.sh 2>&1); rc=$?
[ $rc -ne 0 ] || { echo "FAIL a failing step must give a non-zero exit"; fail=1; }
grep -Eq '^first +PASS' <<<"$out" || { echo "FAIL first should PASS: $out"; fail=1; }
grep -Eq '^second +FAIL' <<<"$out" || { echo "FAIL second should FAIL: $out"; fail=1; }
grep -Eq '^third +PASS' <<<"$out" || { echo "FAIL third must still run after a failure: $out"; fail=1; }
grep -q 'boom' <<<"$out" || { echo "FAIL the failing step's last output lines are shown: $out"; fail=1; }

# A failure early in a long output must be shown, not only the last 15 lines: the first 20 lines with FAIL, --- FAIL, panic or Error.
cat >"$tmp/noisy.sh" <<'NOISY'
echo "--- FAIL: TestEarlyOne (0.1s)"
echo "    some_test.go:12: Error: the first failing line"
for i in $(seq 40); do echo "ok  	package$i	0.3s"; done
echo "panic: boom in a late test"
for i in $(seq 30); do echo "ok  	other$i	0.3s"; done
false
NOISY
printf 'noisy|bash %s\n' "$tmp/noisy.sh" >"$tmp/noisy"
out=$(DEMO_CHECK_STEPS="$tmp/noisy" scripts/demo-check.sh 2>&1)
grep -q 'TestEarlyOne' <<<"$out" || { echo "FAIL the first failing test must be shown even when it is far from the end: $out"; fail=1; }
grep -q 'the first failing line' <<<"$out" || { echo "FAIL lines matching Error/FAIL are listed: $out"; fail=1; }
grep -q 'panic: boom' <<<"$out" || { echo "FAIL panic lines are listed: $out"; fail=1; }

printf 'only|true\n' >"$tmp/green"
DEMO_CHECK_STEPS="$tmp/green" scripts/demo-check.sh >/dev/null 2>&1 || { echo "FAIL all green must exit 0"; fail=1; }

# Before web-build, a stale web/.next is removed (it made next build fail with "Cannot find module .../page.js") and the output says so.
mkdir -p "$tmp/next/dev/types"; touch "$tmp/next/dev/types/stale.ts"
printf 'web-build|test ! -e "%s"\n' "$tmp/next" >"$tmp/web"
out=$(DEMO_CHECK_WEB_NEXT="$tmp/next" DEMO_CHECK_STEPS="$tmp/web" scripts/demo-check.sh 2>&1); rc=$?
[ $rc -eq 0 ] || { echo "FAIL web-build must run after .next was removed: $out"; fail=1; }
grep -q 'removing web/.next' <<<"$out" || { echo "FAIL the output must say web/.next was removed: $out"; fail=1; }
# Other steps do not remove it.
mkdir -p "$tmp/next2"; printf 'lint|true\n' >"$tmp/other"
DEMO_CHECK_WEB_NEXT="$tmp/next2" DEMO_CHECK_STEPS="$tmp/other" scripts/demo-check.sh >/dev/null 2>&1
[ -d "$tmp/next2" ] || { echo "FAIL only the web-build step removes .next"; fail=1; }

# The smoke step gets the compose project the runner uses for smoke in SMOKE_COMPOSE_PROJECT (WEB-1's expiry spec reads it).
printf 'smoke|test "$SMOKE_COMPOSE_PROJECT" = stayguard-demo-check\n' >"$tmp/smoke"
DEMO_CHECK_STEPS="$tmp/smoke" scripts/demo-check.sh >/dev/null 2>&1 || { echo "FAIL the smoke step must see SMOKE_COMPOSE_PROJECT=stayguard-demo-check"; fail=1; }
grep -q 'SMOKE_PROJECT=\$SMOKE_COMPOSE_PROJECT' scripts/demo-check.steps || { echo "FAIL the real smoke step must use that same project"; fail=1; }

# The real step list names the eight checks of the demo gate.
for s in lint go-unit go-integration docker-build smoke rehearse-test web-build layout-spec; do
  grep -q "^$s|" scripts/demo-check.steps || { echo "FAIL missing step $s"; fail=1; }
done
exit $fail
