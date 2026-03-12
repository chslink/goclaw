# Scaling Patterns

## Scale Thresholds

| Scale | Entries | Strategy |
|-------|---------|----------|
| **Small** | <100 | Single memory.md, no namespacing needed |
| **Medium** | 100-500 | Split into domains/, basic indexing |
| **Large** | 500-2000 | Full namespace hierarchy, aggressive compaction |
| **Massive** | >2000 | Archive yearly, summary-only HOT tier |

## Tier Size Limits

| Tier | Max Lines | Overflow Action |
|------|-----------|-----------------|
| HOT (memory.md) | 100 | Demote least-used to WARM |
| WARM (each file) | 200 | Compact or demote to COLD |
| COLD (archive/) | Unlimited | Yearly rotation |

## Compaction Rules

### When to Compact
- Any WARM file exceeds 200 lines
- memory.md exceeds 100 lines
- Weekly maintenance cycle
- User requests "clean up memory"

### How to Compact
1. **Merge similar corrections** — "Use X not Y" appearing 3 times → single entry with count
2. **Summarize verbose patterns** — Long explanations → concise rule
3. **Archive with context** — When moving to COLD, include enough context to understand later
4. **Never lose data** — Compaction summarizes, never deletes. Original entries move to archive.

### Compaction Examples

Before:
```
- [correction] Don't use var, use const — 2026-01-10
- [correction] Prefer const over var — 2026-01-12
- [correction] Always use const, never var — 2026-01-14
```

After:
```
- [pref] Always use const, never var in JavaScript — confirmed 2026-01-14 (3x)
```

## Multi-Project Patterns

### Inheritance Chain
```
global (memory.md) → domain (domains/code.md) → project (projects/myapp.md)
```

More specific rules override less specific ones.

### Override Syntax
```markdown
- [override] For this project, use tabs not spaces (overrides global 2-space rule)
```

### Conflict Detection
When a new entry contradicts an existing one:
1. Note the conflict in the new entry
2. Ask the user which should win
3. If project-scoped, only override within that project
4. If global, update the original entry

## Index Maintenance

### index.md Format
```markdown
# Self-Improving Index

_Last updated: 2026-01-15_

| File | Lines | Entries | Last Modified |
|------|-------|---------|---------------|
| memory.md | 45 | 12 | 2026-01-15 |
| corrections.md | 30 | 8 | 2026-01-15 |
| domains/code.md | 120 | 35 | 2026-01-14 |
| projects/myapp.md | 25 | 7 | 2026-01-13 |
| archive/2025.md | 400 | 150 | 2026-01-01 |
```

Update index.md after any write operation to a self-improving file.
