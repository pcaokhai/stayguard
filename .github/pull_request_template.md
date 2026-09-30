## Story
`SG-xxx` · lane · slice · flag · plan: `docs/plans/SG-xxx.md`

## What and why
<what changed and why, in a few sentences>

## Acceptance criteria to tests
| AC | Test name | Status |
| --- | --- | --- |
| 1 | | |

## Checklist
- [ ] Tests written first and `make lint`, `make test`, `make contracts` green (paste last lines of output)
- [ ] Domain rules honoured (money, server time, payment events only, tenant scope, authorization, idempotency)
- [ ] No contract change unless this is a `contract/` PR
- [ ] Logs and metrics added; no personal data logged
- [ ] Strings in both languages; screenshots in `vi` and `en` for UI changes
- [ ] Docs updated in this PR
- [ ] `docs/progress.md` row updated
- [ ] `docs/release-notes.md` entry drafted (user-visible features)
- [ ] ADR added or superseded (hard-to-reverse decisions)
- [ ] Bug-log entry with root cause, options and trade-offs (bug fixes)
- [ ] No secrets, real personal data or customer names

## Risk and rollback
<what could go wrong; how to turn it off (flag) or revert>
