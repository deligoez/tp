# tp — A round can be driven from the envelope

Class: tool

## 1. Overview

**A round cannot be driven from the envelope**: the ask set is a count and a prose marker (§2.1), a
growing floor cannot say what grew (§2.2), and a large floor has no **machine-readable** sharding path
while the sentence that seems to forbid one is `skills/tp/SKILL.md`'s and means something else (§2.3).
Sharding *as such* already works — a payload holding a strict subset records at exit 0, the shortfall
is reported and `--check` holds; what is missing is `asked`, so a driver must rebuild the ask set by
joining the floor against the previous round file. This was `ground-command-friction.md` §10, split
out of that file on 2026-09-08 with the non-goals and the four test rows that belong to it; it is the
one of that file's ten defects reported by both independent field reports and by neither of the two
this repository measured on its own. Every claim below about tp's own behaviour was re-derived on a
freshly built binary; the measurements and the commands that derive them are in
`a-round-can-be-driven-from-the-envelope-measurements.md`, and no figure from that file is restated
here. The two field reports are described in `what-the-record-does-not-say-measurements.md` under *§1.1
The second evidence base*; their own figures are quoted there as the reports' numbers, never
re-derived, and no decision here turns on one.

## 2. Driving a round: three things the emission cannot hand a driver

Three findings that look separate in the reports and are one subject: what a process other than the
reader itself can do with a round.

### 2.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*, and all three of its claims hold (measurements
file, "§10.1"): `carried` is a count, the `(carried)` marking is prose inside the `prompt` string, and
the floor file on disk is unmarked — deliberately, for the reason `runGround`'s own comment gives: *"a
copy in the floor file would be a second statement of the same fact with nothing comparing the two"*.
So a driver that wants the ask set reconstructs it, and the reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**. That is the same P2 tell `record-diagnoses-every-bad-row.md` records,
in a different surface.

**The decision: the emission's envelope carries `asked`, the list of `unit_id`s this round owes.** A
list beside the count, not instead of it — `carried` and `floor_size` are what an operator branches
on and stay exactly as they are. It is added to the **envelope**, never to the floor file, for the
reason quoted above. And it is the shipped fact rather than a new one: `asked` is
`floor_size − carried` ids, the same set the prompt's ask sentence already names in English.

### 2.2 Floor growth conflates two different things

A unit is in the ask set either because its text changed or because it never existed, and nothing
in the envelope, the prompt, the floor file or `--status` separates the re-ask from the first ask
(measurements file, "§10.2"). New-versus-edited is **not recoverable from hashes at all** — identity
*is* the hash, so an edited sentence and a deleted-plus-added pair are the same event to every
artifact tp writes, and no decision here pretends otherwise. What *is* exact and cheap is the
comparison the growth question wants: **the previous round's floor as a set of hashes against this
one's.**

**The decision: the envelope reports `floor_delta` against the preceding round's floor —
`{added, removed, unchanged}`, counts over `text_sha`.** That separates net new sentences from
whatever churn replaced existing ones. Stated with its limit, because the limit is the reason the
other cut was refused: **a rewritten sentence is one `added` and one `removed`, indistinguishable
from an unrelated insertion and an unrelated deletion.** The counts bound the edit churn; they do not
identify it.

### 2.3 A large floor has no sharding path, and the instruction that seems to forbid one is not the emission's

Both reports sharded a large floor across several readers and read *"Spawn **one** sub-agent on that
prompt"* as a capacity claim. **The emitted prompt does not say it** — it says nothing about how many
readers it has (measurements file, "§10.3"). The sentence is `skills/tp/SKILL.md`'s ground-loop step,
and the reason the same section gives for the singular is in the step above it: *"grounding asks one
question of every unit, so there is no panel and no role"*. That is a claim about the panel, not about
capacity. **Slicing already works, measured and not inferred**: a payload holding a strict subset of
the owed units records at exit 0, `--status` reports the shortfall, and `--status --check` exits 1
until the slices are all in. NDJSON concatenates, and grounding has no `--merge` to need.

**The decision has two halves and neither is a code change to the emission.** SKILL.md's step says
what it means — **one panel, any number of readers**, because the floor is a partition and a slice of
it is a well-formed ask, with the recorded evidence above that a slice records and gates correctly.
And the emitted prompt **stays silent** on the question, deliberately: it is addressed to whoever is
reading the units, and a sentence about process there would be the emission telling an orchestrator
how to spend its context, which is not something tp knows. `asked` (§2.1) is what makes a slice
expressible without a join, which is why §2 is one section and not three.

## 3. Non-Goals

1. **No change to the floor's *arms*.** The arms are §2.2's cut step of the ground spec —
   `internal/cli/ground.go` prints *"§2.1 produced %d units and the arms cut every one"* and
   `groundcarry.go` has *"The absence of the hash is the cut (§2.2)"* — and nothing here touches it:
   §2.3 splits the reading of a floor and never the floor.
2. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.
3. **No `--merge` for ground, and no orchestration.** §2.3 establishes that a slice records and
   gates correctly and that NDJSON concatenates; it adds no command to do the concatenating and
   nothing that spawns, schedules or counts readers.

## 4. Tests

Every row derives from a numbered decision and names an input that must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 | `asked` holds exactly the ids the prompt's ask sentence counts: `len(asked) == floor_size - carried`, on a fixture with at least one carried and one asked unit | emit `asked` as every floor unit, which is right in round 1 and wrong from round 2 on |
| 2 | §2.1 *unchanged* | the floor file on disk still carries **zero** `(carried)` markers after §2.1 ships | write the marks into the floor, which is the decision `runGround`'s own comment refuses |
| 3 | §2.2 | on a round where one unit was edited and one written fresh, `floor_delta` reports `added: 2, removed: 1` against the preceding floor | compute the delta against the current spec rather than the preceding round's floor, which reports `added: 0` for a spec nobody edited since the emission |
| 4 | §2.3 | a payload holding a strict subset of the owed units records at exit 0, `--status` reports the shortfall, and `--check` exits 1 | refuse a partial payload, which forbids the split both reports had to perform |
