# tp — What the record does not say

Class: tool

## 1. Overview

Four things a ground round does and does not report. A `FAIL` cleared by an edit to an unrelated
section, with the record showing a clean round (§2); a carried disposition displaced by a payload
row, with the envelope reporting `carried: 0` and nothing saying an inherited disposition was
overridden (§3); a coverage ratio whose denominator counts non-claims, with no key a reader can
correct it by (§4); and a verdict the code exports that the document the operator drives the loop from
never named (§5). Split out of `ground-command-friction.md` on 2026-09-08, where these were §9/§9.3,
§11, §12 and §13, with the non-goals, the open question and the five test rows that belong to them.
Three of the four came from two independent field reports; §3's report claim is right in effect and
wrong in mechanism, which is the whole of §3's decision. Every claim below about tp's own behaviour
was re-derived on a freshly built binary; the measurements and the commands that derive them are in
`what-the-record-does-not-say-measurements.md`, and no figure from that file is restated here. That
file is also where the split's shared provenance lives — *§1 The evidence base*, the grounding
programme this repository ran, and *§1.1 The second evidence base*, the two field reports — and every
sibling split cites it rather than restating it.

From the original file's own cost order:

7. **A carried disposition has no escape hatch the protocol permits** (§3) — the hatch exists, the
   prompt closes it, and the sink does not report an override.
8. **The coverage denominator counts non-claims** (§4), and the payload cannot be corrected by
   subtraction because it mixes a unit count with a row count.
9. **`SKILL.md` did not name `UNVERIFIABLE`** (§5) — five verdicts of six reached the document the
   operator drives the loop from. **Closed by commit** while §9–§14 were being written; §5 routes
   the durable form to a derived guard.

## 2. `--check` exits 0 with `FAIL`s standing

The original §9.1 (the gate: `--status --check` exits 1 when the latest recorded round holds a
`FAIL`) and §9.2 (`--status` carries `next_action`) are
`spec/backlog/next-action-and-check-tell-the-truth.md`'s §2 and §3, with the former rows 10–13; the
measurement that motivated them is the measurements file's "§9" and "§9.1". What stays here is the
limitation, because the record is what it is a limitation of.

### 2.1 The limitation the gate ships with, and why it ships anyway

The gate has a hole that runs the other way from a deadlock: on a spec whose two sections hold a
byte-identical sentence, editing the section that does *not* carry the `FAIL` shifts the
`(text_sha, ordinal)` join onto the `PASS` row, and the `FAIL` is cleared with no repair at all
(constructed and run in the measurements file's "§9.3", with the two exposure counts over this
repository). `spec/backlog/what-the-carry-can-promise.md` §2.2 takes that defect.

**The decision: the gate ships before that fence, with this limitation stated and guarded by §8
row 1, and the test implementing row 1 carries the characterisation in its own name and doc
comment, the doc comment naming the mutant that must retire it** — the multiplicity fence. A green
test asserting that a `FAIL` is cleared by editing an unrelated section, with nothing at the test
saying the green is deliberate, is the shape most likely to be tidied away by someone who reads it as
a bug in the test.

## 3. A carried disposition has no escape hatch the protocol permits

Report B. A `PARTIAL` on unit X whose *cause* lives in section Y: fix Y, X's own text is unchanged, so
X carries its stale `PARTIAL` forward indefinitely. Their concrete case is a test-case unit flagged
because §4's window bound said `23:59` while the test said `23:59:59`; they fixed §4 and the unit
still carried the old `PARTIAL`. Constructed and confirmed in the reporter's own shape on a fixture
outside this repository (measurements file, "§11"): the repaired unit is asked about, the unit the
repair was for carries its stale `PARTIAL`, and the prompt tells the reader *"do not decide those
units again, and write no row for them."*

**The report's claim is right in effect and wrong in mechanism, and the difference is the whole
decision.** A hatch exists: a row naming a carried unit **overrides the carry** and records at exit 0 —
`groundCarryForward` takes the round's own payload as `decided` and does not carry what the round
decides, which is the documented behaviour: *"A unit it decides is not also carried."* So the
mechanism has the hatch and **the protocol closes it**. Two sentences do: the ask states the unit is
not owed, and the prompt says to write no row for it. A reader following the prompt cannot reach the
override; a reader who ignores the prompt gets it silently, and the `--record` envelope reports
`carried: 0` with nothing saying an inherited disposition was displaced. The field produced exactly
that route once more, on `spec/1.1.0.md`'s round 3 (measurements file, "§11").

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
repair can remove the sentence it repairs from every future floor — are the measurements file's
"§11.1"; the second is an entry in `spec/undecided.md`.

## 4. The coverage ratio's denominator counts non-claims, and the payload cannot be corrected by subtraction

Both reports want a claims-only denominator beside the raw one, on the ground that a ratio over a
floor a sizeable share of which asserts nothing is not the number it looks like. Derived over this
repository's own corpus (measurements file, "§12"), the `NOT-A-CLAIM` share of a round-1 floor varies
by an order of magnitude document by document — not a constant a reader can mentally correct for,
which is why it has to be reported.

**And the payload cannot be corrected by hand, which is the measured half.** `emitted` and
`dispositioned` count **units**; `by_verdict` counts **rows** — `GroundStatus`'s own doc says so
(*"the breakdown's total is the round's row count and need not equal `Dispositioned`"*) — so
`emitted − by_verdict["NOT-A-CLAIM"]` subtracts a row count from a unit count and is wrong by exactly
the reader-added and off-floor rows, which move neither side of the ratio. Nothing in the payload
labels it as unavailable — the two counts sit adjacent and read as commensurable.

**The decision: `--status` reports the claims-only denominator itself** — the count of **emitted floor
units** whose disposition is `NOT-A-CLAIM` — beside `emitted` and `dispositioned`, under a name that
cannot be confused with `by_verdict`'s row count. The raw ratio stays exactly as it is: the ground
spec's §8 *did anyone look* is a question about the floor, and narrowing its denominator would change
what coverage means. This adds the second reading; it replaces nothing.

## 5. `SKILL.md` did not name `UNVERIFIABLE` — closed by commit, and here is what keeps it closed

**Recorded as closed rather than dropped**, on the rule the marker-unit and idempotence findings
follow: an entry deleted once someone fixes it leaves nothing that would notice the fix being undone.
At `v1.0.0`, the state both reports read, `skills/tp/SKILL.md` named five of the six verdicts; the
commit *"name all six verdicts where the loop is driven, `UNVERIFIABLE` included"* closed it
(measurements file, "§13").

**The decision is therefore not the paragraph but the guard**: §8 row 5 asserts that every verdict
in the set the code exports appears in `SKILL.md`, derived from `GroundVerdicts()` rather than from a
literal list. The guard is one-directional — it does not fail when a verdict is deleted from
`groundVerdictOrder`, because the assertion quantifies over the code's set — and closing that third
direction needs a second, opposite assertion this release does not take.

## 6. Non-Goals

1. **No change to `--record`'s atomicity or to §7.2's table.** The corpus exercised both and they
   held; §3 leaves the `(text_sha, ordinal)` join and the inheritance untouched and changes what the
   prompt permits and what `--record`'s envelope reports about an override.
2. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.

## 7. Open questions inherited when `spec/candidates.md` was split

The item that arrived here, *"What `UNVERIFIABLE` costs"*, is closed by measurement rather than by a
decision (measurements file, "§7").

## 8. Tests

Every row derives from a numbered decision and names an input that must fail it. Where a row's mutant
is a change to the test rather than to the product, the row says so; that is row 3. Row 1 is the one
**characterisation** row: it asserts a defect this release does not close, and its named mutant is
the fix that closes it. The mutant-column reasoning trimmed from rows 1 and 5 is in the measurements
file's "§15".

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *the limitation* | on a fixture whose §1 and §2 hold **byte-identical** sentences, a round-1 `FAIL` recorded against the §2 copy is cleared at exit 0 by editing §1 alone and grading nothing — asserted as the shipped behaviour this release does **not** close (§2.1), so the row states the hole rather than denying it | land `spec/backlog/what-the-carry-can-promise.md` §2.2's multiplicity fence, under which the unit is re-asked, the `FAIL` survives, and this row goes red — its purpose |
| 2 | §3 | a row naming a carried unit displaces the inherited disposition, and `--record`'s envelope reports the displacement count as 1 | report only `rows` and `carried`, under which the override records silently — the shipped behaviour |
| 3 | §3 *the stale carry* | on the §3 fixture, after §1's repair, `u2` is still marked `(carried)` and is still not in the ask | assert that repairing §1 clears `u2`'s disposition, which no join in tp can do and which this section explicitly does not decide — a test-side mutant, and the row that keeps §3 from being read as more than it is |
| 4 | §4 | the claims-only count equals the number of **emitted floor units** disposed `NOT-A-CLAIM`, on a round carrying a reader-added and an off-floor `NOT-A-CLAIM` row as well | derive it as `emitted - by_verdict["NOT-A-CLAIM"]`, which returns 0 where the answer is 2 on exactly that fixture |
| 5 | §5 | all six of the ground spec's §3 verdicts appear in `SKILL.md`, derived from the verdict set the code exports rather than from a literal list | append a seventh verdict to `groundVerdictOrder` and leave `SKILL.md` alone — the derived guard fails naming it, the literal-list version passes |
