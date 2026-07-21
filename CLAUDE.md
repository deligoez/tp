# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Install

```bash
brew tap deligoez/tap && brew install tp   # Homebrew
go install github.com/deligoez/tp/cmd/tp@latest   # or Go
```
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

# Stripped binary (production, <10MB)
go build -ldflags="-s -w" -o tp ./cmd/tp
```

## Commands

### Primary (2-call architecture)
```bash
tp plan                        # Full execution plan in one call (THE primary command)
tp plan --minimal              # Minimal: id + acceptance only (~80% fewer tokens)
tp commit <id> [reason]        # Stage + structured commit + record SHA (--files for selective)
tp done <id> <reason>          # Close task with implicit claim + verification
tp done id1 id2 id3 "reason"   # Multi-ID close (shared reason, last arg = reason)
tp done <id> --auto-commit     # Commit + close in one call
tp done <id> --covered-by <id> # Close as covered by another done task
tp done --batch results.ndjson # Batch close from NDJSON (primary close mechanism)
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready task
tp next --minimal              # Minimal output: {id, acceptance} only
tp next --peek                 # Preview without claiming
```

### Task State
```bash
tp claim <id> [id...]          # open -> wip (batch: multiple IDs, --all-ready)
tp close <id> <reason>         # wip -> done (low-level, prefer tp done)
tp reopen <id>                 # done -> open (clears gate_passed_at, commit_sha)
tp remove <id>                 # Remove task (--force for dep cleanup)
tp set <id> field=value        # Update field (managed fields protected)
tp set --workflow field=value  # Update workflow-level fields (convergence params)
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
tp review <spec.md> --perspective code-audit --affected-files <paths>  # Code audit with source files
tp review <spec.md> --round N --final-round --affected-files <paths>  # Final round: mandatory code read-through
tp review --merge r1.ndjson r2.ndjson -o merged.ndjson  # Merge + dedup findings from NDJSON files
tp review --resolve findings.ndjson <idx> <disposition> "evidence"  # Mark finding fixed/wontfix/duplicate
tp review --resolve-all findings.ndjson <disposition> "evidence"  # Mark all unresolved findings
tp review --verify <spec.md> --findings all.ndjson  # Lightweight verification prompt (verifier role)
tp review --report r1.ndjson r2.ndjson  # Cross-round convergence report
tp review <spec.md> --diff-from <old-spec.md>  # Diff-based review (changed sections only)
tp review <spec.md> --spec-inline             # Embed full spec inline (default: reference mode)
tp review --resolve ... --force              # Force re-resolve already resolved findings
tp audit <spec.md>              # Post-implementation: verify code matches spec (auto-detects changed files via git diff)
tp audit <spec.md> --affected-files <paths>  # Manual file selection (comma-separated or repeated)
tp audit <spec.md> --findings <file.ndjson>  # Also verify review findings were addressed
tp validate                    # Task file validation (--strict)
```

### Data
```bash
tp init <spec.md>              # Create empty task file
tp add <json>                  # Add task (--stdin, --bulk for NDJSON bulk)
tp import <file>               # Import + validate (--force to overwrite, auto-fills coverage)
tp import <file> --spec <spec> # Import bare JSON array (auto-wraps into TaskFile)
tp use <file>                  # Set active task file (writes .tp/local.json, git-ignored)
tp use --clear                 # Clear the active pointer
tp use                         # Show current active file
```

### Project Config (v0.24.0)
```bash
tp config                      # Effective (resolved) config as JSON
tp config --resolved           # Annotate each setting with {value, source} layer
tp config --extract            # Hoist policy shared by ALL task files into .tp/config.json (--dry-run/--force)
tp set --workflow --project <field>=<value>  # Edit a project-level workflow default
tp set --local defaults.<flag>=<bool>        # Set a CLI flag default (compact/quiet/no_color)
tp validate --project          # Cross-spec workflow drift (informational; --strict → exit 1)
```

### Global Flags
```
--file <path>    Explicit task file
--json           Force JSON output
--compact        Minimal JSON (~40% smaller); --no-compact forces full
--quiet          Suppress info messages; --no-quiet forces info output
--no-color       Disable colors; --color forces color
```

### Task File Discovery
Priority: `--file` flag > `TP_FILE` env var > `.tp/local.json` active pointer > auto-detect. The legacy `.tp-active` marker was removed in v0.25.0.
Auto-detect scans current dir, then one level of subdirectories for `*.tasks.json`.
The active pointer lives in `.tp/local.json` (git-ignored); set it with `tp use <file>`. The `.tp/` dir is found by walking up to the `.git` boundary.

### JSON Field Aliases
- `deps` accepted as alias for `depends_on` in task JSON (import, add)
- `estimation_minutes` accepted as alias for `estimate_minutes`
- `acceptance` accepts string or `["item1", "item2"]` (array joined with `\n- `)

## Project Structure

```
cmd/tp/              Main entry point
internal/
  cli/               Cobra commands (plan, done, next, list, claim, close, commit, report, ...)
                     review.go          — core review + mode routing + prompt generators
                     review_merge.go    — --merge mode (dedup, sort, output)
                     review_resolve.go  — --resolve/--resolve-all mode (flock, in-place update)
                     review_verify.go   — --verify mode (lightweight verification prompt)
                     review_report.go   — --report mode (convergence analysis, TTY/JSON output)
                     audit.go           — tp audit (post-implementation spec verification)
  engine/            Core logic (toposort, closure, validate, lint, parallel, discover, lock, excerpt, linecoverage, structured)
                     diff.go            — section-level spec diff (for --diff-from)
                     fileio.go          — shared file I/O, budget-aware reading, affected summary
                     suggest.go         — task ID suggestion for covered_by did-you-mean hints
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
3. **Init + workflow**: `tp init spec/<version>.md --quality-gate "go test ./... && golangci-lint run"`, then `tp set --workflow` for convergence counts / round budgets / `checks` (before the review loop, so the loop reads them)
4. **Review loop**: `tp review spec/<version>.md` → spawn sub-agents → `tp review --merge` → `tp review spec/<version>.md --record merged.ndjson` → resolve findings → repeat until `tp review spec/<version>.md --status --check` exits 0
5. **Decompose into tasks** with `source_sections` for every task (`source_lines` optional precision)
6. **Import**: `tp import <tasks.json>` (plain — the init shell holds zero tasks; convergence checks stay armed)
7. **Validate**: `tp validate` — check coverage gaps
8. **Implement each task**, then:
   - `tp commit <id> "evidence"` — atomic structured commit
   - `tp done <id> "evidence" --commit <sha>` — the quality gate runs automatically
   - Or: `tp done <id> "evidence" --auto-commit`
9. **Audit loop**: `tp audit spec/<version>.md` → spawn sub-agents → `tp audit spec/<version>.md --record results.ndjson` → fix code → repeat until `tp audit spec/<version>.md --status --check` exits 0
8. **Report**: Last `tp done` auto-includes report summary. Or: `tp report` for full details
9. **Release**: tag, push, `gh release edit` with notes

### Rules
- Every task MUST have `source_sections` (canonical headings); `source_lines` is optional precision (§13.4). A task with neither anchor is a validation error.
- The quality gate runs automatically at `tp done`/`tp close`; `--skip-gate "why"` and — on budget exhaustion — raising `review_max_rounds`/`audit_max_rounds` or `tp import --force` are **user-approved decisions, never the agent's own**.
- Commit the `.tp-review/` state directory to version control (import enforcement + CI depend on it).
- Every table row and numbered list item in spec must appear in a task's acceptance
- Run backward pass: `tp validate` line coverage + structured element check
- Use `--covered-by` when a task is satisfied by another task's work
- Quality gate after every task: `go test ./... && golangci-lint run`
- One task = one commit = one `tp commit` call
- **Dogfood the in-progress binary** — during self-development, always run tp against its OWN repo with a freshly built binary (`go build -o /tmp/tp-dev/tp ./cmd/tp`), never the PATH-installed release (it lags and hides new behavior). Rebuild after every implementing commit, and once a task adds a new command/flag, immediately exercise that new capability on tp's own spec/task files to surface bugs the unit tests miss (real dogfooding). When a version's new feature can manage tp's own workflow (e.g. a new config file), adopt it for the remaining development of that same version. This applies to the **review and audit loops too**: run every `tp review` / `tp audit` (including `--record` / `--status`) through the current, to-be-released binary so each round dogfoods the exact code being shipped — never a lagging PATH release. Once a version's own feature can drive review/audit (e.g. user-defined reviewer roles), the remaining rounds of that version use it. Before implementation starts (spec + review phase) the current version is simply the latest release — no behavioral difference — but still route every tp call (`lint` / `init` / `review`) through the freshly-built binary.
- **English everywhere in committed artifacts** — commit messages, `tp commit`/`tp done` closure reasons (they land in `commit_sha` bodies and `closed_reason`), code comments, and docs are ALWAYS in English, regardless of the conversation language. Author notes/thinking may be in any language, but nothing committed to the repo may be. If a Turkish (or other non-English) message slips into history, fix it with `hc rewrite` (commit messages) or by editing the artifact (closure reasons, docs) before release.
- Task-closing commits always go through `tp commit` (records `commit_sha` on the task) — never raw `git commit`
- Every other commit (spec progression, docs, tooling, changes outside a task) goes through the `hc` skill (hunk-based atomic commits) — never raw `git commit`
- `source_sections` entries should use canonical form `"## Heading Text"` (with `##` prefix and space). v0.22.0+ accepts plain text and auto-normalizes, but prefer canonical form for clarity in committed `.tasks.json` files.
- **Update `skills/tp/SKILL.md` before every release** — new commands, flags, lint rules, and workflow changes MUST be reflected in the skill file before creating the release tag
- **Pre-release checklist** — before running `gh release create`, verify:
  1. `skills/tp/SKILL.md` reflects all new commands, flags, lint rules, and workflow changes
  2. `CLAUDE.md` Self-Development Rules reflect any new conventions or process changes
  3. `README.md` reflects all new commands, flags, and features
  4. All three files are committed and included in the release tag
  5. **Dogfood any migration the release introduces on tp's own repo.** When a version adds a config/layout that makes an existing file or duplicated data redundant, migrate tp's own repo as part of the release: adopt the new mechanism, delete the now-redundant files, and commit the result (so tp ships already using its own new feature). For v0.24.0: write `.tp/config.json` with the shared workflow policy, remove the duplicated `workflow` blocks from every `spec/*.tasks.json`, migrate the active pointer with `tp use` (writes `.tp/local.json`, git-ignored), and delete the deprecated `.tp-active`; commit `.tp/config.json` + `.tp/.gitignore` + the thinned task files.
- **Post-release commands** — after `gh release create`, run these two commands (in order):
  ```bash
  go install github.com/deligoez/tp/cmd/tp@v<VERSION>
  npx skills update -g deligoez/tp
  ```
  Use the exact version tag (e.g., `v0.17.0`), not `@latest` — the Go module proxy and skill registry may not update immediately.

### Continuous Improvement
- After each implementation cycle, note friction points and AX issues
- If a tp command is awkward to use during self-development, fix it immediately
- If a workflow step is error-prone, add tooling or guidance to prevent it
- Agent feedback from other projects is high-priority — real-world usage reveals blind spots
- Every improvement should be evaluated: does this reduce token overhead or agent friction?

### Deferred Ideas (evaluate when agent feedback warrants)
- **Full audit NDJSON parser** (`tp audit --merge`): schema validation + dedup of audit rows. v0.23.0 `--record` only counts non-PASS rows; the full parser is deferred.
- **Broken cross-reference lint**: detect `§3.2 step 10` when section 3.2 has only 9 steps. High false positive risk — needs careful format detection. Revisit if agents report wasted review rounds on broken cross-refs.
- **Duplicate paragraph lint**: detect two consecutive identical paragraphs (blank-line separated). Currently `duplicate-line` catches line-level duplicates; paragraph-level needs paragraph boundary detection. Revisit if line-level check proves insufficient.
- ✅ **Project-level workflow config** (`.tp/config.json`) — **shipped in v0.24.0.** Repo-root `.tp/config.json` holds workflow **defaults** (committed); each `<base>.tasks.json` `workflow` block holds only explicit **overrides**; effective values **resolve at read time** (CLI > env > task override > project config > built-in). `.tp/local.json` (git-ignored) holds the `active` pointer + CLI flag `defaults`. Commands: `tp config [--resolved|--extract]`, `tp set --workflow --project`, `tp set --local`, `tp validate --project`. See README / SKILL.md / REFERENCE.md "Project Configuration".

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
- Workflow fields (`tp set --workflow`): `review_clean_rounds`/`audit_clean_rounds` (default 2, 1-10), `gate_timeout_seconds` (default 600, 30-3600), `review_max_rounds`/`audit_max_rounds` (default 0=no cap, 0-50), `checks` (array of `{class, cmd}`, replace semantics). `quality_gate` stays read-only (author at `tp init --quality-gate`). These fields also have project-level **defaults** in `.tp/config.json` (v0.24.0); a task file's `workflow` block holds only explicit overrides and effective values resolve at read time (`tp config --resolved`). `tp set --workflow --project` edits the project defaults; `tp set --local defaults.<flag>=<bool>` sets CLI flag defaults (`compact`/`quiet`/`no_color`).
- Managed fields also reject `gate_skipped_reason`. `tp reopen` clears `gate_passed_at`, `gate_skipped_reason`, and `commit_sha`.
- Pretty-printed JSON with 2-space indentation
- spec_excerpt capped at 2000 chars
- source_lines supports multi-range: "4-10,15-20,25-30" and auto-normalizes single numbers ("72" → "72-72")
- `tp lint` reports structured elements, duplicate consecutive lines, and section numbering gaps
- `tp validate` checks line coverage (source_lines vs spec content lines)
- `tp done --batch` auto-toposorts entries by in-batch dependencies
- `tp import` accepts bare JSON arrays with `--spec` flag
- Acceptance criteria delimiters: `. ` (period+space), `; ` (semicolon+space), `\n- ` (bullet list)

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
