# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Key Concept: AX (Agent Experience)

AX = Agent Experience (analogous to DX = Developer Experience).
Every design decision in tp optimizes for the AI agent as the primary user:
- Minimize round-trips (2-call architecture: `tp plan` + `tp done --batch`)
- Minimize output tokens (concise JSON, `--compact` flag strips ~40%)
- Clear actionable errors with `hint` field ("cannot claim X: blocked by Y")
- Deterministic behavior (no ambiguity, no prompts)
- Fast startup (<50ms per call)

**Always evaluate changes through the AX lens: does this reduce token overhead for the agent?**

## Four Foundational Principles

| # | Principle | Definition |
|---|-----------|------------|
| P1 | AX First | Every decision optimizes for the AI agent |
| P2 | Batch Parity | What's easy for 1 task must be equally easy for N tasks |
| P3 | Minimal Tokens | Every output byte costs agent context |
| P4 | Agent Plans, Tool Executes | Agent produces decisions, tool deterministically executes |

## Quick Reference

```bash
# Build
go build ./cmd/tp

# Test
go test ./...

# Lint
golangci-lint run

# Quality gate (run after every task)
go test ./... && golangci-lint run

# Stripped binary (production, <5MB)
go build -ldflags="-s -w" -o tp ./cmd/tp
```

## Commands

### Primary (2-call architecture)
```bash
tp plan                        # Full execution plan in one call (THE primary command)
tp plan --minimal              # Minimal: id + acceptance only (~80% fewer tokens)
tp commit <id> [reason]        # Stage + structured commit + record SHA (--files for selective)
tp done <id> <reason>          # Close task with implicit claim + verification
tp done <id> --auto-commit     # Commit + close in one call
tp done <id> --covered-by <id> # Close as covered by another done task
tp done --batch results.ndjson # Batch close from NDJSON (primary close mechanism)
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready task
tp next --peek                 # Preview without claiming
```

### Task State
```bash
tp claim <id> [id...]          # open -> wip (batch: multiple IDs, --all-ready)
tp close <id> <reason>         # wip -> done (low-level, prefer tp done)
tp reopen <id>                 # done -> open (clears gate_passed_at, commit_sha)
tp remove <id>                 # Remove task (--force for dep cleanup)
tp set <id> field=value        # Update field (managed fields protected)
tp set --bulk sets.ndjson      # Bulk update from NDJSON {id, field, value}
```

### Query
```bash
tp list                        # All tasks (--status, --tag, --ids, --compact)
tp ready                       # Deps-satisfied tasks (--first, --count, --ids)
tp show <id>                   # Full details + spec_excerpt + blocks
tp status                      # Progress summary
tp report                      # Per-task duration and estimation accuracy
tp blocked                     # Tasks waiting on deps
tp graph                       # Dependency tree (--tag, --from)
tp stats                       # Parallelism analysis
```

### Spec & Validation
```bash
tp lint <spec.md>              # Spec quality checks (no task file needed)
tp review <spec.md>            # Adversarial review prompts (implementer, tester, architect)
tp validate                    # Task file validation (--strict)
```

### Data
```bash
tp init <spec.md>              # Create empty task file
tp add <json>                  # Add task (--stdin, --bulk for NDJSON bulk)
tp import <file>               # Import + validate (--force to overwrite, auto-fills coverage)
```

### Global Flags
```
--file <path>    Explicit task file
--json           Force JSON output
--compact        Minimal JSON (~40% smaller)
--quiet          Suppress info messages
--no-color       Disable colors
```

### Task File Discovery
Priority: `--file` flag > `TP_FILE` env var > auto-detect.
Auto-detect scans current dir, then one level of subdirectories for `*.tasks.json`.

### JSON Field Aliases
- `deps` accepted as alias for `depends_on` in task JSON (import, add)

## Project Structure

```
cmd/tp/              Main entry point
internal/
  cli/               Cobra commands (plan, done, next, list, claim, close, commit, report, ...)
  engine/            Core logic (toposort, closure, validate, lint, parallel, discover, lock, excerpt, linecoverage, structured)
  model/             Data types (TaskFile, Task, Workflow, Coverage)
  output/            Formatting (JSON/TTY, compact, colors, hint errors)
spec/
  0.1.0.md           Original specification (1431 lines)
  <version>.md       New feature specs — one file per version/feature
skills/tp/
  SKILL.md           Claude Code skill (workflows, decomposition rules, commit format)
.claude-plugin/
  marketplace.json   Skill distribution manifest
```

## Self-Development: tp Uses tp

**tp develops itself using its own workflow.** When implementing new features:

1. **Write a spec** in `spec/<version>.md` describing the feature
2. **Lint the spec**: `tp lint spec/<version>.md`
3. **Review the spec**: `tp review spec/<version>.md` → spawn sub-agents, fix findings
4. **Decompose into tasks** with `source_lines` for every task
4. **Import**: `tp import <tasks.json>`
5. **Validate**: `tp validate` — check line coverage gaps
6. **Implement each task**, then:
   - `tp commit <id> "evidence"` — atomic structured commit
   - `tp done <id> "evidence" --gate-passed --commit <sha>`
   - Or: `tp done <id> "evidence" --gate-passed --auto-commit`
7. **Report**: Last `tp done` auto-includes report summary. Or: `tp report` for full details
8. **Release**: tag, push, `gh release edit` with notes

### Rules
- Every task MUST have `source_lines` mapping to spec lines
- Every table row and numbered list item in spec must appear in a task's acceptance
- Run backward pass: `tp validate` line coverage + structured element check
- Use `--covered-by` when a task is satisfied by another task's work
- Quality gate after every task: `go test ./... && golangci-lint run`
- One task = one commit = one `tp commit` call
- Always use `tp commit` for committing — never raw `git commit`

### Continuous Improvement
- After each implementation cycle, note friction points and AX issues
- If a tp command is awkward to use during self-development, fix it immediately
- If a workflow step is error-prone, add tooling or guidance to prevent it
- Agent feedback from other projects is high-priority — real-world usage reveals blind spots
- Every improvement should be evaluated: does this reduce token overhead or agent friction?

## Tech Stack

- **Language:** Go
- **CLI:** spf13/cobra
- **Colors:** fatih/color (NO_COLOR + TTY detection built-in)
- **File locking:** gofrs/flock
- **Testing:** stretchr/testify
- **JSON:** encoding/json (stdlib)
- **Validation:** Manual struct validation (no JSON Schema library)

## Conventions

- Exit codes: 0=success, 1=validation, 2=usage, 3=file, 4=state
- JSON output when piped or `--json`, colored text in TTY
- `--compact` omits: description, source_sections, source_lines, tags, closed_reason, spec_excerpt
- All write operations use flock; reads are lock-free
- Task status: open -> wip -> done (3 states only, blocked computed from deps)
- Managed fields (tp set rejects): status, started_at, closed_at, closed_reason, gate_passed_at, commit_sha
- Pretty-printed JSON with 2-space indentation
- spec_excerpt capped at 2000 chars
- source_lines supports multi-range: "4-10,15-20,25-30"
- `tp lint` reports structured elements (tables, numbered lists, code blocks)
- `tp validate` checks line coverage (source_lines vs spec content lines)

## Manual QA Testing

When running manual QA, use this setup to avoid wasting time:

```bash
# 1. Build binary to temp dir
mkdir -p /tmp/tp-qa/project
go build -ldflags="-s -w" -o /tmp/tp-qa/tp ./cmd/tp
export TP=/tmp/tp-qa/tp
cd /tmp/tp-qa/project

# 2. Create a spec with multiple headings (for lint, coverage, excerpt tests)
cat > spec.md << 'SPEC'
# Todo App
## 1. Models
### 1.1 Task Model
Create a Task model with title, status, due_date fields.
### 1.2 User Model
Create a User model with email and password fields.
## 2. API
### 2.1 Create Task
POST /tasks endpoint that creates a new task.
### 2.2 List Tasks
GET /tasks endpoint that returns all tasks.
## 3. Testing
### 3.1 Unit Tests
Unit tests for Task and User models.
SPEC

# 3. Init + add tasks with dependency chain
$TP init spec.md
$TP add '{"id":"task-model","title":"Create Task model","estimate_minutes":5,"acceptance":"Task model exists with migration.","source_sections":["### 1.1 Task Model"],"source_lines":"4-4","depends_on":[]}'
$TP add '{"id":"user-model","title":"Create User model","estimate_minutes":5,"acceptance":"User model exists.","source_sections":["### 1.2 User Model"],"depends_on":[]}'
$TP add '{"id":"create-api","title":"Create task endpoint","estimate_minutes":8,"acceptance":"POST /tasks creates task.","source_sections":["### 2.1 Create Task"],"depends_on":["task-model"]}'
$TP add '{"id":"list-api","title":"List tasks endpoint","estimate_minutes":5,"acceptance":"GET /tasks returns all.","source_sections":["### 2.2 List Tasks"],"depends_on":["task-model"]}'
$TP add '{"id":"tests","title":"Write unit tests","estimate_minutes":8,"acceptance":"Task and User model tests pass.","source_sections":["### 3.1 Unit Tests"],"depends_on":["task-model","user-model"]}'

# Setup ready — 5 tasks, 2 ready (task-model, user-model), 3 blocked
```

### QA Test Checklist

**All output is JSON when piped. Use `| python3 -c "import sys,json; ..."` to parse.**

**Known gotcha:** `tp add` succeeds silently (no stdout output). Check exit code.

| Area | Commands to test | What to verify |
|------|-----------------|----------------|
| **Basics** | `--version`, `--help`, `lint spec.md` | Version shows, help lists all commands, lint runs |
| **Status/Query** | `status`, `ready`, `ready --first`, `blocked`, `show <id>`, `list`, `list --status open`, `list --tag`, `list --ids`, `list --compact` | Correct counts, correct filtering, compact strips fields |
| **Plan** | `plan`, `plan --compact`, `plan --from <id>`, `plan --level 0` | Topo order, WIP first, excerpt present, compact strips excerpt |
| **Next** | `next` (claim), `next` again (WIP resume), `next --peek` | Same task returned twice, peek doesn't claim |
| **Done single** | `done <id> "reason"`, `done <id> "reason" --gate-passed --commit sha` | has_next correct, gate_passed_at/commit_sha set |
| **Done batch** | Write NDJSON, `done --batch file` | closed/failed counts, partial failure works |
| **Claim batch** | `claim id1 id2`, `claim --all-ready` | claimed array, failures with hint |
| **Close (low-level)** | `close <id> "reason"` on open task (should fail with hint) | Error mentions "use tp done" |
| **Reopen** | `reopen <id>`, then `show` | status=open, closed_at/gate_passed_at/commit_sha all null |
| **Remove** | `remove <id>` with dependents (should fail), `remove --force` | Force cleans deps to `[]` not null |
| **Set** | `set <id> estimate_minutes=3`, `set <id> status=done` (should fail) | Field updated, managed field rejected with hint |
| **Validate** | `validate`, `validate --strict` (add task with estimate>15) | Strict makes atomicity violations into errors |
| **Graph** | `graph`, `graph --tag`, `graph --from` | JSON adjacency list when piped |
| **Error cases** | Done already-done, done with "deferred", done single word, claim blocked | Correct exit codes (1/2/4), actionable hints |
| **Nil slices** | `show` on task with no dependents, `ready` when all done, `blocked` when none blocked | `[]` not `null` in JSON |
| **Import** | Create full task file JSON, `import file.json` | Status shows imported tasks |
| **Excerpt** | Add task with source_lines, check `plan` output | spec_excerpt contains correct lines |

### Common nil-slice pattern to watch for

Any `var x []T` that reaches `output.JSON()` will serialize as `null` when empty. Always use `x := make([]T, 0)` for JSON-output slices. Grep: `grep -rn 'var .* \[\]' internal/ --include='*.go' | grep -v _test.go`
