# Shared by backup.sh and restore-rehearsal.sh: how to run docker compose, and how to run rclone.
# shellcheck shell=bash

# docker compose command: production by default; STAYGUARD_COMPOSE overrides it (the local backup test points it at deploy/compose.yaml).
if [ -n "${STAYGUARD_COMPOSE:-}" ]; then
	read -r -a COMPOSE <<<"$STAYGUARD_COMPOSE"
else
	COMPOSE=(docker compose -f deploy/compose.prod.yaml --env-file "$ENV_FILE")
fi

# The remote: BACKUP_REMOTE is an rclone remote and folder (you configured rclone yourself), or it is built from BACKUP_S3_* as an
# S3-compatible remote named "s3backup" through rclone's environment configuration (no config file).
if [ -z "${BACKUP_REMOTE:-}" ] && [ -n "${BACKUP_S3_BUCKET:-}" ]; then
	: "${BACKUP_S3_ENDPOINT:?set BACKUP_S3_ENDPOINT}" "${BACKUP_S3_ACCESS_KEY:?set BACKUP_S3_ACCESS_KEY}" "${BACKUP_S3_SECRET_KEY:?set BACKUP_S3_SECRET_KEY}"
	export RCLONE_CONFIG_S3BACKUP_TYPE=s3
	export RCLONE_CONFIG_S3BACKUP_PROVIDER="${BACKUP_S3_PROVIDER:-Other}"
	export RCLONE_CONFIG_S3BACKUP_ENDPOINT="$BACKUP_S3_ENDPOINT"
	export RCLONE_CONFIG_S3BACKUP_ACCESS_KEY_ID="$BACKUP_S3_ACCESS_KEY"
	export RCLONE_CONFIG_S3BACKUP_SECRET_ACCESS_KEY="$BACKUP_S3_SECRET_KEY"
	export RCLONE_CONFIG_S3BACKUP_REGION="${BACKUP_S3_REGION:-}"
	export RCLONE_CONFIG_S3BACKUP_NO_CHECK_BUCKET=true
	BACKUP_REMOTE="s3backup:${BACKUP_S3_BUCKET}${BACKUP_S3_PREFIX:+/$BACKUP_S3_PREFIX}"
fi
: "${BACKUP_REMOTE:?set BACKUP_REMOTE (an rclone remote) or BACKUP_S3_BUCKET and the other BACKUP_S3_* settings}"

# rclone: the installed one, else the official Docker image with the same environment. Local files the command names are
# mounted at the same path; "localhost" in an S3 endpoint means the host, so the container reaches it as host.docker.internal.
export RCLONE_LOG_LEVEL="${RCLONE_LOG_LEVEL:-ERROR}" # the missing-config notice is expected: the remote comes from the environment
RCLONE_MOUNTS=()
rc() {
	if command -v rclone >/dev/null 2>&1; then
		rclone "$@"
		return
	fi
	local args=(run --rm --add-host host.docker.internal:host-gateway -e RCLONE_LOG_LEVEL)
	local var
	for var in $(env | grep -o '^RCLONE_CONFIG_[A-Z0-9_]*'); do
		args+=(-e "$var")
	done
	if [ -n "${RCLONE_CONFIG_S3BACKUP_ENDPOINT:-}" ]; then
		args+=(-e "RCLONE_CONFIG_S3BACKUP_ENDPOINT=$(printf '%s' "$RCLONE_CONFIG_S3BACKUP_ENDPOINT" | sed -E 's#//(localhost|127\.0\.0\.1)#//host.docker.internal#')")
	fi
	local m
	for m in "${RCLONE_MOUNTS[@]+"${RCLONE_MOUNTS[@]}"}"; do
		args+=(-v "$m:$m")
	done
	docker "${args[@]}" rclone/rclone:latest "$@"
}
