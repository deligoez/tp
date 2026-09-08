# tp — The gate sequence

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `gate-sequence-measurements.md` beside it; this file stands without them.

Class: **tool** — it changes a config field, adds a verb and rewires CI; the loop that reviews it is
not the thing it changes. Budget it at the tool-class median in `CLAUDE.md`'s *What a cycle costs*
table.

**This file is decisions.** Its central one is not the array — it is that **CI stops restating the
gate**. v0.36.0's audit returned to that restatement across several rounds and closed none of it; the
round-by-round record is in the sidecar under "Why the restatement goes: the five-round record". A
repaired guard *is* available, so the reason for removing the restatement is not that guarding it is
impossible. It is that a guarded restatement is still two definitions of the gate, now with a third
artifact keeping them in step.

## 1. Overview

`quality_gate` is a shell string, `.tp/config.json` holds it, and **CI restates it** as four separate
`run:` steps (`grep -n 'run:' .github/workflows/ci.yml` returns exactly four keys; `checkout` and
`setup-go` are `uses:` steps and are not among them). `TestCIRunsEveryStepOfTheProjectGate` ties the
two by splitting the string on `&&` and asserting each step appears in `ci.yml`.

That guard does not work **as written**: it is a `strings.Contains` over the whole file
(`internal/cli/ci_gate_test.go`), and every input §1.1 summarises defeats it. A better-scoped
assertion is available — the sidecar records the two that were built and the matrix they produced — so
what this release does is remove the second definition of the gate, leaving nothing for any guard to
keep *in step*. That is narrower than it first read here, and the difference is §4's: removing the
restatement removes what two definitions can drift apart on, and removes nothing of CI's dependency on
the gate. `ci.yml` still has to invoke `tp gate`, and §4 measures that nothing in the release noticed
when it stopped.

Four deliverables:

1. **`quality_gate` becomes an ordered array of named entries** (§2), the string form still accepted.
2. **`tp gate` runs it** (§3) — a verb that does not exist today, which is what makes restating the
   gate in CI convenient rather than necessary.
3. **CI invokes `tp gate`** (§4), so there is no restatement left to certify.
4. **A doc task:** the red-gate procedure section for `skills/tp/SKILL.md`, specified in
   `spec/backlog/red-gate-procedure.md`, ships as a task of this release — it is what consumes the
   entry name §3 introduces, and a procedure with nothing to name it is prose.

### 1.1 What the shipped guard measures, and what it does not

The shipped guard is a `strings.Contains` over the whole of `ci.yml`, and every input the sidecar
records — a deleted step with a TODO comment naming it, a wrapper replaced by a bare `go test` beside
a comment naming the full path, an `echo` naming the wrapper above a bare `go test`, `if: false`,
`continue-on-error: true` — leaves it green while CI runs less than the gate. Deleting only the
`echo` turns it red, which is the control: the guard tracks text, not execution, and restricting the
search to executable lines does not help because the mention is itself executable. Two repaired
assertions were built and run against the same inputs. **A** — equality over parsed run-blocks —
reddens the text inputs and stays green on the YAML keys; **B** — a prefix blacklist over lines
beginning `if:` or `continue-on-error:` — reddens the keys and nothing else, and is defeated by
quoting the key (`"if": false`), so the sound form of its intent is a parser reading each step's key
set back. The rule is about scope, not polarity: a local assertion inside an unbounded text is
satisfied by any mention whichever direction it points, and A is the shape §4 ships. So the honest
claim is narrower than "the guard cannot work": a guarded restatement is still two definitions of the
gate, and deleting one is the only option that removes work. The full matrix is in the sidecar under
"What the shipped guard measures, and what it does not".

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
is not a gate entry, and the sidecar's fifth input is what that flag does to CI. This is about a
per-*entry* skip; tp's per-*invocation* one, `--skip-gate`, is untouched (§7 item 5), and the two are
easy to conflate.

## 3. `tp gate` runs the resolved gate

**There is no way to ask tp to run the gate.** `tp gate` exits 2 with
`unknown command "gate" for "tp"`. That makes restating the gate in CI convenient; it does not make
it necessary, and the honest form of the argument is the one §4 gives.

**And tp already runs the gate through two executors.** `internal/cli/gate.go` builds
`engine.RunCommand(wf.QualityGate, gateDir(taskFilePath), …)` at two independent call sites after
two independent calls to `engine.EffectiveWorkflowForTaskFile`:

- `executeQualityGate`, which returns a `RunResult` — reached from exactly one place, the
  **`tp done --batch <file>`** path in `internal/cli/done.go` (the call sits inside `runDoneBatch`),
  whose own comment says that path "does not exit through `runQualityGatePreFlock`";
- `runQualityGatePreFlock`, which prints the error object and calls `os.Exit(ExitState)` — reached
  from **`tp done <id>`** (`runDoneSingle`), **`tp done <id1> <id2>`**'s surviving tasks
  (`runDoneMulti`), and **`tp close`**.

**The multi-ID form is not the batch path, and an earlier revision of this section said it was.**
`runDone` dispatches `--batch` before it parses positional IDs and only then splits one ID from many,
so `tp done <id1> <id2>` is `runDoneMulti` and reaches `runQualityGatePreFlock` — never
`executeQualityGate`. The tree draws the same line in its own error text: *"--auto-commit is not
supported with multiple task IDs. Use `tp done --batch` for multi-task auto-commit."* The structural
conclusion is unaffected — two executors, no overlap — but §8 row 3 asks an implementer to assert on
every call site the merged executor has, and the wrong command name sends them to the wrong one.

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
reported, retried or branched on. The red-gate procedure (`spec/backlog/red-gate-procedure.md`,
deliverable 4) is what consumes the name.

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

**Absence of the restatement is not presence of the gate, and that needs a second assertion.** The
replacement guard is sound for the property it states and for nothing else. A post-change `ci.yml`
with the `run: tp gate` step **deleted** — CI running no gate at all — is green under *both* readings
of it, and green under every other row of §8 as this section first wrote them: nothing in the release
asserted that CI invokes the gate. That is the sidecar's first input reopened against the guard
shipped to replace the one that input defeated — delete the step, everything stays green — with only
the shape of the deletion changed, from a missing `run: ./scripts/check-deadcode.sh` to a missing
`run: tp gate`. §8 row 10 asserts the other half. It is stated as equality over the parsed set of
run-blocks, in A's shape, rather than as a substring over the file, because a presence assertion
inherits §1.1's lesson whichever direction it points.

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

`TestRoleWriteHookStaysCheapOnADeepMissingPath` (`internal/cli/role_write_symlink_test.go`) asserts
a ratio of deep to shallow wall-clock and failed one full gate run during v0.36.0's implementation;
the failing figures live in the `closed_reason` of task `instruction-test` in
`spec/0.36.0.tasks.json`, and no repetition since has reproduced them. **The design stays and the
constant moves.** A ratio rather than a deadline, fastest-of-N each side, is right; what was wrong
was the reason first given for widening — the constant does not sit at the noise floor, the problem
is a tail, and a sample maximum is not a bound because the maximum is exactly the statistic a tail
moves. The constant moves to **4** — above the recorded outlier and still below the unbounded walk
the test's own comment records — and the sample grows from three iterations per side to nine, because
a fastest-of-N estimate is what a tail defeats and N is the only knob that answers it. It is fixed
here because a gate that randomly reddens is a gate people learn to re-run, which is the habit that
makes *temporarily skipping (flaky on CI)* a thing someone writes. The repetitions, the timings and
the rejected mechanism are in the sidecar under "A load-sensitive gate test".

A gate step that is green on the first run and red on the second — `NewRootCmd` binding package-level
flag variables, found when two parallel tests built a root command under `-race` — is the same class
by a different route and takes no decision here; `spec/undecided.md` carries it as "`NewRootCmd`
writes package globals".

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
6. **The mutation-run check is not an entry of this gate.** It excludes itself from the per-task
   `quality_gate` — the engine package's mutant set is not something to run hundreds of times — and
   lives in `spec/backlog/mutation-run-check.md` as a script and a `CLAUDE.md` line, not as a release.

## 8. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a four-entry array **names** the entry that failed and records a per-entry result for every entry that ran | collapse to `&&`, which has no names and no per-entry record, so nothing can satisfy the assertion |
| 2 | §2 *compat* | a string `quality_gate` resolves to one entry and behaves identically at a task close | accept only the array — which breaks `.tp/config.json`, the only string gate in this repository, and every other repo's config and task files |
| 3 | §3 | `tp gate` and a task close run the same resolved entries through **one** executor — asserted on recorded argv under a stub, at every call site the merged executor has | give `tp gate` its own executor, making three where the tree already has two |
| 4 | §3 *exit* | `tp gate` exits with the failing entry's code; 0 when all pass | exit 1 always — a regression, because the shipped string gate already propagates the failing link's own code |
| 5 | §4 | the sidecar's third input — an `echo` naming a gate command above a bare `go test` — is **rejected** by the replacement guard | keep a presence assertion, which the sidecar measured green on every shipped guard for that input |
| 6 | §4 *immunity* | the replacement guard is the **whole-file** reading: a comment naming a gate command leaves it red | scope it to `run:` steps, which the same comment leaves green — measured, both readings, on the same fixture |
| 7 | §5 | a `release.yaml` with every ref floating fails the pin guard | keep `*.yml` and the global counter, the shipped behaviour, measured to pass |
| 8 | §5 *per-file* | **with §5's `uses:` and `go-version:` patterns in place**, a fully pinned `ci.yml` does not rescue an unpinned second workflow | keep `checked > 0`, satisfied by `ci.yml` alone |
| 9 | §6 | the widened constant passes the runs the sidecar records **and still reddens on an unbounded walk** | keep `3*shallow`; or widen past the unbounded signal, which passes a bounded and an unbounded hook alike |
| 10 | §4 *presence* | `ci.yml`'s parsed set of run-blocks contains one **equal** to the gate invocation — deleting the `run: tp gate` step reddens it | ship rows 1–9 alone: a post-change `ci.yml` with that step deleted was measured green under every one of them, and under both readings of row 6's guard |

Rows 5 and 8 each depend on a recorded fixture — row 8 on §5's two halves landing together, row 5
on the `echo` spelling the wrapper's full `./scripts/` path — and the sidecar states both under
"Notes on rows 5 and 8".
