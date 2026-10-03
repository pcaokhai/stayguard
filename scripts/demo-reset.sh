#!/usr/bin/env bash
# `make demo-reset`: wipes the demo stack and rebuilds it in a known state. Same path as a real handover: the installer commands
# (`tenant import`, `sepay set-secret`) create the guesthouse and a script then works it through the public API (check-in, extras,
# check-out, cash and QR payments by signed SePay webhooks, cleaning, a damage report, expenses, roster, a closed shift).
# What it creates: docs/runbooks/demo.md. Needs docker, python3, openssl. Test data only; never use it for real guests.
#   DEMO_PORT=18200    the local port (the database is on DEMO_DB_PORT, default 18201)
set -euo pipefail
cd "$(dirname "$0")/.."

PORT="${DEMO_PORT:-18200}"
export SMOKE_PORT="$PORT" SMOKE_DB_PORT="${DEMO_DB_PORT:-18201}"
BASE="http://localhost:$PORT"
COMPOSE=(docker compose -p stayguard-demo -f deploy/compose.yaml -f deploy/compose.smoke.override.yaml)
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

make deploy/.env.local >/dev/null
echo "== wiping the demo stack and starting a fresh one"
"${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
"${COMPOSE[@]}" up -d --build db >/dev/null
"${COMPOSE[@]}" run --rm api migrate >/dev/null # the API refuses to start on a database without a schema, so migrate first
"${COMPOSE[@]}" up -d api >/dev/null
for _ in $(seq 60); do
	curl -fsS "$BASE/readyz" >/dev/null 2>&1 && break
	sleep 1
done
curl -fsS "$BASE/readyz" >/dev/null || { echo "FAIL: the stack did not become ready" >&2; "${COMPOSE[@]}" logs api | tail -n 20 >&2; exit 1; }

echo "== importing the guesthouse 'demo' with the installer command"
"${COMPOSE[@]}" run --rm -T -v "$PWD/scripts/demo/tenant.json:/tmp/tenant.json:ro" api tenant import --file /tmp/tenant.json >"$log"
hook="$(awk '/^webhook path/ { print $3 }' "$log")"
[ -n "$hook" ] || { echo "FAIL: could not read the import output" >&2; exit 1; }
secret="$(openssl rand -hex 24)"
python3 scripts/smoke/set-secret.py "$secret" "${COMPOSE[@]}" run --rm api sepay set-secret --tenant demo >/dev/null

echo "== working the guesthouse through the API"
python3 scripts/demo/populate.py "$BASE" "$log" "$hook" "$secret" 1017588888

cat <<OUT

================ DEMO READY ================
App               $BASE/vi/   (English: $BASE/en/)
Guesthouse code   demo
Owner             owner  PIN 482915
Manager           mina   PIN 739106
Receptionist A    linh   PIN 260814   (edits building A, views B)
Receptionist B    viv    PIN 731902   (edits building B, views A)
Housekeeping      hoa    PIN 846205
SePay webhook     $hook   secret $secret
Reset again       make demo-reset   (everything is deleted first)
============================================
OUT
