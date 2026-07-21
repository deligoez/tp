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
| `tp done <id> "reason"` | Single close with implicit claim + verification; runs the quality gate |
| `tp done <id> --skip-gate "why"` | Skip gate execution, record `gate_skipped_reason` (needs user approval) |
| `tp done <id> --gate-passed` | Gate-less projects only: record an attestation; ignored when a gate is set |
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
| `tp set --workflow field=value` | Update workflow fields: `review_clean_rounds`/`audit_clean_rounds` (1-10), `gate_timeout_seconds` (30-3600), `review_max_rounds`/`audit_max_rounds` (0-50, 0=no cap) |
| `tp set --workflow checks='[{"class":"s","cmd":"c"}]'` | Replace the mechanical-checks list (JSON array; `class` kebab-case unique, `cmd` non-empty) |
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
| `tp review spec.md --diff-from old-spec.md` | Diff-based review; overrides the snapshot baseline, forces the block at any round |
| `tp review spec.md --spec-inline` | Embed full spec inline (default is reference mode) |
| `tp review spec.md --record merged.ndjson` | Record a review round; auto-numbers R, freezes count + clean flag |
| `tp review spec.md --status` | Show recorded rounds, `consecutive_clean`, `converged`, `stale`, `mechanical_checks` |
| `tp review spec.md --status --check` | Run registered checks; exit 0 only when converged AND every check passes |
| `tp review spec.md --perspective regression` | Standalone regression pass (needs state R≥2, or `--diff-from` + `--findings`) |
| `tp review spec.md --no-state` | Disable all state reads/writes; restores pre-0.23.0 manual `--round` numbering |
| `tp audit spec.md` | Post-implementation audit: verify code matches spec |
| `tp audit spec.md --affected-files src/a.go` | Manual file selection (comma or repeated) |
| `tp audit spec.md --findings review.ndjson` | Also verify review findings were addressed (route to spec-coverage) |
| `tp audit spec.md --record results.ndjson` | Record an audit round (non-PASS rows = findings); independent sequence |
| `tp audit spec.md --status` | Show recorded audit rounds, `consecutive_clean`, `converged`, `stale` |
| `tp audit spec.md --status --check` | Exit 0 only when the audit is converged |
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
| `--compact` / `--no-compact` | Minimal JSON (~40% smaller) / force full output |
| `--quiet` / `--no-quiet` | Suppress info messages / force info output |
| `--no-color` / `--color` | Disable / force colored output |

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
{"id":"task-id","reason":"- criterion 1 evidence\n- criterion 2 evidence","started_at":"2026-04-01T13:00:00Z","commit":"abc123"}
```

- `id` and `reason`: required. For N ≥ 2 acceptance criteria, `reason` must contain ≥ N lines each starting with `- ` (the `\n` in the string is literal).
- `skip_gate`: optional string; when non-empty, that entry closes with `gate_skipped_reason` recorded and does not require the gate to pass (needs user approval). Present-but-empty fails the entry.
- `started_at`: ISO 8601 timestamp when you began the task (optional, enables `tp report`).
- `commit`: git commit SHA (optional).
- `gate_passed`: gate-less projects only; the gate now runs automatically once per batch invocation.

The batch gate runs once before any entry is processed, iff at least one surviving entry does not carry `skip_gate`. On gate failure, `skip_gate` entries still close and every other entry fails.

## Task File Discovery

Priority: `--file` flag > `TP_FILE` env var > `.tp/local.json` active pointer > legacy `.tp-active` marker (deprecated; removed in v0.25.0) > auto-detect (current dir, then one level of subdirs).

Set active task file persistently:
```bash
tp use spec/project.tasks.json  # writes the active pointer to .tp/local.json (git-ignored)
tp use --clear                  # clear the active pointer
tp use                          # show current active file (reports dangling_active if the target is gone)
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

## Workflow Fields (v0.23.0)

| Field | Type | Default | Range | `tp set --workflow` |
|-------|------|---------|-------|---------------------|
| `quality_gate` | string | `""` | — | read-only (author at `tp init --quality-gate`) |
| `gate_timeout_seconds` | int | 600 | 30-3600 | settable |
| `checks` | array of `{class, cmd}` | `[]` | — | settable (replace semantics) |
| `review_clean_rounds` | int | 2 | 1-10 | settable |
| `audit_clean_rounds` | int | 2 | 1-10 | settable |
| `review_max_rounds` | int | 0 | 0-50 | settable (0 = no cap) |
| `audit_max_rounds` | int | 0 | 0-50 | settable (0 = no cap) |

Out-of-range `tp set --workflow` writes are rejected with exit 1. Out-of-range values in a hand-edited task file fall back at read time (`gate_timeout_seconds`→600, caps→0) and `tp validate` warns.

These are the **defaults** a project-level `.tp/config.json` supplies and a task file's `workflow` block overrides — see [Project Configuration](#project-configuration-v0240). `tp set --workflow --project <field>=<value>` writes to the project config instead of the task file.

## Project Configuration (v0.24.0)

A repo-root `.tp/` directory holds workflow policy shared across every spec, so multi-spec repos keep one source of truth instead of copying policy into each `<base>.tasks.json`.

| File | Tracked? | Holds |
|------|----------|-------|
| `.tp/config.json` | commit to VCS | workflow **defaults** (same fields as the table above) |
| `.tp/local.json` | git-ignored (auto) | `active` task-file pointer + CLI flag `defaults` |
| `.tp/.gitignore` | commit (auto-written) | ignores `local.json`, tracks `config.json` |

**Discovery**: walk up from the CWD to the first `.git` boundary; the `.tp/` there is the project config (single deterministic anchor).

**Resolution (resolve-at-read)** — effective value per field, highest layer wins:
```
CLI flag  >  environment  >  task-file workflow override  >  .tp/config.json  >  built-in default
```
A field in a task file's `workflow` block counts as an override only when present (absent ≠ zero). `checks` uses replace semantics (the winning layer's array wins whole).

| Command | Purpose |
|---------|---------|
| `tp config` | effective configuration as JSON |
| `tp config --resolved` | annotate each setting with `{value, source}` (source ∈ override/project/local/default/…) |
| `tp config --extract` | hoist policy shared by ALL task files into `.tp/config.json` |
| `tp config --extract --dry-run` | print the hoist plan without writing |
| `tp config --extract --force` | merge into an existing `.tp/config.json` |
| `tp set --workflow --project <f>=<v>` | edit a project-level workflow field (flock, range-validated) |
| `tp set --local defaults.<flag>=<bool>` | set a CLI flag default (`compact`/`quiet`/`no_color`) |
| `tp validate --project` | report cross-spec workflow drift (informational; `--strict` → exit 1) |

**Negating flags** override a `defaults` entry for a single run: `--no-compact`, `--no-quiet`, `--color`. Precedence for `no_color`: `--color`/`--no-color` > `NO_COLOR` env > `defaults.no_color` > TTY detection.

Malformed `.tp/config.json` or `.tp/local.json` aborts with exit 3 and a repair hint; unknown keys and out-of-range values warn (to stderr) and fall back.

## State Directory (`.tp-review/`)

tp owns the review/audit round lifecycle in `<spec-dir>/.tp-review/<spec-base>/`:

| File | Content |
|------|---------|
| `state.json` | Round index: `{spec, review_rounds: [...], audit_rounds: [...]}` |
| `snapshot-round-<N>.md` | Byte copy of the spec at round N prompt generation |
| `review-round-<N>.ndjson` | Recorded review round N findings |
| `audit-round-<N>.ndjson` | Recorded audit round N results |

Each round entry is `{round, findings, clean, recorded_at, file, spec_hash}`. `spec_hash` is `sha256:<hex>` of the spec bytes at record time and powers the staleness rule. **Commit this directory to version control** — import convergence enforcement and CI both depend on the recorded rounds traveling with the repo. Only snapshots older than the newest are prunable. A corrupt or index-less directory aborts state-reading commands with exit 3 and a `repair or delete <path>` hint; tp never silently rebuilds the index.

## Spec Frontmatter (`tp:` mapping)

A spec whose first line is `---` may carry a YAML frontmatter block. tp reads only the `tp:` mapping (every other top-level key is ignored) and excludes the whole block from every spec parser while preserving absolute line numbers.

```yaml
---
tp:
  domain: prose        # free string; default "software"; only "software" enables software-specific prompts
  lens:
    all: ["question appended to every role + regression"]
    implementer: ["question appended to the implementer role only"]
    tester: []
    architect: []
---
```

`tp lint --json` reports a `frontmatter` object: `{present, lines, domain, lens_roles}` (`lines` as `"1-K"`; `lens_roles` = non-empty lens keys in order implementer, tester, architect, all). Malformed YAML is a lint error; unknown lens keys, non-list values, and non-string elements are lint warnings and are ignored by review. An unterminated block is treated as content with a lint error.

## Finding `class` and Report

Review finding NDJSON gains an optional `class` (kebab-case slug naming a mechanically checkable pattern). `tp review --merge` preserves it (first non-empty wins on dedup); `tp review --report` adds a `by_class` breakdown and `mechanize_candidates` (a class in ≥ 2 distinct rounds OR ≥ 5 times in one round), sorted by total descending, ties alphabetical.

## Audit JSON Schema (v0.23.0 — clean break from v0.22.0)

`tp audit` emits one prompt per non-empty role. There is no `--legacy-format` flag; downstream consumers MUST update.

| Field | v0.22.0 | v0.23.0 |
|-------|---------|---------|
| `prompts[].role` | always `"implementation-auditor"` | `spec-coverage` \| `security` \| `maintainability-conventions` |
| `prompts[].category` | always `null` | REMOVED |
| `prompts[].prompt` | paragraph text | structured: Role → Role Rules → Spec Excerpt → Project Context → JSON-array Checklist → Affected Files → Output Schema |
| `prompts[].checklist_items` | absent | `[]ChecklistItem` (`item_id`, `type`, `spec_line`, `section`, `text`, `expected_evidence`) |
| `prompts[].affected_files` | absent | `[]{path, tasks, diff_summary}` |

Item ids are deterministic: `table-<t>-<r>`, `list-<l>-<n>`, `task-<id>`, `file-sec-<n>`/`file-maint-<n>`, `finding-<n>`. Sub-agents return one NDJSON row per checklist item: `{item_id, status(PASS|PARTIAL|FAIL), evidence_file, evidence_lines, category, severity, notes, class?}`. `category`/`severity` are `null` for PASS and one of the enum values for PARTIAL/FAIL. Finding category enum: `security > concurrency > error-handling > correctness > contract` (resolution precedence when several apply).
