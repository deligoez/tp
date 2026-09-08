# tp — The ground command's own friction

Class: tool

> **This file is decisions.** **Ten defects in `tp ground` itself** — four (§2–§5) measured by one
> orchestrator against this repository's own specs, six more (§9–§14) reported from the field and
> re-measured here. Every one was run rather than reasoned about; the measurements and the commands
> that derive them are in `ground-command-friction-measurements.md`, under the heading of the section
> each came from, and no figure from that file is restated here. §9.1 and §9.2 — the `--check` gate
> and `next_action` — have moved to `spec/backlog/next-action-and-check-tell-the-truth.md` with their
> test rows, and §16 takes three tasks from the absorbed two-zeros spec.

## 1. Overview

**The evidence base.** A grounding programme ran `tp ground` across this repository's pending specs.
Which specs, the counting rule and the verdict breakdown are in the sidecar's "§1 The evidence base";
all six verdicts and all six tiers were exercised.

The four defects, in the order they cost the most:

1. **The scratch file the emission names is not unique across specs** (§2). Two units grounding two
   different specs are told to write the same filename, and the second overwrites the first at exit 0.
2. **`--record` diagnoses one bad row per invocation** (§3), so a payload with *N* schema violations
   costs *N* round trips.
3. **A cut unit cannot be addressed** (§4). `--units` lists floor units only, so a reader who finds a
   defect in text the arms cut has no id to file against and must reconstruct the hash.
4. **A bare ordered-list marker becomes a floor unit** (§5) — two bytes, no claim, and a disposition
   the round is obliged to spend.

### 1.1 The second evidence base: two independent field reports

Two Claude sessions drove `tp ground` v1.0.0 through a full loop on real specs in other repositories,
neither having seen the other's work. Their figures, what was re-derived here and what could not be,
and which two of the four defects above they confirmed independently are in the sidecar's "§1.1 The
second evidence base". Every claim here *about tp's own behaviour* was re-derived on a freshly built
binary, against this repository's own specs or against a fixture built outside it; the reports' own
figures are quoted as the reports' numbers, never re-derived, and no decision turns on one.

### 1.2 Six further defects, and where they sit

**One of them costs more than any of the four above.** In the order they cost the most:

5. **`--check` exits 0 with `FAIL`s standing** (§9). Both reports hit it independently. The gate and
   its pair — a `next_action` key on `--status` — are now
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s decisions; §9.3 states the one hole the
   gate ships with.
6. **A round cannot be driven from the envelope** (§10): the ask set is a count and a prose marker
   (§10.1), a growing floor cannot say what grew (§10.2), and a large floor has no
   **machine-readable** sharding path while the sentence that seems to forbid one is `SKILL.md`'s and
   means something else (§10.3). Sharding *as such* already works — a payload holding a strict subset
   records at exit 0, the shortfall is reported and `--check` holds; what is missing is `asked`, so a
   driver must rebuild the ask set by joining the floor against the previous round file.
7. **A carried disposition has no escape hatch the protocol permits** (§11) — the hatch exists, the
   prompt closes it, and the sink does not report an override.
8. **The coverage denominator counts non-claims** (§12), and the payload cannot be corrected by
   subtraction because it mixes a unit count with a row count.
9. **`SKILL.md` did not name `UNVERIFIABLE`** (§13) — five verdicts of six reached the document the
   operator drives the loop from. **Closed by commit** while §9–§14 were being written; §13 routes
   the durable form to a derived guard.
10. **A fear the tree refutes** (§14): re-emitting an unrecorded round is idempotent. The ask survives
    the refutation, because nothing in the **output** says so and the same call advances a recorded
    round.

**The numbering is by position and the cost order is this list, and that is a deliberate split.**
§9–§14 are appended after §8 rather than inserted after §5, because every row of §8's table names §2,
§3, §4 or §5 and a renumbering would falsify all nine at once; §15 continues §8's numbering for the
same reason, so no row index moves. The sidecar's "§1.2" records the three `skills/tp/SKILL.md`
commits that closed parts of §9, §13 and §14 while they were being written, and at which state each
section measured.

## 2. The scratch file the emission names is not unique across specs

**Measured.** `internal/cli/ground.go`, in the emission path:

```go
outputPath := fmt.Sprintf("ground-r%d.ndjson", round)
```

The round is in the name; the spec is not, so both of these print **the same string** — assert the
equality, never the literal:

```
tp ground spec/backlog/what-the-carry-can-promise.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
tp ground spec/backlog/ground-command-friction.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
```

Two units grounding two specs concurrently are told to write the same file. The second finishes and
**overwrites the first at exit 0** — no lock, no warning, and the first spec's dispositions are gone
with nothing recording that they existed. **The loss is silent; the *swap* mostly is not.** Recording
the survivor against the wrong spec is refused — emitting the first of the two specs above at round 1
and then `--record`ing a payload built from the second's floor exits **1** (*"the emitted floor gives
u3 the hash c0df7c57c079, and this row carries 9c5874db7c50"*), because `groundRowMatchesFloor`
compares `text_sha` **and** `ordinal`. Two carve-outs stop that being a full defence, both rows the
join never reaches: a cut unit's row (`unit_id` absent from the floor) is deliberately not compared,
and one with `unit_id: null` is never joined, so a cross-spec payload of only those records silently.
And no message on any path says a *second spec* was involved.

**This is not the recorded round's name, and the difference is deliberate.**
`engine.groundRoundFileName` carries the reason in its own doc comment: `ground-rN.ndjson` is *"the
scratch file a unit writes and an operator collects"*, while `--record` writes
`ground-round-<N>.ndjson` beside the snapshot and the floor. **That decision is not what this section
changes.** The defect is narrower: a scratch name must be unique among the things that may be
scratched at once, and the spec base is the only thing separating them.

**The decision: put the base in the scratch name** — `ground-<base>-r<N>.ndjson`, `<base>` being the
one the state directory already uses (`spec/.tp-review/<base>/`), so nothing new is derived and no two
specs collide.

**The sibling case is named and NOT taken here.** `roleOutputPath` (`internal/cli/prompt_framing.go`)
has two branches and only the **fallback** collides: outside a run it returns
`review-r<N>-<role>.ndjson` / `audit-r<N>-<role>.ndjson`, qualified by round and role and equally
un-qualified by spec, so two concurrently-reviewed specs collide exactly as §2's do — and outside a
run that fallback is the only branch there is, so it is the path an operator driving rounds by hand
takes. Under `tp run` it does not collide: `TP_ROUND_DIR` is set and the path becomes
`$TP_ROUND_DIR/role-<role>.ndjson.part`. It is left out because this release is `tp ground`'s
friction and because the review/audit collision needs its own acceptance over per-role emission, not
because it is not real. `spec/undecided.md` is not its home either: it has an obvious design and no
undecided part — it wants a release. How the grounding programme worked around the collision is in
the sidecar's "§2".

## 3. `--record` diagnoses one bad row per invocation

**Two properties, and only one of them is a defect.**

**Atomicity is correct and stays.** `engine.RecordGroundRound`'s doc comment states it and gives the
reason from §7.2 of the ground spec: *"a partially valid round would make coverage a lie"* — a round
recorded with its bad rows dropped is counted as a round in which those units were decided, and
nothing in the record says otherwise. Validation completes before anything is opened, created or
truncated. **Nothing here changes that.**

**Diagnosis is first-error-only, and that is the defect.** `parseGroundRows` returns on the first
invalid row, so the caller reports one. Measured on a payload with three rows broken the same way (a
`document` claim given `tier: run`, which §4.1 refuses):

```
{"error":"line 2: field \"tier\": \"run\" says nothing about a \"document\" claim (§4.1), and a PASS row must be reached at a tier that does", ...}
```

One line named, three broken. Each fix is a full round trip, and in the reset-native model a round
trip is a subagent re-invocation. It is a **P2 violation on tp's own terms** — what is easy for one row
is not equally easy for *N* — and the sidecar's "§3" records the whole-file validator the operator
wrote to get around it.

**The decision: collect every row's violations and report them together**, keeping the write atomic
exactly as it is. The refusal stays one refusal — it gains a list.

## 4. A cut unit cannot be addressed

**Measured.** `--units` lists floor units only, so a reader who finds a defect in text the floor's arms
cut has no id to file against, infers one from the splitter's merge behaviour, and computes a
`text_sha` by guessing the canonicalisation; where that fails the reader records no id at all, and the
recorded corpus carries rows with `unit_id: null` for exactly that reason (sidecar, "§4"). The text
least visible to the floor is disproportionately the text worth grading — the arms index a section's
*reasoning* and drop the *wording it produces* — and it is the text a reader cannot cite.

**The decision: `--units` lists cut spans too**, each with an id, its `text_sha` under the same
canonicalisation the floor uses, and a marker saying it is cut. A reader then files against a real id
and a hash tp computed, and a wrong join becomes impossible rather than repairable.

**What this does NOT do, so it is not read as more than it is.** It does not put cut text in the
floor, does not make a cut unit owed a disposition, and does not change coverage or `--check`. The
floor's arms decide what a round *must* answer; this decides what a reader can *name*. Whether the
arms cut the right things is a different question and is not taken here.

## 5. A bare ordered-list marker becomes a floor unit

**Measured.** Floor units whose whole text matches `^[0-9]+[.)]$` exist across this repository's
specs, `spec/1.0.0.md` — the document that defines the floor — among them; the counting rule, the
enumerated set and the ids are in the sidecar's "§5". **The marker is not a line in the source — the
splitter makes it.** No line in the affected files matches `^\s*[0-9]+[.)]\s*$`; the items are single
lines of the form `N. **Bold phrase.** text…`, and the split happens *inside* the line. Which items
of that shape produce a marker unit is not explained and is left unexplained rather than guessed: the
shape is necessary and not sufficient, and an earlier mechanism asserted for it was refuted by one
command.

**The decision: the splitter drops a fragment whose whole text is an ordered-list marker.** The test
is the marker shape, not a length floor. A length floor would separate the markers from every
legitimate unit in today's corpus — the sidecar gives the two extremes — and it is still the wrong
rule, because it says nothing about what it means and would silently start dropping real units the
day someone writes a shorter one.

### 5.1 A figure about this file's own floor does not belong in this file

**The decision: no figure about this file's own floor appears in this file.** Writing the sentence
changes the quantity the sentence measures, so no ref can pin it; the rule now lives in
`skills/tp/SKILL.md` Step 0.5, and the two sentences of this file that were false as they were typed
are the sidecar's "§5.1".

## 6. Non-Goals

1. **No change to the floor's *arms*.** The arms are §2.2's cut step — `internal/cli/ground.go` prints
   *"§2.1 produced %d units and the arms cut every one"* and `groundcarry.go` has *"The absence of the
   hash is the cut (§2.2)"* — and nothing here touches it: §4 makes what they cut *nameable* without
   moving what they cut, and §10.3 splits the reading of a floor and never the floor. §5 removes
   two-byte list markers from the floor — *fragments*, not sentences — so the non-goal stands under
   tp's own vocabulary.
2. **No change to `--record`'s atomicity or to §7.2's table.** The corpus exercised both and they
   held; §3 changes what a refusal *says*, never what it refuses. §11 leaves the `(text_sha, ordinal)`
   join and the inheritance untouched and changes what the prompt permits and what `--record`'s
   envelope reports about an override.
3. **No change to the recorded round's filename.** `ground-round-<N>.ndjson` is deliberate and §2
   says why it stays.
4. **The review/audit scratch-name collision is named in §2 and not fixed here.**
5. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.
6. **No `--merge` for ground, and no orchestration.** §10.3 establishes that a slice records and
   gates correctly and that NDJSON concatenates; it adds no command to do the concatenating and
   nothing that spawns, schedules or counts readers.

## 7. Open questions inherited when `spec/candidates.md` was split

Claim enumeration — the weakest step of the grounding protocol, of which §5 is one small measured
piece — is an untaken decision and lives in `spec/undecided.md`. The other item that arrived here,
*"What `UNVERIFIABLE` costs"*, is closed by measurement rather than by a decision (sidecar, "§7").

## 8. Tests

Every row derives from a numbered decision and names an input that must fail it. Where a row's mutant
is a change to the test rather than to the product, the row says so. The mutant-column reasoning
trimmed from rows 4, 8 and 9 is in the sidecar's "§8".

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | two specs emitted at the same round produce **different** `output_path` values, asserted on the pair rather than on either alone | keep the round-only name, under which the two are equal and a concurrent run silently overwrites |
| 2 | §2 | the emitted `output_path` contains the same base the state directory uses, asserted by deriving the base rather than by matching a literal | hardcode a base in the test, which passes for the fixture spec and for no other |
| 3 | §2 *unchanged* | the **recorded** file is still `ground-round-<N>.ndjson` | rename the recorded file to match the scratch name, which is the decision §2 explicitly does not take |
| 4 | §3 | a payload with **three** rows broken three *different* ways reports three violations naming three distinct line numbers | report the first and stop — the shipped behaviour; the three break differently to also kill a mutant collecting at most one violation per class |
| 5 | §3 *atomicity* | after a refused `--record`, the state directory is byte-identical to what the preceding emission left | validate row by row as each is appended, which satisfies §3's wording for a payload whose first row is bad and breaks it for every other |
| 6 | §4 | `--units` lists an id for a span the floor cut, and the `text_sha` on that line equals the hash of the text printed beside it | list the cut span with no hash, which puts the reader back to guessing the canonicalisation — the exact failure the `unit_id: null` rows show |
| 7 | §4 *bounded* | a cut unit is **not** owed a disposition: coverage and `--check` answer identically before and after §4, on one spec with at least one cut span | count cut spans toward coverage, which makes every document permanently uncovered |
| 8 | §5 | a document with a `N. **Bold.** text` list produces no floor unit whose whole text matches `^[0-9]+[.)]$`, **and** produces the same number of other units as before | drop every fragment shorter than a fixed byte count, which also drops the shortest legitimate unit the sidecar names |
| 9 | §5 *not a length rule* | a legitimate **2-byte** unit that is not a marker (`v2`) survives | implement §5 as a length floor of any kind — two bytes is the marker's own length, so no threshold that drops the markers keeps this unit |

## 9. `--check` exits 0 with `FAIL`s standing

§9.1 (the gate: `--status --check` exits 1 when the latest recorded round holds a `FAIL`) and §9.2
(`--status` carries `next_action`) are `spec/backlog/next-action-and-check-tell-the-truth.md`'s §2
and §3, with §15's former rows 10–13; the measurement that motivated them is the sidecar's "§9" and
"§9.1".

### 9.3 The limitation the gate ships with, and why it ships anyway

The gate has a hole that runs the other way from a deadlock: on a spec whose two sections hold a
byte-identical sentence, editing the section that does *not* carry the `FAIL` shifts the
`(text_sha, ordinal)` join onto the `PASS` row, and the `FAIL` is cleared with no repair at all
(constructed and run in the sidecar's "§9.3", with the two exposure counts over this repository).
`spec/backlog/what-the-carry-can-promise.md` §2.2 takes that defect.

**The decision: the gate ships before that fence, with this limitation stated and guarded by §15
row 24, and the test implementing row 24 carries the characterisation in its own name and doc
comment, the doc comment naming the mutant that must retire it** — the multiplicity fence. A green
test asserting that a `FAIL` is cleared by editing an unrelated section, with nothing at the test
saying the green is deliberate, is the shape most likely to be tidied away by someone who reads it as
a bug in the test.

## 10. Driving a round: three things the emission cannot hand a driver

Three findings that look separate in the reports and are one subject: what a process other than the
reader itself can do with a round.

### 10.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*, and all three of its claims hold (sidecar,
"§10.1"): `carried` is a count, the `(carried)` marking is prose inside the `prompt` string, and the
floor file on disk is unmarked — deliberately, for the reason `runGround`'s own comment gives: *"a copy
in the floor file would be a second statement of the same fact with nothing comparing the two"*. So
a driver that wants the ask set reconstructs it, and the reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**. That is the same P2 tell §3 records, in a different surface.

**The decision: the emission's envelope carries `asked`, the list of `unit_id`s this round owes.** A
list beside the count, not instead of it — `carried` and `floor_size` are what an operator branches
on and stay exactly as they are. It is added to the **envelope**, never to the floor file, for the
reason quoted above. And it is the shipped fact rather than a new one: `asked` is
`floor_size − carried` ids, the same set the prompt's ask sentence already names in English.

### 10.2 Floor growth conflates two different things

A unit is in the ask set either because its text changed or because it never existed, and nothing
in the envelope, the prompt, the floor file or `--status` separates the re-ask from the first ask
(sidecar, "§10.2"). New-versus-edited is **not recoverable from hashes at all** — identity *is* the
hash, so an edited sentence and a deleted-plus-added pair are the same event to every artifact tp
writes, and no decision here pretends otherwise. What *is* exact and cheap is the comparison the
growth question wants: **the previous round's floor as a set of hashes against this one's.**

**The decision: the envelope reports `floor_delta` against the preceding round's floor —
`{added, removed, unchanged}`, counts over `text_sha`.** That separates net new sentences from
whatever churn replaced existing ones. Stated with its limit, because the limit is the reason the
other cut was refused: **a rewritten sentence is one `added` and one `removed`, indistinguishable
from an unrelated insertion and an unrelated deletion.** The counts bound the edit churn; they do not
identify it.

### 10.3 A large floor has no sharding path, and the instruction that seems to forbid one is not the emission's

Both reports sharded a large floor across several readers and read *"Spawn **one** sub-agent on that
prompt"* as a capacity claim. **The emitted prompt does not say it** — it says nothing about how many
readers it has (sidecar, "§10.3"). The sentence is `skills/tp/SKILL.md`'s ground-loop step, and the
reason the same section gives for the singular is in the step above it: *"grounding asks one question
of every unit, so there is no panel and no role"*. That is a claim about the panel, not about
capacity. **Slicing already works, measured and not inferred**: a payload holding a strict subset of
the owed units records at exit 0, `--status` reports the shortfall, and `--status --check` exits 1
until the slices are all in. NDJSON concatenates, and grounding has no `--merge` to need.

**The decision has two halves and neither is a code change to the emission.** SKILL.md's step says
what it means — **one panel, any number of readers**, because the floor is a partition and a slice of
it is a well-formed ask, with the recorded evidence above that a slice records and gates correctly.
And the emitted prompt **stays silent** on the question, deliberately: it is addressed to whoever is
reading the units, and a sentence about process there would be the emission telling an orchestrator
how to spend its context, which is not something tp knows. `asked` (§10.1) is what makes a slice
expressible without a join, which is why §10 is one section and not three.

## 11. A carried disposition has no escape hatch the protocol permits

Report B. A `PARTIAL` on unit X whose *cause* lives in section Y: fix Y, X's own text is unchanged, so
X carries its stale `PARTIAL` forward indefinitely. Their concrete case is a test-case unit flagged
because §4's window bound said `23:59` while the test said `23:59:59`; they fixed §4 and the unit
still carried the old `PARTIAL`. Constructed and confirmed in the reporter's own shape on a fixture
outside this repository (sidecar, "§11"): the repaired unit is asked about, the unit the repair was
for carries its stale `PARTIAL`, and the prompt tells the reader *"do not decide those units again,
and write no row for them."*

**The report's claim is right in effect and wrong in mechanism, and the difference is the whole
decision.** A hatch exists: a row naming a carried unit **overrides the carry** and records at exit 0 —
`groundCarryForward` takes the round's own payload as `decided` and does not carry what the round
decides, which is the documented behaviour: *"A unit it decides is not also carried."* So the
mechanism has the hatch and **the protocol closes it**. Two sentences do: the ask states the unit is
not owed, and the prompt says to write no row for it. A reader following the prompt cannot reach the
override; a reader who ignores the prompt gets it silently, and the `--record` envelope reports
`carried: 0` with nothing saying an inherited disposition was displaced. The field produced exactly
that route once more, on `spec/1.1.0.md`'s round 3 (sidecar, "§11").

**The decision, in two parts.** The prompt **names the override** — a carried disposition may be
re-decided by writing a row for that unit, and the round that does so says why in `note` — replacing
the unconditional *"write no row for them"* with the condition it means: do not re-decide a carried
unit **to repeat its verdict**; re-decide it when the ground beneath it moved. And `--record`'s
envelope **reports the displacement**: a count of carried dispositions the payload overrode, beside
`rows` and `carried`, so an override is never silent at the sink. Both are needed by the gate in
`spec/backlog/next-action-and-check-tell-the-truth.md`: a gate on a standing `FAIL` needs an exit that
the protocol permits and the record shows.

**What this does not do.** It does not make the carry re-derive a disposition, does not invalidate a
carry when another unit changes — tp cannot know that §1 is *why* §2 was `PARTIAL`, and the fixture's
`note` is the only place that lives — and does not touch the `(text_sha, ordinal)` join. It makes the
override sayable and visible; deciding when it is right stays the reader's. The instruction to use the
hatch must not be written into `skills/tp/SKILL.md` while the prompt still forbids it: a skill saying
*write the row* beside a prompt saying *write no row for them* turns an undocumented gap into a
documented contradiction, and this decision is what closes it. Two further properties of the carry
the field instance exposed — a plan written against unit ids does not survive an emission, and a
repair can remove the sentence it repairs from every future floor — are the sidecar's "§11.1"; the
second is an entry in `spec/undecided.md`.

## 12. The coverage ratio's denominator counts non-claims, and the payload cannot be corrected by subtraction

Both reports want a claims-only denominator beside the raw one, on the ground that a ratio over a
floor a sizeable share of which asserts nothing is not the number it looks like. Derived over this
repository's own corpus (sidecar, "§12"), the `NOT-A-CLAIM` share of a round-1 floor varies by an
order of magnitude document by document — not a constant a reader can mentally correct for, which is
why it has to be reported.

**And the payload cannot be corrected by hand, which is the measured half.** `emitted` and
`dispositioned` count **units**; `by_verdict` counts **rows** — `GroundStatus`'s own doc says so
(*"the breakdown's total is the round's row count and need not equal `Dispositioned`"*) — so
`emitted − by_verdict["NOT-A-CLAIM"]` subtracts a row count from a unit count and is wrong by exactly
the reader-added and off-floor rows, which move neither side of the ratio. Nothing in the payload
labels it as unavailable — the two counts sit adjacent and read as commensurable.

**The decision: `--status` reports the claims-only denominator itself** — the count of **emitted floor
units** whose disposition is `NOT-A-CLAIM` — beside `emitted` and `dispositioned`, under a name that
cannot be confused with `by_verdict`'s row count. The raw ratio stays exactly as it is: §8's *did
anyone look* is a question about the floor, and narrowing its denominator would change what coverage
means. This adds the second reading; it replaces nothing.

## 13. `SKILL.md` did not name `UNVERIFIABLE` — closed by commit, and here is what keeps it closed

**Recorded as closed rather than dropped**, on the rule §5 and §14 follow: an entry deleted once
someone fixes it leaves nothing that would notice the fix being undone. At `v1.0.0`, the state both
reports read, `skills/tp/SKILL.md` named five of the six verdicts; the commit *"name all six verdicts
where the loop is driven, `UNVERIFIABLE` included"* closed it (sidecar, "§13").

**The decision is therefore not the paragraph but the guard**: §15 row 21 asserts that every verdict
in the set the code exports appears in `SKILL.md`, derived from `GroundVerdicts()` rather than from a
literal list. The guard is one-directional — it does not fail when a verdict is deleted from
`groundVerdictOrder`, because the assertion quantifies over the code's set — and closing that third
direction needs a second, opposite assertion this release does not take.

## 14. A fear the tree refutes, and the ask that survives the refutation

Report B feared that a sub-agent running bare `tp ground <spec>` to fetch its own prompt would open a
new round and orphan the one in flight. It worked around this with saved envelopes and explicit
warnings in six briefs.

**The fear is false.** Two consecutive bare emissions on an unrecorded round return the same round
number, the same floor size and a byte-identical floor file (sidecar, "§14"). **Re-emitting an
unrecorded round is idempotent** — `NextGroundRound` answers *recorded rounds + 1*, and the emission
rewrites the same two files. `skills/tp/SKILL.md`'s ground loop now says so.

**The ask survives the correction.** Nothing in the **output** says it is idempotent, and once a round
is recorded a bare emit legitimately opens round N+1, so the same call is safe in one state and
round-advancing in the next, with no way to ask which state you are in from the emission's own
output. `--status` and the state directory both answer, at the cost of a call the reader that matters
cannot make — the sub-agent holding only its prompt — and none of `tp ground`'s flags re-prints a
prompt.

**The decision: a read-only way to re-print the current round's prompt**, which emits nothing, writes
nothing, advances no round, and refuses rather than emitting when there is no round in flight. Named
here as a decision and not as a design: whether it is a flag on `ground` or a mode of `--status` is
the implementing task's call, and the property that matters is that it cannot be the same call as the
emission — a mode that is safe or destructive depending on state is the shape the report was right to
be afraid of even though its specific fear was wrong.

This is the second report claim the tree contradicts. The first is §11's: the escape hatch exists in
the mechanism and is closed by the prompt. Both are recorded with the measurement rather than quietly
corrected, on the same rule §5 and the header follow.

## 15. Tests for §9–§14

The same shape as §8 and continuing its numbering, so no row index moves. Rows 10–13 moved with
§9.1 and §9.2 to `spec/backlog/next-action-and-check-tell-the-truth.md`. Where the mutant is a change
to the test rather than to the product, the row says so; that is row 19. Row 24 is the one
**characterisation** row: it asserts a defect this release does not close, and its named mutant is
the fix that closes it. The mutant-column reasoning trimmed from rows 21 and 24 is in the sidecar's
"§15".

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 14 | §10.1 | `asked` holds exactly the ids the prompt's ask sentence counts: `len(asked) == floor_size - carried`, on a fixture with at least one carried and one asked unit | emit `asked` as every floor unit, which is right in round 1 and wrong from round 2 on |
| 15 | §10.1 *unchanged* | the floor file on disk still carries **zero** `(carried)` markers after §10.1 ships | write the marks into the floor, which is the decision `runGround`'s own comment refuses |
| 16 | §10.2 | on a round where one unit was edited and one written fresh, `floor_delta` reports `added: 2, removed: 1` against the preceding floor | compute the delta against the current spec rather than the preceding round's floor, which reports `added: 0` for a spec nobody edited since the emission |
| 17 | §10.3 | a payload holding a strict subset of the owed units records at exit 0, `--status` reports the shortfall, and `--check` exits 1 | refuse a partial payload, which forbids the split both reports had to perform |
| 18 | §11 | a row naming a carried unit displaces the inherited disposition, and `--record`'s envelope reports the displacement count as 1 | report only `rows` and `carried`, under which the override records silently — the shipped behaviour |
| 19 | §11 *the stale carry* | on the §11 fixture, after §1's repair, `u2` is still marked `(carried)` and is still not in the ask | assert that repairing §1 clears `u2`'s disposition, which no join in tp can do and which this section explicitly does not decide — a test-side mutant, and the row that keeps §11 from being read as more than it is |
| 20 | §12 | the claims-only count equals the number of **emitted floor units** disposed `NOT-A-CLAIM`, on a round carrying a reader-added and an off-floor `NOT-A-CLAIM` row as well | derive it as `emitted - by_verdict["NOT-A-CLAIM"]`, which returns 0 where the answer is 2 on exactly that fixture |
| 21 | §13 | all six of §3's verdicts appear in `SKILL.md`, derived from the verdict set the code exports rather than from a literal list | append a seventh verdict to `groundVerdictOrder` and leave `SKILL.md` alone — the derived guard fails naming it, the literal-list version passes |
| 22 | §14 | two bare emissions on an unrecorded round leave the round number and the floor file's bytes identical, asserted on the pair | assert only that the second call exits 0, which is true of a call that opened a new round |
| 23 | §14 | the read-only re-print emits no round: the state directory is byte-identical before and after, and it refuses when no round is in flight | implement it as a bare emit with the write skipped, which is idempotent on an unrecorded round and advances the round on a recorded one |
| 24 | §9 *the limitation* | on a fixture whose §1 and §2 hold **byte-identical** sentences, a round-1 `FAIL` recorded against the §2 copy is cleared at exit 0 by editing §1 alone and grading nothing — asserted as the shipped behaviour this release does **not** close (§9.3), so the row states the hole rather than denying it | land `spec/backlog/what-the-carry-can-promise.md` §2.2's multiplicity fence, under which the unit is re-asked, the `FAIL` survives, and this row goes red — its purpose |

## The silent overwrite: emitting over an unrecorded round

**Measured on this repository's own hotfix cycle** (sidecar, "The silent overwrite"): a round was
graded, the spec repaired before the round was recorded, and `tp ground <spec>` re-emitted round 1
against the repaired text, **overwriting the unrecorded round's floor file without a word**; `--status`
then reported a round with no dispositions — indistinguishable, from the record alone, from a round
nobody has graded yet. Recovery was possible only by accident, and the cost of the missing guard was
one full round of grading.

**Review and audit already have the signal ground lacks.** Both expose `in_flight_round` — a snapshot
with no recorded round file — and `tp resume` routes to `record-round` on it. Ground records by
filename rather than through `state.json`, so it has no equivalent and nothing notices.

**What this release should do**: when a floor exists for a round that carries no recorded findings
file, and the floor the current text would produce differs from it, refuse the emission, name the
round, and say the earlier one is unrecorded. `--force` overwrites. An identical floor is the
idempotent re-emission the loop relies on and must stay silent.

One fact worth carrying into the design, measured during the recovery: **`--record` matches rows
against the floor file rather than against the spec's current hash.** That is what made recording a
rescued round possible at all, and it means the guard belongs on the emit path rather than the record
path.

## `FloorAnchorOf` bills six kinds of unit to an anchor no reader would predict

Found while a lint field grouped by anchor was being specified and the specification kept describing
behaviour the function does not have (sidecar, "`FloorAnchorOf`"). The field was dropped; the six
defects are the function's and belong here. Each is a fixture built and run against
`engine.FloorAnchorOf`.

| # | fixture | anchor produced | what a reader expects |
|---|---|---|---|
| 1 | a unit under `## 2. Second` with no blank line above the heading | `§1` | `§2` |
| 2 | a unit under a top-level `## Unnumbered parent` that follows a `### 1.1 Child` branch | `§1.1` | the preceding *top-level* numbered section |
| 3 | a document whose numbered headings are all `# 1.`-style H1s | every unit `§0` | `§1` and after |
| 4 | a `## 9.` heading inside a ` ``` ` block that is itself inside a `~~~` block | `§9` | no anchor — it is a code sample |
| 5 | a `## 9.` heading in an indented code block | `§9` | no anchor |
| 6 | `## 2026-09-07 release notes` and `## 0.19.0 — Agent Friction Reduction` | `§2026`, `§0.19.0` | no anchor — neither is a section number |

Two of these are one bug with two faces. `floorSectionHeadingRe` matches `#{2,6}`, which is why a
numbered H1 opens nothing (#3), and `floorFenceRe` toggles on ` ``` ` **or** `~~~` without recording
which delimiter opened the block, so a ` ``` ` line closes a `~~~` fence (#4). Both have a code
comment conceding the gap; neither has a test.

**What makes this worth a release rather than a note.** The sum is not affected — every uncut unit
gets exactly one anchor, so any grouping still totals the floor size, and that invariance is exactly
why the defect survived a grading round that asserted the sum. A consumer of anchors therefore
cannot detect any of the six by checking its own arithmetic; only a fixture whose expected anchor is
written down can. Any release that ships an anchor-keyed output must ship these fixtures with it.

**Non-goal, stated because it was the first proposal:** this is not a request to redefine the
family's notion of a code block. `~~~` and indented blocks are out of scope for every rule in
`internal/engine/vague.go` by decision. Defects 4 and 5 are in scope only because `floorAnchorsByLine`
carries a comment claiming *"a heading inside a fence is code"* — the defect is the gap between the
comment and the code, not the family's scope.

## 16. Tasks taken from the two-zeros spec

`tp ground` emits a floor of zero for two different reasons and says the same thing about both: a
document of headings and fenced blocks produces no unit at all, and a document whose every sentence the
arms dropped produces units and keeps none. `GroundStatus.Cut` and `--status --check` already separate
the two; the ask and the emission envelope — the two surfaces an operator meets first — do not. The
absorbed spec's measurements, non-goals and test rows are the sidecar's "The two zeros (from the
absorbed spec)". Three tasks, taken whole:

1. **The ask branches on `cut` and states no count.** `groundPromptAsk` gains `cut` as a fourth
   parameter, an `int` like its siblings, and its `floorSize == 0` arm becomes two literals that keep
   the opening clause `This round owes no dispositions:` — one saying §2.1 produced no unit, one
   saying it produced units and the arms cut every one. Neither literal states the cut count: the
   index block above it already does, and a count would force a `cut == 1` declension into a function
   that must not gain a third sentence.
2. **The emission envelope carries `cut`.** `groundResult` gains an integer `cut` key, present at zero
   rather than omitted, populated from the emitted index by the rule `groundFloorSize` reads; on a
   partly-cut floor it is the index's cut count beside a non-zero `floor_size`. `--status --check`'s
   `cut` stays sourced from `GroundStatus` alone.
3. **The ask tests become a generated `(floorSize, carried)` table**, with a `require` that the
   generated list holds a pair satisfying `2 ≤ carried < floorSize`, and each pair asserting the
   sentence from `This round owes` through the end of its clause rather than a prefix of it.
   `groundPromptAsk`'s *"walks the whole set"* comment goes, replaced by naming the generator.
