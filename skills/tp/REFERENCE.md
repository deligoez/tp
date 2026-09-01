# tp — Reference

Detailed command reference, field formats, and operational details. For workflows and rules, see [SKILL.md](SKILL.md).

## Command forms

The exact form of every command and flag is the inventory in [SKILL.md](SKILL.md) — the document an
agent already has loaded mid-cycle. This document covers what those forms produce: fields, exit
codes and schemas.

`-o`/`--output` belongs to `--merge` alone. On any other mode both `tp review` and `tp audit`
reject it with exit 2 rather than accepting it and writing nothing there (v0.32.0); the payload
goes to stdout, so redirect it.

## Advisories an agent can see (v0.32.0)

tp writes two kinds of message to stderr. **Progress narration** travels `output.Info`, which is
silent whenever stdout is not a terminal — an agent driving tp never sees it, and loses nothing,
because the JSON payload already carries the same facts. **Advisories that say an instruction was
ignored or content was dropped** travel `output.Notice`: still stderr, still suppressed by
`--quiet`, but **not** suppressed in JSON mode. That covers `matches no active role`, a git probe
that failed, an unreadable spec excerpt or `CLAUDE.md`, a file list truncated to the cap, a missing
prior round, and the malformed rows `--record` skipped.

The distinction matters most for `enabled: false`: a mistyped role id would otherwise be
indistinguishable from an applied deactivation, and the run would silently pay the sub-agent round
the flag exists to save. Parse stdout as JSON and read stderr separately — the two are never mixed.

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

- `id` and `reason`: required. For N ≥ 2 acceptance criteria, `reason` must contain ≥ N lines each starting with `- ` (the `\n` in the string is literal). An optional trailing line beginning `Out of scope:` is accepted, preserved verbatim in `closed_reason`, and surfaced by `tp report` — the home for a problem found outside the task's acceptance (the scope fence forbids fixing it; record it instead).
- `skip_gate`: optional string; when non-empty, that entry closes with `gate_skipped_reason` recorded and does not require the gate to pass (needs user approval). Present-but-empty fails the entry.
- `started_at`: ISO 8601 timestamp when you began the task (optional, enables `tp report`).
- `commit`: git commit SHA (optional).
- `gate_passed`: gate-less projects only; the gate now runs automatically once per batch invocation.

The batch gate runs once before any entry is processed, iff at least one surviving entry does not carry `skip_gate`. On gate failure, `skip_gate` entries still close and every other entry fails.

## Task File Discovery

Priority: `--file` flag > `TP_FILE` env var > `.tp/local.json` active pointer > auto-detect (current dir, then one level of subdirs). The legacy `.tp-active` marker was removed in v0.25.0.

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

## Reset-Native Workflow (v0.28.0)

### `tp resume [spec]` — resume oracle (read-only)

Resolves the task file by the discovery order (a spec argument wins and uses its adjacent `<base>.tasks.json`; an absent adjacent file → empty task set); exits 3 when neither a task file nor a spec argument is found. Emits JSON when piped. Output:

| Field | Meaning |
|-------|---------|
| `phase` | `review` / `decompose` / `implement` / `audit` / `release` (task-first: an open task is `implement` even when the spec is stale) |
| `spec` | resolved spec path |
| `changes` | byte-sorted repo-root-relative uncommitted paths **not** on the keep-list |
| `kept` | byte-sorted `{path, reason}` — uncommitted paths matched by the keep-list |
| `bookkeeping` | `[]` of `{path, kind, ref}` — tp-owned dirty files that need committing (§5.2). `kind` ∈ `closure` (the task file; `ref` = task id) / `round` (a `.tp-review/` round or snapshot file; `ref` = round number) / `config` (any other dirty `.tp/` state; `ref` = basename). Reported separately from `changes` and never an `unexplained-changes` blocker. Under `commit_strategy: hc` a close legitimately leaves these modified |
| `guidance` | one-line implement-phase note (run each unit in a fresh subagent/context); absent outside `implement` |
| `next_action` | `{command, brief_command, summary, payload}`; `command` is null for `decompose`/`release`. `brief_command` names the command that produces the brief for this phase. Payload: review/audit `{round, unresolved_findings}` (round = recorded+1, 0 unresolved on round 1), implement `{task: {id}|null, wip}`, `{action:"record-round", round:N}` when a snapshot exists without its recorded round, decompose/release `{}` |
| `blockers` | `{code, class, message, data}` in fixed code order |
| `next_units` (v0.35.0) | ordered `[]` of `{kind, id, brief_command}` — the units a driver should execute **now**. `[]` when the phase is blocked, awaiting an operator decision, or `release`. Concurrency is not repeated per entry: it is fixed per kind (see [Unattended Run](#unattended-run-v0350)) |
| `round` (v0.35.0) | the round `next_units` belongs to — the round being collected for role units, the round just recorded for the resolve/fix kinds — and `null` outside a round-based phase |
| `last_failure` (v0.35.0) | `{unit_kind, unit_id, phase, exit_code, summary, at}` from `.tp/last_failure-<base>.json`, or `null`. Advisory: it never changes which unit runs next |

`next_units`, `round` and `phase` are the whole machine surface — a driver parses those three and nothing else. `next_action` stays the human-facing summary, and its `command`/`brief_command`/`payload` render `next_units[0]` when the array is non-empty. `tp brief` surfaces `last_failure` too, so a fresh unit sees the wall the previous attempt hit.

`--compact` drops `next_action.summary`, each `kept[].reason`, and each `blockers[].message`; keeps every `data` plus `bookkeeping`, `guidance` and `next_units` (all decision-critical — §8.4).

Blocker vocabulary (fixed order): `unexplained-changes` (**agent-clearable**, `{count}`), `no-ready-task` (escalate, `{blocked_by}`), `review-budget-exhausted` / `audit-budget-exhausted` (escalate, `{cap}`; 0 = no cap, never fires), `spec-stale` (escalate, `{spec}`).

### `commit_strategy` — `builtin` / `auto` / `hc`

Resolves task override > `.tp/config.json` > built-in default `auto`. A present unrecognized value → `builtin` (with a stderr warning); an absent value → `auto`.

Under effective `hc`, `tp commit`, `tp done --auto-commit`, a bare `tp done`, and `tp close` are rejected with exit 2 and the hint `commit_strategy is hc: commit with hc, then tp done --commit <sha>`. A `tp done --batch` row with neither `commit_shas` nor `covered_by` is a failed row. No commit-strategy path returns exit 4.

Under `builtin`, `tp commit <id> [reason]` writes a conventional commit message carrying the task metadata:

```
feat(auth-model): Create User model

Model and migration created

Task: auth-model
Acceptance: Model exists. Migration runs.
```

`tp config` adds top-level `commit_strategy_effective` (`builtin`/`hc`); `tp config --resolved` reports `commit_strategy` as `{value, source}` with the resolved name.

`commit_shas` (`[]string`, canonical) records the ordered commits; `commit_sha` mirrors `commit_shas[0]` for pre-0.28.0 readers. It is a managed field (`tp set` rejects it; `tp reopen` clears it alongside `commit_sha`, `gate_passed_at`, `gate_skipped_reason`). A `--covered-by` close records neither.

**`commit_files` / `commit_files_total` (v0.30.0).** On a close that records commits (`tp commit`, `tp done --auto-commit`/`--commit`/`--batch`), tp resolves each sha's changed paths (added/modified/deleted/renamed-new) into `commit_files` — a managed, deduplicated, lexical-sorted array capped at 50 paths; `commit_files_total` records the true count when the set is larger. Both are managed (`tp set` rejects; `tp reopen` clears). A `--covered-by` close records none. When git is unavailable or a sha cannot be resolved, the field is omitted.

**`duration_source` (v0.30.0).** `claimed` when `started_at` came from an explicit claim (`tp claim`, `tp next`, `tp next --brief`); `implicit` when it came from an implicit claim (a bare `tp done`, or `tp commit` on an open task). Managed (`tp set` rejects; `tp reopen` clears). `tp report` carries it per task and excludes `implicit`-duration tasks from `estimation_accuracy` under a separate `implicit_duration` count; a `--covered-by` close is excluded from accuracy under `excluded_from_accuracy` (no measured span).

**Close checkpoint (v0.29.0, §5.1).** Under `builtin`, `tp commit` and `tp done --auto-commit` fold the closure into the implementation commit: tp stages the implementation files with the task file still `wip`, commits (sha `C1`), writes the closure record (`status: done`, `closed_at`, `gate_passed_at`, `commit_sha`/`commit_shas` = `C1`), then `git commit --amend --no-edit` folds it in (sha `C2`). The amend runs only when `HEAD` is still `C1` and the working-tree diff lists only paths tp itself wrote this command; otherwise tp makes a separate follow-up `chore(tp): record <id> closure` commit leaving `C1` as `commit_sha`. Either path leaves `git status` clean for tp-owned paths. `commit_sha`/`commit_shas[0]` records the pre-amend `C1` (never the post-amend `C2`), so `suggested_files` (§11) reads `C1`'s diff. Under `hc`, tp classifies instead of committing — see the `bookkeeping` field of `tp resume` for the tp-owned files a close legitimately leaves modified.

### `tp keep` — the keep-list

`.tp/local.json` (git-ignored, repo-scoped) gains `keep_uncommitted: [{path, reason}]`.

| Command | Purpose |
|---------|---------|
| `tp keep <path> "<reason>"` | add or update (a repeated path overwrites; path stored repo-root-relative from any subdirectory; a missing reason or malformed glob exits 2) |
| `tp keep --remove <path>` | drop an entry (an absent path is a no-op, exit 0) |
| `tp keep --list` | print the keep-list as JSON (`[]` when empty) |

Matching is Go `filepath.Match` (`*`/`?` do not cross `/`, no `**`); the first matching entry supplies the reason. After a successful close, `tp done`/`tp close` print a one-line stderr warning naming any uncommitted change not on the keep-list (exit 0; tp never commits or discards it).

## Unattended Run (v0.35.0)

`tp run [spec]` drives the whole cycle: read the cycle state through the same path `tp resume` uses,
stop if it is releasable, take `next_units`, spawn one runner process per unit, re-read the state
**from disk**, check caps, loop. A unit's result is whatever it wrote to disk; the driver reads a
child's exit code and one spend number and nothing else it said. `tp resume` stays the single
authority on whether the cycle is finished — `tp run` holds no opinion of its own.

It resolves the task file exactly as `tp resume` does (spec positional, `--file`, or discovery) and
takes a **run-scoped** lock at `.tp/locks/run-<base>.lock`, distinct from the task-file write lock
and held for the whole run, so a second `tp run` over the same task file exits 4. It never holds the
task-file write lock while a child is in flight — that lock stays the children's, taken and released
by each `tp` write they make.

| Mode | Exits | Lock | Writes run state |
|------|-------|------|------------------|
| `tp run` | **0** on `stop_reason: converged`, **4** on every other stop reason, **2** on a usage error raised before the loop starts | run lock | yes |
| `tp run --status` | **0** whenever it can report, **3** when no run state exists for the resolved task file | none | no |
| `tp run --dry-run` | **0** | none | no |

`tp run` prints `{run_id, phase, stop_reason, units}`, plus `notify: {cmd, exit_code, error}` when a
`notify_cmd` ran. `--dry-run` prints `{phase, round, next_units}` — the same batch the loop would
spawn — so it is safe to point at a cycle another run is already driving.

### Unit kinds

A unit is the smallest piece of work that ends in a durable write. `next_units` returns these eight
kinds and no others; `(kind, id)` identifies a unit, and `id` is its durable subject.

| Kind | `id` | Durable write | Concurrency | `brief_command` |
|------|------|---------------|-------------|-----------------|
| `implement` | task id | the task's `status` is `done` | alone | `tp next --brief` |
| `review-role` | role id | `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` exists and every content line parses | parallel with sibling roles | `tp review <spec>` |
| `review-record` | round number | `$TP_ROUND_DIR/merged.ndjson` **and** a review round file for `$TP_ROUND` exist | alone | `[ -f $TP_ROUND_DIR/merged.ndjson ] \|\| tp review --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson; tp review <spec> --record $TP_ROUND_DIR/merged.ndjson` |
| `review-resolve` | spec base name | every finding in `$TP_ROUND_DIR/merged.ndjson` carries a disposition | alone | `tp review <spec> --status` |
| `decompose` | spec base name | the task file holds at least one task | alone | `tp resume` |
| `audit-role` | role id | `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` exists and every content line parses | parallel with sibling roles | `tp audit <spec>` |
| `audit-record` | round number | `$TP_ROUND_DIR/merged.ndjson` **and** an audit round file for `$TP_ROUND` exist | alone | the `tp audit` form of the review-record command above |
| `audit-fix` | the finding's `role:item_id` | that row carries a disposition in the round's results file | alone | `tp audit <spec> --status` |

An attempt **succeeded** when the child exited 0 **and** the kind's durable write is present; either
alone is a failed attempt. Every predicate is a state, never a delta, which is what lets the `Stop`
hook and the driver test the same condition. The merge step of a record brief is guarded by the
merged file's absence, so a retried record unit never merges over the dispositions `review-resolve`
and `audit-fix` accumulate in that file.

A role unit writes `role-<id>.ndjson.part`; the **driver's rename** to the final name on exit 0 is
what completes the durable write, so a crashed unit's partial file is never mistaken for a clean
role. A role file's predicate reads content lines only — blank and whitespace-only lines are
ignored, so a trailing newline never fails a role.

### Stop reasons

A run stops for exactly one reason, recorded verbatim in the run state. Caps bound a run; they never
conclude it — **every non-converged stop is a report to a human, never an acceptance**, and no stop
reason ever records a round, marks a phase converged, or closes a task its own unit did not close.

| `stop_reason` | Cause |
|---|---|
| `converged` | the oracle reports `phase: release` — the only exit-0 stop |
| `cap-units` | spawned children reached `run_max_units` (a retry counts again) |
| `cap-wall-clock` | elapsed seconds reached `run_max_wall_clock_seconds` |
| `cap-budget` | accrued spend reached `run_max_budget_usd`; disabled when that value is 0 |
| `escalation` | a unit wrote an escalation record |
| `unit-failure` | one unit exhausted its `1 + run_max_unit_retries` attempts |
| `no-units` | the oracle reports no pending units and the cycle is not releasable |
| `interrupted` | `SIGINT`/`SIGTERM`: in-flight children finish, no new unit is spawned, run state is written |
| `driver-error` | the driver could not spawn a unit or write its own state — charged to the driver, not to the unit |

When one checkpoint satisfies several rows the recorded reason is the first of `converged`,
`driver-error`, `escalation`, `unit-failure`, `interrupted`, `cap-budget`, `cap-wall-clock`,
`cap-units`, `no-units`. Caps are evaluated **between iterations only**: children already spawned run
to completion, so a run can overshoot its wall-clock and budget caps by at most one iteration, and
the driver kills nothing.

### Run state, logs and `--status`

Run state lives at `.tp/run-<base>.json` (git-ignored, named per task file because the run lock is
per task file):

```json
{"run_id": "", "started_at": "", "phase": "", "stop_reason": null,
 "totals": {"units": 0, "wall_clock_seconds": 0, "spend_usd": 0},
 "units": [{"seq": 0, "kind": "", "id": "", "attempt": 1, "exit_code": 0,
            "duration_seconds": 0, "spend_usd": null, "log_path": ""}]}
```

A row is appended before its child is spawned (`exit_code`, `duration_seconds`, `spend_usd` null) and
updated in place when the child exits; both writes are atomic (temp + rename) because concurrent
siblings write rows in the same iteration. **The file is observability, not truth** — no decision,
the driver's or a hook's, reads it back, and a new run restarts the accrued totals at zero. `seq`
counts unit *attempts*, so `totals.units` and `--status`'s `units_done` read the same number.

Each unit's log is `$TP_RUN_DIR/<seq>-<kind>-<id>.jsonl`. A runner template that omits `{log_path}`
has its stdout and stderr redirected there by the driver; one that uses the placeholder receives that
same path and owns the file. Both built-in templates take the first form. The driver reads only the
spend key from the final line and never reads a log into any context.

`tp run --status` reports `{phase, run_id, started_at, units_done, wall_clock_seconds, spend_usd,
caps: {max_units, max_wall_clock_seconds, max_budget_usd}, last_unit: {…the whole row…},
stop_reason, run_state}`. `run_state` is `stopped` once `stop_reason` is set, otherwise `in_flight`
when `.tp/locks/run-<base>.lock` is held and `crashed` when it is not — the lock is the only evidence
separating the last two. On a stop in the audit phase it also carries `spec_coverage_clean_rounds`,
`role_streaks` and `divergence`, verbatim from `tp audit --status`. Under `--compact` `stop_reason`
and the cap totals survive and `last_unit` — its log path with it — is stripped.

### The child environment

Every child is spawned in the repository root with stdin closed and these variables, applied **after**
the `runner.env` merge so an `env` entry can never override them:

| Variable | Value |
|---|---|
| `TP_RUN_ID` | the run's ULID (26 Crockford characters) |
| `TP_RUN_DIR` | absolute `.tp/runs/$TP_RUN_ID` — logs and escalation records |
| `TP_ROUND_DIR` | `.tp/rounds/<base>/$TP_PHASE-r$TP_ROUND`, keyed by the **cycle** rather than the run, so a round survives a driver death |
| `TP_FILE` | the resolved task file, so a child works the target the driver resolved |
| `TP_UNIT_ID`, `TP_UNIT_KIND`, `TP_UNIT_SEQ` | the unit's id, kind and attempt sequence |
| `TP_PHASE`, `TP_ROUND` | the cycle's phase and round |
| `TP_UNATTENDED` | `1` |

`TP_ROUND` and `TP_ROUND_DIR` are **unset** — not empty — when the oracle reports `round` null (the
`implement`, `decompose` and `release` phases).

### The `runner` field

`runner` takes one of three shapes, told apart by their JSON alone: a **built-in template name**
(string), a **runner object** (an object carrying `cmd`), or a **per-kind map** (any other object,
dispatching each unit kind to one of the first two, with a `default` key covering the rest). An
object carrying both `cmd` and `default` is a runner; one carrying neither is a map. A map missing
`default`, or a runner object missing `cmd`, is a usage error.

| Runner-object field | Meaning |
|---|---|
| `cmd` | executable to spawn |
| `args` | argument template; placeholders `{prompt}`, `{max_budget_usd}`, `{unit_id}`, `{unit_kind}`, `{log_path}` |
| `env` | extra environment for the child, merged over the parent's |
| `spend_key` | dot path to the cost field in the runner's final log line; optional |

Two templates ship by name. `claude` is `claude -p {prompt} --output-format stream-json --verbose
--permission-mode auto`, with `--max-budget-usd {max_budget_usd}` appended **only** when the resolved
`run_max_unit_budget_usd` is non-zero (0 omits the pair rather than passing a literal 0), and
`spend_key: total_cost_usd`; it also appends `--agent <name>` per unit kind — `tp-implementer`,
`tp-reviewer`, `tp-auditor` — and none for the record, resolve, decompose and fix kinds. `opencode`
is `opencode run {prompt}`, with no budget flag and no `spend_key`, so `cap-budget` is inert for it.

`{prompt}` expands to a fixed per-kind instruction the driver owns: run this unit's `brief_command`,
do that one unit, stop. The driver never executes `brief_command` itself and never reads its output.
A placeholder the driver cannot resolve is a usage error (exit 2) raised before any child is spawned.

**Spend.** After a child exits the driver reads one number from the final line of that unit's log —
`spend_key`, or `total_cost_usd` for `claude`. A runner declaring none reports `spend_usd: null` for
its units and `run_max_budget_usd` is inert for them, reported once at run start rather than
silently. Under a per-kind map the cap accrues only the reporting kinds.

`TP_RUNNER_SEAM` is the test seam: its value is the `cmd` of a runner object whose `args` are
`["{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}"]` and whose `spend_key` is
`total_cost_usd`. It outranks every layer including a CLI flag, and the driver reads it from its own
environment at start, never from a child's.

### `TP_UNATTENDED` — user-only decisions fail closed

`tp run` sets `TP_UNATTENDED=1` for every child. The mode is active when the variable is present,
non-empty and not `0`. Under it the decisions reserved for the user stop being available to the
agent:

| Attempt | Result |
|---|---|
| `tp done --skip-gate` (and its other sinks: a `--batch` row's `skip_gate`, `tp close --skip-gate`) | exit 2, hint naming it a user-approved decision and pointing at `tp escalate` |
| `tp set --workflow review_max_rounds=` / `audit_max_rounds=` **above** the resolved value | exit 2, same shape |
| `tp set --workflow run_max_*=` **above** the resolved value | exit 2, same shape |
| `tp import --force` | exit 2, same shape |
| `tp set --workflow runner=` / `tp set --local notify_cmd=`, at **any** layer | exit 2, `names a command the driver executes and cannot be set under TP_UNATTENDED, at any layer` |

The cap comparison is against the currently **resolved** value; an equal or lower value is accepted
and exits 0, since lowering a budget cannot manufacture convergence. The exception is `0`, which
means *disabled* for both budget fields rather than *lowest*: setting either to 0 while the resolved
value is non-zero is a **raise** and is refused, while setting 0 where 0 already resolves is
accepted. Under the variable the fence applies to
every write path a fenced field has. (An earlier version of this sentence named `TP_RUN_MAX_UNITS`
and `TP_REVIEW_MAX_ROUNDS` as an env layer the fence ignores; neither variable exists in any
non-test source, and workflow fields have no env layer to ignore.)

This is a fence, not a sandbox: the refusals are enforced at tp's own CLI and the plugin's
`PreToolUse` hook denies the file-writing tools a path to the same values. A unit that strips the
variable from its own environment, or edits a config file through a shell, is outside what tp can
prevent — the guarantee is that no unattended unit reaches those decisions through a supported route.

### `tp escalate` — the escalation record

An escalation is a normal, expected outcome, not a crash. `tp escalate --decision <name> --evidence
<text> [--option <text>]…` writes `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` and exits **2**:

```json
{"decision": "raise-review-cap", "unit_kind": "review-role", "unit_id": "implementer",
 "phase": "review", "evidence": "…", "options": ["…"], "at": "2026-01-01T00:00:00Z"}
```

`decision` is one of `skip-gate`, `raise-review-cap`, `raise-audit-cap`, `import-force`, `other`;
`--option` is repeatable and `options` is `[]` when none is given. Outside a run (`TP_RUN_DIR` unset)
the command is a **usage error**, so it cannot be used to fabricate a record.

**The record, not the exit code, is the signal.** The driver spawns a harness, not `tp` itself, so
the harness's exit code need not carry the inner command's. A unit that wrote a valid record stops
the run with `stop_reason: escalation` whatever it exited with, is **not** counted as a failed
attempt, and does **not** set `last_failure`. A unit that wrote none, or one that fails schema
validation, is judged by its predicate and exit code as usual. The record is per unit, so concurrent
siblings never clobber each other; when an iteration produces several, the driver reports the lowest
`seq` and preserves them all. Resuming after an escalation is the operator making the decision and
running `tp run` again — nothing is replayed.

### `notify_cmd`

Invoked on **every non-converged stop**, not on escalation alone. It is exec'd **without a shell**,
split on whitespace, with `TP_STOP_REASON`, `TP_RUN_ID` and — on an escalation — `TP_ESCALATION_PATH`
in its environment. What it did is reported under `notify: {cmd, exit_code, error}`; its own exit code
is reported and changes nothing, so a failing `notify_cmd` never changes the run's `stop_reason`.

### `last_failure`

When a unit exits non-zero, or closes with a failed quality gate, tp records **one** object at
`.tp/last_failure-<base>.json`: `{unit_kind, unit_id, phase, exit_code, summary, at}`. It has two
writers, because the two triggers are visible to different processes:

- `tp done` writes it when the quality gate **it** ran fails, with the gate's own output as
  `summary`, and only when `TP_UNIT_KIND` is set — outside a run it writes nothing, since the
  record's `unit_kind`, `unit_id` and `phase` have no value there.
- the driver writes it when a child exits non-zero, with the failing command and the log path as
  `summary`.

Neither writer ever copies the child's prose. A second failure overwrites it; a success clears it
only when `(unit_kind, unit_id)` matches — id alone collides, since both record kinds are identified
by a round number. It lives outside the run state because it must survive the run that wrote it, it
is git-ignored, and it is **advisory**: its absence never changes which unit runs next. `tp resume`
and `tp brief` surface it when present.

### The plugin

The repository publishes a Claude Code plugin at its root: `.claude-plugin/plugin.json` (identity)
beside the existing `.claude-plugin/marketplace.json`, the existing `skills/tp`, plus `hooks/` and
`agents/`. **The Go binary is not shipped inside the plugin** — a marketplace is a git repository and
cross-platform binaries do not belong in one; installation stays Homebrew and `go install`. The
plugin preflights instead: if `tp` is absent from `PATH` or older than `plugin.json`'s own version,
the `SessionStart` hook fails with the install command rather than degrading quietly.

| Event | Matcher | Behaviour |
|---|---|---|
| `SessionStart` | `*` | preflight `tp`'s presence and version, then inject `tp resume --compact` as additional context |
| `PreToolUse` | `Write\|Edit\|MultiEdit\|NotebookEdit` | deny writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json` |
| `Stop` | — | inside a role unit whose findings file fails its predicate and with no escalation record, block **once** with the reason |

The session hook deliberately does not branch on the matcher (`clear` is not reliably reported on
every client, and an occasionally redundant orientation costs less than an occasionally missing one).
Shell tools are deliberately outside the `PreToolUse` matcher: the denial exists to stop
hand-editing, not to sandbox, and tp's own commands rewrite `*.tasks.json` on every close. The stop
hook is scoped to `review-role`/`audit-role` — the two kinds whose durable write is a file at a path
already in its environment — reads `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part` (it runs inside the
child, before the driver's rename) alongside `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json`, consults
`stop_hook_active`, and blocks at most once per unit. **Every hook declares a `timeout` of 10
seconds** and exits non-zero rather than hanging.

`agents/` declares `tp-implementer`, `tp-reviewer` and `tp-auditor`, carrying **tool restrictions
only** — role content stays in the corpus and reaches the unit through the prompt `tp review`/`tp
audit` already emits. The reviewer and auditor register their own `PreToolUse` allowlist permitting
exactly `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part` and `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json`
and refusing every other path; the implementer registers none, because an implement unit's durable
write is code. A runner that cannot load a plugin gets no agent definitions and the unit runs without
them: the restrictions are defence in depth, and the durable contract is the brief and the tp
commands the unit runs, which are identical in both paths.

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

You manage findings files yourself. Each emitted prompt carries an **`output_path`** (`review-r<N>-<role>.ndjson` / `audit-r<N>-<role>.ndjson`, relative to the working directory) and names that file in its text, so the merge glob is predictable rather than invented. Inside a `tp run` unit — where `TP_ROUND_DIR` is set — the same field instead carries `$TP_ROUND_DIR/role-<role>.ndjson.part`, the single path that unit is allowed to write and the one the driver renames when it exits 0; the name above is what you get everywhere else. Convention:
```
spec/
  feature.md                    # spec (keep)
  feature.tasks.json            # task file (keep)
  feature-r0.md                 # snapshot before round 1 edits (for --diff-from)
  feature-r1-merged.ndjson      # round 1 merged findings
  feature-r2-merged.ndjson      # round 2 merged findings
```

**Cleanup after review converges**: Delete review artifacts (snapshots `*-r0.md`, `*-r1.md`, etc. and findings `*.ndjson`). Keep the spec `.md` and task file `.tasks.json`.

**Injection caps**: an injected source file is capped at 8000 chars (50000 chars across all files), and a single emitted prompt is capped at 60000 chars.

## Workflow Fields (v0.23.0)

| Field | Type | Default | Range | `tp set --workflow` |
|-------|------|---------|-------|---------------------|
| `quality_gate` | string | `""` | — | task-level read-only (author at `tp init --quality-gate`); the project default is settable with `tp set --workflow --project` |
| `gate_timeout_seconds` | int | 600 | 30-3600 | settable |
| `lock_timeout_seconds` | int | 5 | 1-60 | settable (§12.1: write-lock retry/backoff window; timeout exits 4) |
| `checks` | array of `{class, cmd}` | `[]` | — | settable (replace semantics) |
| `review_clean_rounds` | int | 2 | 1-10 | settable |
| `audit_clean_rounds` | int | 2 | 1-10 | settable |
| `review_converge_on` | string | `blocking` | `blocking`\|`all` | settable (a review round is **clean** when no surviving finding is critical/high under `blocking`, or when no finding survives under `all`; audit never reads it) |
| `review_max_rounds` | int | 0 | 0-50 | settable (0 = no cap) |
| `audit_max_rounds` | int | 0 | 0-50 | settable (0 = no cap) |
| `run_max_units` | int | 100 | 1-10000 | settable (v0.35.0; `tp run` cap) |
| `run_max_wall_clock_seconds` | int | 28800 | 60-604800 | settable (v0.35.0) |
| `run_max_budget_usd` | number | 0 (disabled) | 0-10000 | settable (v0.35.0; decimal dollars) |
| `run_max_unit_budget_usd` | number | 0 (flag omitted) | 0-1000 | settable (v0.35.0; decimal dollars, passed to the runner's own budget flag) |
| `run_max_unit_retries` | int | 1 | 0-5 | settable (v0.35.0; a unit is attempted `1 + this`) |
| `runner` | string or object | `"claude"` | built-in name, runner object, or per-kind map | **not a `tp set --workflow` field** (`unknown workflow field`, exit 2) — hand-written under `workflow` in `.tp/config.json` or a task file |
| `notify_cmd` | string | unset | any command string | **not a `tp set` field of any form** — hand-written at the top level of `.tp/local.json` |

The five `run_max_*` fields resolve through the normal precedence and are settable with
`tp set --workflow` and `tp set --workflow --project`. `runner` and `notify_cmd` are the two
exceptions, and both name a command the driver executes, so neither has a setter:

- `runner` is read from a `workflow` block — a task file's or `.tp/config.json`'s — and
  `tp config --resolved` reports it as `{value, source}` like any other field (`project` /
  `override`). A `runner` key at the **top level** of `.tp/config.json` is ignored with
  `warning: unknown top-level key: runner`.
- `notify_cmd` is per-operator rather than per-project and is read from `.tp/local.json` **only**,
  as a top-level key; `tp config --resolved` reports `source: "local"`. Put it under a `workflow`
  block instead and it is ignored with `warning: unknown workflow key: notify_cmd`.
- Both are fenced under `TP_UNATTENDED` at **every** layer (not just above the resolved value):
  `tp set --workflow runner=…` and `tp set --local notify_cmd=…` exit 2 with
  `names a command the driver executes and cannot be set under TP_UNATTENDED, at any layer`.
- `TP_RUNNER_SEAM` is a test-only override of `runner` that outranks every layer including a CLI flag.

Out-of-range `tp set --workflow` writes are rejected with exit 1. Out-of-range values in a hand-edited task file fall back at read time (`gate_timeout_seconds`→600, caps→0) and `tp validate` warns.

These are the **defaults** a project-level `.tp/config.json` supplies and a task file's `workflow` block overrides — see [Project Configuration](#project-configuration-v0240). `tp set --workflow --project <field>=<value>` writes to the project config instead of the task file.

## Project Configuration (v0.24.0)

A repo-root `.tp/` directory holds workflow policy shared across every spec, so multi-spec repos keep one source of truth instead of copying policy into each `<base>.tasks.json`.

| File | Tracked? | Holds |
|------|----------|-------|
| `.tp/config.json` | commit to VCS | workflow **defaults** (same fields as the table above) |
| `.tp/local.json` | git-ignored (auto) | `active` task-file pointer + CLI flag `defaults` |
| `.tp/.gitignore` | commit (auto-written) | ignores `local.json`, `locks/` and the run artifacts (`run-*.json`, `runs/`, `rounds/`, `last_failure-*.json`); tracks `config.json` |

**Discovery**: walk up from the CWD to the first `.git` boundary; the `.tp/` there is the project config (single deterministic anchor).

**Resolution (resolve-at-read)** — effective value per field, highest layer wins. **The chain differs
by field kind**, and conflating them has cost two specs a review round each:
```
workflow fields   task-file workflow override  >  .tp/config.json  >  built-in default
output defaults   CLI flag  >  environment  >  .tp/local.json  >  built-in default
```
No workflow field has a CLI flag or a `TP_<FIELD>` environment layer: `engine/configresolve.go`
merges exactly two layers over the default for every one of them. Output defaults such as `no_color`
do have both, which is why the upper two rows exist at all.
A field in a task file's `workflow` block counts as an override only when present (absent ≠ zero). `checks` uses replace semantics (the winning layer's array wins whole).

| Command | Purpose |
|---------|---------|
| `tp config --resolved` | annotate each setting with `{value, source}` (source ∈ override/project/local/default/…) |
| `tp config --extract --dry-run` | print the extraction plan without writing |
| `tp config --extract --force` | merge into an existing `.tp/config.json` |
| `tp set --workflow --project <f>=<v>` | edit a project-level workflow field (flock, range-validated) |

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

Each round entry is `{round, findings, clean, recorded_at, file, spec_hash}`. `spec_hash` is `sha256:<hex>` of the spec bytes at record time and powers the staleness rule. tp writes `snapshot-round-<N>.md` **atomically** (write to a `.tmp` then rename) when round N begins (prompt emission) and `review-round-<N>.ndjson`/`audit-round-<N>.ndjson` when the round is recorded; a snapshot with no round file means a round was started and never recorded, and `--status` reports it as `in_flight_round: N` (`tp resume` then points at `{action:"record-round", round:N}`). A directory holding round files with no `state.json` aborts state-reading commands with exit 3 and a `repair or delete <path>` hint — that index referenced recorded history, and tp never rebuilds over it. A directory holding only snapshots is the in-flight window between emitting a round and recording it: nothing was ever recorded there, so the next `--record` rebuilds the index rather than refusing.

## Spec Frontmatter (`tp:` mapping)

A spec whose first line is `---` may carry a YAML frontmatter block. tp reads only the `tp:` mapping (every other top-level key is ignored) and excludes the whole block from every spec parser while preserving absolute line numbers.

```yaml
---
tp:
  domain: prose          # free string; default "software"; selects & filters the role corpus
  review_roles:          # additive focus override, keyed by role id
    implementer:
      focus: ["appended to the implementer role's focus"]
  audit_roles:
    security:
      focus: ["appended to the security auditor's focus"]
---
```

- `tp.review_roles` / `tp.audit_roles`: each maps a role id to an object whose permitted keys are `focus` (a string array, **appended** to that role's corpus focus at emission, project focus first) and `enabled` (a boolean, v0.32.0). Any other override key is a lint warning — `tp.<field>.<id>.<key> is not a permitted override key (only focus and enabled); ignored`; an override id matching no active role is ignored with a lint warning; the built-in `regression` role accepts no overrides.
- **Per-spec deactivation (v0.32.0):** `enabled: true` is accepted and is a no-op — it does not resurrect a role `domains` removed; a non-boolean value warns (`tp.<field>.<id>.enabled is not a boolean (got %T); ignored`) and leaves the role active; `enabled: null` or a valueless `enabled:` is unset. The deactivated role is named in `skipped_roles` with the reason `disabled-by-spec`, omitted under `--compact` like every other `skipped_roles` entry. Two refusals fire **on prompt emission only** — never under `tp review --record`/`--status`/`--merge`/`--resolve`/`--resolve-all`/`--verify`/`--report`, `tp audit --record`/`--status`/`--merge`, nor `tp review --perspective`, which short-circuits before the corpus is resolved — and each exits 2 before any prompt is emitted or any state is written: the **empty-phase** refusal `every <phase> role is deactivated by this spec: <ids>` (phase rendered from `PhaseReviewers`/`PhaseAuditors`, ids sorted and comma-separated and naming only what this spec deactivated; hint `re-enable at least one role, or remove the enabled: false entries`), and the **`spec-coverage`** refusal `spec-coverage cannot be deactivated: it carries the entire spec-derived checklist` (hint `remove the enabled: false entry for spec-coverage`), which is reported first when a spec trips both. Toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` unchanged. An entry whose id matches no active role is ignored with a stderr warning that `--quiet` suppresses but JSON mode does **not**, so a typo'd role id stays visible in a piped, agent-driven run.
- The standalone `tp: lens` block is **retired** (see Role Corpus). A legacy `lens` with no new overrides auto-translates to review-role focus (`lens.all` → every review role except regression; `lens.<id>` → that role) with a deprecation warning; the new form wins when both are present.
- `tp lint --json` reports a `frontmatter` object `{present, lines, domain, lens_roles}`. Malformed YAML is a lint error; unknown lens keys, non-list values, disallowed override keys, and unknown override ids are lint warnings.

## Role Corpus (v0.25.0)

Reviewer and auditor roles are project-owned JSON files under the repo-root `.tp/` (discovered via the git-boundary anchor, committed to VCS):

- `.tp/reviewers/*.json` — `tp review` roles; `.tp/auditors/*.json` — `tp audit` roles. The phase is inferred from the directory.
- Schema (one shared parser/validator): `id` (MUST equal the filename stem, `^[a-z0-9]+(-[a-z0-9]+)*$`, not the reserved `regression`), `title`, `instructions`, `focus` (string[], optional), `domains` (string[], optional — default: every domain). Any other top-level key is a validation error — tp owns the finding output contract.
- The embedded default auditor prompts changed in v0.31.2 (language-neutral wording). Every eject writes an advisory to stderr — `note: these roles are starting points; rewrite their focus for your project's stack and conventions.` — suppressed by `--quiet` but not by JSON mode; the eject JSON payload keys stay exactly `ejected` and `domain`.

| Command / flag | Meaning |
|----------------|---------|
| `tp init --eject-roles` | Write the default corpus into `.tp/reviewers` and `.tp/auditors` (byte-identical to the embedded prompts) |
| `tp init --eject-roles --domain <name>` | Eject the corpus for a shipped domain (`software`, `prose`); an unknown domain is a usage error (exit 2) |
| `tp init --eject-roles --force` | Overwrite existing role files regardless of validity |

Validation: `tp lint` validates both phases, `tp review` validates reviewers, `tp audit` validates auditors. A malformed or invalid role file aborts the phase-reading command with exit 3 and a `repair or delete <path>` hint (a broken auditor never blocks review — phase independence).

**Overlap fields** (`tp review --merge`/`--report`/`--status`): findings cluster by `(location key, class)`, the location key being the first `§<n>(.<n>)*` token. Each merged finding carries `found_by` (count of distinct diversity reviewer roles) and `found_by_roles` (the sorted set, `regression` excluded). `overlap_report` is a per-role array of `{role, unique, shared, trim_candidate}` — `trim_candidate` is true when `unique == 0 && shared >= 1`.

**`location_clusters` (v0.35.0, `tp review --merge`/`--status`):** the same merged records cut by `location` instead of by `(location, class)`. Roles compose their class slugs independently, so two roles naming one defect almost never collide and the merged count reads as "more to do" — this array says how much of it is one place seen through several lenses. One entry `{location, roles[], severities[], count}` per location key **two or more roles** reported: `roles[]` is the distinct contributing roles sorted (`found_by_roles` where the merge attributed the record, its own `role` otherwise; `regression` and blank excluded), `severities[]` the distinct severities most-severe-first, and `count` the number of merged **records** at that location — not pre-merge rows, since `(location, class)` duplicates have already collapsed. A location every finding of which came from one role produces no entry; the key is always present and `[]` when nothing clusters. It is **reporting only** — convergence arithmetic, the stored `clean` flag and the `--status --check` exit code are untouched — and on `--status` it is recomputed from the latest recorded round at read time, never stored. Being explanatory it is omitted under `--compact` (§8.4).

**`inputs` (v0.35.0, both `tp review --merge` and `tp audit --merge`):** one entry per input file, in argument order — `{path, parsed, skipped}` — beside the existing `input_files` count. A malformed line is warned about on stderr and counted in `skipped`; blank and whitespace-only lines are neither parsed nor skipped and never contribute to either number. An input with **at least one content line and zero parsed** makes the merge **exit 1**, naming that file: a role that emitted a trailing comma per line has every line skipped, and without this the merged set was silently short one role while the exit code stayed 0 and `--record` froze the undercount. A **zero-byte** file stays the documented way a role reports nothing found and keeps exiting 0, so a clean round is unaffected. Both merges apply the rule identically, because an unattended driver reads the exit code alone.

**Acting on a `trim_candidate` (v0.32.0):** a flagged role has **two levers, and they are guarded differently** — pick by whether the role is worthless on this one spec or worthless everywhere.

- **`enabled: false` in the spec frontmatter** removes the role from **one** spec (`tp.review_roles`/`tp.audit_roles`, see Spec Frontmatter above). It is **guarded** by the two emission-only refusals: it cannot empty a phase (`every <phase> role is deactivated by this spec: <ids>`) and it cannot deactivate the `spec-coverage` auditor (`spec-coverage cannot be deactivated: it carries the entire spec-derived checklist`). Both exit 2 before any prompt is emitted or any state is written.
- **Deleting the role file** removes the role from **every** spec and is **not guarded at all** — including for `spec-coverage`: the refusal protects it from a per-spec `enabled: false`, but nothing protects it from file deletion. And deleting the phase's **last** role file removes nothing: an empty phase directory reads as unpopulated, so `ResolveActiveCorpus` falls back to the embedded default corpus and the phase emits the built-in panel instead of no roles. To thin a phase down to one role, delete the others and keep a file.

tp does **not** guard against deactivating a role that has open findings — no refusal, no warning, for either lever. `ReviewRoundClean` evaluates cleanliness per round over that round's rows, so a deactivated role's earlier open findings do not keep a later round from being clean. Resolve or re-triage them before you trim.

**Measuring whether a second model earns its cost (v0.34.0):** `overlap_report` credits a *role label*, and nothing ties that label to the lens axis — so the same instrument measures model diversity. Run one reviewer role on model A in one round and the same role, same prompt, on model B in the next, then read `unique`.

1. Keep each round's raw per-role NDJSON — the file the reviewer wrote, before `--merge`.
2. **Relabel one of the two runs' `role` before merging.** Attribution credits the role string, so two runs sharing one label collapse into a single contributor: merging an `implementer` round with a second `implementer` round reports one row whose `unique` is just the count of the two rounds' combined clusters, and the question is unanswerable. Stamping the second file's rows `implementer@<model>` keeps them apart. `role` is free-form attribution metadata — `--merge` requires only `location`, `severity` and `finding`, and never checks `role` against the corpus.
3. `tp review --merge <round-a>.ndjson <round-b>.ndjson`, then read the two `overlap_report` rows.

`unique` is what only that model found, `shared` what both found, and `trim_candidate` on the second label is the answer *no* — the second model contributed nothing the first did not. It is a measurement of the two rounds merged, one role, and nothing more.

`--merge` is the read because `tp review --status`/`--report` compute `overlap_report` over the **latest recorded round only** and therefore can never span the two rounds. The relabelled `role` travels into whatever you `--record`, so record a copy with the corpus id restored if round history should keep it. No flag or config supports the recipe — it is the existing instrument read along a second axis.

**Transparency fields (v0.29.0, §9):** prompt emission (`tp review`/`tp audit`) reports `skipped_roles: [{role, reason}]` naming every corpus role it did not emit (`reason` ∈ `no-checklist-items` / `no-spec-change` / `domain-mismatch` / `no-baseline` / `disabled-by-spec`; `[]` when none skipped). Merge/`--status` summary adds `attribution_excludes: ["regression"]` **only** when excluding the built-in `regression` role causes `merged_count` to exceed the overlap-report finding count (omitted otherwise). `tp audit --merge`/`--status` emit their own `overlap_report` over non-PASS rows clustered by `(item_id, category)` with the same `{role, unique, shared, trim_candidate}` shape.


**`--role` payload contract (v0.36.0).** `--role <name>` narrows a `tp review`/`tp audit` emission to
one role. Its exact form and the reasoning are in [SKILL.md](SKILL.md); what the payload does is:

| Name class | Exit | `prompts[]` | Other keys |
|---|---|---|---|
| in the emitted set | 0 | exactly one entry, **byte-identical** to that role's entry in the same invocation without the flag | unchanged, except `review_loop.instruction` (below) |
| recognised, not emitted this round | 0 | `[]` — an array, never `null` | when the name is one *this* phase skipped, `skipped_roles` carries its own reason. A name recognised only through the **other** phase's corpus appears nowhere in `skipped_roles` — `tp review <spec> --role spec-coverage` exits 0, and the array holds whatever else the round skipped (at round 1, `regression`/`no-baseline`). `--perspective` and `--verify` carry no `skipped_roles` key at all. Under `--compact` the key is kept **only for this case** — an empty `--role` payload — because there the reason is the payload rather than commentary on it (§8.4's own criterion); with a prompt present `--compact` omits it as before |
| recognised nowhere | 2 | — | stderr carries `{"error":"unknown role: <name>","code":2,"hint":"this invocation emits: …"}`; the hint is built from the invocation's own emitted set plus `skipped_roles`, not from the corpus, because `regression` is emitted and belongs to no corpus |

Recognition spans the user corpus **and** the embedded default corpus for **both** phases, plus the
built-in `regression`. `tp review` never emits an auditor id, so a set built from one phase's
emission would make an auditor's own first command a usage error.

`review_loop.instruction` is the one key `--role` rewrites, and it has two rules. **An empty
`prompts[]` gets an empty key** — a payload with no prompt can support no directive at all, in every
mode. **A one-prompt payload gets a sentence-subset**, described next. Unrestricted the key is
addressed to a caller holding the whole panel (spawn a sub-agent per prompt, `--merge`, `--record`, `--status --check`,
the regression ordering, the uncounted delta pass); under `--role` it is a **sentence-subset** of
that string containing no directive a single-prompt payload cannot support. Nothing else moves:
`prompts[]` and `review_loop.instruction` are the only differences between a `--role` payload and
the unrestricted one, `review_loop`'s other members included.
**`--compact` disposition (§8.4):** decision-critical new fields survive `--compact` — `bookkeeping`, `suggested_files`, `max_rounds`/`rounds_remaining`/`in_flight_round`, `next_action`, `nonblocking_open` (review-only, emitted only on an accepted-open clean round); explanatory fields are omitted — `skipped_roles`, `attribution_excludes`, `location_clusters`, the audit `overlap_report`, the report `note`, and the wrapper-drift diagnostics `harness_note`/`harness_stale`. **One exception, v0.36.0:** `skipped_roles` survives `--compact` when `--role` reduced `prompts[]` to empty, because an empty payload is exactly where the reason stops being explanatory — it is the only content the payload has. `tp audit --compact` also omits `prompts[].checklist_items` and `prompts[].affected_files`: both duplicate content already rendered into `prompts[].prompt`, and together they are about a fifth of the audit payload (a 3-role/5-file run drops from ~24 KB to ~17 KB). Default output is unchanged — both stay arrays, never `null`.

**Role staleness** (`tp review --status`/`tp audit --status`): each recorded round stores `roles_hash` (`"builtin"` on the defaults, else a clone-stable sha256 over the phase's user files). `--status` reports `roles_stale` beside the spec `stale` flag; a pre-v0.25.0 round with no stored hash is treated as matching.

## Finding `class` and Report

Every review finding carries `role`, `location` (a `§`-anchor), `class`, and `severity` (tp's output contract). Finding text and `class` slugs are written in **English** whatever language the spec is in — the rows are committed artifacts and `--merge` clusters slugs byte-for-byte, so a second language splits one idea into two families; see "Recording language" in `SKILL.md`. The `class` field is free-form (a kebab-case slug, the dedup/cluster key), and an arbitrary class is carried through `--merge`/`--resolve` untouched. `tp review --merge` clusters by `(location key, class)` — a finding with no class is its own cluster — and emits each cluster's representative (highest severity, then lexicographic role, then finding text) annotated with `found_by`/`found_by_roles`. `tp review --report` adds a `by_class` breakdown, the per-role `overlap_report`, and `mechanize_candidates` (a class in ≥ 2 distinct rounds OR ≥ 5 times in one round), sorted by total descending, ties alphabetical.

**Canonical class `over-specification` (altitude).** The review output contract documents one canonical `class` value, `over-specification`. Its definition, the severity that decides whether it blocks, and why it is never mechanized are in [SKILL.md](SKILL.md).

## Audit JSON Schema (v0.23.0 — clean break from v0.22.0)

`tp audit` emits one prompt per non-empty role. There is no `--legacy-format` flag; downstream consumers MUST update.

| Field | v0.22.0 | v0.23.0 |
|-------|---------|---------|
| `prompts[].role` | a single fixed `implementation-auditor` | any active role id from the corpus |
| `prompts[].category` | always `null` | REMOVED |
| `prompts[].prompt` | paragraph text | structured; shared arm: Role → Role Rules → Project Context → JSON-array Checklist → Disposition → Affected Files → Output Schema; `spec-coverage`: Role → Role Rules → Spec Excerpt → JSON-array Checklist → Affected Files → Output Schema |
| `prompts[].checklist_items` | absent | `[]ChecklistItem` (`item_id`, `type`, `spec_line`, `section`, `text`, `expected_evidence`) |
| `prompts[].affected_files` | absent | `[]{path, tasks, diff_summary}` |

Item ids are deterministic: `table-<t>-<r>`, `list-<l>-<n>`, `task-<id>`, `file-<role-id>-<slug>` (the slug derives from the file path plus the item text, truncated to 40 characters, so the same file keeps the same id across rounds), `finding-<n>`. Sub-agents return one NDJSON row per checklist item: `{item_id, status(PASS|PARTIAL|FAIL), evidence_file, evidence_lines, category, severity, notes, class?}`. `category`/`severity` are `null` for PASS and one of the enum values for PARTIAL/FAIL. Finding category enum: `security > concurrency > error-handling > correctness > contract` (resolution precedence when several apply — the auditor picks one, tp does not resolve for it). **`tp audit --record` enforces the enum (v0.34.1):** a row carrying anything else aborts the record at exit 1, naming every offending line at once and listing the five, and no round file is written. A row with no `category`, which is every PASS row, is untouched.

**`file_summary` (v0.35.0):** `{total_files, total_lines, chars_included, truncated, total_changed}`. Auto-detect caps the audited set at **50** files. `total_files` stays the **audited** count, `total_changed` is the **pre-cap** count, and `truncated` is true exactly when the cap bit. The stderr notice names both numbers — `63 files changed, auditing first 50 — name the rest with --affected-files` — but the flag lives in the payload, so `--quiet`, which erases that line, cannot hide the truncation from a driver. `--compact` drops `file_summary` in its entirety, and the flag with it, so a `--compact` audit is not the place to read truncation from.

## Loop Integrity (v0.29.0)

Cross-cutting correctness, transparency, and contract fixes — no new lifecycle phase.

### Entry validation (§6)

`tp add` applies the same rules `tp import` applies, at entry:

| Rule | Exit |
|------|------|
| `id` missing/blank/whitespace, or already present | 1 |
| `title` missing/blank | 1 |
| `acceptance` missing/blank | 1 |
| Neither `source_sections` nor `source_lines` | 1 |
| `depends_on` referencing an unknown task | 1 |
| Task JSON is not valid JSON (decoder detail in `hint`) | 2 |

In `--bulk`/`--stdin`, validation runs after the whole batch is staged, so an entry may `depends_on` a task appearing later in the batch.

**Slice normalization (§6.2/6.3):** every task tp writes carries `[]` (not `null`) for `depends_on`, `source_sections`, and `tags`, whichever command wrote it. `tp lint`'s `structured_elements.tables`/`numbered_lists` and `tp stats`'s `tags` are likewise `[]` when empty.

### Coverage on every write (§7)

`AutoFillCoverage` runs after any command that changes the task set or a task's anchors — `add`, `import`, `remove`, and `set` of `source_sections`/`source_lines` — whenever the spec is readable, so `tp init` + `tp add` + `tp validate` is clean with no `import` round trip. Status-only writes (`claim`/`close`/`done`/`reopen`) skip the recompute. When the spec cannot be read, the coverage block is left untouched and `tp validate`'s coverage finding hints the unreadable spec path.

### `spec_excerpt` from `source_sections` (§8)

When a task has `source_sections` and no `source_lines`, `spec_excerpt` is the content of those sections: each entry's heading line plus the body up to the next heading of the same or shallower level, in the listed order, blank-line-joined, capped at 2000 chars. Applies to `tp plan`, `tp show`, `tp next`, and `tp next --peek` (which previously returned `""`). `--compact` still omits `spec_excerpt` entirely.

### Locking (§5.3, §5.4, §12)

- The task-file lock lives at `.tp/locks/<base>-<hash>.lock` (covered by `.tp/.gitignore`), not the sibling `<base>.tasks.json.lock`; a stale sibling lock is removed on first write. tp never stages a path under `.tp/locks/`, and `tp commit`/`--auto-commit` refuse any `--files` path ending in `.tasks.json.lock`. When they drop an accidentally-staged lock they untrack **only** `.tp/locks/**` and `*.tasks.json.lock` — a bare `*.lock` pathspec matches across directories, and until v0.33.0 it recorded `yarn.lock`, `Gemfile.lock` and every other lock file in the repo as deleted.
- **The lock file persists after the lock is released, by design — do not delete it while tp may be running.** flock is held on an inode, so unlinking the file lets the next waiter open the same path, get a fresh inode, lock that, and enter the critical section concurrently; that cost 4 silently lost rounds per 100 four-way concurrent `tp audit --record` runs before v0.33.0. The file is a zero-byte, git-ignored marker; one per locked target, never more.
- `tp init` creates `.tp/.gitignore` (it covers `local.json`, `locks/`, and `tp run`'s artifacts: `run-*.json`, `runs/`, `rounds/`, `last_failure-*.json`), and since v0.33.0 so does any locked write, so a project that never ran `tp init` does not accumulate an untracked `.tp/locks/`. Entries you add yourself are preserved: the file is only written when one of the required lines is missing.
- Write-lock acquisition retries with backoff until `lock_timeout_seconds` (default 5, range 1-60) elapses, so two concurrent writes both succeed. On timeout tp exits **4** (state) with a hint naming the lock path and the elapsed wait.

### Exit-code conformance (§13)

Exit scheme: 0 success, 1 validation, 2 usage, 3 file, 4 state. `tp add '{not valid json'` exits 2 (decoder detail in `hint`); any cobra flag-parse failure exits 2 as a tp error object; `tp done <id> "<reason starting with a dash>"` exits 2 with a hint naming the `--` separator. Every non-zero error object carries a `hint` naming the next command to run.

**An arity violation exits 2 (v0.35.0).** Every command's `Args` validator is a usage error, so
exit **2** uniformly means "tp did not run the request" and exit **1** stays "it ran and failed" —
the distinction a driver branches on. It used to exit 1 while an unknown command exited 2. Exit 2 is
deliberately not subdivided further: a refusal and a typo are the same instruction to the driver,
which is to stop and hand the unit's log to a human; what separates an *escalation* from either is
the record, and a unit that refuses to write one is a unit failure, which is the same stop.
`tp show`'s arity hint names the **missing argument** (`usage: tp show <id>`) rather than claiming to
name the failing object.

A path that names a **missing file** exits **3** everywhere it can (v0.32.0). A missing `--findings`
path now exits 3 in both phases: `tp review` previously exited 2, and `tp audit` previously accepted
it, verified zero review findings, and recorded the round as clean — a path typo could declare
convergence. A missing spec path exits 3 with a hint naming the spec rather than the task file. Once
a stat of that same path has already succeeded, a later failure passes the OS error as the hint,
because the path was right and the I/O was not.

A line past the **1MB per-line NDJSON cap** exits **3** with a hint naming the cap, at every reader
in the review/audit family (`--merge`, `--report`, `--resolve`, `--verify`, `--record`, and
`tp audit --findings`). It used to warn and drop the rows after the over-long line, which could turn
an unclean round clean. `tp add --bulk`/`tp set --bulk` keep their own warn-and-continue contract.

Both `--merge` modes report **`inputs`** — one `{path, parsed, skipped}` entry per input file, in
argument order — and exit **1** when an input had at least one content line and parsed none of them
(v0.35.0). That is the shape a dropped role takes: a reviewer emitting every line with a trailing
comma used to be skipped line by line on stderr, merge clean, and let `--record` freeze an
undercounted round, with `--quiet` able to erase the only signal. Blank and whitespace-only lines are
neither parsed nor skipped and never trigger it, and a **zero-byte file stays the way a role reports
nothing found** and keeps exiting 0 — so a clean round is unaffected. The exit-1 path still emits the
full payload and still writes `-o`: the surviving roles merge, only the exit code changes, because an
unattended driver reads nothing else.

`tp review --perspective code-audit --findings <file>` exits **2**: that perspective never reads the
file, and previously accepted the flag while reporting `previous_findings: 0` about it. A
spec-looking (`.md`) positional handed to `--merge` or `--resolve` exits **2** as well: those modes
take NDJSON inputs only.

### Report accuracy (§14)

`actual_minutes` rounds to one decimal place. When it rounds to `0.0`, `accuracy` is `null` and the task carries `note: "duration below resolution"` (omitted under `--compact`). The summary excludes null-accuracy tasks from `estimation_accuracy` and reports the count via `excluded_from_accuracy`.

### Lint: `empty-section` (§15)

`empty-section` no longer fires on a **container** heading — one whose next heading is deeper than itself. An empty **leaf** heading (no content, no deeper child) remains an error.

## Honest Convergence Signals (v0.33.0)

Four fields report *what* an audit or review round found without changing what the gate counts.
The audit gate is untouched: `engine.Converged`, `engine.ConsecutiveClean`, the stored per-round
`clean` flag, the exit code of `tp audit --status --check`, and `next_action`'s three-state audit
precedence all behave exactly as in v0.32.0.

### `role_streaks` (audit, §2.2)

Emitted on `tp audit <spec> --status` (with or without `--check`) and on
`tp audit <spec> --record <file>` — and nowhere else: not on `tp audit <spec>` prompt emission,
not on `tp audit --merge`, and on no `tp review` mode.

Type: an array of objects, one per role appearing in the **latest** recorded audit round (the panel
the current decision rests on, not every role ever seen).

| Field | Type | Meaning |
|-------|------|---------|
| `role` | string | the role id |
| `consecutive_clean` | int | trailing recorded rounds in which the role is clean |
| `open` | int | that role's non-PASS row count in the latest recorded round |

```json
"role_streaks": [
  {"role": "spec-coverage", "consecutive_clean": 9, "open": 0},
  {"role": "ax-contract", "consecutive_clean": 0, "open": 3}
]
```

A role is **clean in a round** when it has at least one row in that round and every one of its rows
is PASS. A role with no rows in a round is not clean in it, so its streak ends. A round that
contributes no rows (any of §2.1's four causes) ends every streak at once, as does a round whose
rows all carry no role — the conservative direction, resetting the signal toward "keep auditing".

Order: `spec-coverage` first when present, then the remaining ids in ascending byte order (the
ordinary Go string comparison, not a case-insensitive or locale-aware one), so a non-lowercase id
stays distinct.

Empty form: `[]` — an emitted empty array, never `null` and never an omitted key. Four states reach
it: no recorded round at all; the latest round contributes no rows; the latest round recorded zero
rows; the latest round's rows all carry no role. The array does not distinguish them —
`spec_coverage_clean_rounds` reports `null` in all four. Convergence arithmetic is independent of
the array, so `converged: true` beside `role_streaks: []` is reachable and correct.

Every entry has rows in the latest round, so `open == 0` and `consecutive_clean >= 1` are the same
condition and a role holding open rows always carries a streak of `0`. Both fields are reported on
every entry anyway: the streak separates a lens clean since round 2 from one that went clean this
round, and `open` gives the per-role magnitude on rounds where `divergence` is withheld.
`overlap_report` (`--status`, non-`--compact` only) is unchanged and cannot carry these numbers — it
credits a role only for the non-PASS clusters it contributed to, so the quiet roles never appear in
it.

### `spec_coverage_clean_rounds` (audit, §2.3)

Emitted on the same two outputs, as a top-level field. Type: integer or `null` — the
`consecutive_clean` of the `spec-coverage` entry of `role_streaks`. The key is **always** emitted,
`null` as an explicit JSON null, never an omitted key.

spec_coverage_clean_rounds is null, not 0, when the latest recorded round holds no spec-coverage row.
`null` and `0` are different answers and must not be collapsed: `0` means the role was measured
in the latest round and at least one of its rows is not PASS; `null` means the latest round did not
measure conformance at all, and a driver must never read the resulting absence of `divergence` as
evidence of anything.

Paths to `null`: the round simply carries no `spec-coverage` row (no sub-agent spawned, none
returned, or the merge dropped its file); no round is recorded at all; the auditor corpus holds no
`spec-coverage` role; the role is active but emits no prompt and is named in `skipped_roles` with
`no-checklist-items`; the latest round contributes no rows, recorded zero rows, or holds only rows
carrying no role. tp adds no refusal for any of them and does not judge which occurred. "Contributes"
is §2.1's sense, so a round whose rows are readable but whose stored `roles_hash` is empty yields
`null` even though the file holds `spec-coverage` rows.

### `divergence` (audit, §2.4)

Emitted on the same two outputs, and only when **all five** conditions hold:

1. `spec_coverage_clean_rounds` is non-null and at least the effective `audit_clean_rounds`.
2. The latest recorded round holds at least one non-PASS row **not** attributed to `spec-coverage`,
   including any row carrying no role.
3. The spec is not stale (the same `stale` both outputs report).
4. The sequence is not converged (the same `converged` both outputs report) — which is what keeps
   the hint from being printed beside an already-open gate.
5. The latest recorded round's stored `roles_hash` equals the auditor-corpus hash computed now.

Conditions 3 and 5 constrain `--status` alone: on `--record` the round is stored with the spec hash
and the corpus hash computed in the same invocation, so both equalities hold by construction. On
`--record` the signal is computed **after** the round is stored, so "the latest recorded round" is
the round just recorded. `--record`'s exhausted-`audit_max_rounds` refusal still exits 4 before any
payload, so no signal is emitted on that path; read it from `--status` instead.

When any condition fails the key is **omitted** — never emitted as `null`. Absence means only that
the five conditions did not all hold. When the object is emitted, all five of its fields are
present; none uses absence to mean zero.

| Field | Type | Meaning |
|-------|------|---------|
| `other_roles_open` | int | count of the non-PASS rows of condition 2 |
| `open_roles` | []string | ids of the roles holding those rows, each once, ascending by the same byte order `role_streaks` uses; `[]` (never `null`, never omitted) when every such row carries no role |
| `unattributed_open` | int | how many of those rows carry no role; `0` when none do |
| `message` | string | one of the three sentences below |
| `hint` | string | the constant below, verbatim |

```json
"divergence": {
  "other_roles_open": 24,
  "open_roles": ["ax-contract", "go-safety"],
  "unattributed_open": 0,
  "message": "spec-coverage clean 9 rounds; 24 findings open from other roles",
  "hint": "spec-coverage is the only role that measures spec conformance; ..."
}
```

The three `message` forms — the round count is always `spec_coverage_clean_rounds` (never the
threshold it was compared against), the finding count is always `other_roles_open`, and
`round`/`rounds` and `finding`/`findings` agree with their numbers:

```
spec-coverage clean 9 rounds; 24 findings open from other roles
spec-coverage clean 9 rounds; 24 findings open from other roles (including 3 with no role, which may be spec-coverage's)
spec-coverage clean 9 rounds; 24 findings open, none attributed to a role (possibly spec-coverage's)
```

The first when `unattributed_open` is 0, the second when it is between 1 and `other_roles_open - 1`,
the third when the two are equal — the state where `open_roles` is `[]`.

`hint` is `engine.DivergenceHint`, emitted verbatim on every round the five conditions hold:

```
spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Whether they gate this release is the operator's decision, not the agent's — surface it rather than deciding either way; audit convergence still counts every non-PASS row.
```

It names the operator rather than issuing a bare imperative: the reader is usually an agent, and
accepting open findings is a user-approved decision. Whenever `divergence` is emitted over state tp
itself recorded, `next_action` reads the fix-and-re-audit directive — `next_action` names the only
step tp can verify, `hint` names a decision tp cannot execute or record.

### `mechanized_classes` (review, §3.3)

Emitted on `tp review <spec> --record <file>` and nowhere else — not on `tp review --status` (which
carries no candidate array for it to explain) and not on `tp review --report`.

Type: an array of strings.
mechanized_classes names the candidate classes withheld because they are mechanized, and is [] when none were.
Each class appears once and the array is sorted ascending;
every member equals a valid `checks` entry's `class` and is therefore lowercase kebab-case, so byte
order and a case-insensitive sort cannot differ. It lists the intersection of the round's mechanize
candidates with the registered classes, not the whole registered set (read that from `tp config` or
a bare `tp review <spec> --status`).

```json
"mechanize_candidates": [],
"mechanized_classes": ["test-inventory"]
```

It is always an array on the output that carries it — `[]` when nothing was withheld, never `null`
and never an omitted key — and the filtered `mechanize_candidates` beside it keeps that shape too.
The same filtering applies to all three of the record path's sinks: the emitted
`mechanize_candidates` array, the register-a-check hint, and `next_action`'s mechanize branch, so a
registered check retires its mechanize candidate and the class is named here instead of vanishing.

### `--compact`

role_streaks, spec_coverage_clean_rounds and divergence all survive --compact, divergence with every field.
That includes `message`, `hint`, and the `spec_coverage_clean_rounds` key when its value is
`null`. `mechanized_classes` survives it too, because `mechanize_candidates` always has. `--compact`
is the mode an agent driver runs in, and these fields are the conclusion rather than a restatement
of it. The audit fields `--compact` already drops — `harness_stale`, `harness_note`,
`overlap_report` — are unchanged.
