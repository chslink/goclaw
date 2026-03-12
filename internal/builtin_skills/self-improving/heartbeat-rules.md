# Heartbeat Rules

Keep `${WORKSPACE}/self-improving/` organized without churn or data loss.

## Core Flow

1. **Start** — Ensure `${WORKSPACE}/self-improving/heartbeat-state.md` exists. Write `last_heartbeat_started_at` with current timestamp.

2. **Scan** — Check which files inside `${WORKSPACE}/self-improving/` changed since `last_reviewed_change_at`.

3. **No changes?** — Write `last_heartbeat_result: HEARTBEAT_OK` and stop.

4. **Changes found?** — Perform ONLY conservative tidying:
   - Refresh `index.md` line counts
   - Merge duplicate corrections in `corrections.md`
   - Move clearly misplaced notes to correct file (e.g., project-specific entry in global memory.md → projects/)
   - Trim corrections.md if over 50 entries (remove oldest)

5. **Finish** — Update `last_reviewed_change_at` and `last_heartbeat_result`. Log actions taken in `## Last actions`.

## Safety Rules

- **Most heartbeat runs should do nothing.** If nothing changed, `HEARTBEAT_OK`.
- **Prefer append, summarize, or index-fix** over rewrite.
- **Never delete data.** Move to archive/ if needed, never remove.
- **Never rewrite memory.md wholesale.** Only add, remove, or move individual entries.
- **Never promote patterns automatically.** Promotion requires user confirmation or 3x threshold.
- **Keep Last actions short** — one line per action, factual, no commentary.

## Error Handling

- If `heartbeat-state.md` is missing, create it from template (see `memory-template.md`).
- If `index.md` is missing, rebuild it by scanning all files.
- If any file is corrupted or unreadable, log the error and skip it — do not attempt repair.
