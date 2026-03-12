# Corrections Log

This file tracks the last 50 corrections. When count reaches 50, remove the oldest entries.

## Format

```markdown
## [Date]
### [Time] — [Category]
- **Correction:** What was wrong and what's right
- **Context:** What you were doing when corrected
- **Count:** N (first occurrence | Nth occurrence)
- **Action:** Logged as tentative | Promoted to memory.md | Written to projects/X.md
```

## Example

```markdown
## 2026-02-19
### 14:32 — Code style
- **Correction:** "Use 2-space indentation, not 4"
- **Context:** Editing TypeScript file
- **Count:** 1 (first occurrence)
- **Action:** Logged as tentative

### 16:15 — Communication
- **Correction:** "Don't start responses with 'Great question!'"
- **Context:** Chat response
- **Count:** 3 → **PROMOTED to memory.md**
- **Action:** Written to Confirmed Preferences
```

## Rules

1. Keep entries concise — one correction per block
2. Always include context — what you were doing
3. Track count accurately — search for similar entries before adding
4. Note promotions — when count >= 3 and user confirms
5. Maintain chronological order — newest at top
6. Trim at 50 entries — remove oldest when full
