#!/usr/bin/env bash
# `make rehearse-test`: the rehearsal checklist as Playwright specs (web/e2e/rehearsal), run against the rehearse stack (project
# stayguard-rehearse, plain HTTP on 127.0.0.1:$REHEARSE_PORT, default 18090) with a FRESH guesthouse every run. No clicking, no real
# money, no real keys: payments arrive as signed webhooks, time-based cases backdate the rehearsal database only.
# Writes docs/rehearsal/results-<date>.csv and docs/rehearsal/evidence-<date>/ (screenshots, API logs).
#   RH_ONLY='TT-0'     only the tests whose title matches this regular expression
#   RH_SHOTS=1         afterwards, also the visual sweep (scripts/rehearsal-shots.sh)
#   RH_ENV_OUT=file    only prepare: write `export RH_...` lines to file (and keep the pins file) so a spec can be run by hand
#   RH_KEEP_JOBS=1     leave the jobs service running (it is stopped for the run so it cannot raise the alerts the specs count)
# Needs docker, python3, openssl, and `npm ci` done in web/. Never put real guest data in the stack.
set -euo pipefail
cd "$(dirname "$0")/.."

PORT="${REHEARSE_PORT:-18090}"
export REHEARSE_PORT="$PORT" REHEARSE_S3_PORT="${REHEARSE_S3_PORT:-19100}"
BASE="http://localhost:$PORT"
ENV_FILE="deploy/.env.rehearse" # git-ignored; the same file `make rehearse` makes, so the key matches the data volume
COMPOSE=(docker compose -p stayguard-rehearse -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml --env-file "$ENV_FILE" --profile backup)
DATE="$(date +%F)"
OUT="docs/rehearsal"
mkdir -p "$OUT"

if [ ! -s "$ENV_FILE" ]; then
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

log="$(mktemp)"; tenant_file="$(mktemp)"; pins_file="$(mktemp)"
cleanup() {
	status=$?
	rm -f "$log" "$tenant_file"
	[ -n "${RH_ENV_OUT:-}" ] || rm -f "$pins_file.set"
	[ -n "${RH_ENV_OUT:-}" ] || rm -f "$pins_file"
	if [ "${RH_KEEP_JOBS:-0}" != 1 ]; then "${COMPOSE[@]}" up -d jobs >/dev/null 2>&1 || true; fi
	exit "$status"
}
trap cleanup EXIT

echo "== starting or reusing the rehearse stack on :$PORT"
if ! "${COMPOSE[@]}" up -d --build db api jobs >"$log" 2>&1; then
	# A broken image build must not hide itself. Retry once with a rehearsal-only Dockerfile that also copies contracts/ (web tests import
	# ../contracts/*.json, which the product Dockerfile leaves out); say loudly that main does not build as it stands.
	echo "WARNING: the product image build FAILED on this commit:" >&2
	grep -i -E 'error TS|ERROR' "$log" | tail -n 3 >&2
	fix="$(mktemp -d)"
	sed 's|^COPY web/ ./$|COPY web/ ./\nCOPY contracts/ /contracts/|' deploy/Dockerfile >"$fix/Dockerfile"
	printf 'services:\n  api:\n    build:\n      dockerfile: %s/Dockerfile\n  migrate:\n    build:\n      dockerfile: %s/Dockerfile\n  jobs:\n    build:\n      dockerfile: %s/Dockerfile\n' "$fix" "$fix" "$fix" >"$fix/override.yaml"
	if "${COMPOSE[@]}" -f "$fix/override.yaml" up -d --build db api jobs >"$log" 2>&1; then
		echo "WARNING: built with the rehearsal-only Dockerfile fix (contracts/ copied); the results are for this commit anyway." >&2
	else
		echo "WARNING: still no build; testing the images already running, which are NOT this commit." >&2
		"${COMPOSE[@]}" up -d db api jobs >/dev/null 2>&1 || true
	fi
	rm -rf "$fix"
fi
for _ in $(seq 120); do curl -fsS "$BASE/readyz" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS "$BASE/readyz" >/dev/null || { echo "FAIL: the API did not become ready" >&2; "${COMPOSE[@]}" logs --tail 20 api >&2; exit 1; }
# The jobs service would raise the partial and unpaid alerts on its own clock; the specs raise them on purpose and count them.
"${COMPOSE[@]}" stop jobs >/dev/null 2>&1 || true

code="rh$(openssl rand -hex 3)"
python3 - "$tenant_file" "$code" <<'PY'
import json, sys
t = json.load(open("scripts/smoke/tenant.json"))
t["guesthouseCode"] = sys.argv[2]
t["name"] = t["property"]["name"] = "Rehearsal Test " + sys.argv[2]
t["bankAccount"]["accountName"] = "REHEARSAL TEST"
t["buildings"] = [{"code": "A", "name": "Building A", "floors": 3, "roomsPerFloor": 40}]
contract = {"payType": "MONTHLY", "rate": 6000000, "fixedAllowance": 0, "standardShifts": 26, "startDate": "2026-01-01", "annualLeaveDays": 12}
# One receptionist per concern (r1..r12) so shift cases never share a drawer, and one housekeeper.
for i in range(1, 13):
    t["staff"].append({"name": f"Rehearsal R{i}", "username": f"r{i}", "position": "FRONT_DESK", "appAccess": "RECEPTIONIST",
                       "contract": contract, "buildingAccess": {"A": "EDIT"}})
t["staff"].append({"name": "Rehearsal Housekeeper", "username": "hk", "position": "HOUSEKEEPING", "appAccess": "HOUSEKEEPING",
                   "contract": contract, "buildingAccess": {"A": "EDIT"}})
json.dump(t, open(sys.argv[1], "w"))
PY
echo "== creating the guesthouse $code with the installer commands"
"${COMPOSE[@]}" run --rm -T -v "$tenant_file:/tmp/tenant.json:ro" api tenant import --file /tmp/tenant.json >"$log" 2>&1 \
	|| { echo "FAIL: tenant import" >&2; grep -v -i 'pin' "$log" | tail -n 10 >&2; exit 1; }
hook="$(awk '/^webhook path/ { print $3 }' "$log")"
python3 - "$log" "$pins_file" <<'PY'
import json, re, sys
pins = {}
for line in open(sys.argv[1]):
    p = line.split()
    if len(p) >= 2 and re.fullmatch(r"[A-Za-z0-9_]+", p[0]) and re.fullmatch(r"\d{6}", p[1]):
        pins[p[0]] = p[1]
json.dump(pins, open(sys.argv[2], "w"))
PY
[ -n "$hook" ] && grep -q '"owner"' "$pins_file" || { echo "FAIL: could not read the import output" >&2; exit 1; }
secret="$(openssl rand -hex 24)"
python3 scripts/smoke/set-secret.py "$secret" "${COMPOSE[@]}" run --rm api sepay set-secret --tenant "$code" >/dev/null

# Handed to the specs; the compose words let a spec restart the api and run `jobs run` through the same project.
export E2E_BASE_URL="$BASE" RH_GUESTHOUSE="$code" RH_HOOK_PATH="$hook" RH_SEPAY_SECRET="$secret" RH_ACCOUNT_NO=1017588888 \
	RH_PINS_FILE="$pins_file" RH_DATE="$DATE" RH_OUT="$PWD/$OUT" RH_ROOT="$PWD" RH_ENV_FILE="$ENV_FILE"
rm -rf "$OUT/evidence-$DATE"
if [ -n "${RH_ENV_OUT:-}" ]; then
	(umask 077; env | grep '^\(E2E_BASE_URL\|RH_\)' | grep -v '^RH_ENV_OUT' | sed 's/^/export /' >"$RH_ENV_OUT")
	echo "== prepared; source $RH_ENV_OUT then run: cd web && npx playwright test -c e2e/rehearsal/playwright.config.ts"
	exit 0
fi

echo "== running the rehearsal specs (tenant $code)"
cd web
set +e
npx playwright test -c e2e/rehearsal/playwright.config.ts ${RH_ONLY:+--grep "$RH_ONLY"} 2>&1 | tail -n 30
rc=${PIPESTATUS[0]}
set -e
cd ..
rm -rf "$OUT/evidence-$DATE/_pw"
echo "== results: $OUT/results-$DATE.csv"
python3 - "$OUT/results-$DATE.csv" <<'PY'
import collections, csv, sys
n = collections.Counter(r["status"] for r in csv.DictReader(open(sys.argv[1])))
print(f"pass {n['pass']}  fail {n['fail']}  skip {n['skip']}")
PY
if [ "${RH_SHOTS:-0}" = 1 ]; then scripts/rehearsal-shots.sh; fi
exit "$rc"
