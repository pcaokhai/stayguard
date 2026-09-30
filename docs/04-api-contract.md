# API Contract

Version 1.0 · 2026-09-30 · Owner: Tech lead
Normative source: `contracts/openapi.yaml` (OpenAPI 3.1), `contracts/events/payment-events.schema.json`

## 1. Topology

One origin. The Go binary serves the web app at `/vi/*` and `/en/*` and the API at `/v1/*`, plus `/healthz`, `/readyz` and internal `/metrics`. There is no BFF. Realtime uses Server-Sent Events on `/v1/payments/{paymentId}/events` (ADR-012).

## 2. Conventions

| Topic | Rule |
| --- | --- |
| Versioning | Major version in the URL (`/v1`); breaking changes need `/v2` and an ADR |
| Media type | `application/json`; errors `application/problem+json`; events `text/event-stream` |
| Naming | JSON camelCase; enums SCREAMING_SNAKE_CASE; operationIds camelCase verbs |
| IDs | Opaque strings (prefixed ULIDs); clients never parse them |
| Money | Integer whole VND in fields typed `Vnd`; never a float; negative only for differences |
| Time | RFC 3339 with offset; the server supplies check-in, check-out and payment times, clients never do |
| Idempotency | Header `Idempotency-Key` (UUID) required on the writes that declare it |
| Pagination | Lists bounded by domain size are returned whole (`items`); unbounded lists, when added, use cursor pagination (`limit`, `cursor`, `nextCursor`) |
| Filtering | Query parameters, documented per operation |
| Tracing | Request header `traceparent` accepted; response header `X-Trace-Id` always set |
| Actor and audit | Actor comes from the session, never from a header or body |
| Sensitive data | ID numbers are write-only (masked on read); QR payload and tokens are never logged |
| Localisation | The API returns codes and structured values; owner-entered names are `{vi, en}` objects; the web app translates |

### 2.3 Authorization matrix

Owner has implicit EDIT on every building. "Building access" is checked against the building of the room, stay, invoice or payment being touched, on every request (ADR-008).

| operationId | Roles | Minimum building access |
| --- | --- | --- |
| createDemoSession, getHealth, getReadiness | public (session endpoint only when DEMO_MODE) | none |
| receiveBankWebhook | signature header | none |
| getMe, setMyLocale | any signed-in role | none |
| listBuildings | any role | buildings with NONE are omitted |
| listRooms, getRoom | any role | VIEW |
| listServices | OWNER, RECEPTIONIST | none |
| createStay, addStayExtras, checkoutStay | OWNER, RECEPTIONIST | EDIT |
| getStay | OWNER, RECEPTIONIST | VIEW |
| createPayment, simulatePaymentReceived | OWNER, RECEPTIONIST | EDIT |
| getPayment, streamPaymentEvents | OWNER, RECEPTIONIST | VIEW |
| listHousekeepingTasks | OWNER, RECEPTIONIST, HOUSEKEEPING | VIEW (only buildings with VIEW or more) |
| completeHousekeepingTask | OWNER, HOUSEKEEPING | EDIT |
| reportRoomUsage | OWNER, RECEPTIONIST, HOUSEKEEPING | EDIT |
| getCurrentShift, closeShift, recordCashPayout | OWNER, RECEPTIONIST | EDIT on at least one building |
| getOwnerOverview, listStaffPermissions, setBuildingPermission, getShiftReview | OWNER | not applicable |

A caller without the role gets 403 `ROLE_FORBIDDEN`; without the building access, 403 `BUILDING_FORBIDDEN`. Another tenant's ids always return 404.

## 3. Error model

```json
{
  "type": "https://stayguard.example/problems/room-not-vacant",
  "title": "Room is not vacant",
  "status": 409,
  "code": "ROOM_NOT_VACANT",
  "detail": "Room A102 already has an active stay.",
  "instance": "/v1/rooms/room_01J8/stays",
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

| `code` | Status | When |
| --- | --- | --- |
| VALIDATION_FAILED | 422 | Body or query fails the contract; `errors[]` lists fields |
| UNAUTHENTICATED, SESSION_EXPIRED | 401 | Missing, invalid or expired token |
| ROLE_FORBIDDEN | 403 | Role not allowed for the operation |
| BUILDING_FORBIDDEN | 403 | Access to the building is below what the operation needs |
| NOT_FOUND | 404 | Unknown id or an id from another tenant |
| DEMO_DISABLED | 404 | `/v1/demo/*` called while DEMO_MODE is off |
| ROOM_NOT_VACANT | 409 | Check-in on a room that is not VACANT |
| STAY_NOT_ACTIVE | 409 | Extras or check-out on a stay that is not ACTIVE |
| INSUFFICIENT_STOCK | 409 | Extras exceed stock |
| INVOICE_ALREADY_PAID | 409 | Payment for a paid invoice |
| SHIFT_ALREADY_CLOSED | 409 | Close on a closed shift |
| IDEMPOTENCY_KEY_REUSED | 409 | Same key with a different request body |
| CASH_REASON_REQUIRED | 422 | Shift close with a cash gap and no reason |
| PRICING_INVALID_INTERVAL | 500 | Internal invariant: server-recorded check-out not after check-in (should be unreachable) |
| RATE_LIMITED | 429 | Trial creation limit; `Retry-After` set |
| INTERNAL | 500 | Unexpected; no detail leaked |
| SERVICE_UNAVAILABLE | 503 | Database not reachable |

Not HTTP errors: a transfer of the wrong amount (payment `MISMATCH`), a cash gap at shift close (data plus alerts), an expired transfer (payment `EXPIRED`).

## 4. Endpoint catalogue

| Method | Path | Story | Purpose |
| --- | --- | --- | --- |
| POST | `/v1/demo/sessions` | SG-102 | Start a trial session for a role (demo mode only) |
| GET | `/v1/me` | SG-102 | Current user, tenant and building access |
| PUT | `/v1/me/locale` | SG-102 | Save the caller's preferred language |
| GET | `/v1/buildings` | SG-201 | Buildings the caller may see, with status counters |
| GET | `/v1/buildings/{buildingId}/rooms` | SG-201 | Rooms of a building with status and active stay summary |
| GET | `/v1/rooms/{roomId}` | SG-201 | One room with its active stay summary |
| GET | `/v1/services` | SG-205 | Extras catalogue (drinks, food, towels) with stock |
| POST | `/v1/rooms/{roomId}/stays` | SG-203 | Check a guest into a vacant room |
| GET | `/v1/stays/{stayId}` | SG-203 | Stay with a live running total |
| POST | `/v1/stays/{stayId}/extras` | SG-205 | Add extras to an active stay |
| POST | `/v1/stays/{stayId}/checkout` | SG-205 | End the stay and produce the final invoice |
| POST | `/v1/invoices/{invoiceId}/payments` | SG-301 | Start a cash or bank-transfer payment for an invoice |
| GET | `/v1/payments/{paymentId}` | SG-301 | Payment status (polling fallback for the event stream) |
| GET | `/v1/payments/{paymentId}/events` | SG-302 | Server-Sent Events for one payment |
| POST | `/v1/demo/payments/{paymentId}/simulate` | SG-302 | Simulate the bank reporting the money as received (demo mode only) |
| POST | `/v1/webhooks/bank` | - | Bank or reconciliation-provider webhook (full product) |
| GET | `/v1/housekeeping/tasks` | SG-401 | Rooms waiting to be cleaned in buildings the caller may edit or view |
| POST | `/v1/housekeeping/tasks/{taskId}/complete` | SG-401 | Mark a room clean; the room becomes VACANT |
| POST | `/v1/rooms/{roomId}/usage-reports` | SG-401 | Report a room that looks used but has no active stay |
| GET | `/v1/owner/overview` | SG-403 | Revenue, cash expected, occupancy, alerts and latest payments for a day |
| GET | `/v1/owner/staff-permissions` | SG-501 | Every staff member with their access level per building |
| PUT | `/v1/owner/staff/{userId}/building-permissions/{buildingId}` | SG-501 | Grant, change or revoke one staff member's access to one building |
| GET | `/v1/shifts/current` | SG-503 | The caller's open shift with the cash the system expects |
| POST | `/v1/shifts/current/close` | SG-503 | Close the caller's shift with a counted cash total |
| POST | `/v1/shifts/current/payouts` | SG-503 | Record cash paid out of the drawer during the shift (for example ice) |
| GET | `/v1/owner/shifts/{shiftId}` | SG-503 | Reconciliation detail of a closed shift |
| GET | `/healthz` | SG-001 | Liveness |
| GET | `/readyz` | SG-003 | Readiness (database reachable, migrations applied) |

`receiveBankWebhook` is full-product (`x-release: full`); it ships as a documented stub with the shared handler and is not exposed in the demo build.

## 5. Realtime events

`GET /v1/payments/{paymentId}/events` is a Server-Sent Events stream. Each event: `id` = `sequence`, `event` = `type`, `data` = JSON matching `contracts/events/payment-events.schema.json`. Rules: send the current state first; send a comment heartbeat every 15 s; close the stream after a terminal state (`PAID`, `EXPIRED`, `MISMATCH`); support `Last-Event-ID`; clients ignore any sequence not greater than the last one seen and fall back to polling `getPayment` every 3 s when the stream fails twice.

## 6. Async events

None cross a process boundary in the demo. Internal domain events (for example `PaymentSettled`) are plain Go values handled in the same transaction (ADR-004). A provider webhook is the only inbound async input (full product).

## 7. Parallel development workflow

1. Contract PR per slice touching only `contracts/`: OpenAPI paths and schemas, event schemas, fixtures.
2. CI: Spectral lint, oasdiff breaking-change check against `main`, JSON Schema compile, pricing vector check (`generate_vectors.py --check`).
3. After merge: API regenerates strict server stubs (unimplemented handlers do not compile); WEB regenerates the typed client and MSW handlers from the same spec.
4. Both sides build in parallel; the API validates real responses against the spec in tests; WEB runs on mocks.
5. Integration checkpoint: `make up` with the slice flag on and mocks off; the slice E2E journey and contract tests pass before the flag is enabled.
