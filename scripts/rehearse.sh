#!/usr/bin/env bash
# `make rehearse`: the production compose on this machine (DEMO_MODE off, non-superuser DB role, MinIO backup target), plain HTTP on
# 127.0.0.1:$REHEARSE_PORT (default 18090) for a Cloudflare tunnel, with a fake test guesthouse imported through the installer
# commands. Prints the sign-in codes, the one-time PINs and the webhook path ONCE: they are not stored anywhere readable later.
#   make rehearse-down          stop and delete everything (database, bucket, secrets file)
#   REHEARSE_PORT=18090         the local port
#   SEPAY_SECRET=...            the secret SePay signs with (else a random one is made and printed once)
#   REHEARSE_ACCOUNT_NO / REHEARSE_BANK_BIN / REHEARSE_ACCOUNT_NAME   receiving account for the QR (else a fake one)
#   EXTERNAL_S3=1               skip MinIO and use the S3 server already at BACKUP_S3_ENDPOINT
# Needs docker, python3, openssl. Never use it for real guest data.
set -euo pipefail
cd "$(dirname "$0")/.."

PORT="${REHEARSE_PORT:-18090}"
export REHEARSE_PORT="$PORT" REHEARSE_MINIO_PORT="${REHEARSE_MINIO_PORT:-19100}"
ENV_FILE="deploy/.env.rehearse" # git-ignored (.env.*); generated once so the key matches the data volume
CODE="rehearse"
COMPOSE=(docker compose -p stayguard-rehearse -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml --env-file "$ENV_FILE" --profile backup)

if [ ! -f "$ENV_FILE" ]; then
	umask 077
	cat >"$ENV_FILE" <<ENV
POSTGRES_PASSWORD=$(openssl rand -hex 16)
APP_DB_PASSWORD=$(openssl rand -hex 16)
DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
DOMAIN=localhost
ACME_EMAIL=rehearse@example.invalid
BACKUP_S3_ACCESS_KEY=rehearse-$(openssl rand -hex 4)
BACKUP_S3_SECRET_KEY=$(openssl rand -hex 16)
BACKUP_S3_BUCKET=stayguard-rehearse
ENV
fi
set -a; . "$ENV_FILE"; set +a

if [ "${EXTERNAL_S3:-0}" = 1 ]; then
	: "${BACKUP_S3_ENDPOINT:?EXTERNAL_S3=1 needs BACKUP_S3_ENDPOINT}"
	services=(db api)
else
	export BACKUP_S3_ENDPOINT="http://localhost:$REHEARSE_MINIO_PORT" BACKUP_S3_PROVIDER=Minio BACKUP_S3_REGION=us-east-1
	services=(db api minio minio-init)
fi

echo "== starting the production stack (first build takes a few minutes)"
"${COMPOSE[@]}" up -d --build "${services[@]}" >/dev/null
for _ in $(seq 90); do
	curl -fsS "http://localhost:$PORT/readyz" >/dev/null 2>&1 && break
	sleep 1
done
curl -fsS "http://localhost:$PORT/readyz" >/dev/null || { echo "FAIL: the API did not become ready" >&2; "${COMPOSE[@]}" logs api | tail -n 20 >&2; exit 1; }
if curl -fsS "http://localhost:$PORT/vi/" | grep -qi "demo"; then echo "FAIL: the demo role picker is in this build" >&2; exit 1; fi

tenant_file="$(mktemp)"; log="$(mktemp)"
trap 'rm -f "$tenant_file" "$log"' EXIT
python3 - "$tenant_file" <<'PY'
import json, os, sys
t = json.load(open("scripts/smoke/tenant.json"))
t["guesthouseCode"] = "rehearse"
t["name"] = t["property"]["name"] = "Rehearsal Guesthouse"
acc = t["bankAccount"]
acc["bankBin"] = os.environ.get("REHEARSE_BANK_BIN", acc["bankBin"])
acc["accountNo"] = os.environ.get("REHEARSE_ACCOUNT_NO", acc["accountNo"])
acc["accountName"] = os.environ.get("REHEARSE_ACCOUNT_NAME", "REHEARSAL GUESTHOUSE")
t["owner"]["name"] = "Rehearsal Owner"
t["staff"][0]["name"] = "Rehearsal Receptionist"
json.dump(t, open(sys.argv[1], "w"))
PY

echo "== importing the test guesthouse '$CODE'"
if ! "${COMPOSE[@]}" run --rm -T -v "$tenant_file:/tmp/tenant.json:ro" api tenant import --file /tmp/tenant.json >"$log" 2>&1; then
	echo "FAIL: tenant import (already imported? 'make rehearse-down' starts clean). Output:" >&2
	grep -v -i 'pin' "$log" | tail -n 10 >&2
	exit 1
fi
pin() { awk -v u="$1" '$1 == u { print $2 }' "$log"; }
hook="$(awk '/^webhook path/ { print $3 }' "$log")"
owner_pin="$(pin owner)"; desk_pin="$(pin linh)"
[ -n "$hook" ] && [ -n "$owner_pin" ] && [ -n "$desk_pin" ] || { echo "FAIL: could not read the import output" >&2; exit 1; }

secret="${SEPAY_SECRET:-$(openssl rand -hex 24)}"
python3 scripts/smoke/set-secret.py "$secret" "${COMPOSE[@]}" run --rm api sepay set-secret --tenant "$CODE" >/dev/null

cat <<OUT

================ REHEARSAL READY (shown once) ================
App (local)        http://localhost:$PORT/vi/
Guesthouse code    $CODE
Owner              user: owner   one-time PIN: $owner_pin
Receptionist       user: linh    one-time PIN: $desk_pin
                   (each asks for a new PIN at first sign-in)

SePay webhook path $hook
  Full URL         https://<your-tunnel-host>$hook
  Secret in SePay  $secret
Tunnel             cloudflared tunnel --url http://localhost:$PORT
Receiving account  ${REHEARSE_ACCOUNT_NO:-1017588888} (the QR pays this one; set REHEARSE_ACCOUNT_NO for your SePay account)
Stop and wipe      make rehearse-down
==============================================================

OUT

echo "== backup check (MinIO)"
ENV_FILE="$ENV_FILE" STAYGUARD_COMPOSE="${COMPOSE[*]}" BACKUP_DIR="$PWD/deploy/.rehearse-backups" deploy/backup.sh
