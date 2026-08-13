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

A unit's `id` is its durable subject: the task id for `implement`, the role id for a role unit, the
round number for a record unit, the finding index for `audit-fix`, and the spec base name for
`review-resolve` and `decompose`. An id is unique within its kind, so `(kind, id)` identifies a
unit — which is the key `last_failure` clears on (§4.2).

`tp run` takes a distinct run-scoped lock at `.tp/locks/run-<base>.lock`, held for the whole run, so
a second `tp run` over the same task file exits 4. It never holds the task-file write lock while a
child is in flight: that lock stays the children's, taken and released by each `tp` write they make.
A crashed `implement` unit leaves its task `wip`; the next run's `implement` unit resumes that same
task, which is what `tp next` already does.

Every child is spawned with `TP_RUN_ID`, `TP_UNIT_ID`, `TP_UNIT_KIND`, `TP_UNIT_SEQ`, `TP_ROUND`,
`TP_RUN_DIR` and `TP_ROUND_DIR` in its environment, applied on the same terms as `TP_UNATTENDED`
(§3.2). They are what let a unit address its own artifacts without the driver passing them through
prose. `TP_ROUND_DIR` is `$TP_RUN_DIR/<phase>-r$TP_ROUND`, so a round's role files sit in their own
directory and a record unit merges only its own round.

`seq` counts unit *attempts*, not units, so a retry gets a fresh `seq` and reuses no artifact path.
Before retrying, the driver removes the failed attempt's files from the round directory, leaving
only successful role files for the merge.

At spawn the driver writes `$TP_RUN_DIR/$TP_UNIT_SEQ-baseline.json` — `{head_sha, spec_hash,
task_count, round}` — captured immediately before the child starts. Every delta predicate in §3.3 is
measured against it, so the driver and the Stop hook read the same baseline.

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
A map missing `default` is a usage error.

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

`TP_RUNNER` is the test seam: when set, its value is the `cmd` of a runner object whose `args` are
`["{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}"]` and whose `spend_key` is
`total_cost_usd`, and it outranks every other source of `runner`. It is the env layer of the
existing precedence, so no new precedence rule is introduced. The fake runner writes a final log
line carrying that key, which is what makes the spend and budget-cap paths testable without an
agent.

#### 3.2.2 Spend

After a child exits the driver reads one number from the final line of that unit's log:
`total_cost_usd` for `claude`, `spend_key` for any runner that declares one. A runner that declares
none reports `spend: null` for its units, and `run_max_budget_usd` is inert for them — reported once
at run start, not silently. Reading that single key is not prose parsing (Non-Goal 6).

### 3.3 Unit kinds

A unit is the smallest piece of work that ends in a durable write. There are eight kinds:

| Kind | Ends in | Durable write | Concurrency |
|---|---|---|---|
| `implement` | One task committed and closed | The task's `status` is `done` | Alone |
| `review-role` | One role's findings file for the round | `$TP_ROUND_DIR/$TP_UNIT_SEQ-$TP_UNIT_ID.ndjson` exists | Parallel with sibling roles |
| `review-record` | The round merged and recorded | A review round file for `$TP_ROUND` exists | Alone |
| `review-resolve` | The spec revised and each addressed finding resolved | The spec's hash differs from `spec_hash` | Alone |
| `decompose` | Tasks imported into the task file | The task count exceeds `task_count` | Alone |
| `audit-role` | One role's results file for the round | `$TP_ROUND_DIR/$TP_UNIT_SEQ-$TP_UNIT_ID.ndjson` exists | Parallel with sibling roles |
| `audit-record` | The round recorded, convergence recomputed | An audit round file for `$TP_ROUND` exists | Alone |
| `audit-fix` | The code change for one audit finding committed | `HEAD` differs from `head_sha` | Alone |

Each predicate is absolute or measured against the unit's baseline file (§3.1), never against an
unstated "before". The column is the Stop hook's predicate (§6.2) and the driver's own success test,
so both read the same condition against the same baseline.

Each kind's brief comes from the `brief_command` the oracle supplies for it (§4.1). The driver
decides a unit succeeded from its exit code and from re-reading cycle state, never from its output.

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
| `cap-units` | Executed units reached `run_max_units` |
| `cap-wall-clock` | Elapsed seconds reached `run_max_wall_clock_seconds` |
| `cap-budget` | Accrued spend reached `run_max_budget_usd`; disabled when that value is 0 |
| `escalation` | A unit wrote an escalation record (§5.2) |
| `unit-failure` | One unit exhausted its attempts |
| `no-units` | The oracle reports no pending units and the cycle is not releasable |
| `interrupted` | The operator signalled the driver |

A unit is attempted `1 + run_max_unit_retries` times, so the default of 1 gives two attempts and 0
gives one attempt and no retry. Exhausting them stops the run with `unit-failure`.

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

Run state lives at `.tp/run.json`, which is git-ignored like `.tp/local.json`:

```json
{"run_id": "", "started_at": "", "stop_reason": null,
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

Each unit's runner output streams to `.tp/runs/<run-id>/<seq>-<kind>-<id>.jsonl`. The driver reads
only the spend key from the final line (§3.2) and never reads these into any context; they exist for
the operator and for post-hoc diagnosis. Verbose command output — a full `go test ./...` run, a lint
sweep — is the fastest way to poison an agent's window, and keeping it on disk is the cheapest
defence.

`tp run --status` prints the current or last run: phase, units done, spend against each cap, the last
unit's exit code and log path, and `stop_reason` when the run has ended. On a stop in the audit
phase it also carries `spec_coverage_clean_rounds`, `role_streaks` and `divergence`, verbatim from
`tp audit --status`, which is how §1's promise is kept.

`tp run --dry-run` prints the units the driver would execute next and exits without spawning
anything.

## 4. Oracle contract

### 4.1 One component owns "done"

`tp resume` is the single authority on whether the cycle is finished, and `tp run` holds no opinion
of its own. The runaway loop above burned eleven days because no participant was permitted to declare
the work complete; tp's inversion of that is that no participant except the oracle is permitted to.

`tp resume --json` gains `next_units`: an ordered array of the units the driver should execute now,
each carrying `{kind, id, brief_command}`. Concurrency is not repeated per entry — it is fixed per
kind by §3.3's table — and a non-concurrent kind is never returned alongside another unit. The array
is the machine surface, and the only field the driver parses. The existing `next_action` object
stays and stays the human-facing summary; its
`command`, `brief_command` and `payload` render `next_units[0]` when the array is non-empty.

The cycle is **releasable** when `phase` is `release`. `tp run` exits 0 with `stop_reason:
converged` on that condition alone. `next_units` is `[]` in two other cases — a phase whose work is
blocked, and a phase awaiting an operator decision — and both stop the run with `no-units` (§3.4).
`next_action.summary` names what the phase is waiting for.

### 4.2 `last_failure`

When a unit exits non-zero, or closes with a failed quality gate, tp records a `last_failure` object
at `.tp/last_failure.json`: `{unit_kind, unit_id, phase, exit_code, summary, at}`. `summary` is
tp-authored — the failing command and the gate's own output when a gate failed — never the child's
prose. The file holds at most one object: a second failure overwrites it, and a success clears it
only when `(unit_kind, unit_id)` matches — id alone collides, since both record kinds are identified
by a round number. It lives outside `.tp/run.json` because it must survive the run that wrote it; it
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

The cap comparison is against the currently resolved value; an equal or lower value is accepted and
exits 0, since lowering a budget cannot manufacture convergence.

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

**The record, not the exit code, is the signal.** A unit that exits 2 having written a record stops
the run with `stop_reason: escalation`, is not counted as a failed attempt, and does not set
`last_failure`. A unit that exits 2 without one, or with one that fails schema validation, is an
ordinary unit failure. The record is per unit, so concurrent siblings never clobber each other; when
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
against `tp --version`.

### 6.2 Hooks

| Event | Behaviour |
|---|---|
| `SessionStart` (every matcher) | Inject the cycle's orientation as additional context |
| `PreToolUse` on `Write`, `Edit`, `MultiEdit`, `NotebookEdit` | Deny writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json` |
| `Stop` | Inside a unit whose durable write is absent, and with no escalation record, block once with the reason |

The session hook deliberately does not branch on the matcher. The `clear` source is not reliably
reported on every client, and an orientation that is occasionally redundant costs far less than one
that is occasionally missing. Its payload is `tp resume --compact` and nothing else, so its size is
bounded by an existing command's compact output.

The write denial is what turns the scope fence from prose into enforcement. The fence has been
documented since v0.30.0 and a unit that ignores it produces exactly the failure the postmortem
described: a boundary that exists only as instruction text. Shell tools are deliberately outside the
matcher: the denial exists to stop hand-editing, not to sandbox, and tp's own commands rewrite
`*.tasks.json` on every close.

The stop hook's predicates are observable, not inferences: it is inside a unit when `TP_UNIT_KIND`
is set, the durable write is the condition that kind names in §3.3's fourth column — read against
`$TP_RUN_DIR/$TP_UNIT_SEQ-baseline.json` where that condition is a delta — and an escalation is
`$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json`. It never reads `.tp/run.json`. The hook
must consult `stop_hook_active` and block at most once per unit. A hook that always blocks produces
an agent that can never finish, which is the loop this whole version exists to make terminating.

### 6.3 Agents

`agents/` declares `tp-implementer`, `tp-reviewer` and `tp-auditor` with their own tool restrictions.
The reviewer and auditor may write exactly one path — their own findings file at
`$TP_ROUND_DIR/$TP_UNIT_SEQ-$TP_UNIT_ID.ndjson` — and are denied every other write. When
`TP_ROUND_DIR` is set, `tp review` and `tp audit` emit that same path as the role's output file, so
the prompt and the allowlist name one filename rather than two; the `review-record` and
`audit-record` units merge `$TP_ROUND_DIR/*.ndjson`, which holds exactly their own round's files.
Outside a run the emitted filename is unchanged.

The definitions carry tool restrictions only: review and audit role content stays in the corpus
(`.tp/reviewers/`, `.tp/auditors/`) and reaches the unit through the prompt `tp review`/`tp audit`
already emits, so no role text is duplicated here.

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

Budget fields are decimal dollars. The fields resolve through the existing precedence — CLI > env >
task override > project config > built-in — except `notify_cmd`, which is per-operator rather than
per-project and is read from `.tp/local.json` only.

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

Four measured defects in the signals an unattended driver consumes. Each is a required behaviour of
this release, not context. The measurements are in `spec/0.35.0-candidates.md` §1, §3, §7 and §10.

**Clustered counts.** `tp review --merge` deduplicates on `(location, class)`, and roles compose
their class slugs independently, so two roles reporting one defect almost never collide. Measured
over nine rounds on an external spec: merge removed 7 duplicates while 63 blocking findings shared a
`location` with another role's finding. A human reads that as "more to do"; an unattended driver
reads it as rounds remaining.

`tp review --merge` and `--status` gain `location_clusters`: one entry per `location` carrying
findings from more than one role, `{location, roles[], severities[], count}`. Class vocabularies are
not unified — roles are supposed to name things through their own lens — and every record stays
separate. This is reporting only: convergence arithmetic, the stored `clean` flag and the
`--status --check` exit code are unchanged, so Non-Goal 4 holds.

**Phase-honest mechanization advice.** Registered `workflow.checks` run in the review phase only, so
a check whose subject a later phase writes can never verify it — and tp tells every reviewer to stop
reporting a mechanized class, so registering early suppresses a finding class while verifying
nothing. `next_action`'s mechanize branch (precedence step 3) gains the qualifier: it names the
class and states that a check is worth registering only when the artifact it measures already exists
in the review phase. `skills/tp/SKILL.md`'s mechanize-candidate rule and Workflow D step 3 carry the
same qualifier, in the same task as the emitter change.

**Truncated audits report as truncated.** `tp audit`'s auto-detect caps its file set at 50, says so
in one stderr line that `--quiet` erases, and reports `file_summary.total_files` as the cap rather
than the total. `file_summary` gains `truncated` (bool) and `total_changed` (the pre-cap count),
`total_files` keeps reporting the audited count, and the notice names both numbers. The flag is in
the payload, so `--quiet` cannot erase it.

**Arity violations are usage errors.** An arity violation exits 1 — the code for "ran and failed" —
while an unknown command exits 2. Every command's `Args` validator exits 2, matching §9's definition
of a usage error, so a driver can separate a mistyped invocation from a failed one without reading
the message. `tp show`'s arity hint names the missing argument rather than claiming to name the
failing object.

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

1. A fake runner seam (`TP_RUNNER`) records every invocation and returns scripted exit codes, in the
   same spirit as the existing `TP_HC` seam, so the loop is testable without an agent.
2. The loop advances through implement, review and audit phases and stops with `converged` when the
   oracle reports `phase` `release`.
3. Each cap produces its own `stop_reason`, and no cap records a round, marks a phase converged, or
   closes a task its own unit did not close.
4. A cap tripping while a blocking finding is open does not mark the phase converged.
5. Role units run concurrently and every other kind does not overlap them.
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
15. `.tp/run.json`, `.tp/runs/` and `.tp/last_failure.json` are git-ignored by the shipped
    `.tp/.gitignore`.
16. Each §7 field takes its documented default, accepts both range endpoints, and rejects one step
    outside each; a CLI flag beats a task override, which beats project config; and `notify_cmd` is
    read from `.tp/local.json` only.
17. A retried unit gets a fresh `seq`, writes no path its failed attempt used, and the record unit
    merges only the successful attempt's file.
18. Each delta predicate is measured against the unit's baseline file, and a spec, `HEAD` or task
    count unchanged since spawn fails the predicate.

### 10.3 Unattended mode and escalation

19. Under `TP_UNATTENDED=1`, each of the three user-only operations exits 2 with its hint; a cap set
    to an equal or lower value exits 0; and each operation exits 0 without the variable.
20. A `runner.env` entry for `TP_UNATTENDED` does not reach the child.
21. An escalating unit stops the run with artifacts intact, is not counted as a unit failure, sets no
    `last_failure`, and invokes `notify_cmd` when configured. An exit 2 with no escalation record is
    a unit failure instead.
22. `tp escalate` writes the per-unit record and exits 2; two concurrent siblings each write their
    own; outside a run it is a usage error.
23. `last_failure` is written to `.tp/last_failure.json` on a non-zero unit exit, surfaced by `tp
    resume` and `tp brief`, survives into the next run, and is cleared on the next success of that
    unit. A success of a different kind with the same id does not clear it.

### 10.4 The plugin

24. The plugin validates with `claude plugin validate`, and its hook payloads are asserted against
    fixtures.
25. The `PreToolUse` deny covers `.tp-review/`, `*.tasks.json`, `.tp/config.json` and
    `.tp/local.json`, and does not fire for tp's own writes through a shell.
26. The Stop hook does not block a unit that wrote an escalation record, and does block one that
    ended with neither its durable write nor a record.
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

### 10.6 Existing tests this change invalidates

`tp resume`'s JSON shape gains `next_units` and `last_failure`, so every assertion pinning its exact
object set moves in the same task as the field. The `.tp/.gitignore` fixture gains three entries.
Every test asserting exit 1 for a wrong argument count moves to exit 2 in the same task as §8a's
arity change; no other command's exit codes change outside `TP_UNATTENDED=1`, which no current test
sets.
