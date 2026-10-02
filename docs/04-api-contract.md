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
| Sensitive data | Guest ID numbers and photos are write-only for front desk and housekeeping; only OWNER and MANAGER read them through audited endpoints (docs/15 rules 21 to 25). QR payload and tokens are never logged |
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


## 8. Production additions (contract 1.1.0, from docs/15)

Version bump 1.0.0 → 1.1.0 is additive except two changes: `receiveBankWebhook` moved to `/v1/webhooks/bank/{hookId}`; the old path stays as deprecated `receiveBankWebhookLegacy` answering 410 GONE so oasdiff passes and stale provider settings fail loudly and `Role` gained `MANAGER`. New schemas and operations are tagged by story (x-story) and `x-release: full`.

### 8.1 New access values

| x-access | Meaning |
| --- | --- |
| PUBLIC | No session (sign-in only) |
| ANY | Any signed-in user |
| ANY_STAFF | Any signed-in user acting on their own data |
| OWNER_OR_MANAGER | OWNER or MANAGER role |
| EDIT_ANY | EDIT on at least one building |

`completeHousekeepingTask` now also allows RECEPTIONIST and MANAGER with EDIT on the building. MANAGER is denied: bank accounts, removeStaff, createStaff, updateStaff, payroll, expenses, report, linkTransferToInvoice, updateRatePlan, audit log (docs/15 §2).

### 8.2 New error codes

| code | Status | When |
| --- | --- | --- |
| PIN_INVALID | 401 | Wrong guesthouse code, user name or PIN (same response for all three) |
| ACCOUNT_LOCKED | 401 | Five wrong PINs; `lockedUntil` in the problem body |
| PIN_CHANGE_REQUIRED | 403 | One-time PIN used; only changeMyPin is allowed |
| PIN_TOO_SIMPLE | 422 | Runs or repeated digits |
| OWNER_PIN_INVALID | 403 | Owner PIN re-entry failed for a sensitive change |
| SHIFT_OPEN | 409 | Removing staff with an open shift |
| ROOM_OCCUPIED | 409 | Type change, retire, lock or move target with a guest |
| CHECKIN_EDIT_OUT_OF_RANGE | 422 | New check-in later than 60 minutes after recorded, or in the future |
| LEAVE_OVERLAP | 409 | Leave overlaps another request |
| LEAVE_NOT_CANCELLABLE | 409 | Leave already taken or declined |
| EXPENSE_AUTOMATIC | 409 | Editing an automatic expense line |
| BANK_ACCOUNT_NOT_CONNECTED | 409 | Making a pending account default |
| BANK_ACCOUNT_IS_DEFAULT | 409 | Removing the default account |
| EVENT_ALREADY_LINKED | 409 | Linking a transfer twice |
| INVOICE_ALREADY_PAID | 409 | Linking to a paid invoice (existing code) |
| ID_CONSENT_REQUIRED | 422 | ID number or photo without guest consent |
| PHOTO_TOO_LARGE | 413 | ID photo over 5 MB |
| UNSUPPORTED_MEDIA | 415 | ID photo not JPEG or PNG |
| GUEST_ID_LOCKED | 409 | Uploading more than 24 h after check-out |

### 8.3 Catalogue

| Story | Method | Path | operationId | Access | Purpose |
| --- | --- | --- | --- | --- | --- |
| SG-1001 | GET | `/v1/owner/bank-accounts` | listBankAccounts | OWNER | Receiving accounts |
| SG-1001 | GET | `/v1/owner/property` | getProperty | OWNER_OR_MANAGER | Property details and QR expiry |
| SG-1001 | GET | `/v1/owner/sepay-status` | getSepayStatus | OWNER_OR_MANAGER | Connection status of the default account |
| SG-1001 | PATCH | `/v1/owner/property` | updateProperty | OWNER | Update property details |
| SG-1001 | POST | `/v1/owner/bank-accounts` | createBankAccount | OWNER | Add a receiving account (pending SePay) |
| SG-1001 | POST | `/v1/owner/bank-accounts/{accountId}/make-default` | makeDefaultBankAccount | OWNER | Make an account the QR default (must be CONNECTED) |
| SG-1001 | POST | `/v1/owner/bank-accounts/{accountId}/remove` | removeBankAccount | OWNER | Remove a non-default account |
| SG-1002 | PATCH | `/v1/owner/buildings/{buildingId}` | updateBuilding | OWNER | Rename a building |
| SG-1002 | PATCH | `/v1/owner/rooms/{roomId}` | updateRoom | OWNER_OR_MANAGER | Edit a room, set maintenance or retire it |
| SG-1002 | POST | `/v1/owner/buildings` | createBuilding | OWNER | Create a building with generated floors and rooms |
| SG-1002 | POST | `/v1/owner/buildings/{buildingId}/floors` | createFloor | OWNER | Add a floor, optionally with rooms |
| SG-1002 | POST | `/v1/owner/rooms` | createRooms | OWNER | Create one room or a range of rooms |
| SG-1003 | GET | `/v1/owner/rate-plans` | listRatePlans | OWNER_OR_MANAGER | Rate plans per unit type |
| SG-1003 | POST | `/v1/owner/rate-plans/preview` | previewPrice | OWNER_OR_MANAGER | Price a sample stay with a draft rate plan |
| SG-1003 | PUT | `/v1/owner/unit-types/{unitTypeCode}/rate-plan` | updateRatePlan | OWNER | Save a new rate plan version (applies to later check-ins) |
| SG-1004 | GET | `/v1/owner/services/{serviceCode}/movements` | listStockMovements | OWNER_OR_MANAGER | Stock history |
| SG-1004 | PATCH | `/v1/owner/services/{serviceCode}` | updateService | OWNER_OR_MANAGER | Edit details (never the stock count) |
| SG-1004 | POST | `/v1/owner/services` | createService | OWNER_OR_MANAGER | Add an item with price, cost and opening quantity |
| SG-1004 | POST | `/v1/owner/services/{serviceCode}/remove` | removeService | OWNER | Remove an item (stops selling if it has sales) |
| SG-1004 | POST | `/v1/owner/services/{serviceCode}/restock` | restockService | OWNER_OR_MANAGER | Record stock in with unit cost |
| SG-1004 | POST | `/v1/stocktakes` | createStocktake | EDIT_ANY | Record a stocktake; differences alert the owner |
| SG-1101 | GET | `/v1/owner/staff` | listStaff | OWNER | Staff with position, app access, contract and building access |
| SG-1101 | PATCH | `/v1/owner/staff/{userId}` | updateStaff | OWNER | Change position, app access or contract |
| SG-1101 | POST | `/v1/owner/staff` | createStaff | OWNER | Add a staff member; returns a one-time PIN when appAccess is not NONE |
| SG-1101 | POST | `/v1/owner/staff/{userId}/lock` | lockStaff | OWNER_OR_MANAGER | Lock sign-in |
| SG-1101 | POST | `/v1/owner/staff/{userId}/pin-reset` | resetStaffPin | OWNER_OR_MANAGER | Issue a one-time PIN (24 h) |
| SG-1101 | POST | `/v1/owner/staff/{userId}/remove` | removeStaff | OWNER | Remove a staff member (deactivate; history kept) |
| SG-1101 | POST | `/v1/owner/staff/{userId}/unlock` | unlockStaff | OWNER_OR_MANAGER | Unlock sign-in |
| SG-1102 | GET | `/v1/owner/leave-requests` | listLeaveRequests | OWNER_OR_MANAGER | Leave requests to decide |
| SG-1102 | GET | `/v1/owner/roster` | getRoster | OWNER_OR_MANAGER | Roster for a date range with leave and uncovered shifts |
| SG-1102 | POST | `/v1/owner/leave-requests/{leaveId}/approve` | approveLeave | OWNER_OR_MANAGER | Approve a request (or a cancel request) |
| SG-1102 | POST | `/v1/owner/leave-requests/{leaveId}/decline` | declineLeave | OWNER_OR_MANAGER | Decline with a reason |
| SG-1102 | POST | `/v1/owner/roster/copy-week` | copyRosterWeek | OWNER_OR_MANAGER | Copy the previous week into a week |
| SG-1102 | PUT | `/v1/owner/roster` | putRoster | OWNER_OR_MANAGER | Set and remove assignments in one change |
| SG-1103 | GET | `/v1/me/leave-requests` | listMyLeaveRequests | ANY_STAFF | My leave requests and balance |
| SG-1103 | GET | `/v1/me/roster` | getMyRoster | ANY_STAFF | My shifts for a date range |
| SG-1103 | POST | `/v1/me/leave-requests` | createLeaveRequest | ANY_STAFF | Request leave |
| SG-1103 | POST | `/v1/me/leave-requests/{leaveId}/cancel` | cancelMyLeave | ANY_STAFF | Cancel a pending request, or ask to cancel an approved one |
| SG-1104 | GET | `/v1/owner/payroll/{month}` | getPayroll | OWNER | Payroll for a month from contracts and the roster |
| SG-1104 | PATCH | `/v1/owner/payroll/{month}/lines/{userId}` | updatePayrollLine | OWNER | Set bonus, deduction or note |
| SG-1104 | POST | `/v1/owner/payroll/{month}/mark-paid` | markPayrollPaid | OWNER | Mark lines paid; posts STAFF_PAY expense |
| SG-1201 | GET | `/v1/owner/maintenance-tickets` | listTickets | OWNER_OR_MANAGER | Tickets with totals |
| SG-1201 | GET | `/v1/owner/maintenance-tickets/{ticketId}` | getTicket | OWNER_OR_MANAGER | One ticket |
| SG-1201 | PATCH | `/v1/owner/maintenance-tickets/{ticketId}` | updateTicket | OWNER_OR_MANAGER | Set status, lock, expected date and costs |
| SG-1201 | POST | `/v1/rooms/{roomId}/damage-reports` | reportDamage | EDIT | Report damage or missing items; creates a ticket |
| SG-1202 | DELETE | `/v1/owner/expenses/{expenseId}` | deleteExpense | OWNER | Delete a manual expense |
| SG-1202 | GET | `/v1/owner/expenses` | getExpenseMonth | OWNER | Expenses of a month by category with manual items |
| SG-1202 | PATCH | `/v1/owner/expenses/{expenseId}` | updateExpense | OWNER | Edit a manual or recurring expense |
| SG-1202 | POST | `/v1/owner/expenses` | createExpense | OWNER | Add a manual or recurring expense |
| SG-1203 | GET | `/v1/owner/reports/income-costs` | getIncomeCostReport | OWNER | Revenue, expenses and profit for a month range |
| SG-701 | POST | `/v1/auth/sign-in` | signIn | PUBLIC | Sign in with guesthouse code, user name and PIN |
| SG-701 | POST | `/v1/auth/sign-out` | signOut | ANY | End the current session |
| SG-701 | PUT | `/v1/me/pin` | changeMyPin | ANY | Change my PIN (required after a one-time PIN) |
| SG-703 | POST | `/v1/webhooks/bank/{hookId}` | receiveBankWebhook |  | Bank or reconciliation-provider webhook (full product) |
| SG-801 | POST | `/v1/stays/{stayId}/check-in-time` | editCheckInTime | EDIT | Correct the check-in time with a reason |
| SG-801 | POST | `/v1/stays/{stayId}/move` | moveStay | EDIT | Move the guest to another vacant room |
| SG-802 | GET | `/v1/invoices/{invoiceId}/receipt` | getReceipt | VIEW | Receipt data for printing |
| SG-802 | GET | `/v1/stays` | listStays | VIEW | Stay history |
| SG-805 | DELETE | `/v1/owner/stays/{stayId}/guest-id/number` | deleteGuestIdNumber | OWNER_OR_MANAGER | Delete the stored ID number (audited) |
| SG-805 | DELETE | `/v1/owner/stays/{stayId}/guest-id/photos/{side}` | deleteGuestIdPhoto | OWNER_OR_MANAGER | Delete an ID photo (audited) |
| SG-805 | GET | `/v1/owner/stays/{stayId}/guest-id` | getGuestIdRecord | OWNER_OR_MANAGER | Masked ID number and photo metadata |
| SG-805 | GET | `/v1/owner/stays/{stayId}/guest-id/photos/{side}` | getGuestIdPhoto | OWNER_OR_MANAGER | Stream an ID photo to view or download (audited) |
| SG-805 | POST | `/v1/owner/stays/{stayId}/guest-id/reveal` | revealGuestIdNumber | OWNER_OR_MANAGER | Show the full ID number (audited) |
| SG-805 | PUT | `/v1/stays/{stayId}/guest-id/number` | setGuestIdNumber | EDIT | Add or replace the guest ID number (write-only for front desk) |
| SG-805 | PUT | `/v1/stays/{stayId}/guest-id/photos/{side}` | uploadGuestIdPhoto | EDIT | Upload or replace an ID photo (front desk cannot read it back) |
| SG-902 | GET | `/v1/owner/alerts` | listAlerts | OWNER_OR_MANAGER | Alerts |
| SG-902 | GET | `/v1/owner/audit-logs` | listAuditLogs | OWNER | Activity log (append-only) |
| SG-902 | POST | `/v1/owner/alerts/{alertId}/read` | markAlertRead | OWNER_OR_MANAGER | Mark an alert read |
| SG-903 | GET | `/v1/owner/transactions` | listTransactions | OWNER_OR_MANAGER | Cash and bank transactions with reconciliation state |
| SG-903 | POST | `/v1/owner/payment-events/{eventId}/link` | linkTransferToInvoice | OWNER | Link an unmatched bank transfer to an unpaid invoice |
| SG-904 | GET | `/v1/owner/stays/{stayId}/timeline` | getStayTimeline | OWNER_OR_MANAGER | Everything that happened to a stay |
| SG-905 | GET | `/v1/owner/shifts` | listClosedShifts | OWNER_OR_MANAGER | Closed shifts with differences |
