# tp — Emitting does not lose a round

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `emitting-does-not-lose-a-round-measurements.md` beside it, and this file stands
without them. It absorbs `scratch-name-is-unique-per-spec.md` whole and `reconcile.md` §2.1 as it
stood before 2026-09-11.

Class: **tool** — it changes what an emission refuses and which file it names, and no convergence
signal.

## 1. The decision

**Context.** Two things an emission can do to work in flight, both silently, both at exit 0.

- **It names a scratch file two specs share.** The file an emission tells a unit to write carries the
  round and, on review and audit, the role — never the spec. Two units working on two specs at once
  are told the same filename, and the second overwrites the first at exit 0 (sidecar of
  `scratch-name-is-unique-per-spec`).
- **It replaces an unrecorded round.** Re-emitting a round nobody has recorded yet rewrites that
  round's snapshot — and on `tp ground`, its floor — from the spec as it stands now. When the text is
  unchanged that is the idempotent re-emission the loop relies on. When the spec was repaired between
  the emission and the record, the round's graded work no longer matches anything on disk, and nothing
  says so. This repository lost a full round of grading to it on `tp ground`; the same overwrite
  reproduces on `tp review` and `tp audit` (sidecar, *The silent overwrite* and *Re-verified
  2026-09-11*).

**Decision.** The scratch file an emission names carries the spec's base (§2). An emission over an
unrecorded round whose text has changed refuses, on all three phases (§3).

**Consequences.** Emission gains an exit code it did not have (§3), and an operator who repaired a
spec mid-round records the round or discards it explicitly rather than having it discarded for them.
`reconcile.md` §2.1's `spec_moved_mid_round` counter becomes unnecessary (§3.1). The documentation
owes an update wherever it names a scratch file or describes re-emission: `skills/tp/SKILL.md`'s
ground loop and its role-output naming, its *order inside a round* (which says a new emission
overwrites an unrecorded round silently), and `skills/tp/REFERENCE.md`'s re-emission and role-output
passages. The implementing task finds every passage by searching both files for the scratch-name
patterns and for "overwrite", rather than trusting a list.

**Alternatives.** A guard at `--record` instead of at emission — rejected: on `tp ground`, `--record`
matches rows against the floor file on disk rather than against the spec's current text, which is what
made recording a rescued round possible at all, so by the time a record runs the overwritten floor is
already gone. A read-only way to re-print a round's prompt — dropped (Non-Goal 3).

## 2. The scratch name carries the spec

**The scratch file an emission names carries the base the state directory already uses**
(`spec/.tp-review/<base>/`): `ground-<base>-r<N>.ndjson` on ground, and on review and audit outside a
run `<phase>-<base>-r<N>-<role>.ndjson`. Nothing new is derived, and no two specs collide. A default
every caller must override is the tell: the grounding programme that measured this carried a
hand-written override in every brief (sidecar of `scratch-name-is-unique-per-spec`).

**Two names do not change.** The **recorded** round files — `ground-round-<N>.ndjson` and
`<phase>-round-<N>.ndjson` — are the files `--record` writes beside the snapshot, a deliberate and
different name, and stay. **Under `tp run`** a role's path is `$TP_ROUND_DIR/role-<role>.ndjson.part`,
the one path the run's write fence allows; it does not collide and stays.

## 3. An emission does not replace an unrecorded round whose text changed

**When a round has been emitted and not recorded, and the snapshot the current spec would produce —
on `tp ground`, the floor as well — differs from the one on disk, the emission refuses.** It exits 3,
tp's file-or-state code, writes nothing, names the round, says it is unrecorded, and names the two
ways on: record the round, or re-emit with `--force` to discard it. `--force` is a new flag on
`tp ground` and a new meaning, at emission, for the flag `tp review` and `tp audit` already carry for
`--resolve`.

**An identical re-emission stays silent.** Same text, same round number, same bytes, exit 0 — the
re-emission the loop and its sub-agents rely on is unchanged. A recorded round is not in flight, so
the next emission opens round N+1 whatever the text says.

**It holds on all three phases** because the overwrite does. Review and audit already report an
unrecorded round as `in_flight_round`; ground has no such field and records by filename, which is why
nothing noticed there first. The refusal reads the same on-disk state each phase already uses to find
the round in flight.

### 3.1 `spec_moved_mid_round` is no longer needed

`reconcile.md` §2.1 proposed counting re-emissions whose bytes differed, on the round's entry. Once
this refusal ships, such a re-emission cannot happen silently: a refused one writes nothing, and a
forced one discards the round, so the round later recorded read exactly the text its snapshot holds.
The counter would measure nothing and is dropped. A spec that changes between emission and record
**without** a re-emission is a different gap — the recorded hash is read at record, not at emit — and
it is `reconcile`'s.

## 4. Non-Goals

1. **No new workflow field and no convergence effect.** Nothing here adds a knob, reads one, or
   changes `clean`, a streak or coverage. §3's refusal is the one exit-code change, taken
   deliberately.
2. **No change to `--record`.** The guard sits on the emit path, for the reason §1 *Alternatives*
   gives; the recorded filenames stay (§2).
3. **No read-only re-print of a round's prompt.** Proposed from one field report whose fear — that a
   bare re-emission opens a new round — the tree refuted; `skills/tp/SKILL.md`'s ground loop already
   states that re-emitting an unrecorded round costs nothing, and one report does not carry a new
   mode.
4. **No repair of a round already overwritten.** Its floor or snapshot is gone; this stops the next
   one.

## 5. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Every row is
written, not yet watched; their subjects do not exist at `HEAD`, so each row's two counts belong to the
implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | two specs emitted at the same round produce **different** scratch paths — ground's `output_path`, and each review and audit role's path outside a run — asserted on the pair rather than on either alone | keep the round-only name, under which the two are equal |
| 2 | §2 *base* | the emitted path contains the base the state directory uses, asserted by deriving the base rather than by matching a literal | hardcode a base in the test, which passes for the fixture spec and no other |
| 3 | §2 *unchanged* | the recorded files are still `ground-round-<N>.ndjson` and `<phase>-round-<N>.ndjson`, and with `TP_ROUND_DIR` set a role's path is still `$TP_ROUND_DIR/role-<role>.ndjson.part` | rename the recorded file to the scratch name, or apply the base to the run path the write fence allows |
| 4 | §3 | on `tp ground`: emit round 1, edit the spec, emit again — the second emission exits 3, names round 1 as unrecorded, and the snapshot and floor are byte-identical to the first emission's | `HEAD`, which rewrites both at exit 0 |
| 5 | §3 *phases* | the same sequence on `tp review` and on `tp audit`, each asserted on its own snapshot | guard `tp ground` alone, leaving the two phases that reproduce the overwrite unguarded |
| 6 | §3 *identical* | two emissions with the text unchanged return the same round number and byte-identical snapshot and floor, exit 0, nothing on stderr — asserted on the pair | refuse whenever a round is in flight, which breaks the idempotent re-emission the loop relies on |
| 7 | §3 *recorded* | after round 1 is recorded, editing the spec and emitting opens round 2 at exit 0 | compare against the last recorded round's snapshot, which refuses every emission after a repair |
| 8 | §3 *force* | `--force` over an unrecorded round whose text changed replaces its snapshot and floor and exits 0 | accept `--force` and still refuse, leaving no way on but deleting state by hand |
