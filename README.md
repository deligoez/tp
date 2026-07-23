# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents.

Break specs into atomic, dependency-ordered tasks. Agents execute them with **2 tool calls** instead of hundreds.

> **tp's primary user is the AI coding agent, not the human.** Every command, flag, and output is designed for *AX* (Agent Experience): minimal round-trips, minimal output tokens, deterministic behavior (no prompts), and actionable error hints. The human authors specs and approves releases; the **agent plans and the tool executes**.

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
# Homebrew (recommended)
brew tap deligoez/tap
brew install tp

# Go install
go install github.com/deligoez/tp/cmd/tp@latest

# Or build from source
git clone https://github.com/deligoez/tp.git
cd tp && go build -ldflags="-s -w" -o tp ./cmd/tp

# Install Claude Code skill (first time)
npx skills add -g deligoez/tp

# Update skill (after tp updates)
npx skills update -g
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
tp done <id> <reason>          # Close with implicit claim + verification; runs the quality gate
tp done <id> --skip-gate "why" # Skip gate execution, record gate_skipped_reason (needs user approval)
tp done <id> --auto-commit     # Stage + commit + close in one call
tp done <id> --auto-commit --files src/engine/*.go  # Selective staging + commit + close
tp done <id> --covered-by <id> # Close as covered by another done task
tp done <id> --commit <sha>    # Record implementing commit SHA
tp done id1 id2 id3 "reason"   # Multi-ID close (shared reason)
tp done <id> --commit a --commit b   # Record multiple commits (hc flow); commit_sha mirrors the first
tp done --batch file.ndjson    # Batch close from NDJSON
tp resume [spec]               # Reset-native: report phase + next action from durable state (read-only)
tp resume --compact            # Machine-only: strip summary/reason/message, keep every data field
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
tp set --workflow field=value  # Update workflow-level fields (convergence params)
tp set --bulk sets.ndjson      # Bulk update from NDJSON {id, field, value}
tp keep <path> "<reason>"      # Keep-list a deliberately-uncommitted file (--remove, --list)
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
tp review --merge r1.ndjson r2.ndjson -o merged.ndjson  # Merge + dedup (all-empty inputs exit 0; a clean round merges to an empty file)
tp review --resolve findings.ndjson 3 fixed "evidence"  # Mark finding (0-based index) as fixed/wontfix/duplicate
tp review --resolve-all findings.ndjson wontfix "reason"  # Mark all unresolved findings
tp review --verify spec.md --findings all.ndjson  # Lightweight verification (verifier role)
tp review --report r1.ndjson r2.ndjson  # Cross-round convergence report
tp review spec.md --diff-from spec-r0.md  # Diff-based review (changed sections only)
tp review spec.md --spec-inline            # Embed full spec inline (default: reference mode)
tp review --resolve ... --force           # Force re-resolve already resolved findings
tp review spec.md --record merged.ndjson   # Record a review round (auto-numbered)
tp review spec.md --status --check         # Convergence + mechanical checks; exit 0 when converged
tp review spec.md --perspective regression # Standalone regression delta pass
tp review spec.md --no-state               # Disable state (pre-0.23.0 manual --round)
tp audit spec.md               # Post-implementation: 3 role prompts, verify code matches spec
tp audit spec.md --affected-files src/a.go  # Manual file selection
tp audit spec.md --affected-from-tasks     # Audit files touched by done tasks' commit_shas
tp audit spec.md --findings review.ndjson  # Also verify review findings
tp audit spec.md --record results.ndjson   # Record an audit round (non-PASS = finding)
tp audit --merge r1.ndjson r2.ndjson -o results.ndjson  # Merge + dedup per-role audit results
tp audit spec.md --status --check          # Audit convergence; exit 0 when converged
tp validate                    # Task file + section/line coverage + atomicity (--strict)
```

### Data
```bash
tp init spec.md                # Create empty task file
tp add <json>                  # Add task (--stdin for piped input)
tp add --bulk tasks.ndjson     # Bulk add from NDJSON
tp import file.json            # Import + validate (--force to overwrite + relax atomicity)
tp import tasks.json --spec spec/feature.md  # Import bare JSON array (auto-wraps)
tp use spec.tasks.json         # Set active task file (writes .tp/local.json, git-ignored)
tp use --clear                 # Clear the active pointer
tp use                         # Show current active file
```

### Global Flags
```
--file <path>    Explicit task file path
--json           Force JSON output (default when piped)
--compact        Minimal JSON (~40% smaller); --no-compact forces full
--quiet          Suppress info messages; --no-quiet forces info output
--no-color       Disable colored output; --color forces color
```

### Task File Discovery

tp finds your `.tasks.json` automatically:

1. `--file` flag (highest priority)
2. `TP_FILE` environment variable
3. `.tp/local.json` active pointer (set with `tp use`)
4. Auto-detect: scans current directory, then one level of subdirectories

```bash
# Set once, use everywhere
export TP_FILE=spec/project.tasks.json
```

## Project Configuration

Multi-spec repos share **one** workflow policy instead of copying it into every `*.tasks.json` — so an agent working across specs reads a single source of truth and can't silently drift. A repo-root `.tp/` directory holds it:

- **`.tp/config.json`** (commit to VCS) — shared **workflow defaults**: `quality_gate`, `commit_strategy`, `review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `review_max_rounds`, `audit_max_rounds`, `lock_timeout_seconds`, `checks`.
- **`.tp/local.json`** (git-ignored automatically) — per-checkout state: the `active` task-file pointer (`tp use`) and CLI flag `defaults`.
- **`.tp/.gitignore`** — written automatically so `config.json` is tracked and `local.json` is not.

Discovery walks up from the current directory to the `.git` boundary to find `.tp/` — a single, deterministic anchor the agent never has to disambiguate.

### Layered resolution (resolve-at-read)

A task file's `workflow` block holds only **explicit overrides**; effective values merge at read time (never materialized — `tp init` and every read-modify-write keep the block sparse, presence-preserving since v0.26.0, so a materialized default can never mask `.tp/config.json`). Precedence, highest first:

```
CLI flag  >  environment  >  task-file workflow override  >  .tp/config.json  >  built-in default
```

Absent ≠ zero: a field counts as an override only when actually present, so `.tp/config.json` fills every gap a task file leaves. `checks` uses replace semantics (the winning layer's array wins whole).

### Commands

```bash
tp config                      # Effective configuration as JSON (what the agent will actually run)
tp config --resolved           # Annotate each setting with its {value, source} layer
tp config --extract            # Hoist policy shared by ALL task files into .tp/config.json
tp config --extract --dry-run  # Preview the hoist plan without writing
tp config --extract --force    # Merge into an existing .tp/config.json
tp set --workflow --project review_clean_rounds=3   # Edit a project-level workflow field
tp set --workflow --project commit_strategy=hc      # Set the project commit_strategy (builtin|auto|hc)
tp set --local defaults.compact=true                # Set a CLI flag default (compact/quiet/no_color)
tp validate --project          # Report cross-spec workflow drift (informational; --strict → exit 1)
```

Negating flags override a `defaults` entry for a single run: `--no-compact`, `--no-quiet`, `--color`.

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

`source_sections` entries use canonical heading form: `"## Heading Text"` (heading marker prefix +
space + heading text). `tp import` and `tp add` are lenient — plain text like `"User Model"` is
accepted and auto-normalized to canonical form when unambiguous (v0.22.0+). Use the full canonical
form when the same heading text appears at multiple levels (e.g. both `## Setup` and `### Setup`).

The task file's `workflow` section supports convergence parameters:

```json
{
  "workflow": {
    "quality_gate": "go test ./... && golangci-lint run",
    "gate_timeout_seconds": 600,
    "review_clean_rounds": 2,
    "audit_clean_rounds": 2,
    "review_max_rounds": 0,
    "audit_max_rounds": 0,
    "checks": []
  }
}
```

- `quality_gate` — command run automatically at `tp done`/`tp close`; read-only (author it at `tp init --quality-gate`)
- `gate_timeout_seconds` (default: 600, range: 30-3600) — hard timeout for a single gate run
- `lock_timeout_seconds` (default: 5, range: 1-60) — write-lock retry/backoff window before exiting 4 (v0.29.0)
- `review_clean_rounds` / `audit_clean_rounds` (default: 2, range: 1-10) — consecutive finding-free rounds required before decomposition / after implementation
- `review_max_rounds` / `audit_max_rounds` (default: 0 = no cap, range: 0-50) — round budget: at the cap while still unconverged, `tp review`/`tp audit` prompt generation and `--record` refuse with exit 4 and an escalation hint (raising the cap is a user-approved decision)
- `checks` — array of `{class, cmd}` mechanical detectors, replace semantics (see [Mechanical checks & finding class](#mechanical-checks--finding-class))
- Set via `tp set --workflow review_clean_rounds=3` (managed fields like `quality_gate` stay read-only), or during the skill's interview phase. `commit_strategy` is authored at `tp init`; change the project default with `tp set --workflow --project commit_strategy=<builtin|auto|hc>` (the task-file setter exits 2)

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

tp prevents lazy task closure with a deterministic, language-agnostic rule. Every `tp done` and `tp close`:

- **Evidence lines**: for a task with N ≥ 2 acceptance criteria, the reason must contain ≥ N lines each starting with `- ` at column 0 (indented sub-bullets do not count) — one top-level evidence line per criterion. A single-criterion task accepts any non-empty reason. The error enumerates each parsed criterion.
- **Forbidden patterns**: rejects "deferred", "will be done later", single-word reasons, and "covered by existing" without a path.

```bash
# This fails (3 criteria, no evidence lines):
tp done create-model "done"

# This passes (one "- " line per criterion; -- separates the reason from flags):
tp done create-model -- "- User model at app/Models/User.php:18
- migration 0007 applied, schema verified
- go test ./... green"
```

### Automatic quality gate

When `workflow.quality_gate` is set, `tp done`/`tp close` **run the command automatically** (once per invocation) before closing; a failing gate blocks the close (exit 4) and no task closes. `--gate-passed` is ignored when a gate is configured (it only records an attestation on gate-less projects). `--skip-gate "why"` skips execution and records `gate_skipped_reason` — a user-approved escape hatch, never the agent's own decision.

```bash
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

## Reset-Native Workflow (v0.28.0)

tp's target user is an AI agent whose context degrades over long runs. The most reliable pattern is to **reset the agent's context between units** — decompose, then run each atomic task, review round, and audit round in a fresh context. tp is the durable state machine that carries the work across those resets: it guarantees **resumability**; the orchestrator triggers the reset (a CLI subprocess cannot clear its caller's context).

### `tp resume` — the resume oracle

```bash
tp resume [spec]     # phase + the single next action, from durable state (read-only)
tp resume --compact  # machine-only: drop summary/reason/message, keep every data field
```

From the task file, the spec, `.tp-review/`, `.tp/local.json`, and git alone, `tp resume` reports which lifecycle phase the project is in and the concrete next action — the note a finishing agent leaves for the next one. It writes no file. Output:

```json
{
  "phase": "review | decompose | implement | audit | release",
  "spec": "spec/feature.md",
  "changes": ["src/a.go"],
  "kept": [{"path": ".env.local", "reason": "developer secrets"}],
  "bookkeeping": [{"path": "spec/feature.tasks.json", "kind": "closure", "ref": "auth-model"}],
  "guidance": "run each unit in a fresh subagent/context",
  "next_action": {"command": "tp next", "summary": "...", "payload": {"task": {"id": "x"}, "wip": false}},
  "blockers": [{"code": "unexplained-changes", "class": "agent-clearable", "message": "...", "data": {"count": 1}}]
}
```

Phase detection is **task-first**: an open task always reads as `implement`, even when the spec is stale (the staleness surfaces as a `spec-stale` blocker, it does not revert the phase). `next_action.payload` embeds the immediate work — for review/audit `{round, unresolved_findings}`, for implement `{task, wip}` — so a fresh agent needs one call, not a probe across `tp status`/`tp next`/`--status`.

Blockers carry a `class`: **`agent-clearable`** (`unexplained-changes` — the agent commits or `tp keep`s the change, then re-runs `tp resume`) or **`escalate`** (`no-ready-task`, `review-budget-exhausted`, `audit-budget-exhausted`, `spec-stale` — the driver stops and hands to a human). `bookkeeping` lists tp-owned dirty files (`{path, kind, ref}`; `kind` ∈ closure/round/config) that need committing — under `commit_strategy: hc` a close legitimately leaves them modified, so they are reported here (never as an `unexplained-changes` blocker). `guidance` is a one-line implement-phase note. Both survive `--compact`.

### Reference driver (runtime-neutral)

tp ships no driver — embedding the loop would bind tp to one agent runtime. The loop is the orchestrator's:

```
loop:
  r = tp resume
  for each blocker in r.blockers:
    if blocker.class == "agent-clearable":   # reconcile the unexplained change...
      commit it, or tp keep it, then restart the loop
    if blocker.class == "escalate":          # spec-stale, budget-exhausted, no-ready-task
      stop and hand to a human
  if r.blockers is empty:
    run r.next_action.command in a FRESH unit context (sub-agent / new session / new process)
  repeat until r.phase == "release"
```

**The orchestrator's injection duty (runtime-neutral).** A fresh unit does not see the orchestrator's history, so when you spawn one inject three things: (1) a **durable-state pointer** (`tp next`/`tp resume`) so it fetches its unit from disk; (2) the **close recipe** for the effective `commit_strategy` (`builtin`: `tp commit`/`tp done --auto-commit`; `hc`: `hc run` → `tp done --commit <sha>`); (3) the project's **live gotchas**.

**Realizing the reset in Claude Code.** The "FRESH unit" is a Claude Code **subagent** (Agent/Task tool) — clean context, one unit, work persisted to disk; the orchestrator re-orients via `tp resume` between units. Subagents don't nest, so the orchestrator runs each round's fan-out itself; for a *full* driver reset too, use `/clear` + `tp resume` or headless `claude -p` per unit.

**Driving tp from any agent runtime (v0.29.0).** tp is a CLI subprocess and the loop above is runtime-neutral — fan out each unit with whatever sub-agent primitive your runtime provides (a Task tool, a headless `agent -p`, a forked process, a fresh session); the injection duty above applies verbatim. **Permission prompts truncate the loop:** tp runs non-interactively, but review/audit units fan out to sub-agents that read files the runtime may gate behind a prompt — a headless runtime that **auto-denies** prompts truncates the round (sub-agents return empty findings instead of reading the blocked files) and produces a false clean round. Configure the runtime to allow the file reads tp's units need before driving the loop.

A unit returns to the loop only at a clean checkpoint; a crashed unit is recovered on the next `tp resume`, never lost.

### Commit strategy (`commit_strategy`)

`commit_strategy` resolves (task override > `.tp/config.json` > built-in) to one of:

- **`builtin`** — tp makes the commit. `tp commit`, `tp done --auto-commit`, and `tp done --commit <sha>` all work (pre-0.28.0 behavior).
- **`hc`** — tp makes no commit. The agent commits with `hc` (the hunk-commit tool), then records the SHAs with `tp done --commit <sha> [--commit <sha> …]`. `tp commit` and `tp done --auto-commit` are rejected (exit 2), as is a bare `tp done` (a close needs `--commit` or `--covered-by`). tp never runs `hc`, so its absence never fails a tp command.
- **`auto`** (the built-in default) — `hc` when it is on `PATH`, otherwise `builtin`.

`tp config` adds a top-level `commit_strategy_effective` (the concrete `builtin`/`hc` behavior after auto-resolution); `tp config --resolved` reports `commit_strategy` as `{value, source}`. A task closed with a commit records the ordered commits in **`commit_shas`** (`commit_sha` mirrors `commit_shas[0]` for older readers). `tp done --commit` is repeatable; a repeated sha exits 1.

`commit_strategy` is authored at `tp init`; set the **project default** with `tp set --workflow --project commit_strategy=<builtin|auto|hc>` (the task-file setter exits 2). Under `builtin`, `tp commit`/`tp done --auto-commit` fold the closure into the implementation commit so the committed task file shows `status: done` and `git status` is clean for tp-owned paths; `commit_sha` records the pre-amend implementation sha. Under `hc`, tp leaves the tp-owned closure files modified — `tp resume` reports them in `bookkeeping` (never as an unexplained change).

### The keep-list (`tp keep`)

Files a project deliberately leaves uncommitted live in the git-ignored `.tp/local.json` keep-list, so `tp resume` classifies them as known-intentional (`kept`) rather than unexplained (`changes`):

```bash
tp keep <path> "<reason>"   # add or update (a repeated path overwrites; globs use filepath.Match, no **)
tp keep --remove <path>     # drop (an absent path is a no-op, exit 0)
tp keep --list              # print the keep-list as JSON ([] when empty)
```

Paths are stored repo-root-relative from any subdirectory. After a close, `tp done`/`tp close` print a one-line stderr warning naming any uncommitted changes **not** on the keep-list (exit 0 — tp never commits or discards them). Feed the `kept[].path` values from `tp resume` into `hc`'s `allow_unplanned` so the same files stay out of the commit.

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

# Record the round: tp owns the `.tp-review/` state directory (commit it to VCS), auto-numbers rounds, and
# injects previous findings + the changed-sections diff into R2 automatically
tp review spec.md --record r1.ndjson                  # record round 1
tp review spec.md                                     # R2: auto diff + findings injected

# Lightweight verification pass
tp review --verify spec.md --findings all.ndjson      # Lightweight verification

# Convergence is a recorded fact — loop until this exits 0
tp review spec.md --status --check                    # converged AND all checks pass?
```

`tp review --status` (and `tp audit --status`) also report `max_rounds`/`rounds_remaining` (null when uncapped), `in_flight_round` (a started-but-unrecorded round — `tp resume` then points at recording it), and — over `--merge`/`--report`/`--status` — an `overlap_report` for trimming redundant roles. Prompt emission reports `skipped_roles` naming every corpus role it did not emit; merge/`--status` add `attribution_excludes` when excluding the `regression` role changes the finding count.

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
| `--record <file>` | Record a review round (auto-numbered R; freezes count + clean flag) |
| `--status` / `--status --check` | Show convergence state / gate exit 0 on converged + passing checks |
| `--perspective regression` | Standalone regression pass guarding settled decisions |
| `--no-state` | Disable state reads/writes (pre-0.23.0 manual `--round` numbering) |

### Mechanical checks & finding class

Review findings may carry an optional `class` — a kebab-case slug naming a pattern a script could detect across the whole spec. When a class recurs (≥ 2 distinct rounds, or ≥ 5 times in one round), `tp review --report` and `--record` surface it under `mechanize_candidates`, alongside a `by_class` breakdown. Turn a recurring class into a permanent detector:

```bash
tp set --workflow checks='[{"class":"code-citation-drift","cmd":"scripts/check-citations.sh"}]'
```

tp then runs every registered check at the start of each review round, reports pass/fail under `mechanical_checks`, and tells reviewers to stop hand-reporting that class. `tp review --status --check` exits 0 only when the review is converged **and** every check passes. (`checks` uses replace semantics — one `tp set --workflow` call sets the whole array.)

### Spec frontmatter (`tp:` domain & role overrides)

A spec can open with a YAML frontmatter block; tp reads only the `tp:` mapping (line numbers stay absolute, and the block is excluded from every parser):

```yaml
---
tp:
  domain: prose          # default "software"; selects & filters the role corpus (see Role Corpus)
  review_roles:          # append focus questions to an existing reviewer role
    implementer:
      focus:
        - "Can each section be written without inventing facts not in the outline?"
  audit_roles:           # same, for an auditor role
    spec-coverage:
      focus:
        - "Is every outline element present and fully developed?"
---
```

- `domain` no longer swaps Go personas; it **selects the embedded default corpus** (`software` → implementer/tester/architect; `prose` → the leaner coherence/soundness panel) and **filters** a user corpus by each role's `domains`. An unknown domain falls back to `software` with a lint warning.
- `tp.review_roles` / `tp.audit_roles` are **additive overrides**: each maps a role id to `{focus: [...]}` whose questions are appended to that role's corpus focus (project focus first). An override id matching no active role is ignored with a lint warning; the built-in `regression` role accepts no overrides. New roles are files, not frontmatter.
- The standalone `tp: lens` block is **retired**; a legacy `lens` still auto-translates to review-role focus with a deprecation warning (the new `review_roles`/`audit_roles` form wins when both are present).

### Role Corpus (v0.25.0)

Review and audit roles are **project-owned data** — one JSON file per role under the repo-root `.tp/`:

```
.tp/reviewers/*.json   # tp review roles (phase inferred from the directory)
.tp/auditors/*.json    # tp audit roles
```

Schema `{id, title, instructions, focus[], domains[]}`: `id` MUST equal the filename stem and match `^[a-z0-9]+(-[a-z0-9]+)*$`; `regression` is reserved. tp owns the finding output contract — a role only customizes the prompt (persona, instructions, focus questions).

```bash
# Make the built-in default prompts visible & editable
tp init --eject-roles                 # software corpus (default)
tp init --eject-roles --domain prose  # prose corpus
tp init --eject-roles --force         # overwrite existing role files
```

A populated phase directory **replaces** the embedded default corpus for that phase; an absent or empty one keeps the built-in defaults, so a project happy with defaults carries zero role files. `tp lint`, `tp review`, and `tp audit` validate the corpus and abort with exit 3 on a malformed role file (`repair or delete <path>`), each for the phase it reads (a broken auditor never blocks review).

**Per-role overlap report.** `tp review --merge` clusters findings by `(location, class)` and reports, per reviewer role, its `unique` (sole-contributor) and `shared` (co-found, `found_by >= 2`) cluster counts. A role with `unique == 0` and `shared >= 1` is flagged a **trim candidate** — it found only what others also found. tp only reports; trimming is your decision (edit the role files). The report also appears in `tp review --report` and `--status`.

**Role staleness.** Each phase has a corpus hash (`"builtin"` on the defaults) stored on every recorded round; `tp review --status` / `tp audit --status` report `roles_stale` beside the spec `stale` flag when the corpus changed since the last round. A pre-v0.25.0 round has no stored hash and is treated as matching, so upgrading never forces a re-review.

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
| `duplicate-paragraph` | warning | Two consecutive identical paragraphs — copy-paste artifacts the line-level check misses |
| `broken-cross-ref` | warning | `§X.Y step N` where section X.Y has fewer than N numbered steps |
| `empty-section` | warning | A leaf heading with no body content (container headings — whose next heading is deeper — are skipped) |

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

# Audit exactly the files touched by done tasks' commit_shas (post-implementation default)
tp audit spec.md --affected-from-tasks

# Also verify review findings were addressed
tp audit spec.md --findings findings.ndjson
```

When `tp audit` finds no audit-able file it exits 4 with a `suggested_files` array (the type-filtered union of paths touched by done tasks' `commit_shas`) and a hint naming `--affected-files`/`--affected-from-tasks`; `--affected-from-tasks` runs that derivation directly. `tp audit --merge` dedups per-role results (by `role`+`item_id`); like `tp review --merge`, all-empty inputs exit 0.

The command parses the spec's structured elements (table rows, numbered lists), task acceptance criteria, and optionally review findings, then emits **one prompt per non-empty role** — `spec-coverage`, `security`, `maintainability-conventions` (v0.23.0). Each prompt carries an embedded JSON-array checklist and its per-role affected files; sub-agents return one NDJSON row per checklist item (`status` ∈ PASS/PARTIAL/FAIL). Record rounds and converge like review:

```bash
tp audit spec.md --json | jq '.prompts[].role'
# → "spec-coverage"  "security"  "maintainability-conventions"

tp audit spec.md --record results.ndjson    # non-PASS rows count as findings
tp audit spec.md --status --check            # exit 0 only when the audit is converged
```

> **Schema break:** v0.23.0 audit JSON is incompatible with v0.22.0 — `role` is one of the three above (was `implementation-auditor`), the `category` field is removed, and `checklist_items` / `affected_files` are new. Downstream consumers must update; there is no `--legacy-format` flag.

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
| **Edit hygiene lint** | `tp lint` detects duplicate lines/paragraphs, numbering gaps, and broken cross-refs |
| **Estimation calibration** | `tp add` warns when historical estimates are consistently high |
| **Duration tracking** | `tp report` shows per-task timing and estimation accuracy |
| **Entry validation** | `tp add` rejects bad tasks at entry (no id/title/acceptance/anchor), normalizes slices to `[]` |
| **Coverage on write** | `tp add`/`set`/`remove` recompute coverage, so init+add+validate is clean |
| **Audit file hint** | `tp audit` suggests files from done tasks' commits when none are detected |
| **Loop budget** | `--status` shows `max_rounds`/`rounds_remaining`/`in_flight_round` |

## Claude Code Integration

tp ships with a Claude Code skill via the [Agent Skills](https://agentskills.io) standard:

```bash
# Install skill (first time)
npx skills add -g deligoez/tp

# Update skill (after tp updates)
npx skills update -g
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
