# ADR-013 Deployment targets: one container, Cloud Run with Neon for self-test, a small VPS for prospects

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §8, SG-603, PRD A-08

## Context
Low cost, fast first impression for prospects, easy for one person to operate.

## Options considered
1. Cloud Run with a managed free PostgreSQL
2. Small VPS with Docker Compose and Caddy
3. Render free tier
4. Kubernetes

## Decision
One multi-stage image. Self-test on Cloud Run with Neon (accepting cold starts). Prospect-facing instance on a VPS in Singapore with Compose, PostgreSQL and Caddy. No free-tier hosting for prospects. Backups and a rehearsed restore are part of SG-603.

## Consequences
- Good: Near-zero cost for testing; fast and predictable demo for prospects; portable image
- Bad or costly: Two deployment paths to document; VPS needs patching and backups
- Revisit when: Production hosting requirements (availability, data residency) are defined.
