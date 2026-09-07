# tp v1.57.0 — What §8's carry can promise

> **This file is decisions.** Two of them, and both must be taken before implementation starts.
> Each is about the key `tp ground` joins a disposition to its unit by: the first is a claim the
> shipped grounding spec makes that its mechanism does not keep, the second a rule that spec never
> states at all. Everything after §4 is settled.
>
> **Every figure below names the tree state it was measured at, and that is the whole of what this
> file promises about its figures.** An earlier draft promised more — *"each is given with the
> command that derives it"* — and grounding round 1 counted the file's fenced blocks against its
> figures and refuted it. The honest split, in two kinds. **Every count over the tree** — the corpus
> counts in §1, §2.2, §3.1, §3.2 and §5, the code counts in §2.2, §2.3 and §3.1 — is given with the
> command that derives it. **Every figure that is the output of a fixture** — §2.1's two tables,
> §2.2's comparison table, §3.1's two input tables, and every `text_sha` quoted anywhere — is not
> reproducible by a command alone, so it names its fixture and its binary instead: §2.1 gives its
> six-command sequence and its arrangement, §3.1 gives the byte-exact `printf`, and the hashes were
> re-run under a shipped binary and a six-byte probe built side by side.
>
> **A sentence introduced as *"an earlier draft said"* is quoted, not asserted.** Round 1 refuted
> several of this file's claims and each repair states what it withdrew, because a correction with
> the withdrawn text deleted is one a later reader cannot check. Those quotations are the only place
> a refuted claim appears; every one of them is followed by the measurement that killed it. A reader
> grading this document should grade the surrounding sentence, never the quotation inside it.
>
> **The two tree states in this file are not interchangeable.** The fence comparison in §2.2 and the
> hash values throughout were measured against a `rsync -a --exclude .git` copy at `dbf7fdeb`, where
> the prototype was built. Every corpus count was re-derived at **`b9b24388`** after round 1, and
> eight spec files were added between the two: `spec/1.0.0.md`'s floor is 397 at `dbf7fdeb` and
> **401** at `b9b24388`. Every figure says which. Figures over `spec/*.md` are figures over a glob,
> which §2.1 of the grounding spec rules worse than either an artifact-backed one or none — they are
> quoted here only where the glob is the subject.

## 1. Overview

`tp ground` carries a disposition from one round to the next by `(text_sha, ordinal)`. Two findings
sit on that key, and they are one release because both are about how a unit's identity is computed
and matched.

**§8 of the grounding spec says its carry-forward *"makes an unrepaired `FAIL` permanent while its
text stands"*.** It does not, in one direction, and the direction it fails in is the silent one.

**§2.1 names no whitespace set**, while the pipeline that derives `text_sha` holds three different
definitions of whitespace — and the sites that disagree are not confined to hashing.

**Neither finding has ever fired, and that is a measurement rather than an assumption.** An earlier
draft of this section said there was no corpus to fire on, having counted at `dbf7fdeb` where every
count is 0. That premise was false within hours: the ground corpus was built the same day. At
`b9b24388`, `spec/.tp-review/` holds **31** recorded ground rounds over **22** specs, **9** of them
carrying a round 2 — so the carry has actually run nine times. **The first finding cannot have fired
in any of them: across all 32 tracked floor files, 1,835 floor units, no `text_sha` is duplicated in
any floor**, so no multiplicity could have moved.

```
git ls-files 'spec/.tp-review/*/ground-round-*.ndjson' | wc -l          # recorded rounds
git ls-files 'spec/.tp-review/*/ground-round-2.ndjson' | wc -l          # rounds that carried
for f in $(git ls-files 'spec/.tp-review/*/floor-ground-round-*.txt'); do
  awk 'NF>=4 && $4 ~ /^#[0-9]+$/ {print $3}' "$f" | sort | uniq -d
done | wc -l                                                            # duplicated hashes: 0
```

That is a stronger claim than the one it replaces, and it is the reason to keep the ordering: an
absent corpus says nothing about whether a defect bites, while a present one with nine carries and no
duplicated unit in any floor says the defect has had nine opportunities and taken none. So this
release still repairs a mechanism before its record can be harmed by it — and §6 fences the repair
that would otherwise be tempting.

## 2. The first decision: the join is positional, and §8 promises otherwise

### 2.1 What was measured, in both directions

`ordinal` is the 1-based index of a unit among those sharing its `text_sha`, **in emission order**.
It is positional, so deleting one of several identical units renumbers the survivors. §2.1.1 is the
silent direction, §2.1.2 the loud one and the same input on the real corpus.

#### 2.1.1 Deletion, and what it takes with it

Two fixtures, each run under a binary built from `dbf7fdeb`. Each is the same six-command sequence —
`tp ground <spec>`, `tp ground <spec> --record <round-1 rows>`, the edit, `tp ground <spec>`,
`tp ground <spec> --record <round-2 rows>`, `tp ground <spec> --status` — run in a scratch directory,
never in this repository, because an emission is a state write. An earlier draft said *"each three
commands"* without naming them, which is a count a reader cannot check.

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
citations now name lines holding a different sentence than the one graded.** The arrangement is
load-bearing and an earlier draft did not state it: under `[copy, copy, other]` the ordinal-1 row's
citation lands on the surviving twin, which is the byte-identical *same* sentence, so only one of
the two rows is misdirected. Either way the `FAIL` is gone and `--check` exits 0 — the finding does
not turn on the arrangement, but this sentence about the citations does, so the fixture states it.

#### 2.1.2 Insertion, the asymmetry, and the real corpus

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
`spec/*.md` at **`b9b24388`**, **58 of 62 files hold no duplicated `text_sha` at all, and the other
four hold six duplicated hashes between them — five at multiplicity 2 and one at multiplicity 5** —
covering 15 units of 6,916 (0.22%). The four are `spec/0.1.0.md` (the multiplicity-5 hash
`80218b571f18`), `spec/0.13.0-review-perspectives.md` (three), `spec/0.17.0-ax-improvements.md` and
`spec/0.23.0.md`. This is a figure over a glob, which §2.1 of the grounding spec ranks below an
artifact-backed one — it is quoted because the glob *is* the subject here, and because it moves
whenever a release adds a spec, it is stated with its commit and its command rather than left bare.

Derivations, run from a clean export of that commit with a freshly built `tp`:

```
for f in spec/*.md; do tp ground "$f" --units; done | wc -l
for f in spec/*.md; do tp ground "$f" --units | cut -f2 | sort | uniq -c |
  awk -v F="$f" '$1>1{print $1, F, $2}'; done | sort -rn
```

#### 2.2.2 And the claim moves with it

The unfenced promise stands in exactly **three** places, and all
three are corrected to what the fenced mechanism promises: *a disposition is carried forward while
its unit's text stands **and the number of units carrying that text has not changed**; otherwise the
unit is re-asked.* An earlier draft of this paragraph said §8's sentence is replaced and *"the same
correction is owed in three more places"*, then listed three of which two are in §8 itself — one of
them the sentence just declared replaced. The count is three in total, not one plus three:

| where | today |
|---|---|
| the grounding spec §8, the coverage paragraph | *"makes an unrepaired `FAIL` permanent while its text stands"* |
| the grounding spec §8, the narrowing paragraph | *"an unrepaired `FAIL` staying `FAIL` while its text stands"* |
| `groundPromptAsk`, `internal/cli/ground.go` | *"A carried disposition stands while its unit's text stands (§8)"* — **emitted in any round where at least one unit carries** |

Derived at `b9b24388`, and the two searches differ because the spec's copies wrap across lines while
the code's does not:

```
grep -n 'while its text' spec/1.0.0.md                    # 2, both inside §8
grep -rn 'A carried disposition stands while' internal/   # 1, groundPromptAsk
```

The third is a promise tp makes to the reader it is asking not to re-decide those units, and **its
quantifier is narrower than it looks.** An earlier draft of this row said the sentence is emitted
*"in every round from 2 on"*. It is not: `groundPromptAsk` returns from a `carried == 0` branch —
and from the empty-floor branch above it — before reaching the `Sprintf` that carries the promise,
so **any round in which nothing carries omits it entirely**. That is round 1 always, and any later
round after a repair that moved every unit, which is the ordinary case rather than an exotic one.
Round 1 of this file's own grounding built the input: round 1 recorded on a one-unit spec, the unit's
text then replaced, and round 2's whole ask is `This round owes a disposition for the 1 floor unit
above.` with the promise absent. The correction is owed wherever the sentence is emitted; the
quantifier that says when that is must be stated on `carried`, never on the round number. **No test
asserts the sentence**: `internal/cli/ground_ask_test.go`'s ask assertion stops at *"their rows end
in"*.

### 2.3 The roads not taken, and what each costs

**Re-keying.** `unit_id` and `anchor` are rejected explicitly, with two reasons that hold: `unit_id`
is numbered over every unit in document order, so one inserted sentence renumbers everything below
it, and `anchor` moves when a unit moves section. **The rejection carrying those two reasons is
`groundJoinKey`'s doc comment in `internal/engine/groundcarry.go`, not §8.** §8 names `unit_id` once,
in its reader-added row count, and does not mention `anchor` at all; an earlier draft of this
paragraph attributed the reasons to §8, where they are not. Derive it by content rather than by line
number, so the check survives §8 moving:

```
awk '/^## 8\./{f=1} /^## 9\./{f=0} f' spec/1.0.0.md | grep -n 'unit_id\|anchor'
awk '/The two cells the key does NOT use/,/^type groundJoinKey/' internal/engine/groundcarry.go
```

At `b9b24388` the first prints one line — the reader-added row count — and no `anchor` at all, and
the second prints the rejection with both reasons verbatim.

**The cost of re-keying is worse than the figure an earlier draft quoted, and in a different way.**
That draft said either key would re-decide *"most of a floor"*, on the order of the 397 dispositions
round 2 carries. That is exact under one reading — a `unit_id` join that also verifies the unit's
text re-decides essentially the whole floor — and wrong under the other. A **bare** `unit_id` join
(keyed on the id alone) was built and run on `spec/1.0.0.md` at `dbf7fdeb` with one sentence
inserted: at line 2, 302 of 398 carry and **96** are owed; at line 200, 327 carry; at line 600, 359
carry. At worst 24% of the floor, not most of it: ids run over all **523** units of that document —
397 in floor and 126 cut, from the summary line of its emitted index at `dbf7fdeb` — so a shifted id
lands on an id round 1 recorded far more often than it falls outside the record. **The 302 that do
carry are the objection**: they carry dispositions written about different sentences, silently, with
no signal that anything moved. That is strictly worse than re-deciding the floor, and it is the
reason to state. §8 chose positional `ordinal` knowingly, having measured the five identical units
above; nothing in this release disturbs that choice.

**Correcting the claim alone.** Cheaper by a function, and it leaves the measured behaviour in
place: `--check` still exits 0 on a document holding an unrepaired `FAIL`, on the input in §2.1. A
`--check` an operator cannot branch on is the failure the grounding release exists to remove, and a
corrected sentence would have to say so in the emitted prompt — telling every reader that a carried
disposition may silently become a different one. That sentence is worse to write than the fence is
to build.

**Refusing a duplicated unit at `--record`.** The stronger answer to the underlying oddity — two
byte-identical units graded differently — and deliberately not taken here. It is a change at a
record sink whose boundary this release has no input on. `groundcarry.go` records the same **kind**
of decision, at the same sink, for the same reason, about a **different** duplication: its comment
declines to refuse a duplicated *row* — two dispositions in round N−1's file under one
`(text_sha, ordinal)` — where this paragraph declines to refuse a duplicated *unit*, two
byte-identical sentences in one floor. One argument, two duplications; an earlier draft called them
the same decision.

## 3. The second decision: §2.1 names no whitespace set

### 3.1 What was measured

§3.1.1 is the three predicates and the hash they disagree on; §3.1.2 is what that reaches beyond
hashing, and what the corpus actually holds.

#### 3.1.1 Three predicates, one derivation

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
hashes `73128ec5a280` — identical to the byte-free sentence — because step 5's trim is the Unicode
one. (An earlier draft gave `912597aa446a` here, which is §2.1's fixture sentence and not this one;
§3.2's ten-input matrix, run under both binaries, is what caught it.)

#### 3.1.2 Not three sites, and not confined to hashing

**The three definitions are not three sites, and the consequences are not confined to hashing** —
stated in the body as well as in the heading above, because §2.1 step 1 drops every heading, so a
claim that lives only in one is a claim no round is asked to grade.

Counted in `internal/engine/floor.go` at `b9b24388`: **8 of its 11 compiled patterns carry `\s`**
in their source, and **`strings.TrimSpace` is called at 8 sites** —
`grep -c regexp.MustCompile internal/engine/floor.go`,
`grep regexp.MustCompile internal/engine/floor.go | grep -cF '\s'` and
`grep -c strings.TrimSpace internal/engine/floor.go`. Step 1's drop predicates are on the RE2 side
while step 2's blank-line test is on the Unicode side — inside one function. Two inputs, each a
single invisible byte, each measured against a control that differs only by that byte:

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

**Constructible, and not present in any floor — but the raw grep has moved and the difference
matters.** At `dbf7fdeb` the file list was 58 and this grep returned **0**:

```
LC_ALL=C grep -l -e "$(printf '\t')" -e "$(printf '\v')" -e "$(printf '\f')" \
                -e "$(printf '\r')" spec/*.md *.md skills/tp/*.md | wc -l
```

At `b9b24388` the same list is **66 files** and the same grep returns **1** — `spec/1.43.0.md`,
added since, whose shell excerpt carries TABs. **The claim this section actually rests on survives
intact, because it is about floors and not about files**: those TABs sit inside a fenced block,
which §2.1 step 1 drops, and across all 62 specs **no floor unit carries TAB, VT, FF, CR, U+00A0 or
U+0085 — 0 of 6,916** (§5 item 4 gives the command). So every input in this section is constructed,
none is found, and the file-level grep is quoted as the weaker check it is.

### 3.2 The decision

**Name one predicate in §2.1, as an explicit set of bytes, and use it at every site in the
pipeline.** The set is `[\t\n\v\f\r ]` — the six ASCII whitespace bytes, which is what
`isFloorSpaceByte` already holds. §3.2.1 is why a sentence alone will not do and what the change
costs the corpus; §3.2.2 is what it moves and what that rests on.

#### 3.2.1 Why the set is named, and what it costs

A spec sentence alone is not the fix, and this is the whole argument for the behaviour change: a
sentence describing the shipped code would have to name **three different sets for three steps of
one derivation**, which is precisely the thing a reader porting §2.1 cannot reproduce. §2.1 states
its algorithm because §7.2 has a reader supply a `text_sha` for a claim tp never emitted, and
because the first end-to-end run of the protocol agreed with tp only by reading the prototype's
source. A rule that is three rules is no better than the unstated one.

**Measured, the change is free on this tree and at this commit — and it was measured directly rather
than inferred.** The change §4 proposes was built over a copy of the tree, and every floor unit of
every spec dumped under both binaries and compared. At `dbf7fdeb`: 54 specs, 0 files differing.
Re-run independently at `b9b24388`, over a `git archive HEAD` export so that no uncommitted edit to
this file could enter the corpus: **62 specs, 6,916 floor units, 0 files differing.** With `$SHIP`
and `$SIX` the two binaries, from the root of that export:

```
for f in spec/*.md; do
  diff <("$SHIP" ground "$f" --units) <("$SIX" ground "$f" --units) >/dev/null || echo "DIFFERS: $f"
done
```

#### 3.2.2 What the change moves, and what the conclusion rests on

**An earlier draft licensed that conclusion by an inference instead, and the inference is false.** It
said the Unicode predicate is *"the strictly wider of the two candidates, so no narrower one can
differ either"*. It is not strictly wider: **step 5 already uses the Unicode predicate**, so replacing
it with Unicode changes nothing there while narrowing it to six bytes does. The falsifying input was
built and run: §3.1's fixture sentence with a **leading** U+00A0 hashes `73128ec5a280` under the
shipped derivation — the same as the byte-free sentence, because `strings.TrimSpace` folds the byte
away — and `7a61317d494b` under the six-byte predicate, which keeps it. A Unicode-everywhere
experiment agrees with shipped on that input and the six-byte one does not, so a null result from
the wider candidate cannot license the narrower one, and the direct comparison above is what the
conclusion stands on. **That falsifier is also why §7 row 10 says *interior*.**

**What that comparison rests on is the corpus holding none of the byte that separates them**, which
is stated here rather than left implicit, because it is the assumption the whole figure inherits:

```
LC_ALL=C grep -c -e $'\xc2\xa0' -e $'\xc2\x85' spec/*.md *.md skills/tp/*.md | awk -F: '$2>0'
```

returns nothing at `b9b24388` — 0 occurrences of U+00A0 and U+0085 in any of the 66 files. A single
non-breaking space typed into any spec moves that unit's hash under this change, so the figure is
exactly as durable as the corpus and no more.

**And what a user's tree would see is one byte in one position, not four bytes.** An earlier draft
said *"a user's spec holding one of the four bytes would see the affected unit's hash move once"*.
Ten inputs were run under both binaries — TAB, VT, FF, CR and a plain space, each interior and each
leading, on §3.1's fixture sentence — and **nine of the ten agree exactly**, at `73128ec5a280`.
Interior TAB, FF and CR are already inside RE2's `\s`, inside `isFloorSpaceByte` and inside
`unicode.IsSpace`; a leading VT is already stripped by step 5's trim, so it never reached the hash.
**Exactly one input moves a hash: an interior U+000B**, `d623d1a7a271` → `73128ec5a280`. §5 says what
to tell a user, and §6 Non-Goal 1 states the other two shapes this change has on a spec carrying a
VT, neither of which is a hash moving.

### 3.3 The roads not taken

**`unicode.IsSpace`.** What a reader "collapsing whitespace" would most naturally reach for, and it
is already what step 5 uses. Rejected because it folds U+0085 and U+00A0, which are content the
author typed rather than layout: a non-breaking space is a decision about where a line may break,
and collapsing it to an ordinary space rewrites the sentence. **Two of tp's own three predicates
already agree with that reading**, which is the only appeal to authority available here: RE2 `\s`
and `isFloorSpaceByte` both exclude U+00A0 and U+0085, and `unicode.IsSpace` alone includes them —
so standardising on the widest predicate is the one option that changes what two of the three
already do. An earlier draft argued instead that *"Markdown does not treat U+00A0 as whitespace
either"*. **That sentence is deleted rather than repaired**: "Markdown" names no single document, no
Markdown specification is vendored in this repository, and under CommonMark's *Unicode whitespace
character* class — the Zs category plus tab, LF, FF and CR — U+00A0 **is** whitespace, while under
its narrower *space character* class it is not. A claim that is true of one class and false of
another, resting on a document nothing here can read, is worse than no claim.

**And the corpus gives no evidence on this point** — 0 occurrences of U+00A0 and U+0085 across the
66 files at `b9b24388`, by §3.2's grep — so the decision rests on the argument above, which is
stated rather than dressed as a measurement.

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
3. **§2.1's whitespace is six named bytes**, at every step. That removes **one** of the things a
   reader implementing §2.1 in another language has to guess, and it is worth saying which promise
   that serves and how far it goes. The promise — *"the derivation rule is read from the document,
   not held in the checker"* — is a row of the escape table in the grounding spec's **§1**, not of
   §2.1; §2.1 is named only in that row's right-hand cell, and §2.1 carries no escape table of its
   own. Derive it with `awk '/^## 1\./{f=1} /^## 2\./{f=0} f' spec/1.0.0.md | grep -n 'the escape'`
   against `awk '/^### 2\.1/{f=1} /^### 2\.2/{f=0} f' spec/1.0.0.md | grep -c escape`, which
   returns **0**, at `b9b24388`. And the guessing does not end here: the list-marker gate, the
   blockquote allowances and the table em-dash join are each stated in §2.1's prose and in no set
   or pattern, so reproducing `text_sha` from the document alone is what §2.1 **aims at**, and this
   change moves it one step closer rather than delivering it.
4. **No recorded round is invalidated — and the corpus exists, so that is a measurement.** At
   `b9b24388` `spec/.tp-review/` holds **31** recorded ground rounds over **22** specs, **9** of
   them carrying a round 2 (§1 gives the commands). None is invalidated, for a reason the round
   count itself cannot give: **no floor unit of any `spec/*.md` carries TAB, VT, FF, CR, U+00A0 or
   U+0085 — 0 of 6,916** — and only an *interior* U+000B would move a hash in the first place
   (§3.2). A user's tree may hold rounds over a spec that does carry one; §6 Non-Goal 1 says what
   this release does about them.

   ```
   for f in spec/*.md; do tp ground "$f" --units; done | wc -l    # 6916 floor units
   ```

   with each unit's text — the third tab-separated field — tested for those six code points.

## 6. Non-Goals

1. **No re-grading, re-hashing or rewriting of any round already recorded.** A round is what its
   roles read. Where a user's tree holds ground rounds over a spec carrying one of the six bytes,
   whatever this change does to that spec's floor shows up in the **next** round and in no earlier
   one, and that needs no special case because it is the fence's own behaviour — a unit whose hash
   or multiplicity moved carries nothing and is re-asked.

   **What it does to that floor is three different things, and only one of them is a hash moving.**
   §3.1 measures all three and §3.2 withdraws the single characterisation an earlier draft of this
   Non-Goal used; it is restated here rather than left to a cross-reference, because the wrong
   version is the one a reader of a Non-Goal would carry away. An **interior** U+000B: the unit's
   hash moves, `d623d1a7a271` → `73128ec5a280`, and that unit alone goes uncovered. A U+000B before
   a **heading**: the floor *gains* a unit and a cut unit, the section's claim re-anchors `§1` →
   `§0`, and every later `unit_id` shifts — units appear and disappear rather than re-hash. A
   U+000B before an **opening fence**: the claim after the closing fence leaves the floor entirely
   and a fenced line enters it. A **leading** U+000B moves nothing at all, because step 5's trim is
   already the Unicode one. Interior TAB, FF and CR move nothing either (§3.2's ten-input matrix).
2. **No repair of the corpus, and no migration command.** The corpus is not empty — 31 recorded
   ground rounds at `b9b24388` (§1) — and there is still nothing to migrate, for a reason that is
   measured rather than assumed: **no floor unit of any `spec/*.md` carries one of the six bytes,
   0 of 6,916** (§5 item 4). A migration would have no input to run on.
3. **No change to `--check`'s other conditions.** The empty-floor-with-cut-units condition is
   untouched; this release changes only which units are covered, never what coverage means.
4. **No change to the join key.** `(text_sha, ordinal)` stays, with `ordinal` positional. §2.3 gives
   the cost of the alternatives.
5. **No change to `--record`.** It still accepts more than one row for one unit, and a duplicated
   unit is still accepted. Refusing either is a record-sink decision this release does not take.
   **The first-wins tie-break is named here as surviving, not as `--record`'s**: it lives in
   `groundCarryForward` — the very function this release changes — and that function's own comment
   says so, which is why §2.3 makes the same distinction. It survives this release untouched.
6. **No Unicode-aware canonicalisation.** §3.3.
7. **No `tp lint` rule for an invisible whitespace byte in a spec.** It is the adjacent idea and it
   is a different release's: lint runs over a spec, the fence runs over a floor, and a rule with
   nothing to fire on has nothing to be prototyped against — which is the bar this repository has
   already refuted four candidate rules against. **"Nothing to fire on" is now a narrower claim
   than it was, and the narrowing is the point.** §3.1's raw grep returns 1 file at `b9b24388`,
   not 0: `spec/1.43.0.md` was added after `dbf7fdeb` and its shell excerpt carries TABs. Those
   TABs sit inside a fenced block, so **no floor unit carries them — 0 of 6,916** (§5 item 4), and
   a rule that skipped fenced blocks would still find nothing. A rule that did not skip them would
   fire on one file, which is a prototype corpus of one and still not enough.
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
| 10 | §3.3 | a unit containing an **interior** U+00A0 keeps that byte, and hashes differently from the same sentence with a space — `c1c3daf08462` against `73128ec5a280` on §3.1's fixture sentence | standardise on `unicode.IsSpace`, which folds it to a space and reproduces `73128ec5a280`, rewriting text the author typed |
| 11 | §6.1 | after round N is recorded under the fence, round N−1's own file is byte-identical to what it was before | have the carry normalise round N−1's rows in place so the counts agree — the repair Non-Goal 1 forbids, and every coverage assertion above still passes under it |

**Row 9 is recorded because row 8 cannot reach it.** Both inputs are one U+000B and one mutant, and
the natural assertion — the index line — is byte-identical between the control and the defect for
the fence case. The measurement that separates them is the unit's own text.

**Row 10 says *interior* because the position decides what the row is pinning, and an earlier draft
did not say.** Run under both binaries on §3.1's fixture sentence: an interior U+00A0 hashes
`c1c3daf08462` under the shipped derivation **and** under this change, so the row pins behaviour
this release preserves and only the `unicode.IsSpace` mutant breaks it. A **leading** U+00A0 is the
opposite: shipped hashes `73128ec5a280` — the same as the byte-free sentence, because
`strings.TrimSpace` folds the byte away before it reaches the hash — and this change hashes
`7a61317d494b`. That input is the one place the change is not free, it is §3.2's falsifier for the
*"strictly wider"* inference, and a row written on it would be asserting a behaviour change rather
than the invariant §3.3 decided — so it is recorded here and not made a test.
