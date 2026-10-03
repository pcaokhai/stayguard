# Contracts

Normative interfaces. SHIP MODE: change `openapi.yaml` on `main` with `make gen` in the same commit and keep `make contracts` green (Spectral, oasdiff, schemas, vectors).

| Path | What |
| --- | --- |
| `openapi.yaml` | REST and SSE contract, OpenAPI 3.1 |
| `events/payment-events.schema.json` | SSE payload for payment status |
| `pricing/generate_vectors.py` | Reference implementation and generator; `--check` fails CI if the vectors are stale |
| `pricing/golden-cases.json` | 25 price cases, 2 error cases, 2 bill cases; generated, never hand-edited |
| `pricing/rate-plan.schema.json` | Rate plan configuration schema |
| `fixtures/demo-tenant-seed.json` | Sample tenant for trials; test data only |
| `audit-actions.json` | Every audit action code: its activity-log category and the detail keys it shows (room and bill are resolved at read time); checked by `internal/app` and the e2e suite |

Rule: a new pricing rule means a new vector first.

## Accepted breaking changes

`oasdiff-ignore.txt` lists breaking changes the owner accepted (one reason line above each entry); `make contracts` ignores exactly those and nothing else.
