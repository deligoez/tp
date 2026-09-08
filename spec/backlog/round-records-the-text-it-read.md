# tp — The round carries the text it read

Class: loop

> **This file is decisions.** It cites code by what the search finds rather than by a line number
> wherever it can: this spec's first grounding round found one of its two line citations pointing 37
> lines above its subject after three unrelated commits, and its own re-derived corpus table stale by
> the very mechanism its header warned about. Two drafts were merged into it — the hash a round
> records, and the streak that hash resets — because the second is meaningless without the first.

## 1. Overview

A round's snapshot is written **at emission**, in both phases. `engine.WriteSnapshotAtomic` is called
from `internal/cli/review.go` under the comment *"snapshot the spec at round start (prompt
emission)"*, and from `internal/cli/audit.go` with `engine.PhaseAudit`:

```
grep -rn 'WriteSnapshotAtomic' internal/cli/ --include='*.go' | grep -v _test.go   # exactly two
```

A round's `spec_hash` is computed **at record**, by `engine.SpecHash(specPath)`, which re-reads the
file from disk. There are two such record-time call sites, one per phase — see §2 for the list and
for the third site that must be left alone.

Between those two moments the spec is edited — that is what a review round is *for*. So the round ends
up carrying two artifacts that describe different texts: a snapshot of what the roles were given, and
a hash of whatever the file said when the operator got round to recording. How often the two disagree
in this repository's own record, per phase, and the two cautions on reading that count, are in the
sidecar under "The corpus, re-derived"; the asymmetry between the phases is the argument, and it is a
numerator argument — the phase whose purpose is to change the spec certifies a text it did not read an
order of magnitude more often than the phase that changes code.

**`spec_hash` becomes the hash of the round's own snapshot**, which makes the two agree by
construction rather than by the operator's timing — wherever a snapshot exists. Where none does, §2's
fallback applies and there is no second artifact to disagree with.

**And once the hash means what it says, the audit streak can key on it.** `consecutive_clean` is **a
claim about a spec**: when the spec changes, rounds recorded against the old text no longer support
the claim. The review phase already acts on this — a `fixed` resolution forces a further round and
`tp resume` raises `spec-stale`. The audit phase checks the trailing edge only: `engine.Converged`'s
second conjunct is `!StateStale(rounds, currentHash)`, which compares the *last* recorded round's hash
to the spec on disk, so movement after the last round unconverges and movement *between* rounds is
invisible. Measured in a fixture built outside the repo — `tp init` on a two-section spec plus one
task, record an all-PASS audit round, append a section, record a second all-PASS round:
`consecutive_clean: 2`, `converged: true`, `tp audit <spec> --status --check` exit 0. Appending once
more, with no further round recorded, gives `consecutive_clean: 2`, `stale: true`,
`converged: false`. A reset keyed on a hash that was itself re-read at record time resets on the wrong
event, which is why §3 needs §2.

## 2. `spec_hash` is the snapshot's hash

At record time, `spec_hash` is `sha256` of the round's snapshot file rather than of the spec path.

**This is not a new artifact, a new field or a new write.** The snapshot is already written, already
atomic, already named per round and phase, and already read back by the regression path. The release
changes **two arguments at two record-time hash calls, in two files**:

```
grep -rn 'SpecHash(specPath)' internal/cli/review_record.go internal/cli/audit_record.go
  review_record.go:91    record, review phase   -> change
  audit_record.go:102    record, audit phase    -> change
  audit_record.go:375    --status, audit phase  -> leave: this is the CURRENT-spec side of
                                                   Converged/StateStale and must keep hashing the path
```

Two, not one, and the distinction is load-bearing rather than pedantic: §6 row 2 exists *because* a
fix applied to one phase passes rows 1 and 3, which is a hazard only if there are two sites. The
invariant this buys — *a round's `spec_hash` is the hash of the text stored beside it* — then holds
for every round recorded afterwards **that has a snapshot**, with no operator discipline; the
fallback below exempts the rest, vacuously, since there is then no snapshot to disagree with.

**A round whose snapshot is missing keeps today's behaviour and is not failed.** `spec_hash` for such
a round falls back to hashing the spec path, exactly as now. The reason is the live path — a new
`--record` with no preceding emission — rather than backward compatibility, and the measurement that
separates those two reasons, together with the measured overwrite that limits what §2 can promise
(the hash is *the last text emitted for the round*, not the text each role read), is in the sidecar
under "Why the fallback is about the live path, and the overwrite measured".

## 3. A `spec_hash` that differs between consecutive audit rounds resets the streak

**On the audit path, `consecutive_clean` and `converged` count only trailing clean rounds recorded
against one unchanged text.** When two consecutive recorded audit rounds carry different `spec_hash`
values, the streak restarts at the later one. `Converged` keeps its `StateStale` conjunct unchanged and
narrows the first conjunct only, so a converged verdict means "enough trailing clean rounds over one
unchanged text, and that text is still on disk": the two tests are not the same and neither subsumes
the other, and replacing `StateStale` would let a spec move after the last round pass, which is
behaviour tp has today and must not lose.

**The reset applies from this release forward, by a per-round vintage marker.** A round recorded by
this release carries the marker; a pair in which either round lacks it never resets. The alternative
— a retroactive reset over recorded history — flips shipped, converged cycles to `converged: false`
on install, and its cost is derived in the sidecar under "The retroactive alternative, costed on
v0.35.0". The existing legacy marker cannot stand in for the vintage: it separates v0.30.0 from
v0.29.0, not this release from its predecessors (sidecar, "The legacy marker is not a vintage byte").

The streak is derived rather than stored, so the shape is to **add** rather than change: a
vintage-aware pair beside the shared streak and convergence functions, called from the audit sinks
only — the audit record path, the budget and run-status reports, and the resume oracle's audit line,
which feeds the `release` phase and is the one that gets missed — while the review status surface
stays on the shared functions. Convergence must move together with the streak, because it folds the
streak internally and a reset that leaves `converged` answering the pre-reset value is not a reset.
The call-site count and its derivation are in the sidecar under "The convergence call sites".

## 4. The boundary against `--reconcile`

`--reconcile` (`spec/backlog/reconcile.md`) records **why** a spec moved, preserving the operator's
reason. This records only **that** it moved, and only for the streak. They compose and neither implies
the other: a reset with no explanation tells an operator to redo work without saying why; an
explanation with no reset lets a stale claim stand. Two readers — the loop, and the person.

The other two shapes considered for §2 — pinning the hash at emission into the state, and making the
emission the recorded unit — are rejected in the sidecar under "Why not the other two shapes".

## 5. Non-Goals

1. **No repair of the existing mismatches.** A round already recorded keeps its stored hash, whatever
   its snapshot says; rewriting them would fabricate a claim about what those rounds read. The sidecar's
   corpus section counts them per phase.
2. **No `spec_moved_mid_round`.** An earlier draft recorded a count of re-emissions that changed a
   round's snapshot. Its only consumer is the reconcile release, so the design moved to the sidecar
   under "`spec_moved_mid_round`, handed to reconcile" and this release produces no such signal.
3. **No *edit* to `StateStale`.** It stays `rounds[len(rounds)-1].SpecHash != currentHash`, and every
   caller already passes a freshly computed `SpecHash(specPath)` as `currentHash`
   (`grep -rn 'StateStale' internal/ --include='*.go' | grep -v _test.go`), so no call site changes.
   **Its answers do change, and that is the correction rather than a side effect**: a round emitted,
   then edited, then recorded reports `stale: false` today — measured in a sandbox, because under
   shipped behaviour both sides hash the edited file — and will report `stale: true` afterwards. The
   Non-Goal is "no edit", not "no consequence".
4. **The snapshot-less audit rounds are not backfilled.** They predate snapshotting on that phase
   (`git log --reverse -S WriteSnapshotAtomic -- internal/cli/audit.go`); no snapshot exists to hash,
   and synthesising one from today's spec would assert exactly the falsehood this release removes.
5. **No change to the review phase, and the surface is `tp review --status`.** Its streak and
   convergence calls stay on the shared functions; review already acts on spec movement by its own
   route. Naming the surface is load-bearing, because the review phase reports `consecutive_clean`
   through two different functions — `tp review --status` uses the severity-blind shared one, while
   the record path uses the severity-aware review-specific one over the same rounds — so the two can
   already disagree. This release freezes that inconsistency knowingly: it is a severity question, not
   a hash question, and belongs to whichever release takes review-side convergence next.
6. **No reset of `spec_coverage_clean_rounds` or `role_streaks`.** They are derived from the same
   rounds and the same argument applies, but each is a separate claim with its own consumers, and
   widening the blast radius of a reset is how a correct change becomes an incident.
7. **No new workflow field.** The reset is not configurable; a spec that moved, moved.
8. **No `--reconcile` interaction.** A reconciliation explains a movement; it does not exempt it.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | emit, edit the spec, record — the recorded `spec_hash` equals sha256 of the round's snapshot and **not** of the edited file | hash the spec path, which is the shipped behaviour and produces the corpus mismatches |
| 2 | §2 | over every round in a fixture repo, `spec_hash` equals its snapshot's sha256 — asserted as an invariant over the set, not on one round | hash the path on the audit phase only, which a single-round review test cannot see |
| 3 | §2 *fallback* | a round whose snapshot is absent records the spec path's hash and does not error | error on a missing snapshot, which breaks a **new** `--record` that had no preceding emission. Measured live: delete `snapshot-round-N.md`, run `tp review <spec> --record <file>` — exit 0 today, spec path's hash stored. It does **not** break reading old rounds; the sidecar says why |
| 4 | §5.1 | a round recorded before this release keeps its stored `spec_hash` byte for byte after upgrade | recompute historical hashes on read, rewriting what past rounds are understood to have covered |
| 5 | §3 | two clean audit rounds either side of a spec change leave `consecutive_clean` at 1, not 2 | the shipped behaviour, which reports 2 |
| 6 | §3 *converged* | `converged` is false in the same fixture, not merely `consecutive_clean` reduced | move the streak alone, leaving `converged` answering the pre-reset value |
| 7 | §3 *resume* | `tp resume` reports the audit phase, **not** `phase: release`, in that fixture | move the `internal/cli` sites and leave `internal/engine/resume.go`'s audit line on the shared function. That file carries two convergence calls, review and audit, and only the audit one reaches `DetectPhase`, whose `numTasks > 0 && numDone == numTasks` branch returns `PhaseRelease` on `auditConverged` alone — so the mutant is reachable and the release branch is the only thing that discriminates it |
| 8 | §5.5 | **`tp review --status`** — the surface §5.5 names — reports byte-identical `consecutive_clean` and `converged` before and after. The `--record` path is out of this row's scope because it runs the severity-aware review function, which this release does not touch | move the review status surface too, resetting a phase that already handles movement its own way |
| 9 | §3 *vintage* | a pre-release round is unaffected: a fixture whose two clean rounds either side of a spec change both lack the marker reports `consecutive_clean: 2` | drop the marker check, so the release ships (a)'s schema with (b)'s behaviour |
| 10 | §3 *no change* | with `audit_clean_rounds` **pinned at the default 2**, an unchanged spec across five rounds leaves the streak at 5 | reset on every round. **The pin is the row, not decoration:** the field's legal range is 1–10 (enforced in `internal/engine/projectconfig.go` and warned in `internal/engine/validate.go`), and at 1 this mutant survives its own test — measured, one clean round at `audit_clean_rounds=1` gives `consecutive_clean: 1`, `converged: true`, `tp audit --status --check` exit 0, so a per-round reset leaves convergence reachable |

**Row 2 is the one that must be stated over the set.** Rows 1 and 3 are single cases and a fix applied
to one phase passes both — and the audit path is not merely a second phase, it is a second *file*
(§2), so a reviewer reading only `review_record.go` sees a complete-looking change. Only an assertion
quantified over every recorded round in the fixture fails when the audit path is left behind, and this
repository has lost rounds to exactly that shape: a claim about a set checked against one member the
author chose. **That quantifier is vacuous on the audit side unless the fixture holds an audit round**
— an all-review fixture satisfies "every round" while proving nothing about `audit_record.go` — so the
fixture's audit-round count is *asserted* rather than chosen, the same rule that turned a `zz`
directory name into a `require.Less` on the sort order.
