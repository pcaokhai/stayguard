#!/usr/bin/env bash
# `make demo-check`: everything the portfolio demo must pass, one PASS/FAIL table, non-zero exit when anything failed. The steps are
# in scripts/demo-check.steps (DEMO_CHECK_STEPS points at another list, as the self-test does). It is only a runner: the checks are the
# existing make targets, and the rehearsal runner (`make rehearse-test`) stays the engine of the checklist. Needs docker, node (npm ci in
# web/), go and golangci-lint.
#   DEMO_CHECK_ONLY='smoke|lint'   only the steps whose name matches this regular expression
set -u
cd "$(dirname "$0")/.."
steps="${DEMO_CHECK_STEPS:-scripts/demo-check.steps}"
logs="$(mktemp -d)"
# The real steps use the rehearse stack: one run at a time per clone. The rehearse-test step passes through because it inherits the lock.
# (A self-test with its own step list does not touch the stack and takes no lock.)
. scripts/rehearse-env.sh
[ -n "${DEMO_CHECK_STEPS:-}" ] || rehearse_lock "make demo-check"
trap 'rm -rf "$logs"; rehearse_unlock' EXIT

# The compose project the smoke step runs in; the step and the specs it runs read it (WEB-1's expiry spec looks at its containers).
export SMOKE_COMPOSE_PROJECT="${SMOKE_COMPOSE_PROJECT:-stayguard-demo-check}"
web_next="${DEMO_CHECK_WEB_NEXT:-web/.next}"

names=(); results=(); secs=()
while IFS='|' read -r name cmd; do
	case "$name" in ''|'#'*) continue ;; esac
	if [ -n "${DEMO_CHECK_ONLY:-}" ] && ! [[ "$name" =~ ^(${DEMO_CHECK_ONLY})$ ]]; then continue; fi
	echo ">> $name" >&2
	if [ "$name" = web-build ]; then # a stale .next (dev types) made next build fail with "Cannot find module .../page.js"
		echo "   removing web/.next first (a stale one breaks next build)" >&2
		rm -rf "$web_next"
	fi
	start=$SECONDS
	if bash -c "$cmd" </dev/null >"$logs/$name.log" 2>&1; then r=PASS; else r=FAIL; fi
	names+=("$name"); results+=("$r"); secs+=("$((SECONDS - start))")
	echo "<< $name $r (${secs[${#secs[@]}-1]}s)" >&2
done <"$steps"

failed=0
printf '\n%-18s %-6s %s\n' STEP RESULT TIME
for i in "${!names[@]}"; do
	printf '%-18s %-6s %ss\n' "${names[$i]}" "${results[$i]}" "${secs[$i]}"
	[ "${results[$i]}" = PASS ] || failed=1
done
for i in "${!names[@]}"; do
	if [ "${results[$i]}" = FAIL ]; then
		printf '\n--- %s: the first 20 lines with FAIL, panic or Error ---\n' "${names[$i]}"
		grep -n -E 'FAIL|panic|Error' "$logs/${names[$i]}.log" | head -n 20
		printf '\n--- %s: last lines of its output ---\n' "${names[$i]}"
		tail -n 15 "$logs/${names[$i]}.log"
	fi
done
if [ $failed -eq 0 ]; then echo; echo "DEMO CHECK: ALL PASS"; else echo; echo "DEMO CHECK: FAILED"; fi
exit $failed
