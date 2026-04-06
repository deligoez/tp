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
tp done <id> --auto-commit --files src/engine/*.go  # Selective staging + commit + close
tp done <id> --covered-by <id> # Close as covered by another done task
tp done <id> --commit <sha>    # Record implementing commit SHA
tp done id1 id2 id3 "reason"   # Multi-ID close (shared reason)
tp done --batch file.ndjson    # Batch close from NDJSON
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready
tp next --minimal              # Minimal output: {id, acceptance} only
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
tp review spec.md              # Adversarial review prompts (3 personas)
tp review spec.md --perspective code-audit --affected-files src/a.go  # Code audit with source files
tp review spec.md --round 2 --findings r1.ndjson  # Multi-round with previous findings
tp review spec.md --round 2 --final-round --affected-files src/a.go  # Final round: mandatory code read-through
tp review --merge r1.ndjson r2.ndjson -o merged.ndjson  # Merge + dedup findings
tp review --resolve findings.ndjson 3 fixed "evidence"  # Mark finding as fixed/wontfix/duplicate
tp review --resolve-all findings.ndjson wontfix "reason"  # Mark all unresolved findings
tp review --verify spec.md --findings all.ndjson  # Lightweight verification (verifier role)
tp review --report r1.ndjson r2.ndjson  # Cross-round convergence report
tp review spec.md --diff-from spec-r0.md  # Diff-based review (changed sections only)
tp review spec.md --spec-inline            # Embed full spec inline (default: reference mode)
tp review --resolve ... --force           # Force re-resolve already resolved findings
tp audit spec.md               # Post-implementation: verify code matches spec
tp audit spec.md --affected-files src/a.go  # Manual file selection
tp audit spec.md --findings review.ndjson  # Also verify review findings
tp validate                    # Task file + line coverage + atomicity (--strict)
```

### Data
```bash
tp init spec.md                # Create empty task file
tp add <json>                  # Add task (--stdin for piped input)
tp add --bulk tasks.ndjson     # Bulk add from NDJSON
tp import file.json            # Import + validate (--force to overwrite + relax atomicity)
tp import tasks.json --spec spec/feature.md  # Import bare JSON array (auto-wraps)
tp use spec.tasks.json         # Set active task file (.tp-active)
tp use --clear                 # Remove .tp-active marker
tp use                         # Show current active file
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

### Acceptance Criteria Delimiters

tp parses acceptance criteria using three delimiters:

| Delimiter | Example |
|-----------|---------|
| Period + space (`. `) | `"Model exists. Migration runs."` |
| Semicolon + space (`; `) | `"Model exists; migration runs"` |
| Bullet list (`\n- `) | `"- Model exists\n- Migration runs"` |

JSON arrays are also accepted and joined with `\n- ` on import.

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

`tp review` generates adversarial review prompts that agents feed to sub-agents:

```bash
# Default: 3 prompts (implementer, tester, architect)
tp review spec.md --json | jq '.prompts | length'
# → 3

# Perspective-specific review
tp review spec.md --perspective code-audit --affected-files src/a.go --json

# Documentation perspective
tp review spec.md --perspective documentation --json

# Testing perspective
tp review spec.md --perspective testing --json
```

For multi-round review, use `--round` and `--findings` to auto-exclude previously reported issues:

```bash
# Round 1: generate prompts, spawn sub-agents, collect findings to findings.ndjson
tp review spec.md --json > review-r1.json

# Round 2: tp auto-injects findings summary, prompts focus on new issues only
tp review spec.md --round 2 --findings findings.ndjson --json > review-r2.json
```

### Code-Aware Review

Inject source files into review prompts to catch state-dependent behaviors that specs miss:

```bash
# Inject files into default review — each prompt gets file content + checklist
tp review spec.md --affected-files src/form.vue src/api.ts

# Code audit perspective: C1-C5 checklist (state-dependent behaviors, spec coverage, etc.)
tp review spec.md --perspective code-audit --affected-files src/form.vue

# Final round: force mandatory code read-through to prevent false convergence
tp review spec.md --round 2 --final-round --affected-files src/form.vue
```

Files are capped at 8000 chars each (50000 total). Prompt budget enforced at 60000 chars total.

### Multi-Round Review Workflow

```bash
# R1: generate review prompts, spawn sub-agents, collect findings
tp review spec.md                                    # R1: generate review prompts

# Merge findings from multiple sub-agents
tp review --merge r1-*.ndjson -o r1.ndjson            # Merge + dedup findings

# Resolve individual findings
tp review --resolve r1.ndjson 3 fixed "evidence"      # Mark finding as fixed

# R2: diff-based review (only changed sections inline)
tp review spec.md --round 2 --findings r1.ndjson --diff-from spec-r0.md  # R2: diff-based

# Lightweight verification pass
tp review --verify spec.md --findings all.ndjson      # Lightweight verification

# Cross-round convergence report
tp review --report r1.ndjson r2.ndjson                # Convergence report
```

| Flag | Purpose |
|------|---------|
| `--merge` | Merge and dedup findings from multiple NDJSON files |
| `--resolve` | Mark a finding as fixed/wontfix/duplicate |
| `--resolve-all` | Mark all unresolved findings at once |
| `--verify` | Lightweight verification prompt (single prompt, verifier role) |
| `--report` | Cross-round convergence report |
| `--spec-inline` | Embed full spec inline (default: reference by path) |
| `--diff-from` | Diff-based review (only changed sections inline) |
| `-o` / `--output` | Output file path for merge |
| `--force` | Force re-resolve already resolved findings |

### Lint Checks

`tp lint` detects structured elements and quality issues:

```bash
tp lint spec.md --json | jq '.findings[] | select(.rule)'
```

| Rule | Severity | What it checks |
|------|----------|----------------|
| `structured-elements` | info | Tables, numbered lists, code blocks in spec |
| `acceptance-quality` | warning/info | Removal-only acceptance, vague verbs, short acceptance |
| `affected-files-scope` | warning | Modify rows in affected files table without scope description |
| `duplicate-line` | warning | Consecutive identical non-empty lines (edit artifacts) |
| `numbering-gap` | warning | Gaps in numbered section headings (e.g., 4.1 → 4.3, missing 4.2) |
| `orphan-list-item` | info | Numbered lists starting at >1 or with gaps (e.g., 1, 3 — missing 2) |

`tp validate` checks line coverage — verifying that task `source_lines` cover the entire spec:

```bash
tp validate --json | jq .checks.line_coverage
```

## Post-Implementation Audit

`tp audit` verifies that the spec's requirements actually made it into the code:

```bash
# Auto-detect changed files via git diff (zero-config)
tp audit spec.md --json

# Manual file selection
tp audit spec.md --affected-files src/form.vue src/api.ts

# Also verify review findings were addressed
tp audit spec.md --findings findings.ndjson
```

The command parses the spec's structured elements (table rows, numbered lists), task acceptance criteria, and optionally review findings. Each becomes a checklist entry. It reads the changed source files and generates adversarial prompts that verify each requirement against actual code.

```bash
tp audit spec.md --json | jq '.checklist_summary'
# → {"total": 12, "by_type": {"table_row": 5, "list_item": 4, "task_acceptance": 3}}
```

## AX (Agent Experience)

tp is designed for AI agents first (AX), not humans (DX):

| Principle | How |
|-----------|-----|
| **Minimal tokens** | `--minimal` ~80%, `--compact` ~40% smaller. 2-call architecture saves ~90% |
| **Batch parity** | `tp claim --all-ready`, `tp done --batch`, `tp set --bulk` |
| **Dependency-aware batch** | `tp done --batch` auto-toposorts by in-batch deps — no manual ordering needed |
| **Actionable errors** | Every error includes `hint` field with recovery action |
| **Did-you-mean** | `--covered-by` typos suggest similar task IDs |
| **Structured commits** | `tp commit` generates conventional commit messages with task metadata |
| **Implicit claim** | `tp done` and `tp commit` auto-claim open tasks |
| **WIP resume** | `tp next` returns existing WIP task (crash recovery) |
| **Covered-by** | Close tasks covered by other tasks without duplicate work |
| **Auto-normalize** | `source_lines` accepts `"72"` (normalized to `"72-72"`) |
| **Import flexibility** | `tp import` accepts bare JSON arrays with `--spec` flag |
| **Spec-only review** | Review prompts include disclaimer to prevent code-checking |
| **Edit hygiene lint** | `tp lint` detects duplicate lines and numbering gaps |
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
