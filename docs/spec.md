# tp -- Task Plan: Spec-to-Task Lifecycle for AI Agents

## 1. Problem Statement

AI coding agents fail at long tasks. Research shows exponential success decay: doubling task duration squares the failure rate (Toby Ord, 2025). SWE-bench data: <15 min tasks achieve 70%+ success; >50 min tasks drop to ~23%. Context rot compounds the problem -- LLM accuracy drops from 99% to 50% as input grows (Chroma Research).

**The scarce resource is agent context tokens.** Every tool invocation costs the agent tokens -- the finite resource that determines how much work an agent can do before it degrades. At a 100:1 input-to-output token ratio (Manus), every unnecessary output byte is amplified a hundredfold in its impact on the agent's reasoning capacity.

**The 2-call architecture** solves this: `tp plan` + `tp done --batch` = 2 calls for the entire plan, reducing token overhead by ~90% (see Section 3.2 for the full analysis).

The solution is atomic task decomposition: break large specs into small, dependency-ordered tasks that agents execute independently. But no existing tool provides the full lifecycle:

| Tool | Structured Format | Atomicity Enforcement | Closure Verification | Parallelism Analysis | Spec Lint | 2-Call Architecture | Token Overhead |
|------|-------------------|----------------------|---------------------|---------------------|-----------|---------------------|----------------|
| Taskmaster AI | JSON | No | No | No | No | No | High |
| GSD | YAML + MD | No | No | No | No | No | High |
| Spec Kit | MD (.specify/) | No | No | No | Partial | No | High |
| Beads | SQLite + JSONL | No | No | No | No | No | Very High (~500ms/op) |
| Claude Code Tasks | JSONL (~/.claude/) | No | No | No | No | No | Medium |
| **tp** | **JSON (spec-adjacent)** | **Yes** | **Yes** | **Yes** | **Yes** | **Yes** | **~5K tokens** |

Note: Task decomposition (AI reading a spec and producing tasks) is the AI's job, not the tool's. `tp` provides the structured format, validates the output, and manages the lifecycle.

### Motivation

A prior system (beads-from-plan) revealed five failure modes that `tp` is designed to eliminate:

1. **Silent task dropping** -- Tasks lost during creation with no error output
2. **Early task closure** -- Agent closed tasks as "deferred" without completing the work
3. **Tool overhead** -- Database-backed CLI added ~500ms per operation; 20 tasks = 10s overhead
4. **Missing enforcement** -- Atomicity rules existed as docs but were not enforced by tooling
5. **Context overhead** -- Each tool call consumes agent context tokens. Verbose tools accelerate context rot.

---

## 2. Core Principles

### 2.1 Four Foundational Principles

| # | Principle | Definition |
|---|-----------|------------|
| P1 | AX First | Every decision optimizes for the AI agent as primary user |
| P2 | Batch Parity | What is easy for 1 task must be equally easy for N tasks |
| P3 | Minimal Tokens | Every output byte costs agent context. Unnecessary info = harmful info |
| P4 | Agent Plans, Tool Executes | Agent produces decisions (JSON), tool deterministically executes. Planning happens at decomposition time, not during execution. |

### 2.2 AX (Agent Experience)

AX (Agent Experience) is to AI agents what DX (Developer Experience) is to human developers. tp is designed AX-first. The 2-call architecture reduces token overhead by ~90% (see Section 3.2).

**Research backing:**
- ACI (Agent-Computer Interface) design gave 3-4x improvement without changing models (SWE-agent, Princeton, 2024)
- Anthropic tool design: human-written tools achieved 70% accuracy; agent-optimized tools achieved 95% (Anthropic, 2025)
- ReWOO: planning all tool calls upfront gave 64% token reduction with +4.4% accuracy (Xu et al., 2023)

**Principle: Actionable errors.** Every error message tells the agent exactly what to do next:
- BAD: "cannot claim task"
- GOOD: "cannot claim auth-login: blocked by auth-user-model (status: open). Complete auth-user-model first, then retry."

**Principle: Context-efficient output.** The `--compact` flag strips descriptions, source_lines, and metadata from JSON output, reducing token usage by ~40%.

**Principle: Batch-native (P2).** All write commands accept multiple IDs or batch input: `tp claim id1 id2 id3`, `tp done --batch results.ndjson`. The agent handles N tasks as easily as 1.

### 2.3 Spec-Adjacent Storage

The task file lives next to the spec it was decomposed from:

```
docs/
  typed-inter-machine-communication.md          <- spec (source of truth)
  typed-inter-machine-communication.tasks.json  <- tasks (derived, git-tracked)
```

Convention: `{spec-basename}.tasks.json`. The tool auto-detects by scanning for `*.tasks.json` in the current directory or a specified path.

### 2.4 Single File, Single Source of Truth

All task state lives in one JSON file. No database, no daemon, no sync. The file is:
- Git-trackable (diffable, mergeable, branchable)
- Human-readable (always written as pretty-printed JSON with 2-space indentation)
- Machine-parseable (JSON Schema validatable)
- Self-contained (no external references except the source spec path)

### 2.5 Agent-First Interface

Every command outputs structured JSON when stdout is not a TTY (or when `--json` is passed). The `--compact` flag further reduces output by ~40% for token-constrained sessions. This enables pipe-friendly workflows:

```bash
tp plan --json | jq '.execution_order[].id'   # all task IDs in order
tp ready --first | jq -r '.id'                # get next task ID
```

Human-friendly colored output when interactive. `--quiet` suppresses info-level messages. `NO_COLOR` env var respected.

### 2.6 Tool Does Deterministic Work, AI Does Semantic Work

tp is a passive CLI tool. It never calls the agent. The agent (Claude Code) calls tp via Bash.

| Concern | Who | How |
|---------|-----|-----|
| Spec validation (structure, vague words, sizing) | `tp lint` | Regex, line counting, heading parsing |
| Task validation (atomicity, coverage, cycles) | `tp validate` | Struct validation, DAG algorithms, spec parsing |
| Parallelism computation | `tp stats` | Topological sort, level assignment |
| Closure verification | `tp close` / `tp done` | Keyword matching against acceptance criteria (always on) |
| Spec decomposition (understanding content) | AI + skill | SKILL.md instructions |
| Acceptance criteria authoring | AI + skill | SKILL.md patterns |

### 2.7 Zero Required Infrastructure

No daemon. No database. No config file. No required init step. The first `tp add` or `tp import` creates the `.tasks.json` file automatically if it does not exist. `tp init` exists as a convenience to create the skeleton explicitly, but it is never required.

---

## 3. Architecture

### 3.1 System Overview

```
Claude Code (AI Agent)  --[Bash calls]-->  tp CLI (Passive Tool)  --[reads/writes]-->  .tasks.json
   Reads SKILL.md                            No AI. No network.                         Single file.
   Does semantic work                        Deterministic only                         Git-tracked.
   (understand, implement)                   (validate, sort, match)                    Source of truth.
```

### 3.2 The 2-Call Architecture

The optimal agent workflow uses exactly 2 tp calls per session:

```
1. tp plan --json          -> Agent gets full execution context
2. [Agent implements all tasks, building results.ndjson]
3. tp done --batch results.ndjson  -> Tool closes all tasks
```

For long sessions (>8 tasks), agents flush completed work periodically:

```
1. tp plan --json          -> Full plan
2. [Implement tasks 1-8]
3. tp done --batch chunk1.ndjson   -> Flush
4. [Implement tasks 9-16]
5. tp done --batch chunk2.ndjson   -> Flush
...
```

Session boundaries serve CONTEXT MANAGEMENT, not planning scope. The plan is complete from step 1.

Token budget for 67-task plan:

| Approach | Calls | Token overhead |
|----------|-------|----------------|
| Naive (ready+claim+show+close) | 268 | ~54K |
| Compound (next+done) | 134 | ~35K |
| **2-call (plan+batch)** | **2-4** | **~5K** |

### 3.3 The Skill Layer

tp is useless without the skill that teaches Claude Code how to use it. The skill is a SKILL.md file (Claude Code skill) that provides:
- When to activate (detects .tasks.json or spec files)
- Which workflow to follow (decompose, execute, or resume)
- Exact commands to run
- How to interpret output

The skill is part of tp's distribution. See Section 11 for full skill design.

### 3.4 Multi-Agent Support

tp's parallelism analysis (Section 9) identifies which tasks can run simultaneously. Multi-agent orchestration:

```
Orchestrator: tp plan --json -> distribute tasks by parallelism level
Agent 1: Level 0 tasks (no deps) -> tp done --batch
Agent 2: Level 0 tasks (parallel) -> tp done --batch
[Level 0 complete]
Agent 1: Level 1 tasks -> tp done --batch
...
```

File locking (flock) prevents concurrent write corruption. Multiple agents can safely close tasks simultaneously.

---

## 4. Data Format

### 4.1 Task File Schema

```json
{
  "version": 1,
  "spec": "docs/typed-inter-machine-communication.md",
  "created_at": "2026-03-30T10:00:00Z",
  "updated_at": "2026-03-30T14:30:00Z",
  "workflow": {"quality_gate": "composer test && composer lint", "commit_strategy": "agentic-commits"},
  "coverage": {"total_sections": 43, "mapped_sections": 40, "context_only": ["# Title", "## Overview"], "unmapped": []},
  "tasks": [{
    "id": "auth-user-model", "title": "Create User model with migration",
    "description": "Define User model with email, password_hash, timestamps.",
    "status": "open", "tags": ["auth", "model"], "depends_on": [], "estimate_minutes": 8,
    "acceptance": "User model at app/Models/User.php. Migration creates users table. Factory generates valid instances.",
    "source_sections": ["### 1.1 User Model"], "source_lines": "15-42",
    "closed_at": null, "closed_reason": null, "gate_passed_at": null, "commit_sha": null
  }]
}
```

### 4.2 Field Definitions

#### Root Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | integer | Yes | Schema version (currently 1) |
| `spec` | string | Yes | Relative path to source spec markdown |
| `created_at` | ISO 8601 | Yes | When the task file was created |
| `updated_at` | ISO 8601 | Yes | When any task was last modified |
| `workflow` | object | No | Default quality gate and commit strategy |
| `coverage` | object | Yes | Section-to-task mapping summary |
| `tasks` | array | Yes | Task list (flat, no nesting) |

#### Workflow Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `quality_gate` | string | `""` | Shell command that must exit 0 after each task |
| `commit_strategy` | string | `"agentic-commits"` | How to commit: `agentic-commits`, `conventional`, `manual` |

#### Coverage Fields

| Field | Type | Description |
|-------|------|-------------|
| `total_sections` | integer | Total content headings in spec |
| `mapped_sections` | integer | Headings mapped to at least one task |
| `context_only` | string[] | Headings marked as non-actionable (titles, overviews, references) |
| `unmapped` | string[] | Headings not mapped to any task -- **must be empty** |

#### Task Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique kebab-case identifier |
| `title` | string | Yes | Imperative verb + object (<=8 words) |
| `description` | string | No | Implementation context beyond title (<=300 chars) |
| `status` | enum | Yes | `open`, `wip`, `done` |
| `tags` | string[] | No | Grouping labels (replaces epic hierarchy) |
| `depends_on` | string[] | Yes | Task IDs that must be `done` before this can start |
| `estimate_minutes` | integer | Yes | 1-15 (enforced). 1 means <=1 minute. Sub-minute tasks are valid and encouraged. |
| `acceptance` | string | Yes | Verifiable criteria (<=3 sentences) |
| `source_sections` | string[] | Yes | Spec headings this task implements |
| `source_lines` | string | No | Line range in spec (e.g., "15-42") |
| `closed_at` | ISO 8601 | No | When task was closed (set by `tp close` / `tp done`) |
| `closed_reason` | string | No | Evidence of completion (set by `tp close` / `tp done`) |
| `gate_passed_at` | ISO 8601 | No | When quality gate last passed (set by `tp done --gate-passed`) |
| `commit_sha` | string | No | Git commit implementing this task (set by `tp done --commit`) |

### 4.3 Design Decisions

**Flat task list (no epics/subtasks):** Epics add hierarchy complexity without value for agents. Tags provide the same grouping ability (`--tag auth` filters like an epic would). An agent executing tasks cares about dependency order, not organizational hierarchy.

**Three statuses only:** `open` -> `wip` -> `done`. No `blocked` status (computed dynamically from deps). No `deferred` status (forbidden by closure rules). Multiple tasks can be `wip` simultaneously to support multi-agent parallelism.

**Acceptance is required:** Every task must have verifiable acceptance criteria. This is the enforcement mechanism for early closure prevention. The tool validates that this field is non-empty and that closure reasons address it.

**estimate_minutes 1-15 range:** Hard limit enforced by validation. Research-backed: METR shows exponential success decay beyond ~35 min, SWE-bench shows <15 min tasks at 70%+ success. The 15-minute cap keeps every task well within the high-success zone.

**description is optional:** The `title` (what) + `acceptance` (done when) are sufficient for most tasks. `description` provides optional implementation hints when the title alone is ambiguous. Capped at 300 chars to prevent scope creep -- if you need more, the task is too broad.

**The plan is the task file:** Planning happens at decomposition time (`tp import`), not during execution. The agent receives the full plan from `tp plan` and executes it. No replanning step built into tp.

**Managed fields:** `status`, `closed_at`, `closed_reason`, `gate_passed_at`, and `commit_sha` are managed exclusively by tp commands (`claim`, `close`, `done`, `reopen`). They cannot be set via `tp set` -- this prevents bypassing closure verification. `tp reopen` clears all closure-related fields: `closed_at`, `closed_reason`, `gate_passed_at`, `commit_sha`.

---

## 5. CLI Interface

### 5.1 Command Overview

```
tp -- task-plan: spec-to-task lifecycle for AI agents

USAGE:
  tp <command> [options] [arguments]

PLAN COMMANDS (primary workflow):
  tp plan                       Full execution plan with context (THE primary command)
  tp done <id> <reason>         Close task with verification
  tp done --batch <file>        Batch close from NDJSON file

INCREMENTAL COMMANDS (fallback / multi-agent):
  tp next                       Get or resume WIP task, or claim next ready
  tp next --peek                Preview next without claiming

SPEC COMMANDS:
  tp lint <spec.md>             Deterministic spec quality checks

TASK STATE COMMANDS:
  tp ready                      List tasks with all deps satisfied
  tp show <id>                  Show task details with spec excerpt
  tp claim <id> [id...]         Transition: open -> wip (multiple IDs supported)
  tp close <id> <reason>        Transition: wip -> done (low-level, prefer tp done)
  tp reopen <id>                Transition: done -> open
  tp remove <id>                Remove a task (open only)
  tp set <id> <field>=<value>   Update a task field

QUERY COMMANDS:
  tp list                       List tasks with filters
  tp status                     Summary counts
  tp blocked                    Tasks waiting on deps
  tp graph                      Dependency graph

VALIDATION COMMANDS:
  tp validate                   Full validation
  tp validate --strict          Atomicity violations become errors

ANALYSIS COMMANDS:
  tp stats                      Estimates, parallelism, breakdown

DATA COMMANDS:
  tp add <task-json>            Add single task
  tp add --bulk <path>          Bulk add from NDJSON
  tp import <file>              Import complete task file
  tp init <spec.md>             Create empty task file

GLOBAL FLAGS:
  --file <path>                 Explicit task file (default: auto-detect *.tasks.json)
  --json                        Force JSON output (default when not a TTY)
  --compact                     Minimal JSON output (see 5.2)
  --quiet                       Suppress info messages
  --no-color                    Disable colors
  --version                     Show tp version
  --help                        Show help
```

### 5.2 --compact Output Mode

The `--compact` flag reduces JSON output size by ~40%, saving agent context tokens. Fields omitted in compact mode:

| Field | Compact behavior |
|-------|-----------------|
| `description` | omitted |
| `source_sections` | omitted |
| `source_lines` | omitted |
| `tags` | omitted |
| `closed_reason` | omitted |
| `spec_excerpt` | omitted |

Fields always kept: `id`, `title`, `status`, `depends_on`, `estimate_minutes`, `acceptance`, `closed_at`.

### 5.3 Command Details

#### `tp plan`

THE primary command. Returns the full execution plan in one call.

```bash
tp plan                    # full plan, all open/wip tasks
tp plan --compact          # without spec_excerpts and descriptions
tp plan --from <id>        # from this task onward (resume after partial session)
tp plan --level 0,1        # specific parallelism levels only (multi-agent)
```

**Output (JSON):**

```json
{
  "workflow": {
    "quality_gate": "go test ./... && golangci-lint run",
    "commit_strategy": "agentic-commits"
  },
  "execution_order": [
    {
      "id": "auth-user-model",
      "title": "Create User model",
      "description": "Define User model with email, password_hash, timestamps.",
      "tags": ["auth", "model"],
      "acceptance": "Model at app/Models/User.php. Migration runs.",
      "estimate_minutes": 8,
      "depends_on": [],
      "source_sections": ["### 1.1 User Model"],
      "source_lines": "15-42",
      "spec_excerpt": "### 1.1 User Model\n\n..."
    }
  ],
  "summary": {
    "total": 67,
    "remaining": 67,
    "done": 0,
    "estimated_minutes": 551,
    "parallelism_levels": 8
  }
}
```

**Behavior:**
- Tasks are in topological order (dependency-safe execution order)
- Done tasks are excluded from `execution_order`
- WIP tasks appear first (resume support)
- `spec_excerpt` is capped at 2000 characters per task. If longer, truncated with `\n[...truncated, see spec lines 15-42]`
- `--from <id>`: starts the execution_order from the given task ID onward, skipping earlier tasks in the topological order. If the task is `done`, starts from the next non-done task after it. Useful for resuming a partial session.
- `--level 0,1`: filters to only tasks at the specified parallelism levels. Useful for multi-agent distribution.

**When no remaining tasks:** exit 4 with:

```json
{"done": true, "message": "All tasks complete"}
```

**Exit codes:** 0 = success, 4 = no remaining tasks.

#### `tp done <id> <reason>`

Close a single task with verification. Preferred over `tp close` because it includes `--gate-passed`, `--commit`, `has_next`, and implicit claim (open tasks are auto-claimed before closing).

```bash
tp done <id> <reason>                                # reason as argument
tp done <id> --stdin                                 # reason from stdin
tp done <id> --reason-file evidence.txt              # reason from file
tp done <id> <reason> --gate-passed                  # attest quality gate passed
tp done <id> <reason> --commit abc123                # record commit SHA
tp done <id> <reason> --gate-passed --commit abc123  # both
```

**`--gate-passed`:** Records `gate_passed_at` timestamp. tp does NOT run the gate (agent's job per P4). If omitted, `tp done` succeeds but warns: "quality gate not attested."

**`--commit`:** Records `commit_sha`. Optional.

**Output on success (JSON):**

```json
{
  "closed": "auth-user-model",
  "remaining": {"total": 67, "open": 11, "wip": 0, "done": 56, "ready": 3},
  "has_next": true
}
```

**`has_next`:** `true` if there are ready tasks after this close. `false` means all done or all blocked. Agent checks this to decide whether to continue or stop.

**Output on closure verification failure (exit 1, stderr):**

```json
{
  "error": "closure verification failed: reason does not address criterion 3: \"Factory generates valid instances\"",
  "code": 1,
  "acceptance": "User model at app/Models/User.php. Migration creates users table. Factory generates valid instances.",
  "hint": "Rewrite reason to address all acceptance criteria, then retry tp done."
}
```

No task state changes on error -- task remains WIP.

**Implicit claim:** If the task is `open`, `tp done` claims it (open -> wip) before closing (wip -> done). If the task is already `wip`, closes directly. If `done`, errors.

**Exit codes:** 0 = success, 1 = closure verification failure, 4 = task already done or not found.

#### `tp done --batch <file>`

Batch close from NDJSON file. THE primary close mechanism for the 2-call architecture.

```bash
tp done --batch results.ndjson
```

**NDJSON format** (one task per line):

```ndjson
{"id":"auth-user-model","reason":"Model at app/Models/User.php, 12 fields.","gate_passed":true,"commit":"abc123"}
{"id":"auth-login-flow","reason":"POST /api/login returns JWT.","gate_passed":true,"commit":"def456"}
```

Fields: `id` and `reason` are required. `gate_passed` (boolean, equivalent to `--gate-passed` flag on single `tp done`) and `commit` (string, equivalent to `--commit` flag) are optional.

**Behavior:**
- Tasks in `open` status are implicitly claimed (open -> wip) before closing. This enables the 2-call flow (`tp plan` -> implement -> `tp done --batch`) without a separate claim step.
- Tasks already in `wip` are closed directly.
- Tasks in `done` status are skipped with a warning (idempotent for retries).
- Validates ALL tasks (closure verification on each)
- Closes tasks that pass validation
- Reports failures with acceptance text and hints
- Partial success is allowed (65/67 pass -> 65 closed, 2 reported)

**Output (JSON):**

```json
{
  "closed": 65,
  "failed": 2,
  "failures": [
    {
      "id": "auth-token-refresh",
      "error": "reason does not address criterion 2: \"Refresh token rotated\"",
      "acceptance": "Token refreshed. Refresh token rotated.",
      "hint": "Add evidence about token rotation."
    }
  ],
  "remaining": {"total": 67, "open": 0, "wip": 2, "done": 65, "ready": 0}
}
```

The agent fixes the 2 failures and resubmits: `tp done --batch fixes.ndjson`

**Exit codes:** 0 = all closed, 1 = partial failure (some closed, some failed), 4 = all failed.

#### `tp next`

Incremental fallback command. For interactive use and multi-agent scenarios where the full plan is not practical.

**Behavior priority:**
1. If a WIP task exists -> return it (resume, no new claim)
2. If no WIP -> claim highest-priority ready task -> return it
3. If no ready tasks -> exit 4

Idempotent for crash recovery: calling twice returns the same WIP task.

```bash
tp next                        # get/claim next task with full context
tp next --peek                 # preview without claiming
```

**Output (JSON):**

```json
{
  "task": {
    "id": "auth-user-model",
    "title": "Create User model",
    "description": "Define User model with email, password_hash, timestamps.",
    "status": "wip",
    "tags": ["auth", "model"],
    "acceptance": "Model at app/Models/User.php. Migration runs.",
    "estimate_minutes": 8,
    "depends_on": [],
    "source_sections": ["### 1.1 User Model"],
    "source_lines": "15-42"
  },
  "spec_excerpt": "### 1.1 User Model\n\n...",
  "blocks": ["auth-login-flow", "auth-token-model"],
  "remaining": {"total": 20, "open": 12, "wip": 1, "done": 7},
  "quality_gate": "go test ./... && golangci-lint run"
}
```

**`spec_excerpt`:** Same truncation rules as `tp plan` (see Section 5.3 `tp plan`).

**Everything the agent needs in ONE call:** task details, spec context, what this unblocks, progress, and quality gate command.

**`--peek` flag:** Always shows the next READY task (ignores WIP). Does not claim. Useful for agent planning.

**When no ready tasks:** exit 4 with either:
- `{"done": true, "message": "All tasks complete"}` -- if all done
- `{"done": false, "message": "No ready tasks. 3 tasks blocked.", "blocked": ["task-a","task-b","task-c"]}` -- if blocked

**Exit codes:** 0 = success (task returned), 4 = no ready tasks (all done or all blocked).

#### `tp list`

General task listing with filters.

```bash
tp list                        # all tasks
tp list --status open          # filter by status (comma-separated: open,wip)
tp list --tag auth             # filter by tag
tp list --ids                  # just IDs (newline-separated)
tp list --compact              # minimal fields
```

**Output:** JSON array of task objects (same fields as stored in the task file). Does NOT include computed fields (`blocks`, `is_ready`, `spec_excerpt`) -- use `tp show <id>` for those. With `--compact`, strips description, source_sections, source_lines, tags, closed_reason.

**Exit codes:** 0 = success.

#### `tp lint <spec.md>`

Deterministic quality checks on a spec markdown file. Does not require a task file -- operates directly on the spec. Can be used before any task decomposition.

**Checks performed:**

| Check | Rule | Severity |
|-------|------|----------|
| Heading hierarchy | h2 -> h3 -> h4 (no skips) | error |
| Vague language | Flags: "appropriate", "relevant", "as needed", "etc.", "various", "some", "proper" | warning |
| Section sizing | Any section > 50 lines | warning (split candidate) |
| Empty sections | Heading with no content before next heading | error |
| Duplicate headings | Same heading text appears twice under the same parent | error |
| Orphan references | `[text](#anchor)` where anchor heading does not exist | error |
| Spec size | Total > 500 lines | info (consider modular specs) |

**Output (JSON when not TTY):** `file`, `errors`, `warnings`, `info` counts, and `findings` array. Each finding: `{line, severity, rule, message, context}`.

**Exit codes:** 0 = no errors (warnings OK), 1 = errors found.

#### `tp ready`

List tasks where `status` is `open` and all `depends_on` tasks have `status` = `done`.

```bash
tp ready              # all ready tasks (JSON array, may be empty [])
tp ready --first      # single highest-priority ready task (JSON object, exit 4 if none)
tp ready --count      # just the count (integer)
tp ready --ids        # just IDs (newline-separated strings)
```

**Priority ordering:** Tasks ordered by:
1. Number of dependents (tasks blocked by this one) -- descending
2. Estimate minutes -- ascending (smaller tasks first)
3. Alphabetical by ID (tiebreaker)

This prioritizes tasks that unblock the most downstream work.

**Empty result behavior:**
- `tp ready` -> empty array `[]`, exit 0
- `tp ready --first` -> exit 4 (state error) with message "No ready tasks" (enables agent loop termination: `while tp ready --first; do ... done`)
- `tp ready --count` -> `0`, exit 0

#### `tp show <id>`

Display full task details with spec context.

```bash
tp show auth-user-model
```

**Output includes** all task fields plus computed fields:
- `blocks`: list of task IDs that depend on this one
- `is_ready`: boolean (all deps satisfied and status is open?)
- `spec_excerpt`: the actual spec lines (if `source_lines` is set and spec file exists) -- MUST be populated when spec file exists, capped at 2000 characters
- `quality_gate`: from workflow (so agent does not need to read the task file)

**Exit codes:** 0 = success, 4 = task ID not found.

#### `tp claim <id> [id...]`

Claim one or multiple tasks. Transition open -> wip.

```bash
tp claim auth-user-model                         # single
tp claim auth-user-model auth-token-model        # batch
tp claim --all-ready                             # claim all ready tasks
```

**Batch behavior:** Claims each in order. If any fails (not open, blocked), that one is skipped with a warning and the rest continue. Output lists successes and failures.

**Rules:**
- Task must be `open` (error if `wip` or `done` -- first-writer-wins via flock, second agent gets "already claimed" error)
- All dependencies must be `done` (error if blocked)
- Multiple WIP tasks allowed (multi-agent, different tasks)
- Sets `updated_at` on the root object

**Output (single):** `{"claimed": "auth-user-model"}`

**Output (batch/--all-ready):**
```json
{
  "claimed": ["auth-user-model", "auth-token-model"],
  "failed": [{"id": "auth-login-flow", "error": "blocked by auth-user-model (status: open)", "hint": "Complete auth-user-model first."}]
}
```

**Exit codes:** 0 = all claimed, 1 = partial failure (some claimed, some failed), 4 = all failed.

#### `tp close <id> <reason>`

Low-level close command. Agents should prefer `tp done`. Differences:

| Feature | `tp done` | `tp close` |
|---------|-----------|------------|
| Implicit claim (open -> wip -> done) | Yes | No (must be wip) |
| `--gate-passed` flag | Yes | No |
| `--commit` flag | Yes | No |
| `has_next` in output | Yes | No |
| Batch mode (`--batch`) | Yes | No |

`tp close` exists for scripting and edge cases where explicit status control is needed.

```bash
tp close auth-user-model "Evidence..."
tp close auth-user-model --stdin
tp close auth-user-model --reason-file evidence.txt
```

No batch mode on `tp close`. Use `tp done --batch` for batch operations.

**Reason source precedence:** Exactly one source: positional argument, `--stdin`, or `--reason-file`. Multiple sources = error.

**Closure verification** runs automatically (see Section 8 for full algorithm). Cannot be disabled.

**Rules:**
- Task must be `wip` (error if `open` or `done` -- use `tp done` for implicit claim)
- Reason required, non-empty, passes closure verification (Section 8)
- Sets `closed_at`, `closed_reason`, and `updated_at`

**Output on success:** `{"closed": "auth-user-model"}`

**Output on failure:** Same format as `tp done` error output: `{error, code, acceptance, hint}` on stderr.

**Exit codes:** 0 = success, 1 = closure verification failure, 4 = wrong task status.

#### `tp reopen <id>`

Transition `done` -> `open`. Clears `closed_at`, `closed_reason`, `gate_passed_at`, and `commit_sha`.

```bash
tp reopen auth-user-model
```

**When to use:** A task is reopened when post-completion review finds the work was insufficient -- e.g., a reviewer discovers missing test cases, a downstream task fails because this task's output was incomplete, or acceptance criteria were met superficially but not in substance. Reopening resets the task to `open` so it re-enters the ready queue when deps allow.

**Output:** `{"reopened": "auth-user-model"}`

**Exit codes:** 0 = success, 4 = task not in done status.

#### `tp remove <id>`

Remove a task from the task file.

```bash
tp remove auth-user-model
```

**Rules:**
- Task must be `open` status (error if `wip` or `done` -- reopen first)
- If other tasks depend on this one, `tp remove` lists them and errors: "Cannot remove: 3 tasks depend on auth-user-model (auth-login-flow, auth-token-model, auth-logout). Remove or update those dependencies first, or use --force to remove and clean up references."
- With `--force`: removes the task and strips its ID from all other tasks' `depends_on` arrays, printing a warning for each affected task
- Updates `updated_at` on the root object

**Exit codes:** 0 = success, 1 = has dependents (without --force), 4 = wrong task status.

#### `tp set <id> <field>=<value>`

Update individual task fields.

```bash
tp set auth-user-model estimate_minutes=10
tp set auth-user-model 'tags=["auth","model","v2"]'
tp set auth-user-model 'description=New implementation approach'
```

**Managed fields** (cannot be set via `tp set`):
- `status` -- use `tp claim`, `tp close`, `tp reopen`
- `closed_at` -- set automatically by `tp close` / `tp done`
- `closed_reason` -- set automatically by `tp close` / `tp done`
- `gate_passed_at` -- set automatically by `tp done --gate-passed`
- `commit_sha` -- set automatically by `tp done --commit`

Attempting to set a managed field returns an error explaining which command to use instead.

For nested/array values, use JSON syntax.

**Exit codes:** 0 = success, 2 = managed field rejected, 4 = task not found.

#### `tp validate`

Validate the task file structure, atomicity, coverage, and dependency graph.

```bash
tp validate                    # warnings are non-fatal
tp validate --strict           # atomicity warnings become errors
```

**Validation layers:**

1. **Schema validation** -- Go struct validation (required fields, types, enums)
2. **Atomicity checks** -- 7 rules (see Section 7.1):
   - estimate_minutes in 1-15 range
   - Title <= 8 words
   - No conjunctions in title (and, +, commas)
   - source_sections <= 2
   - Description <= 300 chars (if present)
   - Acceptance <= 3 criteria
   - Acceptance must be non-empty
3. **Coverage verification** -- Parses the spec.md file, extracts all headings, and cross-references:
   - Verifies `total_sections` matches actual heading count in spec
   - Each task's `source_sections` references a real heading in the spec
   - Each spec heading is either in a task's `source_sections`, in `context_only`, or flagged in `unmapped`
   - `unmapped` must be empty
4. **Dependency graph** -- No circular dependencies (DFS cycle detection)
5. **Referential integrity** -- All `depends_on` IDs exist in the task list, no self-dependencies
6. **ID uniqueness** -- No duplicate task IDs

**Output:** JSON with `valid` (bool), `errors`/`warnings`/`atomicity_violations` (int), and `checks` object (schema, atomicity, coverage, cycles, references, uniqueness -- each "pass" or description).

**`valid` semantics:** `true` when there are zero errors (exit 0). Atomicity violations are warnings by default, so `valid` can be `true` with `atomicity_violations > 0`. In `--strict` mode, atomicity violations become errors, making `valid: false` (exit 1).

If the spec file referenced by the `spec` field does not exist, coverage verification is skipped with a warning (not an error). This allows validating task files even when the spec is in a different checkout.

**Exit codes:** 0 = valid, 1 = validation errors found.

#### `tp stats`

Plan statistics and parallelism analysis.

```bash
tp stats
```

**Output:** JSON with `tasks` (total/open/wip/done), `estimates` (min/avg/max/total), `tags` (per-tag count and minutes), `dependencies` (with_deps/independent/cross_tag), `parallelism` (see Section 9.3), and `plan_size_heuristic` (spec_lines, expected_tasks, actual_tasks, assessment).

The `plan_size_heuristic` field is only present when the spec file exists. The `parallelism` object matches the format described in Section 9.3.

**Exit codes:** 0 = success.

#### `tp graph`

Dependency graph visualization.

```bash
tp graph                       # all tasks
tp graph --tag auth            # filter by tag
tp graph --from auth-user-model  # subgraph from a specific task
```

**Interactive output:** ASCII tree showing `id (Nm, status)` with indentation for dependencies. Already-shown nodes marked `[already shown]`.

**JSON output (`--json` or non-TTY):** Adjacency list with `nodes` (id, status, estimate_minutes) and `edges` (from, to).

**Exit codes:** 0 = success.

#### `tp status`

Summary of task counts and progress.

```bash
tp status
```

**Output:** JSON with `total`, `open`, `wip`, `done`, `blocked`, `ready`, `progress_percent`.

`blocked` and `ready` are computed: `blocked` = open tasks with unsatisfied deps, `ready` = open tasks with all deps done. `progress_percent` = `done / total * 100` (rounded).

**Exit codes:** 0 = success.

#### `tp blocked`

List tasks that are `open` but have at least one unsatisfied dependency.

```bash
tp blocked
```

Returns a JSON array of task objects, each with an additional `waiting_on` field listing the unsatisfied dependency IDs and their current statuses.

**Exit codes:** 0 = success.

#### `tp add <task-json>`

Add a single task. The JSON is passed as a command argument or via stdin.

```bash
tp add '{"id":"auth-user-model","title":"Create User model",...}'   # as argument
tp add --stdin                                                       # via stdin
tp add --bulk tasks.ndjson                                           # bulk NDJSON
```

**Behavior:**
- Validates the task against atomicity rules before adding
- Rejects if ID already exists (use `tp set` to update)
- Sets `status` to `open` if not specified
- Updates `updated_at` on the root object

**Output (single):** `{"added": "auth-user-model"}`

**Output (--file bulk):** `{"added": 15, "failed": 0}`

**When no task file exists:** `tp add` and `tp add --bulk` require `--spec <path>` to create a new task file. The tool creates the file with empty coverage and the specified spec path, then adds the task(s). Without `--spec`, these commands error: "No task file found. Use --spec to create one, or run tp init first."

**Exit codes:** 0 = success, 1 = validation error, 3 = no task file (without --spec).

#### `tp init <spec.md>`

Create an empty task file for a spec. This is a convenience command -- not required before using `tp add` or `tp import`.

```bash
tp init docs/typed-inter-machine-communication.md
# Creates: docs/typed-inter-machine-communication.tasks.json
```

**Creates the minimal structure:** version 1, empty workflow, empty coverage (all zeros), empty tasks array.

**Optional flags:**
- `--quality-gate "command"` -- set workflow quality gate
- `--commit-strategy "strategy"` -- set commit strategy

Errors if the task file already exists (prevents accidental overwrite).

**Exit codes:** 0 = success, 3 = file already exists.

#### `tp import <file>`

Import a complete task file. Validates on import, then writes to the spec-adjacent location.

```bash
tp import /tmp/generated-tasks.json
# Validates, then writes to {spec-basename}.tasks.json
```

**Behavior:**
- The imported file must include all root fields (`version`, `spec`, `coverage`, `tasks`)
- Runs full validation (schema + atomicity + coverage + cycles) before writing. Atomicity violations are **always errors** during import (implicit `--strict`) -- a plan with violations should not be created in the first place
- **Target path** is derived from the `spec` field inside the imported JSON: if `spec` is `"docs/foo.md"`, the task file is written to `docs/foo.tasks.json` (spec-adjacent convention)
- Errors if a task file already exists at the target path (use `--force` to overwrite)
- This is the primary command in the AI decomposition workflow: AI generates the JSON, `tp import` validates and places it

**Exit codes:** 0 = success, 1 = validation error, 3 = file already exists (without --force).

---

## 6. Spec Lint Rules (Detail)

`tp lint` operates directly on a markdown file. It does not need a task file and can be used before any decomposition.

### 6.1 Structural Rules (Errors)

**heading-hierarchy**: Headings must not skip levels.

```markdown
## Section       <- h2
#### Subsection  <- ERROR: skipped h3
```

**empty-section**: Every heading must have content.

```markdown
### 3.1 Config
### 3.2 Migration  <- ERROR: 3.1 has no content
```

**duplicate-heading**: Same heading text must not appear twice under the same parent. Identical headings under different parents are allowed (e.g., `## Auth > ### Config` and `## Payment > ### Config` are fine).

```markdown
## Auth
### Setup
### Setup          <- ERROR: duplicate under same parent

## Payment
### Setup          <- OK: different parent
```

**orphan-reference**: Internal links must resolve.

```markdown
See [Config](#configuration)  <- ERROR if no "## Configuration" heading exists
```

### 6.2 Quality Rules (Warnings)

**vague-language**: Flags words that resist testability.

| Flagged Word | Suggestion |
|-------------|------------|
| appropriate | Specify the exact condition |
| relevant | List the specific items |
| as needed | Define when it is needed |
| etc. | List all items explicitly |
| various | Enumerate the specific items |
| some | Specify which ones or how many |
| proper/properly | Define what "proper" means |
| should (without SHALL) | Use EARS format: WHEN X THE SYSTEM SHALL Y |
| might/may (ambiguous) | Use IF X THEN THE SYSTEM SHALL Y |

**section-size**: Sections over 50 lines suggest multiple concerns -- candidate for splitting.

**long-spec**: Specs over 500 lines should be split into modular sub-specs (one per domain/concern).

---

## 7. Validation Rules (Detail)

### 7.1 Atomicity Rules

These rules apply to each task individually. In `--strict` mode, any violation is a blocking error.

| Rule | Check | Threshold | Rationale |
|------|-------|-----------|-----------|
| Estimate bound | `estimate_minutes` | 1-15. 1 = <=1 minute. | METR/SWE-bench data: >15m = exponential success decay |
| Title brevity | word count in `title` | <= 8 words | Long titles = multiple concerns |
| No conjunctions | regex on `title` | No `\band\b`, `,`, `+` | Conjunctions = multiple tasks |
| Section count | `source_sections` length | <= 2 | >2 sections = multiple files/concerns |
| Description scope | `description` length (if present) | <= 300 chars | Long descriptions = broad scope |
| Acceptance count | sentence count in `acceptance` | <= 3 | >3 criteria = multiple concerns |
| Acceptance present | `acceptance` non-empty | Required | Prevents closure without verifiable criteria |

### 7.2 Graph Rules

| Rule | Check | Action |
|------|-------|--------|
| Circular dependencies | DFS traversal | Error with cycle path |
| Dangling references | All `depends_on` IDs exist in tasks | Error with missing ID |
| Self-dependency | `id` not in own `depends_on` | Error |

### 7.3 Coverage Rules

When the spec file (referenced by the `spec` field) exists, `tp validate` parses it and cross-references:

| Rule | Check | Action |
|------|-------|--------|
| Section count match | Actual headings in spec = `total_sections` | Error if mismatch |
| Source sections exist | Each task's `source_sections` entries are real headings in spec | Error with the non-existent heading |
| Full coverage | Every spec heading is in a task's `source_sections` OR in `context_only` | Remaining headings reported as `unmapped` |
| No unmapped sections | `unmapped` is empty | Error |
| Consistent counts | `mapped_sections` <= `total_sections` | Error |
| Coverage arithmetic | `mapped_sections + len(context_only) + len(unmapped) == total_sections` | Error if mismatch |
| Non-zero tasks | `tasks` array is non-empty | Error |

When the spec file does not exist, coverage is checked structurally only (unmapped must be empty, counts consistent) with a warning that spec-level cross-referencing was skipped.

---

## 8. Closure Verification (Detail)

This is the core anti-pattern prevention mechanism. It runs automatically on every `tp close` and `tp done` call, and on each entry in `tp done --batch`. There is no flag to disable it.

### 8.1 Acceptance Criteria Parsing

The `acceptance` field is split into individual criteria:

```
Input:  "Model exists at app/Models/User.php. Migration runs clean. Factory works."
Output: ["Model exists at app/Models/User.php", "Migration runs clean", "Factory works"]
```

Split rules:
- Split on `. ` (period + space)
- Split on `; ` (semicolon + space)
- Filter empty strings
- Trim whitespace

### 8.2 Keyword Extraction

From each criterion, extract significant words:
- Remove stop words: the, a, an, is, are, at, in, with, and, or, for, to, of, be, by, on, it, has, was
- Keep file paths intact (anything containing `/` or `.ext`)
- Keep technical terms (CamelCase, snake_case, UPPER_CASE)
- Result: list of significant keywords per criterion

### 8.3 Matching

Two checks must both pass:

**Check 1 -- Per-criterion keyword match:** For each acceptance criterion, the closure reason must contain at least one significant keyword from that criterion (case-insensitive matching). This ensures no criterion is completely ignored.

**Check 2 -- Minimum reason length:** The closure reason must be at least half the length (in characters) of the acceptance text. This prevents trivially short reasons like "done" or "works."

**Pass example:** acceptance="User model at app/Models/User.php. Migration creates users table. Factory generates valid instances." reason="User model at app/Models/User.php (12 fields). Migration 2024_01_create_users runs. Factory tested with 3 states." -- all 3 criteria have keyword matches, reason length (103) >= acceptance length (95) / 2.

**Fail example:** reason="Model created. Migration works." -- criterion 3 ("Factory generates valid instances") has no keyword match -> ERROR.

### 8.4 Forbidden Patterns

These patterns in closure reasons are always rejected, regardless of keyword matching:

| Pattern | Error Message |
|---------|--------------|
| `deferred` | "Deferral is forbidden. Leave the task open or complete it." |
| `will be done later` | "Deferral is forbidden. Leave the task open or complete it." |
| `covered by existing` (no path with `/`) | "Claim requires evidence. Include file path and line numbers." |
| `not needed` (no `because`) | "Explain why the task is no longer needed." |
| Single-word reason (any word) | "Closure reason must address each acceptance criterion with evidence." |

---

## 9. Parallelism Analysis (Detail)

### 9.1 Level Computation

Tasks are assigned to parallelism levels using iterative fixed-point computation:

```
Level 0: Tasks with no dependencies (can start immediately)
Level 1: Tasks whose deps are all in Level 0
Level 2: Tasks whose deps are all in Level 0 or 1
...
Level N: Tasks whose deps are all in Level 0..N-1
```

Only `open` and `wip` tasks are included. `done` tasks are excluded (already completed).

### 9.2 Metrics

| Metric | Formula | Meaning |
|--------|---------|---------|
| Parallelism levels | Count of distinct levels | Depth of the remaining dependency graph |
| Max parallel tasks | Max tasks in any single level | Peak concurrency opportunity |
| Critical path | Sum of max(estimate) per level | Minimum wall-clock time with infinite agents |
| Total estimate | Sum of all remaining estimates | Wall-clock time with one agent |
| Speedup potential | total_estimate / critical_path | Theoretical max speedup from parallelism |

### 9.3 Output

JSON with `levels`, `max_parallel`, `critical_path_minutes`, `total_estimate_minutes`, `speedup`, and `per_level` array. Each level: `{level, tasks: [ids], bottleneck_minutes}`.

---

## 10. Technology

### 10.1 Language: Go

| Criterion | Go | Alternatives |
|-----------|-----|-------------|
| Startup time | ~45ms | Rust ~30ms, Node ~200ms, Python ~325ms |
| Binary distribution | Single binary, zero deps | Rust similar, Node/Python need runtime |
| Cross-compilation | `GOOS=linux GOARCH=amd64` | Rust: good but complex. Others: N/A |
| JSON handling | `encoding/json` stdlib | All adequate |
| CLI framework | Cobra (industry standard) | Clap (Rust), Commander (Node) |
| Development speed | Fast (simple type system) | Rust slower (borrow checker) |
| Agent overhead (2-call) | 2 x 45ms = 90ms total | vs beads: 134 x 500ms = 67s |

Go is the optimal choice. The ~45ms startup is acceptable (vs beads' ~500ms per operation due to SQLite + daemon overhead). Total overhead for a 67-task plan using the 2-call architecture = ~90ms vs ~67s with beads.

### 10.2 Dependencies

Minimal external dependencies:

| Dependency | Purpose |
|-----------|---------|
| `spf13/cobra` | CLI framework |
| `fatih/color` | Terminal colors, NO_COLOR, TTY detection |
| `gofrs/flock` | Cross-platform file locking |
| `stretchr/testify` | Test assertions (test-only) |
| stdlib only | JSON, file I/O, regexp, sort |

No database drivers. No HTTP clients. No daemon infrastructure.

### 10.3 Binary Size Target

< 5 MB (vs beads 33 MB). Achievable since tp has no SQLite, Dolt, or integration code.

### 10.4 Performance Targets

| Operation | Target | Rationale |
|-----------|--------|-----------|
| `tp plan` | < 100ms | Full plan read + topological sort |
| `tp next` | < 50ms | Ready + claim + show |
| `tp done` (single) | < 50ms | Close + peek next |
| `tp done --batch` (100 tasks) | < 200ms | Validate all + write |
| `tp ready` | < 50ms | Most frequent standalone query |
| `tp show` | < 50ms | Single task lookup |
| `tp claim` / `tp close` | < 50ms | File read + write + validate |
| `tp validate` | < 100ms | Full graph traversal + spec parsing |
| `tp stats` | < 100ms | Parallelism computation |
| `tp lint` (500-line spec) | < 200ms | Line-by-line regex scanning |
| `tp add` (single task) | < 50ms | Append + validate |
| `tp import` (100 tasks) | < 200ms | Full validate + write |

All operations are O(n) or O(n log n) in task count. No operation should exceed 200ms for a 200-task plan.

---

## 11. Skill Design

tp ships with a companion Claude Code skill that provides AI instructions for spec writing, decomposition, and task execution. The skill calls tp commands for deterministic operations and provides semantic guidance the tool cannot.

### 11.1 Skill Activation

The skill activates when:
- A .tasks.json file exists in the project
- A spec markdown file is referenced
- The user asks to "implement the spec/plan/tasks"

### 11.2 Three Workflows

The skill handles three scenarios:

**Workflow A: Decompose** (spec exists, no tasks)

```
1. tp lint <spec.md>
2. [AI reads spec, decomposes into tasks, produces tasks.json]
3. tp import tasks.json
```

**Workflow B: Execute** (tasks exist, work remaining) -- PRIMARY

```
1. plan = tp plan --json
2. for each task in plan.execution_order:
     - Read task.acceptance and task.spec_excerpt from plan
     - Implement the task
     - Run quality gate (plan.workflow.quality_gate)
     - Append to results: {id, reason, gate_passed: true, commit: sha}
3. tp done --batch results.ndjson
```

**Workflow C: Resume** (tasks exist, some done/wip)

Same as Workflow B. `tp plan` automatically excludes done tasks and puts WIP first.

### 11.3 Skill Content (SKILL.md)

The actual skill file should be concise (under 100 lines). It contains:
- Workflow detection logic (which of A/B/C to run)
- The 2-call pattern with exact commands
- results.ndjson format
- Error handling: if `tp done --batch` reports failures, fix reasons and resubmit
- Quality gate: run before recording results, use `--gate-passed`
- Context management: flush every 6-8 tasks for long plans

### 11.4 Execution Loop Detail

The 2-call architecture in practice (see Section 3.2 for rationale):

```
Phase 1: plan = tp plan --json              # ONE call
Phase 2: for task in plan.execution_order:  # ZERO calls
            implement(task)
            run(plan.workflow.quality_gate)
            results.append({id, reason, gate_passed: true, commit: sha})
            if len(results) >= 8: tp done --batch chunk.ndjson  # flush
Phase 3: tp done --batch results.ndjson     # ONE call
Phase 4: if failures: fix reasons, tp done --batch fixes.ndjson
```

Note: `tp done --batch` implicitly claims open tasks, so no separate `tp claim` step is needed.

### 11.5 Spec Writing Protocol

9-step review checklist: (1) LINT: `tp lint spec.md`, (2) COVERAGE: every heading has purpose, (3) AMBIGUITY: semantic review beyond lint, (4) CONTRADICTION: cross-section consistency, (5) TESTABILITY: every requirement verifiable, (6) SIZING: no section >50 lines, total <500, (7) DEPENDENCIES: implicit deps made explicit, (8) EDGE CASES: errors, empty states, concurrency, rollback, (9) COMPLETENESS: all entities, endpoints, events, migrations listed.

### 11.6 Decomposition Protocol

**Pass 1 (Coverage):** Map every spec section to a task or context_only. Goal: 100% coverage.

**Pass 2 (Atomicity):** For each task: single-commit test, verb-object title, <=2 files (excl. tests), <=15 min estimate, <=3 acceptance criteria. If any fails -> split.

**Self-check:** Write the commit message for each task. If you cannot -> split. Run `tp validate --strict`. Fix all violations.

### 11.7 Task Closure Protocol

Before recording a result: (1) re-read acceptance criteria from plan, (2) verify implementation matches FULL spec not just "enough for CI", (3) count items if spec specifies N, (4) write reason addressing each criterion with file paths, (5) never use "deferred", "covered by existing" (without proof), or single-word reasons.

### 11.8 Research Backing

The 2-call architecture is supported by:
- **Plan-and-Act** (Wang et al., 2025): planning improved web agent success from 9.85% to 57.58%
- **Anthropic**: "one feature at a time" is critical for long-running agents
- **METR**: <15 min tasks achieve 70%+ success
- **SWE-agent** (Princeton, 2024): ACI design gave 3-4x improvement
- **Manus**: 100:1 input-to-output token ratio means every output token matters
- **ReWOO** (Xu et al., 2023): planning all tool calls upfront gave 64% token reduction with +4.4% accuracy

---

## 12. File Discovery

### 12.1 Auto-Detection

When no `--file` flag is given, `tp` searches for task files:

1. Current directory: `*.tasks.json`
2. If exactly one found -> use it
3. If multiple found -> error: "Multiple task files found. Use --file to specify."
4. If none found and command requires a task file -> error: "No task file found. Run `tp init <spec.md>` or `tp import <file>` to create one."

Commands that do NOT require a task file: `tp lint`, `tp init`, `tp --help`, `tp --version`.

### 12.2 Spec File Resolution

When `tp validate` needs the spec for coverage cross-referencing:
1. Read `spec` field from the task file
2. Resolve relative to the task file's directory
3. If not found -> skip spec-level coverage verification with a warning

`tp lint` does not use the task file at all -- the spec path is passed as a direct argument.

---

## 13. Error Handling

### 13.1 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Validation error (structure, atomicity in strict mode, cycles, closure verification) |
| 2 | Usage error (wrong arguments, missing required flags, managed field rejection) |
| 3 | File error (file not found, permission denied, invalid JSON) |
| 4 | State error (wrong status transition, blocked task, no ready tasks) |

### 13.2 Error Output

Errors are written to stderr. In JSON mode, errors are also structured:

```json
{"error": "task auth-login has unsatisfied dependency: auth-user-model (status: open)", "code": 4}
```

**Actionable errors:** Every error message tells the agent exactly what to do next. Error messages include the failing entity, the reason for failure, and the command or action needed to resolve it. This eliminates the need for the agent to make follow-up diagnostic calls, saving context tokens. Error JSON includes a `hint` field with the recovery action.

Example:

```json
{
  "error": "cannot claim auth-login: blocked by auth-user-model (status: open)",
  "code": 4,
  "hint": "Complete auth-user-model first, then retry."
}
```

### 13.3 Concurrent Access

File-level locking with `flock` for write operations (`claim`, `close`, `reopen`, `add`, `set`, `remove`, `import`, `next`, `done`, `done --batch`). Read operations (`ready`, `show`, `status`, `blocked`, `graph`, `stats`, `validate`, `list`, `plan`) are lock-free.

This prevents corruption when multiple agent processes work on independent tasks from the same task file (the multi-agent scenario enabled by parallelism analysis).

---

## 14. Non-Goals

These are explicitly out of scope for v1:

| Feature | Reason |
|---------|--------|
| Web UI / TUI | Agent-first. CLI + JSON output is sufficient. |
| Database backend | Single JSON file is sufficient for <1000 tasks. |
| Daemon / background process | Zero infrastructure principle. |
| Integration with Jira/Linear/GitHub | Use git + JSON. External integration is a separate concern. |
| Spec authoring / generation | AI's job via skill instructions. Tool does validation only. |
| Task decomposition AI | AI's job via skill instructions. Tool does deterministic checks only. |
| Real-time notifications | Agents poll with `tp ready`. No push needed. |
| Plugin system | Premature abstraction. Add if needed in v2. |
| Spec evolution / delta tracking | v2 candidate. For v1, regenerate tasks when spec changes. |
| Cross-organization collaboration | Use git branches + PRs. tp is single-repo scoped. |

---

## 15. Distribution

### 15.1 Installation

```bash
# macOS (Homebrew)
brew install deligoez/tap/tp

# Go install
go install github.com/deligoez/tp@latest

# Binary download
curl -sSL https://github.com/deligoez/tp/releases/latest/download/tp-$(uname -s)-$(uname -m) -o /usr/local/bin/tp
chmod +x /usr/local/bin/tp
```

### 15.2 Release

GoReleaser for cross-platform builds:
- macOS (arm64, amd64)
- Linux (arm64, amd64)
- Windows (amd64)

### 15.3 Claude Code Skill

The companion skill is distributed as a Claude Code plugin (see Section 11): `.claude-plugin/plugin.json` + `skills/tp/SKILL.md`.

---

## 16. Testing Strategy

### 16.1 Unit Tests

Go `testing` package. Table-driven tests for:
- Struct validation (valid and invalid task files)
- Atomicity rule checks (each rule independently, boundary values)
- Cycle detection (acyclic, 2-node cycle, 3-node cycle, complex DAG)
- Parallelism level computation (linear chain, wide graph, diamond pattern)
- Closure verification (keyword matching, forbidden patterns, length check)
- Spec lint rules (each rule independently with positive and negative cases)
- Status transitions (all valid and invalid transitions)
- Managed field protection (`tp set` rejects status/closed_at/closed_reason/gate_passed_at/commit_sha)
- `tp plan` output (topological ordering, WIP-first, done exclusion, spec_excerpt truncation)
- `tp done --batch` validation (partial success, all-fail, all-pass)
- `--compact` field stripping (verify omitted and kept fields)

### 16.2 Integration Tests

Full CLI tests using Go's `exec.Command`:
- 2-call workflow: `tp plan` -> implement -> `tp done --batch` (primary workflow)
- `tp init` -> `tp add` -> `tp validate` -> `tp next` -> `tp done` workflow (incremental)
- `tp next` WIP resume (crash recovery: call twice, get same task)
- Batch claim (`tp claim id1 id2 id3`) with partial failures
- NDJSON batch close (`tp done --batch`) with partial failures
- Error cases: invalid JSON, missing fields, cycle injection, blocked claim, closure rejection
- File discovery and auto-detection (single file, multiple files, no files)
- JSON output mode verification (TTY vs non-TTY)
- `--compact` output mode (verify fields are stripped)
- Concurrent access with flock (two processes writing simultaneously)
- `tp import` with `--force` overwrite behavior
- `tp remove` with dependency cleanup
- `tp plan --from` resume behavior
- `tp plan --level` multi-agent distribution

### 16.3 Golden File Tests

For complex outputs (`stats`, `graph`, `lint`, `plan`), compare against known-good output files. Enables regression detection for formatting changes.

### 16.4 Performance Benchmarks

Go `testing.B` benchmarks for:
- `tp plan` with 10, 50, 100, 500 tasks
- `tp done --batch` with 10, 50, 100, 500 tasks
- `tp next` with 10, 50, 100, 500 tasks
- `tp done` (single) with 10, 50, 100, 500 tasks
- `tp ready` with 10, 50, 100, 500 tasks
- `tp validate` with complex dependency graphs
- `tp stats` parallelism computation
- JSON parse/write roundtrip
- Closure verification keyword matching

---

## Appendix A: Research References

| Finding | Source |
|---------|--------|
| Task duration 2x -> failure rate 4x (exponential decay) | Toby Ord, "Half-Life for AI Agent Success Rates", May 2025 |
| <15 min tasks = 70%+ success | SWE-bench leaderboard |
| >50 min tasks = ~23% success | SWE-bench Pro, Scale Labs 2025 |
| Context rot: 99% -> 50% accuracy | Chroma Research |
| ACI design: 3-4x improvement without model change | SWE-agent, Princeton, 2024 |
| Agent-optimized tools: 70% -> 95% accuracy | Anthropic, 2025 |
| Plan-and-Act: 9.85% -> 57.58% with planning | Wang et al., 2025 |
| Dynamic replanning: +10.31% over static | Plan-and-Act |
| One feature at a time is critical | Anthropic long-running agents |
| 100:1 input-to-output token ratio | Manus |
| ReWOO: 64% token reduction + 4.4% accuracy | Xu et al., 2023 |
| 5-8 step reliable planning horizon | Synthesis: METR, DeepPlanning |
| Agents with todo lists: 71-76% SWE-bench | Warp, Verdent |
| Flat multi-agent: 17.2x error amplification | Google DeepMind |
| Accuracy below 50% within 15 turns | arxiv 2509.09677 |
| 50% time horizon doubling ~4 months | METR, 2026 |
| Performance cliff at ~35 minutes | Zylos Research, January 2026 |
| Quality is #1 barrier to agent deployment (32%) | LangChain State of Agent Engineering, 2025 |
| Agents frequently produce partially correct solutions | FeatureBench, February 2025 |
| Self-organized dependencies -> circular dependencies | QuantumBlack/McKinsey, February 2026 |
| Spec size limit: ~5K tokens, 8-15 tasks per spec | Compound Product / Ralph ecosystem |
| EARS format: 5 requirement patterns (WHEN/WHILE/IF/WHERE -> SHALL) | Alistair Mavin, Rolls-Royce, 2009 |
