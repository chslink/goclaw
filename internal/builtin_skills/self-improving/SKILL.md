---
name: self-improving
description: 自我反思、自我纠错、自我学习的智能体技能。当以下情况发生时使用：(1) 命令、工具、API 或操作失败；(2) 用户纠正你或拒绝你的工作；(3) 你发现自己的知识过时或不正确；(4) 你发现了更好的方法；(5) 用户明确安装或引用此技能。
version: 1.2.16
---

**When to Use**: User corrects you or points out mistakes. You complete significant work and want to evaluate the outcome. You notice something in your own output that could be better. Knowledge should compound over time without manual maintenance.

**Architecture**: Memory lives in `${WORKSPACE}/self-improving/` with tiered structure. If `${WORKSPACE}/self-improving/` does not exist, run `setup.md`. Workspace setup should add the standard self-improving steering to the workspace AGENTS, SOUL, and `HEARTBEAT.md` files, with recurring maintenance routed through `heartbeat-rules.md`.

```
${WORKSPACE}/self-improving/
├── memory.md          # HOT: ≤100 lines, always loaded
├── index.md           # Topic index with line counts
├── heartbeat-state.md # Heartbeat state: last run, reviewed change, action notes
├── projects/          # Per-project learnings
├── domains/           # Domain-specific (code, writing, comms)
├── archive/           # COLD: decayed patterns
└── corrections.md     # Last 50 corrections log
```

**Quick Reference**:

| Topic | File |
|-------|------|
| Setup guide | `setup.md` |
| Heartbeat state template | `heartbeat-state.md` |
| Memory template | `memory-template.md` |
| Workspace heartbeat snippet | `HEARTBEAT.md` |
| Heartbeat rules | `heartbeat-rules.md` |
| Learning mechanics | `learning.md` |
| Security boundaries | `boundaries.md` |
| Scaling rules | `scaling.md` |
| Memory operations | `operations.md` |
| Self-reflection log | `reflections.md` |

**Learning Signals**:
- **Corrections** -> add to `corrections.md`, evaluate for `memory.md`: "No, that's not right...", "Actually, it should be...", "You're wrong about...", "I prefer X, not Y", "Remember that I always...", "I told you before...", "Stop doing X", "Why do you keep..."
- **Preference signals** -> add to `memory.md` if explicit: "I like when you...", "Always do X for me", "Never do Y", "My style is...", "For [project], use..."
- **Pattern candidates** -> track, promote after 3x
- **Ignore** (don't log): One-time instructions, context-specific, hypotheticals

**Self-Reflection**: After completing significant work, pause and evaluate: (1) Did it meet expectations? (2) What could be better? (3) Is this a pattern?

**Core Rules**:
1. Learn from Corrections and Self-Reflection
2. Tiered Storage (HOT <=100 lines / WARM <=200 lines each / COLD unlimited)
3. Automatic Promotion/Demotion (3x in 7 days -> HOT; unused 30 days -> WARM; unused 90 days -> COLD)
4. Namespace Isolation
5. Conflict Resolution (most specific wins)
6. Compaction
7. Transparency
8. Security Boundaries
9. Graceful Degradation
