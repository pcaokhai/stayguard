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
