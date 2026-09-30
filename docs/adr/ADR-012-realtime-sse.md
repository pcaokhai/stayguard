# ADR-012 Real-time updates with Server-Sent Events and polling fallback

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/04 §5, SG-302, SG-303

## Context
Staff wait for a payment confirmation and expect the screen to change by itself.

## Options considered
1. Server-Sent Events
2. WebSocket
3. Polling only

## Decision
One SSE stream per payment with heartbeat and `Last-Event-ID`; the client falls back to polling every 3 s after two failures. SSE is one-way, works through simple proxies and needs no extra library.

## Consequences
- Good: Simple server and client; automatic reconnect; easy to test
- Bad or costly: Long-lived connections on scale-to-zero hosts can be cut; one connection per open payment screen
- Revisit when: A need for two-way or high-fan-out messaging: introduce WebSocket behind the same hub interface.
