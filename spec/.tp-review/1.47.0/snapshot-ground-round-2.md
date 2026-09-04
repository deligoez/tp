# tp v1.47.0 — The gate sequence

> **This file is decisions.** Its central one is not the array — it is that **CI stops restating the
> gate**. v0.36.0's audit spent four rounds trying to guard the restatement and closed none of it, and
> §1.1's five inputs are why. But a repaired guard *is* available — §1.1 records two, built and run —
> so the reason for removing the restatement is not that guarding it is impossible. It is that a
> guarded restatement is still two definitions of the gate, now with a third artifact keeping them in
> step.

## 1. Overview

`quality_gate` is a shell string, `.tp/config.json` holds it, and **CI restates it** as four separate
`run:` steps (`grep -n 'run:' .github/workflows/ci.yml` returns exactly four keys; `checkout` and
`setup-go` are `uses:` steps and are not among them). `TestCIRunsEveryStepOfTheProjectGate` ties the
two by splitting the string on `&&` and asserting each step appears in `ci.yml`.

That guard does not work **as written**: it is a `strings.Contains` over the whole file
(`internal/cli/ci_gate_test.go`), and §1.1's five inputs each defeat it. A better-scoped assertion is
available — §1.1 records the two that were built and the matrix they produced — so what this release
does is remove the second definition of the gate, leaving nothing for any guard to keep in step.

Three deliverables:

1. **`quality_gate` becomes an ordered array of named entries** (§2), the string form still accepted.
2. **`tp gate` runs it** (§3) — a verb that does not exist today, which is what makes restating the
   gate in CI convenient rather than necessary.
3. **CI invokes `tp gate`** (§4), so there is no restatement left to certify.

### 1.1 What the shipped guard measures, and what it does not

Every row below was built and run against the guards as they ship, in an `rsync -a --exclude .git`
copy outside the repository. **The third row was re-run against `HEAD` with its control**, because an
inherited measurement is a claim about a tree that has since moved:

```
baseline (untouched ci.yml)                                          ok
run: |  echo "temporarily skipping ./scripts/check-suite-state.sh (flaky on CI)"
        go test ./...                                                ok      <- green
delete ONLY the echo, changing nothing CI runs                       FAIL    <- red
                                                 TestCIRunsEveryStepOfTheProjectGate
```

Same CI behaviour in both of the last two — a bare `go test ./...`, no race detector, no state check —
and the guard's verdict flips on the presence of a string inside an `echo`. Note what that line is
**not**: it carries no `#` and it is executed, so *restricting a substring search to executable lines
does not help* — the mention is itself executable. That repair was written during v0.36.0's audit, in
a commit making the guard "search executable lines, not the whole file", and reverted two commits
later. **The guard tracks text, not execution**, and the control is what proves it rather than merely
suggesting it. What does help is dropping the *substring* comparison as well as the whole-file scope
— assertion A below.

**Counting rule for "gate guard", because two counts of this set have already disagreed:** every test
function in `internal/cli/ci_gate_test.go` plus `internal/cli/quality_gate_config_test.go` —
`grep -hcE '^func Test' internal/cli/ci_gate_test.go internal/cli/quality_gate_config_test.go`. There
are **six** at `HEAD`, and `git log -S` puts the set at 3 → 4 → 6, so a five-guard state never
existed.

| input | result |
|---|---|
| the deadcode step deleted, `# TODO: re-enable ./scripts/check-deadcode.sh` left behind | all six gate guards green, CI runs no deadcode |
| `run: ./scripts/check-suite-state.sh` replaced by `run: go test ./...`, plus a comment naming the wrapper **by its full `./scripts/` path** | all six green, CI runs neither the race detector nor the state check |
| `echo "temporarily skipping ./scripts/check-suite-state.sh (flaky on CI)"` above a bare `go test ./...` | all six green; deleting only the `echo` turns the guard red |
| `if: false` on the deadcode step | all six green |
| `continue-on-error: true` on the suite-state step | all six green |

**The second row's full path is load-bearing.** `ci.yml`'s comment names the wrapper as
`check-suite-state.sh` while the gate step is `./scripts/check-suite-state.sh`, and the substring does
not match: a reproduction that keeps the bare basename reddens the guard and looks like a refutation
of this table. Rows four and five are measured on one side only — that Actions skips an `if: false`
step, and that `continue-on-error: true` keeps a job green while a step fails, is documented Actions
behaviour rather than something observed here; what was observed is that all six guards stay green.

**A repaired guard exists. It was built and run, and that is why this section no longer claims
otherwise.** Two assertions, both in the same test file:

- **A — equality over parsed run-blocks.** Collect the argument of each inline `run:` plus each
  more-indented line under `run: |`, comments excluded; require each `&&`-split gate step to **equal**
  one of them, rather than to be a substring of the file. ~60 lines.
- **B — bounded absence over the whole artifact.** No non-comment line in `ci.yml` begins with `if:`
  or `continue-on-error:`. ~15 lines.

| input | the shipped six | A | B |
|---|---|---|---|
| untouched `ci.yml` | green | green | green |
| deadcode step deleted, TODO comment left | green | **red** | green |
| wrapper → bare `go test`, comment names the full path | green | **red** | green |
| `echo` naming the wrapper above a bare `go test` | green | **red** | green |
| *control:* only the `echo` deleted, CI unchanged | **red** | red | green |
| `if: false` on the deadcode step | green | green | **red** |
| `continue-on-error: true` on the suite-state step | green | green | **red** |

**So the honest claim is narrower than "the guard cannot work".** What cannot work is an
unstructured `strings.Contains` over the file's whole text, because its subject is a text that any
mention satisfies. That is a defect in the assertion's **scope and its comparator**, not proof that
no assertion exists.

**And the rule is about scope, not polarity.** `CLAUDE.md` records this, measured twice: absence is
not the safe direction. A windowed `NotContains` is a presence assertion over a finite blacklist, and
both it and `Contains` are *local* assertions inside an *unbounded* text, so a negation simply moves
elsewhere. Two shapes survive — an assertion whose subject is the whole of a **bounded artifact** (B
above, and §4's replacement guard), and a verdict resting on **read-backs** rather than on matched
text. A is the first shape, reached by narrowing the subject from the file's text to the set of
commands the file runs.

**The release still removes the restatement, and only the reason changes.** Not that the restatement
cannot be guarded — A and B guard it — but that a guarded restatement is still two definitions of the
gate, now with a third artifact keeping them in step. Deleting one of the two definitions is the only
option on the table that removes work instead of adding it.

## 2. `quality_gate` is an ordered array of named entries

```json
"quality_gate": [
  {"name": "suite",      "cmd": "./scripts/check-suite-state.sh"},
  {"name": "lint",       "cmd": "golangci-lint run"},
  {"name": "deadcode",   "cmd": "./scripts/check-deadcode.sh"},
  {"name": "complexity", "cmd": "./scripts/check-complexity.sh"}
]
```

**Entries run in order and stop at the first failure**, which is what `&&` already means. The array
makes the sequence data instead of syntax.

**What the array buys is the entry NAME and a per-entry record — not the exit code.** The string form
already carries the code, and this is the claim that had to be measured rather than assumed:
`sh -c '(exit 3) && (exit 9)'` returns **3**, not 1, and the second command never runs; codes 1, 2,
3, 4, 5 and 42 each come back unchanged from the middle of a chain. End to end through tp, a
four-link gate whose third link exits 33 produces `exit_code: 33` with
`output_tail: ["step1-ok","step2-ok","step3-failing"]`. So a red gate today does **not** say only
*something failed* — it says *the third thing to print exited 33*. What it cannot say is **which
entry** that was, or what the entries before it did: `&&` is syntax, and syntax has no names.

**The string form is still accepted and resolves to a single unnamed entry.** Every existing task
file and project config keeps working, and `tp config --resolved` reports the array form for both.
That last half is an **output-contract** change as well as an input compatibility one: a reader that
unmarshals `quality_gate` into a string breaks on a config it never edited.
`internal/cli/ci_gate_test.go` does exactly that today and is the in-tree instance.

**No `continue-on-error` and no per-entry skip.** A gate entry that may fail without failing the gate
is not a gate entry, and §1.1's fifth row is what that flag does to CI. This is about a per-*entry*
skip; tp's per-*invocation* one, `--skip-gate`, is untouched (§7 item 5), and the two are easy to
conflate.

## 3. `tp gate` runs the resolved gate

**There is no way to ask tp to run the gate.** `tp gate` exits 2 with
`unknown command "gate" for "tp"`. That makes restating the gate in CI convenient; it does not make
it necessary, and the honest form of the argument is the one §4 gives.

**And tp already runs the gate through two executors.** `internal/cli/gate.go` builds
`engine.RunCommand(wf.QualityGate, gateDir(taskFilePath), …)` at two independent call sites after
two independent calls to `engine.EffectiveWorkflowForTaskFile`:

- `executeQualityGate`, which returns a `RunResult` — reached from exactly one place,
  `tp done <id1> <id2>`'s batch path in `internal/cli/done.go`, whose own comment says that path
  "does not exit through `runQualityGatePreFlock`";
- `runQualityGatePreFlock`, which prints the error object and calls `os.Exit(ExitState)` — reached
  from `tp done <id>`, `tp done`'s batch survivors, and `tp close`.

They do not call each other: searching the tree for `executeQualityGate` returns one call site, and
`runQualityGatePreFlock` is not it. So the drift this release exists to remove **already exists**,
unmentioned, between two paths that differ in whether they return or exit.

`tp gate` resolves the gate through the existing layers, runs the entries in order, and reports each
entry's name, command, exit code and duration. Exit 0 when every entry passed; the failing entry's
exit code otherwise.

**One executor, not three.** The two above collapse into the one `tp gate` uses, and every existing
call site moves onto it. Two code paths that run "the gate" are a second thing to keep in step —
which is why `tp gate` cannot simply be bolted onto whichever of them is nearer.

**`--json` reports the per-entry results, and the entry NAME is what that buys.** A driver reading a
red gate today is not told *nothing* about which step failed: the shipped string gate already emits
`exit_code` — the failing link's own code, measured at 33 for a four-link gate whose third link exits
33 — alongside `gate_cmd` and an `output_tail` truncated at that step. What it cannot do is **name**
the step, so the driver must infer it from unstructured output, and a step with no name cannot be
reported, retried or branched on. The red-gate release (named in `CLAUDE.md`'s roadmap, not a section
of this file) is what consumes the name.

## 4. CI invokes `tp gate`

`.github/workflows/ci.yml`'s four `run:` steps become **two**: one that installs the tools, one that
runs `tp gate`. The installations stay — `golangci-lint`, `deadcode` and `gocognit` must be on `PATH`
before the gate runs, and that is CI's job, not the gate's — but they cannot stay *as they are*.
Three of today's four steps are named "Install and run X" and each also invokes its tool, so a
post-change `ci.yml` that keeps any of them whole still restates that step's gate command. Measured
on both: a file keeping two such steps beside `run: tp gate` is **red** under the replacement guard
below, while install-only steps plus `run: tp gate` are **green**.

**`TestCIRunsEveryStepOfTheProjectGate` is deleted, not repaired.** With no restatement there is
nothing left to compare, and a guard whose subject has been removed is not a guard. That is the whole
argument. §1.1 is not part of it — §1.1 records a repair that *works*, and the case for deleting the
restatement is that a guarded restatement is still a second definition of the gate.

**What replaces it is narrower and sound:** a test asserting that `ci.yml` **nowhere contains** a gate
entry's command — a whole-file assertion, not one scoped to `run:` steps. The distinction is not a
wording quibble, and it was measured on one fixture: a post-change `ci.yml` carrying a historical
comment that names three gate commands is **green** under the scoped reading and **red** under the
whole-file one. The whole-file reading is the one that ships, for §1.1's reason — its subject is the
whole of a bounded artifact, so a mention has nowhere to go. Its cost is stated here rather than
discovered later: `ci.yml`'s surviving comments must avoid every gate command verbatim.

## 5. Two guards narrower than their claims

**`ci_pin_test.go` walks only `*.yml`.** GitHub Actions accepts `.yaml` identically, so a
`release.yaml` installing `golangci-lint@latest` is invisible while the guard passes. Measured: a
`nightly.yaml` carrying three floating installs passes; the same bytes renamed `nightly.yml` fail on
all three refs by name. The extension alone decides the verdict, and the walk covers both.

**The regex scope is what lets `release.yml`'s floating refs through — the counter is not.** Floating
**every** ref in `release.yml` leaves `TestWorkflowToolsArePinned` passing, and the reason is not the
global counter: `goInstallRef` matches only `go install`, and `release.yml` has none of those
(`grep -cE 'go install[[:space:]]+[^[:space:]]+@[^[:space:]]+' .github/workflows/release.yml` → 0,
`ci.yml` → 3), so the guard never looks at the refs that were floated. The counter half built on its
own was run twice — against a fully floated `release.yml` and against the shipped, fully pinned one —
and returned the **identical** verdict, `release.yml contributed zero matches`. It cannot tell them
apart either.

**So the two halves are one change, not two.** `goInstallRef` extends to `uses:` refs, a third
pattern covers `go-version:` (which is not a `module@ref` at all and no `uses:` pattern can reach),
**and** `checked` becomes per file — which is what turns a workflow contributing nothing from a
silence into a failure.

**The extension reddens the shipped tree, so repinning both workflows is part of this release.**
Built as described and run against the untouched workflows: **7** floating refs — `ci.yml`
`checkout@v4`, `setup-go@v5`, `go-version: stable`; `release.yml` those same three plus
`goreleaser-action@v6` (`grep -nE 'uses:|go-version:' .github/workflows/*.yml`). Every one is pinned
to an exact version here. A release does not ship a guard that is red on its own tree.

**Scope, measured rather than assumed.** `release.yml` is not worse *in kind* than `ci.yml` — same
mutable major tags, same `go-version: stable` line. It is worse in *impact*, because its output is
what users install rather than a pass/fail signal.

## 6. A load-sensitive gate test

`TestRoleWriteHookStaysCheapOnADeepMissingPath` asserts `deep < 3*shallow` on wall-clock
(`internal/cli/role_write_symlink_test.go`). It **failed one of four full gate runs** during v0.36.0's
implementation, at deep 56.95 ms against shallow 17.82 ms — a ratio of **3.196**, not the 3.198 first
recorded (`python3 -c 'print(56.95/17.82)'`).

**That rate is unreproduced, and this section states it as unreproduced rather than as measured.**
Fifty-six repetitions, zero failures: 32 in this file's ground round (24 solo, 4 under `-race`, 4
inside a full `go test -count=1 -race ./...` — the condition the original sentence names) and 24
since, in an `rsync` copy —
`go test ./internal/cli -run TestRoleWriteHookStaysCheapOnADeepMissingPath -count=20 -v` (ratios
1.208–1.417) and `go test ./internal/cli -race -count=4 -v` (1.258–1.433). Under p = 0.25,
P(0 failures in 56) ≈ 1e-7. The **shallow** side matches the original closely — 15.1–18.9 ms against
17.82 — while the **deep** side does not: 20.6–23.9 ms against 56.95. So the original run's deep side
is what spiked, which points at concurrent load the sentence does not name. "One of four full gate
runs" describes the four-step gate, whose later steps load the machine while the suite is still
finishing; `go test ./...` alone does not create that condition, and load is the variable the test's
own comment says decides the result.

**The design stays and the constant moves.** A ratio rather than a deadline, fastest-of-three each
side, is right. What was wrong is the reason first given for widening: the constant does **not** sit
at the noise floor — across those 56 runs the observed ratio never exceeded 1.44, so 3 already sits
at more than twice the highest ratio seen. The problem is a **tail**, not a floor, and that decides
how far to widen. The constant must clear the recorded 3.196 outlier and still redden an unbounded
walk, which the test's own comment records at ~6× shallow. **It moves to 4**, and the sample grows
from three iterations per side to nine, because a fastest-of-N estimate is what a tail defeats and N
is the only knob that answers it.

**It is fixed here because a gate that randomly reddens is a gate people learn to re-run.** That
habit is what makes §1.1's third row — *temporarily skipping (flaky on CI)* — a thing someone writes.

## 7. Non-Goals

1. **No new gate entries.** The four stay exactly what they are; only their representation and their
   caller change.
2. **No per-entry timeout, retry or parallelism.** Entries run in order, once, sequentially.
3. **No `continue-on-error` equivalent, at any layer.** §2 states why.
4. **`tp gate` does not record anything.** No round, no state, no task-file write. It runs and reports.
   Satisfiable but not free: `runQualityGatePreFlock` reaches `recordGateFailure`
   (`internal/cli/gate.go`) whenever its `recordFailure` argument is true, and that writes
   `.tp/last_failure-<base>.json` under `TP_UNIT_KIND`. `tp close` already passes false for this
   reason; the merged executor must let `tp gate` do the same.
5. **No change to when a task close runs the gate**, or to `--skip-gate`, which remains a user-approved
   decision and remains fenced under `TP_UNATTENDED=1`.

## 8. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a four-entry array **names** the entry that failed and records a per-entry result for every entry that ran | collapse to `&&`, which has no names and no per-entry record, so nothing can satisfy the assertion |
| 2 | §2 *compat* | a string `quality_gate` resolves to one entry and behaves identically at a task close | accept only the array — which breaks `.tp/config.json`, the only string gate in this repository, and every other repo's config and task files |
| 3 | §3 | `tp gate` and a task close run the same resolved entries through **one** executor — asserted on recorded argv under a stub, at every call site the merged executor has | give `tp gate` its own executor, making three where the tree already has two |
| 4 | §3 *exit* | `tp gate` exits with the failing entry's code; 0 when all pass | exit 1 always — a regression, because the shipped string gate already propagates the failing link's own code |
| 5 | §4 | §1.1's third input — an `echo` naming a gate command above a bare `go test` — is **rejected** by the replacement guard | keep a presence assertion, which §1.1 measured green on all six guards for that input |
| 6 | §4 *immunity* | the replacement guard is the **whole-file** reading: a comment naming a gate command leaves it red | scope it to `run:` steps, which the same comment leaves green — measured, both readings, on the same fixture |
| 7 | §5 | a `release.yaml` with every ref floating fails the pin guard | keep `*.yml` and the global counter, the shipped behaviour, measured to pass |
| 8 | §5 *per-file* | **with §5's `uses:` and `go-version:` patterns in place**, a fully pinned `ci.yml` does not rescue an unpinned second workflow | keep `checked > 0`, satisfied by `ci.yml` alone |
| 9 | §6 | the widened constant passes the runs §6 records **and still reddens on an unbounded walk** | keep `3*shallow`; or widen past the unbounded signal, which passes a bounded and an unbounded hook alike |

**Row 8 cannot be implemented alone**, which is why it names its dependency. Built in isolation — the
counter moved inside the per-file loop, `goInstallRef` untouched — it returns the identical verdict
`release.yml contributed zero matches` on a fully floated *and* on the shipped, fully pinned
`release.yml`. The two halves of §5 are one change.

**Row 5 is the acceptance and its fixture is a recorded input, not an invented one.** §1.1's rows were
built and run against the shipping guards; using the one that turned all six green is what separates a
fix from a reword. Reproducing it needs the input stated exactly: the `echo` must spell the full
`./scripts/check-suite-state.sh`. The guard is a substring match, so a bare `check-suite-state.sh`
reddens it — and a naive reproduction then looks like a refutation of §1.1 rather than a
misconstruction of its fixture.
