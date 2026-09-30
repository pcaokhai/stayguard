# ADR-011 Frontend delivery: Next.js static export, generated client, mock mode and query-string routes

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §4, SG-004

## Context
The Go binary should serve the web app; the app is behind sign-in and needs no SEO.

## Options considered
1. Next.js static export embedded in the Go binary
2. Next.js server runtime
3. Vite single-page app

## Decision
Next.js App Router with `output: 'export'`, TypeScript strict, Tailwind, TanStack Query, generated client and MSW mock mode. Entity ids travel in query strings because static export cannot resolve unknown dynamic segments at build time. The SG-004 spike records confirmed limits.

## Consequences
- Good: One container, no Node runtime in production, familiar tooling
- Bad or costly: No server components at request time; dynamic routes are constrained; some Next.js features unavailable
- Revisit when: A need for server rendering, image optimisation or SEO: move to the Next.js server runtime or a separate front host. A Vite single-page app is the fallback if static-export constraints hurt.
