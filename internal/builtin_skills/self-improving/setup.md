# Self-Improving Setup Guide

## First-Time Setup

Run these steps the first time a user installs or activates the self-improving skill.

### Step 1 — Create Memory Structure

```bash
mkdir -p ${WORKSPACE}/self-improving/{projects,domains,archive}
```

### Step 2 — Initialize Core Files

Create the following files from templates (see `memory-template.md`):

| File | Purpose |
|------|---------|
| `${WORKSPACE}/self-improving/memory.md` | HOT tier: ≤100 lines, always loaded |
| `${WORKSPACE}/self-improving/corrections.md` | Last 50 corrections |
| `${WORKSPACE}/self-improving/index.md` | Topic index with line counts |
| `${WORKSPACE}/self-improving/heartbeat-state.md` | Heartbeat state tracking |

### Step 3 — Choose Operating Mode

Ask the user which mode they prefer:

| Mode | Behavior |
|------|----------|
| **Passive** (default) | Log corrections only when user explicitly corrects you |
| **Active** | Also self-reflect after significant work |
| **Strict** | Active + ask before writing any memory entry |

Write the chosen mode to `${WORKSPACE}/self-improving/memory.md` header.

### Step 4 — Add SOUL.md Steering

Append the following block to the workspace `SOUL.md` (create if it doesn't exist). Do NOT overwrite existing content:

```markdown
**Self-Improving**
Compounding execution quality is part of the job.
Before non-trivial work, load `${WORKSPACE}/self-improving/memory.md` and only the smallest relevant domain or project files.
After corrections, failed attempts, or reusable lessons, write one concise entry to the correct self-improving file immediately.
Prefer learned rules when relevant, but keep self-inferred rules revisable.
Do not skip retrieval just because the task feels familiar.
```

### Step 5 — Refine AGENTS.md Memory Section (Non-Destructive)

If the workspace has an `AGENTS.md`, append a memory section (do NOT delete existing content):

```markdown
## Self-Improving Memory

This agent uses the self-improving skill for persistent learning.
- Memory location: `${WORKSPACE}/self-improving/`
- Hot memory is loaded automatically on session start
- Corrections are logged and promoted based on frequency
```

### Step 6 — Add HEARTBEAT.md Steering

If the workspace has a `HEARTBEAT.md`, append the self-improving heartbeat check (see `HEARTBEAT.md` template).

### Verification

After setup, confirm:
- [ ] `${WORKSPACE}/self-improving/` directory exists with subdirectories
- [ ] `memory.md`, `corrections.md`, `index.md`, `heartbeat-state.md` exist
- [ ] Operating mode is set
- [ ] SOUL.md steering is in place

Tell the user: "Self-improving memory is ready. I'll learn from corrections and self-reflect on my work."
