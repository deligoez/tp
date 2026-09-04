# tp v1.48.0 — The red gate

> **This file is decisions.** It ships **text in a brief and enforces nothing**, which is the decision
> rather than a shortfall — §4 records the three mechanical proxies considered and why each was
> rejected.

## 1. Overview

A unit handed a red gate today gets two things and nothing between them: **the wall** — `tp brief`
carries the resolved gate command verbatim (`engine.CloseRecipeText`) and `last_failure` when a
previous attempt recorded one (the `omitempty` field on `engine.Brief`, rendered by `(*Brief).Text`
and filled from `engine.ReadLastFailure` by `internal/cli/brief.go` and `internal/cli/next.go`) — and
**the prohibition**, carried on the same `CloseRecipeText` line: a red gate is never closed over and
`--skip-gate` is a user decision. **Cited by symbol rather than by line**, because a line number here
was wrong the day it was written — an earlier draft gave `internal/engine/brief.go:95` for
`last_failure`, which is a doc comment inside `closeRecipeCommand` at `HEAD` and at this file's own
commit.

The failure that produces is not a wrong fix. It is a **loop**: an agent with no bound and no named
exit re-runs the gate, edits, re-runs, and burns the unit.

**Reset-native execution makes the loop expensive rather than annoying.** The context that learned
*why* the gate was red dies with the unit, and the next attempt restarts from whatever `last_failure`
holds — which is less than it sounds. The record has two writers with two shapes: the driver's is one
line naming the failed command and its log (`failureSummary`, `internal/engine/driver.go`), while the
`tp done` gate's — the one that fires in exactly the situation this release is about — is the gate's
own output tail, up to `gateOutputTailLines` (`gateFailureSummary`, `internal/cli/gate.go`). And
**outside a run neither writer fires at all** (§3.1). An orchestrator's memory does not survive the
reset either. The brief does.

**What is deliberately *not* claimed: that a rule in `CLAUDE.md` is lost.** That is a fact about the
harness rather than about tp, and this cycle did not settle it — settling it means spawning a runner
and asking the child what instructions it loaded. What is checkable from the tree is that the claim
is **runner-conditional**: the default runner is `claude` (`engine.RunnerDefault`,
`internal/engine/runfields.go`), but tp also ships `TemplateOpencode`
(`internal/engine/runnertemplate.go`), whose child reads no `CLAUDE.md` at all. So the sentence holds
for at least one shipped runner and is unmeasured for the default. Nothing in this release depends on
which.

**This release needs the gate sequence.** Step 1 below runs *one entry*, which is only a name once
`quality_gate` is an ordered array of named entries. Under the string form that step is the manual
split the sequence release exists to remove.

## 2. The procedure

The brief gains an ordered procedure. **Every step is a rule this repository already paid for; the
contribution is the sequence and the bound, not the steps.**

1. **Isolate, then reproduce.** Run the failing entry alone, not the gate. A failure that does not
   reproduce is a finding about the gate, not about the change.
2. **Observe the failure before editing it.** `CLAUDE.md`'s rule read in the other direction: a fix
   accepted without watching the failure first proves nothing, and an assertion never seen failing
   may be a tautology that passes identically either way.
3. **Name three to five falsifiable causes and rank them, before testing any.** Each states its
   prediction: *if X is the cause, then changing Y removes it.* A cause with no prediction is a
   guess. Then **show the ranked list and carry on** — the operator often re-ranks it instantly from
   knowledge the unit does not have, and a unit that stops to wait has turned a checkpoint into an
   escalation.
4. **Re-run the entry, then the whole gate.** Both, in that order. In v0.31.2 a red `golangci-lint`
   stood behind a green test suite across **ten task closes**, which is what this step exists to
   catch — and which is not what an earlier draft of this line said (below).
5. **Exit by name when the investigation ends.**

**Step 4's incident, derived.** Counting rule: one row per JSON object in
`spec/.tp-review/0.31.2/audit-round-N.ndjson` for N = 1..7, a row counted when its serialized JSON
mentions `golangci` case-insensitively. Under that rule the red lint appears in **one** round — round
4, carrying one `FAIL` (*"QUALITY GATE RED: golangci-lint run exits 1 on the new code"*) and ten
`PARTIAL` rows each naming a task and each saying the acceptance conjunct *"The quality gate passes"*
is false. All ten of those task ids carry a **non-null `gate_passed_at`** in
`spec/0.31.2.tasks.json`: they were closed on a gate the closing agent read as green. Round 5 records
every one of them re-verified, and the other six rounds carry only `PASS` rows on that term.

**So the shape the step catches is a red entry surviving a *close*, not a red entry surviving
rounds.** An earlier draft of step 4 said the lint *"stood behind a green test suite for four audit
rounds"*; under the rule above it stood for one, and that round is what caught it. The corrected
incident still motivates the step, because the audience is the unit doing the closing.

### 2.1 Why the ranked list is a step and not advice

**Single-cause reasoning anchors on the first plausible idea, and this repository has a receipt with
a dated home.** `CLAUDE.md` records v0.36.0's audit round 1 measuring `scripts/check-complexity.sh`
running each tool inside a `$( … )` pipeline ending in `sort -u`, so `$?` was `sort`'s and `set -e`
never fired — the checker read as failing open. The first repair then recorded *"127 when it cannot
run"* as `gocognit`'s convention and guarded on it; the 127 was the **shell's** command-not-found,
because the experiment had taken the tool off `PATH` rather than breaking it. One plausible cause,
acted on, wrong; the second cause is what settled it.

**No count is asserted, because none could be derived.** An earlier draft opened *"three times in one
cycle"* and named two further incidents. Neither survives being traced. The nearest match for *a
false-positive class characterised from the wrong reading of its own output* is `CLAUDE.md`'s
lint-rule prototyping, which the same file dates to the 2026-09-02 roadmap pass and not to v0.36.0's
audit; and *a digest fix that read correctly and changed nothing when run* has no located record at
all — `grep -n -i digest CLAUDE.md` returns two hits, both about the gate wrapper's before/after
hash, neither about such a fix, and a search of `spec/.tp-review/0.3[67].0/audit-round-*.ndjson` for
`false.positive` returns one row, on an unrelated subject. This repository's own rule is never to
assert a quantifier it did not count. The step's argument does not need three; it needs one that can
be checked, and it has one.

**The step is cheap and it is the one a unit under pressure skips.** Steps 1 and 2 feel like progress;
enumerating causes feels like delay. That is exactly the shape of the premature completion §3's bound
is written against.

## 3. When it ends, and how

**It ends on a condition, not a count.** Either the next step would be a user-only decision, or an
attempt produced no new information about the failure. An attempt count is the first of the three
gameable proxies §4 rejects — a unit that must finish in three attempts learns to declare victory on
the third.

**And it ends on a minimal reproduction, which is a checkable bound rather than a feeling.** Once the
entry is red, cut inputs, config and steps one at a time, re-running after each cut. **Done when every
remaining element is load-bearing: removing any one of them turns the entry green.** That test is what
stops "I have reproduced it" from being the kind of fuzzy criterion this section opened against — a
feeling rather than a checkable bound — and the minimal case is also the regression test, so the work
is not thrown away.

**Where no seam can hold that test, the absence is itself the finding.** A red gate whose failure
cannot be pinned at any available seam is reporting something about the codebase, not about the
change; the unit records that rather than writing a test at a seam too shallow to catch it, which
would hand back false confidence.

**The exit differs by context and the brief resolves which one applies**, because naming a command
that exits 2 on the path the unit is on is worse than naming none. Measured:

```
$ tp escalate --decision skip-gate --evidence "probe"
{"error":"TP_RUN_DIR is not set: tp escalate runs only inside a unit tp run spawned","code":2,
 "hint":"...outside a run there is no unit to stop, so make the decision directly"}
```

| context | the exit |
|---|---|
| under `tp run` | `tp escalate --decision skip-gate --evidence <text>`; the run stops with `stop_reason: escalation` and the operator answers |
| outside a run | a hand-back: stop, leave the task `wip`, report the wall **in the unit's own report, which is the only carrier** — no `last_failure` is written on this path |

### 3.1 The hand-back, and what it does not carry

**The hand-back row names no file, because on that path there is no writer.** Measured outside the
repository, one open task, `quality_gate` = `exit 7`: `tp done t1 "…" --commit HEAD` with no `TP_*`
variable set exits 4, emits the gate error object, and leaves `.tp/last_failure-spec.json`
**absent**; the identical command under `TP_UNIT_KIND=implement TP_UNIT_ID=t1 TP_PHASE=implement`
**writes** it. `recordGateFailure` (`internal/cli/gate.go`) returns before writing when
`TP_UNIT_KIND` is empty and its own doc comment says so; `engine.WriteLastFailure` has two non-test
call sites, that fenced one and the driver's; `TP_UNIT_KIND` is set only by `engine.driverEnv`
(`internal/engine/childenv.go`). Derive with
`git grep -n 'WriteLastFailure\|EnvUnitKind' -- '*.go'`. This release accepts that limit rather than
closing it (§5.5) — an earlier draft of the row said `last_failure` carried the hand-back, which
named the one mechanism switched off on the very path the row is about.

**The procedure is short and always emitted.** It belongs where the prohibition already is: a *"never
close over this"* with no stated alternative is the shape that produces the loop. The `last_failure`
specifics — its exit code and the previous attempt's summary — appear only when a record exists,
which is already how `tp brief` treats that key: the field is `omitempty` and the section renders
only on a non-nil pointer, so an absent record is an absent key rather than an empty placeholder.
**The failing entry's name is not among the specifics.** `engine.LastFailure`
(`internal/engine/lastfailure.go`) holds `unit_kind`, `unit_id`, `phase`, `exit_code`, `summary` and
`at`; the gate command reaches the next unit only inside the free-text `summary`, and a *named* entry
waits on the gate sequence exactly as §1 says.

## 4. Why this is not a check, stated so it is not re-proposed

**Whether an investigation was systematic is not observable from its output.** Three proxies were
considered and all three are rejected — two because a unit that must pass the proxy learns to pass
the proxy, and one because it mis-prices the work rather than being gamed:

| proxy | why it fails |
|---|---|
| attempt count | §3's bound is a condition; a count teaches the unit to stop counting, not to stop learning |
| diff size | punishes a legitimate large repair and rewards a wrong small one |
| test-before-code, detected from diff or commit order | the only proxy here that would change what a green close means, and a unit that must pass it learns to pass it |

**The third has a measured precedent in this repository.** v0.34.0 §7.1 registered a check that
suppressed, for eight rounds, the very class it was meant to measure — and the first round after
unregistering it surfaced three stale claims the suppression had hidden. A mechanism that shapes what
a unit reports is not neutral about what the unit finds.

**What tp can do instead is put the procedure and its exit in front of a unit that has lost the
previous attempt's context, every time.** That is the whole release.

## 5. Non-Goals

1. **No enforcement, no gate, no exit code.** Nothing here changes what a green close means or what
   any command returns.
2. **No audit-repair surface check.** *"A repair that introduces a new abstraction belongs to the next
   version"* is real and among the most expensive rules tp has learned, and every mechanical proxy
   considered — diff size, new symbol count — is gameable and punishes legitimate repairs. It stays
   prose until someone finds a predicate that is not.
3. **No test-first gate.** §4's third row; not without a measurement showing the gate catches more
   than it teaches.
4. **No change to `--skip-gate`.** It remains a user-approved decision and remains fenced under
   `TP_UNATTENDED=1` at **three sinks, not one**: the `tp done` flag, `tp close --skip-gate`, and a
   batch entry's `skip_gate` field. Each exits 2 with `--skip-gate is a user-approved decision and is
   refused under TP_UNATTENDED` and a hint naming `tp escalate`, the task does not close, and both
   arms — refused under the variable, available when it is `0` or empty — are pinned by
   `internal/cli/unattended_fence_test.go`. Checked by running all three rather than assumed, because
   a sibling spec in this programme asserted a comparable fence on another command and it was measured
   **absent**. §3's escalation path is how a unit *asks*, not a way to take it.
5. **No new record.** `last_failure` is written and surfaced exactly as it already is — including the
   part §3.1 depends on and states there: outside a run it is not written at all. This release adds no
   field and no writer.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | every brief carries all **five** steps in order, whether or not `last_failure` exists — asserted over both states | emit the procedure only when a failure is recorded, so the first attempt gets the prohibition alone |
| 1b | §2 *the ranked list* | step 3 asks for **three to five** causes, each with its prediction, and says to carry on rather than wait | emit it as "consider alternatives", which is a no-op: the agent already believes it does that, so the line changes nothing |
| 1c | §3 *minimality* | the procedure names the load-bearing test — remove any remaining element and the entry goes green | say "reduce the repro", which has no observable done-condition and reads as advice |
| 2 | §3 | under a unit's **whole** driver-set identity — `TP_RUN_DIR` *and* `TP_UNIT_SEQ` — the brief names `tp escalate`; with either absent it names the hand-back and **does not** name `tp escalate` | key the branch on `TP_RUN_DIR` alone — with it set and `TP_UNIT_SEQ` unset, `tp escalate` exits 2 with `TP_UNIT_SEQ is not set, so the escalation record has no path`, which is the exact failure the row exists to prevent |
| 3 | §3.1 *specifics* | the exit code and the previous attempt's summary appear only when `last_failure` exists — the record carries no entry-name field to assert (§3.1) | emit empty placeholders, which teach a unit to read absent fields as facts |
| 4 | §2 *entry* | step 1 names the failing **entry**, not the whole gate command | name the gate string, which is the manual split this release depends on the sequence to remove |
| 5 | §5.1 | **the procedure adds no write and no exit code of its own** — the task file's digest is unchanged across `tp brief <id>`, and every command's exit code for a given input is what it was before the procedure existed | let it record an attempt, turning a text procedure into state |
| 6 | §5.4 | the **procedure block** the release emits equals the constant the release adds, and that constant names no `--skip-gate` invocation — asserted as equality over a bounded artifact, never as a search over the brief | name the flag as a step, converting a user decision into a unit's |

### 6.1 Notes on rows 2, 5 and 6

**Row 2 is the acceptance, and it needs two fixtures because it is a claim about two contexts.** §3's
transcript is the outside-a-run one. The inside-a-run one is not in the document, so it is given here
and was built to check the row: a scratch project with one open task, `TP_RUNNER_SEAM` pointing at a
script whose only command is `tp escalate --decision skip-gate --evidence <text>`; `tp run` returns
`{"phase":"implement","stop_reason":"escalation","units":1}` at exit 4, writes
`.tp/runs/<run_id>/1-escalation.json`, and leaves the task `wip`. A procedure naming the wrong exit
is worse than naming none, so the test must run in both contexts and not only in the one its author
is standing in — a warning an earlier draft made in this very paragraph while fixturing only the
author's own arm.

**Row 2's mutant is the one that a real run cannot reach, which is why it has to be constructed.**
`engine.driverEnv` (`internal/engine/childenv.go`) sets `TP_RUN_DIR`, `TP_UNIT_SEQ`, `TP_UNIT_KIND`,
`TP_UNIT_ID` and `TP_PHASE` in one map literal, so under `tp run` they arrive together and a test
keyed on `TP_RUN_DIR` alone passes without ever entering the arm it was written for. Measured on
three arms against a binary built from `HEAD`: no variable set → `TP_RUN_DIR is not set`, exit 2;
`TP_RUN_DIR` set with `TP_UNIT_SEQ` unset → `TP_UNIT_SEQ is not set`, exit 2; both set → the record is
written and the command **still exits 2**, because a successful escalation exits 2 by design
(`runEscalate`, `internal/cli/escalate.go`). So the exit code does not separate the three arms
either; the record on disk is what does.

**Row 6 is an equality, and it is not `NotContains("--skip-gate")`, because that guard fails on text
that must stay.** The brief already carries the literal, inside the prohibition this release keeps:
`CloseRecipeText` writes *"A red gate is never closed over: --skip-gate is a human decision, never the
unit's."* A `Contains` cannot separate a prohibition from an instruction either, and both are local
assertions inside an unbounded text. The distinguishing signal is structural — which block the string
sits in — so the row asserts equality between the emitted procedure block and the constant the
release adds, which is a bounded artifact with no elsewhere for a negation to go. (§3's escalate
invocations read `--decision skip-gate`, which does not contain the flag literal, so they are not the
collision.)

**Row 5's quantifier was wrong and is now scoped to what this release can break.** An earlier draft
asserted "writes no file" *across the brief's callers*, which is false of one caller by design:
measured with an md5 of the task file around each call, `tp brief t1` leaves it byte-identical while
`tp next --brief` changes it — `tp next` claims, so the task goes `open` → `wip`. That write predates
this release and is what the command is for. A test written to the old clause fails on `tp next`; a
test that quietly narrows to `tp brief` passes while asserting less than the row claims. The row now
says what is actually at stake: the procedure adds no write of its own.
