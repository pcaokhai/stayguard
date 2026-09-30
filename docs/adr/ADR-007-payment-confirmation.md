# ADR-007 Payments confirmed only by server-side events, provider port and client-rendered QR

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §6, SG-301, SG-302

## Context
The main fraud is staff marking transfers as paid or routing them to personal accounts.

## Options considered
1. Staff marks transfer paid
2. Bank reconciliation events processed by a server handler
3. Client polls the bank

## Decision
A transfer becomes PAID only in the payment-event handler, which deduplicates by (provider, external_id) and matches bill code and amount. The demo feeds it from a simulator endpoint that exists only with DEMO_MODE; production feeds it from a provider webhook through the same `PaymentSource` port. The QR always pays the tenant's own account; the API returns the EMVCo payload and the browser renders the image.

## Consequences
- Good: Staff cannot fake a transfer; one code path for demo and production; QR generation is testable without images
- Bad or costly: Production needs a reconciliation provider and its fees; mismatched amounts need an owner workflow
- Revisit when: The chosen provider cannot deliver events reliably or reports too slowly for front-desk use.
