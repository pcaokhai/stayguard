#!/usr/bin/env bash
# Visual sweep for Khai to look at (no assertions): the main screens of each role at 390, 834 and 1280 px, in vi and en, captured with
# agent-browser into docs/rehearsal/shots-<date>/<role>/<locale>/, plus an index.html contact sheet.
# Run it after `make rehearse-test` with RH_SHOTS=1, or alone: `RH_ENV_OUT=/tmp/rh.env make rehearse-test; . /tmp/rh.env; scripts/rehearsal-shots.sh`.
# Needs E2E_BASE_URL, RH_GUESTHOUSE and RH_PINS_FILE (the runner sets them), agent-browser, python3.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${E2E_BASE_URL:?run through make rehearse-test (RH_SHOTS=1) or source the RH_ENV_OUT file}" "${RH_GUESTHOUSE:?}" "${RH_PINS_FILE:?}"
command -v agent-browser >/dev/null || { echo "agent-browser not installed: npm i -g agent-browser && agent-browser install" >&2; exit 1; }
DATE="${RH_DATE:-$(date +%F)}"
OUT="docs/rehearsal/shots-$DATE"
SIZES=("390x844" "834x1112" "1280x900")
rm -rf "$OUT"; mkdir -p "$OUT"

# role:username:routes (the owner's detail pages need ids, found below)
declare -a ROLES=(
	"owner:owner:/owner /owner/rooms /owner/alerts /owner/transactions /owner/stays /owner/shifts /owner/activity /owner/reports /owner/expenses /owner/maintenance /owner/items /owner/staff /owner/access /owner/property /owner/buildings /owner/rates /owner/roster /owner/payroll /account"
	"manager:mina:/owner/rooms /owner/alerts /owner/maintenance /owner/stays /account"
	"receptionist:linh:/rooms /stays /shift /shift/payout /me/schedule /me/leave /account"
	"housekeeping:hk:/housekeeping /me/schedule /me/leave /account"
	"signed-out::/sign-in"
)
token() { python3 scripts/rehearsal-session.py "$1"; }
shot() { # role locale route size
	local w="${4%x*}" h="${4#*x}" slug
	slug="$(echo "$3" | sed 's|^/||; s|[/?=&]|_|g')"; slug="${slug:-home}"
	mkdir -p "$OUT/$1/$2"
	agent-browser set viewport "$w" "$h" >/dev/null
	agent-browser open "$E2E_BASE_URL/$2$3" >/dev/null
	# Screens poll the API, so "network idle" never comes; give the first data a moment instead.
	agent-browser wait 1200 >/dev/null 2>&1 || true
	agent-browser screenshot --full "$OUT/$1/$2/$slug-$w.png" >/dev/null
}

for entry in "${ROLES[@]}"; do
	role="${entry%%:*}"; rest="${entry#*:}"; user="${rest%%:*}"; routes="${rest#*:}"
	echo "== $role"
	agent-browser cookies clear >/dev/null 2>&1 || true
	if [ -n "$user" ]; then
		tok="$(token "$user")" || { echo "skip $role: no session" >&2; continue; }
		agent-browser cookies set sg_session "$tok" --url "$E2E_BASE_URL" --httpOnly >/dev/null
		# One stay detail page, from the newest stay the specs made.
		stay="$(curl -fsS -H "authorization: Bearer $tok" "$E2E_BASE_URL/v1/stays?date=$DATE" 2>/dev/null | python3 -c 'import json,sys; print((json.load(sys.stdin).get("items") or [{}])[0].get("id", ""))' 2>/dev/null || true)"
		if [ -n "$stay" ]; then
			case "$role" in owner | manager) routes="$routes /owner/stay?id=$stay" ;; receptionist) routes="$routes /stay?id=$stay" ;; esac
		fi
	fi
	for locale in vi en; do
		for route in $routes; do
			for size in "${SIZES[@]}"; do shot "$role" "$locale" "$route" "$size" || echo "  failed: $role $locale $route $size" >&2; done
		done
	done
done
agent-browser close >/dev/null 2>&1 || true
python3 scripts/rehearsal-index.py "$OUT"
echo "== shots: $OUT/index.html"
