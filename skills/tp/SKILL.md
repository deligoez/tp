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

**tp does NOT generate tasks — you are the decomposer. tp validates your output.**

1. `tp lint <spec.md>` — fix structural issues, review `structured_elements`
2. `tp review <spec.md>` — adversarial review loop (see **Review Workflow** below)
3. Read spec, decompose into tasks (JSON) — see **Decomposition Rules** below
4. **Backward pass** — verify coverage:
   - For each table in `structured_elements`: does every row map to a task's acceptance?
   - For each numbered list: does every item have a task or explicit acceptance entry?
   - If spec has multiple checklists, take the union — gaps = potential missing task
   - Run `tp validate` — check `line_coverage` for uncovered spec line gaps
5. `tp import tasks.json` — validates and stores (auto-fills coverage)
   - If import rejects tasks (atomicity warnings), split by concern axis per Decomposition Rules
6. If `tp validate` reports line coverage gaps, inspect uncovered ranges and add tasks

### Review Workflow

**Round 1 — full spec read:**
```bash
tp review spec.md --json > r1-prompts.json
# Spawn 3 sub-agents (implementer, tester, architect) with each prompt
# Collect findings → r1-implementer.ndjson, r1-tester.ndjson, r1-architect.ndjson
tp review --merge r1-*.ndjson -o r1-merged.ndjson
```

**Round 2+ — use `--diff-from` to avoid re-reading the entire spec:**
```bash
# Save spec snapshot before edits: cp spec.md spec-r1.md
# Fix spec based on round 1 findings, then:
tp review spec.md --round 2 --findings r1-merged.ndjson --diff-from spec-r1.md --json
# This injects ONLY changed sections — ~80-90% fewer tokens than full re-read
```

**Convergence**: Stop when no new high/critical severity findings. Typically 2-3 rounds.

**Final round** — force mandatory code read-through:
```bash
tp review spec.md --round N --final-round --affected-files src/a.go src/b.go
```

**Code-aware review** (optional):
- `--affected-files src/a.go src/b.vue` — inject source files into prompts
- `--perspective code-audit --affected-files src/a.go` — C1-C5 checklist
- Default review is **spec-only** — do NOT check implementation code unless `--affected-files` or `--perspective code-audit` is used

**Findings lifecycle:**
```bash
tp review --resolve r1.ndjson 3 fixed "evidence"     # Mark individual finding
tp review --resolve-all r1.ndjson wontfix "reason"    # Mark all unresolved
tp review --verify spec.md --findings all.ndjson      # Lightweight verification
tp review --report r1.ndjson r2.ndjson                # Convergence report
```

**File management**: You manage findings files yourself. Convention:
```
spec/
  feature.md                    # spec (keep)
  feature.tasks.json            # task file (keep)
  feature-r0.md                 # snapshot before round 1 edits (for --diff-from)
  feature-r1-merged.ndjson      # round 1 merged findings
  feature-r2-merged.ndjson      # round 2 merged findings
```

**Cleanup after review converges**: Delete review artifacts (snapshots `*-r0.md`, `*-r1.md`, etc. and findings `*.ndjson`). Keep the spec `.md` and task file `.tasks.json`.

### Decomposition Rules

**You are the decomposer — tp validates your output, it does not generate tasks.**

Follow these rules when breaking a spec into tasks:

1. **Atomicity**: Each task = one commit, one verb, 1-15 minutes estimated
   - Max 3 acceptance criteria per task
   - Max 8 words in title, no conjunctions (and/,/+)
   - Max 2 source_sections per task
   - If a task has >3 acceptance criteria, split by concern axis
2. **Concern axes** for splitting oversized tasks:
   - **Types/Models**: struct definitions, type aliases, interfaces
   - **Logic/Engine**: core business logic, algorithms
   - **Validation**: input validation, error handling, forbidden patterns
   - **CLI/Wiring**: cobra commands, flag parsing, output formatting
   - **Tests**: unit tests, integration tests, test helpers
   - **Docs**: README, SKILL.md, CLAUDE.md updates
3. **Structured elements** (from `tp lint`): every table data row, numbered list item, and code block in the spec must appear in some task's acceptance criteria
4. **Source lines**: every task MUST have `source_lines` as a range: `"15-42"` or `"15-42,50-60"`. Single numbers like `"72"` are **auto-normalized** to `"72-72"`.
5. **Dependencies**: model dependency order — types before logic, logic before CLI, CLI before tests
6. **Preview before import**: list your proposed tasks (id, title, acceptance count) and ask for confirmation before writing the JSON file. This prevents wasted import/fix cycles.

**Example split** — if a spec section has 10 requirements covering model + API + validation + tests:
```
scaffold-types     (3 criteria): struct defs, field types, JSON tags
scaffold-logic     (3 criteria): core functions, error returns
scaffold-validate  (2 criteria): input checks, edge cases
scaffold-tests     (2 criteria): unit tests for logic + validation
```

### B: Execute (tasks exist) — PRIMARY

The 2-call architecture minimizes token overhead:

```
# Phase 1: Get full plan (ONE call)
plan=$(tp plan --minimal --json)  # minimal: id + acceptance only (~80% fewer tokens)

# Phase 2: Implement each task
# IMPORTANT: Always note the current time before starting each task.
# This enables accurate duration tracking in tp report.
#
# For each task in plan.execution_order:
#   1. Note current time (started_at)
#   2. Read task.acceptance
#   3. Implement the task
#   4. Run plan.workflow.quality_gate
#   5. tp commit <id> "evidence"           # structured commit, records SHA
#   6. tp done <id> "evidence" --gate-passed --commit <sha>
#
# Alternative: commit + close in one call per task
#   tp done <id> "evidence" --gate-passed --auto-commit
#
# Alternative: batch close (include started_at for accurate timing)
#   {"id":"x","reason":"y","gate_passed":true,"started_at":"<iso8601>","commit":"<sha>"}
#   tp done --batch results.ndjson
```

If batch reports failures: fix reasons, resubmit `tp done --batch fixes.ndjson`.

When the last task is closed, `tp done` automatically includes a `report` summary in its output with total estimated vs actual minutes, estimation accuracy, and fastest/slowest task.

### C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first.

## Acceptance Criteria Format

Acceptance criteria support three delimiters:

| Delimiter | Example |
|-----------|---------|
| Period + space | `"Model exists. Migration runs. Tests pass."` |
| Semicolon + space | `"Model exists; migration runs; tests pass"` |
| Bullet list | `"- Model exists\n- Migration runs\n- Tests pass"` |
| JSON array | `["Model exists", "Migration runs", "Tests pass"]` |

All delimiters are equivalent — tp parses them into individual criteria for closure verification and atomicity checking. JSON array is joined with `\n- ` on import.

**Max 3 criteria per task.** If exceeded, `tp validate` warns with a split hint:
```
task X: acceptance has 6 criteria (max 3); hint: split into ~2 tasks by concern
```

## JSON Field Aliases

- `deps` is accepted as an alias for `depends_on` in task JSON (import, add)
- `estimation_minutes` is accepted as an alias for `estimate_minutes`
- `acceptance` can be a string or `["item1", "item2"]` (array joined with `\n- `)

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
6. Use `--covered-by <task-id>` when a task is satisfied by work in another done task (not a deferral — work IS done, just in a different task). Batch: `"covered_by":"other-task-id"`
   - If the referenced ID is not found, tp suggests similar IDs ("did you mean: X, Y?")

**Important:** `tp done` auto-claims open tasks — no need for a separate `tp claim` call.

**Code snippets:** When spec contains inline code, validate against the actual codebase (types, casts, method signatures) before implementing. Spec code may be illustrative, not literal.

## Task File Discovery

Priority: `--file` flag > `TP_FILE` env var > `.tp-active` marker > auto-detect (current dir, then one level of subdirs).

Set active task file persistently:
```bash
tp use spec/project.tasks.json  # writes .tp-active in CWD
tp use --clear                  # remove .tp-active
tp use                          # show current active file
```

Or set `TP_FILE` for session-level override:
```bash
export TP_FILE=spec/project.tasks.json
```

## Incremental Fallback

For interactive use or when full plan is impractical:
```
tp next              # get/resume WIP task
tp next --minimal    # minimal output: {id, acceptance} only
tp next --peek       # preview next without claiming
# implement
tp done <id> "reason" --gate-passed
tp done id1 id2 id3 "shared evidence" --gate-passed  # multi-ID close
```

## Key Commands

### Primary Workflow
| Command | Purpose |
|---------|---------|
| `tp plan` | Full execution plan (THE primary command) |
| `tp plan --minimal` | Minimal plan: id + acceptance (~80% fewer tokens) |
| `tp plan --compact` | Stripped plan: no description, source_lines, tags (~40% fewer) |
| `tp plan --from <id>` | Start plan from a specific task onward |
| `tp plan --level 0,1` | Filter by parallelism levels (multi-agent) |
| `tp commit <id> [reason]` | Stage + structured commit + record SHA |
| `tp commit <id> --files "*.go"` | Selective file staging |
| `tp done <id> "reason"` | Single close with implicit claim + verification |
| `tp done <id> --gate-passed` | Relax keyword matching (agent attests gate passed) |
| `tp done <id> --auto-commit` | Commit + close in one call |
| `tp done <id> --auto-commit --files "*.go"` | Selective staging + commit + close |
| `tp done <id> --covered-by <id>` | Close as covered by another done task |
| `tp done <id> --commit <sha>` | Record implementing commit SHA |
| `tp done id1 id2 "reason"` | Multi-ID close (shared reason) |
| `tp done --batch file.ndjson` | Batch close from NDJSON |

### Incremental
| Command | Purpose |
|---------|---------|
| `tp next` | Resume WIP or claim next ready |
| `tp next --minimal` | Minimal output: {id, acceptance} only |
| `tp next --peek` | Preview without claiming |

### Task State
| Command | Purpose |
|---------|---------|
| `tp claim <id> [id...]` | open -> wip (batch: multiple IDs) |
| `tp claim --all-ready` | Claim all ready tasks at once |
| `tp close <id> <reason>` | wip -> done (low-level, prefer tp done) |
| `tp reopen <id>` | done -> open (clears timestamps + SHA) |
| `tp remove <id>` | Remove task (--force cleans deps) |
| `tp set <id> field=value` | Update field (managed fields protected) |
| `tp set --bulk sets.ndjson` | Bulk update from NDJSON {id, field, value} |

### Query
| Command | Purpose |
|---------|---------|
| `tp list` | All tasks (--status, --tag, --ids, --compact) |
| `tp ready` | Deps-satisfied tasks (--first, --count, --ids) |
| `tp show <id>` | Full details + spec_excerpt + blocks |
| `tp status` | Progress summary (open/wip/done counts) |
| `tp report` | Per-task duration + estimation accuracy |
| `tp blocked` | Tasks waiting on unsatisfied deps |
| `tp graph` | Dependency tree (--tag, --from) |
| `tp stats` | Parallelism analysis |

### Spec & Validation
| Command | Purpose |
|---------|---------|
| `tp lint spec.md` | Spec quality + structured elements + duplicate lines + numbering gaps |
| `tp review spec.md` | Adversarial review prompts (3 personas) |
| `tp review spec.md --perspective code-audit --affected-files src/a.go` | Code audit with source file injection |
| `tp review spec.md --round N --findings file.ndjson` | Multi-round with previous findings exclusion |
| `tp review spec.md --round N --final-round --affected-files src/a.go` | Final round with mandatory code read-through |
| `tp review --merge r1.ndjson r2.ndjson -o merged.ndjson` | Merge + dedup findings from NDJSON files |
| `tp review --resolve findings.ndjson <idx> <disposition> "evidence"` | Mark finding fixed/wontfix/duplicate |
| `tp review --resolve-all findings.ndjson <disposition> "reason"` | Mark all unresolved findings |
| `tp review --resolve ... --force` | Force re-resolve already resolved findings |
| `tp review --verify spec.md --findings all.ndjson` | Lightweight verification (verifier role) |
| `tp review --report r1.ndjson r2.ndjson` | Cross-round convergence report |
| `tp review spec.md --diff-from old-spec.md` | Diff-based review (changed sections only) |
| `tp review spec.md --spec-inline` | Embed full spec inline (default is reference mode) |
| `tp audit spec.md` | Post-implementation audit: verify code matches spec |
| `tp audit spec.md --affected-files src/a.go` | Manual file selection (comma or repeated) |
| `tp audit spec.md --findings review.ndjson` | Also verify review findings were addressed |
| `tp validate` | Task file validation + line coverage + atomicity |
| `tp validate --strict` | Atomicity warnings become errors |

### Data
| Command | Purpose |
|---------|---------|
| `tp init spec.md` | Create empty task file |
| `tp add <json>` | Add task (--stdin for piped input) |
| `tp add --bulk tasks.ndjson` | Bulk add from NDJSON |
| `tp import file.json` | Import + validate (--force to overwrite + relax atomicity) |
| `tp import tasks.json --spec spec.md` | Import bare JSON array (auto-wraps into TaskFile) |
| `tp use <file>` | Set active task file (.tp-active) |
| `tp use --clear` | Remove .tp-active marker |
| `tp use` | Show current active file |

### Global Flags
| Flag | Purpose |
|------|---------|
| `--file <path>` | Explicit task file path |
| `--json` | Force JSON output (default when piped) |
| `--compact` | Minimal JSON (~40% smaller) |
| `--quiet` | Suppress info messages |
| `--no-color` | Disable colored output |

## Audit Workflow

`tp audit` generates prompts — **you spawn the sub-agents and collect results**, just like review.

```bash
# 1. Generate audit prompts (auto-detects changed files via git diff)
tp audit spec.md --json > audit-prompts.json

# 2. Or specify files manually
tp audit spec.md --affected-files src/engine.go,src/cli.go --json > audit-prompts.json

# 3. Also verify review findings were addressed
tp audit spec.md --findings review-findings.ndjson --json > audit-prompts.json

# 4. Spawn sub-agents with each prompt, collect results
# (same pattern as tp review — tp gives you prompts, you run them)
```

**tp does NOT run sub-agents.** The agent (you) spawns sub-agents using the Agent tool. This is by design — tp is a deterministic tool, not an orchestrator.

## Phase Management

Use **tags** to organize tasks into phases. No special `phase` field needed:

```json
{"id": "auth-model", "tags": ["phase-1"], ...}
{"id": "auth-api", "tags": ["phase-2"], ...}
```

Then scope commands with `--tag`:
```bash
tp list --tag phase-1           # Only phase 1 tasks
tp plan --tag phase-1           # Plan for phase 1 only (if supported)
tp ready --tag phase-1          # Ready tasks in phase 1
tp graph --tag phase-1          # Dependency tree for phase 1
```

## Progress & Estimation

Don't look for separate progress/estimate/scope commands — they already exist:

| Need | Use |
|------|-----|
| How many tasks done/open/wip? | `tp status` |
| Per-task timing and accuracy? | `tp report` |
| Critical path and parallelism? | `tp stats` |
| Which spec lines lack tasks? | `tp validate` (line_coverage) |
| What's blocking progress? | `tp blocked` |
| Full dependency tree? | `tp graph` |

## Batch Close — Dependency Order

`tp done --batch` **automatically toposorts** entries by in-batch dependencies before processing. You no longer need to manually order your NDJSON file — tp handles dependency chains, `covered_by` references, and already-done tasks:

```ndjson
{"id":"tests","reason":"All tests pass","gate_passed":true}
{"id":"model","reason":"Model created","gate_passed":true}
{"id":"api","reason":"API endpoint works","gate_passed":true}
```

Even though `tests` depends on `model` and `api`, tp will reorder and close `model` → `api` → `tests`.

Output includes `reordered` (bool) and `skipped` (count of already-done entries):
```json
{"closed": 3, "failed": 0, "skipped": 0, "reordered": true, ...}
```

## tp commit

`tp commit` uses **plain `git commit`** — it does NOT require any external tool (like `ac`). It stages files, generates a conventional commit message with task metadata, and records the SHA:

```bash
tp commit <id> "evidence"           # Stage all + commit
tp commit <id> --files "*.go"       # Stage selectively + commit
```
