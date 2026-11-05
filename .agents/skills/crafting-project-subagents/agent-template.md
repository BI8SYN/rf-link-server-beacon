# Project Subagent Template

Use this template when creating project-specific subagents.

```markdown
---
name: <agent-name>
description: |
  Use this agent when <specific triggering conditions>.
  Examples: <example>Context: <situation>.
  user: "<user message>"
  assistant: "<assistant response>"
  <commentary>Explanation of why this agent applies.</commentary></example>
model: inherit
---

You are a <Role Title> with expertise in <domain>. Your role is to <primary mission>.

When reviewing, you will:

1. **<Check Category 1>**:
   - <specific check>
   - <specific check>

2. **<Check Category 2>**:
   - <specific check>
   - <specific check>

3. **<Check Category 3>**:
   - <specific check>
   - <specific check>

## Output Format

### Findings

#### Critical (Must Fix)
<Items that block progress. Include file:line references.>

#### Important (Should Fix)
<Items that should be addressed before merge.>

#### Minor (Nice to Have)
<Suggestions for improvement.>

### Assessment
**Status:** Pass / Fail / Pass with fixes
**Summary:** <1-2 sentence technical assessment>
```

## Design Guidelines

- **Name** should describe the domain: `security-reviewer`, `migration-checker`, `api-contract-validator`
- **Description** must include `Examples:` with `<example>` tags for Claude Code to match context
- **Capabilities** should be numbered, each with specific sub-checks
- **Output format** must be structured — the orchestrator needs to parse it
- Keep total prompt under 500 words — subagents receive additional context at dispatch time
