# ADR Template

Use this format for Architecture Decision Records.

```markdown
---
last-reviewed: YYYY-MM-DD
---

# NNNN: <Decision Title>

**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Deprecated | Superseded by [NNNN]

## Context

<What is the issue we need to decide? What forces are at play? What constraints exist?>

## Considered Options

1. **<Option A>** — <brief description>
2. **<Option B>** — <brief description>
3. **<Option C>** — <brief description>

## Decision

<What is the change being proposed or decided? Which option was chosen and why?>

## Consequences

### Positive
- <benefit>

### Negative
- <cost or risk>

### Neutral
- <trade-off or observation>
```

## Guidelines

- **Title** should be a short noun phrase: "Use PostgreSQL for persistence", "Adopt GraphQL for API"
- **Context** is the most important section — it explains WHY this decision was needed
- **Status** starts as "Proposed", changes to "Accepted" after team approval
- **Consequences** must include both positive and negative — every decision has trade-offs
- ADRs are **immutable** once accepted — supersede with a new ADR instead of editing
