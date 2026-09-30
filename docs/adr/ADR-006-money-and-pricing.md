# ADR-006 Money as whole VND and pricing as a pure versioned function

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §7.7, docs/10 §0

## Context
Correct money is the product's core promise, and rules change by property.

## Options considered
1. Integer whole VND
2. Decimal type
3. Floating point (rejected)

## Decision
Whole VND everywhere (`BIGINT`, `int64`). Pricing is a pure function of (rate plan, rental type, check-in, check-out). The rate plan is JSON validated by a schema and snapshotted on each stay. `contracts/pricing/generate_vectors.py` is the executable spec; golden vectors are generated, never hand-written.

## Consequences
- Good: No rounding bugs; rule changes cannot alter past stays; easy exhaustive testing
- Bad or costly: A second currency or fractional currency needs a new value type; two implementations (Python and Go) must agree, guarded by the vectors
- Revisit when: A market with minor units or a per-stay rule that needs data beyond the rate plan.
