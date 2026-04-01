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

1. `tp lint <spec.md>` — fix all errors
2. Read spec, decompose into tasks (JSON), ensure 100% section coverage
3. `tp import tasks.json` — validates and stores

### B: Execute (tasks exist) — PRIMARY

The 2-call architecture minimizes token overhead:

```
# Phase 1: Get full plan (ONE call)
plan=$(tp plan --minimal --json)  # minimal: id + acceptance only (~80% fewer tokens)

# Phase 2: Implement each task (ZERO tp calls)
# Read task context from plan.execution_order
# For each task:
#   1. Read task.acceptance and task.spec_excerpt
#   2. Implement the task
#   3. Run plan.workflow.quality_gate
#   4. Record: {"id":"<id>","reason":"<evidence>","gate_passed":true,"commit":"<sha>"}
# Write results to results.ndjson
# Flush every 6-8 tasks if context is growing

# Phase 3: Close all tasks (ONE call)
tp done --batch results.ndjson
```

If batch reports failures: fix reasons, resubmit `tp done --batch fixes.ndjson`.

### C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first.

## JSON Field Aliases

- `deps` is accepted as an alias for `depends_on` in task JSON (import, add)

## NDJSON Result Format

One line per task:
```
{"id":"task-id","reason":"Evidence addressing each acceptance criterion.","gate_passed":true,"commit":"abc123"}
```

- `id` and `reason`: required
- `gate_passed`: set true after quality gate passes
- `commit`: git commit SHA (optional)

## Closure Rules

Before recording a result:
1. Re-read acceptance criteria from plan
2. Verify implementation matches FULL spec
3. Write reason addressing EACH criterion with file paths
4. Never use: "deferred", "covered by existing" (without proof), single-word reasons
5. Use `--gate-passed` (or `"gate_passed":true` in batch) to relax keyword matching — evidence like "2559 tests pass" is accepted without needing exact acceptance wording

**Important:** `tp done` auto-claims open tasks — no need for a separate `tp claim` call.

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
| `tp done --batch file` | Batch close (PRIMARY) |
| `tp next` | Incremental fallback |
| `tp done <id> "reason"` | Single close |
| `tp lint spec.md` | Spec quality check |
| `tp validate` | Task file validation |
| `tp set --bulk file` | Bulk update from NDJSON `{id, field, value}` |
| `tp list --status open` | Filter tasks |
| `tp status` | Progress summary |
