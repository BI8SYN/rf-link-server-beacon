# Requirement Template

Use this format for each individual requirement.

```markdown
### SPEC-<ID>: <Title>

**Priority:** Must / Should / Could / Won't (MoSCoW)
**Status:** Draft | Reviewed | Approved
**Source:** <who requested this>

**Description:**
<What the system should do — clear, unambiguous, one thing per requirement>

**Acceptance Criteria:**
- [ ] Given <precondition>, when <action>, then <expected result>
- [ ] <Additional specific, testable criterion>

**Out of Scope:**
- <Explicitly excluded items for this requirement>

**Dependencies:**
- SPEC-<ID> — <why this depends on another requirement>
```

## Quality Checklist

Each requirement should be:
- **Specific** — No ambiguous words ("fast", "user-friendly", "robust")
- **Testable** — Acceptance criteria can be verified with a test
- **Complete** — All necessary information is present
- **Consistent** — Does not conflict with other requirements
- **Independent** — Minimal dependencies on other requirements (where possible)

## Priority Guidelines (MoSCoW)

| Priority | Meaning |
|----------|---------|
| **Must** | System is unusable without this |
| **Should** | Important but system works without it |
| **Could** | Desirable if time and resources allow |
| **Won't** | Explicitly deferred (out of scope for now) |
