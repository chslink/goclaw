# Memory Templates

Use these templates when initializing self-improving memory files.

## memory.md (HOT tier)

```markdown
# Self-Improving Memory

_Mode: [passive|active|strict] | Last updated: [date]_

## Confirmed Preferences
<!-- Patterns confirmed 3+ times. Format: [pref] Description — confirmed DATE (Nx) -->

## Active Patterns
<!-- Patterns seen 2+ times but not yet confirmed. Format: [pattern] Description — emerging (Nx since DATE) -->

## Recent (last 7 days)
<!-- New corrections and observations. Format: [correction] DATE: Description -->
```

## index.md

```markdown
# Self-Improving Index

_Last updated: [date]_

| File | Lines | Entries | Last Modified |
|------|-------|---------|---------------|
| memory.md | 0 | 0 | [date] |
| corrections.md | 0 | 0 | [date] |
```

## corrections.md

```markdown
# Corrections Log

_Entries: 0 | Last: never_

<!-- Corrections will be logged here. See corrections.md template for format. -->
```

## heartbeat-state.md

```markdown
# Self-Improving Heartbeat State

last_heartbeat_started_at: never
last_reviewed_change_at: never
last_heartbeat_result: never

## Last actions
- none yet
```
