# Contracts

Normative interfaces. Change them only in `contract/<slice>-<slug>` PRs approved by the tech lead (docs/04 §7).

| Path | What |
| --- | --- |
| `openapi.yaml` | REST and SSE contract, OpenAPI 3.1 |
| `events/payment-events.schema.json` | SSE payload for payment status |
| `pricing/generate_vectors.py` | Reference implementation and generator; `--check` fails CI if the vectors are stale |
| `pricing/golden-cases.json` | 25 price cases, 2 error cases, 2 bill cases; generated, never hand-edited |
| `pricing/rate-plan.schema.json` | Rate plan configuration schema |
| `fixtures/demo-tenant-seed.json` | Sample tenant for trials; test data only |

Rule: a new pricing rule means a new vector first.
