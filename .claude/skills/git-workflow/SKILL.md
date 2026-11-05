---
name: git-workflow
description: Use when starting feature work, creating branches, making commits, opening PRs, or managing releases — enforces this project's git branching model, naming conventions, and development flow
---

# Git Workflow

## Overview

Enforces project-specific git conventions for consistent, reviewable development. All conventions are defined in the **Git Conventions** section of CLAUDE.md — this skill reads and applies them.

**Core principle:** Consistent git hygiene enables async collaboration. When every branch, commit, and PR follows the same pattern, code review is faster and git history tells a story.

## When to Use

- Starting new feature or fix work (branch creation)
- Making commits (message format)
- Opening pull requests (PR template)
- Before merging (checklist verification)
- When unsure about the project's git conventions

## The Process

**Announce:** "I'm using the git-workflow skill to follow this project's git conventions."

**Step 1: Read conventions**

Read the **Git Conventions** section in CLAUDE.md. It defines:
- Branching model (e.g., GitHub Flow, Gitflow, trunk-based)
- Main/develop branch names
- Branch naming pattern
- Commit message format
- PR process and requirements

If Git Conventions section is missing from CLAUDE.md, ask the user to define it (see `flow-reference.md` for common patterns).

**Step 2: Apply to current task**

Based on the task type, follow the appropriate flow:

**Starting work:**
1. Ensure you are on the correct base branch
2. Create branch following the naming pattern from Git Conventions
3. Confirm branch name with user before creating

**Making commits:**
1. Follow the commit format from Git Conventions
2. Keep commits atomic — one logical change per commit
3. Reference issue/ticket numbers if the project uses them

**Opening PR:**
1. Run doc-drift-check to assess documentation state
2. If drift detected: run syncing-docs-with-code, then syncing-docs-and-skills
3. Push branch to remote
4. Create PR following the project's PR process
5. Include: summary, what changed, how to test
6. Link related issues if applicable

**Merging:**
1. Verify all checks pass
2. Follow the merge strategy from Git Conventions (squash/merge/rebase)
3. Clean up branch after merge

## Red Flags

- **Never** push directly to the main branch — always use feature branches
- **Never** force-push to shared branches without explicit user confirmation
- **Never** ignore the commit format — consistency matters for history readability
- **Never** merge without verifying the PR checklist in Git Conventions

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Ignoring branch naming convention | Read Git Conventions in CLAUDE.md before creating branches |
| Freeform commit messages | Follow the exact format specified in Git Conventions |
| Skipping PR description | Always include what changed and how to verify |
| Merging without running checks | Verify tests pass before merge |

## Integration

- **superpowers:finishing-a-development-branch** — this skill provides conventions, that skill handles completion
- **doc-drift-check** — run before opening PR to assess documentation state
- **syncing-docs-with-code** — run before opening PR to ensure docs are current
- **superpowers:verification-before-completion** — verify before claiming work is done
