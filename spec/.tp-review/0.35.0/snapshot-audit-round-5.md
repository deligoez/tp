# tp v0.35.0 — Unattended run

## 1. Overview

Today a tp cycle needs a human in the loop for one reason only: nothing starts a fresh agent
process. The state is already durable — `tp resume` reconstructs where a cycle stands from
`.tp-review/`, the task file, `commit_shas` and `gate_passed_at` — but the driver's own context
window is not, and it is the driver that decides what runs next. So the cycle stops whenever that
context fills, and the operator restarts it by hand.

This version closes that gap from both sides. `tp run` becomes the driver: it spawns **one fresh
process per unit of work**, reads the result from disk rather than from the process's prose, and
repeats until the oracle says the cycle is releasable or a cap trips. A companion Claude Code
**plugin** ships what has to be true *inside* each of those processes: the brief arrives without the
agent having to ask for it, the scope fence is denied rather than requested, and the review and audit
roles are declared rather than retyped.

The design premise is that a unit should begin with an empty context, not a compacted one.
Compaction only fires once the window is nearly full, which means it repairs the context after the
agent has already spent a stretch reasoning in the degraded region. Fresh-process-per-unit skips
that region entirely, and it removes the compaction-drift failure mode rather than mitigating it.

**The audit loop does not terminate on its own, and this version does not pretend it will.** An
earlier draft of this spec claimed a dependency on scope-aware audit convergence. v0.33.0 shipped
the divergence *signal* and deliberately left the gate counting every non-PASS row, because a
`scope` label is a judgement made by the sub-agent that wrote the row: wire it into the release gate
and one row mislabelled `codebase` lets a genuine spec violation ship. That reasoning stands, so
`tp run` must work with a predicate that can grind. It does, and the mechanism is already in this
spec: an audit that keeps producing findings hits `audit_max_rounds` or a run cap, and the driver
stops with a non-`converged` `stop_reason` and reports to the operator. The divergence signal —
`spec_coverage_clean_rounds`, `role_streaks`, `divergence` — travels in `tp run --status` on any
audit-phase stop, so the operator gets at the end of a run exactly the information v0.32.0's operator
had to derive by hand at round ten. A driver that never gates on `scope` and never accepts findings
on its own authority is the correct shape here; the tool surfaces, the human decides.

## 2. Prior art and the lessons taken

Every comparable system converged on the same shape — a fresh process per iteration, state on disk —
and each one failed somewhere tp can avoid.

| System | What it did | Lesson taken |
|---|---|---|
| Continuous-Claude-v3 | Daemon extracts learnings into Postgres + pgvector; ledger and handoff files; `/clear` triggered by hand | Its expensive half exists to reconstruct state tp already stores typed. **tp builds no memory layer** — `tp resume` is the handoff |
| agent-loop | Fresh invocation per iteration, one task each, markdown checklist state, verification command feeds the next iteration | Fresh-per-unit is the right shape, and **a failed verification must reach the next unit** — §4.2 |
| ralph-loop | Stop hook re-feeds the prompt in one session; exits on a completion string | A string match is not an exit condition. **The oracle is the exit condition** — §4.1 |
| A runaway multi-agent loop | Four agents, no step cap, eleven days of recursion because no shared state could declare the work finished | **Exactly one component owns "done"** — §4.1 — and caps are the backstop, not the exit |
| An unattended overnight postmortem | Agents authored their own "user-approved" files; role boundaries enforced only by instruction text; hooks recursed without timeout | §5.1 makes user-only decisions fail closed; §6.2 denies the fence; §6.4 bounds every hook |
| Overnight operating practice | Verbose command output floods context; backgrounded runs hang on stdin; unbounded spend | §3.5 keeps output on disk; §3.2 fixes stdin; §3.4 bounds spend per unit |
| Cognition's two multi-agent posts | Writes stay single-threaded; read-only fan-out is safe; a reviewer on clean context finds materially more | §3.3 runs role units in parallel and record units alone |

## 3. `tp run` — the driver

### 3.1 The loop

`tp run [spec]` takes the same optional spec argument and `--file` flag as `tp resume`, and
discovers the same way when neither is given. It executes one iteration at a time and never holds
work in memory that it could re-derive:

1. Read the cycle state through the same path `tp resume` uses.
2. If the cycle is releasable, exit 0 with `stop_reason: converged`.
3. Take `next_units` for the current phase; an empty array stops the run with `no-units`.
4. Spawn a runner process per unit — units marked concurrent together, every other kind alone.
5. Re-read the cycle state from disk. Never parse the agent's prose for status.
6. Check caps; loop.

The fifth step is the load-bearing rule. A unit's result is whatever it wrote to disk: a commit, a
closed task, a role's NDJSON file, a recorded round. The driver's only reading of the process itself
is its exit code and the one spend key (§3.2). This is what removes the completion-token fragility
that both prior loops carry.

#### 3.1.1 Units, ids and the child environment

A unit's `id` is its durable subject: the task id for `implement`, the role id for a role unit, the
round number for a record unit, the finding's `role:item_id` for `audit-fix`, and the spec base name
for `review-resolve` and `decompose`. An id is unique within its kind, so `(kind, id)` identifies a
unit — which is the key `last_failure` clears on (§4.2).

`tp run` takes a distinct run-scoped lock at `.tp/locks/run-<base>.lock`, held for the whole run, so
a second `tp run` over the same task file exits 4. It never holds the task-file write lock while a
child is in flight: that lock stays the children's, taken and released by each `tp` write they make.
A crashed `implement` unit leaves its task `wip`; the next run's `implement` unit resumes that same
task, which is what `tp next` already does.

Every child is spawned with `TP_RUN_ID`, `TP_UNIT_ID`, `TP_UNIT_KIND`, `TP_UNIT_SEQ`, `TP_PHASE`,
`TP_ROUND`, `TP_RUN_DIR`, `TP_ROUND_DIR` and `TP_FILE` in its environment, applied on the same terms as
`TP_UNATTENDED` (§3.2). They are what let a unit address its own artifacts without the driver
passing them through prose.

`TP_RUN_ID` is a ULID. `TP_RUN_DIR` is the absolute path of `.tp/runs/$TP_RUN_ID` and holds what
belongs to one run: logs and escalation records. `TP_ROUND_DIR` is
`.tp/rounds/<base>/$TP_PHASE-r$TP_ROUND`, keyed by the cycle rather than the run, so a round
interrupted by a driver death keeps the role files it already collected and the next run's record
unit merges the completed set instead of an empty directory. `TP_FILE` is the resolved task file, so
a child works the same target the driver resolved rather than re-running discovery. Children are
spawned in the repository root.

`seq` counts unit *attempts*, not units, and is unique within a run, so no two attempts share a log
path. A role's findings file is named by role rather than by `seq`, so a round directory holds at
most one file per role however many runs contributed to it. The oracle decides the unit set first,
omitting roles whose file already satisfies §3.3's predicate (§4.1); the driver then deletes the
role findings file of every `-role` unit that set does contain — both the final name and any stale
`.part` — immediately before spawning it.
Only role kinds have a file deleted this way; a record unit's `merged.ndjson` is never deleted,
because it accumulates dispositions (§6.3). A leftover therefore either means the role is
done and is not re-run, or the role is being re-run and its leftover is gone before it starts.

`TP_ROUND` and `TP_ROUND_DIR` are unset — not empty — when the oracle reports `round` null.

### 3.2 The runner abstraction

The harness is configuration, not a hard dependency. `.tp/config.json` gains a `runner` object:

| Field | Meaning |
|---|---|
| `cmd` | Executable to spawn |
| `args` | Argument template; placeholders `{prompt}`, `{max_budget_usd}`, `{unit_id}`, `{unit_kind}`, `{log_path}` |
| `env` | Extra environment for the child, merged over the parent's |
| `spend_key` | Dot path to the cost field in the runner's final log line; optional |

`runner` resolves as a workflow field (§7) and takes one of three shapes: a built-in template name,
a runner object, or a map from unit kind to either of those with a `default` key covering the rest.
The two object shapes are told apart by their keys: an object carrying `cmd` is a runner, anything
else is a per-kind map. A map missing `default`, or a runner object missing `cmd`, is a usage error.

The per-kind map is what lets the operator run audit units on a different model from the one that
produced the work. Two independent lines argue for the capability: a model scoring its own output
favours it, and the harm peaks precisely when its output is wrong and the alternative is right; and
the judging role has a capability floor that the executing role does not. tp takes no position on
which model to use — it has no way to know, and it enforces nothing about the substitute — but it
must not be the reason an operator cannot separate producer from judge.

#### 3.2.1 Templates and placeholders

Two built-in templates ship by name. The `claude` template's `args` are `["-p", "{prompt}",
"--output-format", "stream-json", "--verbose", "--permission-mode", "auto"]`, with
`["--max-budget-usd", "{max_budget_usd}"]` appended when the resolved `run_max_unit_budget_usd` is
non-zero; a value of 0 omits the flag entirely rather than passing a literal 0. The `opencode`
template's `args` are `["run", "{prompt}"]`, with no budget flag and no `spend_key`, so `cap-budget`
is inert for it.

Every child is spawned with stdin closed; a backgrounded run that inherits a TTY-less stdin hangs
silently, which is the hardest unattended failure to diagnose.

`{prompt}` expands to a fixed per-kind instruction the driver owns: run this unit's `brief_command`,
do that one unit, stop. The driver never executes `brief_command` itself and never reads its output.
`{unit_id}` and `{unit_kind}` expand to the unit's id and kind, `{log_path}` to its log path (§3.5).
A placeholder the driver cannot resolve is a usage error (exit 2) raised before any child is spawned.

`env` is merged over the parent's environment; the driver's own variables are applied after that
merge and cannot be overridden by it, so an `env` entry for `TP_UNATTENDED` never reaches the child.

`TP_RUNNER_SEAM` is the test seam: when set, its value is the `cmd` of a runner object whose `args` are
`["{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}"]` and whose `spend_key` is
`total_cost_usd`. It is a test-only override that outranks every layer of §7's precedence, including
a CLI flag, so a test can pin the runner whatever the repo's config says. The driver reads it from
its own environment at start and never from a child's, and its name is deliberately not `TP_RUNNER`
— that is the fenced env layer of the `runner` field (§5.1), which is a different thing. The fake runner writes a
final log line carrying that key, which is what makes the spend and budget-cap paths testable
without an agent.

`{max_budget_usd}` expands to the resolved `run_max_unit_budget_usd`, including a literal `0`. Only
the `claude` template's flag pair is dropped at 0; a positional template receives the number, so the
placeholder always resolves.

#### 3.2.2 Spend

After a child exits the driver reads one number from the final line of that unit's log:
`total_cost_usd` for `claude`, `spend_key` for any runner that declares one. A runner that declares
none reports `spend: null` for its units, and `run_max_budget_usd` is inert for them — reported once
at run start, not silently. Reading that single key is not prose parsing (Non-Goal 6).

Under a per-kind map the cap accrues only the reporting kinds, and the run-start report names which
kinds are unmetered, so a partial total is never mistaken for a complete one.

### 3.3 Unit kinds

A unit is the smallest piece of work that ends in a durable write. There are eight kinds:

| Kind | Ends in | Durable write | Concurrency |
|---|---|---|---|
| `implement` | One task committed and closed | The task's `status` is `done` | Alone |
| `review-role` | One role's findings file for the round | `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` exists and every line parses | Parallel with sibling roles |
| `review-record` | The round merged and recorded | `$TP_ROUND_DIR/merged.ndjson` and a review round file for `$TP_ROUND` exist | Alone |
| `review-resolve` | The spec revised and each addressed finding resolved | Every finding in `$TP_ROUND_DIR/merged.ndjson` carries a disposition | Alone |
| `decompose` | Tasks imported into the task file | The task file holds at least one task | Alone |
| `audit-role` | One role's results file for the round | `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` exists and every line parses | Parallel with sibling roles |
| `audit-record` | The round recorded, convergence recomputed | `$TP_ROUND_DIR/merged.ndjson` and an audit round file for `$TP_ROUND` exist | Alone |
| `audit-fix` | One audit finding closed, with or without a code change | That row carries a disposition in the round's results file | Alone |

`audit-fix` disposes its row with `tp audit --resolve <results.ndjson> <idx> <fixed|wontfix|
duplicate> "<evidence>"`, the audit-side mirror of `tp review --resolve` — same dispositions, same
`--resolve-all` and `--force`. The selector is a 0-based index or a `role:item_id` key, so a unit
can name its own row without first locating its index. The results file is the one the round's
`audit-record` unit wrote, `$TP_ROUND_DIR/merged.ndjson`, which is also what that unit passes to
`tp audit --record`. Without it the kind has no way to record the
no-code-change outcome §3.3 admits, and the audit side already mirrors `--merge`.

A role file's predicate reads only its content lines: blank and whitespace-only lines are ignored,
matching §8a.4, so a trailing newline never fails a role.

#### 3.3.1 What completes a unit

A role unit writes `role-<id>.ndjson.part`; the driver's rename to the final name on exit 0 is what
completes that unit's durable write, so the predicate above reads the final name while §6.3's
allowlist names the `.part` the unit itself may write.

Every predicate is a state, never a delta: each is decidable by reading the named artifacts as they
are, with no baseline and no "before" — the record kinds name two, and both are simple existence
checks. That is what lets the Stop hook (§6.2) and the driver's own success test read the
same condition, and it is why a unit that correctly closes a finding without changing code still
passes.

Each kind's brief comes from the `brief_command` the oracle supplies for it (§4.1): `tp next
--brief` for `implement`, `tp review <spec>` and `tp audit <spec>` for the role kinds, `[ -f $TP_ROUND_DIR/merged.ndjson ] ||
tp review --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson; tp review <spec>
--record $TP_ROUND_DIR/merged.ndjson` for `review-record` and the `tp audit` equivalent for
`audit-record` — the merge produces the file the record consumes, which is why one unit owns both
steps, and it is skipped when the file already exists so a re-run never merges over the dispositions
§6.3 says it accumulates — `tp review
<spec> --status` for `review-resolve`, `tp audit <spec> --status` for `audit-fix`, and `tp resume`
for `decompose`. A unit succeeded
when it exited 0 **and** its durable write is present; either alone is a failed attempt. The driver
reads those two and never the unit's output.

Role units are read-only with respect to shared state — each writes only its own findings file
(§6.3) — so they run concurrently, bounded by the number of active roles for the phase, which the
corpus fixes. Every other kind serializes. Splitting roles into their own processes, rather than
fanning out to subagents inside one process, is deliberate: it gives every role the clean context the
generator-verifier evidence rewards, and it removes the per-session subagent ceiling that ended
v0.32.0's audit early.

### 3.4 Caps and stop conditions

Caps bound the run; they never conclude it. A run stops for exactly one of these reasons, recorded
verbatim in the run state:

| `stop_reason` | Cause |
|---|---|
| `converged` | The oracle reports the cycle releasable (§4.1) |
| `cap-units` | Spawned children reached `run_max_units`; a retry counts again |
| `cap-wall-clock` | Elapsed seconds reached `run_max_wall_clock_seconds` |
| `cap-budget` | Accrued spend reached `run_max_budget_usd`; disabled when that value is 0 |
| `escalation` | A unit wrote an escalation record (§5.2) |
| `unit-failure` | One unit exhausted its attempts |
| `no-units` | The oracle reports no pending units and the cycle is not releasable |
| `interrupted` | The operator signalled the driver |
| `driver-error` | The driver could not spawn a unit or write its own state |

`tp run` exits 0 on `converged`, 4 on every other `stop_reason`, and 2 on a usage error before the
loop starts. A failure the driver itself cannot recover from — a runner that will not exec, a run
directory it cannot write — stops the run with `driver-error` rather than being charged to the unit
as a failed attempt. A driver's caller therefore separates "the cycle is done" from "a human is needed"
without parsing the payload, and `stop_reason` names which of the seven it was.

A unit is attempted `1 + run_max_unit_retries` times, so the default of 1 gives two attempts and 0
gives one attempt and no retry. Exhausting them stops the run with `unit-failure`. `totals.units`
and `--status`'s units-done count attempts too, so all three counters read the same number.

When one checkpoint satisfies several rows, the recorded reason is the first of `converged`,
`driver-error`, `escalation`, `unit-failure`, `interrupted`, `cap-budget`, `cap-wall-clock`,
`cap-units`, `no-units` — `converged` leads because §3.1 checks releasability before caps, and a cycle that
became releasable is releasable whatever else the iteration also hit.

Caps are evaluated between iterations only. Children already spawned run to completion, so a run can
overshoot its wall-clock and budget caps by at most one iteration; the driver kills nothing. `SIGINT`
and `SIGTERM` set `interrupted` on the same terms: in-flight children finish, no new unit is spawned,
and the run state is written before the driver exits.

Every non-converged stop is a report to a human, never an acceptance. In particular a cap that trips
while an audit finding is open leaves that finding open; the run does not downgrade the convergence
rule to fit the budget it was given. No stop reason ever records a round, marks a phase converged, or
closes a task that its own unit did not close.

The per-unit budget cap is the runner's own flag, passed through rather than reimplemented. A unit
that exhausts it exits non-zero and counts as a failed attempt.

### 3.5 Run state, logs and status

Run state lives at `.tp/run-<base>.json`, git-ignored like `.tp/local.json` and named per task file,
because the run lock is per task file and two runs over different cycles in one repo are permitted:

```json
{"run_id": "", "started_at": "", "phase": "", "stop_reason": null,
 "totals": {"units": 0, "wall_clock_seconds": 0, "spend_usd": 0},
 "units": [{"seq": 0, "kind": "", "id": "", "attempt": 1, "exit_code": 0,
            "duration_seconds": 0, "spend_usd": null, "log_path": ""}]}
```

A unit's row is appended before its child is spawned, with `exit_code`, `duration_seconds` and
`spend_usd` null, and updated in place when the child exits. Both writes are atomic (temp + rename),
because concurrent siblings write rows in the same iteration.

That file is observability, not truth: no decision — the driver's or a hook's — reads it back. If
the driver dies, the next `tp run` re-derives everything it needs from the cycle state; it does not
need the previous run's file to continue. Two sources of truth for the same fact is the drift the
overnight postmortem catalogued. The accrued totals therefore restart at zero on a new run: caps
bound a run, not a cycle.

One process writes each unit's log, and the template's args decide which: a template that omits
`{log_path}` has its stdout and stderr redirected by the driver to
`$TP_RUN_DIR/<seq>-<kind>-<id>.jsonl`; a template that uses the placeholder receives that same path
and owns the file, and the driver does not redirect. Both built-in templates take the first form. The driver reads
only the spend key from the final line (§3.2) and never reads these into any context; they exist for
the operator and for post-hoc diagnosis. Verbose command output — a full `go test ./...` run, a lint
sweep — is the fastest way to poison an agent's window, and keeping it on disk is the cheapest
defence.

`tp run --status` prints the current or last run: phase, units done, spend against each cap, the last
unit's exit code and log path, and `stop_reason` when the run has ended. It also carries `run_state`:
`stopped` once `stop_reason` is set, otherwise `in_flight` when `.tp/locks/run-<base>.lock` is held
and `crashed` when it is not — the lock is the only evidence separating the last two. On a stop in
the audit
phase it also carries `spec_coverage_clean_rounds`, `role_streaks` and `divergence`, verbatim from
`tp audit --status`, which is how §1's promise is kept.

`tp run --dry-run` prints the units the driver would execute next and exits 0 without spawning
anything or writing run state. `tp run --status` exits 0 whenever it can report, and 3 when no run
state exists for the resolved task file. Neither sub-mode takes the run lock.

## 4. Oracle contract

### 4.1 One component owns "done"

`tp resume` is the single authority on whether the cycle is finished, and `tp run` holds no opinion
of its own. The runaway loop above burned eleven days because no participant was permitted to declare
the work complete; tp's inversion of that is that no participant except the oracle is permitted to.

`tp resume --json` gains `next_units`: an ordered array of the units the driver should execute now,
each carrying `{kind, id, brief_command}`, alongside `round`: the number of the round the returned
units belong to — the round being collected for role units, and the round just recorded for the
resolve and fix kinds that act on its findings — or null outside a round-based phase. `round` and the existing `phase` are what the driver substitutes
into `TP_ROUND` and `TP_ROUND_DIR` (§3.1). Concurrency is not repeated per entry — it is fixed per
kind by §3.3's table — and a non-concurrent kind is never returned alongside another unit. In a
round-based phase `next_units` omits any role whose findings file already satisfies §3.3's
predicate — present and wholly parseable — so a resumed round re-runs a role that left a malformed
file instead of skipping it. When the panel is **non-empty** and every role in it satisfies that
predicate, and the round has no recorded entry, the oracle returns the single `review-record` or
`audit-record` unit for that round: the step between collecting a round and acting on it, and the one
point at which `next_units` would otherwise empty — stopping the run with `no-units` — while the
round's own work is unfinished. The non-empty guard is load-bearing rather than defensive: a panel
that could not be resolved, or one a spec's frontmatter has wholly deactivated, satisfies "every role
in it" vacuously, and a record unit emitted there would merge an unmatched glob and freeze a round
holding zero role files — the outcome §8a.4 exists to prevent. An empty panel keeps returning no
unit, and its emptiness stays a `no-units` stop a human sees.
After a round is recorded, the oracle returns a single `audit-fix` unit
for the first row in `$TP_ROUND_DIR/merged.ndjson` that is neither `PASS` nor already disposed — one
at a time, since the kind runs alone — and a single `review-resolve` unit while any finding there
lacks a disposition.

`next_units`, `round` and `phase` are the whole machine surface: the driver parses those three and
nothing else. The existing `next_action` object stays and stays the human-facing summary; its
`command`, `brief_command` and `payload` render `next_units[0]` when the array is non-empty.

The cycle is **releasable** when `phase` is `release`. `tp run` exits 0 with `stop_reason:
converged` on that condition alone. `next_units` is `[]` in two other cases — a phase whose work is
blocked, and a phase awaiting an operator decision — and both stop the run with `no-units` (§3.4).
`next_action.summary` names what the phase is waiting for.

### 4.2 `last_failure`

When a unit exits non-zero, or closes with a failed quality gate, tp records a `last_failure` object
at `.tp/last_failure-<base>.json`: `{unit_kind, unit_id, phase, exit_code, summary, at}`. It has two
writers, because the two triggers are visible to different processes: `tp done` writes it when the
quality gate it ran fails, with the gate's own output as `summary`, and only when `TP_UNIT_KIND` is
set — outside a run it writes nothing, since the record's `unit_kind`, `unit_id` and `phase` have no
value there; the driver writes it when a child exits non-zero, with the failing command and the log
path as `summary`. A gate failure the
harness swallows is therefore still recorded, and neither writer ever copies the child's prose. The file holds at most one object: a second failure overwrites it, and a success clears it
only when `(unit_kind, unit_id)` matches — id alone collides, since both record kinds are identified
by a round number. It lives outside the run state because it must survive the run that wrote it; it
is git-ignored, and it is advisory — its absence never changes which unit runs next.

`tp resume` and `tp brief` surface `last_failure` when one is present, which puts it in front of the
next unit that starts. Without this, a fresh process is fresh in the harmful sense too: it re-enters
the same wall with no idea the previous attempt hit it. This is the one piece of continuity a
reset-native loop genuinely needs, and it is small enough to stay a single object rather than a log.

## 5. Unattended mode

### 5.1 User-only decisions fail closed

`tp run` sets `TP_UNATTENDED=1` for every child process, applied after the `runner.env` merge (§3.2).
The mode is active when the variable is present, non-empty and not `0`. Under it, the decisions
CLAUDE.md already reserves for the user stop being available to the agent:

| Attempt | Result |
|---|---|
| `tp done --skip-gate <why>` | Exit 2, hint naming it a user-approved decision |
| `tp set --workflow review_max_rounds=` or `audit_max_rounds=` above the resolved value | Exit 2, same shape |
| `tp import --force` | Exit 2, same shape |
| `tp set --workflow run_max_*=` above the resolved value | Exit 2, same shape |

The cap comparison is against the currently resolved value; an equal or lower value is accepted and
exits 0, since lowering a budget cannot manufacture convergence. The exception is the value 0, which
means *disabled* for both budget fields rather than *lowest*: setting either to 0 while the resolved
value is non-zero is a raise and is refused, while setting 0 where 0 already resolves changes
nothing and is accepted. The driver's own caps are fenced on
the same terms as the round budgets: a unit that could raise `run_max_units` or
`run_max_wall_clock_seconds` could run itself indefinitely. `runner` and `notify_cmd` are fenced too,
and more strictly: they name commands the *driver* executes, so under the variable a unit cannot set
them at all, at any layer.

No environment layer can raise a fenced field, and the reason is structural rather than a rule the
fence enforces: tp reads no `TP_<FIELD>` variable for any workflow field (§7), so there is nothing
for the fence to exclude. A test that sets `TP_RUN_MAX_UNITS` and observes no change therefore
**proves nothing** — it would pass identically with the fence removed. The fence's discriminating
tests are the ones that raise a cap through a layer tp does read: a task override or the project
config, refused under the variable and accepted without it.

The postmortem's sharpest finding is that a filesystem cannot tell an agent-authored approval from a
user-authored one, so downstream steps treat a forged approval as authoritative. tp's equivalent of a
forged approval is an agent that raises its own round budget and then declares convergence. The
countermeasure raises the cost of that hatch rather than instructing against it, and it is a fence,
not a sandbox: the refusals are enforced at tp's own CLI, and §6.2 denies the file-writing tools a
path to the same values. A unit that strips the variable from its own environment, or edits a config
file through a shell, is outside what tp can prevent — the guarantee is that no unattended unit
reaches those decisions through a supported route.

### 5.2 Escalation

An escalation is a normal, expected outcome — not a crash. A unit that needs a user-only decision
runs `tp escalate --decision <name> --evidence <text> [--option <text>]…`, which writes
`$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` — `{decision, unit_kind, unit_id, phase, evidence,
options[], at}` — and exits 2. `decision` is one of `skip-gate`, `raise-review-cap`,
`raise-audit-cap`, `import-force` or `other`. Outside a run (`TP_RUN_DIR` unset) `tp escalate` is a
usage error, so the command cannot be used to fabricate a record.

**The record, not the exit code, is the signal.** The driver spawns a harness, not `tp` itself, so
the harness's exit code need not carry the inner command's — the record is what the driver tests. A
unit that wrote a valid record stops the run with `stop_reason: escalation` whatever it exited with
— subject only to §3.4's precedence, where a cycle that became releasable in the same iteration
still records `converged` — is not counted as a failed attempt, and does not set `last_failure`. A unit that wrote none, or one
that fails schema validation, is judged by its §3.3 predicate and its exit code as usual.

The record is per unit, so concurrent siblings never clobber each other; when
an iteration produces several, the driver reports the lowest `seq` and preserves them all.
`tp run` leaves every artifact in place and invokes the configured `notify_cmd` if one is set.

`notify_cmd` is invoked on every non-converged stop, not on escalation alone. It is exec'd without a
shell, split on whitespace, with `TP_STOP_REASON`, `TP_RUN_ID` and — on an escalation —
`TP_ESCALATION_PATH` in its environment. Its exit code is reported and changes nothing; a
`notify_cmd` that fails never changes the run's `stop_reason`.

Resuming after an escalation is the operator making the decision and running `tp run` again. Nothing
about the run is replayed; the cycle state already reflects everything the stopped run completed.

## 6. The plugin

### 6.1 Layout

The repository already publishes `.claude-plugin/marketplace.json` with a single skill entry. This
version fills the plugin out at the same root: `.claude-plugin/plugin.json` for identity, the
existing `skills/tp`, plus `hooks/` and `agents/`.

The Go binary is **not** shipped inside the plugin. A marketplace is a git repository and
cross-platform binaries do not belong in one; installation stays Homebrew and `go install`. The
plugin instead preflights: if `tp` is absent from `PATH` or older than `0.35.0`, it fails with the
install command rather than degrading quietly. The minimum is `plugin.json`'s own version, compared
against `tp --version`; the check runs in the `SessionStart` hook (§6.2), which is the only hook
that fires before a unit does any work.

### 6.2 Hooks

| Event | Behaviour |
|---|---|
| `SessionStart` (every matcher) | Preflight `tp`'s presence and version (§6.1), then inject the cycle's orientation as additional context |
| `PreToolUse` on `Write`, `Edit`, `MultiEdit`, `NotebookEdit` | Deny writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json` |
| `Stop` | Inside a role unit whose findings file fails §3.3's predicate and with no escalation record, block once with the reason |

The session hook deliberately does not branch on the matcher. The `clear` source is not reliably
reported on every client, and an orientation that is occasionally redundant costs far less than one
that is occasionally missing. Its payload is `tp resume --compact` and nothing else, so its size is
bounded by an existing command's compact output.

The write denial is what turns the scope fence from prose into enforcement. The fence has been
documented since v0.30.0 and a unit that ignores it produces exactly the failure the postmortem
described: a boundary that exists only as instruction text. Shell tools are deliberately outside the
matcher: the denial exists to stop hand-editing, not to sandbox, and tp's own commands rewrite
`*.tasks.json` on every close.

The stop hook is deliberately scoped to `review-role` and `audit-role`, the two kinds whose durable
write is a file at a path the hook already holds in its environment: it fires when `TP_UNIT_KIND`
ends in `-role` and applies §3.3's predicate to `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part` —
present and wholly parseable; the hook runs inside the child, before the driver's rename, so the
`.part` name is the one it can see — alongside `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json`. The predicate is two
conditions over one file precisely so a hook can implement it without tp's state readers; the
oracle and the driver evaluate the same two, and test 52 asserts the three agree on a malformed
file, which is the case where a divergence would show. Every other kind's predicate needs tp's own state
readers, which is the driver's job, not a hook's — the driver evaluates §3.3's column for all eight
kinds after the child exits, so nothing goes unchecked. It never reads the run state. The hook
must consult `stop_hook_active` and block at most once per unit. A hook that always blocks produces
an agent that can never finish, which is the loop this whole version exists to make terminating.

### 6.3 Agents

`agents/` declares `tp-implementer`, `tp-reviewer` and `tp-auditor` with their own tool restrictions.
The reviewer and auditor may write exactly one path — their own findings file at
`$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part`, plus the escalation record `tp escalate` writes on
their behalf — and are denied every other write. The driver renames `.part` to
`role-$TP_UNIT_ID.ndjson` when the child exits 0, so the final name exists only for a unit that ran
to completion and a crashed unit's empty file is never mistaken for a clean role. When
`TP_ROUND_DIR` is set, `tp review` and `tp audit` emit that path per role — each role's own prompt
names `role-<that role's id>.ndjson.part` — so the prompt and the allowlist name one filename rather than
two; the `review-record` and
`audit-record` units merge `$TP_ROUND_DIR/role-*.ndjson`, which holds exactly their own round's role
files, and write the merged set to `$TP_ROUND_DIR/merged.ndjson`, deliberately outside that glob.
A record unit writes that file only when it is absent: once written it accumulates the dispositions
`review-resolve` and `audit-fix` add, so a re-run record unit re-records from it rather than merging
over it. Recording is idempotent — re-recording a round already recorded rewrites that round's entry
rather than adding one — so a retry after a partial failure converges on the same state.

**The two record kinds do not share that guarantee, and the asymmetry is the shipped recorders'.**
`tp review --record` refuses a row pre-resolved `fixed` and exits 1 — a fix means the spec changed
and the round's findings were read against older text. `tp audit --record` applies no such rule: it
counts every row whose status is not exactly `PASS` and reads no disposition at all, so a `fixed`
audit row re-records as an open finding. That side matters most, because `audit-fix` is the kind that
produces `fixed` dispositions.

The consequences are stated as what happens, not as a licence the driver does not have. A
`review-record` unit whose merged file has accumulated a `fixed` row exits 1, leaves no durable
write, and is therefore a failed attempt under §3.3.1: it exhausts its attempts and stops the run
with `unit-failure` (§3.4). Nothing turns a refused record into a new round — §3.4 is explicit that
no stop reason ever records one — so the remedy is the operator's, and the run surfaces the state
rather than recovering from it.

Two claims in the paragraph above are this release's to implement rather than the recorders'
today: both `--record` paths currently append `len(rounds) + 1` and write a new round file, so
"rewrites that round's entry rather than adding one" is a behaviour a task must own, not an
existing one to rely on.
Outside a run the emitted filename is unchanged.

The definitions carry tool restrictions only: review and audit role content stays in the corpus
(`.tp/reviewers/`, `.tp/auditors/`) and reaches the unit through the prompt `tp review`/`tp audit`
already emits, so no role text is duplicated here.

The `claude` template appends `["--agent", "<name>"]` per unit kind: `tp-implementer` for
`implement`, `tp-reviewer` for `review-role`, `tp-auditor` for `audit-role`, and none for the record,
resolve, decompose and fix kinds, which need the ordinary tool set.

A runner that cannot load a plugin gets no agent definitions, and the unit runs without them. The
restrictions are defence in depth; the durable contract is the brief and the tp commands the unit
runs, which are identical in both paths.

### 6.4 Hook safety

Every hook the plugin ships declares a `timeout` of 10 seconds and exits non-zero rather than
hanging. An unbounded hook strands an unattended run past its wall-clock budget with no diagnostic,
and it is the one failure mode in this design that the driver cannot observe from outside.

## 7. Workflow fields

New project-level defaults, resolving at read time through the existing layered path:

| Field | Default | Range |
|---|---|---|
| `run_max_units` | 100 | 1-10000 |
| `run_max_wall_clock_seconds` | 28800 | 60-604800 |
| `run_max_budget_usd` | 0 (disabled) | 0-10000 |
| `run_max_unit_budget_usd` | 0 (flag omitted) | 0-1000 |
| `run_max_unit_retries` | 1 | 0-5 |
| `runner` | `claude` | Built-in name or object |
| `notify_cmd` | unset | Any command string |

Budget fields are decimal dollars. The fields resolve through the existing precedence, which for
workflow fields is **task override > project config > built-in**: tp exposes no CLI flag and reads no
`TP_<FIELD>` environment variable for any of them, so the two upper layers named elsewhere in tp's
documentation are absent here rather than merely unused. Two exceptions: `notify_cmd` is per-operator
rather than per-project and is read from `.tp/local.json` only, and `TP_RUNNER_SEAM` (§3.2.1) is a
test seam that outranks every layer.

An earlier draft of this section wrote the precedence as `CLI > env > task override > project config
> built-in`. The audit's spec-coverage role reported, across two rounds, that the top two layers
could not be verified because nothing implements them — a claim no test can settle either way. The
text now states what resolves.

`tp config --resolved`, `tp config --extract` and `tp validate --project` cover the new fields on the
same terms as every existing workflow field. Under `--compact`, `stop_reason` and the cap totals
survive; per-unit rows and log paths are stripped.

## 8. Documentation

`README.md`, `skills/tp/SKILL.md` and `CLAUDE.md` gain the unattended run: the `tp run` command
surface, the runner configuration, the unattended restrictions, and the plugin's installation and
hook behaviour. The reset-native section of `CLAUDE.md` is rewritten around `tp run` — the
subagent-per-unit guidance becomes the fallback for interactive sessions rather than the primary
model.

## 8a. Signals the driver reads

Five measured defects in the signals an unattended driver consumes. Each is a required behaviour of
this release, not context. The measurements are in `spec/0.35.0-candidates.md` §1, §3, §7 and §10,
and — for §8a.4 — in this spec's own round 4.

### 8a.1 Clustered counts

`tp review --merge` deduplicates on `(location, class)`, and roles compose
their class slugs independently, so two roles reporting one defect almost never collide. Measured
over nine rounds on an external spec: merge removed 7 duplicates while 63 blocking findings shared a
`location` with another role's finding. A human reads that as "more to do"; an unattended driver
reads it as rounds remaining.

`tp review --merge` and `--status` gain `location_clusters`: one entry per `location` carrying
findings from more than one role, `{location, roles[], severities[], count}`. Class vocabularies are
not unified — roles are supposed to name things through their own lens — and every record stays
separate. This is reporting only: convergence arithmetic, the stored `clean` flag and the
`--status --check` exit code are unchanged, so Non-Goal 4 holds.

### 8a.2 Phase-honest mechanization advice

Registered `workflow.checks` run in the review phase only, so
a check whose subject a later phase writes can never verify it — and tp tells every reviewer to stop
reporting a mechanized class, so registering early suppresses a finding class while verifying
nothing. `next_action`'s mechanize branch (precedence step 3) gains the qualifier: it names the
class and states that a check is worth registering only when the artifact it measures already exists
in the review phase. `skills/tp/SKILL.md` carries the same qualifier at its two sites that advise
registering a check — the mechanize-candidate rule, and step 3 of the `next_action` precedence list —
in the same task as the emitter change. (Workflow D step 3 is the audit merge-and-record step and has
no mechanize branch; an earlier draft named it here, and the audit's spec-coverage role reported the
mismatch twice before this was corrected.)

### 8a.3 Truncated audits report as truncated

`tp audit`'s auto-detect caps its file set at 50, says so
in one stderr line that `--quiet` erases, and reports `file_summary.total_files` as the cap rather
than the total. `file_summary` gains `truncated` (bool) and `total_changed` (the pre-cap count),
`total_files` keeps reporting the audited count, and the notice names both numbers. The flag is in
the payload, so `--quiet` cannot erase it.

### 8a.4 A dropped role does not merge clean

`tp review --merge` and `tp audit --merge` warn on stderr
for each malformed NDJSON line and still exit 0, so a role whose whole file fails to parse is
silently absent from the merged set and `--record` then freezes an undercounted round. Measured on
this spec's own round 4: a reviewer emitted 13 findings with a trailing comma per line, every line
was skipped, and the round recorded 33 findings instead of 45 — one of the five roles missing, with
`--quiet` able to erase the only signal. The merge payload gains `inputs`: one entry per input file
`{path, parsed, skipped}`, and an input with at least one content line, none of which parsed, makes
`--merge` exit 1. Blank and whitespace-only lines are neither parsed nor skipped and never
contribute to that condition. A zero-byte file stays the documented way a role reports nothing found and keeps
exiting 0, so a clean round is unaffected. The same rule applies to both merges, because an
unattended driver reads the exit code alone.

### 8a.5 Arity violations are usage errors

An arity violation exits 1 — the code for "ran and failed" —
while an unknown command exits 2. Every command's `Args` validator exits 2, matching §9's definition
of a usage error, so exit 2 means uniformly "tp did not run the request", and 1 stays "it ran and
failed" — the distinction a driver branches on. Exit 2 is deliberately not subdivided further: a
refusal and a typo are the same instruction to the driver, which is to stop and hand the unit's log
to a human. What separates an *escalation* from either is the record (§5.2), and a unit that refuses
to write one is a unit failure, which is the same stop. `tp show`'s arity hint names the missing
argument rather than claiming to name the failing object.

## 9. Non-Goals

1. **No memory or retrieval layer.** No embeddings, no vector store, no learning extraction. `tp
   resume` plus `last_failure` is the entire handoff, and the prior art shows the rest is only needed
   when the state is untyped prose.
2. **No self-clear.** No supported mechanism lets an agent reset its own caller's context, and this
   version does not pretend otherwise; the fresh process is the reset.
3. **No scheduler.** `tp run` runs until it stops. Running it on a cron, a routine, or a CI trigger
   is the operator's business and needs nothing from tp.
4. **No convergence policy changes.** v0.33.0 owns what counts against convergence. `tp run` obeys
   the resolved policy and never relaxes it to reach an exit.
5. **No parallel writing units.** Read-only role units fan out; anything that writes shared state
   runs alone.
6. **No prose parsing.** The driver reads exit codes, disk state, and the single spend key named in
   §3.2. It never infers status from a unit's natural-language output.
7. **No new decomposition or review semantics.** Units are the existing phases, not a new task model.

## 10. Tests

### 10.1 The loop

1. A fake runner seam (`TP_RUNNER_SEAM`) records every invocation and returns scripted exit codes, in the
   same spirit as the existing `TP_HC` seam, so the loop is testable without an agent.
2. The loop advances through implement, review and audit phases and stops with `converged` when the
   oracle reports `phase` `release`.
3. Each cap produces its own `stop_reason`, and no cap records a round, marks a phase converged, or
   closes a task its own unit did not close.
4. A cap tripping while a blocking finding is open does not mark the phase converged.
5. Role units run concurrently and every other kind does not overlap them, asserted from the
   seam's recorded spawn and exit timestamps.
6. A crashed driver loses nothing: a second `tp run` over the same state resumes at the same unit.
7. An empty `next_units` on a non-releasable cycle stops the run with `no-units` on the first
   iteration rather than re-polling.
8. A unit is attempted `1 + run_max_unit_retries` times.
9. A child's own `tp` write succeeds while the driver holds the run lock, and a second `tp run`
   exits 4.

### 10.2 Runner and run state

10. Runner templates resolve every placeholder, and an unresolved placeholder is a usage error before
    any child is spawned.
11. Spend is read from the runner's final log line; a runner declaring no `spend_key` reports `spend`
    null and leaves `cap-budget` inert.
12. A run.json row exists with null `exit_code` before its child exits and is updated after.
13. `tp run --dry-run` spawns nothing and lists the units that would run.
14. `tp run --status` reports an in-flight run, a converged run and each stopped run, and carries the
    divergence signals on an audit-phase stop.
15. `.tp/run-<base>.json`, `.tp/runs/`, `.tp/rounds/` and `.tp/last_failure-<base>.json` are git-ignored by the shipped
    `.tp/.gitignore`.
16. Each numeric §7 field takes its documented default, accepts both range endpoints, and rejects
    one step outside each; `runner` and `notify_cmd` are checked for their defaults and shapes only;
    a task override beats project config, which beats the built-in — the whole precedence §7 states,
    since no CLI flag or environment layer exists for these fields to test above it; and `notify_cmd`
    is read from `.tp/local.json` only.
17. A retried unit gets a fresh log path, and its role findings file is deleted before the attempt,
    so a failed attempt's leftover cannot satisfy the retry's predicate.
18. Each durable-write predicate is decidable from the artifacts it names with no baseline, and an
    `audit-fix` that closes its finding without a code change passes.

### 10.3 Unattended mode and escalation

19. Under `TP_UNATTENDED=1`, each of the four user-only operations exits 2 with its hint; a cap set
    to an equal or lower non-zero value exits 0; setting a budget field to 0 while a non-zero value
    resolves exits 2; and each operation exits 0 without the variable.
20. A `runner.env` entry for `TP_UNATTENDED` does not override the driver's value: the child sees `1`.
21. An escalating unit stops the run with artifacts intact, is not counted as a failed attempt, sets
    no `last_failure`, and invokes `notify_cmd` when configured. An exit 2 with no escalation record
    is a failed attempt, which stops the run only once the unit's attempts are exhausted.
22. `tp escalate` writes the per-unit record and exits 2; two concurrent siblings each write their
    own; outside a run it is a usage error.
23. `last_failure` is written to `.tp/last_failure-<base>.json` on a non-zero unit exit, surfaced by `tp
    resume` and `tp brief`, survives into the next run, and is cleared on the next success of that
    unit. A success of a different kind with the same id does not clear it.

### 10.4 The plugin

24. The plugin validates with `claude plugin validate`, and its hook payloads are asserted against
    fixtures.
25. The `PreToolUse` deny covers `.tp-review/`, `*.tasks.json`, `.tp/config.json` and
    `.tp/local.json`, and does not fire for tp's own writes through a shell.
26. The Stop hook does not block a role unit that wrote an escalation record, does block a role unit
    that ended with neither its findings file nor a record, and never fires for a non-role kind.
27. Under `TP_ROUND_DIR`, `tp review` and `tp audit` emit the round-scoped findings path, and the
    record unit merges that directory.

### 10.5 Signals the driver reads

28. `location_clusters` groups a location reported by two roles under different classes, and leaves
    the recorded finding count, the `clean` flag and the `--status --check` exit code unchanged.
29. `next_action`'s mechanize branch carries the phase qualifier.
30. A file set truncated by the cap reports `truncated` true and `total_changed` above
    `total_files`, under `--quiet` as well.
31. An arity violation exits 2 for every command, including cobra's own built-ins, and `tp show`
    with no argument names the missing argument.
32. `--merge` reports `inputs` with per-file parsed and skipped counts, exits 1 when an input has
    content lines and none parsed, and exits 0 when every input is zero-byte or blank — for both
    merges.
33. `run_max_units`, `totals.units` and `--status`'s units-done all count attempts, so a run with
    one retried unit reads the same number in all three.
34. Two simultaneously satisfied stop conditions record the higher-precedence reason.
35. `tp run --status` reports `run_state` as `in_flight`, `crashed` or `stopped`, using the run lock
    to separate the first two.
36. A round directory survives a driver death: a second run's record unit merges the role files the
    first run already collected, and a re-run role overwrites its own file rather than adding one.
37. Every child receives the documented environment variables in their documented forms —
    `TP_RUN_ID` a ULID, `TP_RUN_DIR` absolute, `TP_ROUND_DIR` derived from phase and round, `TP_FILE`
    the driver's resolved task file — with `TP_ROUND`/`TP_ROUND_DIR` present only in a round-based
    phase.
38. `next_units` carries `{kind, id, brief_command}` plus `round`, `next_action` renders
    `next_units[0]`, and a non-concurrent kind is never returned alongside another unit.
39. The Stop hook blocks at most once per unit: a second invocation with `stop_hook_active` set
    allows the stop.
40. Under `TP_UNATTENDED=1` a raise is refused against the value §7's precedence actually resolves,
    whatever an environment variable of the same name says — §5.1 explains why the environment half
    of that sentence proves nothing on its own, so the discriminating assertion is the resolved
    value, not the variable. `runner` and `notify_cmd` cannot be set at any layer.
### 10.6 Invariants across components

41. `SIGINT` and `SIGTERM` stop the run with `interrupted`, letting in-flight children finish and
    spawning no new unit.
42. `notify_cmd` fires on every non-converged stop, is exec'd without a shell, carries
    `TP_STOP_REASON`, `TP_RUN_ID` and (on escalation) `TP_ESCALATION_PATH`, and its own failure does
    not change `stop_reason`.
43. Every shipped hook declares a 10-second timeout and exits non-zero rather than hanging.
44. The `claude` template appends the documented `--agent` per unit kind, and none for the kinds
    that take no agent.
45. A resumed round's `next_units` omits every role whose findings file satisfies §3.3's predicate,
    and the driver deletes the findings file of every role unit the oracle did return before
    spawning it.
45a. A round whose every role file satisfies that predicate returns exactly one record unit for that
    round, and a round whose panel is empty or unresolvable returns none — the handover between
    test 45's omission rule and §4.1's record emission.
46. `TP_ROUND` and `TP_ROUND_DIR` are unset, not empty, when the oracle reports `round` null.
47. A unit that wrote a valid escalation record stops the run as an escalation whatever exit code
    the harness returned.
48. A role unit whose findings file exists but holds an unparseable line fails its predicate.
49. `--merge` exits 0 on a file of only blank lines and 1 on a file whose sole content line is
    malformed.
50. The `SessionStart` hook fails with the install command when `tp` is absent or below the
    plugin's minimum version.
51. A unit that exits 0 with its durable write absent, and one that exits non-zero with it present,
    are both failed attempts.
52. The oracle, the driver and the Stop hook apply the same role predicate: a role that left a
    malformed file is re-run, not skipped.
53. `tp audit --resolve` and `--resolve-all` dispose audit rows with the same dispositions and flags
    as their review counterparts, and additionally accept a `role:item_id` selector, and an `audit-fix` unit that disposes its row without a code change succeeds.
54. `tp run` exits 0 on `converged` and 4 on every other `stop_reason`.
55. The `tp-reviewer` and `tp-auditor` agent definitions permit writing only the role's own findings
    file and its escalation record, and deny every other path.
56. A per-kind `runner` map dispatches each kind to its own template and unlisted kinds to
    `default`; a map without `default` and a runner object without `cmd` are both usage errors.
57. A re-run record unit preserves the `wontfix` and `duplicate` dispositions already written into
    `merged.ndjson`, and re-recording an already-recorded round rewrites that round's entry rather
    than adding one. Its complement, on the review path only: a `merged.ndjson` carrying a `fixed`
    row makes the record unit exit 1 with the recorder's re-review hint, and the audit path records
    that same row as an open finding instead (§6.3).
58. `tp run --dry-run` exits 0 and writes no run state; `tp run --status` exits 0 when it can report
    and 3 when no run state exists; neither takes the run lock.
59. A role findings file ending in a newline, and one holding only blank lines, both satisfy the
    role predicate's blank-line rule.
60. A template omitting `{log_path}` gets the driver's redirect; one using it owns the file and is
    not redirected.
61. `tp done` writes `last_failure` with the gate's output when its gate fails under `TP_UNIT_KIND`,
    and writes nothing when that variable is absent.
62. Every child is spawned with stdin closed: a runner that reads stdin sees EOF rather than hanging.
63. A cap that trips mid-iteration lets the in-flight children finish and kills nothing, and the run
    overshoots by at most that iteration.
64. `TP_UNATTENDED` activates on any present, non-empty value other than `0`, and does not activate
    on unset, empty or `0`.
65. A role unit that dies after creating its file leaves only a `.part`, so the oracle re-runs it.
### 10.7 Coverage added in review

66. A runner declaring no `spend_key` is reported once at run start, and a per-kind map with one
    metered kind and one unmetered kind names the unmetered one.
67. The Stop hook allows the stop when the role's `.part` satisfies the predicate.
68. Under `--compact` the run surface keeps `stop_reason` and the cap totals and strips per-unit
    rows and log paths.
69. `tp config --resolved`, `tp config --extract` and `tp validate --project` each cover all seven
    new §7 fields.
70. A driver that cannot exec `runner.cmd`, or cannot write its run directory, stops with
    `driver-error` and exit 4 without charging a unit a failed attempt.

### 10.8 Existing tests this change invalidates

`tp resume`'s JSON shape gains `next_units` and `last_failure`, so every assertion pinning its exact
object set moves in the same task as the field. The `.tp/.gitignore` fixture gains four entries.
Every test asserting exit 1 for a wrong argument count moves to exit 2 in the same task as §8a's
arity change, and every test pinning `--merge`'s exit 0 on a malformed input moves in the same task
as §8a.4; no other command's exit codes change outside `TP_UNATTENDED=1`, which no current test
sets.
