---
name: syncing-docs-and-skills
description: Use when documentation or skills have changed, to detect and update dependent skills, agents, or documents that reference the changed content
---

# Syncing Docs and Skills

## Overview

Bidirectional dependency sync between project knowledge artifacts. When a document changes, find and update skills/agents that depend on it. When a skill changes, find and update documents it references.

**Core principle:** Docs, skills, and agents form a knowledge graph. Changing one node without updating its edges creates silent inconsistency — agents follow outdated skills referencing outdated docs.

## When to Use

- After updating any document in the Documentation Map
- After editing a project skill or agent definition
- After `syncing-docs-with-code` updates documentation
- After `writing-architecture-docs` creates or modifies docs
- When an agent produces unexpected behavior that suggests stale skill/doc references

## The Process

**Announce:** "I'm using the syncing-docs-and-skills skill to check knowledge consistency."

**Step 1: Identify what changed**

Check recent changes:
```bash
git diff --name-only HEAD~1..HEAD
```

Classify each changed file:
- **Document** — any file listed in the Documentation Map in CLAUDE.md
- **Skill** — any file under the Project Skills path
- **Agent** — any file under the Project Agents path
- **Other** — not a knowledge artifact, skip

**Step 2: Find dependents**

For each changed **document**, scan all skills and agents for references:
- Grep skill/agent files for the document's path or key terms
- Check if the document defines conventions, APIs, or structures that skills enforce

For each changed **skill or agent**, scan documents:
- Check if any doc references this skill by name
- Check if the skill's process steps reference docs that may need updating

**Step 3: Report dependency impact**

```markdown
## Knowledge Consistency Report

### Changed artifacts:
- <path> (document/skill/agent)

### Dependents needing review:
- [ ] <skill/agent/doc path> — references <changed artifact>, may need: <specific concern>

### No impact:
- <artifacts checked but unaffected>
```

**Step 4: Guide updates**

For each dependent:
1. Open the file and show the relevant section
2. Explain what changed and how it affects this file
3. Suggest specific edits
4. After update, verify consistency

**Step 5: Update last-reviewed dates**

Update `last-reviewed` frontmatter on all verified documents.

## Red Flags

- **Never** update a skill without re-reading the document it depends on
- **Never** assume a rename-only change has no dependents — paths may be referenced
- **Never** skip agent definitions — they reference docs and conventions too
- **Never** auto-apply fixes without presenting the consistency report first

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Only checking direct path references | Also check semantic references (e.g., skill enforcing a convention described in a doc) |
| Forgetting CLAUDE.md itself | CLAUDE.md references skills and docs — it is part of the knowledge graph |
| Skipping the Documentation Map | If doc paths change, the Documentation Map in CLAUDE.md must be updated first |
| Not checking agent definitions | Agents reference conventions and doc paths just like skills do |

## Integration

- **syncing-docs-with-code** — runs first (code → docs), then this skill runs (docs → skills)
- **crafting-project-skills** — when creating new skills, use this skill to verify doc references
- **auditing-project-docs** — periodic full audit; this skill is per-change incremental check
