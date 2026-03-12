# Memory Operations

## User Commands

| Command | Action |
|---------|--------|
| "What do you know about X?" | Search all tiers, return matches with source |
| "Show my memory" | Display memory.md (HOT tier) |
| "Show all memory" | Display HOT + list WARM files + archive summary |
| "Forget X" | Find and remove entries matching X, update index |
| "Forget everything" | Kill switch (see boundaries.md) |
| "Export memory" | Output all tiers as structured text |
| "Memory status" | Show tier sizes, last update, pending promotions |
| "Review corrections" | Show corrections.md with counts and promotion status |

## Automatic Operations

### On Session Start
1. Load `${WORKSPACE}/self-improving/memory.md` (HOT tier)
2. Check `${WORKSPACE}/self-improving/index.md` for staleness
3. If project context is known, preload `${WORKSPACE}/self-improving/projects/{name}.md`

### On Correction Received
1. Parse correction (see learning.md)
2. Check for duplicates in corrections.md
3. Write to `${WORKSPACE}/self-improving/corrections.md`
4. If count >= 3, trigger promotion to memory.md
5. Update `${WORKSPACE}/self-improving/index.md`

### On Pattern Match
1. Find source entry in self-improving files
2. Apply the learned pattern
3. Briefly cite: "Using your preference for X..."
4. Log usage (bump last-used timestamp)

### Weekly Maintenance
1. Scan for decay candidates (unused 30+ days)
2. Move decayed HOT entries to appropriate WARM file
3. Move decayed WARM entries to archive/
4. Compact: merge similar entries, summarize verbose ones
5. Update index.md with current line counts

## File Formats

### memory.md (HOT)
```markdown
# Self-Improving Memory

_Mode: active | Last updated: 2026-01-15_

## Confirmed Preferences
- [pref] Use 2-space indentation for TypeScript — confirmed 2026-01-10 (5x)
- [pref] Never start responses with "Great question!" — confirmed 2026-01-12 (3x)

## Active Patterns
- [pattern] When editing Go files, run `go vet` after changes — emerging (2x since 2026-01-08)

## Recent (last 7 days)
- [correction] 2026-01-15: Use kebab-case for CSS classes, not camelCase
```

### corrections.md
```markdown
# Corrections Log

_Entries: 12 | Last: 2026-01-15_

## 2026-01-15
### 14:32 — Code style
- **Correction:** "Use 2-space indentation, not 4"
- **Context:** Editing TypeScript file
- **Count:** 1 (first occurrence)
- **Action:** Logged as tentative

## 2026-01-14
### 16:15 — Communication
- **Correction:** "Don't start responses with 'Great question!'"
- **Context:** Chat response
- **Count:** 3 → **PROMOTED to memory.md**
- **Action:** Written to Confirmed Preferences
```

### projects/{name}.md
```markdown
# Project: {name}

_Created: 2026-01-10 | Last updated: 2026-01-15_

## Preferences
- Use pnpm, not npm
- Test command: `pnpm test:unit`

## Patterns
- Always check for existing tests before writing new ones
- Prefer composition over inheritance in this codebase

## Notes
- Main branch is `develop`, not `main`
```
