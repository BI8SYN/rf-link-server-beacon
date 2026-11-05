# Git Flow Reference

Reference material for common git workflows. Used during bootstrapping to help users choose and customize their project's git conventions.

---

## GitHub Flow (Recommended for most projects)

Simple, PR-based workflow. Good for continuous delivery.

```
main ──●──────●──────●──────●──
       │      ↑      │      ↑
       └─feat─┘      └─fix──┘
```

```markdown
## Git Conventions

| Setting | Value |
|---------|-------|
| Branching Model | GitHub Flow |
| Main Branch | main |
| Branch Naming | `<type>/<short-description>` |
| Commit Format | `<type>: <description>` ([Conventional Commits](https://www.conventionalcommits.org/)) |
| Merge Strategy | Squash merge |

### Branch Types
| Prefix | Purpose |
|--------|---------|
| `feat/` | New features |
| `fix/` | Bug fixes |
| `refactor/` | Code restructuring |
| `docs/` | Documentation only |
| `test/` | Test additions/changes |
| `chore/` | Build, CI, tooling |

### Commit Types
`feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`

Breaking changes: add `!` after type (e.g., `feat!: remove deprecated API`)

### PR Process
1. Create feature branch from `main`
2. Make commits, push to remote
3. Open PR with description: what changed, why, how to test
4. Request review
5. Squash merge after approval
6. Delete feature branch
```

---

## Gitflow

Structured workflow with release branches. Good for versioned releases.

```
main    ──●────────────────●──
           ↑                ↑
develop ──●──●──●──●──●──●──●──
           │     ↑  │     ↑
           └feat─┘  └feat─┘
```

```markdown
## Git Conventions

| Setting | Value |
|---------|-------|
| Branching Model | Gitflow |
| Main Branch | main |
| Develop Branch | develop |
| Branch Naming | `<type>/<short-description>` |
| Commit Format | `<type>: <description>` |
| Merge Strategy | Merge commit (no fast-forward) |

### Branch Types
| Prefix | Base | Merge To | Purpose |
|--------|------|----------|---------|
| `feature/` | develop | develop | New features |
| `bugfix/` | develop | develop | Bug fixes |
| `release/` | develop | main + develop | Release preparation |
| `hotfix/` | main | main + develop | Production fixes |

### PR Process
1. Create feature branch from `develop`
2. Open PR to `develop`
3. For releases: create `release/x.y.z` from `develop`, merge to `main` and `develop`
4. Tag releases on `main`
```

---

## Trunk-Based Development

Minimal branching, frequent integration. Good for experienced teams with CI/CD.

```
main ──●──●──●──●──●──●──●──
       │  ↑        │  ↑
       └──┘        └──┘
     (short-lived branches)
```

```markdown
## Git Conventions

| Setting | Value |
|---------|-------|
| Branching Model | Trunk-Based |
| Main Branch | main |
| Branch Naming | `<initials>/<short-description>` |
| Commit Format | `<type>: <description>` |
| Merge Strategy | Rebase and merge |

### Rules
- Feature branches live < 2 days
- Commit directly to main for trivial changes (< 10 lines)
- Use feature flags for incomplete features
- CI must pass before merge

### PR Process
1. Create short-lived branch from `main`
2. Keep PR small (< 400 lines changed)
3. Rebase onto latest `main` before merge
4. Merge same day if possible
```

---

## Bootstrapping Questions

Ask the user these questions to determine their git conventions:

1. **Branching model:** "Which branching model does your team use?"
   - GitHub Flow (simple, PR-based)
   - Gitflow (release branches)
   - Trunk-based (frequent direct integration)
   - Custom (describe your flow)

2. **Main branch name:** "What is your main branch called?" (default: `main`)

3. **Commit format:** "What commit message format do you prefer?"
   - Conventional Commits (`type: description`)
   - Free-form (no enforced format)
   - Custom (describe your format)

4. **PR requirements:** "What is your PR process?"
   - Require review before merge
   - Self-merge allowed for small changes
   - CI must pass before merge

5. **Merge strategy:** "How do you prefer to merge PRs?"
   - Squash merge (clean history)
   - Merge commit (preserve branch history)
   - Rebase and merge (linear history)
