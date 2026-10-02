#!/usr/bin/env bash
# Restore rehearsal: bring a backup back into a SCRATCH database, check it, and drop it. The live database is never touched.
# Do this after setting up backups and once a month: a backup nobody has restored is only a hope.
#   deploy/restore-rehearsal.sh                  newest dump from the remote
#   deploy/restore-rehearsal.sh <file.dump>      a dump already on this server
set -euo pipefail

cd "$(dirname "$0")/.."
ENV_FILE="${ENV_FILE:-deploy/.env.prod}"
COMPOSE=(docker compose -f deploy/compose.prod.yaml --env-file "$ENV_FILE")
# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

SCRATCH=stayguard_rehearsal
psql_admin() { "${COMPOSE[@]}" exec -T db psql -U stayguard -d postgres -v ON_ERROR_STOP=1 "$@"; }
psql_scratch() { "${COMPOSE[@]}" exec -T db psql -U stayguard -d "$SCRATCH" -v ON_ERROR_STOP=1 -At "$@"; }

tmp=""
if [ "${1:-}" != "" ]; then
	dump="$1"
else
	: "${BACKUP_REMOTE:?set BACKUP_REMOTE in $ENV_FILE}"
	latest="$(rclone lsf "$BACKUP_REMOTE" --include 'stayguard-*.dump' | sort | tail -n 1)"
	[ -n "$latest" ] || { echo "no dump on $BACKUP_REMOTE" >&2; exit 1; }
	tmp="$(mktemp -d)"
	dump="$tmp/$latest"
	rclone copyto "$BACKUP_REMOTE/$latest" "$dump"
fi
cleanup() {
	psql_admin -c "DROP DATABASE IF EXISTS $SCRATCH" > /dev/null 2>&1 || true
	[ -z "$tmp" ] || rm -rf "$tmp"
}
trap cleanup EXIT

echo "restoring $(basename "$dump") into scratch database $SCRATCH"
psql_admin -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH" > /dev/null
"${COMPOSE[@]}" exec -T db pg_restore -U stayguard -d "$SCRATCH" --no-owner --exit-on-error < "$dump"

tables="$(psql_scratch -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'app'")"
migrations="$(psql_scratch -c "SELECT count(*) FROM goose_db_version WHERE is_applied")"
tenants="$(psql_scratch -c "SELECT count(*) FROM app.tenants")"
invoices="$(psql_scratch -c "SELECT count(*) FROM app.invoices")"
audit="$(psql_scratch -c "SELECT count(*) FROM app.audit_logs")"

# The restored schema must be the one the code expects: tables exist and the migration history is there.
[ "$tables" -ge 20 ] || { echo "restore looks wrong: only $tables tables in schema app" >&2; exit 1; }
[ "$migrations" -ge 10 ] || { echo "restore looks wrong: only $migrations migrations recorded" >&2; exit 1; }

echo "restore rehearsal ok: $tables tables, $migrations migrations, $tenants tenants, $invoices invoices, $audit audit rows"
echo "(the scratch database is dropped now; the live database was not touched)"
