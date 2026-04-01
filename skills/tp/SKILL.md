---
name: tp
description: Spec-to-task lifecycle manager for AI coding agents. Decomposes specs into atomic tasks, manages execution order via dependency graph, and batch-closes with evidence.
---

# tp — Task Plan Skill

Spec-to-task lifecycle for AI agents. Manages atomic task decomposition, execution, and closure.

## Activation

This skill activates when:
- A `.tasks.json` file exists in the project
- User asks to implement a spec, plan, or tasks
- User references tp commands

## Workflows

### A: Decompose (spec exists, no .tasks.json)

1. `tp lint <spec.md>` — fix all errors, review `structured_elements` output
2. Read spec, decompose into tasks (JSON):
   - Every task MUST have `source_lines` mapping to spec line ranges (e.g., `"15-42"` or `"15-42,50-60"`)
   - Every table data row must appear in some task's acceptance criteria
   - Numbered test lists (#1, #2...) must preserve numbers in acceptance (e.g., "Tests: #26 lifecycle, #27 multi-pause, ...")
3. **Backward pass** — verify coverage:
   - For each table in `structured_elements`: does every row map to a task's acceptance?
   - For each numbered list: does every item have a task or explicit acceptance entry?
   - If spec has multiple checklists (e.g., "What Gets Added" + "Implementation Order"), take the union — items in one but not the other = potential missing task
   - Run `tp validate` — check `line_coverage` for uncovered spec line gaps
4. `tp import tasks.json` — validates and stores
5. If `tp validate` reports line coverage gaps, inspect uncovered ranges and add missing tasks or expand source_lines

### B: Execute (tasks exist) — PRIMARY

The 2-call architecture minimizes token overhead:

```
# Phase 1: Get full plan (ONE call)
plan=$(tp plan --minimal --json)  # minimal: id + acceptance only (~80% fewer tokens)

# Phase 2: Implement each task
# For each task in plan.execution_order:
#   1. Read task.acceptance
#   2. Implement the task
#   3. Run plan.workflow.quality_gate
#   4. tp commit <id> "evidence"           # structured commit, records SHA
#   5. tp done <id> "evidence" --gate-passed --commit <sha>

# Alternative: commit + close in one call per task
#   tp done <id> "evidence" --gate-passed --auto-commit

# Alternative: batch close (if commits already done)
#   Record results to results.ndjson
#   tp done --batch results.ndjson
```

If batch reports failures: fix reasons, resubmit `tp done --batch fixes.ndjson`.

### C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first.

## JSON Field Aliases

- `deps` is accepted as an alias for `depends_on` in task JSON (import, add)

## NDJSON Result Format

One line per task:
```
{"id":"task-id","reason":"Evidence addressing each acceptance criterion.","gate_passed":true,"started_at":"2026-04-01T13:00:00Z","commit":"abc123"}
```

- `id` and `reason`: required
- `gate_passed`: set true after quality gate passes
- `started_at`: ISO 8601 timestamp when you began the task (optional, enables `tp report`)
- `commit`: git commit SHA (optional)

## Closure Rules

Before recording a result:
1. Re-read acceptance criteria from plan
2. Verify implementation matches FULL spec
3. Write reason addressing EACH criterion with file paths
4. Never use: "deferred", "covered by existing" (without proof), single-word reasons
5. Use `--gate-passed` (or `"gate_passed":true` in batch) to relax keyword matching — evidence like "2559 tests pass" is accepted without needing exact acceptance wording

**Important:** `tp done` auto-claims open tasks — no need for a separate `tp claim` call.

**Code snippets:** When spec contains inline code, validate against the actual codebase (types, casts, method signatures) before implementing. Spec code may be illustrative, not literal.

## Task File Discovery

Priority: `--file` flag > `TP_FILE` env var > auto-detect (current dir, then one level of subdirs).

Set `TP_FILE` to avoid repeating `--file` every command:
```bash
export TP_FILE=spec/project.tasks.json
```

## Incremental Fallback

For interactive use or when full plan is impractical:
```
tp next          # get/resume WIP task
# implement
tp done <id> "reason" --gate-passed
```

## Key Commands

| Command | Purpose |
|---------|---------|
| `tp plan --minimal` | Minimal plan: id + acceptance (~80% fewer tokens) |
| `tp plan` | Full execution plan |
| `tp commit <id> [reason]` | Stage + structured commit + record SHA |
| `tp done <id> --auto-commit` | Commit + close in one call |
| `tp done --batch file` | Batch close |
| `tp next` | Incremental fallback |
| `tp done <id> "reason"` | Single close |
| `tp lint spec.md` | Spec quality check |
| `tp validate` | Task file validation |
| `tp set --bulk file` | Bulk update from NDJSON `{id, field, value}` |
| `tp list --status open` | Filter tasks |
| `tp status` | Progress summary |
| `tp report` | Per-task duration and estimation accuracy |
