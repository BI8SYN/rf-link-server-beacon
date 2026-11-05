---
name: auditing-project-docs
description: Use when reviewing project documentation health, checking for staleness, gaps, or inconsistencies, or when /audit-docs command is invoked
---

# Auditing Project Docs

## Overview

Health check for project documentation. Detects stale docs, missing sections, inconsistencies between docs and code, and coverage gaps. Produces an actionable audit report.

**Core principle:** Stale documentation is worse than no documentation — it actively misleads.

## When to Use

- Periodically (start of sprint or milestone)
- After a large refactoring or feature addition
- When agents seem confused about project architecture
- When starting a new sprint or milestone
- For a quick freshness check, use `doc-drift-check` instead — this skill is for comprehensive audits

## The Process

**Announce:** "I'm using the auditing-project-docs skill to check documentation health."

**Step 1: Inventory existing docs**

Read the **Documentation Map** in CLAUDE.md to know the actual paths. Scan all documented directories. For each file, extract:
- `last-reviewed` frontmatter date
- File modification date
- Section headings

**Step 2: Dispatch doc-auditor**

Dispatch `doc-auditor` subagent with:
- Complete file inventory with dates
- Recent git log (last 30 days of commits)
- Current project directory structure

**Step 3: Present audit report**

```markdown
## Documentation Audit Report — YYYY-MM-DD

### Health Score: X/10

### Stale Documents (last-reviewed > 90 days)
| Document | Last Reviewed | Days Stale |
|----------|--------------|------------|

### Missing Documentation
| Code Area | Files | Documentation | Status |
|-----------|-------|---------------|--------|

### Inconsistencies
1. <doc:line> — <description of mismatch>

### Recommendations (Priority Ordered)
1. [Critical] <action>
2. [High] <action>
3. [Medium] <action>
```

**Step 4: Offer to fix**

For each finding, offer: "Should I update this using writing-architecture-docs?"

## Red Flags

- **Never** mark docs as "reviewed" without actually reading content
- **Never** skip git history check — recent code changes are the strongest staleness signal
- **Never** auto-fix docs without presenting the audit report first
- **Never** audit only part of docs/ — audit everything for consistency

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Only checking dates, not content | Cross-reference doc content against actual code |
| Missing undocumented modules | Compare code directories against module docs |
| Ignoring TODO markers in docs | Report unfilled TODOs as completeness issues |
| Not checking ADR index | Verify ADR Index (see Documentation Map) links match actual ADR files |

## Integration

- **doc-auditor** — dispatched for analysis
- **doc-drift-check** — lightweight per-session check; this skill is the heavyweight periodic complement
- **writing-architecture-docs** — used to fix issues found
- **syncing-docs-with-code** — complementary (auditing is periodic, syncing is per-change)
