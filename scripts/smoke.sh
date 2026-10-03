#!/usr/bin/env bash
# `make smoke`: build and start the stack, create a test guesthouse like an installer would, run the money-path
# Playwright test against it, and tear everything down. Needs docker, node (npm ci done in web/), python3.
#   KEEP=1 make smoke     leaves the stack running afterwards (http://localhost:18080)
#   REUSE=1 make smoke    uses the stack KEEP=1 left (each run creates a fresh guesthouse, so it can run again at once)
#   SMOKE_PROJECT=name SMOKE_PORT=n SMOKE_DB_PORT=n   run next to another checkout's smoke stack
set -euo pipefail
cd "$(dirname "$0")/.."

PORT="${SMOKE_PORT:-18080}"
BASE="http://localhost:$PORT"
export SMOKE_PORT="$PORT" SMOKE_DB_PORT="${SMOKE_DB_PORT:-15432}"
COMPOSE=(docker compose -p "${SMOKE_PROJECT:-stayguard-smoke}" -f deploy/compose.yaml -f deploy/compose.smoke.override.yaml)
log="$(mktemp)"
tenant_file="$(mktemp)"
cleanup() {
	status=$?
	if [ "${KEEP:-0}" != 1 ] && [ "${REUSE:-0}" != 1 ]; then "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; fi
	rm -f "$log" "$tenant_file"
	exit "$status"
}
trap cleanup EXIT

make deploy/.env.local >/dev/null
if [ "${REUSE:-0}" != 1 ]; then
	"${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
	echo "== starting the stack"
	# The API now refuses to start on an unmigrated database (key fingerprint check): migrate first, then start it.
	"${COMPOSE[@]}" up -d db >/dev/null
	"${COMPOSE[@]}" build api >/dev/null
	"${COMPOSE[@]}" run --rm api migrate >/dev/null
	"${COMPOSE[@]}" up -d api >/dev/null
fi
for _ in $(seq 60); do
	curl -fsS "$BASE/readyz" >/dev/null 2>&1 && break
	sleep 1
done
curl -fsS "$BASE/readyz" >/dev/null || { echo "FAIL: the stack did not become ready" >&2; "${COMPOSE[@]}" logs api | tail -n 20 >&2; exit 1; }

# A new guesthouse code every run, so a kept stack can be used again.
code="smoke$(openssl rand -hex 3)"
sed "s/\"guesthouseCode\": \"smoke\"/\"guesthouseCode\": \"$code\"/" scripts/smoke/tenant.json >"$tenant_file"
echo "== creating the test guesthouse $code"
"${COMPOSE[@]}" run --rm -T -v "$tenant_file:/tmp/tenant.json:ro" api tenant import --file /tmp/tenant.json >"$log"
pin() { awk -v u="$1" '$1 == u { print $2 }' "$log"; }
hook="$(awk '/^webhook path/ { print $3 }' "$log")"
owner_pin="$(pin owner)"; linh_pin="$(pin linh)"
[ -n "$hook" ] && [ -n "$owner_pin" ] && [ -n "$linh_pin" ] || { echo "FAIL: could not read the import output" >&2; exit 1; }

secret="$(openssl rand -hex 24)"
python3 scripts/smoke/set-secret.py "$secret" "${COMPOSE[@]}" run --rm api sepay set-secret --tenant "$code" >/dev/null

export E2E_BASE_URL="$BASE" SMOKE_HOOK_PATH="$hook" SMOKE_SEPAY_SECRET="$secret" SMOKE_ACCOUNT_NO=1017588888 \
	SMOKE_OWNER_PIN="$owner_pin" SMOKE_RECEPTIONIST_PIN="$linh_pin" SMOKE_GUESTHOUSE="$code"
if [ "${KEEP:-0}" = 1 ]; then # for writing the test: source this file to run playwright by hand against the kept stack
	env | grep '^\(E2E_BASE_URL\|SMOKE_\)' | sed 's/^/export /' > /tmp/stayguard-smoke.env
fi

echo "== running the money path"
cd web
npx playwright test e2e/smoke.spec.ts e2e/smoke.then-partial.spec.ts --workers=1 --reporter=list
