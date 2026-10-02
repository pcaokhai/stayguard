#!/usr/bin/env bash
# Daily database backup: dump, check the dump can be read, keep it here for a few days and copy it off the server.
# Run from the repository root by cron (see deploy/README.md). Needs docker compose and rclone with a remote set in BACKUP_REMOTE.
# The dump holds guest names and phones in the clear (ID numbers, photos and bank accounts are encrypted by the app), so the
# remote must be private; an rclone "crypt" remote encrypts it on the way out.
set -euo pipefail

cd "$(dirname "$0")/.."
ENV_FILE="${ENV_FILE:-deploy/.env.prod}"
COMPOSE=(docker compose -f deploy/compose.prod.yaml --env-file "$ENV_FILE")
# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

: "${BACKUP_REMOTE:?set BACKUP_REMOTE in $ENV_FILE (an rclone remote and folder, e.g. offsite:stayguard-backups)}"
keep_local="${BACKUP_KEEP_LOCAL_DAYS:-7}"
keep_remote="${BACKUP_KEEP_REMOTE_DAYS:-60}"
dir="${BACKUP_DIR:-/var/backups/stayguard}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$dir/stayguard-$stamp.dump"

mkdir -p "$dir"
umask 077

# Custom format: compressed, and pg_restore can restore parts of it. A partial file never keeps its final name.
"${COMPOSE[@]}" exec -T db pg_dump -U stayguard -d stayguard --format=custom --no-owner > "$file.partial"
mv "$file.partial" "$file"

# A dump that cannot be listed is not a backup.
size="$(wc -c < "$file" | tr -d ' ')"
if [ "$size" -lt 10000 ]; then
	echo "backup too small ($size bytes): $file" >&2
	exit 1
fi
"${COMPOSE[@]}" exec -T db pg_restore --list < "$file" > /dev/null

rclone copyto "$file" "$BACKUP_REMOTE/$(basename "$file")"
rclone check "$dir" "$BACKUP_REMOTE" --one-way --include "$(basename "$file")" > /dev/null

find "$dir" -name 'stayguard-*.dump' -mtime "+$keep_local" -delete
rclone delete "$BACKUP_REMOTE" --min-age "${keep_remote}d" --include 'stayguard-*.dump'

echo "backup ok: $(basename "$file") ($size bytes) copied to $BACKUP_REMOTE"
