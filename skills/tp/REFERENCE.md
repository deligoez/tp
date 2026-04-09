# tp — Reference

Detailed command reference, field formats, and operational details. For workflows and rules, see [SKILL.md](SKILL.md).

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
| `tp set --workflow field=value` | Update workflow-level fields (review_clean_rounds, audit_clean_rounds) |
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
| `tp lint spec.md` | Spec quality + structured elements + duplicate lines + numbering gaps + orphan list items |
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

## Global Flags

| Flag | Purpose |
|------|---------|
| `--file <path>` | Explicit task file path |
| `--json` | Force JSON output (default when piped) |
| `--compact` | Minimal JSON (~40% smaller) |
| `--quiet` | Suppress info messages |
| `--no-color` | Disable colored output |

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

## Phase Management

Use **tags** to organize tasks into phases. No special `phase` field needed:

```json
{"id": "auth-model", "tags": ["phase-1"], ...}
{"id": "auth-api", "tags": ["phase-2"], ...}
```

Then scope commands with `--tag`:
```bash
tp list --tag phase-1           # Only phase 1 tasks
tp ready --tag phase-1          # Ready tasks in phase 1
tp graph --tag phase-1          # Dependency tree for phase 1
```

## Batch Close Details

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

## Review File Management

You manage findings files yourself. Convention:
```
spec/
  feature.md                    # spec (keep)
  feature.tasks.json            # task file (keep)
  feature-r0.md                 # snapshot before round 1 edits (for --diff-from)
  feature-r1-merged.ndjson      # round 1 merged findings
  feature-r2-merged.ndjson      # round 2 merged findings
```

**Cleanup after review converges**: Delete review artifacts (snapshots `*-r0.md`, `*-r1.md`, etc. and findings `*.ndjson`). Keep the spec `.md` and task file `.tasks.json`.
