# a-round-can-be-driven-from-the-envelope — measurements

Supplemental material for `a-round-can-be-driven-from-the-envelope.md`; the spec stands without it.
Figures are quoted at the ref each block names; re-derive rather than reading one off the page. The
first two sections were written on 2026-09-11 against `18032abe` (`tp version
v1.1.2-0.20260910212136-18032abe405f`), in scratch directories outside the repository. The
`§10.x` sections were moved here on 2026-09-08 from `ground-command-friction-measurements.md`,
whose §10.1–§10.3 are this spec's §2.1–§2.3. *Implementation notes from the original body*, *The
two zeros* and the cut-delta entry came from `the-floor-names-what-it-cut-measurements.md` on
2026-09-11, with that spec's former §4, now this spec's §2.4 and §2.5.

The two earlier field reports behind §2.1–§2.3 (Reports A and B) are described in
`what-the-record-does-not-say-measurements.md` under *§1.1 The second evidence base*; their own
figures are quoted as the reports' numbers, never re-derived, and no decision turns on one.

## Field report WB-3155, verified 2026-09-11

Two items of a field report (WB-3155) touch this spec, one partly and one weakly. Neither is owned
here.

### #10 — the audit of a repair takes three rounds, and the tool does not say so

**The claim.** Over rounds 7–10 of the reporter's ground loop, FAILs went 5 → 2 → 1 → 0 and the
round's units 613 → 120 → 24 → 8 (the report's numbers, not re-derived). `carried` shows at emit,
but nothing frames the owed units as *your repairs, not yet graded*. The report asks `--record` to
say how many units the next round will owe fresh.

**Verdict: PARTLY.** The count exists at emit: the emission envelope carries `floor_size` and
`carried`, so the owed count is their difference (keys measured on a fresh emission: `carried`,
`floor`, `floor_size`, `output_path`, `prompt`, `round`, `snapshot`, `spec`). What the envelope does
not say is how much of that is new text since the previous round — §2.2's `floor_delta.added` is
that number. The `--record`-time form cannot be built as asked: at record the next round's text has
not been written, so there is nothing to count. The larger gap the same numbers show — a round can
exit `--status --check` 0 with a `FAIL` standing and the repaired sentence ungraded — is
`next-action-and-check-tell-the-truth`'s and is verified there, not here.

### #19 — neither ground nor review can see what was dropped

**The claim.** A rewritten spec can silently lose a normative sentence: ground grades what exists,
review treats the spec as authoritative, lint checks form. The reporter found two real losses by
hand, comparing `git show` of the old version against the new body and its sidecar.

**Verdict for this spec: weak.** The claim itself is CONFIRMED and owned by
`a-rewrite-names-what-it-dropped`. The only thing this spec contributes is `floor_delta.removed`, a
count of floor hashes the preceding round had and this one lacks: it says that something left, never
what, and a rewrite that rewords every sentence reports every one as removed. It is not an answer
to #19 and is not offered as one.

## Re-verified at `18032abe`

- **The two zeros.** Document A — a heading and two fenced blocks — and document B — a heading and
  four prose sentences no arm keeps — each emitted in a fresh `git init` directory with
  `tp ground spec.md`. Envelopes with `prompt` removed: identical, keys
  `carried, floor, floor_size, output_path, round, snapshot, spec`. Asks from `This round owes`
  onward: identical, both reading *"This round owes no dispositions: every unit in this document was
  cut (§2.1), so there is no floor to ground."* `tp ground spec.md --status` reported `cut: 0` on A
  and `cut: 4` on B; `--status --check` exited 0 on A and 1 on B.
- **The empty-payload hint.** `--record` of an empty file exited 1 on both documents with the same
  message and the same hint, whose second sentence is *"If the prompt asked for no dispositions
  because every unit was cut (§2.1), there is no round to record…"* — a false antecedent on A.
- **Slicing.** A three-unit floor (`## 1. A` and three sentences each carrying a digit); a payload of
  one row for `u1` recorded at exit 0 with `rows: 1`; `--status` reported `emitted: 3,
  dispositioned: 1`; `--status --check` exited 1.

## §10.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*. **All three of its claims hold.** Measured on
this spec's round 2 (then `spec/1.56.0.md`) in a copy outside this repository — floor 61 units, all 61 carried:

- **`carried` is a count.** The envelope's keys are `carried, floor, floor_size, output_path, prompt,
  round, snapshot, spec`, and `carried` is the integer 61. No list of ids appears anywhere in it.
- **The `(carried)` marking is prose only.** `grep -c '(carried)' ` over the emitted `prompt` returns
  **62** — 61 index rows plus the ask sentence. It is inside a string a driver would have to parse.
- **The floor file on disk is unmarked.** `grep -c '(carried)' spec/backlog/.tp-review/ground-command-friction/floor-ground-round-2.txt`
  returns **0**. That is deliberate and stays: `runGround`'s own comment gives the reason — *"a copy in
  the floor file would be a second statement of the same fact with nothing comparing the two"* — and
  §2's floor is the artifact the round is graded against.

So a driver that wants the ask set reconstructs it. Report A's reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**, which joins on the same pair for the same reason. *"A 20-line script
every driver will rewrite"* is precise: the script is a reimplementation of shipped tp code.

## §10.2 Floor growth conflates two different things

Report A's rounds ran 191 → 225 → 224 → 226 floor units, because repairing a spec adds sentences. A
unit is in the ask set either because its text changed or because it never existed, and the report
asks whether anything separates them.

**Measured: nothing does.** On the §11 fixture, round 3 emits with `u2` edited (previously `PASS`ed,
hash moved) and `u5` written fresh in a new section. Both are un-marked in the index, both are in the
ask set, and the ask reads *"This round owes a disposition for 2 of the 3 floor units above"*.
`floor_size` 3, `carried` 1. No key, marker or sentence in the envelope, the prompt, the floor file or
`--status` separates the re-ask from the first ask.

**Half of the report's framing does not survive, and the half that does is the useful one.**
New-versus-edited is **not recoverable from hashes at all** — identity *is* the hash, so an edited
sentence and a deleted-plus-added pair are the same event to every artifact tp writes, and no
decision here pretends otherwise. What *is* exact and cheap is the comparison the report actually
wants for its growth question: **the previous round's floor as a set of hashes against this one's.**

## §10.3 A large floor has no sharding path

Both reports. Report A split 191 units into 5 slices by section anchor, each agent spending 100–250k
tokens on 27–49 units, and writes: *"'Spawn ONE sub-agent on that prompt' does not survive a 191-unit
floor. The instruction reads as a correctness constraint (no panel, no roles) but lands as a capacity
claim."* Report B: **213 asked units out of a 240-unit floor** — 240 is that spec's floor, 213 its
round-1 ask — 306k tokens, 83 tool calls: *"it held, but a spec 2× this size would blow the context."*

**The emitted prompt does not say it.** Measured over the emission for this spec (then `spec/1.56.0.md`), at round 1
and round 2 alike: `grep -iE 'sub-agent|panel|alone|isolation|single'` over the `prompt` string
returns **one** line — *"joined, whitespace collapsed to single spaces, a list or blockquote marker
dropped,"* — about canonicalisation, and **nothing about how many readers the prompt has.** An earlier
draft said four lines; re-running returns one, and the error ran against this section's own case,
because at one match the claim is stronger. The sentence Report A quotes is `skills/tp/SKILL.md`'s
ground-loop step (line 178 at `18032abe`) *"Spawn **one** sub-agent on that prompt"*, and the reason
the same section gives for the singular is in the step above it: *"grounding asks one question of
every unit, so there is no panel and no role"*. **That is a claim about the panel, not about
capacity** — Report A's diagnosis is correct, and it is correct about SKILL.md rather than about the
emission.

**Slicing already works, and that is measured, not inferred.** On the fixture, a payload holding one
row for one of three owed units records at **exit 0** with `rows: 1`, `--status` reports
`dispositioned: 2 of 3`, and `--status --check` exits **1**. So a slice is a well-formed round
contribution, the shortfall is reported, and the gate holds until the slices are all in. NDJSON
concatenates, and grounding has no `--merge` to need. (Re-measured at `18032abe` above.)

## Implementation notes from the original body

The three tasks of the original file's §16 — this spec's §2.4 — were written naming the functions
they change. The body keeps the observable requirement; the naming is here, because none of it can
be checked against behaviour that does not exist yet:

1. `groundPromptAsk` gains `cut` as a fourth parameter, an `int` like its siblings, and its
   `floorSize == 0` arm becomes two literals that keep the opening clause
   `This round owes no dispositions:`. A count in either literal would force a `cut == 1` declension
   into a function that must not gain a third sentence.
2. `groundResult` gains an integer `cut` key, populated from the emitted index by the rule
   `groundFloorSize` reads.
3. `groundPromptAsk`'s *"walks the whole set"* comment goes, replaced by naming the generator.

## The two zeros (from the absorbed spec)

Moved from the absorbed two-zeros spec (deleted after absorption), whose three decisions are now
§2.4. Every figure below was re-measured against `dbf7fdeb` in an `rsync -a --exclude .git` copy
outside the repository rather than relayed from the handover that routed these findings here — and
the third claim came back **different** from the way it was handed over.

### The two emissions, measured

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

**Three shipped places already separate the two.** `spec/1.0.0.md` §11 row 23's fixture pair,
`GroundStatus.Cut`'s doc (`internal/engine/groundstatus.go:34-41`, *"only the first is a document
nobody checked"*), and `runGroundStatus`'s second `--check` condition — exit 0 on one, exit 1 on the
other. Two do not, and they are the two an operator meets **first**, before any round exists for
`--status` to report: `groundPromptAsk` (`internal/cli/ground.go:1129`) returns one literal for both,
asserting *"every unit in this document was cut"* — false on the first document, and contradicted by
the index block four lines above it **in the same prompt**; and `groundResult`
(`internal/cli/ground.go:41-50`) carries `floor_size: 0` and no `cut`, so nothing a caller can read
without parsing the prompt string tells the two apart. (Line numbers at `dbf7fdeb`.)

### The guard on `groundPromptAsk`, and a correction

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

### What the absorbed spec fenced out

- No third sentence in `groundPromptAsk` — three consecutive audit rounds of findings on this
  function were each produced by a repair that added text about the state its author had just built
  a fixture for, and each missed the sibling state that fixture excluded.
- No gate change, no exit-code change, no refusal. Measured on both documents: `--units` prints
  zero bytes and exits 0, and `--record` of an empty payload is refused with the same code and the
  same message. The unit's action is identical under both readings today.
- `groundRecordEmptyHint` (`internal/cli/ground.go:151`) is not repaired: it is byte-identical on
  both documents and carries *"If the prompt asked for no dispositions because every unit was cut
  (§2.1)…"* — a conditional with a false antecedent on the 0-cut document, unhelpful rather than
  false. It goes to whichever release next opens `--record`'s hints.
- Only the two sinks the finding names were measured; no claim is made about a third.

### The absorbed spec's tests

The body's rows 5–10 are these, condensed. **Row 9 below no longer holds once
`the-floor-names-what-it-cut` §2 ships**: `--units` then lists the all-cut document's units, marked
cut, so the body's row 10 keeps only the `--record` and `--check` halves.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | the ask | on the two zero-floor documents emitted through the command, the asks **differ**, and the 0-cut ask makes no claim that anything was cut | the shipped single literal, byte-identical on both — so the row is observably red against `dbf7fdeb` before the change, and is written first |
| 2 | the ask *past the opening* | the assertion is on the clause **after** `This round owes no dispositions:`, which both literals keep | assert the opening alone — `ground_ask_test.go:254`'s shipped shape, which passes under the collapsed literal and under every possible repair |
| 3 | the ask *no count* | on a document with an empty floor and **exactly one** cut unit, the emitted ask is byte-equal to the `cut > 0` literal | print the count in the sentence, which renders `1 units` on this exact fixture and forces the declension case the non-goal forbids; a fixture with four cut units passes either way |
| 4 | the ask *the other arms* | the five non-empty arms render identically before and after the parameter is added, asserted on the same fixtures | let `cut` reach a non-empty branch, changing what an ordinary settling round reads |
| 5 | the envelope | across the two documents every envelope key is equal except `cut`, which is 0 and N, and N equals the count read back off the emitted floor index | omit the key — the shipped state, measured to leave the two payloads byte-identical |
| 6 | the envelope *a count, not a flag* | on a **partly**-cut floor `cut` is the index's cut count beside a non-zero `floor_size` | set `cut` only when `floor_size == 0`, which reports 0 for every document that has a floor and makes the key unreadable |
| 7 | the generator | the subtest table is generated from a list of `(floorSize, carried)` pairs, and a `require` asserts that list holds a pair with `2 ≤ carried < floorSize` | a loop that generates only the five pairs the hand-written subtests named — the same test with a `for` around it, which is what an implementer will write |
| 8 | the whole clause | each generated pair asserts the sentence from `This round owes` through the end of the clause, not a prefix of it | `"the other %d already carries"`, measured green on `go test ./...` at `dbf7fdeb` |
| 9 | action unchanged | on both documents `--units` prints zero bytes at exit 0, and `--record` of an empty payload exits 1 with the same message | branch `--record` or the emission on `cut`, turning a reporting fix into a gate change |
| 10 | one source | `--status --check` still exits 0 on the 0-cut document and 1 on the N-cut one, and its `cut` still comes from `GroundStatus` | route the gate through `groundResult.Cut`, giving §11 row 23's condition a second source that can drift from the first |

**Row 7 is the one an implementer will get backwards.** The defect measured above is not that the
default arm is unreached — it is reached, twice, and traced. It is that nothing compares what the arm
renders. A table built from the five pairs the old subtests already covered reproduces exactly that:
every pair passes, the arm runs, and the mutant in row 8 still ships green. The `require` on the
generated list is what makes the loop a fix rather than a refactor.

**Rows 1, 3 and 5 are red against `dbf7fdeb` and are written and watched red first** — they are the
three the defect itself fails. Row 8 is **green** against `dbf7fdeb` and red only against its named
mutant, because the production string is correct today; that is the whole point of it, and stating
the difference is what keeps a row that pins working text from being read as a row that repairs
broken text.

## Decided at the 2026-09-08 decision pass

Moved here on 2026-09-11 from `the-floor-names-what-it-cut-measurements.md`, which keeps the same
entry with a pointer, because `spec/undecided.md` routes to it there.

**From *A sentence rewritten in answer to a finding is exempt from the cut for one round*.** Decided:
**no exemption.** `tp ground --status` reports a `cut` **delta** per round — units cut that were
rewritten since the previous round — so a repair is visible once without being graded twice. The
exemption was the expensive form of the same want and would have had to be expressed in the floor's
arms; the delta is the cheap form and touches only the report. The three instances behind it are
under "§11.1" of `what-the-record-does-not-say-measurements.md`.
