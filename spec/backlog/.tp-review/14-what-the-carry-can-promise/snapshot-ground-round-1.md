# tp v1.57.0 — What §8's carry can promise

> **This file is decisions.** Two of them, and both must be taken before implementation starts.
> Each is about the key `tp ground` joins a disposition to its unit by: the first is a claim the
> shipped grounding spec makes that its mechanism does not keep, the second a rule that spec never
> states at all. Everything after §4 is settled.
>
> **Every figure below was re-measured for this file** against a `rsync -a --exclude .git` copy at
> `dbf7fdeb`, through a binary built from that tree, and each is given with the command that derives
> it. Figures over `spec/*.md` are figures over a glob, which §2.1 of the grounding spec rules worse
> than either an artifact-backed one or none — they are quoted here only where the glob is the
> subject.

## 1. Overview

`tp ground` carries a disposition from one round to the next by `(text_sha, ordinal)`. Two findings
sit on that key, and they are one release because both are about how a unit's identity is computed
and matched.

**§8 of the grounding spec says its carry-forward *"makes an unrepaired `FAIL` permanent while its
text stands"*.** It does not, in one direction, and the direction it fails in is the silent one.

**§2.1 names no whitespace set**, while the pipeline that derives `text_sha` holds three different
definitions of whitespace — and the sites that disagree are not confined to hashing.

**Neither finding has ever fired, because there is no corpus to fire on.** Measured at `dbf7fdeb`:
`git ls-files | grep -c 'ground-round'` returns **0**, and a filename search for `ground-round-*` and
`snapshot-ground-*` under `spec/` — ignore rules disabled, so untracked artifacts are included —
returns **0** of each. So this release repairs a mechanism before it has a record,
which is the cheapest moment it will ever have — and §6 fences the repair that would otherwise be
tempting.

## 2. The first decision: the join is positional, and §8 promises otherwise

### 2.1 What was measured, in both directions

`ordinal` is the 1-based index of a unit among those sharing its `text_sha`, **in emission order**.
It is positional, so deleting one of several identical units renumbers the survivors.

Two fixtures, each three commands, each run under a binary built from `dbf7fdeb`. The spec is three
paragraphs; `The gate ran 3 times.` appears twice, byte-identical, hashing to `912597aa446a` both
times.

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
string appears nowhere in it. The two carried rows keep their source evidence
verbatim; in this fixture that evidence is a line citation, and both now name lines holding a
different sentence than the one graded.

**Insertion — loud.** From a two-unit spec graded `PASS` / `FAIL`, an identical copy of the `FAIL`ed
sentence is inserted **above** it. Round 2's floor is 3, `carried` is 2: the **inserted** copy takes
the `FAIL` and the original goes uncovered. `dispositioned` is 2 of 3 and `--status --check` exits
**1**. The `FAIL` is mis-attributed to text nobody graded, but the round visibly owes a disposition
and says so.

**The asymmetry is the finding.** Insertion leaves the round incomplete, which every consumer of
`--check` already branches on. Deletion leaves it complete, with a lower `FAIL` count than the
document earns.

**On the real corpus, at its sharpest input.** `spec/0.1.0.md` holds `**Exit codes:** 0 = success.`
five times (`80218b571f18`) — the instance §8 cites as its own reason for `ordinal`. Grading its
whole floor `PASS`, then deleting one of the five lines:

| | round-1 floor | round-2 floor | `carried` | owed |
|---|---|---|---|---|
| shipped | 337 | 336 | **336** | **0** |

One of five dispositions was consumed by a renumbering and the round reported complete.

### 2.2 The decision

**Take the mechanism, in its narrow form, and narrow the claim to match it.**

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
copy — the fence is two loops and one `continue` in `groundCarryForward`:

| fixture | shipped | with the fence |
|---|---|---|
| deletion | `carried` 2 of 2, `--check` **0** | `carried` 1 of 2, `--check` **1** |
| insertion | `carried` 2 of 3, `--check` 1, the `FAIL` on the inserted copy | `carried` 1 of 3, `--check` 1, **no** row carries the `FAIL` |
| `spec/0.1.0.md`, one of five deleted | 336 carried, 0 owed | **332 carried, 4 owed** |
| `spec/0.1.0.md` unedited, round 2 | 337 of 337 carry | **337 of 337 carry** |
| `spec/1.0.0.md` unedited, round 2 | 397 of 397 carry | **397 of 397 carry** |
| `go test ./internal/...` | pass | **pass** |

So the fence's whole cost on the sharpest real input is **4 re-asked dispositions out of 336**, and
its cost when nothing moved is zero. It cannot fire at all on a spec with no repeated unit: over
`spec/*.md`, **50 of 54 files hold no duplicated `text_sha` at all, and the other four hold six
duplicated hashes between them — five at multiplicity 2 and one at multiplicity 5** — covering 15
units of 5,474 (0.27%).

Derivations, run from the repository root with a freshly built `tp`:

```
for f in spec/*.md; do tp ground "$f" --units; done | wc -l
for f in spec/*.md; do tp ground "$f" --units | cut -f2 | sort | uniq -c |
  awk -v F="$f" '$1>1{print $1, F, $2}'; done | sort -rn
```

**And the claim moves with it.** §8's sentence is replaced by what the fenced mechanism promises: *a
disposition is carried forward while its unit's text stands **and the number of units carrying that
text has not changed**; otherwise the unit is re-asked.* The same correction is owed in three more
places, and the third is the one that matters:

| where | today |
|---|---|
| the grounding spec §8, the coverage paragraph | *"makes an unrepaired `FAIL` permanent while its text stands"* |
| the grounding spec §8, the narrowing paragraph | *"an unrepaired `FAIL` staying `FAIL` while its text stands"* |
| `internal/cli/ground.go:1169` | *"A carried disposition stands while its unit's text stands (§8)"* — **emitted to every grounding unit, in every round from 2 on** |

The third is a promise tp makes to the reader it is asking not to re-decide those units. **No test
asserts it**: `internal/cli/ground_ask_test.go`'s ask assertion stops at *"their rows end in"*, and
the phrase occurs exactly once in `internal/`.

### 2.3 The roads not taken, and what each costs

**Re-keying.** §8 rejected `unit_id` and `anchor` explicitly, and the reasons hold: `unit_id` is
numbered over every unit in document order, so one inserted sentence renumbers everything below it,
and `anchor` moves when a unit moves section. Either would re-decide most of a floor after a
one-sentence edit — on `spec/1.0.0.md` that is on the order of the 397 dispositions round 2 carries
today, against the handful actually owed. §8 chose positional `ordinal` knowingly, having measured
the five identical units above; nothing in this release disturbs that choice.

**Correcting the claim alone.** Cheaper by a function, and it leaves the measured behaviour in
place: `--check` still exits 0 on a document holding an unrepaired `FAIL`, on the input in §2.1. A
`--check` an operator cannot branch on is the failure the grounding release exists to remove, and a
corrected sentence would have to say so in the emitted prompt — telling every reader that a carried
disposition may silently become a different one. That sentence is worse to write than the fence is
to build.

**Refusing a duplicated unit at `--record`.** The stronger answer to the underlying oddity — two
byte-identical units graded differently — and deliberately not taken here. It is a change at a
record sink whose boundary this release has no input on, and `groundcarry.go` already records the
same decision for the same reason.

## 3. The second decision: §2.1 names no whitespace set

### 3.1 What was measured

§2.1 says *"collapse whitespace runs to one"* (step 3), *"followed by whitespace"* (step 4) and
*"Trim"* (step 5), and names no set for any of them. `internal/engine/floor.go` implements the three
with three predicates that do not agree:

| §2.1 step | implementation | the set |
|---|---|---|
| step 3, collapse | `floorWhitespaceRe`, Go RE2 `\s+` | `[\t\n\f\r ]` |
| step 4, split | `isFloorSpaceByte` | `" \t\n\v\f\r"` |
| step 5, trim | `strings.TrimSpace` → `unicode.IsSpace` | every Unicode space |

Measured over the six ASCII whitespace bytes, **U+000B (VT) is the sole disagreement**: RE2 `\s`
excludes it, the other two include it. U+000C agrees everywhere.

**So a leading VT is stripped and an interior one survives.** On a fixture written byte-exactly:

```
mkdir -p /tmp/vt/spec && cd /tmp/vt
printf '# F\n\n## 1. S\n\nThe gate ran\0134 times.\n' > spec/f.md   # \013 is VT; \0nnn takes 3 octal digits
python3 -c "print(open('spec/f.md','rb').read())"                   # confirm one \x0b byte
tp ground spec/f.md --units | cut -f2
```

| the unit | `text_sha` |
|---|---|
| tp's, `'The gate ran\x0b4 times.'` | **`d623d1a7a271`** |
| a reader collapsing whitespace per Unicode, `'The gate ran 4 times.'` | **`73128ec5a280`** |

`text_sha` is §8's join key, so those two never match. The same fixture with the VT **leading**
hashes `912597aa446a` — identical to the byte-free sentence — because step 5's trim is the Unicode
one.

**The three definitions are not three sites, and the consequences are not confined to hashing.**
Counted in `internal/engine/floor.go`: **8 of its 11 compiled patterns carry `\s`** in their source,
and **`strings.TrimSpace` is called at 8 sites**. Step 1's drop predicates are on the RE2 side while
step 2's blank-line test is on the Unicode side — inside one function. Two inputs, each a single
invisible byte, each measured against a control that differs only by that byte:

| input | control | with the VT |
|---|---|---|
| `\v## 1. Claims` above one claim | `1 in floor, 0 cut`; the claim anchors `§1` | `2 in floor, 1 cut`; the heading becomes prose and splits into a floor unit `## 1.` and a cut unit `Claims`; the claim anchors **`§0`** and its `unit_id` moves `u1` → `u3` |
| `\v` before an opening fence | `1 in floor, 0 cut`; the unit is the claim | `1 in floor, 0 cut`; the unit is **`` ``` this is code that ran 5 times ``** — the opening fence is unrecognised, the **closing** one toggles the fence on, and the real claim after it leaves the floor entirely |

**The second input is why the recorded characterisation of this finding needs correcting.** It was
recorded as *constructible, not biting*, with the failure direction *safe, because a mismatched hash
drops the carry and the unit is re-graded rather than falsely cleared*. That is true of the hashing
sites and false of the step-1 sites: here nothing is re-graded, because the unit never exists. The
floor's shape — `1 in floor, 0 cut` — is byte-identical to the control's, so coverage is 100% and
`--check` exits 0 in both.

**Constructible, and not present.** Over the 58 markdown files in `spec/`, the repository root and
`skills/` at `dbf7fdeb`, TAB, VT, FF and CR occur **0** times, and no floor unit of any `spec/*.md`
contains non-ASCII Unicode whitespace:

```
LC_ALL=C grep -l -e "$(printf '\t')" -e "$(printf '\v')" -e "$(printf '\f')" \
                -e "$(printf '\r')" spec/*.md *.md skills/tp/*.md | wc -l
```

### 3.2 The decision

**Name one predicate in §2.1, as an explicit set of bytes, and use it at every site in the
pipeline.** The set is `[\t\n\v\f\r ]` — the six ASCII whitespace bytes, which is what
`isFloorSpaceByte` already holds.

A spec sentence alone is not the fix, and this is the whole argument for the behaviour change: a
sentence describing the shipped code would have to name **three different sets for three steps of
one derivation**, which is precisely the thing a reader porting §2.1 cannot reproduce. §2.1 states
its algorithm because §7.2 has a reader supply a `text_sha` for a claim tp never emitted, and
because the first end-to-end run of the protocol agreed with tp only by reading the prototype's
source. A rule that is three rules is no better than the unstated one.

**Measured, the change is free on this tree and at this commit.** Across 54 specs and 5,474 floor
units, **0** `text_sha` values differ between tp's derivation and one computed with a single Unicode
predicate — the strictly wider of the two candidates, so no narrower one can differ either. Combined
with the zero recorded ground rounds in §1, nothing in the repository re-hashes. That is a fact
about this tree, not a general one: a user's spec holding one of the four bytes would see the
affected unit's hash move once, and §5 says what to tell them.

### 3.3 The roads not taken

**`unicode.IsSpace`.** What a reader "collapsing whitespace" would most naturally reach for, and it
is already what step 5 uses. Rejected because it folds U+0085 and U+00A0, which are content the
author typed rather than layout: a non-breaking space is a decision about where a line may break,
and collapsing it to an ordinary space rewrites the sentence. Markdown does not treat U+00A0 as
whitespace either. **The corpus gives no evidence on this point** — 0 occurrences of both — so the
decision rests on the argument, and that is stated rather than dressed as a measurement.

**RE2 `\s`, the narrower set.** It would take VT out of step 4's splitter and still require
replacing `strings.TrimSpace` at step 5, so it costs the same edits in the direction that drops a
byte every other definition in the pipeline calls whitespace.

## 4. Scope of the second change

Every predicate reached by §2.1's five steps moves to the named set: the step-1 drop patterns, the
step-2 blank-line test, the step-3 collapse, the step-4 split and the step-5 trim. `FloorTextSHA`
and `FloorOrdinals` are untouched — they take the canonical text and the hashes, and the change is
upstream of both.

**The section-heading pattern moves with them**, because §2.1's anchors are derived by the same
family of rule and the measured `§1` → `§0` shift above is an anchor defect, not a hash defect.

## 5. What a reader of the grounding spec should now believe

1. **A carried disposition stands while its unit's text stands *and* the count of units carrying
   that text has not changed.** Where the count moves, every unit under that hash is re-asked. That
   is narrower than *"permanent while its text stands"*, and it is what the mechanism keeps.
2. **`by_verdict.FAIL` falling between rounds is not by itself evidence that a claim was repaired.**
   Before this release it could also mean a duplicate was deleted. After it, the units go uncovered
   and `--check` says so.
3. **§2.1's whitespace is six named bytes**, at every step. A reader implementing §2.1 in another
   language reproduces tp's `text_sha` from the document alone, which is what §2.1's own escape row
   promises and did not deliver.
4. **No recorded round is invalidated, because there are none.** Verified at `dbf7fdeb` (§1): zero
   tracked and zero untracked `ground-round-*` files, and zero ground snapshots, anywhere under
   `spec/.tp-review/`. A user's tree may hold rounds; §6 says what this release does about them.

## 6. Non-Goals

1. **No re-grading, re-hashing or rewriting of any round already recorded.** A round is what its
   roles read. Where a user's tree holds ground rounds and a spec containing one of the four bytes,
   the affected units' hashes move once and those units go uncovered in the next round — which is
   the fence's own behaviour and needs no special case.
2. **No repair of the corpus, and no migration command.** There is nothing to migrate here, and
   inventing a migration for a corpus that does not exist is surface with no input.
3. **No change to `--check`'s other conditions.** The empty-floor-with-cut-units condition is
   untouched; this release changes only which units are covered, never what coverage means.
4. **No change to the join key.** `(text_sha, ordinal)` stays, with `ordinal` positional. §2.3 gives
   the cost of the alternatives.
5. **No change to `--record`.** The first-wins tie-break on a repeated `(text_sha, ordinal)` stays,
   and a duplicated unit is still accepted. Refusing it is a record-sink decision this release does
   not take.
6. **No Unicode-aware canonicalisation.** §3.3.
7. **No `tp lint` rule for an invisible whitespace byte in a spec.** It is the adjacent idea and it
   is a different release's: lint runs over a spec, the fence runs over a floor, and a rule with
   zero hits on the whole corpus has nothing to be prototyped against — which is the bar this
   repository has already refuted four candidate rules against.
8. **No new workflow field.** Neither change is configurable.

## 7. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant
that must fail it. No row is quantified over `spec/*.md`: §2.1 rules a corpus-wide assertion a
standing tax, so §3.2's corpus figure is a release-time measurement with its command, not a test.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.2 | on two byte-identical floor units graded `#1 PASS` / `#2 FAIL`, deleting the **first** leaves the survivor uncovered and `--status --check` exits **1** | the shipped behaviour, which carries the `PASS` onto it and exits 0 |
| 2 | §2.2 | inserting an identical copy **above** a `FAIL`ed unit leaves **both** copies uncovered and no row in round N carries the `FAIL` | fence only the shrinking direction (`prevMult > nowMult`), which leaves the insertion direction migrating the `FAIL` onto text nobody graded — and leaves row 1 green |
| 3 | §2.2 *keys not rows* | round N−1 holding **two** rows under one `(text_sha, ordinal)` still carries, and carries the first | count `prev`'s rows instead of `source`'s distinct keys — the mutant that reddened `TestARepeatedUnitCarriesItsFirstRowAndTheRecordKeepsBoth` in this release's own prototype, and the only test in the suite that saw it |
| 4 | §2.2 *inert* | a fixture of five identical units plus others, unedited across two rounds, carries every unit | fence on the **presence** of a duplicate rather than on a **change** in its count, which refuses to carry any repeated unit for the life of the cycle |
| 5 | §2.2 *radius* | in that fixture, deleting one of the five leaves exactly the four survivors owed and every other unit carried | fence per **round** rather than per hash — drop the whole carry when any hash's count moves, which passes rows 1, 2 and 4 |
| 6 | §2.2 *the claim* | the emitted ask does not contain the unfenced promise, and states the count condition | correct §8 and leave `groundPromptAsk`'s sentence: no shipped test asserts it, `ground_ask_test.go`'s ask assertion stopping at *"their rows end in"* |
| 7 | §3.2 *hash* | a unit with an interior U+000B hashes as though the byte were a single space | leave `floorWhitespaceRe` on RE2 `\s+`, which reproduces `d623d1a7a271` |
| 8 | §3.2 *step 1* | a heading indented by U+000B is dropped as a heading: the section's claim anchors `§1`, the floor holds one unit, and no unit is cut | change step 3's collapse alone — every hash assertion still passes, while the input reproduces `§0`, a spurious floor unit `## 1.`, a cut unit `Claims`, and every later `unit_id` shifted |
| 9 | §3.2 *fence* | a U+000B before an opening code fence still drops the fenced block, and the claim after the closing fence stays in the floor | the same mutant as row 8. It is a separate row because the floor's shape is `1 in floor, 0 cut` under **both** readings, so row 8's index assertions cannot see it — only the unit's text can |
| 10 | §3.3 | a unit containing U+00A0 keeps that byte, and hashes differently from the same sentence with a space | standardise on `unicode.IsSpace`, which folds it and rewrites text the author typed |
| 11 | §6.1 | after round N is recorded under the fence, round N−1's own file is byte-identical to what it was before | have the carry normalise round N−1's rows in place so the counts agree — the repair Non-Goal 1 forbids, and every coverage assertion above still passes under it |

**Row 9 is recorded because row 8 cannot reach it.** Both inputs are one U+000B and one mutant, and
the natural assertion — the index line — is byte-identical between the control and the defect for
the fence case. The measurement that separates them is the unit's own text.
