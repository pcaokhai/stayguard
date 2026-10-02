#!/usr/bin/env bash
# `make rehearse-down`: stop the rehearsal stack and delete its database, secrets file and local dumps.
set -euo pipefail
cd "$(dirname "$0")/.."
export REHEARSE_PORT="${REHEARSE_PORT:-18090}"
env_args=()
if [ -s deploy/.env.rehearse ]; then
	env_args=(--env-file deploy/.env.rehearse)
fi
# compose wants every required variable to interpolate the files even to take them down; placeholders are enough here.
POSTGRES_PASSWORD=x APP_DB_PASSWORD=x DATA_ENCRYPTION_KEY=x DOMAIN=x ACME_EMAIL=x BACKUP_S3_ACCESS_KEY=x BACKUP_S3_SECRET_KEY=x BACKUP_S3_BUCKET=x \
	docker compose -p stayguard-rehearse -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml "${env_args[@]+"${env_args[@]}"}" --profile backup down -v --remove-orphans
rm -rf deploy/.env.rehearse deploy/.rehearse-backups
echo "rehearsal removed"
