# tp — `next_action` and `--check` tell the truth

Class: tool

## 1. Overview

**Context.** Two status surfaces answer a driver's question with something other than the answer.
`tp ground <spec> --status --check` exits 0 when every emitted floor unit carries a disposition and
gates on nothing else, so a recorded round whose rows hold `FAIL`s — refuted claims standing in the
spec — reports go. Both field reports on `tp ground` v1.0.0 stopped their loop on that exit code,
and a two-unit fixture reproduces the exit (`ground-command-friction-measurements.md`, "§9 `--check`
exits 0 with `FAIL`s standing"). The same `--status` payload carries no `next_action`, where
review's and audit's both do, so a driver that wants the branch re-derives it from `by_verdict`. In
the review loop the shape repeats: the uncounted regression delta pass,
`tp review <spec> --perspective regression`, ships, and the emitted loop instruction recommends it
whenever a regression prompt is included (`buildReviewLoopInstruction`, `internal/cli/review.go`,
around line 2057) with no condition on what the repairs touched — while `next_action`, the surface a
driver reads, never names it, and the rule for when it is worth running lives only in
`skills/tp/SKILL.md`'s review loop, step 6.

**Decision.** Three reporting changes and one help-text fix, none of which adds a flag, a unit kind or
a workflow field: grounding's `--check` gains a third condition on a standing `FAIL` (§2); grounding's
`--status` carries `next_action` (§3); review's `next_action` gains one advisory branch recommending
the delta pass, suppressed under a run (§4); and `tp review --help` names `regression` among the
perspectives (§5).

**Consequences.** A driver that branches on `--check`'s exit code stops with no `FAIL` standing rather
than with false claims in the spec. `tp run` schedules no grounding unit today, so the gate's
beneficiary is that driver when it exists and any script branching on `$?` until then. The gate ships
with one hole that clears it without a repair — a byte-identical sentence in two sections — which
`ground-command-friction.md` §9.3 states and its row 24 guards; it is not closed here. The review
branch is a string in a payload and gates nothing, and under `TP_RUN_ID` it names the counted round
instead, because no unit kind runs a delta pass.

## 2. `tp ground --status --check` exits 1 on a standing `FAIL`

**The decision: `--check` exits 1 when the latest *recorded* round carries a row whose `verdict` is
`FAIL`.** The two conditions it already has are unchanged, and `skills/tp/SKILL.md`'s command-table
sentence saying a round of nothing but `FAIL`s is fully covered and exits 0 goes with the behaviour it
describes.

**Recorded, not emitted.** An emitted-and-unrecorded round is already exit 1 on the coverage
condition, so the two readings never disagree about the exit code — only about what the third
condition reads, and it reads the latest recorded round file.

**`FAIL` and nothing else.** `FAIL` is the only verdict that asserts the spec is wrong: `UNVERIFIABLE`
is a settled answer, `NOT-A-CLAIM` asserts nothing, `QUESTION` is explicitly non-blocking in the same
SKILL.md step that asks for the repairs. `PARTIAL` is the deliberate exclusion, and it is narrower
than the loop: SKILL.md stops its ground loop only when the breakdown carries no `FAIL` **or
`PARTIAL`** you have not repaired, so a `true-when-written` `PARTIAL` is a complete row and an open
repair the gate does not see. What the gate buys is the case no prose caveat reaches — a machine can
gate on a standing `FAIL` without judging prose. Whether SKILL.md's condition should narrow to match
is SKILL.md's decision and is not taken here.

**One unconditional exit, one hole.** Repairing the unit's text moves its hash, so the unit leaves the
carry and is re-asked; re-deciding a carried unit in a later round is the override
`ground-command-friction.md` §11 makes sayable, permitted only when the ground beneath it moved. The
hole — an identical sentence in two sections, the failing copy cleared by editing the other — is
`ground-command-friction.md` §9.3's, ships as characterised behaviour under its row 24, and is closed
by `spec/backlog/what-the-carry-can-promise.md` §2.2's multiplicity fence.

## 3. `tp ground --status` carries `next_action`

**The decision: `--status` carries `next_action`, under that name and in that role.** Review's and
audit's `--status` both carry it — *"the single next step"*, in SKILL.md's words for each — and
grounding's payload does not. It is a pair with the gate above: the exit code says *not yet*; the key
says *what to do*, which is how a driver branches without parsing prose. No new vocabulary: it names
the same step SKILL.md's ground loop names, and it is reporting, so a driver that ignores it is
exactly as correct as one that reads it.

## 4. Review's `next_action` recommends the delta pass

Any `fixed` disposition voids a round — `--record` refuses it, correctly, because a fix means the spec
changed and the round's findings were read against older text. The cost is that a one-section repair
forces a full panel. The pass that solves this already ships: `tp review <spec> --perspective
regression` emits the regression role alone, derives its scope from the newest earlier snapshot
through `DiffSections` (`internal/engine/diff.go:41`), and reports `Round: 0`, *"This pass records no
state"* and `Convergence: "uncounted delta pass — counted rounds stay full-panel"`
(`internal/cli/review_regression.go:126`).

**The decision: `next_action` learns to recommend it — one advisory branch, no flag.** It fires when
all three hold:

- the last recorded round produced findings, and
- the repairs since touched **more than three** sections, and
- no section was added or removed.

Outside that, `next_action` is unchanged; `ReviewNextAction` (`internal/engine/nextaction.go`) has
four branches and the converged one names no round at all.

**No flag.** A `--delta` flag would be a second spelling of a shipped feature: the scope derivation is
already the default, and taking the scope (`--diff-from` + `--findings`) is the shipped opt-in. What
is missing is a recommendation, not a capability.

**The threshold direction is SKILL.md's.** `skills/tp/SKILL.md` has carried *"when a fix batch
touched **more than 3 sections**, run the standalone regression delta pass"* since `f747c354`
(`git log --reverse -S'more than 3 sections' -- skills/tp/SKILL.md`), and the direction has a reason:
a large repair is what needs the cheap look, while a one-section repair is small enough that the next
counted round covers it. The rule has one home and this spec agrees with it rather than restating it.
The threshold is not configurable, because a workflow field would give the rule a second home — the
condition under which an earlier draft inverted it undetected (sidecar, "One draft, not two").

**The added/removed clause is not a refinement.** `DiffSections` reports an addition or removal as a
change with no counterpart to compare against, so a regression pass over it has nothing to regress.
That is a different question from *did this repair drift*, and it belongs to a counted round.

**A delta pass can never move the loop toward convergence** — because it writes no state, not because
of anything about panels: `--record` on a single role's row advances the count, so *"counted rounds
are always full-panel"* is `SKILL.md`'s policy and not the mechanism's (sidecar, "§3 The branch").
Delta findings go to stdout and never into `$TP_ROUND_DIR`; `-o` is not the mechanism
(`-o/--output requires --merge`, `internal/cli/review.go:311`), and an operator who wants the payload
in a file redirects stdout.

**The branch is omitted while a run is driving.** When `TP_RUN_ID` is set, `next_action` names the
counted round instead. `internal/engine/unitkind.go` fixes eight unit kinds and none runs a delta
pass, so an unattended driver reaching the recommendation would be told to run work it has no unit
for — the failure already recorded for the branch that recommended a registration the phase could not
honour. A ninth unit kind is not on the table: it would reopen a review-converged spec
mid-implementation to buy a recommendation a human can already read.

## 5. `tp review --help` names `regression`

**The decision, as a task: the `--perspective` flag's help text names `regression`.** It reads
*"documentation, testing, or code-audit"* (`internal/cli/review.go:391`) while `reviewPerspectives`
(`internal/cli/review.go:512`) accepts `regression`. The absorbed draft named the defect and declined
it; it is why the pass goes unrun, and it is one line.

## 6. Non-Goals

1. **No new flag and no new unit kind** (§4).
2. **No convergence effect from `next_action`.** The recommendation gates nothing, counts nothing, and
   records nothing; `next_action` has never gated an exit code and does not begin here.
3. **No change to `--perspective regression` itself.** Its scope derivation, its round-zero reporting
   and its output destination are unchanged.
4. **The threshold is not configurable** (§4).
5. **No change to grounding's carry, its `(text_sha, ordinal)` join or its recorded filename**, and no
   gate on `PARTIAL` (§2).
6. **The §9.3 hole is not closed** — it is `spec/backlog/what-the-carry-can-promise.md`'s.
7. **No workflow field.** Nothing here adds a knob to `.tp/config.json` or to a task file's `workflow`
   block, and nothing reads one.

## 7. Tests

Every row derives from a numbered decision and names an input that must fail it. Where the mutant is a
change to the test rather than to the product, the row says so. Rows 1–4 are
`ground-command-friction.md`'s former §15 rows 10–13; rows 5–11 are the absorbed draft's.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a round whose rows are fully covered and hold at least one `FAIL` exits **1** from `--status --check` | keep the two shipped conditions, under which a fully covered round holding `FAIL`s exits 0 |
| 2 | §2 *bounded* | the same fixture with the `FAIL` replaced by a `PARTIAL` and by a `QUESTION`, each in turn, exits **0** | gate on any non-`PASS` verdict, which passes row 1 and makes every recorded `PARTIAL` row blocking |
| 3 | §2 *no deadlock* | a round-2 payload that decides a `FAIL`-carrying unit afresh records at exit 0 and `--check` then exits 0, on a fixture whose spec text did not change | carry the `FAIL` unconditionally, which is the accepted-finding release's audit defect reproduced here |
| 4 | §3 | `--status`'s payload carries `next_action`, asserted by naming the key rather than by matching its text | assert on the sentence, which pins prose a rewording breaks while the key survives — a test-side mutant |
| 5 | §4 | a repair touching **four** sections after a round with findings recommends the delta pass; **three** does not | invert the comparator — the draft's own error, which recommends the cheap look for small repairs and the full panel for large ones |
| 6 | §4 *boundary* | exactly three sections does **not** fire, and exactly four does — both asserted | test with one and ten, which passes whether the bound is inclusive or exclusive; this repository has already checked a documented 1–60 range with the value 999 (`internal/engine/lock_timeout_range_test.go:13-33`) |
| 7 | §4 *added* | a repair that adds or removes a section does **not** fire the branch, even at ten sections changed | ignore the clause, recommending a regression pass over text with nothing to regress against |
| 8 | §4 *clean* | a last round with **no** findings does **not** fire the branch, whatever the section count — asserted on not-firing, because what `next_action` says instead depends on whether the spec converged | fire on section count alone, recommending a delta pass after a clean round |
| 9 | §4 *under a run* | with `TP_RUN_ID` set, the branch never fires — asserted on the same input that fires without it | emit it unconditionally, telling a driver to run a unit kind that does not exist |
| 10 | §6 item 2 | the command's exit code and the recorded round are identical with and without the branch — `next_action` is a string in the payload and has no exit code of its own | let it record or gate, turning a recommendation into a step |
| 11 | §4 *no flag* | no new flag is registered — asserted against the command's flag set, not its help text | add `--delta`, shipping a second spelling of `--perspective regression` |
| 12 | §5 | `--perspective`'s help text names every value `reviewPerspectives` accepts, derived from that slice rather than from a literal list | append a perspective to the slice and leave the help text alone — the derived guard fails naming it, a literal list passes |

**Rows 5 and 6 assert the same proposition, and the difference is the mutant column.** Row 6's names
an implementation of row 5 with one and ten that passes whichever way the bound goes, so row 6
constrains how row 5 is written; the comparator matrix that shows it is in the sidecar ("§6 Tests —
the comparator matrix"). Row 2 is the one a `PARTIAL` gate would fail, and it is what keeps §2
narrower than SKILL.md's loop.
