# tp v1.56.0 — The ask and the envelope separate the two zeros

> **This file is decisions.** Every figure below was re-measured against `dbf7fdeb` in an
> `rsync -a --exclude .git` copy outside the repository rather than relayed from the handover that
> routed these findings here — and the third claim came back **different** from the way it was
> handed over. It is corrected in §4 with the derivation beside it.

## 1. Overview

`tp ground` emits a floor of zero for two different reasons and says the same thing about both.

A document of headings and fenced blocks produces no unit at all — `# 0 in floor, 0 cut`. A
document whose every sentence the arms dropped produces units and keeps none — `# 0 in floor,
N cut`. The first is a document with nothing to check; the second is a document nobody checked, and
`spec/1.0.0.md` §2.2's whole reason for announcing the cut set is that both end-to-end runs found
defects inside it.

**Three shipped places already separate the two.** §11 row 23's fixture pair, `GroundStatus.Cut`'s
doc (`internal/engine/groundstatus.go:34-41`, *"only the first is a document nobody checked"*), and
`runGroundStatus`'s second `--check` condition — exit 0 on one, exit 1 on the other. Two do not, and
they are the two an operator meets **first**, before any round exists for `--status` to report:

1. **The ask** (§2). `groundPromptAsk` (`internal/cli/ground.go:1129`) returns one literal for both,
   asserting *"every unit in this document was cut"* — false on the first document, and contradicted
   by the index block four lines above it **in the same prompt**.
2. **The envelope** (§3). `groundResult` (`internal/cli/ground.go:41-50`) carries `floor_size: 0`
   and no `cut`, so nothing a caller can read without parsing the prompt string tells the two apart.

### 1.1 The two emissions, measured

Measured on two documents built for this file — A is a heading plus two fenced blocks, B is four
prose sentences no arm keeps. Their floor indexes read `# 0 in floor, 0 cut` and `# 0 in floor,
4 cut`. Both asks, verbatim, **byte for byte identical**:

```
This round owes no dispositions: every unit in this document was cut (§2.1), so
there is no floor to ground.
```

Both envelopes, prompt removed, byte for byte identical (wrapped here, one line on the wire):

```json
{"spec":"spec.md","round":1,"snapshot":".tp-review/spec/snapshot-ground-round-1.md",
 "floor":".tp-review/spec/floor-ground-round-1.txt","output_path":"ground-r1.ndjson",
 "floor_size":0,"carried":0}
```

A unified diff over the two whole prompts returns the index block and nothing else.

**The derivation.** Build the two documents in a copy outside the repository, emit in each, and
compare:

```
python3 -c 'import json;a=json.load(open("A/emit.json"));b=json.load(open("B/emit.json"));
pa=a.pop("prompt");pb=b.pop("prompt");k="This round owes";
print("envelope identical:",a==b);print("ask identical:",pa[pa.index(k):]==pb[pb.index(k):])'
```

**Both are taken or neither is**, which is the finding's author's own condition and this release
agrees with it. The ask is the sentence a unit acts from; the envelope is what a driver reads, and
under `tp run` nobody reads the prose at all. Taking one is choosing which reader keeps the defect.

A third finding rides with them because its subject is the same function (§4).

## 2. The ask branches on `cut`, and states no count

`groundPromptAsk(round, floorSize, carried)` gains `cut` as a fourth parameter, and its
`floorSize == 0` arm becomes two literals:

| `cut` | the ask |
|---|---|
| 0 | *This round owes no dispositions: §2.1 produced no unit from this document, so there is no floor to ground.* |
| > 0 | *This round owes no dispositions: §2.1 produced units and the arms cut every one, so there is no floor to ground.* |

**Both keep the opening clause, deliberately.** `This round owes no dispositions:` is what the unit
acts on and it is true under both readings; only what follows the colon is the part that was wrong.
That is also why the replacement test has to assert **past** it (§6 row 2) — the shipped assertion
at `ground_ask_test.go:254` matches the opening alone and passes under the collapsed literal.

**Neither literal states the cut count, and that is a decision rather than an omission.** The index
block four lines above reads `# 0 in floor, 4 cut`; restating the number in the sentence is a second
copy of a fact with nothing comparing the two, which is the argument `runGround`'s own comment gives
for not writing the carry marks into the floor file and `TestTheEmittedFloorFileIsUnmarked` gives for
not marking the graded artifact. It also avoids a declension the function cannot afford: a stated
count needs a `cut == 1` case, and a third sentence in this function is precisely the pattern §5.1
fences.

**`cut` is an `int` and not a `bool`.** Its three siblings are ints, `groundStatusResult.Cut` is an
int, and a bool at the call site would put the `cut > 0` rule in the caller — a second home for the
rule that decides which of the two documents this is.

## 3. The emission envelope carries `cut`

`groundResult` gains an integer `cut` key, present at zero rather than omitted, populated from the
emitted index by the same rule `groundFloorSize` reads (§2.2: the absence of the hash is the cut).

**The argument is the one `groundStatusResult.Cut` already made and won.** That doc says a key a
reader must first decide whether an absence stands for is not a key they can read, and it says `cut`
is *"the one key that separates the two states a denominator of zero can mean"*. The emission is the
mode where that separation is needed **earliest**: `--status` cannot answer until a round is
emitted, and by then the operator has already read the emission and acted on it.

**It is a count, not a flag.** On a partly-cut floor it is the index's cut count beside a non-zero
`floor_size`, which is what makes `floor_size: 0, cut: 4` readable as a statement about the same
index rather than as a special case.

## 4. The guard on `groundPromptAsk`, and a correction

`TestTheAskAgreesWithTheCountsItStates` states its input set as `(floorSize ∈ {0, 1, many}) ×
(carried ∈ {0, 1, all})`, and `groundPromptAsk`'s doc comment ends *"walks the whole set"*.

**Measured: the sentence is false, and the handover's reason for it was not quite right.** The
handover says the default arm — `2 ≤ carried < floorSize`, the ordinary shape of a settling round —
is *omitted*. It is not omitted from execution. Tracing every call of `groundPromptAsk` during
`go test ./internal/cli` returns **96 calls over 11 distinct `(floorSize, carried)` pairs**, and the
default arm is reached **twice**, at `(4, 3)`, through
`TestTheRoundTwoPromptAsksOnlyForTheDispositionsItOwes`. What is missing is the **assertion**: that
test's `Contains` stops at `"for 1 of the 4 floor units above"`, before the colon, so the clause the
arm actually renders is never compared to anything.

The consequence is the same and it is measured: mutating `internal/cli/ground.go:1158` from
`"the other %d already carry"` to `"the other %d already carries"` leaves **`go test ./...` green**
on every package, in a clean copy at `dbf7fdeb`.

`grep -rn 'already carry' --include='*_test.go' .` returns four lines. Two are prose comments; of the
two assertion sites, one is the **all-carried** arm's clause and the other is
`ground_ask_test.go:220`, an `assert.NotContains` on `"the other 2 already carry"`. So the default
arm's clause is asserted **absent** in one place and **present** in none — and a mutant that changes
the string leaves the `NotContains` satisfied.

**The replacement is a loop over `(floorSize, carried)` pairs, not five hand-written `t.Run`s**, and
the generated set is guarded rather than trusted: a `require` asserts the set contains a pair
satisfying `2 ≤ carried < floorSize`. A table cannot silently omit the middle of its own range the
way five named subtests can — but only if something says the middle is there.

**The comment goes with it.** *"walks the whole set"* is a claim of exhaustiveness over a set its
author enumerated; it is replaced by naming the generator, which is an artifact and cannot rot
silently.

## 5. Non-Goals

1. **No third sentence in `groundPromptAsk`.** Two literals for the empty floor and no more. Three
   consecutive audit rounds of findings on this function were each produced by a repair that added
   text about the state its author had just built a fixture for, and each missed the sibling state
   that fixture excluded. Anything this release cannot say in the two literals above belongs to a
   later release, not to a fourth sentence.
2. **No gate change, no exit-code change, no refusal.** Measured on both documents: `--units` prints
   zero bytes and exits 0, and `--record` of an empty payload is refused with the same code and the
   same message. **The unit's action is identical under both readings today** — the round owes
   nothing either way — so this release repairs what the emission *says*, not what it *does*.
   §11 row 23's `--check` conditions stay sourced from `GroundStatus.Cut` alone.
3. **The emission envelope is not re-based.** One key, for the finding's own reason. The seven others
   are unchanged.
4. **`groundRecordEmptyHint` is not repaired here** (`internal/cli/ground.go:151`), though it is a
   third sink for the same distinction. Measured: it is byte-identical on both documents and carries
   *"If the prompt asked for no dispositions because every unit was cut (§2.1)…"*. That is a
   **conditional with a false antecedent** on the 0-cut document — unhelpful, not false — which is
   materially different from an ask that asserts the cut happened. It goes to whichever release next
   opens `--record`'s hints.
5. **No survey of other zero-floor sinks.** Only the two the finding names were measured, and no
   claim is made here about a third.
6. **No new flag and no workflow field.** Nothing about this is configurable.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | on the two zero-floor documents emitted through the command, the asks **differ**, and the 0-cut ask makes no claim that anything was cut | the shipped single literal, byte-identical on both — so the row is observably red against `dbf7fdeb` before the change, and is written first |
| 2 | §2 *past the opening* | the assertion is on the clause **after** `This round owes no dispositions:`, which both literals keep | assert the opening alone — `ground_ask_test.go:254`'s shipped shape, which passes under the collapsed literal and under every possible repair |
| 3 | §2 *no count* | on a document with an empty floor and **exactly one** cut unit, the emitted ask is byte-equal to the `cut > 0` literal above | print the count in the sentence, which renders `1 units` on this exact fixture and forces the declension case §5.1 forbids; a fixture with four cut units passes either way |
| 4 | §2 *the other arms* | the five non-empty arms render identically before and after the parameter is added, asserted on the same fixtures | let `cut` reach a non-empty branch, changing what an ordinary settling round reads |
| 5 | §3 | across the two documents every envelope key is equal except `cut`, which is 0 and N, and N equals the count read back off the emitted floor index | omit the key — the shipped state, measured to leave the two payloads byte-identical |
| 6 | §3 *a count, not a flag* | on a **partly**-cut floor `cut` is the index's cut count beside a non-zero `floor_size` | set `cut` only when `floor_size == 0`, which reports 0 for every document that has a floor and makes the key unreadable |
| 7 | §4 *the generator* | the subtest table is generated from a list of `(floorSize, carried)` pairs, and a `require` asserts that list holds a pair with `2 ≤ carried < floorSize` | a loop that generates only the five pairs the hand-written subtests named — the same test with a `for` around it, which is what an implementer will write |
| 8 | §4 *the whole clause* | each generated pair asserts the sentence from `This round owes` through the end of the clause, not a prefix of it | `"the other %d already carries"`, measured green on `go test ./...` at `dbf7fdeb` |
| 9 | §5.2 *action unchanged* | on both documents `--units` prints zero bytes at exit 0, and `--record` of an empty payload exits 1 with the same message | branch `--record` or the emission on `cut`, turning a reporting fix into a gate change |
| 10 | §5.2 *one source* | `--status --check` still exits 0 on the 0-cut document and 1 on the N-cut one, and its `cut` still comes from `GroundStatus` | route the gate through `groundResult.Cut`, giving §11 row 23's condition a second source that can drift from the first |

**Row 7 is the one an implementer will get backwards.** The defect §4 measures is not that the
default arm is unreached — it is reached, twice, and traced. It is that nothing compares what the arm
renders. A table built from the five pairs the old subtests already covered reproduces exactly that:
every pair passes, the arm runs, and the mutant in row 8 still ships green. The `require` on the
generated list is what makes the loop a fix rather than a refactor.

**Rows 1, 3 and 5 are red against `dbf7fdeb` and are written and watched red first** — they are the
three the defect itself fails. Row 8 is **green** against `dbf7fdeb` and red only against its named
mutant, because the production string is correct today; that is the whole point of it, and stating
the difference is what keeps a row that pins working text from being read as a row that repairs
broken text.
