#!/usr/bin/env bash
# `make rehearse-down`: stop the rehearsal stack and delete its database, bucket, secrets file and local dumps.
set -euo pipefail
cd "$(dirname "$0")/.."
export REHEARSE_PORT="${REHEARSE_PORT:-18090}"
[ -f deploy/.env.rehearse ] || touch deploy/.env.rehearse # compose still wants the file to exist
docker compose -p stayguard-rehearse -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml --env-file deploy/.env.rehearse --profile backup down -v --remove-orphans
rm -rf deploy/.env.rehearse deploy/.rehearse-backups
echo "rehearsal removed"
