---
name: doc-drift-check
description: Use at session start or before opening a PR to quickly detect documentation staleness and potential drift from code changes — lightweight, seconds-fast, no subagents
---

# Doc Drift Check

## Overview

Lightweight documentation drift detector. Checks document freshness dates and cross-references code changes against documented modules. Designed to run in seconds — no subagent dispatches, no document reading, no modifications.

**Core principle:** Catching drift early is cheap; fixing stale docs later is expensive. A 10-second check prevents hours of confusion.

## When to Use

- At the start of a new session (automated via session-start hook)
- Before opening a PR (automated via git-workflow skill)
- When you suspect docs may be out of date
- After returning to a project after time away

## The Process

**Announce:** "I'm using the doc-drift-check skill to quickly assess documentation freshness."

**Step 1: Read Documentation Map**

Read the **Documentation Map** section in CLAUDE.md to get actual document paths. If no Documentation Map exists, report "No Documentation Map found in CLAUDE.md — cannot check drift" and stop.

**Step 2: Staleness scan**

For each document listed in the Documentation Map, extract the `last-reviewed` YAML frontmatter date. Compare against today's date:

| Age | Status |
|-----|--------|
| ≤ 30 days | OK |
| 31–90 days | Stale |
| > 90 days | Critical |

If a document has no `last-reviewed` frontmatter, mark it as **Unknown**.

**Step 3: Change scope analysis**

Identify the base branch from Git Conventions in CLAUDE.md (default: `main`).

```bash
git diff --name-only $(git merge-base HEAD <main-branch>)..HEAD
```

If on the main branch with no uncommitted changes, skip this step (report "On main branch, no pending changes").

Classify each changed file:

- **Architecture-impacting** — new directories, deleted modules, config file changes (e.g., package.json, Cargo.toml, tsconfig), API route changes, schema changes
- **Convention-impacting** — linter configs, build configs, CI files, CLAUDE.md itself
- **Internal-only** — test files, implementation within existing modules, comments, formatting

**Step 4: Cross-reference**

For architecture-impacting and convention-impacting files, check whether the changed file falls within a module covered by an existing document (using the Documentation Map's module docs path). Flag:

- Documents covering modules with architecture-impacting changes → potential drift
- Convention-impacting changes with no corresponding doc update in the diff → potential drift
- Architecture-impacting changes in areas with no module doc → coverage gap

## Output Format

```markdown
## Doc Drift Check — YYYY-MM-DD

### Staleness
| Document | Last Reviewed | Status |
|----------|--------------|--------|
| <path> | <date> | OK / Stale (N days) / Critical (N days) / Unknown |

### Code Changes Since Branch Base
- Architecture-impacting: N files
- Convention-impacting: N files
- Internal-only: N files

### Potential Drift
- [ ] <doc path> — <reason>

### Recommendation
<one of the following>
```

Recommendations (pick one):
- **"No drift detected."** — all docs OK, no architecture-impacting changes
- **"Minor drift — consider running syncing-docs-with-code."** — some stale docs or minor changes in documented areas
- **"Significant drift — run syncing-docs-with-code before PR."** — critical staleness or architecture-impacting changes in documented modules

## Red Flags

- **Never** read full document contents — only extract `last-reviewed` dates
- **Never** modify any files — this is a read-only check
- **Never** dispatch subagents — this must complete in seconds
- **Never** block the developer's workflow — findings are informational only
- **Never** run deep consistency checks — that is `auditing-project-docs`' job

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Reading entire docs to check content | Only extract `last-reviewed` frontmatter |
| Modifying docs or updating dates | This skill is read-only; use syncing-docs-with-code to fix |
| Dispatching subagents for analysis | Keep it lightweight — no subagents |
| Blocking workflow on findings | Present findings and continue with user's request |
| Running deep audit instead of quick check | Use auditing-project-docs for thorough audits |

## Integration

- **syncing-docs-with-code** — if drift detected, run this skill to fix it
- **syncing-docs-and-skills** — run after syncing-docs-with-code to propagate changes
- **auditing-project-docs** — heavyweight periodic complement to this lightweight check
- **git-workflow** — triggers this skill before opening PRs
