# StayGuard

Anti-loss management for small guesthouses: automatic pricing by hour, night and day, QR payments that always reach the owner's account, per-building staff permissions, and shift cash reconciliation.

Status: **SHIP MODE** — the demo is live; the production sprint (sign-in, staff, SePay, shifts, owner monitoring, settings, then stock, guest ID, roster, payroll, expenses and reports) runs from `docs/14-production-sprint.md`. Product rules: `docs/15`; UI kit and motion: `docs/16`.

> The demo is for trying the product. Do not enter real guest data.

## Start here

- People: `docs/README.md` (reading order per role)
- Claude Code sessions: `CLAUDE.md`, then `api/CLAUDE.md` or `web/CLAUDE.md`
- Live status: the checkboxes in `docs/14-production-sprint.md`; history: `git log`

## Layout

`api/` Go backend · `web/` Next.js app · `contracts/` OpenAPI, event schemas, pricing vectors, fixtures · `docs/` specification, plans, ADRs, bug log · `.github/` CI and templates. Deployment files arrive with story SG-603.

## Run and test locally

Everything below runs from the repository root unless a `cd` is shown.

### Prerequisites

| Tool | Version | Used for |
| --- | --- | --- |
| Docker (Compose v2) | any recent | the full stack (`api` + PostgreSQL) |
| Go | 1.27.x (see `api/go.mod`) | API tests, `make migrate`, code generation |
| Node.js + npm | 24.x | the web app, web tests |
| golangci-lint | recent | `make lint-api` |
| openssl | any | `make up` creates a local encryption key with it |

### 1. Run the full stack (API + web + database)

The Go binary serves the static Next.js export and `/v1` from one container, so the whole app is on **http://localhost:8080/vi**.

The demo needs `DEMO_MODE` and the feature flags on. Those live in `deploy/compose.demo.override.yaml`; a plain `make up` does **not** include them, and the role picker then fails with "Không bắt đầu được phiên dùng thử" (the API answers 404 to `/v1/demo/sessions`).

```bash
# create deploy/.env.local once (random DATA_ENCRYPTION_KEY, git-ignored)
make deploy/.env.local

# build and start in the background, with demo mode and all finished slices on
docker compose --env-file deploy/.env.local \
  -f deploy/compose.yaml -f deploy/compose.demo.override.yaml \
  up --build -d

# create the schema (first run, or after the database volume was removed)
DATABASE_URL='postgres://stayguard@localhost:5432/stayguard?sslmode=disable' make migrate
```

Check it works (expect `201`):

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/v1/demo/sessions \
  -H 'content-type: application/json' -d '{"role":"OWNER","locale":"vi"}'
```

Open http://localhost:8080/vi and pick a role (Lễ tân, Chủ nhà nghỉ, Buồng phòng). Go back to `/vi` to switch role; the roles share one trial tenant seeded from `contracts/fixtures/demo-tenant-seed.json`.

Stop it with `docker compose -f deploy/compose.yaml down` (add `-v` to also delete the database).

**Port 5432 already in use?** Another PostgreSQL (or container) owns the host port. Move this stack's database port with `DB_HOST_PORT`, and point `make migrate` at the same port:

```bash
export DB_HOST_PORT=5433
docker compose --env-file deploy/.env.local \
  -f deploy/compose.yaml -f deploy/compose.demo.override.yaml up --build -d
DATABASE_URL='postgres://stayguard@localhost:5433/stayguard?sslmode=disable' make migrate
```

Never run `make migrate` without checking which database `DATABASE_URL` points at.

**Logs:** `docker compose -f deploy/compose.yaml logs api --tail 50`.

### 2. Run the web app alone on mocks (no API, no database)

Fastest loop for UI work. Mock Service Worker answers every `/v1` call from deterministic demo data (`web/src/mocks/setup/demo-handlers.ts`) and falls back to data generated from the OpenAPI contract.

```bash
cd web
npm ci
NEXT_PUBLIC_MOCK=1 npm run dev        # http://localhost:3000/vi  (add -- -p 3100 for another port)
```

Mock mode is chosen at build time: `NEXT_PUBLIC_MOCK=1` bundles the mock layer, the default build ships none of it.

### 3. Run the web dev server against the real API

```bash
# API from section 1 must be running on :8080, with demo mode on
cd web
npm run dev
```

`next dev` does not proxy `/v1`; the web client uses same-origin URLs. For an end-to-end check of the real build, use the container in section 1 (it serves the exported site).

### 4. Test

| What | Command | Notes |
| --- | --- | --- |
| API unit tests | `make test-api` | `cd api && go test ./...` |
| API integration tests | `make test-api-int` | needs Docker (Testcontainers); `RACE=1` adds `-race` |
| Web unit tests | `make test-web` | `cd web && npm test` (Vitest) |
| Both unit suites | `make test` | |
| Lint | `make lint` | `golangci-lint` for the API, ESLint for the web |
| Format | `make fmt` | |
| Contract checks | `make contracts` | Spectral lint, oasdiff against `main`, schema compile, pricing vectors |
| Money-path smoke test | `make smoke` | builds its own stack (project `stayguard-smoke`, port 18080, so a `make up` stack can keep running), creates a test guesthouse with the installer commands, then drives the phone UI with Playwright: sign in, check in, extras, check out, pay by a signed SePay-style webhook, clean, close the shift, owner sees the revenue. Needs Docker, `npm ci` in `web/`, `python3`. `KEEP=1` leaves the stack up; `REUSE=1` runs again on it (about 15 s) |
| Backup round trip | `make backup-test` | local MinIO bucket: backup, list, restore into a scratch database, compare row counts (deploy/README.md, "Backup storage") |
| Web production build | `cd web && npm run build` | also run once with `NEXT_PUBLIC_MOCK=1` to check mock mode |

Tests are required for pricing, check-out and invoice, payments and settlement, tenant scoping, sign-in and roles (see `CLAUDE.md` section 4). Everything else is checked by build, typecheck, lint and the manual flow below.

### 5. Manual check: the demo script (about 3 minutes)

Use a fresh browser tab (the saved trial lives in `sessionStorage`) at 390 px width, then repeat on a laptop.

1. Lễ tân: the room map shows two buildings and status counters.
2. Check in **A102** by the hour; the room becomes "Có khách".
3. Open **A101** (2g35, 2 Nước suối already added), add 2 more nước, then **Trả phòng**: the bill lines show, with 100.000đ deposit deducted.
4. Pay by **QR**: the QR shows the exact amount and the owner's account.
5. Press **Giả lập tiền về** (demo only): the screen switches to "Đã thanh toán" by itself and the room goes to "Cần dọn".
6. Back on `/vi`, pick **Buồng phòng** and press "Dọn xong" on the room.
7. Back on `/vi`, pick **Chủ nhà nghỉ**: today's revenue went up and the payment is listed.

### 6. Regenerate code after a contract change

`contracts/openapi.yaml` is the source of truth. After editing it, run `make gen` in the same commit (Go server stubs, sqlc, web types and mocks). Generated files are committed; do not edit them by hand.

### Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Role picker shows "Không bắt đầu được phiên dùng thử" | `DEMO_MODE` is off (stack started with plain `make up`): restart with the demo override (section 1). Check with the `curl` above: `404` = demo off, `500` = no schema, run `make migrate`. |
| API log: `relation "app.tenants" does not exist` | Database has no schema (new volume): run `make migrate` against the right port. |
| `Bind for 0.0.0.0:5432 failed: port is already allocated` | Use `DB_HOST_PORT` (section 1). |
| Room map is empty | Use a fresh trial session. Old trial tenants keep the data they were seeded with. |
| Mock page shows "Mock worker failed to start" | Hard-reload the tab; make sure only one dev server runs. |
| `Another next dev server is already running` | Stop the other one, or start this one with `-- -p 3100`. |
| Build error fetching the font | `next/font/google` needs network access at build time. |

## Licence

Apache-2.0 (see ADR-002). Added by story SG-001.
