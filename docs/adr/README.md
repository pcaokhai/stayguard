# Architecture Decision Records

Copy `ADR-000-template.md`, take the next number, keep it under about 30 lines. Never rewrite an accepted decision; supersede it (docs/11 §9).

| ADR | Title | Status |
| --- | --- | --- |
| [ADR-001](ADR-001-repository-layout.md) | Repository layout and lane ownership | Accepted |
| [ADR-002](ADR-002-public-private-repositories.md) | Public demo repository, private production repository and sync model | Accepted |
| [ADR-003](ADR-003-contract-first-openapi.md) | Contract-first API with OpenAPI 3.1 and code generation | Accepted |
| [ADR-004](ADR-004-go-modular-monolith.md) | Go modular monolith with hexagonal layering and one deployable | Accepted |
| [ADR-005](ADR-005-multi-tenancy-rls.md) | Multi-tenancy with a shared schema, tenant_id and PostgreSQL row-level security | Accepted |
| [ADR-006](ADR-006-money-and-pricing.md) | Money as whole VND and pricing as a pure versioned function | Accepted |
| [ADR-007](ADR-007-payment-confirmation.md) | Payments confirmed only by server-side events, provider port and client-rendered QR | Accepted |
| [ADR-008](ADR-008-building-authorization.md) | Building-level authorization checked in the application layer on every request | Accepted |
| [ADR-009](ADR-009-internationalisation.md) | Internationalisation with locale-prefixed routes, message keys and coded API errors | Accepted |
| [ADR-010](ADR-010-demo-authentication.md) | Demo authentication through role-picker trial sessions, production credentials later | Accepted |
| [ADR-011](ADR-011-frontend-delivery.md) | Frontend delivery: Next.js static export, generated client, mock mode and query-string routes | Accepted |
| [ADR-012](ADR-012-realtime-sse.md) | Real-time updates with Server-Sent Events and polling fallback | Accepted |
| [ADR-013](ADR-013-deployment-targets.md) | Deployment targets: one container, Cloud Run with Neon for self-test, a small VPS for prospects | Accepted |
| [ADR-014](ADR-014-documentation-and-tracking.md) | Documentation and tracking system | Accepted |
