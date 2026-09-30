# ADR-009 Internationalisation with locale-prefixed routes, message keys and coded API errors

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §12, SG-004, SG-602

## Context
Vietnamese and English throughout. A static export runs no middleware, so a server cannot read a locale cookie.

## Options considered
1. Cookie-based locale with middleware
2. Locale-prefixed routes (`/vi`, `/en`) with static rendering
3. Client-only translation with one URL

## Decision
Locale-prefixed routes with next-intl in static-export mode; `/` redirects on the client by stored or browser language; the API returns codes and structured values only; owner-entered names are `{vi, en}` objects; a CI check compares message keys.

## Consequences
- Good: Works with static export; shareable language-specific links; no server negotiation needed
- Bad or costly: Duplicate routes per language; a brief redirect on `/`; pathnames cannot be translated
- Revisit when: SEO or server-side negotiation becomes a requirement, or next-intl static-export limits block a feature.
