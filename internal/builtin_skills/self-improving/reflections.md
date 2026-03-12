# Self-Reflections Log

Track self-reflections on completed work. This is where the agent evaluates its own output quality.

## When to Reflect

- After completing a significant task (not trivial one-liners)
- After a multi-step workflow completes
- When you notice something in your output that could be better
- After a failure or unexpected result

## Format

```markdown
## [Date] — [Task Type]
**What I did:** Brief description of the work
**Outcome:** What happened (success, partial, failed)
**Reflection:** What I noticed about my work
**Lesson:** What to do differently next time
**Status:** candidate | promoted | archived
```

## Example

```markdown
## 2026-02-19 — Code refactoring
**What I did:** Refactored authentication module to use middleware pattern
**Outcome:** Success — all tests passing, cleaner separation of concerns
**Reflection:** I initially tried to change too many files at once. Breaking into smaller commits would have been safer.
**Lesson:** For refactoring tasks, make incremental changes and verify after each step.
**Status:** candidate
```

## Status Flow

- **candidate** — Fresh reflection, not yet validated by repeated experience
- **promoted** — Pattern confirmed (appeared 3+ times), written to memory.md
- **archived** — No longer relevant or superseded by better insight

## Rules

1. Be honest — note what went wrong, not just what went right
2. Be specific — "I should test more" is useless; "Run `go vet` after editing Go files" is actionable
3. Be concise — one reflection per entry, not a journal
4. Promote actionable lessons — if a lesson keeps appearing, promote it to memory.md
