# the-floor-names-what-it-cut — measurements

Supplemental material for `the-floor-names-what-it-cut.md`; the spec stands without it.

Every block below was moved out of `ground-command-friction-measurements.md` on 2026-09-08, where it
sat under the heading of the section it came from, and before that out of the spec body verbatim. The
section numbers in the headings are the original file's: its §4 is this spec's §2, its §5 is §3, its
§5.1 is §3.1, and its §16 is §4. Figures are quoted at the ref each block names; re-derive rather than
reading one off the page. Filenames inside `git show <ref>:<path>` and `git log -- <path>` commands
are the paths at that ref and are left as written; every other citation was rewritten to the file's
current name.

The grounding programme these measurements came out of, and the two independent field reports the
sibling splits lean on, are described in `what-the-record-does-not-say-measurements.md` under *§1 The
evidence base* and *§1.1 The second evidence base*. Report B confirmed §4 independently — *"`cut: 74`
with no indication of what was dropped or why… a sentence's justification lived in a *cut* unit, so
the graded unit only held up because of text the arms had removed"* — which is §4's *"the sharpest
finding of a round sat in cut text again and again"*, reached from the other end.

## Header: the count of decisions and why the file did not split

> **Of the first four, three are in the
> surfaces around the grounding protocol** — what the emission names, what `--record` says when it
> refuses, and what `--units` will show you. **The fourth is in the floor derivation itself, and
> naming the function in it took two tries.** §5's marker units are emitted by `floorSplitUnits`, on
> text whose list marker `floorCanonicalise` declined to strip because its gate is on the block's
> *first* line; what decides whether that happens at all is the block boundary `floorBlocks` draws,
> which emits blocks and never units. A sentence here named `floorBlocks` alone, and §5's own body
> says the other thing — *"the splitter makes it"*. Two fixtures differing in a single blank line
> separate the cases and both are one command: with no blank line before a `N. **Bold** text` list,
> `tp ground a.md --units` returns two units whose whole text is `1.` and `2.`; with the blank line,
> `tp ground b.md --units` returns none.
>
> What did hold, across the corpus below: the carry, the six verdicts, the kind–tier table and the
> atomicity of `--record`. Every defect here is about naming, reporting or splitting — none is about
> a judgement the protocol got wrong.
>
> This file takes `spec/candidates.md`'s **ground-related** material; the stub itself stays, for the
> references to that filename that survive across the tree. **The count that stood here is deleted
> rather than refreshed.** It read *29 surviving references*, which is exact at `3f80f676` and
> returns **41 occurrences across 15 files** at `9e672387`, under `CLAUDE.md`'s own rule — tracked
> non-`.tp-review` files, the bare filename, discounting `0.35.0-candidates.md` and
> `1.0.0-corrections.md`. `spec/candidates.md` restates the same stale 29 and is deliberately not
> edited from here. Derive it against a ref; do not read it off a sentence. That file held three
> kinds of thing, and `spec/undecided.md` took the larger
> share — its whole refuted record and all its non-ground undecided items; four of its undecided rows
> moved into the pending specs that own their subject; and its closed sections were deleted after each
> was verified present in the release that took it. What came here is about `tp ground`, which is what
> this release is.
>
> **This file has outgrown a patch, and saying so is this paragraph's whole job.** It opened as four
> defects one orchestrator measured against this repository's own specs (§2–§5). It now also carries
> **six more** (§9–§14), from two independent field reports on `tp ground` v1.0.0 run in two other
> repositories, on two Turkish specs, by two sessions that had not seen each other — a provenance this
> tree cannot check, and §1.1 says which claims that covers. **Ten numbered items, more declared
> decisions than that, and stating the counting rule is the point of the sentence — the count itself
> is not stated, for §5.1's reason.** Ten is §1's list plus §1.2's, which is fixed by those lists.
> The decisions are `grep -c '^\*\*The decision' spec/backlog/ground-command-friction.md` plus §9.1's own heading, which the
> anchored pattern cannot see; the anchoring matters, because a bare `grep -c 'The decision'` also
> counts this sentence and every cross-reference to one. **No value is quoted, because a count of
> this file's own text is changed by editing this file, which is the defect §5.1 takes.** What is
> durable is the shape: the two counts differ by exactly one heading, and three items carry more than
> one decision each — §5 two (its splitter rule and §5.1's ban on a figure about this file's own
> floor), §9 four (§9.1's gate, §9.2's key, §9.3's ship-order call and §9.3's characterisation label),
> §10 three (§10.1's `asked`, §10.2's `floor_delta`, §10.3's two halves).
> **Exactly one changes an exit code either way** — §9 gates `--status --check` on a standing `FAIL`,
> which Non-Goal 5 forbade until §9 amended it in §6. That is a minor release's surface, not a
> patch's. The **operator decides** whether this stays `1.0.1` or splits, and **nothing here is
> renumbered**: the roadmap table in `CLAUDE.md` is the only place a release number belongs, and a
> spec that names its own successor is how renumbering leaves stale citations behind — the sweep this
> repository ran after its own found 47 such citations, ~35 of them stale.

(Since moved here, §9.1 and §9.2 left for `spec/backlog/next-action-and-check-tell-the-truth.md`,
so the exit-code change the paragraph counts is no longer this spec's. On 2026-09-08 the file did
split, into the seven specs the stub's table names, so the operator question the last paragraph
leaves open is answered by the split rather than by a release number.)

## §4 A cut unit cannot be addressed

**Measured.** `--units` lists floor units. A reader who finds a defect in text the floor's arms cut
has no id to file against:

```
$ tp ground spec/backlog/ground-command-friction.md --units | cut -f1 | grep -cE '^u(15|16|17|18|19|38|49|50|55)$'
0
```

Those nine ids are exactly the ones that round filed rows against, having inferred each from the
splitter's merge behaviour and computed its own `text_sha` by guessing the canonicalisation. Where
that reconstruction failed the reader gave up and recorded no id at all — **13 rows across the 28
recorded rounds carry `unit_id: null`**:

```
python3 -c 'import json,glob;print(sum(1 for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip() and json.loads(l).get("unit_id") is None))'
```

**Why this is the most valuable of the four.** Across the programme the sharpest finding of a round
sat in cut text again and again, and one round diagnosed why: the arms index a section's *reasoning*
and drop the *wording it produces* — a new central clause or a user-facing message string is exactly
the shape that gets cut. So the text least visible to the floor is disproportionately the text worth
grading, and it is the text a reader cannot cite.

## §5 A bare ordered-list marker becomes a floor unit

**Measured.** Counting rule — floor units whose whole text matches `^[0-9]+[.)]$`, read from
`tp ground <spec> --units` (TSV: `unit_id`, `text_sha`, `text`), over an **enumerated** set: the
twenty files then named `spec/1.38.0.md` through `spec/1.58.0.md` (now under `spec/backlog/`), plus `spec/1.0.0.md` — twenty-one in all.
**At `9e672387`: 20 such units across 4 specs, out of 2,192 floor units scanned — 0.91%.** They are
`u84 u87 u89 u91` in `spec/backlog/brief-carries-the-forcing-sentences.md`, `u71 u73 u93 u100` in **`spec/1.0.0.md`** — the document that
defines the floor — `u191 u194 u196 u199 u204 u206 u211` in `spec/backlog/what-the-carry-can-promise.md`, and
`u128 u132 u135 u137 u141` in the spec since dropped into `spec/undecided.md` (then `spec/1.58.0.md`).

**The census moved 2.5× and the instance did not, and keeping those apart is what this paragraph is
for.** The figure first written here was *8 units across 2 specs out of 1,990 — 0.40%*, which
reproduces exactly at `22ed1c4a`, the commit that wrote it. **The eight ids it names still reproduce
at `9e672387`**; the twelve new markers all arrived in `spec/backlog/what-the-carry-can-promise.md` and the spec since dropped into `spec/undecided.md` (then `spec/1.57.0.md` and `spec/1.58.0.md`), added
the same day and edited heavily since. So the defect is neither rarer nor commoner than it was — the
corpus grew — and every share here is quoted with the ref it was measured at, because it will move
again before this is read.

**The set is enumerated because an earlier draft's phrase was not.** That draft said *"every pending
spec plus `spec/1.0.0.md`"* and quoted 1,968, and **no reading of that phrase returns it**: at
`22ed1c4a` the rule gives 1,990 for the twenty-one above and 2,064 adding this file. **A third
denominator stood here — adding this file too — and it is deleted rather than refreshed; §5.1
says why, because it was false when it was written.** The named ids reproduce under every reading;
only the denominator moved, which is the half a stated counting rule exists to pin.

**The marker is not a line in the source — the splitter makes it.** No line in either file matches
`^\s*[0-9]+[.)]\s*$` (`0` hits in both). The items are single lines of the form
`N. **Bold phrase.** text…`, and the split happens *inside* the line.

**Two things are unexplained and are left unexplained rather than guessed.** In `spec/backlog/brief-carries-the-forcing-sentences.md` the
items are numbered 1–5 and only `2.` through `5.` become units. In `spec/1.0.0.md`, 16 lines match
`^\s*[0-9]+[.)]\s+\*\*` and only 4 produce a marker unit. So the shape is necessary and not
sufficient, and what selects the four is not known. An earlier draft of this entry asserted a
mechanism — items whose bold phrase sits on the *following* line — and one command refuted it.

**The extremes a length floor would sit between.** The shortest legitimate
non-marker floor unit across the same corpus is **9 bytes** (`Measured:`, `spec/backlog/red-gate-procedure.md` `u49`, at
`9e672387`; the next shortest are 10) against the markers' 2, so any threshold in 3–8 separates
them.

## §5.1 A figure about this file's own floor does not belong in this file

**Two sentences in this release quoted this file's own floor size, and both were false when
they were written, for the same reason.** §5's triple `1,990 / 2,064 / 2,275` holds at `2c39f718`.
`git show 22ed1c4a -- spec/1.0.1.md` shows the sentence was **written** four commits after that,
where the same rule returns `1,990 / 2,064 / 2,295` — so `2,275` was already false as it was typed.
(At `9e672387` the first two are `2,192` and `2,266`; the third is not stated, for this
subsection's own reason.) It is `2,064 + 211`: this file's floor as it stood four commits earlier,
carried forward unrefreshed. And §10.3's *"it skips this file at 211"* is **that same stale
211**, in a sentence whose entire job is correcting an earlier unreproducible figure; the floor was
**247** at `339368c0`, where that correction was written, and **257** at `9e672387`. Walk it rather
than reading a trace off this page — `tp ground <this file> --units | wc -l`, run in a worktree of
each commit `git log --format=%h -- spec/1.0.1.md` names, returns a different value at nearly every
one of them.

**Stated plainly rather than quietly refreshed: this is the defect the release is about, committed
by the release.** The two places this file corrects a figure nobody could reproduce are the two
places it introduced one — one of them the same number twice, four commits apart.

**The decision: no figure about this file's own floor appears in this file.** Deleted, not annotated
with a ref. This is an exception to the rule every other figure here obeys — *state the command that
derives it and the tree state it was measured at* — and the exception is what the rule cannot cover.
Every other number has a window in which it stands: the corpus grows between commits, the code
changes at release cadence, and a held-at ref names a state a reader can check out and re-run.
**A figure about the containing file's own floor has no such window, because writing the sentence
changes the quantity the sentence measures.** The only honest ref would be the commit being composed
while the sentence is composed, which does not exist yet — so the sentence is falsified by the very
commit that records it, unless it is the last edit the file ever receives. Nothing can promise that,
and neither of these two was. So the sound forms are the derivation with no value, or nothing.

**The scope, said exactly, because §5.1 would otherwise break its own rule three times.** What is
forbidden is a **live** figure — a value this file asserts about its own present floor, or about its
own present text, which the act of asserting it invalidates. (The front matter's decision count is
the same object and is now stated as a rule with no value, for the same reason.) The pinned values
above are a different object: each is tied to an immutable past commit and is offered as *evidence
that the practice fails*, not as a fact about the document a reader is holding. A past commit cannot
be edited by writing about it, which is precisely the window a live figure lacks.

**Neither argument needed the value.** §5's point is that the denominator moves under different
readings of an unstated phrase, which its first two figures make; §10.3's is that a restricted
command returned a narrower set than the rule it was offered under, which does not turn on where
this file ranks inside it. What replaces the number in both places is the command and a ref — a
reader who wants it runs one line, and nothing in either argument waits on the answer.

(The rule itself now lives in `skills/tp/SKILL.md` Step 0.5; this subsection is kept as the
measurement that produced it.)

## §8 Tests — the mutant-column prose trimmed from rows 8 and 9 (now rows 3 and 4)

Row 8: drop every fragment shorter than **10** bytes, which also drops `Measured:` — the 9-byte legitimate unit §5 names. That is the only length floor this row kills, and it is the least tempting one: over §5's own enumerated twenty-one files at `9e672387`, markers are 2 bytes and the shortest non-marker floor unit is `Measured:` at 9 (`spec/backlog/red-gate-procedure.md` `u49`; the next shortest are 10), so every threshold in §5's own blessed 3–8 range survives this row. **An earlier draft wrote *probed over 527 real floor units*, and that number is deleted rather than refreshed: it carries no counting rule and no set returns it** — the twenty-one files hold 2,192 units, 2,172 of them non-marker, `spec/1.0.0.md` alone 401, and units of 60 bytes or fewer 173. The argument never needed a denominator, only the two extremes, and both re-derive. Row 9 is what kills those.

Row 9: implement §5 as a length floor of any kind. Two bytes is the marker's own length, so no threshold that drops the markers can keep this unit under either reading of the comparison. An earlier draft asserted a **3**-byte unit and does not kill them: under *drop when length < T* — the reading §5's 3–8 range makes natural — **T = 3 satisfies rows 8 and 9 together**, dropping every 2-byte marker, keeping `Measured:` and keeping the 3-byte unit. That is a length floor, which is the rule §5 argues hardest against, and it survived the table until this round. **The input this row named was `Go` and it cannot be built as a floor unit** — `printf '# R\n\nThe count is 12 things. Go\n' > c.md; tp ground c.md --units` prints only `u1`, because a bare `Go` fails all three of §2.1's arms; and read as the code span it is 4 bytes, which survives `T = 3` and collapses this row's own argument. It *is* a real 2-byte fragment one level up, at the splitter, pre-`inFloor`, which is where §5's decision operates — so the two readings disagree about it and the row is only buildable under one of them. **`v2` is buildable under both**: 2 bytes, and it reaches the floor on the digit arm. The same fixture with `v2` in place of `Go` prints `u1` and `u2 fb04dcb6970e v2` (built and run at `9e672387`).

## Implementation notes from the original body

The three tasks of §16 — this spec's §4 — were written naming the functions they change. The body
keeps the observable requirement; the naming is here, because none of it can be checked against
behaviour that does not exist yet:

1. `groundPromptAsk` gains `cut` as a fourth parameter, an `int` like its siblings, and its
   `floorSize == 0` arm becomes two literals that keep the opening clause
   `This round owes no dispositions:`. A count in either literal would force a `cut == 1` declension
   into a function that must not gain a third sentence.
2. `groundResult` gains an integer `cut` key, populated from the emitted index by the rule
   `groundFloorSize` reads.
3. `groundPromptAsk`'s *"walks the whole set"* comment goes, replaced by naming the generator.

## The two zeros (from the absorbed spec)

Moved from the absorbed two-zeros spec (deleted after absorption), whose three decisions are now §4 of
the spec. Every figure below was re-measured against `dbf7fdeb` in an
`rsync -a --exclude .git` copy outside the repository rather than relayed from the handover that
routed these findings here — and the third claim came back **different** from the way it was
handed over.

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

**Three shipped places already separate the two.** §11 row 23's fixture pair, `GroundStatus.Cut`'s
doc (`internal/engine/groundstatus.go:34-41`, *"only the first is a document nobody checked"*), and
`runGroundStatus`'s second `--check` condition — exit 0 on one, exit 1 on the other. Two do not, and
they are the two an operator meets **first**, before any round exists for `--status` to report:
`groundPromptAsk` (`internal/cli/ground.go:1129`) returns one literal for both, asserting *"every
unit in this document was cut"* — false on the first document, and contradicted by the index block
four lines above it **in the same prompt**; and `groundResult` (`internal/cli/ground.go:41-50`)
carries `floor_size: 0` and no `cut`, so nothing a caller can read without parsing the prompt string
tells the two apart.

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

Two entries of `spec/undecided.md` were decided onto the original spec, and both concern this one.

**From *A sentence rewritten in answer to a finding is exempt from the cut for one round*.** Decided:
**no exemption.** `tp ground --status` reports a `cut` **delta** per round — units cut that were
rewritten since the previous round — so a repair is visible once without being graded twice. The
exemption was the expensive form of the same want and would have had to be expressed in the floor's
arms; the delta is the cheap form and touches only the report. The three instances behind it are
under "§11.1" of `what-the-record-does-not-say-measurements.md`.

**From *Claim enumeration in the grounding floor*.** Decided: **the floor's own arms define a claim.**
Intuition counts — 11 where a spec carried 17, and 10 where another carried 17 again after a second
read — are not a measurement and are retired. The one measured leftover is §5 above, a bare
ordered-list marker becoming a floor unit, and it stays there as a defect of the arms rather than as
evidence about what a claim is.
