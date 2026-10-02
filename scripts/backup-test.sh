#!/usr/bin/env bash
# `make backup-test`: start a throwaway stack with rclone's built-in S3 server (official rclone image) as the backup target, put a test guesthouse in the database, take a
# backup, list it from the bucket, restore it into a scratch database and check that every table has the same row count.
# Needs docker. EXTERNAL_S3=1 skips the local S3 server and uses the S3 server already listening at BACKUP_S3_ENDPOINT (bucket must exist). Ports: db 15433, S3 19000; project stayguard-backup-test.
set -euo pipefail
cd "$(dirname "$0")/.."

export DB_HOST_PORT=15433 BACKUP_S3_PORT=19000
export BACKUP_S3_ACCESS_KEY="stayguard-test" BACKUP_S3_SECRET_KEY="stayguard-test-secret" BACKUP_S3_BUCKET="stayguard-backups"
export BACKUP_S3_ENDPOINT="${BACKUP_S3_ENDPOINT:-http://localhost:$BACKUP_S3_PORT}" BACKUP_S3_PROVIDER="${BACKUP_S3_PROVIDER:-Other}" BACKUP_S3_REGION=us-east-1
export STAYGUARD_COMPOSE="docker compose -p stayguard-backup-test -f deploy/compose.yaml --profile backup"
export BACKUP_DIR="$(mktemp -d)" COMPARE_LIVE=1 ENV_FILE=/dev/null
read -r -a COMPOSE <<<"$STAYGUARD_COMPOSE"
tenant_file="$(mktemp)"
cleanup() {
	status=$?
	"${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
	rm -rf "$BACKUP_DIR" "$tenant_file"
	exit "$status"
}
trap cleanup EXIT

make deploy/.env.local >/dev/null
"${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
echo "== starting the database and the S3 server"
if [ "${EXTERNAL_S3:-0}" = 1 ]; then
	"${COMPOSE[@]}" up -d --build db >/dev/null
else
	"${COMPOSE[@]}" up -d --build db s3 >/dev/null
fi
"${COMPOSE[@]}" run --rm api migrate >/dev/null
sed 's/"guesthouseCode": "smoke"/"guesthouseCode": "backuptest"/' scripts/smoke/tenant.json >"$tenant_file"
"${COMPOSE[@]}" run --rm -T -v "$tenant_file:/tmp/tenant.json:ro" api tenant import --file /tmp/tenant.json >/dev/null

# The scripts run rclone through this helper; use it here the same way.
# shellcheck disable=SC1091
. deploy/lib-backup.sh
for _ in $(seq 30); do # the bucket is made when the S3 server starts
	rc lsf "$BACKUP_REMOTE" >/dev/null 2>&1 && break
	sleep 1
done

echo "== backup"
deploy/backup.sh
echo "== bucket listing"
listing="$(rc lsf "$BACKUP_REMOTE" --include 'stayguard-*.dump')"
echo "$listing"
[ -n "$listing" ] || { echo "FAIL: no dump in the bucket" >&2; exit 1; }
echo "== restore rehearsal"
deploy/restore-rehearsal.sh
echo "PASS: backup-test"
