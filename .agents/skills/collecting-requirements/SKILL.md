---
name: collecting-requirements
description: Use when gathering, structuring, or refining project requirements, user stories, or acceptance criteria before planning implementation work
---

# Collecting Requirements

## Overview

Structured requirements gathering through collaborative dialogue. Produces requirements documents that feed directly into Superpowers planning workflows.

**Core principle:** Requirements exist to prevent building the wrong thing. Ambiguous requirements produce ambiguous code.

## When to Use

- After project bootstrapping, before first implementation cycle
- When starting a new feature area
- When requirements are scattered across conversations, tickets, or informal notes
- When the team needs to formalize informal requirements

## The Process

**Announce:** "I'm using the collecting-requirements skill to structure requirements."

**Step 1: Understand scope**

Ask: "What area are we defining requirements for?"
- Entire project (first time)
- New feature area
- Refinement of existing requirements

**Step 2: Gather raw input**

Ask one question at a time (following `superpowers:brainstorming` conversational pattern):
- What problem does this solve?
- Who are the users?
- What does success look like?
- What are the constraints?
- What is explicitly **out of scope**?

If user has existing requirements (tickets, docs, conversations): ask them to paste or point to them.

**Step 3: Dispatch requirements-analyst**

Dispatch `requirements-analyst` subagent with gathered raw input. Analyst returns: structured requirements with quality scores, identified gaps, suggested acceptance criteria.

**Step 4: Present for review**

Present structured requirements in sections (200-300 words each). After each section, ask: "Does this capture your intent?"

**Step 5: Write to docs/**

Check the **Documentation Map** in CLAUDE.md for actual paths. Save to:
- **Product Spec** path (functional requirements)
- **Technical Spec** path (non-functional requirements)

Add `last-reviewed: YYYY-MM-DD` frontmatter.

**Step 6: Commit**

```bash
git add <spec directory from Documentation Map>
git commit -m "docs: add/update requirements for <area>"
```

**Step 7: Handoff**

"Requirements documented. Next steps:
1. Use `superpowers:brainstorming` to design the solution
2. Use `superpowers:writing-plans` to create implementation plan"

## Red Flags

- **Never** write requirements without user validation — present sections, get confirmation
- **Never** skip the "out of scope" section — scope creep prevention
- **Never** leave acceptance criteria vague ("works correctly" is not acceptance criteria)
- **Never** combine gathering and implementation — requirements first, code later

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Asking too many questions at once | One question at a time, wait for response |
| Skipping "out of scope" | Explicitly ask what is NOT included |
| Vague acceptance criteria | Use Given/When/Then format or specific measurable conditions |
| Inventing requirements user did not express | Structure what they said, flag gaps for confirmation |

## Integration

- **requirements-analyst** — dispatched in Step 3
- **superpowers:brainstorming** — uses same conversational style
- **superpowers:writing-plans** — requirements feed directly into plan creation
- **project-bootstrapping** — natural predecessor (ran during initial setup)
