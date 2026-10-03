# Sourced by scripts/rehearse*.sh, rehearsal-backdate.sh and demo-check.sh, from the repository root. Not run on its own.
# Every clone of this repository gets its own rehearse stack, so two clones (two sessions) never share containers, volumes or a port:
#   REHEARSE_PROJECT   compose project:  stayguard-rehearse in the main clone (the directory named "stayguard"), else stayguard-rehearse-<dir>
#   REHEARSE_PORT      local port:       18090 in the main clone, else 18100 + a number made from the clone's path (the S3 test port too)
#   deploy/.env.rehearse is inside the clone, so each clone also has its own secrets file.
# Both can be set by hand. The run lock (deploy/.rehearse.lock) is per clone: one run at a time on one stack.
clone_name="$(basename "$PWD")"
clone_slug="$(printf '%s' "$clone_name" | tr 'A-Z' 'a-z' | tr -c 'a-z0-9\n' '-' | sed 's/^-*//; s/-*$//')"
clone_n=$(($(printf '%s' "$PWD" | cksum | cut -d' ' -f1) % 800))
if [ "$clone_name" = stayguard ]; then
	: "${REHEARSE_PROJECT:=stayguard-rehearse}" "${REHEARSE_PORT:=18090}" "${REHEARSE_S3_PORT:=19100}"
else
	: "${REHEARSE_PROJECT:=stayguard-rehearse-$clone_slug}" "${REHEARSE_PORT:=$((18100 + clone_n))}" "${REHEARSE_S3_PORT:=$((19200 + clone_n))}"
fi
export REHEARSE_PROJECT REHEARSE_PORT REHEARSE_S3_PORT
REHEARSE_SLUG="${clone_slug:-stayguard}" # in result file names: results-<date>-<slug>.csv, so two clones never write the same file
export REHEARSE_SLUG
REHEARSE_ENV_FILE="deploy/.env.rehearse"
REHEARSE_LOCK_DIR="deploy/.rehearse.lock"

# rehearse_lock <what is running>: take the run lock or exit 3, saying who holds it and since when. A child of the holder (demo-check
# runs rehearse-test) passes through, because it inherits REHEARSE_LOCK_PID. A lock whose process is gone is taken over.
rehearse_lock() {
	if [ -n "${REHEARSE_LOCK_PID:-}" ] && [ "$(cat "$REHEARSE_LOCK_DIR/pid" 2>/dev/null)" = "$REHEARSE_LOCK_PID" ]; then return 0; fi
	if ! mkdir "$REHEARSE_LOCK_DIR" 2>/dev/null; then
		[ -s "$REHEARSE_LOCK_DIR/pid" ] || sleep 1 # the holder is writing its details
		holder="$(cat "$REHEARSE_LOCK_DIR/pid" 2>/dev/null || true)"
		if [ -n "$holder" ] && kill -0 "$holder" 2>/dev/null; then
			echo "REFUSED: $1 cannot start: the rehearse stack ($REHEARSE_PROJECT, port $REHEARSE_PORT) is in use by $(cat "$REHEARSE_LOCK_DIR/what" 2>/dev/null) (pid $holder, $(cat "$REHEARSE_LOCK_DIR/who" 2>/dev/null)), since $(cat "$REHEARSE_LOCK_DIR/since" 2>/dev/null). Wait for it to finish." >&2
			exit 3
		fi
		rm -rf "$REHEARSE_LOCK_DIR"
		mkdir "$REHEARSE_LOCK_DIR" 2>/dev/null || { echo "REFUSED: $1 cannot take the rehearse lock" >&2; exit 3; }
	fi
	echo "$$" >"$REHEARSE_LOCK_DIR/pid"
	echo "$1" >"$REHEARSE_LOCK_DIR/what"
	echo "$(id -un)@$(hostname -s)" >"$REHEARSE_LOCK_DIR/who"
	date '+%Y-%m-%d %H:%M:%S' >"$REHEARSE_LOCK_DIR/since"
	export REHEARSE_LOCK_PID="$$"
}
# rehearse_unlock: release the lock if this process took it (call it from the script's exit trap).
rehearse_unlock() {
	if [ "$(cat "$REHEARSE_LOCK_DIR/pid" 2>/dev/null)" = "$$" ]; then rm -rf "$REHEARSE_LOCK_DIR"; fi
}
