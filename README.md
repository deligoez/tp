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
tp plan                        # ONE call: get full execution plan
# [agent implements all tasks]
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
tp init docs/my-feature.md

# 2. Add tasks (or use tp import for bulk)
tp add '{"id":"create-model","title":"Create User model","estimate_minutes":8,"acceptance":"Model exists. Migration runs.","source_sections":["### User Model"],"depends_on":[]}'

# 3. Get the execution plan
tp plan --json

# 4. Close tasks as you complete them
tp done create-model "User model at app/Models/User.php. Migration runs clean."
```

## The 2-Call Architecture

For AI agents, tp is optimized for minimal tool calls:

```
# Agent reads full plan once
plan=$(tp plan --json)

# Agent implements each task (zero tp calls)
# Builds results.ndjson as it goes

# Agent closes all tasks at once
tp done --batch results.ndjson
```

| Approach | Calls (67 tasks) | Token overhead |
|----------|-----------------|----------------|
| Naive (ready+claim+show+close) | 268 | ~54K |
| Compound (next+done per task) | 134 | ~35K |
| **2-call (plan+batch)** | **2-4** | **~5K** |

## Commands

### Primary Workflow
```bash
tp plan                        # Full execution plan (THE primary command)
tp plan --minimal              # Minimal: id + acceptance only (~80% fewer tokens)
tp commit <id> [reason]        # Stage + structured commit + record SHA
tp commit <id> --files "*.go"  # Selective file staging
tp done <id> <reason>          # Close with implicit claim + verification
tp done <id> --auto-commit     # Commit + close in one call
tp done --batch file.ndjson    # Batch close from NDJSON
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready
tp next --peek                 # Preview without claiming
```

### Task Management
```bash
tp claim <id> [id...]          # Claim tasks (--all-ready for batch)
tp close <id> <reason>         # Low-level close (prefer tp done)
tp reopen <id>                 # Reset to open
tp remove <id>                 # Delete task (--force for dep cleanup)
tp set <id> field=value        # Update field
tp set --bulk sets.ndjson      # Bulk update from NDJSON {id, field, value}
```

### Query
```bash
tp list                        # All tasks (--status, --tag, --compact)
tp ready                       # Tasks with deps satisfied
tp show <id>                   # Full details + spec excerpt
tp status                      # Progress summary
tp blocked                     # Waiting on deps
tp graph                       # Dependency visualization
tp stats                       # Parallelism analysis
tp report                      # Per-task duration + estimation accuracy
```

### Spec & Validation
```bash
tp lint spec.md                # Spec quality checks
tp validate                    # Task file validation (--strict)
tp init spec.md                # Create empty task file
tp import file.json            # Import + validate
```

### Global Flags
```
--json       Force JSON output (default when piped)
--compact    Minimal JSON (~40% smaller)
--quiet      Suppress info messages
--file       Explicit task file path
```

### Task File Discovery

tp finds your `.tasks.json` automatically:

1. `--file` flag (highest priority)
2. `TP_FILE` environment variable
3. Auto-detect: scans current directory, then one level of subdirectories

```bash
# Set once, use everywhere
export TP_FILE=spec/project.tasks.json
tp status   # just works
```

### JSON Field Aliases

`deps` is accepted as shorthand for `depends_on` in task JSON:

```json
{"id": "api", "deps": ["model"], ...}
```

## Task File Format

Tasks live in a single JSON file next to the spec:

```
docs/
  my-feature.md              # spec (source of truth)
  my-feature.tasks.json      # tasks (derived, git-tracked)
```

Each task is atomic — one commit, one verb, <=15 minutes:

```json
{
  "id": "create-model",
  "title": "Create User model",
  "status": "open",
  "estimate_minutes": 8,
  "acceptance": "Model exists. Migration runs.",
  "depends_on": [],
  "source_sections": ["### User Model"]
}
```

## Closure Verification

tp prevents lazy task closure. Every `tp done` and `tp close` verifies:

- **Keyword matching**: reason must address each acceptance criterion
- **Minimum length**: reason >= half the acceptance text
- **Forbidden patterns**: rejects "deferred", "will be done later", single-word reasons

```bash
# This fails:
tp done create-model "done"
# error: closure reason must address each acceptance criterion with evidence

# This passes:
tp done create-model "User model at app/Models/User.php with 12 fields. Migration 2024_01_create_users runs clean."
```

Use `--gate-passed` to relax keyword matching when the quality gate already passed:

```bash
# Accepted: agent attests gate passed, exact keyword match not required
tp done create-model "2559 tests pass, PHPStan level 8 clean" --gate-passed
```

## AX (Agent Experience)

tp is designed for AI agents first (AX), not humans (DX):

| Principle | How |
|-----------|-----|
| **Minimal tokens** | `--minimal` strips ~80%, `--compact` ~40%. 2-call architecture saves ~90% |
| **Batch parity** | `tp claim --all-ready`, `tp done --batch` |
| **Actionable errors** | Every error includes `hint` field with recovery action |
| **Structured commits** | `tp commit` generates conventional commit messages with task metadata |
| **Implicit claim** | `tp done` and `tp commit` auto-claim open tasks (no separate claim step) |
| **WIP resume** | `tp next` returns existing WIP task (crash recovery) |

## Claude Code Integration

tp ships with a Claude Code skill via the [Agent Skills](https://agentskills.io) standard:

```bash
# Install skill (first time)
npx skills add -g deligoez/tp

# Update skill (after tp updates)
npx skills update -g deligoez/tp
```

The skill teaches Claude the 2-call workflow, NDJSON format, closure rules, and discovery conventions automatically.

## Research

tp's design is backed by:

| Finding | Source |
|---------|--------|
| <15 min tasks = 70%+ success | SWE-bench |
| ACI design: 3-4x improvement | SWE-agent, Princeton |
| Planning: 9.85% -> 57.58% success | Plan-and-Act |
| 100:1 input-to-output token ratio | Manus |
| 64% token reduction with upfront planning | ReWOO |

See [docs/spec.md](docs/spec.md) for the full specification with 22 research references.

## License

MIT
