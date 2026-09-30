# AI Workflow, Token Discipline and Tracking

Version 1.0 · 2026-09-30 · Owner: Tech lead

How Claude Code sessions work in this repository and how progress, decisions, releases and bugs are recorded. The root `CLAUDE.md` is the short version; this document is the full text.

## 1. Purpose and tracking files

| File | Purpose | Updated by | When |
| --- | --- | --- | --- |
| `docs/progress.md` | Live status of every story, sprint numbers, blockers, retro notes | The lane session and the tech lead | Story start, PR open, merge, release |
| `docs/release-notes.md` | User-facing notes, one entry per feature, grouped by release | The story's own PR | Drafted in the feature PR; finalised at release |
| `docs/adr/` | Architecture decisions (`ADR-nnn-slug.md`) and an index | Whoever makes or changes a decision | In the PR that changes behaviour |
| `docs/bugs/` | Bug log: symptom, root cause, fix, trade-offs, prevention | The session that fixes the bug | In the fix PR |
| `docs/plans/<ID>.md` | One plan per story, kept after merge | The lane session | Before implementation; status line updated as work proceeds |

Rule: documentation changes in the same PR as the behaviour it describes. A story is not Done until its tracking files are current (docs/07 §7).

## 2. Superpowers workflow (mandatory)

Superpowers skills load as a Claude Code plugin; install it following the plugin's current instructions and verify that the skill names below are listed. Skills trigger on keywords, so name the skill explicitly in prompts.

| Step | Skill | Output in this repository |
| --- | --- | --- |
| 1 | `brainstorming` | Only when the story leaves a design decision open. Outcome: an ADR draft. Otherwise write a one-paragraph understanding that cites doc sections |
| 2 | `using-git-worktrees` | One worktree and branch per story `feat/<ID>-<slug>` in `../stayguard-worktrees/` |
| 3 | `writing-plans` | `docs/plans/<ID>.md`, **without code** (§3) |
| 4 | `subagent-driven-development` (default) or `executing-plans` (small or tightly coupled stories) | Tasks executed one at a time with review between tasks |
| 5 | `test-driven-development` | RED, GREEN, REFACTOR for every task; failing test first |
| 6 | `verification-before-completion` | Run the verification commands; report the last lines of real output before claiming done |
| 7 | `requesting-code-review`, then `receiving-code-review` | Review against docs/10 §12; fix findings with tests |
| 8 | `finishing-a-development-branch` | Update tracking files (§7, §8, §9, §5), rebase, green CI, squash merge with a Conventional Commit title |

Bugs: `systematic-debugging` first, then the bug-log rules in §5. Use `dispatching-parallel-agents` only for tasks with disjoint file sets.

### Repository overrides to Superpowers defaults

1. **The default `writing-plans` behaviour includes complete code in each step. This repository overrides it: plans contain no code.** If a skill instruction conflicts with this document, this document wins for this repository.
2. Tracking files (§1) are part of every story's task list; the last task of every plan is "update tracking files".
3. Worktrees live outside the repository directory so tools never index them.
4. Reviews check the domain non-negotiables in docs/10 §0 first.

## 3. Plans without code

A plan tells a capable engineer what to build and how to know it is right, in words. It is not the implementation.

Allowed: acceptance criteria restated by id; files to create or change with the purpose of each; interfaces described by name, inputs, outputs and errors; tests described by name, arrangement, action and expected result; ordered tasks of two to five minutes; verification commands (`make test`, a filtered test run) with the expected outcome in words; migration numbers; risks; Rulings.
Not allowed: code blocks of any language, pasted function bodies, SQL statements, JSON or YAML bodies, full test code. Short inline names (a function, a field, an error code) are fine.

Length: at most about 120 lines. Template:

```
# Plan <ID> <title>
Status: not started | in progress (task n of m) | in review | merged
Lane, slice, depends, contract operations, migration number reserved
## Understanding (cites docs sections)
## Acceptance criteria to test names
## Files to create or change (path: purpose)
## Interfaces in words
## Tasks (each: what, files, failing test first, verification command, expected result)
## Rulings (conflicts or decisions needing the tech lead)
## Risks and rollback
## Tracking files to update
```

## 4. Token and context discipline

Long sessions cost money and degrade quality. These rules apply to every session and every subagent.

1. Start each story in a fresh session. Use `/clear` between stories and `/compact` before context passes about 60%.
2. Read only what the story needs: the story in docs/06, the doc sections it cites, the files in the plan. Cite sections by number instead of pasting text.
3. Search before opening: use `grep -n` and view line ranges. Never `cat` a large file. Never open generated code, `node_modules`, `dist`, lock files or `golden-cases.json` in full.
4. Extract from the contract instead of reading it whole (for example grep an operationId with a few lines of context).
5. Do not re-read a file or a plan already read in this session unless it changed. Keep the plan's status line current instead of re-deriving state.
6. Keep command output small: use quiet flags, test name filters, `| tail -n 20`, and paste only the failing part. Run the narrow test during TDD and the full suite once before completion.
7. Prefer editing with targeted diffs over rewriting whole files. Do not reformat unrelated code.
8. Delegate exploration to a subagent with a narrow brief (task, files it may read, return format of at most 150 words). Do not ask subagents to return file contents.
9. Do not summarise the diff back to the user; the diff is visible. The final report is at most 10 lines: what changed, tests run with results, tracking files updated, open questions.
10. Write conclusions into the plan or the tracking files, not into long chat explanations.
11. No speculative work: do not add features, abstractions or docs the story does not ask for.
12. Sizes: root `CLAUDE.md` ≤ 200 lines; service `CLAUDE.md` ≤ 120 lines; plans ≤ 120 lines; PRs < 400 changed lines excluding generated code.
13. Do not use `@` imports of large documents in any `CLAUDE.md`; reference paths instead.
14. Configure read-deny rules for generated and vendored directories (SG-001 AC6).

## 5. Bug log

A bug is anything found after a story's tests were green: in review, at an integration checkpoint, in the deployed demo, or by a user. Typos fixed before the same PR merges are not bugs.

Process: reproduce → root cause (`systematic-debugging`) → failing regression test → fix → verify → write the entry in the fix PR.

Files: `docs/bugs/BUG-<nnn>-<slug>.md` from `docs/bugs/_template.md`; one row in the index table in `docs/bugs/README.md`. Numbers are sequential from 001.

Severity: S1 money, personal data, security or tenant isolation; S2 wrong behaviour without data risk; S3 cosmetic. S1 also requires a review of docs/10 and an ADR or rule update if a rule was missing.

Every entry contains: summary; severity and status; where and how it was found; symptom and reproduction steps; **root cause** (the real cause, not the trigger); **fix** (what changed, PR, commit); **options considered** (each with pros and cons); **trade-offs accepted** (what was given up, debt created, follow-up issue); **prevention** (test, lint rule, doc or checklist change); regression test name; impact (data affected, users affected); timeline.

## 6. Public and private repositories

`stayguard` is the demo and the reusable core. `stayguard-pro` is the private production repository (ADR-002).

- The demo repository stays private until the checklist below passes at v0.1.0, then becomes public.
- `stayguard-pro` is a new private repository (GitHub does not allow a private fork of a public repository) with the public repository as the `upstream` remote. Bring changes down with `git fetch upstream` and `git merge upstream/main`.
- Production-only code lives in additive directories (`api/internal/pro/`, `web/src/pro/`, `contracts/pro/`) wired through registration points, so merges stay clean.
- General-purpose changes are written in the public repository first and merged down. Never open a pull request from the private repository to the public one. Customer-specific configuration and data exist only in the private repository.

Public-repository checklist (all must pass before the visibility change):

- [ ] gitleaks: zero findings across the full history; no `.env`, keys or tokens anywhere
- [ ] No real names, phone numbers, bank accounts, ID numbers or customer or employer names in code, fixtures, docs, screenshots or the video
- [ ] The demo bank account in fixtures is a placeholder that is not a real account
- [ ] `LICENSE` (Apache-2.0) and `NOTICE` present; third-party licences reviewed and permissive
- [ ] `SECURITY.md` explains how to report vulnerabilities; `README.md` says the demo must not hold real guest data
- [ ] `DEMO_MODE` defaults to off in the production image; with it off, `/v1/demo/*` returns 404 (test)
- [ ] CI uses `pull_request`, not `pull_request_target`; no secrets exposed to forks; branch protection on `main`
- [ ] Dependabot or Renovate enabled; no high or critical advisories
- [ ] No internal chat transcripts, analytics keys or private URLs in the docs; links to private design artefacts in `docs/README.md` are replaced by exported images or PDFs in `docs/assets/`

## 7. progress.md protocol

Structure: current sprint header (dates, committed points); sprint table (planned versus done points); story table (ID, lane, points, sprint, status, branch or PR, flag, notes); blockers; retro notes per sprint; metrics from docs/07 §11.

Status values: Backlog, Ready, In progress, In review, Merged (flag off), Released.

Update rules: change one row at a time, never rewrite the file; update at story start, PR open, merge and release; edits ship in the story's PR, with commit messages like `docs(progress): SG-203 → In review`; when a sprint ends, collapse its details into one summary line and keep the file under about 150 lines.

## 8. Release notes protocol

`docs/release-notes.md`, newest first, with an `Unreleased` section and one section per version. Each feature entry is written in the feature's PR and contains: title with story id and slice; What's new (at most three sentences in user language, no code terms); Who it is for; How to try it (screens or steps); Behaviour changes or migration notes; Known limits; API operations touched; feature flag; links to docs and ADRs. At release, move Unreleased entries under the version heading, add highlights, breaking changes and upgrade notes, and tag the same version.

## 9. ADR protocol

Write an ADR for any decision that is hard to reverse, crosses lanes, changes a contract, or chooses between real alternatives (including brainstorming outcomes). Copy `ADR-000-template.md`, take the next number, keep it under about 30 lines, and list options with honest trade-offs plus a revisit trigger. Statuses: Proposed → Accepted → Superseded by ADR-nnn or Deprecated. Do not rewrite an accepted decision; supersede it. Update the index in `docs/adr/README.md` in the same PR.

## 10. Session checklists

Start: read the story and its cited doc sections; open the plan or create it; set the progress row to In progress.
End: paste the last lines of real test output; update the plan status; update progress, release-notes draft, ADR and bug log as applicable; open the PR with the AC to test table.
