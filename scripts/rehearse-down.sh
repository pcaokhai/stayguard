#!/usr/bin/env bash
# `make rehearse-down`: stop this clone's rehearsal stack and delete its database, secrets file and local dumps.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/rehearse-env.sh
rehearse_lock "make rehearse-down" # never wipe a stack a run is using
trap rehearse_unlock EXIT
env_args=()
if [ -s "$REHEARSE_ENV_FILE" ]; then
	env_args=(--env-file "$REHEARSE_ENV_FILE")
fi
# compose wants every required variable to interpolate the files even to take them down; placeholders are enough here.
POSTGRES_PASSWORD=x APP_DB_PASSWORD=x DATA_ENCRYPTION_KEY=x DOMAIN=x ACME_EMAIL=x BACKUP_S3_ACCESS_KEY=x BACKUP_S3_SECRET_KEY=x BACKUP_S3_BUCKET=x \
	docker compose -p "$REHEARSE_PROJECT" -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml "${env_args[@]+"${env_args[@]}"}" --profile backup down -v --remove-orphans
rm -rf "$REHEARSE_ENV_FILE" deploy/.rehearse-backups
echo "rehearsal removed"
