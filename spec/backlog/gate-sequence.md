# tp — The gate can be run, checked and set per spec

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `gate-sequence-measurements.md` beside it; this file stands without them. The
slug predates the 2026-09-11 rescope and stays because shipped artifacts cite it.

Class: **tool** — it adds a verb, a check at the writes that set a gate, and one settable field; the
loop that reviews it is not the thing it changes. Budget it at the tool-class median in `CLAUDE.md`'s
*What a cycle costs*.

## 1. The decision

**Context.** A field report (WB-3155) met the quality gate three ways, and each reproduces at `HEAD`
(sidecar, *Field report WB-3155, verified 2026-09-11*):

- **A gate that cannot run is accepted wherever it is written and discovered at the first close.**
  `tp init --quality-gate` and `tp import` take a gate whose first command does not exist and exit
  0, and `tp validate` exits 0 over it. The first `tp done` exits 4 with the shell's own
  *command not found* code in `exit_code` and a hint offering `--skip-gate` — advice for closing
  over a failing check, given when no check ran.
- **One spec's gate cannot be set.** `tp set --workflow quality_gate=` is refused as *authored by tp
  init*; the project-layer write lands in `.tp/config.json`, which every other spec resolves, and is
  shadowed by a task-layer value already present. `skills/tp/SKILL.md`'s Step 1 tells the reader a
  spec may deviate from the shared gate; after `tp init`, tp offers no supported way to make it.
- **There is no way to ask tp to run the gate.** `tp gate` is an unknown command, so the only run
  tp offers is a close, and checking a gate beforehand means copying its string out of the config.

**Decision.**

1. `tp gate` runs the gate that resolves for the task file, through the one executor every closing
   command uses (§2).
2. Every write that sets a gate checks once that its first command resolves, and warns when it does
   not (§3).
3. A close whose gate exits 126 or 127 gets a hint naming the command that could not run, and not
   `--skip-gate` (§4).
4. `tp set --workflow quality_gate=<cmd>` writes the task-layer value, and under `TP_UNATTENDED=1` a
   `quality_gate` write is refused at either layer (§5).

**Consequences.** Workflow A gains a step: run `tp gate` once after `tp import`, before the first
close. The §3 check catches a command that does not exist; only a run catches one that exists and
rejects its arguments, which is the shape the field report describes. `skills/tp/SKILL.md`'s Step 1
line on per-spec deviation becomes true, and `skills/tp/REFERENCE.md`'s `TP_UNATTENDED` table gains a
`quality_gate` row. The string form of `quality_gate` is unchanged.

**Alternatives.** Running the whole gate at `tp import` was rejected: at import nothing is
implemented yet, so a red gate cannot be told from a gate that cannot run, and a suite run is too
expensive to be a side effect of writing a plan. The ordered array of named entries this spec used to
lead with is deferred (§6).

## 2. `tp gate` runs the resolved gate

**`tp gate` resolves the gate the way a close does** — the same task-file discovery, the same
layers, the same working directory and the same `gate_timeout_seconds` — and runs it once. It reports
the resolved command, the layer it resolved from, the gate's exit code and its output tail: the keys a
failing close already carries, plus the layer, so a driver reads one shape from both and an operator
facing a masked gate sees which layer won.

**Exit 0 when the gate passes and 4 when it fails**, with the gate's own code in `exit_code` — the
exit a failing close already uses. Propagating the gate's code as tp's would let a gate that exits 2
read as a tp usage error. **When no gate resolves, `tp gate` fails** with a hint naming the project
setter: a run of nothing is not a green gate, and a CI step that invokes `tp gate` must not pass on an
empty config.

**One executor.** tp runs the gate today through two code paths that do not call each other — one for
`tp done --batch`, the other for `tp done <id>`, `tp done <id1> <id2>` and `tp close` (sidecar, *The
two executors*). `tp gate` does not add a third: it and every closing command share one, so the
directory and the timeout a gate runs under cannot differ between asking and closing.

**`tp gate` writes nothing** — no task-file write, no round, no failure record, inside a unit or out.
It runs and reports.

## 3. A gate is checked when it is written

**Every write that sets a gate checks once that the gate's first command resolves**: `tp init`, over
the gate that resolves for the file it creates, whether or not `--quality-gate` supplied it;
`tp import`; and `tp set --workflow quality_gate=` at either layer. *Resolves* means what the shell
that runs the gate would find from the gate's working directory: a builtin, an executable path, or a
name on `PATH`.

**It warns and does not refuse.** A notice on stderr names the command, the payload names it too, and
the write's exit code is unchanged. The check measures this machine at write time while the gate runs
later and perhaps elsewhere — a CI runner, a machine that installs its tools after planning — so a
refusal would make installing the tools a precondition of writing a plan.

**Only the first command, and the limit is stated rather than hidden.** A later command in an `&&`
chain, and a first command that exists but rejects its arguments, both pass. Checking the whole string
is parsing shell; running it is §2's job, and §1's Workflow A step is where that run happens.

## 4. A gate that cannot run says so at close

**When a close's gate exits 126 or 127, the hint says the gate could not run a command** — naming it
when the §3 check identifies it — and points at fixing the gate, not at `--skip-gate`. Those are the
shell's codes for *not executable* and *not found*, and a close over them records a skipped check that
never ran, which is the wrong record. Under `TP_UNATTENDED=1` the hint points at `tp escalate`
instead, because §5 refuses the gate write on that path and a hint naming a command that exits 2 there
is worse than none. Every other exit keeps the hint it has, and `--skip-gate` itself is unchanged.

## 5. A spec can set its own gate

**`tp set --workflow quality_gate=<cmd>` writes the task-layer value** for the task file the command
resolves, as `--project` already writes the project one, and `tp config --resolved` reports it with
source `override`. A spec that needs a different gate gets one without `tp import --force` and
without changing the gate every other spec resolves.

**Under `TP_UNATTENDED=1`, a `quality_gate` write is refused at either layer**, on the field alone and
with an escalation hint — the rule the `runner` field already follows, and for the same reason: the
value is a command tp executes. A unit that can rewrite its own gate can close over anything, which is
`--skip-gate`'s effect without its record. The project-layer write is not fenced at `HEAD` (sidecar,
*Observed while verifying item 3*), so the fence closes a route that exists as well as the one this
release opens.

`commit_strategy` stays authored by `tp init` alone; this release moves `quality_gate` only.

## 6. Non-Goals

1. **The ordered array of named entries is deferred.** Its benefit is unmeasured, and the string form
   already identifies the failing step: a failing close carries that step's own `exit_code` and an
   `output_tail` that ends at it. The sidecar keeps the decision and its rows verbatim under
   *Deferred at the 2026-09-11 rescope: the named-entry array*. **Reopen condition:** a driver or a
   skill procedure that must branch on which gate step failed and cannot do so from `exit_code` and
   `output_tail`.
2. **Chores, not this release — plain commits, no cycle:** CI invoking `tp gate` instead of restating
   the gate (once this release ships), the workflow pin guard's `.yaml` and per-file coverage, the
   load-sensitive hook-timing test's constant, and the `NewRootCmd` second-call fence. Each is this
   repository's own tooling and none changes what tp does for a user; their decisions are in the
   sidecar under *Chores moved out at the 2026-09-11 rescope* and *Decided at the 2026-09-08 decision
   pass*.
3. **No change to when a close runs the gate, or to `--skip-gate`.** §4 changes a hint, not the flag.
4. **The mutation-run check is not a gate step.** It lives in `spec/backlog/mutation-run-check.md`
   as a script, and excludes itself from the per-task gate.
5. **No gate procedure ships here.** The red-gate procedure (`spec/backlog/red-gate-procedure.md`) is
   a doc note that runs on today's `&&` gate and no longer waits for this release.

## 7. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Rows 6, 9, 12 and 14
quote their `HEAD` value, observed on the fixture the sidecar describes; the other rows' subjects do
not exist at `HEAD`, so their pairs of counts belong to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | with a gate that writes its working directory to a file, run from a shell whose directory is not the task file's, `tp gate`, `tp done <id>`, `tp done <id1> <id2>`, `tp done --batch` and `tp close` all record the same directory | give `tp gate` its own executor running in the process's working directory |
| 2 | §2 *timeout* | with `gate_timeout_seconds` at 1 and a gate that sleeps longer, `tp gate` stops the gate and reports a failure, as a close does | an executor for `tp gate` that ignores the resolved timeout |
| 3 | §2 *exit* | `tp gate` exits 0 on a gate of `true`; on `exit 33` it exits 4 with `exit_code: 33`, and on `exit 2` it exits 4, not 2; its `gate_cmd`, `exit_code` and `output_tail` equal a failing close's on the same gate, and it names the layer | propagate the gate's code as tp's exit code, so a gate exiting 2 reads as a usage error |
| 4 | §2 *empty* | with no gate resolving, `tp gate` exits non-zero and its hint names the project setter | exit 0 on an empty gate, which lets a CI step invoking `tp gate` pass on an empty config |
| 5 | §2 *writes nothing* | a failing `tp gate` with `TP_UNIT_KIND` set leaves the task file and every file under `.tp/` byte-identical | record the failure as a close does |
| 6 | §3 | `tp import` of a task file whose `workflow.quality_gate` is `no-such-gate-cmd --check && true` names `no-such-gate-cmd` on stderr and in the payload, and exits 0; at `HEAD` it exits 0 and names nothing | `HEAD`, which checks nothing |
| 7 | §3 *builtin* | a gate of `cd . && true` passes the check with no notice | resolve through `PATH` alone, which flags the builtin `cd` |
| 8 | §3 *layers* | `tp init` without `--quality-gate`, in a project whose `.tp/config.json` gate's first command does not resolve, names that command | check only a value given on the command line, which misses the gate the new file resolves |
| 9 | §4 | on row 6's fixture, `tp done t1 <reason> --commit <sha>` and a `tp done --batch` row for `t1` each carry a hint naming `no-such-gate-cmd` and not `--skip-gate`; at `HEAD` both offer `--skip-gate` | `HEAD`'s hint, unchanged for 126 and 127 |
| 10 | §4 *unattended* | the same close under `TP_UNATTENDED=1` carries a hint naming `tp escalate` and not the gate setter | the attended hint on both paths, which names a command §5 refuses there |
| 11 | §4 *scope* | a gate exiting 1 keeps the `--skip-gate` hint | give every failing exit the new hint |
| 12 | §5 | `tp set --workflow quality_gate="true"` exits 0 and `tp config --resolved` reports `true` with source `override`; at `HEAD` the set exits 2 | `HEAD`'s read-only field set, which still holds `quality_gate` |
| 13 | §5 *scoped* | after row 12, a second task file in the same project still resolves the project gate | write the value into `.tp/config.json`, which every spec resolves |
| 14 | §5 *fence* | under `TP_UNATTENDED=1`, `tp set --workflow quality_gate=true` and `tp set --workflow --project quality_gate=true` both exit 2 with an escalation hint and leave the task file and `.tp/config.json` byte-identical; at `HEAD` the project write exits 0 and writes | fence only the task-layer write this release adds, leaving the project-layer route open |
