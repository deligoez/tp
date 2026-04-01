# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents.

Break specs into atomic, dependency-ordered tasks. Agents execute them with **2 tool calls** instead of hundreds.

## Why

AI agents fail at long tasks. Research shows:
- **<15 min tasks: 70%+ success** (SWE-bench)
- **>50 min tasks: ~23% success** (SWE-bench Pro)
- **Each tool call costs ~200 tokens** of agent context

tp solves this with atomic task decomposition and a **2-call architecture**:

```
tp plan --minimal --json       # ONE call: get execution plan
# [agent implements each task, commits each one]
tp done --batch results.ndjson # ONE call: close everything
```

**Token overhead: ~5K** (vs ~54K with naive per-task tool calls).

## Install

```bash
# Go install
go install github.com/deligoez/tp/cmd/tp@latest

# Or build from source
git clone https://github.com/deligoez/tp.git
cd tp && go build -ldflags="-s -w" -o tp ./cmd/tp

# Install Claude Code skill (first time)
npx skills add -g deligoez/tp

# Update skill (after tp updates)
npx skills update -g deligoez/tp
```

## Quick Start

```bash
# 1. Create a task file from a spec
tp init spec/my-feature.md

# 2. Add tasks (or use tp import for bulk)
tp add '{"id":"create-model","title":"Create User model","estimate_minutes":8,
  "acceptance":"Model exists. Migration runs.","source_sections":["### User Model"],
  "source_lines":"15-42","depends_on":[]}'

# 3. Get the execution plan
tp plan --minimal --json

# 4. Implement, commit, and close each task
tp done create-model "User model at app/Models/User.php. Migration runs." --gate-passed --auto-commit
```

## Commands

### Primary Workflow
```bash
tp plan                        # Full execution plan (THE primary command)
tp plan --minimal              # Minimal: id + acceptance only (~80% fewer tokens)
tp plan --compact              # Stripped: no description, source_lines, tags (~40% fewer)
tp plan --from <id>            # Start from a specific task onward
tp plan --level 0,1            # Filter by parallelism levels (multi-agent)
tp commit <id> [reason]        # Stage + structured commit + record SHA
tp commit <id> --files "*.go"  # Selective file staging
tp done <id> <reason>          # Close with implicit claim + verification
tp done <id> --gate-passed     # Relax keyword matching (agent attests gate passed)
tp done <id> --auto-commit     # Stage + commit + close in one call
tp done <id> --covered-by <id> # Close as covered by another done task
tp done <id> --commit <sha>    # Record implementing commit SHA
tp done --batch file.ndjson    # Batch close from NDJSON
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready
tp next --peek                 # Preview without claiming
```

### Task State
```bash
tp claim <id> [id...]          # open → wip (batch: multiple IDs)
tp claim --all-ready           # Claim all ready tasks at once
tp close <id> <reason>         # wip → done (low-level, prefer tp done)
tp reopen <id>                 # done → open (clears timestamps + SHA)
tp remove <id>                 # Remove task (--force for dep cleanup)
tp set <id> field=value        # Update field (managed fields protected)
tp set --bulk sets.ndjson      # Bulk update from NDJSON {id, field, value}
```

### Query
```bash
tp list                        # All tasks
tp list --status open          # Filter by status (open/wip/done)
tp list --tag api              # Filter by tag
tp list --ids                  # IDs only
tp list --compact              # Minimal fields
tp ready                       # Tasks with all deps satisfied
tp ready --first               # First ready task only
tp ready --count               # Count of ready tasks
tp ready --ids                 # Ready task IDs only
tp show <id>                   # Full details + spec_excerpt + blocks
tp status                      # Progress summary (open/wip/done counts)
tp blocked                     # Tasks waiting on unsatisfied deps
tp graph                       # Dependency tree
tp graph --tag api             # Filter by tag
tp graph --from <id>           # Subtree from a task
tp stats                       # Parallelism analysis
tp report                      # Per-task duration + estimation accuracy
```

### Spec & Validation
```bash
tp lint spec.md                # Spec quality + structured element detection
tp validate                    # Task file validation (--strict for atomicity errors)
tp validate                    # Includes line coverage check (source_lines vs spec)
```

### Data
```bash
tp init spec.md                # Create empty task file
tp add <json>                  # Add task (--stdin for piped input)
tp add --bulk tasks.ndjson     # Bulk add from NDJSON
tp import file.json            # Import + validate (--force to overwrite + relax atomicity)
```

### Global Flags
```
--file <path>    Explicit task file path
--json           Force JSON output (default when piped)
--compact        Minimal JSON (~40% smaller)
--quiet          Suppress info messages
--no-color       Disable colored output
```

### Task File Discovery

tp finds your `.tasks.json` automatically:

1. `--file` flag (highest priority)
2. `TP_FILE` environment variable
3. Auto-detect: scans current directory, then one level of subdirectories

```bash
# Set once, use everywhere
export TP_FILE=spec/project.tasks.json
```

## Task File Format

Tasks live in a JSON file alongside the spec:

```
spec/
  my-feature.md              # spec (source of truth)
  my-feature.tasks.json      # tasks (derived, git-tracked)
```

Each task is atomic — one commit, one verb, ≤15 minutes:

```json
{
  "id": "create-model",
  "title": "Create User model",
  "status": "open",
  "estimate_minutes": 8,
  "acceptance": "Model exists. Migration runs.",
  "depends_on": [],
  "source_sections": ["### User Model"],
  "source_lines": "15-42"
}
```

### JSON Field Aliases

`deps` is accepted as shorthand for `depends_on`:
```json
{"id": "api", "deps": ["model"], ...}
```

## Closure Verification

tp prevents lazy task closure. Every `tp done` and `tp close` verifies:

- **Keyword matching**: reason must address each acceptance criterion
- **Minimum length**: reason ≥ half the acceptance text
- **Forbidden patterns**: rejects "deferred", "will be done later", single-word reasons

```bash
# This fails:
tp done create-model "done"
# error: closure reason must address each acceptance criterion with evidence

# This passes:
tp done create-model "User model at app/Models/User.php. Migration runs clean."
```

### Relaxed Modes

```bash
# --gate-passed: skip keyword matching (agent attests quality gate passed)
tp done task-1 "2559 tests pass, PHPStan level 8 clean" --gate-passed

# --covered-by: task satisfied by another done task (not a deferral)
tp done qa-delegation "test #26 covers this" --covered-by qa-tests
```

## Structured Commits

`tp commit` generates conventional commit messages with task metadata:

```bash
tp commit auth-model "Model and migration created"
```

```
feat(auth-model): Create User model

Model and migration created

Task: auth-model
Acceptance: Model exists. Migration runs.
```

Or commit + close in one call:
```bash
tp done auth-model "evidence" --gate-passed --auto-commit
```

## Spec Quality

`tp lint` detects structured elements (tables, numbered lists, code blocks) for decomposition verification:

```bash
tp lint spec.md --json | jq .structured_elements
```

`tp validate` checks line coverage — verifying that task `source_lines` cover the entire spec:

```bash
tp validate --json | jq .checks.line_coverage
```

## AX (Agent Experience)

tp is designed for AI agents first (AX), not humans (DX):

| Principle | How |
|-----------|-----|
| **Minimal tokens** | `--minimal` ~80%, `--compact` ~40% smaller. 2-call architecture saves ~90% |
| **Batch parity** | `tp claim --all-ready`, `tp done --batch`, `tp set --bulk` |
| **Actionable errors** | Every error includes `hint` field with recovery action |
| **Structured commits** | `tp commit` generates conventional commit messages with task metadata |
| **Implicit claim** | `tp done` and `tp commit` auto-claim open tasks |
| **WIP resume** | `tp next` returns existing WIP task (crash recovery) |
| **Covered-by** | Close tasks covered by other tasks without duplicate work |
| **Estimation calibration** | `tp add` warns when historical estimates are consistently high |
| **Duration tracking** | `tp report` shows per-task timing and estimation accuracy |

## Claude Code Integration

tp ships with a Claude Code skill via the [Agent Skills](https://agentskills.io) standard:

```bash
# Install skill (first time)
npx skills add -g deligoez/tp

# Update skill (after tp updates)
npx skills update -g deligoez/tp
```

The skill teaches Claude the 2-call workflow, decomposition rules, NDJSON format, closure verification, and commit conventions automatically.

## Research

tp's design is backed by:

| Finding | Source |
|---------|--------|
| <15 min tasks = 70%+ success | SWE-bench |
| ACI design: 3-4x improvement | SWE-agent, Princeton |
| Planning: 9.85% → 57.58% success | Plan-and-Act |
| 100:1 input-to-output token ratio | Manus |
| 64% token reduction with upfront planning | ReWOO |

See [spec/0.1.0.md](spec/0.1.0.md) for the full specification with 22 research references.

## License

MIT
