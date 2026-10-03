#!/usr/bin/env bash
# Moves the time of ONE invoice's records back in the rehearsal database, so the 15-minute and 30-minute alert rules can be tested
# without waiting. It refuses to run against this clone's rehearse compose project ($REHEARSE_PROJECT, stayguard-rehearse in the main clone; `make rehearse` / `make rehearse-test`)
# and only touches the guesthouse you name, which must be a rehearsal one (code `rehearse` or `rh` + 6 hex digits).
#
#   RH_TENANT=rh1a2b3c scripts/rehearsal-backdate.sh partial PH1003A101 20   # bank money first arrived 20 minutes ago
#   RH_TENANT=rh1a2b3c scripts/rehearsal-backdate.sh unpaid  PH1003A101 40   # checked out 40 minutes ago (unpaid AND refund alerts)
#
# partial: payment_events.received_at of that invoice's PARTIAL events, minus N minutes   (raises PAYMENT_PARTIAL after 15)
# unpaid:  invoices.created_at (the check-out time), minus N minutes                        (raises PAYMENT_UNPAID or REFUND_PENDING after 30)
# stay:    stays.check_in_at and check_out_at, minus N minutes                                 (guest ID retention: 40 days = 57600)
# checkin: stays.check_in_at of a stay still IN the room (second argument is the stay id), minus N minutes (early / late bill lines)
# payment: payments.created_at of that bill's payments, minus N minutes                       (the QR expires after qrExpiryMinutes)
# Then run the jobs once: `docker compose -p $REHEARSE_PROJECT ... run --rm api jobs run` (rehearse-test does it) and read the alerts.
set -euo pipefail
cd "$(dirname "$0")/.."

. scripts/rehearse-env.sh
PROJECT="$REHEARSE_PROJECT" # this clone's rehearse stack; nothing else
kind="${1:-}"; bill="${2:-}"; mins="${3:-}"; code="${RH_TENANT:-}"
[ -n "$kind" ] && [ -n "$bill" ] && [ -n "$mins" ] && [ -n "$code" ] || { sed -n 2,16p "$0" >&2; exit 2; }
case "$kind" in partial | unpaid | stay | checkin | payment) ;; *) echo "kind must be partial, unpaid, stay, checkin or payment" >&2; exit 2 ;; esac
[[ "$mins" =~ ^[0-9]{1,6}$ ]] || { echo "minutes must be a whole number" >&2; exit 2; }
[[ "$bill" =~ ^[A-Za-z0-9_]{4,40}$ ]] || { echo "not a bill code or stay id: $bill" >&2; exit 2; }
[[ "$code" =~ ^(rehearse|rh[0-9a-f]{6})$ ]] || { echo "REFUSED: $code is not a rehearsal guesthouse" >&2; exit 1; }
if [ -n "${COMPOSE_PROJECT_NAME:-}" ] && [ "$COMPOSE_PROJECT_NAME" != "$PROJECT" ]; then
	echo "REFUSED: COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT_NAME, this only runs on $PROJECT" >&2; exit 1
fi

COMPOSE=(docker compose -p "$PROJECT" -f deploy/compose.prod.yaml -f deploy/compose.rehearse.yaml --env-file "${RH_ENV_FILE:-$REHEARSE_ENV_FILE}")
db="$("${COMPOSE[@]}" ps -q db 2>/dev/null | head -n 1)"
[ -n "$db" ] || { echo "REFUSED: no running db in compose project $PROJECT" >&2; exit 1; }
label="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$db")"
[ "$label" = "$PROJECT" ] || { echo "REFUSED: that database belongs to compose project '$label', not $PROJECT" >&2; exit 1; }

# The owner connection (superuser inside the container) so row security and the invoice-freezing rules do not get in the way; the
# session setting is local to this one connection.
sql() { "${COMPOSE[@]}" exec -T db psql -U stayguard -d stayguard -v ON_ERROR_STOP=1 -At -v code="$code" -v bill="$bill" -v mins="$mins" -f -; }
case "$kind" in
partial)
	out="$(sql <<'SQL'
SET session_replication_role = replica;
WITH hit AS (
  UPDATE app.payment_events pe SET received_at = pe.received_at - make_interval(mins => :mins)
  FROM app.invoices iv JOIN app.tenants t ON t.id = iv.tenant_id
  WHERE pe.invoice_id = iv.id AND pe.tenant_id = iv.tenant_id AND pe.result = 'PARTIAL'
    AND iv.bill_code = :'bill' AND t.guesthouse_code = :'code'
  RETURNING 1)
SELECT count(*) FROM hit;
SQL
)" ;;
unpaid)
	out="$(sql <<'SQL'
SET session_replication_role = replica;
WITH hit AS (
  UPDATE app.invoices iv SET created_at = iv.created_at - make_interval(mins => :mins)
  FROM app.tenants t
  WHERE t.id = iv.tenant_id AND iv.bill_code = :'bill' AND t.guesthouse_code = :'code'
  RETURNING 1)
SELECT count(*) FROM hit;
SQL
)" ;;
stay)
	out="$(sql <<'SQL'
SET session_replication_role = replica;
WITH hit AS (
  UPDATE app.stays s SET check_in_at = s.check_in_at - make_interval(mins => :mins), check_out_at = s.check_out_at - make_interval(mins => :mins)
  FROM app.invoices iv JOIN app.tenants t ON t.id = iv.tenant_id
  WHERE s.tenant_id = iv.tenant_id AND s.id = iv.stay_id AND iv.bill_code = :'bill' AND t.guesthouse_code = :'code'
  RETURNING 1)
SELECT count(*) FROM hit;
SQL
)" ;;
checkin)
	out="$(sql <<'SQL'
SET session_replication_role = replica;
WITH hit AS (
  UPDATE app.stays s SET check_in_at = s.check_in_at - make_interval(mins => :mins)
  FROM app.tenants t
  WHERE t.id = s.tenant_id AND s.id = :'bill' AND s.status = 'ACTIVE' AND t.guesthouse_code = :'code'
  RETURNING 1)
SELECT count(*) FROM hit;
SQL
)" ;;
payment)
	out="$(sql <<'SQL'
SET session_replication_role = replica;
WITH hit AS (
  UPDATE app.payments p SET created_at = p.created_at - make_interval(mins => :mins)
  FROM app.invoices iv JOIN app.tenants t ON t.id = iv.tenant_id
  WHERE p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND iv.bill_code = :'bill' AND t.guesthouse_code = :'code'
  RETURNING 1)
SELECT count(*) FROM hit;
SQL
)" ;;
esac
n="$(printf '%s\n' "$out" | tail -n 1)"
[ "$n" -ge 1 ] 2>/dev/null || { echo "nothing matched: $kind $bill in $code" >&2; exit 1; }
echo "backdated $n row(s): $kind $bill in $code by $mins minutes ($PROJECT)"
