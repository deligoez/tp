# tp — Reference

Detailed command reference, field formats, and operational details. For workflows and rules, see [SKILL.md](SKILL.md).

## Command forms

The exact form of every command and flag is the inventory in [SKILL.md](SKILL.md) — the document an
agent already has loaded mid-cycle. This document covers what those forms produce: fields, exit
codes and schemas.

`-o`/`--output` belongs to `--merge` alone. On any other mode both `tp review` and `tp audit`
reject it with exit 2 rather than accepting it and writing nothing there (v0.32.0); the payload
goes to stdout, so redirect it. `--merge … -o -` sends the merged rows to stdout and the summary to
stderr (v1.7.0).

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

tp counts acceptance criteria for closure verification and atomicity checking:

| Form | Example | Counted as |
|------|---------|------------|
| Prose | `"Model exists. Migration runs; tests pass."` | split on `. ` and `; ` — 3 |
| Bullet list | `"Setup.\n- Model exists; with a note\n- Migration runs"` | one per column-0 `- ` line, never re-split, plus one for any prose before the first bullet — 3 |
| JSON array | `["Model exists", "Migration runs"]` | one per element (stored joined with `\n- `) — 2 |

A non-bullet line after a bullet continues that bullet. `tp add` and `tp import` report the count per task as `criteria: {"<id>": N}`.

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
- `commit`: a git commit SHA, or an array of them (optional). `commit_shas` is accepted too; a row naming both fails and the other rows still close. A row that does not parse is reported by its line number.
- `gate_passed`: gate-less projects only; the gate now runs automatically once per batch invocation.

The batch gate runs once before any entry is processed, iff at least one surviving entry does not carry `skip_gate`. On gate failure, `skip_gate` entries still close and every other entry fails.

## Task File Discovery

Priority: `--file` flag > `TP_FILE` env var > `.tp/local.json` active pointer > auto-detect (current dir, then one level of subdirs). The legacy `.tp-active` marker was removed in v0.25.0.

An explicit `--file`/`TP_FILE` path that cannot be opened exits 3 naming that path (`task file not found: <path>`, `TP_FILE task file not found: <path>` for the env var) from every command, `tp add` and `tp resume` included (v1.2.2). The `no task file found. Use --spec to create one, or run tp init first` advice belongs to a genuine auto-detect miss, where no path was named.

Every task-file write reports the file it wrote as `file`, relative to the directory the command ran in. A write the pointer resolved while another task file is in reach (the cwd, its subdirectories, the file's own directory) prints one stderr notice naming the file, the pointer and the candidates; `tp init` and `tp import` warn when the pointer names another existing task file, and leave it where it is.

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
| `spec` | resolved spec path, relative to the repository root (v1.8.0; absolute outside a git repository). Every spec path resume prints — `next_action`'s `command`, `brief_command` and `summary`, each `next_units[].brief_command`, a blocker's `message` and `data.spec` — takes the same form, and so do `tp export`'s embedded `resume` and `tp export --list`'s `next_step` and `escalations` |
| `changes` | byte-sorted repo-root-relative uncommitted paths **not** on the keep-list |
| `kept` | byte-sorted `{path, reason}` — uncommitted paths matched by the keep-list |
| `bookkeeping` | `[]` of `{path, kind, ref}` — tp-owned dirty files that need committing (§5.2). `kind` ∈ `closure` (the task file; `ref` = task id) / `round` (a `.tp-review/` round or snapshot file; `ref` = round number) / `config` (any other dirty `.tp/` state; `ref` = basename). Reported separately from `changes` and never an `unexplained-changes` blocker. Under `commit_strategy: hc` a close legitimately leaves these modified |
| `guidance` | one-line implement-phase note (run each unit in a fresh subagent/context); absent outside `implement` |
| `next_action` | `{command, brief_command, summary, payload}`; `command` is null for `release`, and for `decompose` while the resolved workflow registers no check — with at least one registered, the decompose action carries the read-only `tp review <spec> --status --check` in **both** `command` and `brief_command` and its summary names that gate ahead of decomposing. `brief_command` names the command that produces the brief for this phase. Payload: review/audit `{round, unresolved_findings}` (round = recorded+1, 0 unresolved on round 1; `unresolved_findings` counts the last round's open findings — every review row and every non-`PASS` audit row not resolved `wontfix`/`duplicate` with evidence, so a `fixed` row still counts), implement `{task: {id}|null, wip}`, `{action:"record-round", round:N}` when a snapshot exists without its recorded round, decompose `{unit: {id, kind}}`, release `{}`. While an escalate blocker stands and `next_units` is empty, `summary` is the first escalate blocker's message, `command` and `brief_command` are null, and `payload` drops its step preview (`task`/`wip`, `round`/`action`) but keeps `unresolved_findings` |
| `blockers` | `{code, class, message, data}` in fixed code order |
| `next_units` (v0.35.0) | ordered `[]` of `{kind, id, brief_command}` — the units a driver should execute **now**. `[]` when the phase is blocked, awaiting an operator decision, or `release`. Concurrency is not repeated per entry: it is fixed per kind (see [Unattended Run](#unattended-run-v0350)) |
| `round` (v0.35.0) | the round `next_units` belongs to — the round being collected for role units, the round just recorded for the resolve/fix kinds — and `null` outside a round-based phase |
| `last_failure` (v0.35.0) | `{unit_kind, unit_id, phase, exit_code, summary, at}` from `.tp/last_failure-<key>.json`, or `null`. Advisory: it never changes which unit runs next |

`next_units`, `round` and `phase` are the whole machine surface — a driver parses those three and nothing else. `next_action` stays the human-facing summary, and its `command`/`brief_command`/`payload` render `next_units[0]` when the array is non-empty. `tp brief` surfaces `last_failure` too, so a fresh unit sees the wall the previous attempt hit.

The commands those paths name run from any directory of the repository: a spec argument that `tp review` (an emission, `--record`, `--status`), `tp audit`, `tp resume` or `tp run` cannot find from the working directory is resolved against the repository root (v1.8.0). A path found from the working directory keeps that meaning.

`--compact` drops `next_action.summary`, each `kept[].reason`, and each `blockers[].message`; keeps every `data` plus `bookkeeping`, `guidance` and `next_units` (all decision-critical — §8.4). Under an escalate blocker a `--compact` reader therefore reads `blockers[].code` and `data`.

Blocker vocabulary (fixed order): `unexplained-changes` (**agent-clearable**, `{count}`), `no-ready-task` (escalate, `{blocked_by}`), `epic-blocked` (escalate, `{epic, blocked_by_epics}`; v1.5.0: the bound file's epic has a dependency not done and no task is `wip`, so no implement unit is offered; v1.6.0: when every such dependency is a condition, data adds `operator: {decision: "epic-done", commands}` and the message names the operator's `tp epic done <c> --evidence <evidence>`; v1.7.0: an unsatisfied any-of group blocks too, data adds `blocked_by_any_of`, one member list per group, the message names it `one of {A, B}`, and `operator` adds `choices`, the group's records as alternatives, see the epic block below), `review-budget-exhausted` / `audit-budget-exhausted` (escalate, `{cap, blocking_fixed_at_cap}`; fires when the cap is reached and the loop is not done, never under cap 0), `review-carried-blocking` (escalate, `{count, rows}`; v1.16.0: in the review phase, the carried set holds an unresolved blocking row recorded before the latest round — one the roles are told not to re-report, so no further round clears it and `next_units` is empty; the message names the first row's `tp review <file> --resolve <index>` and the epoch, `rows` lists the first 5 as `carried_blocking_rows` does; see Review convergence), `spec-stale` (escalate, `{spec, commands}`; `commands` is, in order, what clears it, and the message says the same: `tp review <spec>`, then `tp review <spec> --record <file>`. v1.14.0: with a review round in flight, data adds `in_flight_round` and `commands` starts with `tp review <spec> --record <file>`, since a new emission is refused (exit 3) while the round's snapshot differs from the spec; over the old text a round over the new one follows, and `operator: {decision: "discard-emission", commands: ["tp review <spec> --force", "tp review <spec> --record <file>"]}` names the discard as the operator's decision; over the current text the record alone clears it. An emission the review cap refuses (exit 4) is never a command: with no round in flight and the cap spent, `commands` is empty and `operator: {decision: "raise-review-cap", commands: ["tp review <spec>", "tp review <spec> --record <file>"]}` names it; when recording the round in flight over the old text reaches the cap, `commands` is that record alone. A `--rewritten` epoch never clears `spec-stale`).

### `commit_strategy` — `builtin` / `auto` / `hc`

Resolves task override > `.tp/config.json` > built-in default `auto`. A present unrecognized value → `builtin` (with a stderr warning); an absent value → `auto`. Neither writer stores one: `tp init --commit-strategy` refuses it with exit 2 and `tp set --workflow --project commit_strategy=` with exit 1, both naming the valid values, so an unrecognized value only reaches a file edited by hand.

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
takes a **run-scoped** lock at `.tp/locks/run-<key>.lock` (`<key>`: see Run state below), distinct from the task-file write lock
and held for the whole run, so a second `tp run` over the same task file exits 4. It never holds the
task-file write lock while a child is in flight — that lock stays the children's, taken and released
by each `tp` write they make.

| Mode | Exits | Lock | Writes run state |
|------|-------|------|------------------|
| `tp run` | **0** on `stop_reason: converged`, **4** on every other stop reason, **2** on a usage error raised before the loop starts, **3** before the loop when the cycle's task file belongs to another project or that project's `.tp/config.json` cannot be read (v1.12.0; so is `--dry-run`) | run lock | yes |
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
| `review-record` | round number | `$TP_ROUND_DIR/merged.ndjson` **and** a review round file for `$TP_ROUND` exist | alone | `[ -f "$TP_ROUND_DIR/merged.ndjson" ] \|\| tp review --merge "$TP_ROUND_DIR"/role-*.ndjson -o "$TP_ROUND_DIR/merged.ndjson" && tp review <spec> --record "$TP_ROUND_DIR/merged.ndjson"` |
| `review-resolve` | spec base name | every finding in `$TP_ROUND_DIR/merged.ndjson` carries a disposition | alone | `tp review <spec> --status` |
| `decompose` | spec base name | the task file holds at least one task | alone | `tp resume` |
| `audit-role` | role id | `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` exists and every content line parses | parallel with sibling roles | `tp audit <spec>` |
| `audit-record` | round number | `$TP_ROUND_DIR/merged.ndjson` **and** an audit round file for `$TP_ROUND` exist | alone | the `tp audit` form of the review-record command above |
| `audit-fix` | the finding's `role:item_id` | every non-`PASS` row under that key carries a disposition in the round's results file | alone | `tp audit <spec> --status` |

An attempt **succeeded** when the child exited 0 **and** the kind's durable write is present; either
alone is a failed attempt. Every predicate is a state, never a delta, which is what lets the `Stop`
hook and the driver test the same condition. The merge step of a record brief is guarded by the
merged file's absence, so a retried record unit never merges over the dispositions `review-resolve`
and `audit-fix` accumulate in that file. In both phases the record step is chained onto it with
`&&`, so a merge that refuses ends the unit at its own stderr diagnosis instead of walking into
`--record`. The `[ -f X ] || merge -o X && record X` form reaches `record` on both live paths,
because a shell reads it as `(A || B) && C`: the guarded path (`X` present, merge skipped) and the
merged path (`X` absent, merge exits 0) each run the record, and only a refused merge stops the
chain. That is a real fence — neither merge leaves anything at `-o` when it refuses.

**The emitted brief differs from the form above in its quoting.** `recordBriefCommand` in
`internal/engine/briefcommand.go` leaves `$TP_ROUND_DIR` bare, so an unset variable merges into
`/merged.ndjson` and a value with a space splits into two arguments. The form above quotes the
variable and leaves the glob outside the quotes (`"$TP_ROUND_DIR"/role-*.ndjson`); run that form.

A role unit writes `role-<id>.ndjson.part`; the **driver's rename** to the final name on exit 0 is
what completes the durable write, so a crashed unit's partial file is never mistaken for a clean
role. A role file's predicate reads content lines only — blank and whitespace-only lines are
ignored, so a trailing newline never fails a role.

Under a run every prompt a round emits has a role unit: one per active corpus role and, on a review
round that carries the built-in `regression` prompt (from round 2, when a section changed since the
newest earlier snapshot or a recorded finding is resolved `fixed`), one more `review-role` unit whose
id is `regression`, after the corpus roles. The oracle and the emission read the same predicate, so
neither side can name a role the other lacks. The regression unit counts against `run_max_units`
like any other unit.

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

Run state lives at `.tp/run-<key>.json` (git-ignored, named per task file because the run lock is
per task file). `<key>` is the task file's base, path-escaped (unchanged for an ordinary name), for
a task file at the project root, and for one below it its directory and base escaped into one name
(`x/a.tasks.json` → `x%2Fa`, v1.12.0), so two same-base task files in different directories, or a
root file named `x%2Fa.tasks.json`, never share run state. The run lock
(`.tp/locks/run-<key>.lock`) and the last-failure record (`.tp/last_failure-<key>.json`) use the
same key, all three computed against the task file's project root and kept in its `.tp/`, so a run
started from a subdirectory is the one `tp run --status` reports from the root. A nested task
file's round collected under the pre-1.12.0 `.tp/rounds/<base>/<phase>-r<N>` is not read after the
upgrade: the driver spawns that round's role units again, and once the respawned round is recorded
the old directory can be deleted by hand. `scripts/clean-emissions.sh` reports it as an
`old-layout round dir` (with the tp plugin installed, run
`${CLAUDE_PLUGIN_ROOT}/scripts/clean-emissions.sh` from the project root). The run state:

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
when `.tp/locks/run-<key>.lock` is held and `crashed` when it is not — the lock is the only evidence
separating the last two. On a stop in the audit phase it also carries `spec_coverage_clean_rounds`,
`role_streaks` and `divergence`, verbatim from `tp audit --status`; when the auditor role corpus
cannot be read those three are left out and `audit_signals_omitted` names the read error. Under `--compact` `stop_reason`
and the cap totals survive and `last_unit` — its log path with it — is stripped.

### The child environment

Every child is spawned in the repository root with stdin closed and these variables, applied **after**
the `runner.env` merge so an `env` entry can never override them:

| Variable | Value |
|---|---|
| `TP_RUN_ID` | the run's ULID (26 Crockford characters) |
| `TP_RUN_DIR` | absolute `.tp/runs/$TP_RUN_ID` — logs and escalation records |
| `TP_ROUND_DIR` | `.tp/rounds/<dir>/<base>/$TP_PHASE-r$TP_ROUND`, keyed by the **cycle** rather than the run, so a round survives a driver death; `<dir>` is the task file's directory relative to the project root, empty for a task file at the root (v1.12.0), so two same-base task files in different directories never share a round |
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
| accepting a finding without a change — `wontfix`/`duplicate` on a critical or high review finding, or on any audit finding, at `--resolve` or `--resolve-all` | exit 2, escalates under `--decision accept-finding`; `fixed` is not fenced, nor is `tp audit --record` carrying an earlier acceptance forward (see Dispositions) |
| `--force` on a `tp ground`/`tp review`/`tp audit` emission that would discard an unrecorded round, and `tp epic review <slice> --discard` | exit 2, escalates under `--decision discard-emission`: sibling role units grade one emission concurrently |
| `tp review <spec> --rewritten` whose cut holds an unresolved blocking review row (v1.16.0) | exit 2, `--rewritten would cut N unresolved blocking review finding(s), which then stop blocking with neither a fix nor an acceptance; …`; the hint names the first row's `--resolve` and escalates under `--decision rewrite-past-blocking`. An epoch that cuts no such row is not fenced |
| `tp set --workflow runner=` / `tp set --local notify_cmd=`, at **any** layer | exit 2, `names a command the driver executes and cannot be set under TP_UNATTENDED, at any layer` |
| a change of `quality_gate` — **any** `tp set --workflow quality_gate=` write, at either layer; at `tp import`, `tp init --quality-gate` and `tp config --extract`, a write that **changes the gate that resolves** (carrying the current gate forward passes) | exit 2, `<what> changes the quality gate every close runs, which is a user-approved decision refused under TP_UNATTENDED`; the hint names that sink's own exit and escalates under `--decision skip-gate`, since a rewritten gate is a skipped one |
| a write of `audit_converge_on` that relaxes the audit gate (v0.37.0) — at `tp set --workflow`, `tp import` and `tp config --extract`, one that **changes the resolved value to `blocking`**; at `tp set --workflow --project`, **any** write of `blocking` | exit 2. The three change-rule sinks say `<what> changes the resolved audit_converge_on from <before> to blocking, which relaxes the audit gate and is a user-approved decision refused under TP_UNATTENDED`; the `--project` sink names no resolved value and says `<what> writes blocking into the layer every base resolves through, including bases tp cannot enumerate; blocking is the value that relaxes the audit gate, …`. Each hint names that sink's own exits and escalates under `--decision audit-converge-on` |

The cap comparison is against the currently **resolved** value; an equal or lower value is accepted
and exits 0, since lowering a budget cannot manufacture convergence. The exception is `0`, which
means *disabled* for both budget fields rather than *lowest*: setting either to 0 while the resolved
value is non-zero is a **raise** and is refused, while setting 0 where 0 already resolves is
accepted. Under the variable the fence applies to
every write path a fenced field has.

**`audit_converge_on` is fenced under `TP_UNATTENDED=1` at all four write paths, and the rule differs
by sink (v0.37.0).** At `tp set --workflow`, `tp import` and `tp config --extract` it is a **change
rule**: the refusal fires when the write moves the **resolved** value — task override > project
config > built-in — to `blocking`, and only then. So an `import` carrying an already-resolved
`blocking` forward passes (nothing changes); a write of `all` never trips it (`all` is the default
and the only value that tightens the gate); and writing `all` first and `blocking` second is refused
on the second write, so there is no walk-around. Fencing the *field* or the *value* at `tp import`
instead would refuse every import an opted-in project makes, because `tp import` carries the existing
block forward.

At `tp set --workflow --project` the rule is the **value**: a `--project` write of `blocking` is
refused whatever any single base resolves today. That write lands in the layer every base resolves
through, and some of those bases tp cannot enumerate — a task file named with `--file` from outside
the project root, or any base under `vendor/`, `node_modules/`, `.tp/`, `.git/` or a nested submodule,
which the project scan skips by design and without returning an error. Three change-rule
implementations were each falsified by a base their population did not reach, so this sink enumerates
nothing and reads the value alone. Two consequences a unit will notice: the refusal names no base,
because it speaks for all of them at once; and a `--project` write of `all` lands even in a tree
holding a directory the scan cannot read, since `all` can relax nothing. A value rule costs nothing at
this sink because `tp set` names a value rather than carrying state forward, which is the same fact
that makes it unusable at `tp import`.

The fourth sink, `tp config --extract`, is fenced with the other three and evaluated over **every**
base the repository resolves: the hoist changes resolution for every context that had no task-level
override, and it is prospective — after the hoist the next `tp init`/`tp import` writes an empty
workflow block that resolves `blocking` from the project layer, with no fenced write ever occurring.
`--force` does not exempt it.

The refusal names the exits **its own sink has** and the condition selecting each — three at
`tp import`, two at `tp set` (either layer) and at `tp config --extract` — because a unit
reading only "refused" stops a run over an authoring error one edit fixes, while a unit that meant
the relax and reads only the authoring exits writes the resolved value back, passes, and silently
reverts an operator-approved `blocking`. The authoring exits differ by sink: at `tp import` the
document is the fix (omit its top-level `workflow` key to carry the current block forward, or write
`"audit_converge_on": "<resolved>"` into it); at `tp set` and `tp config --extract` the write *is*
the change, so the exit is not to make it. `tp escalate --decision audit-converge-on` is reserved
for a unit that **intends** the relax — escalating an authoring error stops the run for a decision
that does not fix the document, and the next unit regenerates the same document and is refused
again.

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

`decision` is one of `skip-gate`, `raise-review-cap`, `raise-audit-cap`, `import-force`,
`audit-converge-on`, `accept-finding`, `discard-emission`, `epic-done`, `gate-met`, `epic-binding`,
`grant-round`, `rewrite-past-blocking`, `other`; the three epic decisions are the v1.5.0 operator
records (`tp epic done`, `gate --met`, a binding), `grant-round` is v1.7.0's
`tp epic review <slice> --grant-round`, `rewrite-past-blocking` is v1.16.0's `--rewritten` over an
unresolved blocking review row (see Epochs), and `tp epic import` keeps `other`;
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
`.tp/last_failure-<key>.json`: `{unit_kind, unit_id, phase, exit_code, summary, at}`. It has two
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
| `PreToolUse` | `Write\|Edit\|MultiEdit\|NotebookEdit` and codedbpro's `create`/`edit`/`patch`/`replace`/`batch` | deny writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json` inside the session's repository (git's top level from `$CLAUDE_PROJECT_DIR`, else the hook's cwd); a write outside it, such as a fixture under `mktemp -d`, passes, and so does a read-only tool; a `batch` is judged op by op, so one whose ops are all reads passes |
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
and refusing every other path, with a `batch` judged op by op as the plugin hook judges it (both use
`hooks/drop-batch-reads.awk`; without it a hook keeps the whole payload); the implementer registers none, because an implement unit's durable
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

You manage findings files yourself. Each emitted prompt carries an **`output_path`** (`<spec dir>/.tp-review/<base>/review-inbox/round-<N>-<role>.ndjson` / `.../audit-inbox/round-<N>-<role>.ndjson`, v1.12.0: the phase's inbox in the spec's own state directory, so no two specs share a role file even when their bases match; the inbox is apart from the recorded rounds, `--record` removes a round's files from it once the round is on disk, and the plugin's write fence lets exactly these names through. The pre-1.12.0 cwd-relative `<phase>-<base>-r<N>-<role>.ndjson` is never read: `--merge` exits 3 on an input in that shape, naming the inbox file to move it to) and names that file in its text, so the merge glob is predictable rather than invented. Inside a `tp run` unit — where `TP_ROUND_DIR` is set — the same field instead carries `$TP_ROUND_DIR/role-<role>.ndjson.part`, the single path that unit is allowed to write and the one the driver renames when it exits 0; the name above is what you get everywhere else. Convention:
```
spec/
  feature.md                    # spec (keep)
  feature.tasks.json            # task file (keep)
  feature-r0.md                 # snapshot before round 1 edits (for --diff-from)
  feature-r1-merged.ndjson      # round 1 merged findings
  feature-r2-merged.ndjson      # round 2 merged findings
```

**Cleanup after review converges**: Delete review artifacts (snapshots `*-r0.md`, `*-r1.md`, etc. and findings `*.ndjson`). Keep the spec `.md` and task file `.tasks.json`.

**Injection caps**: an injected source file is capped at 8000 bytes (50000 bytes across all files), cut on a rune boundary under a marker naming the bytes kept and the file's size, and a single emitted prompt is capped at 60000 chars.

## Workflow Fields (v0.23.0)

| Field | Type | Default | Range | `tp set --workflow` |
|-------|------|---------|-------|---------------------|
| `quality_gate` | string | `""` | — | settable at both layers, attended only; at the task layer `quality_gate=` removes the override (reply `"removed": ["quality_gate"]`) rather than storing an empty gate. Switching a spec's gate off is `--skip-gate` with a recorded reason |
| `gate_timeout_seconds` | int | 600 | 30-3600 | settable |
| `lock_timeout_seconds` | int | 5 | 1-60 | settable (§12.1: write-lock retry/backoff window; timeout exits 4) |
| `checks` | array of `{class, cmd}` | `[]` | — | settable (replace semantics). A check exits 0 when it passed and 1 when it found violations; anything else — 2 or higher, 126/127, a signal, a start failure, a timeout — could not run and is reported `ran: false` in `mechanical_checks`. A prompt suppresses a class only when its check reported `ran: true` in that emission; with no task file resolving no check runs, so nothing is suppressed |
| `review_clean_rounds` | int | 2 | 1-10 | settable |
| `audit_clean_rounds` | int | 2 | 1-10 | settable |
| `review_converge_on` | string | `blocking` | `blocking`\|`all` | settable (a review round is **clean** when no surviving finding is critical/high under `blocking`, or when no finding survives under `all`; the same rule says which unresolved carried row blocks convergence, see Review convergence; audit never reads it) |
| `audit_converge_on` | string | `all` | `blocking`\|`all` | settable, and the one **fenced under `TP_UNATTENDED`** — by a change rule at `tp set --workflow`, `tp import` and `tp config --extract`, and by a value rule at `tp set --workflow --project` (v0.37.0; an audit round is **clean** when it has no non-`PASS` row under `all`, or when no non-`PASS` row carries a blocking severity under `blocking`; review never reads it) |
| `review_max_rounds` | int | 3 | 0-50 | settable (0 = no cap); at the cap the loop is done once every finding of the latest round carries a disposition and no unresolved blocking row is carried |
| `audit_max_rounds` | int | 3 | 0-50 | settable (0 = no cap); as `review_max_rounds` |
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

**The two converge-on fields are twins with deliberately opposite defaults (v0.37.0).**
`review_converge_on` defaults to `blocking`; `audit_converge_on` defaults to `all`, which is the
behaviour tp shipped before the field existed. The vocabularies do not overlap and neither field is
read by the other phase: review grades `critical`/`high` as blocking, audit grades `error` — plus
**any severity tp cannot grade**, which is treated as blocking rather than ignored, so a row with
`severity: null`, an absent key, a non-string value or a string outside the enum keeps a round from
being clean under `blocking`. `severity` on an audit row is self-declared and nothing on the audit
path validates it, which is why the default is not `blocking`.

An audit round is graded under the policy it was **recorded with** (stored as `converge_on`), so
setting the knob governs rounds recorded *after* it is set, in both directions: a round recorded
under `blocking` keeps `clean: true` when the knob returns to `all`, so one approved round of
`blocking` relaxes that cycle's streak permanently — re-record the round to undo it. Dispositions
are read live, as on the review side: an evidenced `wontfix`/`duplicate` written after the record
clears its round, and the next `--record` carries it forward while its file is unchanged (see
Dispositions). `role_streaks[].open`, `consecutive_clean` and `spec_coverage_clean_rounds` stay
severity-blind, so under `blocking` a round can record `clean: true` beside a non-zero `open`.

Illegal values are refused at both ends with the same hint as `review_converge_on`
(`must be one of: blocking, all`): a bad literal at a write sink is a **usage error (exit 2)**, and a
bad value already stored, reaching `tp audit --record`/`--status`, is a **validation error (exit 1)**.
The four write paths are `tp set --workflow`, its `--project` form, `tp import` and
`tp config --extract`; all four are fenced under `TP_UNATTENDED` — by the change rule at
`tp set --workflow`, `tp import` and `tp config --extract`, and by a value rule at the `--project`
form, both described in [`TP_UNATTENDED`](#tp_unattended--user-only-decisions-fail-closed) — and an
attended operator writes `blocking` at any of them with no refusal.

Out-of-range `tp set --workflow` writes are rejected with exit 1. Out-of-range values in a hand-edited task file fall back at read time (`gate_timeout_seconds`→600, caps→0, uncapped) and `tp validate` warns.

These are the **defaults** a project-level `.tp/config.json` supplies and a task file's `workflow` block overrides — see [Project Configuration](#project-configuration-v0240). `tp set --workflow --project <field>=<value>` writes to the project config instead of the task file.

## Project Configuration (v0.24.0)

A repo-root `.tp/` directory holds workflow policy shared across every spec, so multi-spec repos keep one source of truth instead of copying policy into each `<base>.tasks.json`.

| File | Tracked? | Holds |
|------|----------|-------|
| `.tp/config.json` | commit to VCS | workflow **defaults** (same fields as the table above) |
| `.tp/local.json` | git-ignored (auto) | `active` task-file pointer + CLI flag `defaults` |
| `.tp/.gitignore` | commit (auto-written) | ignores `local.json`, `locks/`, the run artifacts (`run-*.json`, `runs/`, `rounds/`, `last_failure-*.json`) and the epic review inboxes (`epic-review/*/inbox/`, v1.7.0); tracks `config.json` |

**Discovery**: walk up to the first `.git` boundary; the `.tp/` there is the project config (single deterministic anchor). The walk starts from the **task file's directory** whenever a command has a task file (`--file`, `TP_FILE`, the active pointer, auto-detection, or for review and audit the task file beside the spec), and from the working directory only when it has none (v1.12.0; before, always the working directory, so `--file` naming another tree's task file ran this tree's `quality_gate`, caps and checks against it). The `.tp/local.json` the `active` pointer and the flag `defaults` come from stays the working directory's, since it is read to find the task file; `notify_cmd` follows the task file. The role corpus is found from the spec's directory, as before. A `config.json` that exists but cannot be read or parsed is an **error (exit 3)** on every command that uses the project layer, never "no project config".

**Project root and work in the tree (v1.12.0).** A task file's **project root** is the directory holding the project `.tp/` discovery stops at; with none, the git top level; with neither, the task file's own directory. A `.tp/` is a **project** `.tp/` when it is empty or holds anything beyond what a lock or a run writes as a side effect (`.gitignore`, `locks/`, `rounds/`, `runs/`, `run-*.json`, `last_failure-*.json`), recognised by name whatever the `.gitignore` says: one that exists only because such a write needed somewhere to put its files is skipped by discovery and never becomes a root. That includes the nested `.tp/` tp 1.11's locks left beside a task file outside a repository (`docs/.tp/` with `.gitignore` and `locks/`), which can be deleted. Inside a repository those writes land in the root's `.tp/`; outside any repository and project they write no `.gitignore`. With no project `.tp/` and no repository at all, **the first deliberate write decides the root**: `tp init` writes `.tp/.gitignore` and, there only, an empty `.tp/config.json` (`{}`, no setting) in the working directory it runs from (the directory above `docs/` for `tp init docs/spec.md`), and every task file below it then belongs to that project; a task file no such write anchors is its own directory's.

A **mode** whose work happens in the working tree exits **3** (a file error: the task file belongs to another project) when the task file it resolved, by its own rule, has a project root other than the working directory's, with a hint naming the directory to run it from. The tree modes are `tp done`, `tp close`, `tp commit`, `tp import`, `tp run` and `tp run --dry-run` (its next units come from the oracle, which reads the working tree's git status), `tp review`'s emission, `--perspective`, `--verify` and `--record` (it runs the registered checks), `tp audit`'s emission (refused before changed-file detection), and any registered check wherever it runs. For a review or audit with no task file the spec itself is what is compared. Modes that read only the task file and its state work from outside: `tp status`, `tp show`, `tp list`, `tp plan`, `tp next`, `tp resume` (which then classifies the task file's tree), `tp config`, `tp run --status`, `tp review --status`/`--merge`/`--resolve`/`--resolve-all`/`--report`, `tp audit --status`/`--record`. Every mode is placed in `internal/cli/command_sides.go`; one test fails on a mode left unplaced, and another drives every tree mode with a foreign task file every way it can be given one (a positional spec, `--file`, `TP_FILE`, the active pointer).

**Resolution (resolve-at-read)** — effective value per field, highest layer wins. **The chain differs
by field kind**, and conflating them has cost two specs a review round each:
```
workflow fields   task-file workflow override  >  .tp/config.json  >  built-in default
output defaults   CLI flag  >  environment  >  .tp/local.json  >  built-in default
```
No workflow field has a CLI flag or a `TP_<FIELD>` environment layer: `internal/engine/configresolve.go`
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

**`epic.threshold_placeholder`** (v1.8.0): a top-level `epic` block in `.tp/config.json`,
`{"epic": {"threshold_placeholder": "<Go regexp>"}}`, sets the pattern that marks a gate threshold as
a placeholder (`gate-threshold-unwritten`, `operator.threshold_rewrites`, the export's
`milestones[].state.threshold`). Absent or empty, the default applies:
`(?i)^(?:\[[^\[\]]*\]|<[^<>]*>|TBD|TODO|TBC)$`, matched against the trimmed threshold. An unknown key
in the block, or a value that is not a string, warns and is ignored. `tp config` prints
`epic: {threshold_placeholder}`, the effective pattern, beside `workflow`, `--resolved` as `{value,
source}` with `source` `project` or `default`, and `tp config --extract` keeps the block. A pattern that
does not compile is refused by `tp validate --project` alone (exit 1, naming the key and the compile
error); `tp epic lint`, `ready`, `status` and `export` print one warning naming the key on stderr and
fall back to the default, and `tp config` warns that the project's pattern does not compile and prints
the default, `--resolved` with `source` `default`. When `.tp/config.json` cannot be read, `lint`,
`ready` and `status` warn and fall back too; `tp epic export` exits 3.

## State Directory (`.tp-review/`)

tp owns the review/audit round lifecycle in `<spec-dir>/.tp-review/<spec-base>/`:

| File | Content |
|------|---------|
| `state.json` | Round index: `{spec, review_rounds: [...], audit_rounds: [...], epochs: [...]}` (`epochs` absent until the first `--rewritten`) |
| `snapshot-round-<N>.md` | Byte copy of the spec at round N prompt generation |
| `review-round-<N>.ndjson` | Recorded review round N findings |
| `audit-round-<N>.ndjson` | Recorded audit round N results |
| `snapshot-ground-round-<N>.md` | Byte copy of the spec at ground round N emission |
| `floor-ground-round-<N>.txt` | The floor index that emission derived from that snapshot |
| `ground-round-<N>.ndjson` | Recorded ground round N dispositions |
| `audit-inbox/round-<N>.pools.json` | Each code role's file pool and listed `file_check` ids at audit round N emission, removed at `--record` (v1.15.0) |

Each round entry is `{round, findings, clean, recorded_at, file, spec_hash, hash_scheme}`, and an audit round recorded after its emission adds `graded` (v1.15.0, see the Audit JSON Schema). `spec_hash` is `sha256:<hex>` of the text the round read and powers the staleness rule; `hash_scheme` names which text that is — `snapshot`, the round's own emission snapshot, or `no-emission`, no snapshot on disk at record time, so the hash is of the spec file and names text no prompt read. Both markers are written from v1.2.2; a round recorded before them carries none and keeps the meaning it was written with, the spec as it stood at record time. Staleness compares the current spec against the last recorded round that read text, skipping `no-emission` rounds; where no round of the phase ever read text the spec is stale whatever it holds, since a record clears staleness only through the snapshot of its own emission. Such a `--record` is still accepted, and says on stderr why `spec-stale` stays and that `tp <phase> <spec>` emits the round that binds to the text. An unmarked round from before v1.2.2 counts as one that read text, so a history converged before the markers stays as it was. tp writes `snapshot-round-<N>.md` **atomically** (write to a `.tmp` then rename) when round N begins (prompt emission) and `review-round-<N>.ndjson`/`audit-round-<N>.ndjson` when the round is recorded; a snapshot with no round file means a round was started and never recorded, and `--status` reports it as `in_flight_round: N` (`tp resume` then points at `{action:"record-round", round:N}`). Re-emitting an unrecorded round is idempotent while the spec is unchanged; if the spec changed since the emission, `tp ground`, `tp review` and `tp audit` exit 3 naming the round and the files they would overwrite, and `--force` discards the emission (exit 2 under `TP_UNATTENDED`). A refused `--role` writes nothing to the state directory. A snapshot that is present but unreadable aborts `--record` with exit 3, naming the round, the snapshot path and the way out: remove or repair it, then re-emit the round and record its findings — the recorded hash is that snapshot's, so tp does not guess the spec file in its place. A directory holding round files with no `state.json` aborts state-reading commands with exit 3 and a `repair or delete <path>` hint — that index referenced recorded history, and tp never rebuilds over it. A directory holding only snapshots is the in-flight window between emitting a round and recording it: nothing was ever recorded there, so the next `--record` rebuilds the index rather than refusing.

**Epochs (v1.14.0).** `tp review <spec> --rewritten "<evidence>"` appends `{after_review_round,
after_audit_round, evidence, at}` to `epochs`. Each position is, per phase, the number of the last
recorded round that read an older text than the spec now holds — a round whose `spec_hash` differs
from the spec's, or whose `hash_scheme` is not `snapshot` (an unmarked or `no-emission` round, whose
reading cannot be established) — and never less than the newest earlier epoch's position for that
phase; rounds over the current text stay uncut. A spec reverted to an earlier text cuts the rounds
that read that text the first time too, since the position is one number per phase. The evidence is
kept verbatim and `at` is the RFC 3339 UTC time. Each phase's rounds numbered at or below the newest
epoch's position stop feeding the lists of earlier findings an emission carries: the review carry
list (`UNRESOLVED`/`ACCEPTED findings from previous rounds`), the regression pass's fixed findings
(and with them `tp run`'s regression unit), the audit Prior Round block, the audit acceptance carry
at `--record`, and `scripts/audit-round-prep.py`, which exits 2 `round-before-epoch`. Nothing
recorded changes and no disposition is written; staleness and the round cap read every recorded
round as before, and so do the mechanize candidates, `--report`, the exports and the audit role
streaks the ship rule reads. Review convergence reads the trailing clean rounds over every round and
the carried set (Review convergence, below) over the rounds after the epoch only, so a row the epoch
cuts stops blocking unless a role raises it again. `--status` in both phases reports `epochs` when
there is one, and keeps it under `--compact`. The mode writes `state.json` only and reads the task
file's side (no working tree). It exits 2 with blank evidence, with no recorded round in either
phase, and when the epoch would cut nothing — in both phases every recorded round already read the
current text or was cut by an earlier epoch, which is also a second `--rewritten` in a row. It exits
3 with a round of either phase in flight: the hint names that round's record, after which
`--rewritten` keeps the round if it read the current text, and its `--force` discard as the
operator's decision when the round's snapshot is not the spec. It refuses `--force`, `--no-state`,
`--docs-path`, `--test-path` and every other mode's flags with exit 2. `--rewritten` run
concurrently with an emission of either phase is unsupported: an emission takes no state lock, so
the in-flight check can race it.

**An epoch past a blocking row is the operator's decision (v1.16.0).** When the review rounds the
epoch would cut hold an unresolved blocking row of the carried set, the epoch is
`rewrite-past-blocking`: under `TP_UNATTENDED` it exits 2 and writes nothing, its hint naming the
first row's `--resolve` (if the rewrite fixed it) and `tp escalate --decision rewrite-past-blocking`.
By hand it runs, as accepting a finding by hand does — the person running it is the operator — and
says so on stderr; the payload adds `cut_blocking` (the count) and `cut_blocking_rows` (the first 5,
each `{round, file, index, severity, location, finding}`). No flag is asked for: `--force` stays the
discard of an unrecorded emission, which an epoch never is. An epoch whose cut holds no unresolved
blocking row is the ordinary mode, unattended too.

**Review convergence (v1.16.0).** The *carried set* is what the next review emission lists under
`UNRESOLVED`/`ACCEPTED findings from previous rounds`: the rows of the rounds after the newest epoch,
each finding — its category, location and the first 80 bytes of its text — taken at its **latest**
occurrence, so a row fixed in a later round is not held open by an earlier copy, and two findings at
one location stay two rows. A carried row is *resolved* when its disposition is `fixed`, *accepted*
when it is `wontfix`/`duplicate` with evidence, and *unresolved* otherwise — no disposition, or an
acceptance with blank evidence. The prompt and convergence read this one set. A review loop is not
converged, and not done at its cap either, while the carried set holds an unresolved row that blocks
under `review_converge_on` (critical, high or ungradable under `blocking`; any under `all`): the
roles are told not to re-report it, so their silence is not its disposition. `--status`, `--check`,
`--record`, the emission's `review_loop`, `tp import`, the budget refusal, `tp resume` and
`next_action` all read it. `--status` and `--record` then report `carried_blocking` (the count) and
`carried_blocking_rows` (the first 5, each `{round, file, index, severity, location, finding}`;
`file` is the round file `--resolve` takes), both absent when there is none and both kept under
`--compact`; `converged` and `done` are false. `next_action` names the first row's
`tp review <file> --resolve <index> fixed|wontfix|duplicate "<evidence>"`, the count of the rest, and
an epoch as the way out when the spec was rewritten. At the cap the refusal's hint leads with the
same clause.

**Dispositions.** `--resolve`/`--resolve-all` write `resolved: {status, evidence, resolved_at}` onto
rows in place. The loop verdict reads the recorded round files, so a resolve into a file no
`state.json` lists reports `recorded_round: false` and says so on stderr. A `wontfix`/`duplicate`
with blank evidence exits 2 in both phases. `tp audit --resolve <role:item_id>` disposes every
non-`PASS` row under the key and reports the count as `disposed`; `tp audit --resolve-all` skips
`PASS` rows. `tp audit --resolve` offers a re-record `next_step` only inside a driver round
(`TP_ROUND`, `TP_FILE`) for an exact copy of that recorded round, where the recorder rewrites it in
place.

**Audit acceptance across rounds.** The next audit prompt's Prior Round block carries each row's
`disposition` and `disposition_evidence` beside `status` and `changed_since`: an accepted row
(`wontfix`/`duplicate` with evidence) is to be recorded as before if it still stands, unless
`changed_since` is true; a `fixed` row asks for the repair to be verified. `tp audit <spec> --record`
then gives a new non-`PASS` row with no disposition of its own the accepted disposition its
`(role, item_id)` carried in the immediately prior round, while the prior row's `evidence_file` is
unchanged since that round was recorded. The basis is the parent of the commit that added the round
file (its oldest `--follow` add), so a commit landing in the same second as the record is not a
change; a record commit with no parent measures from the empty tree, where every tracked path reads
as changed and nothing carries. Where git tracks no commit adding the round file, or cannot answer,
the window is every commit since `recorded_at` (a row with no `evidence_file` carries until
re-decided).
The `resolved` block is copied with `carried_from` naming the round it was first decided in, the
payload reports the count as `carried` (present at `0`), and `fixed` is never carried.

The three ground artifacts are a separate prefix list: they carry no round entry in `state.json`, a directory holding only them loads cleanly rather than reading as review state with a missing index, and their round numbering is its own sequence. Their formats are in the Grounding section below.

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
- **Per-spec deactivation (v0.32.0):** `enabled: true` is accepted and is a no-op — it does not resurrect a role `domains` removed; a non-boolean value warns (`tp.<field>.<id>.enabled is not a boolean (got %T); ignored`) and leaves the role active; `enabled: null` or a valueless `enabled:` is unset. The deactivated role is named in `skipped_roles` with the reason `disabled-by-spec`, omitted under `--compact` like every other `skipped_roles` entry unless `--role` is passed. Two refusals fire **on prompt emission only** — never under `tp review --record`/`--status`/`--merge`/`--resolve`/`--resolve-all`/`--verify`/`--report`, `tp audit --record`/`--status`/`--merge`, nor `tp review --perspective`, which short-circuits before the corpus is resolved — and each exits 2 before any prompt is emitted or any state is written: the **empty-phase** refusal `every <phase> role is deactivated by this spec: <ids>` (phase rendered from `PhaseReviewers`/`PhaseAuditors`, ids sorted and comma-separated and naming only what this spec deactivated; hint `re-enable at least one role, or remove the enabled: false entries`), and the **`spec-coverage`** refusal `spec-coverage cannot be deactivated: it carries the entire spec-derived checklist` (hint `remove the enabled: false entry for spec-coverage`), which is reported first when a spec trips both. Toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` unchanged. An entry whose id matches no active role is ignored with a stderr warning that `--quiet` suppresses but JSON mode does **not**, so a typo'd role id stays visible in a piped, agent-driven run.
- The standalone `tp: lens` block is **retired** (see Role Corpus). A legacy `lens` with no new overrides auto-translates to review-role focus (`lens.all` → every review role except regression; `lens.<id>` → that role) with a deprecation warning; the new form wins when both are present.
- `tp lint --json` reports a `frontmatter` object `{present, lines, domain, lens_roles}`. An unclosed block and malformed YAML are lint errors; an unknown key directly under `tp:` (the known ones are `domain`, `lens`, `review_roles`, `audit_roles`), unknown lens keys, non-list values, disallowed override keys, and unknown override ids are lint warnings. `tp review` and `tp audit` print the same errors and warnings on stderr; on an error the frontmatter is ignored and the defaults apply.

## Role Corpus (v0.25.0)

Reviewer and auditor roles are project-owned JSON files under the repo-root `.tp/` (discovered via the git-boundary anchor, committed to VCS):

- `.tp/reviewers/*.json` — `tp review` roles; `.tp/auditors/*.json` — `tp audit` roles. The phase is inferred from the directory.
- Schema (one shared parser/validator): `id` (MUST equal the filename stem, `^[a-z0-9]+(-[a-z0-9]+)*$`, not the reserved `regression`), `title`, `instructions`, `focus` (string[], optional), `domains` (string[], optional — default: every domain), `context` (`neighbours`, `neighbours-brief` or `dependencies`, optional; read only by `tp epic review`, see Epic Graph), `units` (a non-empty list of `epics`, `gates`, `scope`, optional — default `epics` and `gates`; read only by `tp epic review`) and `purpose` (boolean, optional — default false; read only by `tp epic review`). Any other top-level key is a validation error — tp owns the finding output contract.
- The embedded default auditor prompts changed in v0.31.2 (language-neutral wording). Every eject writes an advisory to stderr — `note: these roles are starting points; rewrite their focus for your project's stack and conventions.` — suppressed by `--quiet` but not by JSON mode; the eject JSON payload keys stay exactly `ejected` and `domain`.

| Command / flag | Meaning |
|----------------|---------|
| `tp init --eject-roles` | Write the default corpus into `.tp/reviewers` and `.tp/auditors` (byte-identical to the embedded prompts) |
| `tp init --eject-roles --domain <name>` | Eject the corpus for a shipped domain (`software`, `prose`); an unknown domain is a usage error (exit 2) |
| `tp init --eject-roles --force` | Overwrite existing role files regardless of validity |

Validation: `tp lint` validates both phases, `tp review` validates reviewers, `tp audit` validates auditors. A malformed or invalid role file aborts the phase-reading command with exit 3 and a `repair or delete <path>` hint (a broken auditor never blocks review — phase independence).

**Overlap fields** (`tp review --merge`/`--report`/`--status`): findings cluster by `(location key, class)`, the location key being the first `§<n>(.<n>)*` token. Each merged finding carries `found_by` (count of distinct diversity reviewer roles) and `found_by_roles` (the sorted set, `regression` excluded). `overlap_report` is a per-role array of `{role, unique, shared, trim_candidate}` — `trim_candidate` is true when `unique == 0 && shared >= 1`.

**`location_clusters` (v0.35.0, `tp review --merge`/`--status`):** the same merged records cut by `location` instead of by `(location, class)`. Roles compose their class slugs independently, so two roles naming one defect almost never collide and the merged count reads as "more to do" — this array says how much of it is one place seen through several lenses. One entry `{location, roles[], severities[], count}` per location key **two or more roles** reported: `roles[]` is the distinct contributing roles sorted (`found_by_roles` where the merge attributed the record, its own `role` otherwise; `regression` and blank excluded), `severities[]` the distinct severities most-severe-first, and `count` the number of merged **records** at that location — not pre-merge rows, since `(location, class)` duplicates have already collapsed. A location every finding of which came from one role produces no entry; the key is always present and `[]` when nothing clusters. It is **reporting only** — convergence arithmetic, the stored `clean` flag and the `--status --check` exit code are untouched — and on `--status` it is recomputed from the latest recorded round at read time, never stored. Being explanatory it is omitted under `--compact` (§8.4).

**`inputs` (v0.35.0, both `tp review --merge` and `tp audit --merge`):** one entry per input file, in argument order — `{path, parsed, skipped}` — beside the existing `input_files` count. A line that is invalid JSON or lacks a required field is warned about on stderr, the warning ending in `<path>:<line>`, and counted in `skipped`; blank and whitespace-only lines are neither parsed nor skipped and never contribute to either number. An input with **at least one content line and zero parsed** makes the merge **exit 1**: the error names each such input with its reasons and lines (the missing fields, invalid JSON), and the hint carries the JSON-format advice only when a line was invalid JSON. A role that emitted a trailing comma per line has every line skipped, so without this the merged set would be silently short one role. A **zero-byte** file stays the documented way a role reports nothing found and keeps exiting 0, so a clean round is unaffected. The zero-parsed rule is identical in both merges, because an unattended driver reads the exit code alone. A line holding only `[]` is a role that found nothing: neither parsed nor skipped at either merge, and dropped before parsing at either `--record`.

**`conflicts` (`tp audit --merge`):** rows sharing `(role, item_id)` collapse only when their verdict — status, severity, disposition status — agrees. Otherwise every row is kept, and each group is listed as `{role, item_id, rows, statuses}` with a stderr warning. The key is absent when every group holds one verdict.

**`by_severity` (v0.37.0, `tp audit --merge`):** a breakdown of the round's **non-`PASS`** rows by severity, beside `by_status` and `by_role`. It is emitted whenever the round holds at least one non-`PASS` row and omitted on an all-`PASS` round — a property of the rows and of nothing else, so it appears under `audit_converge_on: all` exactly as it does under `blocking`, and `--merge` reads no policy (it takes NDJSON inputs and rejects a spec positional). It is decision-critical and **survives `--compact`**, unlike the audit `overlap_report`. Every row lands in a named bucket: `error`, `warning` and `info` key on themselves, and everything else — a string outside the enum, JSON `null`, an absent key, a value that is not a string — keys on **`unrecognised`**. That is the same population the clean predicate blocks on, so `error` + `unrecognised` sums the round's blocking rows and the two cannot drift apart; `""` is deliberately not a bucket. Under `blocking` this is how an agent reading `clean: true` sees what was accepted. The review-side `by_severity` on `tp review --merge` is a different, unconditional field over review severities.

**Acting on a `trim_candidate` (v0.32.0):** a flagged role has **two levers, and they are guarded differently** — pick by whether the role is worthless on this one spec or worthless everywhere.

- **`enabled: false` in the spec frontmatter** removes the role from **one** spec (`tp.review_roles`/`tp.audit_roles`, see Spec Frontmatter above). It is **guarded** by the two emission-only refusals: it cannot empty a phase (`every <phase> role is deactivated by this spec: <ids>`) and it cannot deactivate the `spec-coverage` auditor (`spec-coverage cannot be deactivated: it carries the entire spec-derived checklist`). Both exit 2 before any prompt is emitted or any state is written.
- **Deleting the role file** removes the role from **every** spec and is **not guarded at all** — including for `spec-coverage`: the refusal protects it from a per-spec `enabled: false`, but nothing protects it from file deletion. And deleting the phase's **last** role file removes nothing: an empty phase directory reads as unpopulated, so `ResolveActiveCorpus` falls back to the embedded default corpus and the phase emits the built-in panel instead of no roles. To thin a phase down to one role, delete the others and keep a file.

tp does **not** guard against deactivating a role that has open findings — no refusal, no warning, for either lever. `ReviewRoundClean` evaluates cleanliness per round over that round's rows, so a deactivated role's earlier open findings do not keep a later round from being clean. Resolve or re-triage them before you trim.

**Measuring whether a second model earns its cost (v0.34.0):** `overlap_report` credits a *role label*, and nothing ties that label to the lens axis — so the same instrument measures model diversity. Run one reviewer role on model A in one round and the same role, same prompt, on model B in the next, then read `unique`.

**Scope: rounds recorded at or after v1.1.0.** `--merge` skips a row missing any of `severity`, `finding`, `location` or `evidence`; `evidence` joins that required set in v1.1.0, and nothing before it asked a reviewer for one, so a round emitted earlier will as a rule carry none in any row. Feed `--merge` two such rounds and every content line of both is skipped: `overlap_report` comes back `[]`, and the zero-parsed rule under `inputs` above makes the merge **exit 1** naming both files, so step 3 has nothing left to read. The recipe measures two rounds you run under a v1.1.0-or-later binary; it cannot be run over an earlier corpus.

1. Keep each round's raw per-role NDJSON — the file the reviewer wrote, before `--merge`.
2. **Relabel one of the two runs' `role` before merging.** Attribution credits the role string, so two runs sharing one label collapse into a single contributor: merging an `implementer` round with a second `implementer` round reports one row whose `unique` is just the count of the two rounds' combined clusters, and the question is unanswerable. Stamping the second file's rows `implementer@<model>` keeps them apart. `role` is free-form attribution metadata — `--merge` requires `severity`, `finding`, `location` and `evidence`, and never checks `role` against the corpus.
3. `tp review --merge <round-a>.ndjson <round-b>.ndjson`, then read the two `overlap_report` rows.

`unique` is what only that model found, `shared` what both found, and `trim_candidate` on the second label is the answer *no* — the second model contributed nothing the first did not. It is a measurement of the two rounds merged, one role, and nothing more.

`--merge` is the read because `tp review --status`/`--report` compute `overlap_report` over the **latest recorded round only** and therefore can never span the two rounds. The relabelled `role` travels into whatever you `--record`, so record a copy with the corpus id restored if round history should keep it. No flag or config supports the recipe — it is the existing instrument read along a second axis.

**Transparency fields (v0.29.0, §9):** prompt emission (`tp review`/`tp audit`) reports `skipped_roles: [{role, reason}]` naming every corpus role it did not emit (`reason` ∈ `no-checklist-items` / `no-spec-change` / `domain-mismatch` / `no-baseline` / `disabled-by-spec`, plus `role-filter` for each emitted role `--role` narrowed away — under a driver (`TP_RUN_ID` set), only a prompt no sibling unit covers, which is none: every corpus role and, when the round carries it, review's `regression` is its own unit; `[]` when none skipped; `tp epic review` adds `no-units` for a graph role asked no unit of its kinds, Epic Graph). Merge/`--status` summary adds `attribution_excludes: ["regression"]` **only** when excluding the built-in `regression` role causes `merged_count` to exceed the overlap-report finding count (omitted otherwise). `tp audit --merge`/`--status` emit their own `overlap_report` over non-PASS rows clustered by `(item_id, category)` with the same `{role, unique, shared, trim_candidate}` shape.


**`--role` payload contract (v0.36.0).** `--role <name>` narrows a `tp review`/`tp audit` emission to
one role. Its exact form and the reasoning are in [SKILL.md](SKILL.md); what the payload does is:

| Name class | Exit | `prompts[]` | Other keys |
|---|---|---|---|
| in the emitted set | 0 | exactly one entry, **byte-identical** to that role's entry in the same invocation without the flag | unchanged, except `review_loop.instruction` (below) and the `role-filter` entries `skipped_roles` gains |
| recognised, not emitted this round | 0 | `[]` — an array, never `null` | when the name is one *this* phase skipped, `skipped_roles` carries its own reason. A name recognised only through the **other** phase's corpus appears nowhere in `skipped_roles` — `tp review <spec> --role spec-coverage` exits 0, and the array holds whatever else the round skipped (at round 1, `regression`/`no-baseline`) plus a `role-filter` entry for every role the round emitted. The five single-prompt modes (the four `--perspective` values and `--verify`) carry `skipped_roles` under these same rules — an entry for what `--role` narrowed away, `[]` when nothing was, and the panel's `--compact` rule — but never the driver rule above, since no unit kind runs `--perspective` or `--verify` |
| recognised nowhere | 2 | — | stderr carries `{"error":"unknown role: <name>","code":2,"hint":"this invocation emits: …"}`; the hint is built from the invocation's own emitted set plus `skipped_roles`, not from the corpus, because `regression` is emitted and belongs to no corpus. Nothing is written to the state directory |

Recognition spans the user corpus **and** the embedded default corpus for **both** phases, plus the
built-in `regression`. `tp review` never emits an auditor id, so a set built from one phase's
emission would make an auditor's own first command a usage error.

`review_loop.instruction` is the one key `--role` rewrites, and it has two rules. **An empty
`prompts[]` gets an empty key** — a payload with no prompt can support no directive at all, in every
mode. **A one-prompt payload gets a sentence-subset**, described next. Unrestricted the key is
addressed to a caller holding the whole panel (spawn a sub-agent per prompt, `--merge`, `--record`, `--status --check`,
the regression ordering, the uncounted delta pass); under `--role` it is a **sentence-subset** of
that string containing no directive a single-prompt payload cannot support. Nothing else moves:
`prompts[]`, `review_loop.instruction` and the `role-filter` entries in `skipped_roles` are the only
differences between a `--role` payload and the unrestricted one, `review_loop`'s other members
included.
**`--compact` disposition (§8.4):** decision-critical new fields survive `--compact` — `bookkeeping`, `suggested_files`, `max_rounds`/`rounds_remaining`/`in_flight_round`, `next_action`, `nonblocking_open` (review-only, emitted only on an accepted-open clean round), the audit `by_severity` (v0.37.0, emitted only on a round holding a non-`PASS` row), `spec_truncated` (emitted only when the spec was cut), `epochs` on both `--status` payloads (v1.14.0, emitted only once an epoch exists), and the audit's `notices`, `prompts[].file_count`/`file_pool` and `graded` (v1.15.0); explanatory fields are omitted — `skipped_roles`, `attribution_excludes`, `location_clusters`, the audit `overlap_report`, the report `note`, and the wrapper-drift diagnostics `harness_note`/`harness_stale`. **One exception:** under `--role`, `skipped_roles` survives `--compact` when it is non-empty or `prompts[]` is empty, because a narrowed payload is where the reason stops being explanatory — it says what the payload leaves out. `tp audit --compact` also omits `prompts[].checklist_items` and `prompts[].affected_files`: both duplicate content already rendered into `prompts[].prompt`, and together they are about a fifth of the audit payload (a 3-role/5-file run drops from ~24 KB to ~17 KB). Default output is unchanged — both stay arrays, never `null`.

**Role staleness** (`tp review --status`/`tp audit --status`): each recorded round stores `roles_hash` (`"builtin"` on the defaults, else a clone-stable sha256 over the phase's user files). `--status` reports `roles_stale` beside the spec `stale` flag; a pre-v0.25.0 round with no stored hash is treated as matching.

## Finding `class` and Report

**Required keys and attribution keys (v1.1.0).** Both gates require four keys of a review finding — `severity`, `finding`, `location` (a `§`-anchor) and `evidence` — and neither lets a row through without them: `tp review <spec> --record` refuses the file, naming every offending line in one exit; `tp review --merge` skips the row, warns on stderr and counts it as skipped. `role` and `class` are attribution and cluster keys, and **no gate requires either** — a row carrying neither records at exit 0, with one stderr advisory about the absent `role` and the class-less finding clustered on its own. Until this release this contract read `role`, `location`, `class` and `severity`, which disagreed with the gates in both directions at once: it named two keys neither gate checked, and omitted `finding`, which `--merge` required at `v1.0.1` (`internal/cli/review_merge.go:192`). That was already true before `evidence` joined the set.

Finding text and `class` slugs are written in **English** whatever language the spec is in — the rows are committed artifacts and `--merge` clusters slugs byte-for-byte, so a second language splits one idea into two families; see "Recording language" in `SKILL.md`. The `class` field is free-form (a kebab-case slug, the dedup/cluster key), and an arbitrary class is carried through `--merge`/`--resolve` untouched. `tp review --merge` clusters by `(location key, class)` — a finding with no class is its own cluster — and emits each cluster's representative (highest severity, then lexicographic role, then finding text) annotated with `found_by`/`found_by_roles`. `tp review --report` adds a `by_class` breakdown, the per-role `overlap_report`, and `mechanize_candidates` (a class in ≥ 2 distinct rounds OR ≥ 5 times in one round), sorted by total descending, ties alphabetical.

**Whitespace, exactly (v1.1.0).** Both gates decide *empty* after `strings.TrimSpace`, so the whitespace this release means is the set `unicode.IsSpace` defines — tab, newline, vertical tab, form feed, carriage return and space, plus U+0085, U+00A0, U+1680, U+2000–U+200A, U+2028, U+2029, U+202F, U+205F and U+3000 — and nothing outside it. Every other character is content, the zero-width ones included: a one-row file whose `evidence` is a single U+00A0 makes `--record` exit 1 naming line 1, and the same file with U+200B instead records at exit 0.

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
| `prompts[].file_count`, `prompts[].file_pool` | absent | v1.15.0: listed files, and the pool they were cut from |
| `notices` | absent | v1.15.0: `[]{kind, message, files?}`, omitted when empty |

Item ids are deterministic: `table-<t>-<r>`, `list-<l>-<n>`, `task-<id>`, `file-<role-id>-<slug>.<digest>` (the slug derives from the file path plus the item text, cut to 40 characters, and the digest is the first 16 hex characters of the sha256 of the cleaned path, so an id is a function of the role and the path alone; `--affected-files` paths are cleaned, made repo-relative and deduped first. A prior round's ids of the earlier form, with no digest, name no item of this round, and the prompt lists them apart to be matched by `evidence_file`), `finding-<n>`. Sub-agents return one NDJSON row per checklist item: `{item_id, status(PASS|PARTIAL|FAIL), evidence_file, evidence_lines, category, severity, notes, class?}`. `category`/`severity` are `null` for PASS and one of the enum values for PARTIAL/FAIL. Finding category enum: `security > concurrency > error-handling > correctness > contract` (resolution precedence when several apply — the auditor picks one, tp does not resolve for it). **`tp audit --record` enforces the enum (v0.34.1):** a row carrying anything else aborts the record at exit 1, naming every offending line at once and listing the five, and no round file is written. A row with no `category`, which is every PASS row, is untouched.

**`file_summary` (v0.35.0):** `{total_files, total_lines, chars_included, truncated, total_changed}`. Auto-detect caps the audited set at **50** files. `total_files` stays the **audited** count, `total_changed` is the **pre-cap** count, and `truncated` is true when that cap bit or, from v1.15.0, when a role's cap cut the list a prompt carries (a `*-files-cut` notice below). The stderr notice names both numbers — `63 files changed, auditing first 50 — name the rest with --affected-files` — but the flag lives in the payload, so `--quiet`, which erases that line, cannot hide the truncation from a driver. `--compact` drops `file_summary` in its entirety, and the flag with it; the `notices` array and each prompt's `file_count`/`file_pool` survive it.

**`spec_truncated`:** `{path, kept_bytes, total_bytes}`, present only when the spec a role reads
inline was cut at `engine.SpecContentCap` (10,000 bytes) — at the top level of the `tp audit`
payload (the spec-coverage excerpt) and of the `tp review` payload in every `--spec-inline` mode (the
panel, `--verify`, and the `code-audit`, `documentation` and `testing` perspectives). The cut lands
on a rune boundary, the prompt's marker names the same counts, and the key survives `--compact`.

**File lists, cuts and what a round graded (v1.15.0).**

- **Order.** The code roles share one list: files whose path holds `lock`, `validate`, `auth`,
  `secret` or `perm` first, then the rest, each group by churn — lines added plus deleted over the
  audit's own ranges, descending, ties by path. The churn is read with `-c diff.external=
  --no-ext-diff --numstat -M -z`, so a renamed file ranks by its own edit under its new path; a
  file with no measured diff, a binary's `-` included, comes last in path order. Every path list the
  audit reads from git is NUL-separated (`-z`), so a path git would C-quote under `core.quotePath`
  (non-ASCII, a tab) matches its churn. `CodeFileCap` (10) then cuts the list; spec-coverage keeps
  its own order and `AuditFileCap` (20).
- **A named list is not cut.** When `--affected-files` or `--affected-from-tasks` replaced the
  universe, every code role gets the whole list, and its prompt's heading reads
  `## Affected Files (all <n> named)`. The other headings stay: `(max <cap>)` when nothing was cut,
  `(<n> of <pool>)` when the cap bit.
- **`prompts[].file_count` and `prompts[].file_pool`.** How many files the prompt lists and how
  many its list was cut from; equal when nothing was cut. The pool is counted after the 50-file
  auto-detect cap, which the `auto-detect-cap` notice reports on its own. Both survive `--compact`,
  which drops `affected_files`, so a long named list shows its cost before fan-out.
- **`notices`.** Every cut and universe notice of the round, repeated from stderr as
  `[{kind, message, files?, …}]` and omitted when there are none. It survives `--compact` and
  `--quiet`, and it is taken from the round's prompts before `--role`, so every sibling unit carries
  the same cuts, as `file_summary.truncated` does. The kinds are a closed set:
  - `auto-detect-cap`: auto-detection kept the first 50 of the changed files; `total_changed` is
    the pre-cap count.
  - `code-files-cut`: the code roles' list is cut below the auto-detect cap.
  - `spec-coverage-files-cut`: spec-coverage's own list is cut at 20.
  - `merge-only-files`: under auto-detection, with a committed range (`--base` or the latest tag), a
    file is in `<rev>...HEAD` but no first-parent, non-merge commit of `<rev>..HEAD` changed it, so
    `git log --first-parent` sees fewer files than the audit. Only the committed range is compared;
    a working-tree edit does not take a file out. `merge_only` is the count, `files` names up to 20,
    and stderr names five.
  - `tasks-differ`: under `--affected-from-tasks`, the task-commit files and the auto-detected
    diff (auditable files, no cap) differ. `untasked` counts the changed files no task touched.
    `outside` counts the task files outside the diff, and is present only when a tag gives the
    diff a committed range — without one the diff is the working tree, which every committed task
    file lies outside. `files` names up to 20 of each side, untasked first.
- **What a round graded.** At emission tp keeps each code role's `file_pool` and the `file_check`
  item ids its prompt listed in `audit-inbox/round-<N>.pools.json`, written atomically and
  identically by every sibling unit. At `--record` the round's entry in `state.json` gains
  `graded: [{role, graded, pool}]`. `graded` counts the distinct listed ids the role returned a row
  for, whatever the verdict, so a file read and found clean counts and an id the round never emitted
  does not; it never exceeds `file_count`. `pool` is the prompt's `file_pool`. The pools file is
  removed with the round's role files; a re-record of the same round (`TP_ROUND`) finds none and
  keeps the `graded` already stored. Only a round whose `hash_scheme` is `snapshot` is graded, so a
  round recorded with no emission carries no `graded`, whatever sits in the inbox. `--record`
  reports the same array, and so does each `audit_rounds[]` entry of `--status`. Both
  `next_action`s open with `latest round graded <role> <n> of <m>, …. ` and end with the step
  unchanged, so the command is still the end of the string.

## Grounding (`tp ground`, v1.0.0)

`tp ground` checks a spec's claims against the world before review is told they hold. The loop and
its place in Workflow A are in [SKILL.md](SKILL.md); this section is the record's schema, the enums
`--record` validates against, and what each mode returns.

### What each mode returns

| mode | stdout | shape |
|------|--------|-------|
| `tp ground <spec>` | JSON | `{spec, round, snapshot, floor, output_path, floor_size, carried, prompt}` — one `prompt` **string**, not review's `prompts[]` array. Grounding asks one question of every unit, so there is no panel, no role and no `--role`. `floor_size` is the **floor, not the ask**: the ask is the difference, `floor_size - carried`, and the prompt states it in words. `output_path` is `<spec dir>/.tp-review/<base>/ground-inbox/round-<N>.ndjson` (v1.12.0): in the spec's own state directory, so same-base specs in different directories never share it, and apart from the recorded `ground-round-<N>.ndjson`. `--record` removes it once the round is on disk, and exits 3 when pointed at the pre-1.12.0 name `ground-<base>-r<N>.ndjson` in the working directory, naming the path to move it to; that name is never read, since a file there may be another spec's. The plugin's `PreToolUse` fence lets exactly this name through, as it does the epic review inbox |
| `--units` | **plain text** | one line per floor unit, `<unit_id>\t<text_sha>\t<text>`, in emission order. It writes nothing and emits no round, and it derives from the spec as it stands rather than from a snapshot — which is why every line carries its own hash: a spec edited since the emission disagrees with the index visibly |
| `--record <file>` | JSON | `{spec, round, floor, file, rows, carried}` — `rows` counts the payload's rows, `carried` the dispositions the carry brought forward |
| `--status` | JSON | `{spec, round, emitted, dispositioned, reader_added, off_floor, cut, by_verdict}` for the latest **emitted** round. `emitted` is the same quantity `tp review`'s `ungrounded.floor_size` reports — one number, two names, and the `ungrounded` section below says where they meet |
| `--status --check` | the same JSON | exit 0 when `dispositioned == emitted`, the floor is not one the arms emptied, and `by_verdict` counts no `FAIL`; 1 otherwise. Every condition reads a key the payload prints, so the exit code is reconstructible from it |

A round exists from its **emission**, not from its record, so `--status` reports a round nobody has
come back to as wholly undispositioned rather than reporting the previous round's completeness. N is
the highest existing `ground-round-<N>` plus one and a gap is preserved (rounds 1 and 3 present
yields 4, never 2), so re-emitting before recording reuses that round's number: idempotent while
the spec is unchanged, exit 3 when it changed, unless `--force` discards the emission.

**`carried` is the only machine-readable sign of a carry that FAILED.** A preceding round tp cannot
read back does not refuse the emission: it emits at exit 0 with an envelope of the same shape, asking
for every unit, and the line saying so is an `output.Notice` on stderr — which `--quiet` removes.
`carried: 0` beside a floor that did not move is the fact that survives. `--record`'s envelope
carries the same pair as two counts rather than one sum: `rows` is what the operator handed in and
can check against their own file, `carried` is what tp added, and folding them reports a number that
matches neither artifact.

**Three kinds of line reach stderr, and a driver must expect all of them.** `output.Notice` writes
plain text and `--quiet` suppresses it; `output.Error` writes the failure envelope, `{error, code,
hint?}` under `--json`; and `tp review --merge … -o -` / `tp audit --merge … -o -` write their
summary there, a pretty-printed JSON object over several lines (v1.7.0), ahead of any envelope.
`output.Error` is terminal — checked over the whole tree at `93d89673`, every non-test call site is
within a few lines of an `os.Exit`, with no exceptions — so when an envelope appears it is stderr's
**last** line. Parse the last line for `code`; read anything above it as notices or that summary.

`by_verdict` carries all six verdicts, zeros included, and counts **every row the round recorded** —
so it is larger than `dispositioned` alone, which counts only floor units. **The two sides count
different things and are not an identity: `by_verdict` counts ROWS, `dispositioned` counts UNITS.**
Nothing stops two rows from naming the same floor unit, and then the totals separate. Measured
against a control: a round of two rows naming two floor units records `dispositioned 2` and a
`by_verdict` total of 2; the same round with a third row naming the first unit again records at exit
0 with `dispositioned 2` and a `by_verdict` total of **3**, `reader_added` and `off_floor` both 0.
So read the sum as
`dispositioned` plus `reader_added` plus `off_floor` **plus any repeat rows**, or do not sum it at
all — each verdict's own count is what it is for.
`reader_added` counts rows whose `unit_id` is `null`; `off_floor` counts rows naming a unit the index
emitted as `(cut)`. Neither moves either side of the ratio, and they are two counts rather than one
because they are evidence about different halves of the derivation: *the arms never produced this
unit* against *the arms produced it and then cut it*.

**`cut` is the key `--check`'s empty-floor condition reads, and it is why exit 1 is not always undercoverage.**
It counts the units §2.1 produced that the arms dropped, and it is no part of the ratio — a cut unit
owes no disposition. A floor of no units makes `dispositioned < emitted` read `0 < 0`, which is
false, so coverage alone certifies a document nobody checked. `cut` is what tells the two zeros
apart: `emitted: 0, cut: 0` is a document §2.1 produced nothing from and exits **0**;
`emitted: 0, cut: N` is a document whose every sentence the arms dropped and exits **1**. A stderr
notice names the floor file where those units are listed, but the notice is an `output.Notice` and
`--quiet` removes it — the key is what survives, on the same reading as `carried` above and audit's
`file_summary.truncated`. `cut` is present at zero rather than omitted, so a driver never has to
decide what an absence stands for.

`tp review`'s `ungrounded` advisory is **absent** on that state and does not contradict it: no unit is
undispositioned, so there is nothing for the advisory's own rule to report. The two answer different
questions — *which units has this round not come back to* against *did this round check anything* —
and `--status --check` is where the second one is answered.

### The recorded row

`ground-round-N.ndjson` holds one JSON object per line — the payload's own bytes, never a
re-serialisation, so a key the table below does not name survives the record instead of being
dropped. Blank lines are skipped; every other line is a row. **An unknown top-level key is rejected**
(`field "carried_form": is not a field a ground row takes`), and `null` fails every cell but
`unit_id` and `note`.

| field | type | required | meaning |
|-------|------|----------|---------|
| `unit_id` | string \| null | the **key** always | `u<N>` copied from the index row. `null` for a claim the reader added; a missing key is an error, because absence and `null` must differ here and nowhere else |
| `anchor` | string | yes | the `§n(.n)*` heading the unit sits under, `§0` before the first one |
| `text_sha` | string | yes | the index's hash for that unit — the first 12 lowercase hex characters of the sha256 of the canonical unit text, UTF-8 encoded. A reader-added row supplies its own |
| `ordinal` | integer | yes | the 1-based index of this unit among those sharing its `text_sha`, in emission order; `1` when the hash is unique. The carry joins on `(text_sha, ordinal)`, so neither half is optional |
| `verdict` | enum | yes | one of the six below |
| `kind` | enum | unless `NOT-A-CLAIM` | one of the seven below. Optional on `NOT-A-CLAIM`, where a unit may state a decision *and* a checked fact — and optional per cell, so `kind` may be carried there without `tier` |
| `tier` | enum | unless `NOT-A-CLAIM` | one of the six below — **the deepest tier actually reached**, never the tier the kind requires |
| `evidence` | string | iff `tier` is present | free text: the command that was run, or the artifact and line that was read. Deliberately unstructured, because `tier` is the structured field and `evidence` is what a later reader re-runs |
| `partial_kind` | enum | iff verdict is `PARTIAL` | `two-readings`, `reason-not-conclusion`, `true-when-written` |
| `held_at` | string | iff `partial_kind` is `true-when-written` | the commit, tag or date at which the claim held |
| `causes` | array | iff verdict is `QUESTION` | three to five `{cause, prediction}` objects, ranked. Both halves are required in **every** entry, and a count of 2 or of 6 is rejected |
| `carried_from` | integer | no | the round the disposition was **first** decided in, which is not in general the preceding round. Written by the carry when it is absent — but **descriptive**, not owned: a row that supplies its own value keeps it, and the carry does not overwrite it. The type and a 1-based floor are enforced; the value's relation to its round is not (§7.2) |
| `note` | string | no | free text |

**Validation is atomic.** The whole payload is parsed before anything is opened, created or
truncated, so one bad row writes no round file and leaves the state directory exactly as the emission
made it. The message names the field and the **file's** line number — blank lines counted, so the
number indexes the file an operator opens.

### Verdicts

| verdict | means | and demands |
|---------|-------|-------------|
| `PASS` | evidence at a tier acceptable for the kind supports the claim | nothing |
| `PARTIAL` | true under one reading and not another, or the conclusion holds while the stated reason does not, or it was true when written | `partial_kind` says which |
| `FAIL` | evidence at an acceptable tier contradicts it | repair the claim |
| `UNVERIFIABLE` | no acceptable tier is reachable at all | `tier` records the deepest one attempted |
| `QUESTION` | the attempt did not settle it — either the result was unclear at an acceptable tier, or the tier reached is not one the kind accepts | `causes` |
| `NOT-A-CLAIM` | the unit is a decision, a prediction, or prose carrying no assertion about the world | nothing |

`NOT-A-CLAIM` is a verdict rather than an omission: coverage obliges a disposition for every floor
unit, so without a recordable value for a non-claim every spec would be permanently uncovered.
`UNVERIFIABLE` counts as dispositioned — it is a settled answer, not a gap.

### Kinds and tiers

**The tiers are not ordered.** A tier is acceptable for a kind or it is not; the sets are the whole
rule. Past `run`, a "deeper" tier is not more of the same evidence but evidence about a *different
subject* — `probe`, `red-green` and `break-and-control` rank rigour on an artifact the unit built,
while `read`, `query` and `run` examine the real one.

| `kind` | the claim is about | acceptable `tier`s |
|--------|--------------------|--------------------|
| `document` | what a document says | `read` |
| `code-structure` | how code is structured | `read`, `query` |
| `corpus` | what the corpus contains | `query` |
| `behaviour` | existing behaviour | `run`, `red-green` |
| `mechanism` | whether a proposed mechanism works | `probe` |
| `defect` | whether a defect is real | `red-green` |
| `guard` | whether a guard measures anything | `break-and-control` |

| `tier` | what was done |
|--------|---------------|
| `read` | read the artifact |
| `query` | ran a read-only command over the corpus (a search, a query, a listing) |
| `run` | ran the shipped command |
| `probe` | built a probe and ran it |
| `red-green` | wrote the test, watched it red, fixed, watched it green |
| `break-and-control` | broke the subject, ran the suite, **then** ran the control |

Acceptability is enforced **per verdict**, at record:

| the row says | and the rule reads |
|--------------|--------------------|
| `PASS`, `PARTIAL` or `FAIL` | `tier` **must** be acceptable for `kind`, or the row is rejected naming `tier` and listing the kind's accepted tiers (`"read" says nothing about a "behaviour" claim, and a PASS row must be reached at a tier that does: run, red-green`) |
| `QUESTION` | **no constraint** — either relation is legal, and which one holds *is* the question's shape: an acceptable tier means the unit got there and the result did not settle it, an unacceptable one means it did not get there |
| `UNVERIFIABLE` | no constraint — `tier` names the deepest tier attempted, and the point is that none is reachable |

A row carrying no `kind` on a claim verdict is rejected naming `kind`, before the tier rule runs.

### Exit codes

| code | outcome |
|------|---------|
| 0 | the invocation completed |
| 1 | a row failed validation, **or** the round would record no row at all — the payload's rows plus the carried rows both zero. An empty payload whose every unit carries is not this case and exits 0 |
| 2 | usage: `--check` without `--status`, two of `--units`/`--status`/`--record` together, `--record` with no path argument, `--force` on a mode other than the emission, or a discarding `--force` under `TP_UNATTENDED` |
| 3 | file or corrupt state: an emission over an unrecorded round whose spec changed (without `--force`), a `--record` path that does not exist, a `--record` path that is the pre-1.12.0 scratch name `ground-<base>-r<N>.ndjson` in the working directory (never read; the hint names `.tp-review/<base>/ground-inbox/round-<N>.ndjson` to move it to), and for `tp review` and `tp audit` too: a tree mode whose task file (or spec) belongs to another project (the cross-tree refusal, naming the directory to run from), a `.tp/config.json` of that project that cannot be read or parsed, and `--merge` of an input named in the pre-1.12.0 role-file shape `<phase>-<base>-r<N>-<role>.ndjson` whose `<role>` is a role tp knows (the hint names the inbox file to move it to; any other name in that shape merges as before), no prior emission for the round, an unreadable preceding round, or a state directory holding an unparseable `state.json`. An unreadable preceding round is a **notice at emit** (exit 0) and a refusal at `--record` — the round is decided at the record |
| 4 | the write lock could not be taken within `lock_timeout_seconds` |

The lock's target is the **spec path**, not the state file: `.tp/locks/<spec-file>-<hash>.lock`,
where every other tp lock lives — and that `.tp/` is the **project's**, found by walking up from the
spec, not a `.tp/` beside it. Running `tp ground spec/1.0.0.md --record` in a checkout of this
repository put the lock at `.tp/locks/1.0.0.md-<hash>.lock` and created no `spec/.tp` at all. Only
when the walk finds neither a `.tp/` nor a git root does it fall back to the spec's own directory,
`<spec-dir>/.tp/locks/…` — the shape a bare fixture sees, and the one to expect in a temp dir.
`--record` reads the round number from the state directory and
writes the round file into it, so both are inside one lock and two concurrent records cannot compute
the same N.

### Files, and what a ground round must not disturb

| File | Written by | Content |
|------|------------|---------|
| `snapshot-ground-round-<N>.md` | emission | the spec's bytes as that round read them |
| `floor-ground-round-<N>.txt` | emission | the index derived from those bytes: one row per unit, `<unit_id> <anchor> <text_sha> #<ordinal> <bytes>B`, or `<unit_id> <anchor> (cut)` for a unit the arms dropped |
| `ground-round-<N>.ndjson` | `--record` | the round's rows, payload bytes plus the carried ones |

`--record` reads the **frozen floor**, never the spec, so a spec edited between emit and record does
not re-floor the round. The `(carried)` marking a round-2 prompt shows lives in the prompt alone —
the floor on disk is the unmarked index the round is graded against.

A ground round writes **nothing** into `state.json`: it creates none, and leaves an existing one
byte-identical. Grounding is discovered by filename through its own prefix list, so a directory
holding only a ground round and its snapshot loads cleanly rather than reading as review state with a
missing index. A ground round also runs on a spec with no state directory and creates it — which is
the ordering this release advocates, grounding before review.

### `ungrounded` — the advisory in `tp review`

`tp review`'s top-level envelope carries `ungrounded`: `{round, undispositioned, floor_size}` for the
latest **emitted** ground round. The key is **absent** when every floor unit is dispositioned and
when no ground round exists, on `divergence`'s precedent that a permanently zero-valued key is one
every reader learns to skip. It is emitted once in the envelope rather than per role, it survives
`--compact`, and **review's exit code is identical with and without it** — the advisory tells, it
does not stop.

`floor_size` and `tp ground --status`'s `emitted` are the **same quantity under two names**, so a
driver reading both must not treat them as two facts: `internal/engine/groundadvisory.go` builds the
advisory as `FloorSize: cov.Emitted` from the very `GroundCoverageOf` result `--status` reports. Run
on a checkout of this repository with two of `spec/1.0.0.md`'s floor units dispositioned, `--status`'s
`emitted` and the same tree's `ungrounded.floor_size` were the same number, and `undispositioned` was
that number less 2. **The number itself is deliberately not written here** — the floor moves with
every edit to the spec, and it moved by one while this paragraph was being checked. The identity is
the assignment above, not the value.

### `floor_size` — one name, two quantities

Three of tp's payloads carry a key spelled `floor_size`, and a search for `json:"floor_size"` under
`internal/` returns exactly those three: `tp lint`'s (`internal/cli/lint.go`), `tp ground <spec>`'s
emission envelope (`internal/cli/ground.go`), and `tp review`'s `ungrounded`
(`internal/engine/groundadvisory.go`). All three count the same kind of thing — **uncut floor
units** — so what separates them is not the arithmetic but the *text* it was counted over.

- **`tp lint <spec>`** counts the spec **as it stands on disk**. Lint has no round and reads no
  snapshot; its `floor_size` is the line count of `tp ground <spec> --units` on the same file —
  **while that file carries no frontmatter**. Lint counts `parseSpecFile`'s frontmatter-blanked
  text and `--units` counts the raw bytes, so a frontmatter block can become a floor unit of its
  own, depending on its content.
- **`tp review <spec>`'s `ungrounded.floor_size`** counts the **latest emitted round's frozen
  index**, whatever the spec says now — the same number `--status` prints as `emitted`, by the
  identity above.
- **`tp ground <spec>`'s own `floor_size`** is where the two coincide by construction: the emission
  counts the live text and freezes that very index, so at the moment it prints it is both.

`cut` splits the same way: lint's is over the live text, `--status`'s over the frozen index.

**The two agree while the spec's text has not changed since the latest emission — a condition that is
sufficient, not necessary.** Measured on `spec/1.0.1.md` at `492a0691`, whose latest emission was
round 3: `tp lint spec/1.0.1.md` reported `floor_size: 96, cut: 43` while
`tp ground spec/1.0.1.md --status` reported `emitted: 54, cut: 28` — the spec had been edited many
times since round 3 froze its index. Emitting round 4 from that same unedited text, in an `rsync` copy
of the tree, moved `--status` to `emitted: 96, cut: 43` and closed the gap.

Equal numbers are **not** evidence that the text is unchanged. On a throwaway spec whose round 1 froze
three units, deleting a section added after the emission and rewording a sentence inside a surviving
unit left `tp lint` at `floor_size: 3`, `--status` at `emitted: 3` and `ungrounded.floor_size` at `3`,
while that unit's live `text_sha` read `1638df7a4a1f` against the frozen index's `982cda2bc138`. The
per-unit hashes `--units` prints are what distinguish a matching count from matching text; the counts
alone cannot.

### `review_panel` — the round-1 panel without emitting a round

`tp lint <spec>` also reports **`review_panel`**: the reviewer role ids a round-1
`tp review <spec>` emission of the same spec would carry, with no `--diff-from`. It is a **list, not
a count**, because a count cannot say *which* role is missing.

It comes from the same resolver the emission uses — `engine.ResolveRolePanel`, the pure half of the
split described above — so lint derives nothing a second time. What lint does not take is the
wrapper: the refusals and the notice loop stay with `tp review`, which is why the two commands
disagree on exactly one input.

| the spec's frontmatter | `tp lint` | `tp review` |
|---|---|---|
| nothing, or a partial `tp.review_roles` override | the surviving role ids | the same ids, as prompts |
| **every** reviewer `enabled: false` | `review_panel: []`, exit **0** | exit **2**, no prompts |

Measured on a scratch corpus holding this repository's four reviewers: deactivating `architect`
alone leaves lint reporting `["ax-economist", "implementer", "tester"]`; deactivating all four leaves
it reporting `[]` at exit 0 while `tp review` on the same tree exits 2. **Lint describes a spec; it
does not refuse one** — the empty list is the answer, not an error.

The list is always a JSON array, never `null`: an empty panel serialises as `[]`.

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
- `tp init` creates `.tp/.gitignore` (it covers `local.json`, `locks/`, `tp run`'s artifacts: `run-*.json`, `runs/`, `rounds/`, `last_failure-*.json`, and since v1.7.0 the epic review inboxes, `epic-review/*/inbox/`), and since v0.33.0 so does any locked write, so a project that never ran `tp init` does not accumulate an untracked `.tp/locks/`. Entries you add yourself are preserved: the file is only written when one of the required lines is missing.
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
undercounted round. Blank and whitespace-only lines, and a line holding only `[]`, are
neither parsed nor skipped and never trigger it. A **zero-byte file
stays the way a role reports nothing found** and keeps exiting 0 — so a clean round is unaffected.
Both exit-1 paths still emit
the full payload — the surviving roles merge, only the exit code changes, and the accounting an
unattended driver reads reaches stdout either way — and both decline the `-o` write: a refused merge
creates no file at the `-o` path and leaves a file already there byte-identical, so it cannot be
walked into `--record` as a round.

**`-o -` (v1.7.0)** writes the merged NDJSON to stdout and the summary — the payload `-o <file>`
prints on stdout, `output_path: "stdout"` — to stderr as JSON, whatever the terminal, so stdout holds
rows alone. A merge that exits non-zero writes nothing to stdout and its summary, without
`output_path`, still reaches stderr ahead of the failure envelope. It is for reading: an empty stream
is how a converged round reports itself, so a refused merge piped into `--record` would record a
clean round; `-o <file>` stays the only way to feed `--record`.

`tp review --perspective code-audit --findings <file>` exits **2**: that perspective never reads the
file, and previously accepted the flag while reporting `previous_findings: 0` about it. A
spec-looking (`.md`) positional handed to `--merge` or `--resolve` exits **2** as well: those modes
take NDJSON inputs only.

### Report accuracy (§14)

`actual_minutes` rounds to one decimal place. When it rounds to `0.0`, `accuracy` is `null` and the task carries `note: "duration below resolution"` (omitted under `--compact`). The summary excludes null-accuracy tasks from `estimation_accuracy` and reports the count via `excluded_from_accuracy`.

### Lint: `empty-section` (§15)

`empty-section` no longer fires on a **container** heading — one whose next heading is deeper than itself. An empty **leaf** heading (no content, no deeper child) remains an error.

## Honest Convergence Signals (v0.33.0)

Four fields report *what* an audit or review round found without changing what the gate counts:
adding them in v0.33.0 moved no gate, and nothing reads them to decide one today. What the audit
gate counts is instead governed by `audit_converge_on` (v0.37.0), which changes what the stored
per-round `clean` flag records — and with it `engine.Converged`, `engine.ConsecutiveClean`, the exit
code of `tp audit --status --check` and `next_action`'s three-state audit precedence, each derived
from that flag.

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
| `open` | int | that role's open findings in the latest recorded round: non-PASS rows not accepted `wontfix`/`duplicate` with evidence |

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
spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Whether they gate this release is the operator's decision, not the agent's — surface it rather than deciding either way; audit convergence counts every non-PASS row the resolved policy does not accept.
```

It names the operator rather than issuing a bare imperative: the reader is usually an agent, and
accepting open findings is a user-approved decision. Whenever `divergence` is emitted over state tp
itself recorded, `next_action` names **another audit round** and never the terminal
proceed-to-release — condition 4 is what guarantees that. Which round-naming form it takes depends on
`audit_converge_on`: under `all`, condition 2's rows also make the latest round unclean, so
`next_action` reads the fix-and-re-audit directive; under `blocking`, a round whose open rows are all
advisory is stamped clean, and `next_action` reads the carried-forward form naming the accepted count
(`N accepted rows carried forward — run the next audit round: …`). Either way `next_action` names the
only step tp can verify, and `hint` names a decision tp cannot execute or record.

### `mechanized_classes` (review, §3.3)

Emitted on `tp review <spec> --record <file>` and nowhere else — not on `tp review --status` (which
carries no candidate array for it to explain) and not on `tp review --report`.

Type: an array of strings.
mechanized_classes names the candidate classes withheld because they are mechanized, and is [] when none were.
Each class appears once and the array is sorted ascending;
every member equals a valid `checks` entry's `class` and is therefore lowercase kebab-case, so byte
order and a case-insensitive sort cannot differ. It lists the round's mechanize candidates whose
registered check ran in this `--record` (exit 0 or 1), not the whole registered set (read that from
`tp config` or a bare `tp review <spec> --status`); a check that could not run, or never started,
withholds nothing.

`--record` runs the registered checks only when their result can change its output — a candidate
class has a registered check, or the loop is done or at its cap — and then carries
`mechanical_checks` in the `--status --check` shape (`[]` when no task file resolves); a record that
ran none carries no `mechanical_checks` key.

```json
"mechanize_candidates": [],
"mechanized_classes": ["test-inventory"]
```

It is always an array on the output that carries it — `[]` when nothing was withheld, never `null`
and never an omitted key — and the filtered `mechanize_candidates` beside it keeps that shape too.
The same filtering applies to all three of the record path's sinks: the emitted
`mechanize_candidates` array, the register-a-check hint, and `next_action`'s mechanize branch, so a
registered check that ran retires its mechanize candidate and the class is named here instead of
vanishing. A candidate whose check did not run stays a candidate, and `next_action` names that check
to fix instead of a check to register (`fix the registered "<class>" check — it exited 2 and verified
nothing`). At done or at the cap, the first check that failed (`fix what the registered "<class>"
check reports — it exited 1: violations found`) or did not run replaces every import step; SKILL.md's
`next_action` section has the precedence.

### `--compact`

role_streaks, spec_coverage_clean_rounds and divergence all survive --compact, divergence with every field.
That includes `message`, `hint`, and the `spec_coverage_clean_rounds` key when its value is
`null`. `mechanized_classes` survives it too, because `mechanize_candidates` always has. `--compact`
is the mode an agent driver runs in, and these fields are the conclusion rather than a restatement
of it. The audit fields `--compact` already drops — `harness_stale`, `harness_note`,
`overlap_report` — are unchanged.

## Epic Graph (v1.3.0)

The commands that read and write it are the `tp epic` rows of the inventory in [SKILL.md](SKILL.md);
the loop that uses them is SKILL.md's Workflow E.

### The graph file (`.tp/epics.json`)

The file lives in the project layer — the `.tp/` directory at the `.git` boundary, beside
`config.json` — and is committed like it. The plugin's write-deny hook fences it with `config.json`,
so it changes only through the `tp epic` writers.

```json
{
  "version": 1,
  "milestones": [["A", "B", "C"], ["L1", "L2"]],
  "opens_after": {"C": ["A"], "L2": []},
  "gates": {"A": {"members": ["money-type", "rounding"], "deferrable": ["ledger-export"]}},
  "tag_checks": {"ui": {"provided_by": "ui-rule-checker"}},
  "epics": [
    {"id": "money-type", "title": "Money type", "milestone": "A", "tags": ["det"],
     "depends_on": [{"id": "event-store", "why": "amounts are event payload"}],
     "acceptance": "…"}
  ]
}
```

| Key | Shape | Meaning |
|-----|-------|---------|
| `milestones` | list of tracks, each an ordered list of names | a name appears once across all tracks; order matters within a track only |
| `opens_after.<milestone>` | list of milestone names, `[]` allowed (v1.6.0) | the milestones it opens after, of any track; absent, the previous milestone of its track (none for a track's first); written after `milestones`, omitted when empty; `tp epic set opens_after=` replaces the whole map |
| `gates.<milestone>` | `{members, deferrable, threshold, purpose, met}` | the milestone's **exit** gate, reviewed with it: keyed by the milestone whose epics it lists, not the one it opens; every member must be in that milestone. Every milestone has a gate unit in an epic review, its block listing the effective members, `deferrable`, `threshold` and effective `opens_after`, so editing any of them re-asks it; `tp epic set gates.<m>.<threshold\|purpose\|deferrable\|members>=` edits one gate, carrying `met`: `threshold` and `purpose` as written, an empty value clearing them and a whitespace-only `purpose` counting as unset |
| `tag_checks.<tag>.provided_by` | epic id | every epic carrying the tag must depend on this epic, directly or transitively |
| `tag_checks.<tag>.direct` | `true`, omitted when false (v1.7.0) | every carrier needs a plain direct edge to the provider; a path through other epics does not count |
| `forbidden_edges` | list of `{from, to, why}` (v1.8.0), omitted when empty, written after `tag_checks` | edges that must never exist, checked by `forbidden-dependency`. `from` and `to` are each a selector with exactly one key: `epic` (an id), `milestone` (a name), `tag`, or `track_of` (a milestone name: every epic of the track holding it). A selector with no key, two keys or another key is a schema error, as a value of the wrong type: `tp epic import` exits 2 and every reader of a hand-edited file exits 3. No `tp epic set` key writes it; write it by hand or through `tp epic import`, and every writer carries it |
| `epics[].id` | non-empty string | opaque and unique in the file; spaces and non-ASCII stay as written |
| `epics[].milestone` | milestone name | required; names one of `milestones` |
| `epics[].tags` | list of strings | opaque to tp, read only by `tag_checks` |
| `epics[].depends_on` | list of `{id, why}` or `{any_of, why}` (v1.7.0) | a plain edge to one epic, or an any-of group of alternative epics, conditions or work; exactly one of `id` and `any_of`, the group with two or more distinct members and one `why`; `why` is required and is content, like `title` and `acceptance` |
| `epics[].description` | string, optional, written only when non-empty (v1.11.0) | markdown content in the epic's own words, never parsed: purpose, scope, non-goals, decisions, open questions; empty or only whitespace is unset. `tp epic set <id> description=<text>` takes short text, an empty value unsetting it, and a `tp epic apply` set row's JSON `value` carries multi-line text with no shell quoting; `tp epic add`, its `--bulk` rows and `tp epic import` take it under the same strict decode. It is in the epic's own review block only (*An epic's description*, below); no lint rule reads it |
| `epics[].root` | boolean, written only when `true` | the epic has no dependency on purpose; `no-dependencies` does not fire on it |
| `epics[].condition` | boolean, written only when `true` | a precondition the operator decides: no bound file, no claim, done only by a done record; refused on an epic with either |
| `epics[].estimate_minutes` | integer, written only when set | the epic's estimate; with none, the sum of its bound tasks' estimates when every one has one |
| `epics[].claimed` | `{at, by}` | written by `tp epic claim`; ignored while a file is bound |
| `epics[].done` | `{at, evidence}` | written by `tp epic done`, only on an epic with no bound file |
| `gates.<milestone>.threshold` | string | the exit condition in the operator's words, written before the work |
| `gates.<milestone>.purpose` | string, optional (v1.9.0) | why the milestone exists, in the operator's words, kept apart from the threshold; `tp epic set gates.<m>.purpose=<text>` writes it and an empty value clears it; empty or only whitespace counts as unset everywhere. It enters the `tp epic scope` fingerprint and reaches only graph roles with `purpose: true`; no lint rule reads it |
| `gates.<milestone>.met` | `{at, evidence, threshold}` | written by `tp epic gate --met`, copying the threshold it was judged against |
| a task file's `epic` | epic id, top-level | binds the file (v1.5.0): `tp init --epic`, `tp set --epic` (`""` removes it); refused for an id the graph does not hold, a condition or a done epic. `tp import` keeps it with its `bound_at` when the document has no `epic` key, as it keeps `workflow`, and keeps `bound_at` when the document names the same epic; a document naming another epic is refused at exit 1 and nothing is written (v1.8.0) |
| a task file's `bound_at`, `imported_at` | RFC 3339 UTC, top-level (v1.8.0) | the time the file was bound and last imported; see *The task file's two times* |

Any key outside this schema is the lint error `unknown-key`. The writers rewrite the file with
2-space indentation, leave `<`, `>` and `&` unescaped and omit an empty list (`depends_on: []`) and
`root: false`, so a file already in that form, loaded and written unchanged, is byte-identical. The
file's **revision** is the lowercase hex SHA-256 of its bytes, and the empty string when it is absent;
every graph writer prints the revision it left and takes `--if-revision`, and `tp epic lint` prints
the one it read. `tp epic set opens_after=` and `milestones=`, `tp epic milestone add` and
`tp epic import` also print, after `revision`, `ancestry_changed: [{milestone, lost, gained}]`
(v1.6.0): each milestone of the old graph whose transitive openers, defaults applied, the write
changed, in the new graph's track order and then milestones only the old graph listed; `lost` in
the old order, `gained` in the new; `[]` when none moved. A milestone the write adds is not
listed, so a first write of `milestones`, or an import into a project without a graph, prints `[]`.

**Any-of groups** (v1.7.0). `{"any_of": ["A", "B"], "why": "…"}` is stored with no `id`. A group's
identity is its member set, in any order. `tp epic link <from> --any-of <csv> <why>` adds a group, or replaces the `why` of the
group with that member set (`action: replaced`); a different set is a new group, the members stored
distinct in the order given. `tp epic unlink <from> --any-of <csv>` removes the group with exactly
that set, else exits 4; a plain `unlink` of an id that is only a group member exits 4 with a hint
naming the group and its `unlink --any-of`. `tp epic set <id> depends_on=`, `tp epic add`, its
`--bulk` rows and `tp epic import` take group entries under the same strict decode. A group
membership makes the member a dependency of its epic for `tp epic remove`, which without `--force`
refuses and names the dependent; with `--force` a done member drops its whole group (it was
satisfied), an undone member leaves it, a group left with one member becomes a plain edge keeping
its `why`, and a group that then names a member the epic has a plain edge to, or repeats another
group's member set, is dropped as implied. Done there is the removed epic's own `done` record.

`claimed`, `done` and `met` are written by their commands and by `tp epic import`, the migration; a
writer that replaces a whole object (`add`, a `--bulk` row, `set gates=`) carries them over and
exits 2 on a payload naming them. A file without the v1.5.0 keys reads as before.

### One atomic graph write (`tp epic apply`, v1.7.0)

`tp epic apply <ndjson>` applies a payload of rows, one JSON object per line, to one graph under one
lock, in file order, and lints the after-state once against the before-state, the rule every writer
uses: all rows are written or none.

| Row | Does |
|-----|------|
| `{"op":"add", "epic":{…}}` | adds the epic, or replaces the one with its id, as a `--bulk` row (`claimed` and `done` exit 2) |
| `{"op":"link", "from":…, "to":… \| "any_of":[…], "why":…}` | `tp epic link`, a plain edge or an any-of group |
| `{"op":"unlink", "from":…, "to":… \| "any_of":[…]}` | `tp epic unlink` |
| `{"op":"set", "id":…, "field":…, "value":…}` | `tp epic set <id> <field>=<value>` |
| `{"op":"set", "field":…, "value":…}` | `tp epic set <key>=<value>`: `milestones`, `opens_after`, `gates`, `gates.<m>.<field>`, `tag_checks` |
| `{"op":"remove", "id":…, "force":bool}` | removes the epic and its gate memberships; with `force` also the edges into it and its groups, as `tp epic remove --force` |

A `set` row's `value` is the text after `=`: a JSON string is taken as its text, any other JSON value
as written, so `"value":[["A"]]` and `"value":"[[\"A\"]]"` are one row. Of an unlink and a link of
one edge, the later stands. The only failures at a row are a subject absent at that row — the `from`
of a link or unlink, the `id` of a set or remove — and an unlinked edge or group absent at that row;
each exits 4 as `payload line <n>: <error>`. Every reference inside a row (a link's `to`, an add's
`depends_on`) and every other check waits for the one lint, so a row may name an epic a later row
adds, and an error only an intermediate state holds is not refused. A remove without `force` of an
epic something depends on fails no row: the edge it leaves is a `dangling-reference` unless a later
row removes it. A refused after-state exits 1 naming, after each error, the payload lines of the
rows that name its epics (`payload lines 2, 3`). A malformed row, an unknown `op`, and an `add` of an
id an earlier row `remove`s (an add already replaces by id) exit 2 naming the line, before the graph
is read. Records (`done`, `claim`, `gate --met`) and `import` are not rows.

It prints the writers' payload with `ancestry_changed` (`[]` when no row moved a milestone's
ancestry). `--dry-run` prints the same payload, `revision` the file's unchanged one, plus
`rows: [{line, op, action}]` — `added` or `replaced` for an add or link, `removed` for an unlink or
remove, `set` for a set — and writes nothing, refusing as the write would. `--if-revision` refuses a
graph that moved (exit 4); under `TP_UNATTENDED` it exits 2 like every graph writer.

### Lint rules (`tp epic lint`)

`tp epic lint` reads the file and writes nothing. Its payload is `{errors, warnings, findings[],
revision}`, each finding `{rule, severity, epics, message}`, `severity` `error`, `warning` or `info`
(v1.6.0: a choice the graph states outright, counted in neither total and never exiting 1), `epics`
the sorted ids it names. Every writer runs the error rules over the file as it would be after the
write and refuses one adding an error; an error's identity is its rule, its sorted epic ids and the
milestone, gate or key names it involves, so an error already in the file does not block an
unrelated write. A refused write carries one hint per distinct broken rule, joined by `; ` in the
order the errors come (a `--bulk` refusal keeps its row hint). A write that shrinks a cycle, of
epics or of `opens_after` milestones, adds no error and passes. The refusal names each added error
as `rule [epics] (message)`, the brackets left out for one that names no epic. Every rule but the
two cycle rules reports one finding per offending item.

| Rule | Severity | Fires when |
|------|----------|------------|
| `any-of-shape` | error | a `depends_on` entry carries both or neither of `id` and `any_of`, a group has fewer than two distinct members, a group sits beside a plain edge to one of its members, or two groups of one epic have one member set (v1.7.0) |
| `cycle` | error | the dependency graph has a cycle, an edge to any member of a group counting (a group naming its own epic is a self-loop); one finding per strongly connected component |
| `dangling-reference` | error, or warning for a `forbidden_edges` `tag` selector | a `depends_on` id or group member, a gate member or deferrable, or a `provided_by` names no epic; a `gates` key names no milestone; v1.8.0: a `forbidden_edges` selector naming nothing the graph holds, one finding per selector (`forbidden_edges[i].<from\|to> <key> "<v>" names no epic\|milestone`) — an `epic`, `milestone` or `track_of` selector is an error, a `tag` no epic carries a warning |
| `duplicate-name` | error | an epic id or a milestone name appears twice |
| `unknown-key` | error | a key outside the graph file's schema |
| `missing-why` | error | an edge or an any-of group has no `why`, or one that is empty once trimmed; a group needs one `why`; a `forbidden_edges` rule needs one too (`forbidden_edges[i] has no why`, v1.8.0) |
| `unknown-milestone` | error | an epic's `milestone` is absent or in no track, or an `opens_after` key or value names a milestone in no track |
| `later-milestone-dependency` | error | an epic depends on one in a later milestone of the same track; edges across tracks are free; `opens_after` does not change it. An `any_of` group is one finding only when every member is in a later milestone of the track, its message naming each member's milestone; one member that can be done first leaves it clean (v1.7.0) |
| `gate-member-outside-milestone` | error | a gate member or deferrable epic is not in that milestone |
| `gate-member-deferrable` | error | a gate lists an epic both in `members` and in `deferrable`; a deferrable epic never holds a gate (v1.6.0) |
| `unprovided-tag-check` | error | an epic carries a tag whose `provided_by` is not in its transitive dependencies, or, under `"direct": true`, not a plain direct dependency; the provider itself is exempt. A provider counts through an any-of group only when every member is the provider or depends on it, by the same rule at every level; under `direct` a group never counts, even one holding the provider |
| `forbidden-dependency` | error | for each `forbidden_edges` rule, each direct edge and each any-of member from an epic its `from` selects to one its `to` selects; one finding per edge, `epics` the two ends, message `<from> depends on <to>[ through an any-of group], which forbidden_edges[i] forbids: <why>`. A path through other epics is not checked. A write adding one is refused with the rule's hint (v1.8.0) |
| `opens-after-cycle` | error | `opens_after`, track defaults applied, has a cycle, a self-reference included |
| `opens-after-not-list` | error | an `opens_after` value is null or not a list, which only a hand edit writes: `tp epic set` and `tp epic import` refuse it, and every other reader than `tp epic lint` exits 3 on a non-list value (v1.6.0) |
| `dependency-opens-later` | error, or warning when no gate member reaches the epic | a cycle of the milestone wait graph over the tracked milestones: a milestone's opening waits on the gate of each milestone it opens after, transitively over the effective `opens_after`, and a gate waits on the opening of each other milestone an undone epic of which an effective member reaches through `depends_on` and epics without a done record, a condition like any epic. No gate on the cycle can go green; one finding per cycle, in canonical rotation, its message naming the milestones in order and, per gate-to-opening edge, the members, the epic they reach and the path; `epics` those members and epics. The refusal hint names one edge: `move <epic> to a milestone that opens no later than "<gate>"`. Reach runs through not-done epics only, so a plan's progress never adds a cycle: a graph write can, and is refused, and a `tp epic done --undo` can, which is written and then reported. With no member reaching an epic of a milestone opening after the gate's, each other epic of such a milestone reaching it is a warning, one per epic and target: it can finish once that milestone opens. An edge `later-milestone-dependency` reports is left to that rule and not followed (v1.6.0). An unsatisfied `any_of` group a gate member reaches is an error when no member can be done while that gate is not green: each member waits through its own opening, and through other gates as far as the wait graph reaches, so members that wait only through each other count. One finding per epic and member set, its message naming the gate, the group, the member reaching it with its path and the opening each member waits on; `epics` the reaching members and the group's members; the refusal hint offers to move the group's first member. A group member in a later milestone of the same track is followed here, since the group is legal while another member is not (v1.7.0) |
| `deferrable-required` | warning | a gate's deferrable epic without a done record that an effective member without one reaches through plain edges and epics without one: the gate cannot go green without it, so the deferral changes nothing. One finding per gate and deferrable, `epics` the deferrable, the first member reaching it in gate order and every direct requirer; its message names the gate, every direct requirer — each epic of the required set (the not-done members and the not-done epics they reach by plain edges) whose own plain edge names the deferrable, in graph order, counted past three as `4 (a, b, c, …)`, since fixing one path leaves the warning standing while another requirer remains — and, as an example, that first member's shortest path (`m -> x -> d`). An epic the gate lists both as member and as deferrable is `gate-member-deferrable`'s; an `any_of` alternative is not followed, since choosing the other option leaves the deferrable free. It reads the graph alone, so it can differ from `tp epic scope`'s `deferrable_in_closure`, which derives done from bound task files too: a deferrable whose requiring work is finished in a task file but not yet closed in the graph is still this warning and no longer a scope row (v1.10.0) |
| `no-dependencies` | warning | an epic outside the first milestone of its track has no dependencies, an any-of group counting as one, and is neither marked `root: true` nor a condition |
| `root-with-dependencies` | warning | an epic marked `root: true` has dependencies, an any-of group counting as one |
| `thin-why` | warning | a plain edge's `why`, case-folded and split into words, is non-empty and occurs as a run of whole words in the target epic's id or title; an any-of group has no one target and is not checked |
| `gate-threshold-unwritten` | warning | a gate with no `met` record carries a threshold that is a placeholder: its trimmed text matches `.tp/config.json`'s `epic.threshold_placeholder`, or, unset or empty, the default `(?i)^(?:\[[^\[\]]*\]\|<[^<>]*>\|TBD\|TODO\|TBC)$` — `[threshold]`, `<exit condition>`, `TBD`, `todo` and `Tbc` match; `draft: 80% pass`, `[a] and [b]` and `TBD by Friday` do not. A project pattern replaces the default rather than joining it; an absent or blank threshold is never a placeholder. One finding per gate, in milestone name order, naming no epic, its message the `tp epic set gates.<m>.threshold=<text>` that fixes it (v1.8.0). Writers do not refuse it, and it is no mechanical check |
| `partial-parallel` | warning, or info when the milestone has an `opens_after` entry | a milestone does not open after an earlier milestone of its track, even transitively, yet some of its epics depend directly on that milestone's epics, or on an `any_of` group whose every member is one of them (v1.7.0); one finding per pair, `epics` those dependents |
| `gate-held-by-condition` | info | effective gate members reach an undone condition of another milestone through epics without a done record; one finding per gate milestone and condition, `epics` the condition and every member reaching it, one example path in the message. A condition in the gate's own milestone, the trigger of a trigger-led milestone, is not reported (v1.6.0). An `any_of` group an epic so reached depends on is one finding, named "one of {A, B}", only when every member is an undone condition outside the gate's milestone, `epics` its members and the gate members; the walk does not go on through a group (v1.7.0) |

`tp epic review` runs the error rules over the whole file at every emission as `mechanical_checks`
and names their slugs in `mechanized_classes`, withheld from the role prompts.

### State directory (`.tp/epic-review/`)

Each review slice has its own directory, `.tp/epic-review/<key hash>/`, where the key hash is the
lowercase hex SHA-256 of the slice kind, a NUL byte and its value — so `--milestone A` and
`--epic A` never share a directory and no id reaches the file system. An unnamed `--epics` set's
value is its sorted, de-duplicated ids joined by NUL; a `--name`d set's is its name, under a kind of
its own, so the set may grow without changing directory. `--status` names it as
`state_dir`.

| File | Written by | Content |
|------|------------|---------|
| `snapshot-round-<N>.json` | emission | each unit's hash and block text as round N read it, and each slice milestone's purpose as `purposes` (v1.9.0); a unit's hash covers its own block and, for an epic, the blocks of its direct dependencies, an any-of group's members included (v1.7.0), a dependency's without its description (v1.11.0); per epic unit, `own_rests` and `tails` (v1.11.0) hash its own block without the description and the dependency blocks its hash covers |
| `round-<N>.ndjson` | `--record` | the round's findings, each with `severity`, `finding`, `location` (`epic:<id>`, `gate:<m>` or `scope:<m>`) and `evidence`; `--resolve` writes dispositions into the last one |
| `key.json` | emission (v1.5.0) | the slice's key, `{kind, value?, ids?, name?}` with `kind` one of `epic`, `milestone`, `epics`; `tp epic export` lists it |
| `grant.json` | `--grant-round` (v1.7.0) | `{round}`, the one round past the cap the operator granted; recording that round spends it |

From round 2 a unit whose hash equals the round before's and that holds no open finding is carried
and not asked; a unit with no recorded hash is asked and makes the slice stale. Convergence and the
cap resolve `review_clean_rounds`, `review_converge_on` and `review_max_rounds` from the project
layer over the built-in default, with no task file layer. A round counts as clean when none of its
rows is open and blocking, as in `tp review`: a row resolved `wontfix` or `duplicate` is closed, and
one resolved `fixed` changed the graph, so the slice is stale until a later round reads the change.
A slice at its cap that is stale is `grantable` (v1.7.0): `tp epic review <slice> --grant-round`, the
operator's decision, lets it emit and record one more round with the ordinary carry and no special
prompt, and recording that round spends the grant. It prints `{state_dir, round, next_action}`; a
slice below its cap, unchanged since its last round, done, or already granted exits 4 and writes
nothing, and it takes one slice at a time, no other mode beside it (exit 2).
`--status` (v1.8.0) also prints `stale_units`, the unit keys (`epic:<id>`, `gate:<m>`, `scope:<m>`)
whose hash differs from the last recorded round's snapshot, in slice order — `[]` on a fresh slice,
every unit before a round is recorded; since a unit's hash covers its dependencies' blocks, editing an
epic names every unit whose hash covers it — `purpose_stale_units` (v1.9.0, below), `open_by_role`, the
last round's undisposed findings counted by role (`unattributed` for none), summing to the same
total as `open`, `max_rounds`, the resolved cap or `null` uncapped, and `grants_spent`, the recorded
rounds past the cap, and `asked_block_bytes` (v1.11.0, *Prompt size* below).
With no `.tp/epic-reviewers/` the four built-in graph roles run (`dod-satisfiability`,
`why-auditor`, `phase-economist`, `gate-scope`); a populated directory replaces them, and `--role`
naming a built-in graph role such a directory lacks exits 2 saying so, with the way to add it
(re-eject, or copy the file). From round 2 the built-in `regression` role, which reads `epics` and
`gates` only, is appended whenever an epic or gate unit changed (other than in its own description
alone, v1.11.0: *An epic's description*, below) or the round before resolved a
finding on one `fixed`; when that round left nothing open on an epic or gate unit and fixed no
blocking finding there, regression is the only role asked those units, beside the roles a scope unit
or a changed purpose asks (below). A scope unit's change or fix never appends regression.

**The gate taken whole (v1.9.0).** A slice that asks `gate:<m>` also asks `scope:<m>`, after every
gate unit, while the gate has a `threshold` or a `purpose`; with neither there is no scope unit, and
writing either later creates it. Its hash is the `tp epic scope <m>` fingerprint, and its block holds
only what that fingerprint covers, every part blind to status: the sorted members, the sorted
deferrables, the threshold, the purpose and the sorted ids of the closure, on a line labelled
`closure (status-blind):`. Closing an epic is progress and never re-asks it. A scope unit is asked of
the roles whose `units` list `scope`, in every round, a regression-only one included; a finding on it
resolved `fixed` re-asks it of them with that finding to confirm, and a block that changed is shown
once, as the asked unit, followed by the change itself: each of `members`, `deferrable`,
`threshold` and `purpose` that differs, old → new, and the closure as `+id` added and `−id` removed
(epic and gate units keep the before-and-now copies regression reads). An open finding on a unit of
a kind the corpus no longer reads refuses the emission at exit 4, naming the unit, the row and the
kind, until a role reading that kind is restored or the row is dispositioned with `--resolve`. Known limit: an edit inside the closure that leaves
its id set, the members, the deferrables, the threshold and the purpose unchanged — a rewritten
acceptance or why — does not re-ask the scope unit; the epic units it changes are re-asked as
before. The gate's purpose is what scopes missing work: a purpose that names what the milestone
releases has that release's obligations checked against the closure's acceptances, and without one
`gate-scope` judges only against the threshold. `gate-scope` reads the gate against what the
threshold and the purpose name; it is not a domain audit of each named obligation. The first
emission after an upgrade to v1.9.0 finds one new, so stale, scope unit in every slice whose gate
has a threshold or a purpose under a corpus with a scope role, the built-in one included; a slice
already at its cap is then `grantable`, each needing its own `--grant-round`, which the cap does not
waive.

**Which units a role is asked.** A graph role file's optional `units` lists the unit kinds it is
asked, a non-empty list drawn from `epics`, `gates` and `scope`; absent, it is `epics` and `gates`.
A kind no role of the resolved corpus lists produces no unit of that kind at all — no `scope:<m>`
under a corpus with no scope role, no `gate:<m>` when every role lists only `epics` — and every
reader resolves it the same way: emission, carry, `--status` and `--check`, the batch rows, `tp epic
export` and the dashboard; a corpus that cannot be read is an exit-3 error in each. Each role is
asked the asked units of its kinds only: the "no gate unit is asked" line, the carried-gate blocks
and the carried-by-key list follow its kinds, as the finding contract's locations do, and the
mechanized-classes line reaches only roles that read epics or gates. A role asked no unit of its
kinds gets no prompt and is named in `skipped_roles` with reason `no-units`. An emission whose
listed units include one no prompt asks is refused at exit 4 naming it, so a round can never record
a unit nobody read. A round in which no role is asked anything prints `next_action` naming
`--record-inbox`, which records it empty and needs no file. Re-emitting an emitted round that is
not yet recorded is guarded as spec review's is (v1.10.0): the round snapshot stores the emitted
panel (`roles`: each role's id, the sha256 of its role file, CRLF-normalized, `builtin` for
regression, and its `output_path`, relative to the repository root when it lies under it, so a
moved repository, a clone or another worktree still finds it, and as given otherwise) beside the
units, blocks and purposes, and a re-emission compares the stored emission with the rebuilt one over
the whole panel, whatever `--role` it names. An emission made inside a `tp run` unit stores the
round directory's part paths. Identical, it is idempotent; different — a unit added, removed or
changed, a purpose changed, a role added or removed or its file edited — it exits 3 naming the
round and what changed, each list counted past three (`units changed: 12 (a, b, c, …)`). The guard's
refusal, the frozen `--resolve` below and a batch's `emission-changed` reason end in one shared
sentence naming the two ways forward: record the round as emitted with `--record-inbox`, or, as the
operator's decision, discard it with `tp epic review <slice> --discard`, then emit again. `--discard`
removes the emission's snapshot and exactly the files its stored panel named, says so on stderr and
emits nothing; it exits 2 under `TP_UNATTENDED` (`discard-emission`), 4 with no unrecorded emission,
and 2, tp epic review's invalid-combination code, beside a batch flag or `--record`, `--record-inbox`,
`--resolve`, `--resolve-all`, `--status`, `--check`, `--history`, `--of-epic`, `--report`,
`--grant-round`, `--role` or `--force`. Spec review
spells the same decision `--force`; epic review cannot, because a discard there must be possible
without a new emission while dispositions are frozen, so `--force` keeps its v1.9.0 meaning,
re-resolving beside `--resolve`/`--resolve-all`; anywhere else it exits 2 (`--force only re-resolves
on epic review (with --resolve or --resolve-all); to discard an unrecorded emission use tp epic
review <slice> --discard`). Editing the graph while a round is out can leave `--discard`, the
operator's decision, as the only way forward. A tp upgrade between
an emission and its re-emission that ships changed built-in role files changes their hashes, so the
guard fires on it. A snapshot from before v1.10.0 holds no `roles` and is compared on the units,
blocks and purposes it holds, and `--discard` over it, holding no stored paths, removes the
round's inbox files by name (`<state dir>/inbox/*-r<N>.ndjson`), the upgrade window's fallback only.
Progress is no change: blocks and the scope fingerprint are blind to status, so closing work
between an emission and its re-emission refuses nothing. Dispositions are frozen while a round is
out: `--resolve` and `--resolve-all` address only the last recorded round, the one the emitted round
was built from, and while that emission is unrecorded they exit 3 and write nothing; once it is
recorded a disposition takes effect from the next round, and after `--discard` it goes through at
once. A disposition therefore never changes an emission behind the prompts already handed out.
Emitting many slices at once (`--milestones`, `--all`) freezes each slice's dispositions until that
slice is recorded or discarded, and a discard covers one slice at a time. An empty hand-made
`--record` file is refused (exit 3) for a round whose emission named role files, since it would
record roles that never ran as having found nothing; `--record-inbox` proves they ran. A round with
no emission, an empty panel or a snapshot from before v1.10.0 still records an empty file. A
row of the last recorded round with no resolution on a unit the slice no longer holds — an epic
deleted, or moved out of the slice — refuses the emission at exit 4, naming the rows, with
`--resolve` as the way out; a row resolved `fixed` there passes, since moving or deleting an epic
is the normal fix. The built-ins: `dod-satisfiability`
and `why-auditor` keep both default kinds, `phase-economist` takes `epics` alone with
`purpose: true`, and `gate-scope` takes `scope` alone with `purpose: true`; the gate questions
`phase-economist` used to ask moved to `gate-scope`.

**The purpose (v1.9.0).** A role file's optional `purpose: true` gives the role, once, a header
naming the purpose of each milestone its asked units belong to that has one; a role whose only kind
is `scope` reads it in the scope unit's block instead. The round snapshot keeps each slice
milestone's purpose (`purposes`); when a milestone's purpose changes, removal included, since the
last recorded round, every unit of that milestone of a purpose role's kinds is asked of that role
again, although its hash is unchanged, with the earlier decisions on it. That is no unit change: it
appends no regression role and sets no regression-only round. The cost has a fixed shape: writing or
changing a purpose re-asks `phase-economist` over every epic of the milestone, a full panel's worth
of that role for the milestone, besides `gate-scope` over the scope unit. Until such a round is
recorded, `--status` lists those units in `purpose_stale_units` (`[]` when none; a unit already in
`stale_units` is not repeated), the slice is `stale`, so neither converged nor done and, at its cap,
`grantable`, and the batch `--status` rows and `tp epic export`'s slices carry the same
`purpose_stale_units`. A snapshot written before v1.9.0 holds no purposes, which reads as none set,
so an upgrade that writes no purpose re-asks nothing. The switch is per role: a purpose reaches only
the roles whose file says `purpose: true`, the price of keeping it away from the roles no trial gave
it to. A custom or ejected corpus gets these mechanisms, not the built-in text: a corpus ejected
before v1.9.0 keeps the old `phase-economist`, gate focus included, and has no `gate-scope` until it
is ejected again or the file is added.

**An epic's description (v1.11.0).** `epics[].description` is part of the epic's **own** block only:
its last section, after the edges, a `description:` line, then each line of the text indented two
spaces, a blank line left empty and the text's surrounding blank lines dropped; a `description:`
line anywhere else, such as inside a multi-line acceptance or why, is a continuation line. Writers
store it with CRLF line ends turned to LF, so one text is one description. It is in that epic's unit
hash, and editing it re-asks that epic and nothing else. Where the epic is another epic's dependency
— in that epic's hash and its context — its block appears without the description: a dependent
relies on what the dependency delivers, which is its acceptance. An epic with no description, or an
empty or whitespace-only one, renders the block bytes v1.10.0 rendered, so no hash moves on upgrade.
The `tp epic scope` fingerprint, the `scope:<m>` unit's hash and block and the scope context a scope
role reads never contain a description: a description is not scope. In a prompt the asked epic's
description is shown in full, uncapped, inside its block, every line indented, so no text line can
close the block's `---` frame; context epics, full or brief, show none, and `gate-scope`'s prompt
holds none. Writing a description is authoring, not a change for regression: a unit whose only
difference from the round before is its own description is asked again of the roles reading its
kind but appends no regression role, and a regression-only round still asks it of them, as a purpose
change asks purpose roles. That difference is told by hash, never by the text's shape: the unit's
hash moved while its `own_rests` and `tails` hashes match the snapshot's (against a snapshot from
before v1.11.0, which holds neither, the block before must equal the own block without the
description), so an acceptance or why that merely looks like a description section is a change. A
finding on it resolved `fixed` appends regression as before. Regression's `Changed since round N:`
lists each changed unit in its own form: the block as that round read it and as it reads now, or,
for a description-only change, the description's line diff in hunks (`- ` removed, `+ ` added, one
unchanged line on either side of a change, `…` for each run of unchanged lines left out). The hand-off context compares a
changed title and acceptance with the ones the round before read, so a description naming an epic
hides no hand-off. `--history` reads a description edit as a change of the field `description`,
never its lines as fields or edges. The built-in `dod-satisfiability`, `why-auditor` and
`phase-economist` read a description, when the asked epic has one, as the primary text of the epic
under review: whether the acceptance satisfies its purpose and scope, whether the edges' whys agree
with its decisions, whether the placement fits its purpose. The acceptance stays the contract the
gate checks; whatever a description says about the acceptance or an edge is checked against them
wherever it is said, an open-questions list included, and a conflict is a finding. A missing
description is not a finding, and an open question that leaves the acceptance and the edges as they
stand is not reported back.
`gate-scope` does not read it. A corpus ejected before v1.11.0 gets the text in its blocks and none
of these instructions until it is ejected again. What belongs in a description is intent, scope,
non-goals, decisions and open questions; behaviour written there is re-reviewed on every edit and
ages as unbuilt spec text does, and every description edit is a round against the slice's cap.

**Prompt size (v1.11.0).** The cost of descriptions is shown, not capped. A slice's `--status`, and
each batch `--status` row, prints `asked_block_bytes`: the bytes of the blocks of the units a round
would ask now, the carry's asked units and the purpose-stale ones. Before the first round it counts
every unit's block; it is `0` when nothing is stale, after a recorded round with no change since, and
on a batch row the graph no longer holds. It is a proxy, available before anything is emitted, which
matters because an emitted round is frozen and taking it back is the operator's `--discard`: it
counts no instructions, context or contract, and each role reads only the units of its kinds. The
emission reports each role's real size as `prompt_bytes`, the byte length of `prompts[].prompt`,
printed before it. `tp epic review` has no compact form, so `--compact` drops neither key. On a large
slice, emit `tp epic review <slice> --role <id>` once per sub-agent rather than reading the whole
emission: an identical re-emission is idempotent, so each call prints only that role's prompt over
the same round. A prompt that grows too large is answered with a smaller slice (`--epics … --name`),
not a cap; the peak is the first pass, since describing a whole milestone re-asks every epic of it
at once.

Each prompt reads, in order: the role's instructions, the asked units (and, for each of its kinds of
which none is asked, a line saying that kind's questions do not apply), a purpose role's purpose
header, the earlier rounds'
`wontfix`/`duplicate` decisions on them with their round and evidence, the role's focus questions
and any round input (regression's fixed findings, each asked epic or gate unit whose own block
changed shown as the round before read it and as it reads now, a scope role's fixed findings and
each changed scope unit's field-by-field change, and the blocks of the epics a changed unit's title or
acceptance newly names, the work the change hands on; a name the unit's text already held is not
shown), the output contract and the file to write, then
the context, and a last line repeating the row keys and the file. A role file's optional `context`
key scopes that context: `neighbours` (the default) shows every epic one edge away in either
direction, `dependencies` only the epics an asked unit depends on, and `neighbours-brief` (v1.7.0)
each dependency's full block and each dependent as its id, title, milestone and its edges into the
asked units with their whys, in every round (the regression role's own prompt keeps every neighbour
in full); `dod-satisfiability` and `why-auditor` ship with `dependencies`, and
`phase-economist` with `neighbours-brief` (an ejected copy keeps the value it was ejected with).
An any-of group's every member is one edge away from its epic (v1.7.0): asking the epic shows the
members as dependencies, asking a member shows the epic as a dependent, its brief listing the group
as `one of {A, B}: <why>`.
A carried gate's block is shown read-only in the context of every prompt whose role takes gate
units, and the carried units no block shows are listed by key. A prompt asking `scope:<m>` adds, as
context and never in a hash, the live `tp epic scope <m>` measurement: the members by mode and
count, then every closure epic and every epic of `<m>` outside the closure once, by its title and
full acceptance, marked `member`, `any_of`, `deferrable`, `outside`, `condition` or
`no_dependents`, with its edges by id to the other epics shown and no why (an edge to a done epic is
left out), then the shrink rows, `deferrable_in_closure`, `holds_opening` and the critical path by
count; done epics and other milestones' epics outside the closure are not shown. Each prompt's `output_path` is the absolute `<state dir>/inbox/<role>-r<N>.ndjson`
in the slice's inbox (v1.7.0; until then a cwd-relative `epic-review-<hash prefix>-r<N>-<role>.ndjson`),
the same path the prompt names; inside a `tp run` unit it is the `$TP_ROUND_DIR` path instead.
The emission creates the inbox and removes nothing in it, so an identical re-emission and per-role
emission keep a file a role already wrote; only `--discard` removes the round's files. `.tp/.gitignore` ignores every inbox (`epic-review/*/inbox/`), and the
plugin's write-deny hook lets exactly that name through, for any writer, when the path's physical
form (symlinks resolved; a `..` segment or a symlink at the name is refused) is that name too;
every other path under the state directory stays refused. Spec review's and audit's output paths
keep their relative form.

**Recording the inbox (v1.10.0).** `tp epic review <slice> --record-inbox` records the in-flight
round from the files its emission named: one per role of the panel the snapshot stores, at the
`output_path` stored there (a relative one resolved against the repository root), read in panel
order through the reader `tp review --merge` applies to the role parts a `tp run` record unit
merges. Each row is stamped with its file's `role`, so `open_by_role` counts no inbox row
`unattributed`; a row naming a different role is refused. Missing files are refused at exit 3 as
`no findings file from roles a, b, c in <inbox dir>`, the hint naming the command (run those roles'
prompts, each writing its file, empty when it finds nothing, then `--record-inbox`), since the
contract asks a role that found nothing for an empty file; so is a round that was not emitted or
whose snapshot predates the panel (emitting it again stores the panel). A line the reader would
skip (named by file and line), a row naming another role, or a row `--record` refuses exits 1 and
records nothing; the payload and the round written are `--record`'s. Rows keep panel order: the
merge's clustering and severity sort are not applied, and the epic contract asks for no `class`,
without which a row never clusters. `--discard` removes exactly the files the old panel
named, so a new emission never records rows written against old prompts. `--record <file>` stays
for hand-made rows and stamps nothing. The `tp run` driver records no epic round today; when it
gains epic units, their record command is `--record-inbox`, not a second merge.

### Batch review and the class report (v1.7.0)

`--milestones <m,m,...>` and `--all` exclude each other, every one-slice flag (`--milestone`,
`--epic`, `--epics`, `--name`) and the writes kept one slice at a time (`--record`, `--record-inbox`, `--resolve`,
`--resolve-all`, `--force`, `--discard`, `--grant-round`); each such call exits 2 and writes nothing. A milestone
listed twice is one slice.

| Call | Reads | Payload |
|------|-------|---------|
| emission | each listed milestone; `--all`: every milestone the tracks hold, in track order | `{slices: [{key, <emission>...}...], skipped: [{key, reason}]}` |
| `--status [--check]` | each listed milestone; `--all`: every slice directory, in track order | `{slices: [{key, round, state, stale_units, purpose_stale_units, open, open_by_role, grantable, max_rounds, grants_spent, next_action}]}` |
| `--report` | each listed milestone; `--all`: every slice directory, in track order | `{classes: [{class, epics, slices: [{key, rounds}]}]}` |

`--milestones` keeps the order given. Under `--all`, track order lists each milestone slice where
the graph's `milestones` tracks list its milestone, then every other slice — an `--epic` or
`--epics` slice, a milestone no track holds, a directory with no key — in slice directory name
order; a report's `slices` follow the same order.

`key` is the slice's key as `key.json` records it, `null` for a directory that records none and
whose rounds name none. An emission's entries are each slice's one-slice emission, `--role`
applied to each, led by its `key`, the one `--status --all` prints for that slice. The graph, the
policy and the role corpus are read once and every slice is prepared before any slice writes
state, so an unknown `--role` exits 2 with no slice written. A
slice that cannot emit is listed under `skipped`, in the order met, and the rest still emit: done
(`reason` names `done_by`), at its cap (the one-slice refusal's message and hint), failing to
prepare (a milestone no track lists, an unreadable round, a refusal such as open rows on a unit the
slice no longer reads or holds, message and hint), an unrecorded emission that changed (`reason`
`emission-changed`, with the change list and the slice's `--discard`; a batch never discards), or failing to write its
state.

A `--status` row is the one-slice `--status` cut to those keys, `state` one of `tp epic export`'s
loop states (`none`, `open`, `in_flight`, `stale`, `cap_reached`, `done`). A slice whose key is
unknown or names what the graph no longer holds gets a row graded without its staleness and a
`next_action` saying nothing runs on it; when such a slice is done, `next_action` reads
`legacy slice, done; no action (safe to delete <state dir>)`. `--check` exits 0 only when every
slice's own `--status --check` would — its loop done and every mechanical check passing — except
that such a row counts as done when its slice is done (a legacy slice needs no action); one that is
not done fails it, as its own `--status` exits 4; else exit 1. A round file tp cannot read exits 3.

`--report` reads the recorded `round-<N>.ndjson` files and writes nothing. A class is a row's
`class`; a row without one counts in no class. `epics` lists, sorted, the distinct epics the class's
rows name by `location` (`epic:<id>`; a `gate:<m>` row names none), so an epic two overlapping
slices raised it on is one epic; `slices` lists each slice that raised it with its rounds,
ascending. Classes sort by the number of distinct epics, descending, then by name. It carries no
`mechanize_candidates`: no registered check retires an epic review class, and a lint rule or a
`direct` tag check is the operator's. Without `--all` or `--milestones`, or beside `--status`,
`--check` or `--role`, it exits 2; a round file tp cannot read exits 3.

### Graph history (`--history`, v1.8.0)

`tp epic review <slice> --history` reads the slice's recorded rounds and the graph and writes nothing:
`{state_dir, rounds, current}`. Each recorded round is `{round, findings_by_role, diff_from, diff}`:
`findings_by_role` counts the round's findings by role, a finding naming none under `unattributed`;
`diff` is the difference from round `diff_from`, the round before, and both are `null` on round 1.
`current` is `{from_round, diff}`, the difference from the last recorded round to the slice's units as
the graph renders them now. `--from <a> --to <b>` holds rounds `a` and `b` only, `b` diffed from `a`,
and no `current`.

A difference is `{units_added, units_removed, units_changed, edges_added, edges_removed,
whys_changed, fields_changed, hashes_only, unchanged}`, each list sorted by unit key and never null:
an edge `{unit, to, why}`, a why change `{unit, to, before, after}` with both texts, a field change
`{unit, field, before, after}`. A round snapshotted before blocks were kept holds only hashes:
`hashes_only` is true, the edge, why and field lists are empty, and the unit lists stay known.
`unchanged` names the units under the same hash; `--compact` drops it and keeps every change.

`tp epic review --history --of-epic <id>` takes no slice: `{epic, slices, message?}`, each slice
`{state_dir, key, rounds}` whose recorded rounds' snapshots hold `epic:<id>`, each round `{round,
edges: [{to, why}], hashes_only}`, the edges in block order. An epic no recorded round holds prints
`slices: []` and a `message` saying it has no graph history, at exit 0; an epic the graph no longer
holds is still looked up, since history outlives it.

### Binding and status (v1.5.0)

A task file names its epic in the top-level `epic` (schema above). An epic's bound files are every
tracked or untracked `*.tasks.json` under the repository root, ignore rules respected, whose `epic`
names it. Status is derived on every read and never stored:

| status | holds when |
|---|---|
| `planned` | no bound file, no done record, no claim |
| `claimed` | no bound file, no done record, a claim |
| `specced` | a bound file, and no task of any bound file is `wip` or `done` |
| `in_progress` | a task of a bound file is `wip` or `done`, and not every bound file is at `release` |
| `done` | every bound file is at `release`, or, with no bound file, a done record |

`release` is the phase `tp resume` reports for the file, so a bound spec edited after its release
falls back to `audit` and its epic to `in_progress`. A claim on an epic with a bound file is
ignored; it outlives the binding, and `claim --release` still removes it. `done_at` is the done
record's `at`, else the `recorded_at` of the audit round that most recently brought the last bound
file to `release`; it is `null` when the epic is not done or that round carries no `recorded_at`.

### Gates, conditions and the epic block (v1.5.0)

A gate's members are its listed `members`, or, when it lists none or `gates` has no entry for the
milestone, every epic of the milestone that is neither a condition nor `deferrable`; `deferrable`
epics never hold it. Every milestone therefore has a gate.
A gate whose effective members are empty and that has no `threshold` is green, so a milestone holding
only conditions never holds back what opens after it; a trigger belongs inside the milestone it
gates (SKILL, Workflow E, Conditions).

- **Green**: every member `done` and, when the gate has a `threshold`, a `met` record. Members are
  read live, so one that falls back turns the gate red while `met` stands. `--met` copies the
  threshold into the record; a second `--met` overwrites it. A threshold edited after its record
  adds `threshold_changed: true` to `tp epic gate` and `tp epic status`, and the gate stays green.
- **Open** (v1.6.0): every milestone it opens after is open and has a green gate. A milestone opens
  after its `opens_after` entry, else the previous milestone of its track; a track's first milestone
  without an entry opens after none and is always open, as is one whose entry is `[]`. A milestone
  on an `opens_after` cycle, or opening after a name no track lists, is closed (both lint errors); a
  milestone two tracks name takes its first place. Only an open milestone's epics are ready; a
  closed milestone never holds a bound file back.
- **Conditions**: an epic with `condition: true` has no bound file and no claim, and is done only by
  `tp epic done <id> --evidence`, which also records an unbound work epic done.

The epic block reads direct dependencies only: an epic's own `depends_on`, a done record counting as
done whatever lies behind it. An any-of group is satisfied once one member is done by the same rule
(v1.7.0). While one of a bound file's epic's dependencies is not `done`, or one of its groups is not
satisfied:

| command | behaviour |
|---|---|
| `tp next`, `tp next --peek` | `{done: false, message, blocked_by_epics}`, exit 4; a `wip` task is still returned. When every blocker is a condition, which only the operator decides, it adds `operator: {decision: "epic-done", commands: ["tp epic done <c> --evidence <evidence>"]}` (v1.6.0) |
| `tp ready` | `{ready: [], blocked_by_epics}` in every mode, `--count` and `--ids` too; `--first` exits 4 |
| `tp claim` | exit 1, the error object carrying `blocked_by_epics`, when an id it was given is an `open` task (`--all-ready`: when the file holds one); ids none of which is `open` answer as on an unbound file — not open, not found, exit 4 |
| `tp done` | refuses a task that is not `wip`: by id at exit 1, by several ids and by `--batch` as a failed row carrying `blocked_by_epics`, exit 1 when the block refuses every row; a `wip` task closes, and the next-task counts leave blocked tasks out. The block is judged on the file read under the lock, after the quality gate ran |
| `tp commit` | its implicit claim would start an `open` task, so it exits 1 with `blocked_by_epics`, staging and writing nothing; a `wip` task commits |
| `tp resume` | with no `wip` task, the `epic-blocked` blocker and no implement unit |

`blocked_by_epics` lists the plain dependencies not done (`[]` when only groups block). Every payload
above that carries it adds `blocked_by_any_of` while a group is unsatisfied, one list of member ids
per group in `depends_on` order, and the message names each group `one of {A, B}`, as in `epic w is
blocked by c, one of {a, b}` (v1.7.0). The operator action exists when every plain blocker is a
condition and every group has a condition member whose milestone is open and whose own dependencies
are done, the members the operator can record now; it adds `choices: [["tp epic done <a> --evidence
<evidence>", ...]]`, one list per group of those members' records, any one of which satisfies it
(`commands` is then `[]` when no plain condition blocks).

A file with no `open` task has nothing to start, so `tp next`, `tp ready` and `tp claim` answer it as
they answer an unbound file (`tp claim` exits 4) and carry no `blocked_by_epics`. A `*.tasks.json`
under the repository that cannot be parsed may be bound to any epic, so every command that derives
an epic's status or looks up its bound files exits 3 naming it rather than guess.

A file holding no task at all has no tasks yet, not all of them complete (v1.6.0). `tp next` and
`tp next --peek`, with or without `--epic`, `tp brief` with no id and `tp plan` answer
`{done: false, message, phase, hint, command}` and
exit 4, as their other no-task answers: `phase` is the one `tp resume` reports for the file (`review`
or `decompose`), `hint` its next step and `command` that step's tp command, absent when it has none.
`tp ready --first` and `tp claim --all-ready` exit 4 with the same message and hint, and `tp ready`'s
other modes keep their empty answer and say it on stderr.

A file bound to an epic the graph does not hold, or with no graph file, behaves as unbound, and
`tp next`, `tp ready`, `tp claim` and `tp resume` say so on stderr.

### Read payloads and `tp epic export` (v1.5.0)

The read commands write nothing and list epics in graph order, every list `[]` when empty.

- **`downstream`** (v1.6.0): `{longest_chain, dependents, dependents_open, dependents_by_milestone:
  [{milestone, dependents}]}` over the not-done epics depending on an epic, transitively, itself
  excluded. A done epic cuts every path through it; a condition is traversed but never counted.
  `longest_chain` is the most such epics on one path, `dependents` their count, `dependents_open`
  those in an open milestone (every open track head included). `dependents_by_milestone` lists
  milestones with at least one, tracks in graph order, each in track order, then untracked ones.
  On a dependency cycle (a lint error) its members depend on each other: each counts the others,
  and they share one chain, whatever the order of the epics. v1.7.0: while an any-of group is
  unsatisfied, each member reaches the group's epic and what depends on it; once one member is
  done, the others no longer do. A moot epic counts in no downstream and has none of its own.
  `dependents` counts an epic once however many members or groups lead to it, so summing the
  downstreams of a group's members double-counts by design. `--compact` leaves
  `dependents_by_milestone` out of every ready item, on `tp epic ready` and `tp epic status`.
- **`actionable`** (v1.6.0): a condition is actionable when it is not done, every direct
  dependency is done, its milestone is open, and at least one of: it is a member of an open
  milestone's gate (explicit `members`, since the default members leave conditions out), so
  deciding it can turn that gate green; nothing depends on it; or some not-done dependent, in an
  open milestone or with a bound file, has only conditions among its undone direct dependencies.
  The first keeps a condition that holds a gate from waiting on a dependent in the milestone that
  gate opens, which would never open. v1.7.0: an epic depends on a condition through a plain edge
  or an any-of group naming it; an unsatisfied group counts as a condition when one of its
  condition members has its milestone open and its own dependencies done, and once a group is
  satisfied its epic leaves the other members' actionability.
- **`not_actionable`** (v1.6.0): on every undone condition that is not actionable, every reason that
  holds, in this order: `milestone` (its milestone is not open), `dependencies` (a direct dependency
  of its own is not done), `dependents` (no open milestone's gate holds it and no dependent that can
  move waits on it and on conditions alone; v1.8.0: a dependent `deferrable` in its milestone's gate
  does not count), `deferrable` (v1.8.0: its own milestone's gate lists it `deferrable`). Absent on
  an actionable condition; a condition never carries `waiting_on`, which is a milestone's key.
- **moot** (v1.7.0): an undone epic is moot when no gate holds it (a member of any milestone's
  gate), at least one `depends_on` entry names it, every any-of group naming it is satisfied, and
  every plain edge naming it comes from a moot epic, so what only a lost alternative needed is moot
  too. It is derived on every read: a new plain edge from an epic that is not moot makes it count
  again. A moot epic is in neither `ready`, `claimed` nor `operator.conditions`, `--all` included,
  and in `tp epic status`'s `blocked` a moot condition carries `moot: true` in place of
  `not_actionable`. An epic a gate holds is never moot: it stays the operator's to defer or remove.
- **`tp epic ready [--all]`**: `{ready: [{id, title, milestone, downstream}], claimed: [{id, title,
  milestone, by, at}], operator: {conditions: [{id, title, milestone, actionable, not_actionable?}],
  gates: [{milestone, threshold}], threshold_rewrites: [{milestone, threshold, command}]}}`. `ready` is sorted by `longest_chain`, then `dependents`, both
  descending, then graph order; `claimed` holds the epics that would be ready but for their claim, in
  graph order; a bound epic is in no list. A work epic is a candidate when every direct dependency
  is done and every any-of group has a done member (v1.7.0). `operator.conditions` holds the
  actionable conditions; `--all` lists every not-done condition of a tracked milestone that is not
  moot with `actionable` `true` or `false`, and `not_actionable` on each `false` one.
  `operator.gates` names gates whose members are done and whose threshold is written, not a
  placeholder, and has no `met`, open or not, in track order. `operator.threshold_rewrites`
  (v1.8.0) lists each gate with no `met` whose threshold is a placeholder, done members or not,
  once, in track order: `{milestone, threshold, command}`, `command` reading
  `tp epic set gates.<m>.threshold=<threshold>`. `tp epic status` and `tp epic export` carry both
  lists per milestone. `--all` lifts the open-milestone rule.
- **`tp epic gate <m>`**: `{milestone, green, members: [{id, state, status}], deferrable: [{id,
  status}], threshold?, met?, threshold_changed?}`, `state` `green` or `red`.
- **`tp epic scope <m>`**: `{fingerprint, milestone, purpose, members, members_implicit,
  member_count, closure: [{milestone, track_index?, cross_track?, size, epics: [{id, any_of?,
  deferrable?}]}], shrink: [{epic, member, any_of?, size, leaves}], outside: [{id, deferrable?,
  condition?, no_dependents?}], deferrable_in_closure, holds_opening: [{milestone, members_implicit, size, epics}],
  critical_path: {by_count, by_estimate: {chain, estimate_minutes}}}`. `purpose` is the gate's
  `purpose` trimmed, `null` when it is unset, empty or only whitespace. `members` is `GateMembers`'
  list, `members_implicit` true when the gate names none, `member_count` its length. `closure` is the
  not-done epics the not-done members reach through `depends_on` entries not yet satisfied, members
  included, cut at done epics and never entering a moot one, per milestone in milestone order;
  `track_index` is the index of the milestone's track in `milestones`, absent for a milestone no track lists, and
  `cross_track` marks every milestone other than `<m>` that shares no track with it (two untracked
  milestones share none), as information.
  The *required* part is what the members reach through plain edges alone and carries no `any_of`;
  every other closure epic is optional and its `any_of` is the group of the alternative it hangs
  from: the unsatisfied group that reached it, or the mark of the epic whose plain edge did, the
  first in walk order when two groups reach it. `deferrable: true` marks a closure epic the gate
  lists `deferrable`, required or optional. `shrink` holds, for each closure epic that takes
  another with it, the epics that leave: for a non-member, when every edge into it is cut; for a
  member (`member: true`), when it is dropped from the gate; `any_of` is the epic's closure mark.
  `size` counts the epic itself, `leaves` is sorted, rows go by size descending, then id; the rows
  cost one closure walk per closure epic. `outside` is the milestone's not-done epics the closure
  misses, moot ones left out, in graph order, under implicit members too (they leave deferrable and
  condition epics out); `no_dependents: true` marks one no `depends_on` entry of the graph names,
  plain or as an any-of member. `deferrable_in_closure` is the gate's `deferrable` epics in the
  required part, which cannot actually be deferred (no lint rule reports this); `[]` when none.
  `holds_opening`: for each gate `<m>` opens after directly (its effective `opens_after`, not their
  predecessors in turn), in that order, the gate's not-done effective members `<m>`'s closure does not
  contain, sorted, with their count as `size` and `members_implicit` saying whether that gate names
  no members; a gate with none is left out. They hold `<m>`'s opening for nothing, and are not epics
  to delete; a deferrable is no member, so it is never listed. The lever depends on the
  predecessor's gate: under an implicit one, make the epic deferrable there; under an explicit one
  the only structural move is removing the epic from that gate's `members` (it cannot be both member
  and deferrable), a decision about the predecessor that its own scope unit asks, not `<m>`'s. What
  other gates need is no input, so a row for a product milestone that follows another can
  legitimately be long.
  `critical_path` is `tp epic status`'s `milestones[m].critical_path`.
  `fingerprint` is the hex sha256 of the sorted members, the sorted ids of the closure drawn blind to
  status (every epic taken as not done, every any-of member followed), the threshold, the sorted
  deferrables and the purpose, each part labelled and length-prefixed, an unset purpose adding no
  part: closing an epic, satisfying a group or reordering the members or the deferrables leaves it;
  a rewritten threshold or purpose, a changed deferrable list or a change in the epics the members
  reach moves it. `--compact` prints a strict subset of the same keys, the fingerprint still first:
  `{fingerprint, milestone, purpose, members?, members_implicit, member_count, closure: [{milestone,
  track_index?, cross_track?, size}], shrink: [{epic, member, any_of?, size}],
  deferrable_in_closure?, holds_opening: [{milestone, members_implicit, size}], critical_path: {by_count}}`; it drops an
  implicit gate's `members` list, the closure's epic lists, each shrink row's `leaves`, `outside`,
  each holds_opening row's `epics`, an empty `deferrable_in_closure` and `by_estimate`. It writes
  nothing and runs under `TP_UNATTENDED`.
- **`tp epic status [--milestone <m>] [--open]`**: `{critical_path: {by_count}, milestones: [...]}`.
  The top-level `by_count` is the longest chain of not-done epics in the whole graph, in work order,
  counted in work epics; ties go to the chain ending earlier in graph order; v1.7.0: at an
  unsatisfied any-of group it goes through the member with the shortest remaining chain, the first
  listed on a tie, and a moot epic is on no chain; it never ends at an epic `deferrable` in its
  milestone's gate, though it goes through one a later epic of the chain depends on; it ignores
  `--milestone`, `--open` and `--compact`. Each tracked milestone once, in track order:
  `{milestone, open, opens_after, waiting_on, counts: {planned, claimed, specced, in_progress, done,
  conditions_done},
  share_done, gate, ready, claimed, blocked: [{id, title, milestone, held_by, blocked_by,
  held_by_any_of, blocked_by_any_of, not_actionable?, moot?}], operator,
  critical_path: {by_count, by_estimate: {chain, estimate_minutes}}, throughput: [{week, done}],
  rollup: [{epic, estimate_minutes, actual_minutes, tracked, estimate_total}]}`. `opens_after` is
  the effective list, defaults applied; `waiting_on` those of it not open or not green, `[]` once
  open. `--open` keeps the open milestones only; with a `--milestone` that is closed it exits 4
  naming what the milestone waits on. `gate` is `tp epic gate`'s payload; `ready` and
  `operator` are `tp epic ready`'s, filtered to the milestone; `claimed` is every epic whose status
  is `claimed`, blocked ones too. The status counts and `share_done` cover work epics only:
  `share_done` is `done` over the milestone's work epics to two places, 0 with none, and a done
  condition counts in `conditions_done` alone. `blocked` is each not-done epic with a dependency not
  done, less the epics `deferrable` in the milestone's gate (v1.8.0; export's own `epics[]` item
  for one still says `blocked` and `blocked_by`): `blocked_by` every such dependency once,
  `held_by` the conditions among them (v1.6.0), as export's epics, `blocked_by_any_of` and
  `held_by_any_of` the export's group lists (v1.7.0), and
  on a condition `not_actionable`, or `moot: true` on a moot one (v1.7.0). `by_count` runs in work
  order, the gate member last, `[]` when every member is done; ties go to the earlier gate member,
  then the earlier dependency; at an unsatisfied any-of group the chain goes through the member with
  the shortest remaining chain, `by_estimate` the lightest, the first listed on a tie (v1.7.0).
  epic on a chain has no estimate. `throughput` lists only weeks with a done work epic, ascending, as
  UTC ISO weeks (`2026-W45`), counting epics `done` now. `rollup` has a row per bound epic, summing
  `tp report`'s totals over its files; `estimate_total` sums every task's estimate whatever its
  status, the other figures closed tasks only. `--compact` leaves each milestone's `critical_path`
  (v1.6.0: the top-level one carries the chain), `throughput` and `rollup` out. So
  `--milestone <m> --compact` prints one `critical_path`, the top-level one, which is the whole
  graph's chain and not `<m>`'s: read a milestone's own chain without `--compact`.
- **`tp epic export`**: JSON, `{version, generated_at, critical_path, graph, milestones, epics, slices, lint}`, every key
  always present; `critical_path` is `tp epic status`'s top-level one (v1.6.0); `--md` prints the
  per-milestone tables instead, a group as `one of {A, B}: <why>`, and `--compact` is ignored.
  `version` is 2 (v1.7.0, since a `depends_on` entry may carry `any_of` in place of `id`; 1
  before); renaming, removing or retyping a key bumps it, adding one does not. The graph file keeps
  its own `version: 1`.
  - `graph` is `.tp/epics.json` as stored, each epic's `description` included (v1.11.0, additive,
    so the export keeps its version); `milestones` is `tp epic status`'s `milestones`,
    uncompacted.
  - `epics`: `{id, title, milestone, condition, status, done_at, files, downstream?, actionable?,
    not_actionable?, moot?, held_by, blocked_by, held_by_any_of, blocked_by_any_of, blocked,
    held}`, `done_at` RFC 3339 or `null`. `downstream` is the one above without
    `dependents_by_milestone`, absent on a done or moot epic; `actionable` is present on conditions
    only, `false` once done; `not_actionable` on an undone condition that is not actionable;
    `moot: true` on a moot epic, in place of both (v1.7.0). `blocked_by` lists the undone plain
    dependencies, `held_by` the conditions among them, both epic ids in `depends_on` order;
    `blocked_by_any_of` holds one member list per unsatisfied any-of group, `held_by_any_of` those
    whose every member is a condition (v1.7.0); `blocked` and `held` say whether a list of their
    kind is not empty, so no reader combines them. `files` is `[]` when unbound, else each `{path,
    phase, tasks: {open, wip, done}, review, audit, ground}`: `path` relative to the git toplevel,
    `phase` `tp resume`'s, each loop `{round, state}`. Review and audit count recorded rounds, `state`
    `none`, `done`, `stale` (review only), `cap_reached` or `open`; ground counts emitted rounds,
    `state` `none`, `complete` or `open`.
  - `slices`: each `.tp/epic-review/` directory in name order, `{dir, key, round, state, grantable,
    max_rounds, grants_spent, stale_units, purpose_stale_units, open_by_role}`, the last four as `--status` prints them
    (v1.8.0) and no per-round history, which is `--history`'s. `key` is
    its `key.json` record; without one, the key whose hash equals the directory name among the
    units its recorded rounds name (read-only, nothing is written), else `null`; `state` is `done`,
    `in_flight` (emitted, not recorded), `none`, `stale`, `cap_reached` or `open`, graded as
    `tp epic review --status` does, `grantable` too (v1.7.0). A null-key slice, or one naming an
    epic or milestone the graph no longer holds, is graded without staleness, so it is never
    grantable and its `stale_units` is `[]`.
  - `generated_at` (v1.8.0): the RFC 3339 UTC time of the derivation, right after `version`; two
    exports of an unchanged repository differ in it alone.
  - `lint` (v1.8.0): `{errors, warnings, info}`, the counts `tp epic lint` reports for the same
    file under the same placeholder pattern; it survives `--compact`.
  - `milestones[].state` (v1.8.0): `{members_done, members, threshold}` — the gate's members done
    of its members, `deferrable` epics in neither count, and `threshold` one of `met` (a met record,
    whatever the text), `absent` (empty or blank), `placeholder` (the pattern matches) or `written`.
    Each milestone's `operator` carries `threshold_rewrites` as `tp epic status` does.
  - `epics[].layout` (v1.8.0): `{lane, lane_kind, column, row}`, the epic's place on the dashboard's
    map, derived from the graph alone and the same on every export of one graph. Milestones whose
    `opens_after` entry is an explicit `[]` form the `triggered` lane; over the rest, the longest
    opening chain is the `main` lane and the remaining milestones are taken longest chain first into
    `parallel` lanes, emitted in graph-file order of their first milestone. `lane` is the index in
    the order main, parallel lanes, triggered; `column` is the milestone's opening depth over the
    effective `opens_after`; `row` the epic's place inside its milestone, epics ordered by the mean
    row of their dependencies in milestones already placed, an epic with no dependency just before
    its first dependent, ties in graph-file order. Absent on an epic whose milestone no track lists.
    A cycle's closing edge is ignored, so a graph lint refuses still lays out.
  - bound files (v1.8.0) add `bound_at` and `imported_at` when the task file carries them, each
    omitted otherwise.

### `--epic` on `tp resume`, `tp next`, `tp ready` (v1.6.0)

`--epic <id>` names the task file as `--file` would, overriding `TP_FILE`: the epic's only bound
file, else the only one whose phase is not `release`. Otherwise it exits 4: naming the candidates
(the files not at `release`, or every bound file when all are) in the message and as a `candidates`
array of `--file` arguments; saying the epic has no bound task file; or, for an id the graph does
not hold (or with no graph file), `epic "<id>" is not in the graph`. With `--file` it exits 2; an
unreadable task file exits 3. A task file's path, here and in every write payload's `file`, is
relative to the directory the command ran in, symlinks resolved on both sides, so it works as
`--file` from there, a subdirectory or a symlinked cwd (macOS `/tmp`, `/var`) included.

### Exit codes (`tp epic`)

| code | outcome |
|------|---------|
| 0 | the command completed; for `tp epic lint`, no error-severity finding; for `--status --check`, the loop is `done` and every mechanical check passes, with `--milestones`/`--all` on every slice; for `tp epic gate`, the gate is green; `done --undo` with no record and `claim --release` with no claim change nothing |
| 1 | `tp epic lint` found an error; a write whose after-state adds a lint error (never `tp epic done` or its `--undo`: the operator's record is written, and the lint reports what it adds), a `tp epic apply` payload included; `tp epic remove` of an epic another depends on, through an edge or a group membership, without `--force`, or of a `tag_checks` provider even with it; `tp epic import` over a file holding epics without `--force`; a `--record` row missing a required key or locating outside the slice (the whole file is refused); a `--record-inbox` file holding a line the reader skips (named by file and line) or a row naming another role than its file's, or a row `--record` refuses; `--status --check` not done (with `--milestones`/`--all`, any slice not done or not held by the graph); `--resolve` of an already resolved finding without `--force`; `milestone add` of a name a track already holds; `tp epic gate` of a red gate; `tp epic claim` while a claim exists (the message names its holder), or of a condition, a done epic or one with a bound file; `tp epic done` of an epic with a bound file, or of a condition with an empty acceptance; `gate --met` while a member is not done or the threshold is empty; `set <id> condition=true` on an epic with a bound file or a claim; outside `tp epic`, `tp init --epic` and `tp set --epic` naming an epic the graph does not hold, a condition or a done epic (nothing is written), and `tp import` of a document naming an epic other than the one the task file is bound to (nothing is written; re-bind with `tp set --epic`) |
| 2 | usage: a payload key outside the schema, an unknown `set` field or gate field (`gates.<m>.<field>=`) or bad JSON, an `opens_after=` value that is not an object of string lists (a `null` list included, on `set` and `import` alike), an `add` or `--bulk` payload that names `claimed` or `done` and a `set gates=` one that names `met`, a malformed `tp epic apply` row, an unknown `op` or an `add` row of an id an earlier row removes, a `condition=` value other than `true`/`false`, `review` without exactly one of `--milestone`/`--epic`/`--epics`, `--name` without `--epics` or an empty `--epics`, `milestone add` without exactly one of `--after`/`--before`/`--new-track`, an unknown `--role`, an invalid `--resolve` status, an out-of-range or repeated index or a `wontfix`/`duplicate` without evidence, `tp epic done` without exactly one of `--evidence`/`--undo`, `gate --met` without `--evidence` or `--evidence` without `--met`, `claim --release` with `--by`, `--grant-round` beside another mode or a batch flag, `--milestones` beside `--all`, either beside `--milestone`/`--epic`/`--epics`/`--name`/`--record`/`--record-inbox`/`--resolve`/`--resolve-all`/`--force`/`--discard`, `--record-inbox` beside `--record`, `--discard` beside `--record`/`--record-inbox`/`--resolve`/`--resolve-all`/`--status`/`--check`/`--history`/`--of-epic`/`--report`/`--grant-round`/`--role`/`--force`, `--force` without `--resolve`/`--resolve-all`, a `--milestones` list that is empty or holds an empty name, `--report` without `--all` or `--milestones` or beside `--status`/`--check`/`--role`; and under `TP_UNATTENDED` every writer, `--record`, `--record-inbox`, `--discard` (escalate `discard-emission`), `--grant-round` (escalate `grant-round`), `tp epic done` and its `--undo`, `gate --met`, and `tp init --epic` and `tp set --epic` (an empty value too) — `tp epic claim` runs |
| 3 | file: no graph file, a malformed one, unreadable slice state, or `--resolve` with no recorded round; a re-emission of an unrecorded round whose emission changed; an empty `--record` file over a round whose emission named role files; `--resolve`/`--resolve-all` while the next round is emitted and unrecorded (dispositions are frozen); `--record-inbox` with a role's file missing, no emission, or a snapshot from before v1.10.0; for `tp epic review` and `tp epic export`, a `.tp/config.json` that is not valid JSON (`lint`, `ready` and `status` warn instead and use the default placeholder pattern, and an invalid `epic.threshold_placeholder` exits no command 3); for `ready`, `status`, `gate`, `scope` and `export`, no graph file or an unreadable bound task file |
| 4 | state: an absent epic (`set`, `remove`, `link`/`unlink`'s `<from>`, the subject of a `tp epic apply` row at that row, `review --epic`/`--epics`, `claim`, `done`), edge (`unlink`, an `apply` unlink row), any-of group (`unlink --any-of`) or milestone (`review --milestone`, `milestone add`'s anchor, `gate`'s milestone, `scope`'s milestone, `status --milestone`, `set gates.<m>.<field>=`, the message listing the milestones the tracks hold), or a `status --open --milestone` that is closed; a stale `--if-revision`; a reached cap — `review_max_rounds` rounds recorded and not converged — refuses emission and `--record`, naming the slice's `--resolve` and the key that raises the cap (on a stale slice, `--grant-round` first); `--grant-round` on a slice below its cap, unchanged since its last round, done, or with a grant open; an emission whose last recorded round left a row with no resolution on a unit the slice no longer holds or no role reads; `--discard` with no unrecorded emission |

`--history` (v1.8.0) adds to the table: 2 for `--from` or `--to` without the other or without
`--history`, and for `--of-epic` beside a slice flag, `--from`/`--to` or any other flag of the command,
or without `--history`; 3 for a slice with no state directory; 4 for a `--from`/`--to` round the slice
did not record. It runs under `TP_UNATTENDED`: it writes nothing.

## Task Export and the Dashboard (v1.8.0)

### The task file's two times

| Key | Shape | Written by |
|-----|-------|------------|
| `bound_at` | RFC 3339 UTC, top-level, omitted when unset | `tp set --epic <id>` and `tp init --epic <id>` when the binding changes: re-binding the same epic keeps it, binding another moves it, and `tp set --epic ""` removes it with `epic`. `tp import` keeps it as it keeps `epic` |
| `imported_at` | RFC 3339 UTC, top-level, omitted when unset | every `tp import` into the file, `--force` included; no other write touches it |

A file written before v1.8.0 has neither key, loads, and saves without gaining them; nothing reports a
time for it. `tp epic export` copies both onto the bound file's entry and `tp export` prints both
(`null` when absent).

### `tp export` (version 1)

The read model of one task file, the one `--file` names or discovery finds; it takes no positional and
writes nothing.

```
{version: 1, path, spec, epic, bound_at, imported_at, sections, tasks, loops, resume}
```

- `path` is the task file and `spec` its spec, repo-relative; `epic`, `bound_at` and `imported_at` are
  `null` on a file without them.
- `sections`: `[{anchor, heading, line, paragraphs: [{anchor, line, text}]}]`. `§0` is the preamble
  (left out when empty); a numbered heading of level 2 to 6 opens `§<number>`; an unnumbered level-2
  heading opens `§<slug>` (lowercased, each run of characters other than letters and digits one
  hyphen), a repeated slug taking the next free suffix (`-2`, `-3`); a level-3-or-deeper unnumbered
  heading is a paragraph of its section. A paragraph is a run of non-blank lines, a fenced block one
  paragraph, anchored `§N ¶M`; frontmatter is no paragraph but counts in `line`. An anchor depends only
  on the text above it, so an edit leaves every earlier anchor unchanged.
- `tasks`: each task as stored plus `state` — `ready`, `waits_on_task`, `waits_on_epic`, or its
  `wip`/`done` status — and `waits_on`, its undone dependencies, or, under the epic block, the epic's
  undone dependencies, a group written `one of {A, B}`.
- `loops.{ground, review, audit}`: each `{cap, clean_rounds_needed, streak, rounds}`; `cap` is `null`
  uncapped and on ground; ground's `clean_rounds_needed` is 1, since its `--check` reads the latest
  round. Each recorded round is `{round, recorded_at, findings, clean, snapshot_diff}` (ground: `fails`,
  its `FAIL` rows, in place of `findings`); a ground round stores no time, so its `recorded_at` is the
  round file's modification time. `snapshot_diff` is `{changed, removed}` against the round before's
  snapshot of the same phase, `null` on round 1 or when a snapshot is missing. A review round also
  carries `finding_rows`, its round file's findings `[{severity, role, class, location, finding,
  disposition}]` (`role`, `class` and `disposition` left out when the row has none), absent when the
  round holds none or its file cannot be read.
- `resume` is `tp resume`'s payload for the file, exactly as that command prints it.

`--compact` keeps each section's `anchor`, `heading` and `line` and drops its `paragraphs`, drops each
task's `description`, sets every `snapshot_diff` to `null`, drops every `finding_rows`, and compacts `resume` as
`tp resume --compact` does.

### `tp export --list` (version 1)

```
{version: 1, files: [{path, title, epic?, epic_state?, phase, tasks: {open, wip, done},
                      review: {round, state}, audit: {round, state}, next_step, escalations?,
                      content_hash}]}
```

One row per `*.tasks.json` git sees, tracked or untracked and not ignored, bound or not; outside a
git repository `files` is `[]`. `title` is the spec's first heading; `epic_state` the epic's
derived status (`null` with no graph); `phase` and `next_step` what `tp resume <file>` reports;
`review`/`audit` `{round, state}` as `tp epic export`'s bound files carry them; `escalations` the file's
escalate blockers; `content_hash` the lowercase hex SHA-256 of the task file's bytes followed by its
spec's, which moves with an edit to the spec's text or a task's acceptance or description that no
other field shows. `epic`, `epic_state` and `escalations` are omitted when empty. Rows run in phase
order (`review`, `decompose`, `implement`, `audit`, `release`), escalated rows first within a phase,
then by path. A file that cannot be read keeps a `{path, error}` row, sorted last, and the rest are
whole. `--list` ignores `--file` and `--compact`.

### `tp dashboard`

Serves the embedded page and its data on `127.0.0.1:<port>` (7377 unless `--port`), prints the URL —
the bare URL in a TTY, `{url}` when piped or under `--json` — and serves until `SIGINT` or `SIGTERM`.
It writes nothing to the repository and loads nothing remote.

| Request | Answer |
|---------|--------|
| a `Host` other than `127.0.0.1:<port>` or `localhost:<port>` (name case-insensitive, port required) | 403, before anything else |
| any method but `GET` | 405 with `Allow: GET` |
| `/`, `/style.css`, `/pure.js`, `/app.js`, `/favicon.svg` | the page |
| `/api/events` | the event stream below |
| `/api/export?file=<path>`, `<path>` a `tp export --list` row's `path` | `tp export --file <path>`'s JSON, uncompacted |
| `/api/history?milestone=<m>` (or `epic=`, `epics=`, with `name=`, and `from=`/`to=`) | that slice's `tp epic review --history` JSON |
| `/api/epic-history?epic=<id>` | `tp epic review --history --of-epic <id>`'s JSON |
| `/api/layout?view=<milestones\|scope\|neighbourhood>` (v1.13.0) | one scoped view of the graph laid out in layers (**Layout** below) |

A GET answer is byte-equal to the command's stdout. An error answers tp's `{error, code, hint}`:
HTTP 400 for code 2, 404 for codes 3 and 4. No response carries a CORS header.

`/api/export` takes no filesystem path from the request: `<path>` must be byte-equal to a `path` of
`tp export --list` as the server derives it at that request, and is then answered as
`tp export --file <path>` answers it run at the repository root, which the list's paths are
relative to: the file served is the listed one whatever directory the server started in, and the
payload is byte-equal to that command's from a subdirectory or a symlinked one too; an error carries
its code and hint, and its message names the file as that command does, except that a server
started below the root names a listed file it cannot read by its absolute path. Whether a path is
listed is decided from the paths alone, reading no other task file. Any other value (another
spelling of a listed file, such as `./`
or its absolute path, or none) answers 404 with code 4, the code of a named object that does not
exist, and a hint naming `tp export --list`. A listed task file whose `spec` names a file
elsewhere, or that is a symlink, is read exactly as `tp export` reads it.

**Events.** `/api/events` is a server-sent event stream. While at least one page listens, the server
derives three payloads in-process every 2 seconds and pushes each, as one line of JSON under its event
name, only when it differs from the last push with a top-level `generated_at` left out:

| Event | Payload |
|-------|---------|
| `epic-export` | `tp epic export`'s JSON, while a graph file exists |
| `export-list` | `tp export --list`'s JSON |
| `summary` | the summary below, over an empty graph when there is none |

A page that connects first receives the current state, in the order the payloads were derived. A
derivation that fails is pushed under its event as tp's `{error, code, hint}`; the page shows it as a
banner over the last good payload, greyed. A page more than 32 events behind is dropped and
reconnects, and the loop stops when no page listens.

The epic drawer shows a selected epic's `description` (v1.11.0), read from the export's raw graph,
as preformatted text: escaped, never rendered as markup.

**Layout** (v1.13.0). `/api/layout` lays one scoped view of the graph out in layers, so the page
draws it and computes nothing about the graph. It is presentation, not a versioned contract, and no
command prints it. The query names one `view` of a closed set, each taking only its own parameters:

| Query | View |
|-------|------|
| `?view=milestones` | every tracked milestone, an edge from each milestone it opens after |
| `?view=scope&milestone=<m>` | `tp epic scope <m>`'s closure, banded by milestone, its critical path marked |
| `?view=neighbourhood&epic=<id>[&depth=<n>]` | the epic, what it depends on and what depends on it, `depth` steps each way (default 2) |

The refusals, each tp's `{error, code, hint}`: no `view` or one outside the set, `view` given twice,
a parameter the view does not take, a parameter given twice, a missing or empty `milestone` or
`epic`, or a `depth` that is not an integer answer 400 with code 2. `milestone` and `epic` are
looked up among the milestones a track lists and the graph's epic ids, never built into a path, and
one the graph does not hold answers 404 with code 4. `depth` is clamped to 1-4, an integer of any
size included. An error echoes a value of the query cut to its first 80 characters. A graph that
cannot be read answers as `tp epic export` exits.

The answer is `{view, milestone?, epic?, depth?, layers, bands?, nodes, edges}`: `milestone` for
`scope`, `epic` and the clamped `depth` for `neighbourhood`; `layers` the number of layers;
`bands`, for `scope` only, `[{milestone, with?, from_layer, to_layer}]`, each milestone of the
closure a run of consecutive layers, in dependency order, `with` listing the milestones that share
the run (below). `nodes` lists each node by layer, then order:
`{id, layer, order}`, both small integers from 0, plus, in `scope`, `deferrable` (`tp epic scope`'s
closure mark; an epic's milestone is in `tp epic export`, its any-of group on its edges), in
`neighbourhood` the epic's `milestone`, which the view has no band for, and `centre: true` on the
epic asked for; for a milestone, `epics` and `done` (its work epics and those done),
`inbound` (the dependency edges from other milestones' epics into its epics, an any-of member one
each) and `open`. `edges` is `[{from, to, critical?, any_of?}]`: from a dependency to its dependent
(in `milestones`, from a milestone to one that opens after it), `critical: true` on an edge between
two consecutive epics of the critical path drawn (the scope's own, or the graph's in
`neighbourhood`), and `any_of` naming the group of an edge from an any-of member.

A node's layer is its dependency depth: its longest path over the view's edges. In `scope` each
band's layers come from the longest path among that band's own epics, the bands laid end to end so
a band follows every band it depends on. Two milestones can depend on each other through different
epics while the epic graph stays acyclic (an epic of A on one of B, another of B on another of A):
no order of the two bands keeps every edge pointing forward, so milestones on such a cycle share one
band, `with` naming the rest after the first in closure order, and its epics' layers come from the
longest path over all their edges. Every edge runs from a lower layer to a higher one. Each layer is
ordered by four barycenter sweeps, down, up, down, up, ties broken by graph-file order, so one graph
always lays out alike.

**The summary** is the page's own payload, figures no command prints; its keys are not a versioned
contract:

- `rounds`: per epic review slice, keyed by its state directory name (`tp epic export`'s
  `slices[].dir`), `[{round, total, by_role}]` in round order, every finding counted whatever its
  disposition, a finding naming no role under `unattributed`.
- `band`: `{stages, phases, phases_of}`. `stages` counts work epics per status (`planned`, `claimed`,
  `specced`, `in_progress`, `done`); `phases` counts per spec phase (`review`, `decompose`,
  `implement`, `audit`, `release`) each work epic with a bound file once, at its earliest bound file's
  phase, and `phases_of` reads `epics`. With no graph, or one with no epic, `phases` counts task files
  instead and `phases_of` reads `task_files`. Both maps carry every key of their axis.
- `release_impact`: per task file, by repo-relative path, `{frees, gates}`: the epics that would no
  longer be blocked once this file's epic is done, and `[{milestone, members_done,
  members_done_after, members}]` for each gate whose done count would move. A bound file carries its
  epic's impact only when every other file bound to that epic is at `release`; an unbound file
  carries `{frees: [], gates: []}`.
- `plan`: `{figures, ready, operator_conditions, operator_groups, waiting, most_waited_on}`;
  `operator_groups` is `[{epic, any_of}]`, each unsatisfied any-of group of conditions a not-done work
  epic of an open milestone, not a deferrable member, waits on: the operator records one of them.
  `figures` is `{work, ready, blocked, held, done, share_done, critical_path, operator_queue,
  open_loops, stale_loops, throughput}` over work epics, `operator_queue` counting actionable
  conditions, those groups, gates awaiting `--met`, threshold rewrites and the slices whose
  `grantable` is true, `open_loops` the epic review slices not `done`, `stale_loops` (v1.13.0) those of
  them a re-review alone settles: `stale` (a graph edit changed their units since their last round),
  otherwise done (enough clean rounds, no blocking finding open) and not at their cap. Every other
  slice of `open_loops` needs something else: a round; or, `in_flight`, its emitted round recorded,
  not a new one; or, stale at its cap, the grant `operator_queue` counts it for.
  `throughput` is `[{week, done}]` over every
  milestone. `waiting` leaves `deferrable` gate members out, as `tp epic status`'s and `export`'s
  `blocked` do, and `operator_conditions` is `tp epic ready`'s actionable conditions (v1.8.0: none
  deferrable, none only deferrable work waits on). Empty with no graph.
- `series`: work epics per stage per UTC day, `[{date, planned, claimed, specced, in_progress, done,
  unknown}]`, from the first recorded event's day to the last's. An epic is claimed at `claimed.at`,
  specced at the earliest `bound_at` of its bound files when every one carries it, in progress at the
  earliest `started_at` of their wip or done tasks, and done at `done_at`; while a stage it reached at
  a time tp did not record may already hold it, it counts under `unknown`, never on a guessed day.
  `[]` with no graph or no recorded event.
- `matrix`: the milestone matrix, `[{from, to, edges, done}]`, one cell per pair of milestones a
  dependency edge joins, from the dependent epic's milestone to its dependency's, in track order:
  `edges` counts the edges, each member of an any-of group as one, and `done` those whose dependency
  is done. An edge inside one milestone has no cell. `[]` with no graph.

### Exit codes

| Command | 0 | 1 | 2 | 3 |
|---------|---|---|---|---|
| `tp export` | printed | — | usage | no task file (the hint names `--file` and `tp init`), or one that cannot be read (the hint names the file and `tp validate`) |
| `tp export --list` | printed, `files: []` with no task file; a malformed file keeps its row | — | usage | — |
| `tp dashboard` | interrupted after serving | the port cannot be bound: taken (address in use), or another listen error reported as itself; the message names the port | `--port` outside 1-65535; `TP_UNATTENDED` | neither a graph file nor a task file (the hint names `tp epic set` and `tp init`) |

`tp export` and `--list` run under `TP_UNATTENDED`, since they write nothing; `tp dashboard` does not,
since it serves until interrupted.
