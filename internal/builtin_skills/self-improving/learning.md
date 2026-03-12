# Learning Mechanics

## Trigger Classification

| Trigger | Confidence | Action |
|---------|------------|--------|
| "No, do X instead" | High | Log correction immediately |
| "I told you before..." | High | Flag as repeated, bump priority |
| "Always/Never do X" | Confirmed | Promote to preference |
| User edits your output | Medium | Log as tentative pattern |
| Same correction 3x | Confirmed | Ask to make permanent |
| "For this project..." | Scoped | Write to project namespace |

## What Does NOT Trigger Learning

- **Silence** — No response is not implicit approval
- **Single instance** — One-off instructions without "always" or "never"
- **Hypothetical discussion** — "What if..." or "In theory..."
- **Third-party preferences** — "My boss likes..." (not the user's preference)
- **Group chat mode** — Unless addressed directly
- **Inferred preferences** — Never assume; only log what's explicit

## Pattern Evolution

Patterns progress through stages:

```
Tentative → Emerging → Pending → Confirmed → Archived
   1x          2x        3x       promoted    decayed
```

- **Tentative**: First occurrence. Log in corrections.md only.
- **Emerging**: Second occurrence within 30 days. Note in corrections.md with count.
- **Pending**: Third occurrence. Ask user: "I've noticed this pattern 3 times. Should I make it permanent?"
- **Confirmed**: User approves. Promote to memory.md (HOT tier).
- **Archived**: Unused for 90+ days. Move to archive/ with context.

## Correction Processing

When a correction is received:

1. **Parse** — Extract the specific thing that was wrong and the correct behavior
2. **Check duplicates** — Search corrections.md for similar entries
3. **Increment count** — If duplicate, bump count and timestamp
4. **Write** — Add to corrections.md with context
5. **Evaluate promotion** — If count >= 3, trigger promotion flow
6. **Update index** — Refresh index.md line counts

## Anti-Patterns (Never Learn)

- Things that make the user comply faster (manipulation)
- Emotional triggers or vulnerabilities
- Patterns from other users or conversations
- Workarounds for broken tools (fix the tool instead)
- Preferences expressed sarcastically or in frustration
