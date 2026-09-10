# tp — What the record does not say

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `what-the-record-does-not-say-measurements.md` beside it, and this file stands
without them. That sidecar also holds the shared provenance of the 2026-09-08 split of
`ground-command-friction.md` (*§1 The evidence base*, *§1.1 The second evidence base*), which every
sibling split cites. On 2026-09-11 it absorbed `what-the-carry-can-promise.md`'s multiplicity fence,
because the fence and §3 rewrite the same sentence of the emitted ask; that file is now a forwarding
stub.

Class: **tool** — §2's fence changes what `tp ground` carries, but it cannot fire on a floor with no
duplicated unit, which every recorded floor is (sidecar, *The carry's exposure, re-derived*), so the
ground loop that grades this spec runs exactly as it does today.

## 1. The decision

**Context.** Three things a ground round does and does not report. A disposition carried by
`(text_sha, ordinal)` can move onto a different copy of a repeated sentence when one copy is deleted,
and the round reports itself complete with the `FAIL` gone (§2). A carried disposition can be
overridden by writing a row for its unit, the prompt forbids exactly that, and `--record` reports an
override as `carried: 0` with nothing saying an inherited disposition was displaced (§3). And the
coverage ratio's denominator counts units that assert nothing, with no key a reader can correct it by
(§4). A fourth item, a verdict the operator-facing skill once did not name, is closed and needs only a
guard (§5).

**Decision.** The carry gains a multiplicity fence (§2). The emitted ask states the fenced promise
and permits the override it currently forbids, and `--record` counts the override (§3). `--status`
reports a claims-only denominator (§4). A test derives the verdict list `SKILL.md` must name from the
code (§5).

**Consequences.** §2 and §3 change one sentence of the ground prompt between them, so they ship
together or the second overwrites the first. No recorded round is rewritten, no gate changes, and no
workflow field is added. Once §3 ships, `SKILL.md` may document the override; before it, a skill
saying *write the row* beside a prompt saying *write no row* would turn an undocumented gap into a
documented contradiction.

**Alternatives.** Re-keying the carry on `unit_id` or `anchor`, correcting the promise without the
fence, and refusing a duplicated unit at `--record` were each considered for §2 and rejected; the
argument is in `what-the-carry-can-promise-measurements.md` under *§2.3 — the roads not taken*.

## 2. The carry is fenced by multiplicity

`ordinal` is a unit's 1-based index among the units sharing its `text_sha`, in emission order, so it
is positional: deleting one of two byte-identical sentences renumbers the survivor. On a spec holding
one sentence twice, graded `PASS` then `FAIL`, deleting the **first** copy — an editorial act that
touches nothing about the failing sentence — lets the survivor join the deleted copy's `PASS`. The
round records complete, `by_verdict.FAIL` falls to 0, and `--status --check` exits 0. Inserting a copy
instead is loud: the round owes a disposition and says so. The grounding spec's §8 says the carry
*"makes an unrepaired `FAIL` permanent while its text stands"*; in the silent direction it does not.

**Decision.** A unit inherits a disposition only when the number of units carrying its `text_sha` is
the same in round N as in round N−1, counted over distinct join keys in round N−1 and never over rows.
When that count moves, no unit under that hash carries, and each is re-asked. The join key stays
`(text_sha, ordinal)`. Wherever tp states the carry's promise — the emitted ask and the grounding
spec's §8 — it states the fenced one: *a disposition is carried while its unit's text stands and the
number of units carrying that text has not changed.*

The exposure today is zero — no recorded floor holds a duplicated `text_sha` (sidecar, *The carry's
exposure, re-derived*) — which is the reason to repair the mechanism before its record can be harmed,
and the reason the fence costs nothing on any floor recorded so far.

## 3. A carried disposition can be overridden, and the override is counted

A `PARTIAL` on unit X whose cause lives in section Y: repair Y, X's own text is unchanged, and X
carries its stale `PARTIAL` into every later round. The ask then tells the reader *"do not decide those
units again, and write no row for them."* A row naming the carried unit does override the carry and
records at exit 0 — the documented behaviour is that a unit the round decides is not also carried — so
the mechanism has the exit and the protocol closes it. A reader following the prompt cannot reach it; a
reader ignoring the prompt reaches it silently. The field reached it that second way once, on
`spec/1.1.0.md`'s ground round 3, and a field report (WB-3155) met the same shape in `PARTIAL`s that
were true when written and went stale while their text stood (sidecar).

**Decision, in two parts.** The ask **names the override**: do not re-decide a carried unit to repeat
its verdict; when the ground beneath it moved, write a row for it and say why in `note`, and that row
replaces the carried one. Together with §2 the sentence becomes one statement — the fenced promise,
then the condition under which a carried unit is re-decided — emitted in every round in which at least
one unit carries, stated on the carried count rather than on the round number. And `--record`'s
envelope **reports the displacement**: beside `rows` and `carried`, a count of carried dispositions the
payload overrode, so an override is never silent at the sink.

**What this does not do.** It does not invalidate a carry when another unit changes — tp cannot know
that Y is why X was `PARTIAL`; only the reader's `note` holds that. It makes the override sayable and
visible; deciding when it is right stays the reader's.

## 4. The coverage denominator counts claims

A ratio over a floor, a share of which asserts nothing, is not the number it looks like, and the share
varies too widely document by document to correct for mentally. Nor can the payload be corrected by
hand: `emitted` and `dispositioned` count units while `by_verdict` counts rows, so
`emitted − by_verdict["NOT-A-CLAIM"]` subtracts a row count from a unit count and is wrong by exactly
the reader-added and off-floor rows.

**Decision.** `--status` reports the count of emitted floor units disposed `NOT-A-CLAIM`, beside
`emitted` and `dispositioned`, under a name that cannot be confused with `by_verdict`'s row count. The
raw ratio stays as it is; this adds the second reading and replaces nothing.

## 5. `SKILL.md` names every verdict, and a guard keeps it so

`SKILL.md` at `v1.0.0` named five of the six verdicts; a commit closed it. **Decision:** a test
asserts that every verdict the code exports appears in `SKILL.md`, derived from the exported set rather
than from a literal list. The open question inherited as *"What `UNVERIFIABLE` costs"* is closed by
measurement (sidecar, §7).

## 6. Non-Goals

1. **No change to `--record`'s atomicity or to the row table.** §2 changes which units carry and §3
   what the prompt permits and what the envelope counts; `--record` still accepts more than one row
   for one unit, and the carry still resolves the repeat first-wins.
2. **No gate, no convergence effect, no workflow field.** Nothing here changes `clean`, a streak or an
   exit code; the fence is not configurable. The `--check` gate is
   `next-action-and-check-tell-the-truth`'s.
3. **No recorded round is re-graded, re-hashed or rewritten.** The fence reads round N−1 and decides
   what round N carries.

## 7. Tests

Every row derives from a numbered decision and names the mutant that must fail it. The fence, the
sentence and the envelope count do not exist at `HEAD`, so where a row quotes a count it is the `HEAD`
observable the fix changes; the count under the finished code is the implementing task's acceptance,
per Step 0.5. The characterisation row the old §2.1 carried is deleted, because the fence it
characterised ships here.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *deletion* | two byte-identical floor units graded `#1 PASS` / `#2 FAIL`; the first deleted: the survivor is owed and `--status --check` exits 1. `HEAD`: `carried` 2 of 2, exit 0 | the shipped carry |
| 2 | §2 *insertion* | a copy inserted above a `FAIL`ed unit leaves both copies owed and no row carries the `FAIL` | fence only the shrinking direction, which lets the insertion migrate the `FAIL` onto text nobody graded and leaves row 1 green |
| 3 | §2 *keys, not rows* | round N−1 holding two rows under one `(text_sha, ordinal)`, text unchanged: the unit still carries, and carries the first row | count round N−1's rows instead of its distinct keys, which reads a repeated row as a second unit |
| 4 | §2 *inert and local* | five identical units among others, unedited across two rounds, all carry; delete one of the five and exactly the four survivors are owed while every other unit carries | fence on the presence of a duplicate, which never carries a repeated unit; or fence per round, which drops the whole carry when any count moves |
| 5 | §2 *no rewrite* | after round N records under the fence, round N−1's file is byte-identical | have the carry normalise round N−1 in place so the counts agree |
| 6 | §2, §3 *the sentence* | in a round where at least one unit carries, the ask states the count condition, permits the override with a `note`, and contains neither the unfenced promise nor *"write no row"*. `HEAD`: both present | correct the grounding spec and leave the emitted sentence — no shipped test asserts it |
| 7 | §3 *the count* | a row naming a carried unit displaces the inherited disposition and `--record` reports a displacement count of 1. `HEAD`: `rows: 2, carried: 0`, no such key | report only `rows` and `carried` |
| 8 | §3 *the stale carry* | after a repair to the section that caused a carried `PARTIAL`, the unit is still carried and still not in the ask | assert that the repair clears the carried unit — a test-side mutant, and the row that keeps §3 from being read as more than it is |
| 9 | §4 | the claims-only count equals the emitted floor units disposed `NOT-A-CLAIM`, on a round also carrying a reader-added and an off-floor `NOT-A-CLAIM` row. On that fixture the subtraction gives 0 where the answer is 2 | derive it as `emitted − by_verdict["NOT-A-CLAIM"]` |
| 10 | §5 | every verdict the code exports appears in `SKILL.md`, the list derived from the exported set | append a seventh verdict to the exported set and leave `SKILL.md` alone: the derived guard fails naming it, a literal-list guard passes |
