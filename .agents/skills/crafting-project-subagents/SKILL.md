---
name: crafting-project-subagents
description: Use when project needs domain-specific subagents beyond the general-purpose Superpowers agents, such as specialized reviewers, analyzers, or domain experts
---

# Crafting Project Subagents

## Overview

Create project-specific subagent definitions that encode domain expertise. These complement Superpowers' `code-reviewer` with project-specific knowledge (e.g., a "security-reviewer" that knows your auth patterns, or a "migration-checker" that validates schema changes).

**Core principle:** A subagent should know one domain deeply. Generalist agents miss domain-specific issues.

## When to Use

- Project has domain-specific review needs (security, compliance, performance)
- Repeated patterns of agent mistakes in a specific area
- Team wants automated domain checks as part of the development workflow

## The Process

**Announce:** "I'm using the crafting-project-subagents skill to create a domain-specific subagent."

**Step 1: Identify the domain gap**

What does the general `code-reviewer` miss? Examples:
- Security patterns specific to your auth system
- Database migration safety checks
- API contract compliance
- Accessibility standards

**Step 2: Define the agent**

Use the Superpowers agent format (see `agent-template.md`):

```yaml
---
name: <agent-name>
description: |
  Use this agent when <triggering conditions>.
  Examples: <example>...</example>
model: inherit
---
```

Follow with: role description, numbered capabilities, checklist, output format.

**Step 3: Place in project agent directory**

```
.claude/agents/<agent-name>.md
```

**Step 4: Create dispatch instructions**

Document when and how the orchestrator should invoke this agent (in a project skill or CLAUDE.md).

**Step 5: Test with a real scenario**

Dispatch the agent against actual project code. Verify it catches known issues. Refine prompt until effective.

**Step 6: Commit**

```bash
git add .claude/agents/<agent-name>.md
git commit -m "agent: add <agent-name> project subagent"
```

## Red Flags

- **Never** duplicate `superpowers:code-reviewer` — extend it, do not replace it
- **Never** create subagents without testing against real code
- **Never** make subagents too broad — one domain per agent
- **Never** skip the output format — structured output is essential for orchestration

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Agent tries to do everything | One domain, one agent. Create multiple if needed |
| No output format defined | Always define structured output (Findings → Assessment) |
| Not testing against real code | Run against actual project, verify it catches known issues |
| Prompt too vague | Include specific check items, not just "review for quality" |

## Integration

- **superpowers:subagent-driven-development** — project subagents integrate into the two-stage review
- **superpowers:dispatching-parallel-agents** — project subagents can run in parallel
- **crafting-project-skills** — project skills may reference project subagents
