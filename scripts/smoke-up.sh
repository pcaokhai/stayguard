#!/usr/bin/env bash
# Smoke_SG001_AC3: `make up` serves the placeholder on / and 200 on /healthz, then `make down`.
set -euo pipefail
cd "$(dirname "$0")/.."

BASE=${BASE:-http://localhost:8080}
WAIT_SECONDS=${WAIT_SECONDS:-60}
log=$(mktemp)

cleanup() { make down >/dev/null 2>&1 || true; kill "${up_pid:-0}" 2>/dev/null || true; rm -f "$log"; }
trap cleanup EXIT

make up >"$log" 2>&1 &
up_pid=$!

for _ in $(seq "$WAIT_SECONDS"); do
  curl -fsS "$BASE/healthz" >/dev/null 2>&1 && break
  sleep 1
done

health=$(curl -fsS "$BASE/healthz") || { echo "FAIL: /healthz unreachable" >&2; tail -n 20 "$log" >&2; exit 1; }
echo "$health" | grep -q '"ok"' || { echo "FAIL: /healthz body: $health" >&2; exit 1; }
curl -fsS "$BASE/" | grep -q "StayGuard" || { echo "FAIL: / lacks placeholder" >&2; exit 1; }
echo "PASS: Smoke_SG001_AC3"
