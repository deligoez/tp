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

1. `tp lint <spec.md>` — fix structural issues, review `structured_elements`
2. `tp review <spec.md>` — adversarial review loop:
   - Round 1: `tp review <spec.md>` generates 3 targeted prompts (implementer, tester, architect)
   - Spawn sub-agents with each prompt via the Agent tool (can be parallel)
   - Collect NDJSON findings, fix spec
   - Round 2+: `tp review <spec.md> --round 2 --findings <findings.ndjson>`
   - tp auto-injects previous findings summary into prompts, excludes already-reported issues
   - Combine findings files across rounds for `--findings`
   - Converge within 2 rounds (stop when no new high-severity)
   - **Code-aware review** (optional, for catching state-dependent behaviors):
     - `tp review spec.md --affected-files src/a.go src/b.vue` — inject source files into prompts
     - `tp review spec.md --perspective code-audit --affected-files src/a.go` — code audit perspective with C1-C5 checklist
     - `tp review spec.md --round 2 --final-round --affected-files src/a.go` — force mandatory code read-through
   - **Multi-round review lifecycle** (merge, resolve, verify, report):
     - `tp review --merge r1-*.ndjson -o r1.ndjson` — merge + dedup findings from multiple sub-agents
     - `tp review --resolve r1.ndjson 3 fixed "evidence"` — mark finding as fixed/wontfix/duplicate
     - `tp review --resolve-all r1.ndjson wontfix "reason"` — mark all unresolved findings
     - `tp review --verify spec.md --findings all.ndjson` — lightweight verification prompt (verifier role)
     - `tp review --report r1.ndjson r2.ndjson` — cross-round convergence report
     - `tp review spec.md --diff-from spec-r0.md` — diff-based review (changed sections only)
     - `tp review spec.md --spec-inline` — embed full spec inline (default is reference mode)
     - `--force` — force re-resolve already resolved findings
3. Read spec, decompose into tasks (JSON) — see **Decomposition Rules** below
4. **Backward pass** — verify coverage:
   - For each table in `structured_elements`: does every row map to a task's acceptance?
   - For each numbered list: does every item have a task or explicit acceptance entry?
   - If spec has multiple checklists, take the union — gaps = potential missing task
   - Run `tp validate` — check `line_coverage` for uncovered spec line gaps
5. `tp import tasks.json` — validates and stores (auto-fills coverage)
6. If `tp validate` reports line coverage gaps, inspect uncovered ranges and add tasks

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
4. **Source lines**: every task MUST have `source_lines` mapping to spec line ranges (e.g., `"15-42"` or `"15-42,50-60"`)
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
| `tp lint spec.md` | Spec quality + structured elements |
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
