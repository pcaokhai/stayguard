# ADR-002 Public demo repository, private production repository and sync model

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/11 §6, PRD A-07

## Context
The demo should be public for portfolio and trust; the production product must stay private. Code must flow from demo to production without copying.

## Options considered
1. A: one public repository containing production too
2. B: two independent repositories with copied code
3. C: public core plus a private repository that tracks it as an upstream remote
4. D: private only

## Decision
Option C. `stayguard` starts private and becomes public at v0.1.0 after the checklist in docs/11 §6. `stayguard-pro` is a new private repository (a public repository cannot be forked privately on GitHub) that merges `upstream/main`; production-only code lives in additive directories. Licence: Apache-2.0 for the demo (permissive, with a patent grant).

## Consequences
- Good: Portfolio value and one reusable core; production secrets and customers stay private
- Bad or costly: Anyone may use the demo code commercially under Apache-2.0; merge effort grows with divergence
- Revisit when: Before the public push reconsider a source-available licence if a competitor risk appears; review merge burden after two production features.
