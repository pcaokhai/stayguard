# Visual baselines: NOT APPROVED

24 images (12 key screens × 390 and 1280 px, Vietnamese) made by `web/e2e/rehearsal/a0-baseline.spec.ts` (VS-01 to VS-12, proposed rows).
QA wrote them; **nobody has approved them**. Khai looks at them and says yes (or asks for a change) before they count as the reference.
The pink blocks are masked on purpose (guesthouse code, times, dates). They were stable across two runs on two fresh guesthouses.
`make rehearse-test` compares every run to these files (1 % of pixels may differ). Do not run with `--update-snapshots` without Khai's say-so;
a missing image is written and its case fails, so a new screen is never silently accepted.
