# tp — Emitting does not lose a round

Class: tool

## 1. Overview

Two things an emission can do to a round in flight, one of them measured on this repository's own
work. Re-emitting an unrecorded round is **idempotent**, and the fear that it is not cost one field
reporter six briefs of workarounds — but nothing in the output says so, and the same call advances a
recorded round (§2). And emitting over an unrecorded round whose text has since changed **overwrites
its floor without a word**, which cost this repository one full round of grading (§3). Split out of
`ground-command-friction.md` on 2026-09-08, where these were §14 and *The silent overwrite*, with the
two test rows that belong to §2. Every claim below was run rather than reasoned about; the
measurements and the commands that derive them are in
`emitting-does-not-lose-a-round-measurements.md`, and no figure from that file is restated here. The
grounding programme and the two field reports are described in
`what-the-record-does-not-say-measurements.md` under *§1 The evidence base* and *§1.1 The second
evidence base*.

From the original file's own cost order:

10. **A fear the tree refutes** (§2): re-emitting an unrecorded round is idempotent. The ask survives
    the refutation, because nothing in the **output** says so and the same call advances a recorded
    round.

## 2. A fear the tree refutes, and the ask that survives the refutation

Report B feared that a sub-agent running bare `tp ground <spec>` to fetch its own prompt would open a
new round and orphan the one in flight. It worked around this with saved envelopes and explicit
warnings in six briefs.

**The fear is false.** Two consecutive bare emissions on an unrecorded round return the same round
number, the same floor size and a byte-identical floor file (measurements file, "§14").
**Re-emitting an unrecorded round is idempotent** — `NextGroundRound` answers *recorded rounds + 1*,
and the emission rewrites the same two files. `skills/tp/SKILL.md`'s ground loop now says so.

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

This is the second report claim the tree contradicts. The first is the escape hatch in
`spec/backlog/what-the-record-does-not-say.md` §3: it exists in the mechanism and is closed by the
prompt. Both are recorded with the measurement rather than quietly corrected, on the same rule
`spec/backlog/the-floor-names-what-it-cut.md` §3 and the original file's header follow.

## 3. The silent overwrite: emitting over an unrecorded round

**Measured on this repository's own hotfix cycle** (measurements file, "The silent overwrite"): a
round was graded, the spec repaired before the round was recorded, and `tp ground <spec>` re-emitted
round 1 against the repaired text, **overwriting the unrecorded round's floor file without a word**;
`--status` then reported a round with no dispositions — indistinguishable, from the record alone, from
a round nobody has graded yet. Recovery was possible only by accident, and the cost of the missing
guard was one full round of grading.

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

## 4. Non-Goals

1. **No new workflow field and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak or coverage — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s. This is the one part of the split where
   the original Non-Goal 5's *"and no exit code"* clause does **not** hold: §3's refusal is an
   exit-code change, taken deliberately, and §2's re-print refuses rather than emitting when there is
   no round in flight.
2. **No change to the recorded round's filename or to `--record`'s atomicity.** §3's guard sits on
   the emit path, for the reason its last paragraph gives.

## 5. Tests

Every row derives from a numbered decision and names an input that must fail it. §3 carries no row:
it arrived in the original file after §15's table was written and never got one, and the implementing
task owes the first — an emission over an unrecorded round whose floor differs is refused and names
the round, while an identical floor re-emits silently.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | two bare emissions on an unrecorded round leave the round number and the floor file's bytes identical, asserted on the pair | assert only that the second call exits 0, which is true of a call that opened a new round |
| 2 | §2 | the read-only re-print emits no round: the state directory is byte-identical before and after, and it refuses when no round is in flight | implement it as a bare emit with the write skipped, which is idempotent on an unrecorded round and advances the round on a recorded one |
