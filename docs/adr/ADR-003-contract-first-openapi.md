# ADR-003 Contract-first API with OpenAPI 3.1 and code generation

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/04, SG-002

## Context
API and web are built in parallel by different lanes and must not drift.

## Options considered
1. Code-first with annotations
2. Contract-first OpenAPI with generated server stubs and typed clients
3. GraphQL
4. tRPC-style shared types

## Decision
OpenAPI 3.1 in `contracts/openapi.yaml` is the source of truth. Go strict server stubs via oapi-codegen; TypeScript client via openapi-typescript and openapi-fetch; MSW handlers generated from the same spec; Spectral and oasdiff in CI.

## Consequences
- Good: Parallel lanes; drift becomes a compile or CI error; docs and mocks come for free
- Bad or costly: Generator tooling to maintain; OpenAPI 3.1 support in Go generators is recent and described as initial
- Revisit when: SG-002 spike finds an unsupported 3.1 feature: downgrade the contract to 3.0.3 (nullable instead of type arrays) and supersede this ADR's version choice.

## Amendment: SG-002 generator spike (R-14)
Date 2026-10-01. Draft, finalised in the SG-002 PR. Result: every 3.1 feature used by `contracts/openapi.yaml` (3.1.0, 28 operations) is handled; the contract stays on 3.1 and is not downgraded.

Tools run against the unmodified spec: oapi-codegen v2.8.0 (chi, strict server, models), openapi-typescript 7.13.0, orval 8.39.0 (fetch client with MSW mocks, msw 3.0.1), Spectral 6.16.3 (spectral:oas ruleset). Checks: generator exit status and warnings, then Go `go vet` and TypeScript 6.0.3 `tsc --strict` on the output.

| Feature in the spec | oapi-codegen | openapi-typescript | orval (client and MSW) | Spectral |
| --- | --- | --- | --- | --- |
| `type: [x, null]` arrays (16 lines) | ok: pointer field, absent and null merge | ok: `x \| null` | ok: `x \| null` | ok |
| `oneOf` with a `$ref` and `type: 'null'` (4 sites) | ok: pointer to the referenced type | ok: `Ref \| null` | ok: `Ref \| null` | ok |
| `const: VND` | ok: single-value enum with a validity check | ok: literal type | ok: literal type | ok |
| webhook (`/v1/webhooks/bank` is an ordinary path; no top-level `webhooks` object) | ok | ok | ok | ok |

Outputs: oapi-codegen 5058 lines, `go vet` clean, one strict interface method per operation; openapi-typescript 1692 lines, `tsc --strict` clean; orval 28 MSW handlers (one per operation), `tsc --strict` clean. Spectral: 0 errors, 17 warnings (missing operation descriptions), so the project ruleset in `contracts/.spectral.yaml` sets those to off or per-rule severity instead of editing the contract.

Caveats for the pipeline: (1) oapi-codegen cannot tell an absent field from an explicit null on `[x, null]` fields; acceptable because the API never needs to distinguish them, revisit if a PATCH endpoint does. (2) msw-auto-mock 0.32.1 was also run and produced handlers with random data only; orval was chosen because it emits typed TypeScript for the same spec and msw 3. (3) orval pulls `@faker-js/faker` as a dependency of the generated mocks; it is named in the SG-002 plan and PR.

Decision unchanged: OpenAPI 3.1. No superseding ADR needed.
