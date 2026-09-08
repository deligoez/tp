# tp — What §8's carry can promise

Class: **loop** — it changes what `tp ground` carries between rounds and what the emitted ask
promises, so it is reviewed by the mechanism it changes. Its measurements are in
`what-the-carry-can-promise-measurements.md` beside it; this file stands without them. Every hash and
fixture figure quoted below names the fixture and the binary it was measured under, in the sidecar.

## 1. Overview

`tp ground` carries a disposition from one round to the next by `(text_sha, ordinal)`. **§8 of the
grounding spec (`spec/1.0.0.md`) says its carry-forward *"makes an unrepaired `FAIL` permanent while
its text stands"*.** It does not, in one direction, and the direction it fails in is the silent one.
This release adds the fence that makes the promise true in both directions and narrows the promise,
in the spec and in the emitted ask, to what the mechanism keeps.

**The finding has never fired, and that is a measurement rather than an assumption.** No recorded
round has yet carried a duplicated `text_sha`: across every floor file under both round directories,
no floor holds the same hash twice, so no multiplicity could have moved in any of the rounds that
carried. The sidecar's *The corpus, re-run at `HEAD`* is the loop and its counts. A present corpus
with carries and no duplicated unit says the defect has had its opportunities and taken none, which
is the reason to repair the mechanism before its record can be harmed by it — and §4 fences the
repair that would otherwise be tempting.

A second decision this file once carried — one whitespace set for §2.1's floor predicates — was
withdrawn: no floor unit in the corpus carries a byte the predicates disagree on, so it is recorded in
`spec/undecided.md` under *Refuted* and fenced out here (Non-Goal 6).

## 2. The decision: the join is positional, and §8 promises otherwise

### 2.1 What was measured, in both directions

`ordinal` is the 1-based index of a unit among those sharing its `text_sha`, **in emission order**.
It is positional, so deleting one of several identical units renumbers the survivors. §2.1.1 is the
silent direction, §2.1.2 the loud one.

#### 2.1.1 Deletion, and what it takes with it

Two fixtures, each run under a binary built from `dbf7fdeb`. Each is the same six-command sequence —
`tp ground <spec>`, `tp ground <spec> --record <round-1 rows>`, the edit, `tp ground <spec>`,
`tp ground <spec> --record <round-2 rows>`, `tp ground <spec> --status` — run in a scratch directory,
never in this repository, because an emission is a state write.

**The spec is three paragraphs, arranged `[copy, other, copy]`**, and the arrangement is part of the
fixture rather than an incidental of it — see the note under the deletion table. `The gate ran 3
times.` appears twice, byte-identical, hashing to `912597aa446a` both times.

**Deletion — silent.** Round 1 grades `#1 PASS` and `#2 FAIL`. Then the **first** copy is deleted:
an ordinary editorial act touching nothing about the failing sentence, which still stands in the
document byte for byte.

| | round 1 | round 2, after the deletion |
|---|---|---|
| floor | 3 | 2 |
| `carried` at emit | 0 | **2** — the ask says *"0 of the 2 floor units"* |
| `dispositioned` / `emitted` | 3 / 3 | 2 / 2 |
| `by_verdict.FAIL` | 1 | **0** |
| `--status --check` | exit 0 | **exit 0** |

The surviving `FAIL`ed sentence became ordinal 1, so it joined round 1's row for the **deleted**
copy and inherited its `PASS`. Round 2's file holds two rows, both `PASS`; the `FAIL`'s evidence
string appears nowhere in it.

**The two carried rows keep their source evidence verbatim**, and that half is the mechanism rather
than the fixture: `groundCarriedRow` replaces only `unit_id` and `anchor`, and its own comment says
why — *"a carry that re-derived any of them would be a fresh decision nobody made"*. In this fixture
that evidence is a line citation, and **under the `[copy, other, copy]` arrangement above both
citations now name lines holding a different sentence than the one graded.** Under `[copy, copy,
other]` the ordinal-1 row's citation lands on the surviving twin, which is the byte-identical *same*
sentence, so only one of the two rows is misdirected. Either way the `FAIL` is gone and `--check`
exits 0 — the finding does not turn on the arrangement, but this sentence about the citations does,
so the fixture states it.

#### 2.1.2 Insertion, and the asymmetry

**Insertion — loud.** From a two-unit spec graded `PASS` / `FAIL`, an identical copy of the `FAIL`ed
sentence is inserted **above** it. Round 2's floor is 3, `carried` is 2: the **inserted** copy takes
the `FAIL` and the original goes uncovered. `dispositioned` is 2 of 3 and `--status --check` exits
**1**. The `FAIL` is mis-attributed to text nobody graded, but the round visibly owes a disposition
and says so.

**The asymmetry is the finding.** Insertion leaves the round incomplete, which every consumer of
`--check` already branches on. Deletion leaves it complete, with a lower `FAIL` count than the
document earns. On the real corpus the same deletion on the spec with the most-repeated unit consumed
one of five dispositions by renumbering and reported the round complete — the sidecar's *§2.1.2 —
the real corpus at its sharpest input*.

### 2.2 The decision

**Take the mechanism, in its narrow form, and narrow the claim to match it.** §2.2.1 is the
mechanism and what it costs; §2.2.2 is the claim and the three places it stands in.

#### 2.2.1 The multiplicity fence

The join key stays `(text_sha, ordinal)`. What is added is a **multiplicity fence**: a unit inherits
a disposition only when the number of units carrying its `text_sha` is the same in round N as in
round N−1. When that count moves, no unit under that hash carries and every one of them is re-asked.

**Counted over distinct join keys in round N−1, never over rows.** `--record` accepts more than one
row for one unit and `groundCarryForward` resolves the repeat first-wins, so counting rows makes a
repeat look like a second unit. This is not a hypothetical: a prototype of the fence that counted
rows turned `TestARepeatedUnitCarriesItsFirstRowAndTheRecordKeepsBoth`
(`internal/engine/groundcarry_test.go`) red, and under `go test ./internal/...` it was the **only**
test that went red. Counting the keys already in the carry's own `source` map makes both directions
green.

**Both directions become loud, and nothing else moves.** Measured against a prototype built in the
copy — the fence is two loops and one `continue` in `groundCarryForward` — the deletion fixture
leaves the survivor owed and `--check` exiting 1, the insertion fixture carries the `FAIL` onto no
row, an unedited spec carries its whole floor exactly as before, and the suite stays green; the
fence cannot fire at all on a spec with no repeated unit, and repeated units are a fraction of a
percent of the corpus. The table, the per-spec counts and their derivations are the sidecar's
*§2.2.1 — the fence, measured against the prototype*.

#### 2.2.2 And the claim moves with it

The unfenced promise stands in exactly **three** places, and all
three are corrected to what the fenced mechanism promises: *a disposition is carried forward while
its unit's text stands **and the number of units carrying that text has not changed**; otherwise the
unit is re-asked.* The count is three in total, not one plus three:

| where | today |
|---|---|
| the grounding spec §8, the coverage paragraph | *"makes an unrepaired `FAIL` permanent while its text stands"* |
| the grounding spec §8, the narrowing paragraph | *"an unrepaired `FAIL` staying `FAIL` while its text stands"* |
| `groundPromptAsk`, `internal/cli/ground.go` | *"A carried disposition stands while its unit's text stands (§8)"* — **emitted in any round where at least one unit carries** |

The two searches that derive the three are in the sidecar under *§2.2.2 — where the promise stands,
derived*; both still return the same lines at `HEAD`.

The third is a promise tp makes to the reader it is asking not to re-decide those units, and **its
quantifier is narrower than it looks.** `groundPromptAsk` returns from a `carried == 0` branch —
and from the empty-floor branch above it — before reaching the `Sprintf` that carries the promise,
so **any round in which nothing carries omits it entirely**. That is round 1 always, and any later
round after a repair that moved every unit, which is the ordinary case rather than an exotic one.
Round 1 of this file's own grounding built the input: round 1 recorded on a one-unit spec, the unit's
text then replaced, and round 2's whole ask is `This round owes a disposition for the 1 floor unit
above.` with the promise absent. The correction is owed wherever the sentence is emitted; the
quantifier that says when that is must be stated on `carried`, never on the round number. **No test
asserts the sentence**: `internal/cli/ground_ask_test.go`'s ask assertion stops at *"their rows end
in"*.

### 2.3 The roads not taken

Re-keying on `unit_id` or `anchor` is rejected for the reasons `groundJoinKey`'s doc comment in
`internal/engine/groundcarry.go` already gives, and a bare `unit_id` join was built and measured to
carry dispositions written about different sentences, silently; correcting the claim alone leaves
`--check` exiting 0 over an unrepaired `FAIL`; refusing a duplicated unit at `--record` is a
record-sink decision this release has no input on. Each is argued in full in the sidecar's *§2.3 —
the roads not taken, and what each costs*.

## 3. What a reader of the grounding spec should now believe

1. **A carried disposition stands while its unit's text stands *and* the count of units carrying
   that text has not changed.** Where the count moves, every unit under that hash is re-asked. That
   is narrower than *"permanent while its text stands"*, and it is what the mechanism keeps.
2. **`by_verdict.FAIL` falling between rounds is not by itself evidence that a claim was repaired.**
   Before this release it could also mean a duplicate was deleted. After it, the units go uncovered
   and `--check` says so.
3. **No recorded round is invalidated.** The fence reads round N−1 and decides what round N carries;
   it rewrites nothing already recorded, and on a floor with no duplicated unit — every recorded
   floor today — it changes nothing at all.
4. **The emitted ask says the same thing the spec says.** The promise a round makes to its reader is
   the fenced one, stated on `carried`, in every round in which at least one unit carries.

## 4. Non-Goals

1. **No re-grading, re-hashing or rewriting of any round already recorded.** A round is what its
   roles read. A unit whose multiplicity moved carries nothing and is re-asked in the **next** round;
   no earlier round is touched.
2. **No change to `--check`'s other conditions.** The empty-floor-with-cut-units condition is
   untouched; this release changes only which units are covered, never what coverage means.
3. **No change to the join key.** `(text_sha, ordinal)` stays, with `ordinal` positional. §2.3 gives
   the cost of the alternatives.
4. **No change to `--record`.** It still accepts more than one row for one unit, and a duplicated
   unit is still accepted. Refusing either is a record-sink decision this release does not take.
   **The first-wins tie-break is named here as surviving, not as `--record`'s**: it lives in
   `groundCarryForward` — the very function this release changes — and that function's own comment
   says so, which is why §2.3 makes the same distinction. It survives this release untouched.
5. **No new workflow field.** The fence is not configurable.
6. **No whitespace set for §2.1's floor predicates.** `internal/engine/floor.go` implements §2.1's
   five steps with three predicates that disagree on one byte, and naming one set at every site was
   this file's second decision. It is withdrawn: no floor unit in the corpus carries a byte the
   predicates disagree on, so the change has nothing to fire on. `spec/undecided.md` carries it under
   *A single whitespace set for the floor — corpus-free*, with the condition that reopens it; the
   measurements stay in this spec's sidecar.

## 5. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant
that must fail it. No row is quantified over `spec/*.md`: §2.1 of the grounding spec rules a
corpus-wide assertion a standing tax, so the corpus figures are release-time measurements in the
sidecar, not tests.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.2 | on two byte-identical floor units graded `#1 PASS` / `#2 FAIL`, deleting the **first** leaves the survivor uncovered and `--status --check` exits **1** | the shipped behaviour, which carries the `PASS` onto it and exits 0 |
| 2 | §2.2 | inserting an identical copy **above** a `FAIL`ed unit leaves **both** copies uncovered and no row in round N carries the `FAIL` | fence only the shrinking direction (`prevMult > nowMult`), which leaves the insertion direction migrating the `FAIL` onto text nobody graded — and leaves row 1 green |
| 3 | §2.2 *keys not rows* | round N−1 holding **two** rows under one `(text_sha, ordinal)` still carries, and carries the first | count `prev`'s rows instead of `source`'s distinct keys — the mutant that reddened `TestARepeatedUnitCarriesItsFirstRowAndTheRecordKeepsBoth` in this release's own prototype, and the only test in the suite that saw it |
| 4 | §2.2 *inert* | a fixture of five identical units plus others, unedited across two rounds, carries every unit | fence on the **presence** of a duplicate rather than on a **change** in its count, which refuses to carry any repeated unit for the life of the cycle |
| 5 | §2.2 *radius* | in that fixture, deleting one of the five leaves exactly the four survivors owed and every other unit carried | fence per **round** rather than per hash — drop the whole carry when any hash's count moves, which passes rows 1, 2 and 4 |
| 6 | §2.2 *the claim* | the emitted ask does not contain the unfenced promise, and states the count condition, in a round where at least one unit carries | correct §8 and leave `groundPromptAsk`'s sentence: no shipped test asserts it, `ground_ask_test.go`'s ask assertion stopping at *"their rows end in"* |
| 7 | §4.1 | after round N is recorded under the fence, round N−1's own file is byte-identical to what it was before | have the carry normalise round N−1's rows in place so the counts agree — the repair Non-Goal 1 forbids, and every coverage assertion above still passes under it |
