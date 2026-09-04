# tp v1.53.0 — The spec-hash reset

> **This file is decisions.** It opens with the one decision that must be taken before implementation
> starts, because five review rounds in an earlier cycle established that a review round cannot take
> it. Everything after §2 is settled.

## 1. Overview

`consecutive_clean` is **a claim about a spec**. When the spec changes, rounds recorded against the
old text no longer support the claim.

**The review phase already acts on this** — a `fixed` resolution forces a further round and
`tp resume` raises `spec-stale`. **The audit phase does not.** An audit streak survives any amount of
spec movement.

**It needs the emit-time hash.** A reset keyed on a hash that was itself re-read at record time
resets on the wrong event; the release that pins a round's hash to its own snapshot is what makes the
comparison mean what it says.

## 2. The decision, which must be taken first

Either:

**(a) Store a per-round vintage byte**, so the reset applies only from this release forward. Cost: a
schema addition.

**(b) Accept a retroactive reset** and re-measure every claim resting on recorded history. Cost,
measured on v0.35.0's audit — **9 rounds carrying 7 distinct `spec_hash` values** (r1=r2, r7=r8):

| | before | after a retroactive reset |
|---|---|---|
| `blocking` converges at | round 3 | round 8 |
| `all` converges at | round 9 | **never** |

So a shipped, converged cycle reports `converged: false` on install, and *"six rounds of a nine-round
phase saved"* becomes one. **Whoever takes (b) owns re-deriving those figures.**

**There is no third option, and the existing marker does not supply one.** `IsLegacyRound` is
`r.IDScheme == ""` (`internal/engine/reviewstate.go:503-505`), and the slug has been stamped on every
audit round since v0.30.0 — so it separates v0.30.0 from v0.29.0, not this release from its
predecessors. **Measured across all 15 recorded audit histories, adding that check changes the streak
in 0 of 15.**

## 3. The mechanism, once the decision is made

`consecutive_clean` is **derived, not stored** — `engine.ConsecutiveClean` walks `AuditRounds`, and
review shares the function. So the shape is to **add** rather than change: `ConsecutiveCleanSince` and
`ConvergedSince`, called from the audit path only.

**`Converged` must move too, not just `ConsecutiveClean`.** `engine.Converged` folds
`ConsecutiveClean` internally, so a reset that leaves `converged` answering the pre-reset value is not
a reset.

**Seven non-test call sites, and which ones move is the whole risk.** Re-derive the list rather than
copying it — this section's own predecessor cited `internal/cli/audit_record.go:101` and `:339`, and those are now
`:130` and `:392`:

```
grep -rn 'engine\.Converged(\|= Converged(' internal/ --include='*.go' | grep -v _test.go
```

At the time of writing it returns seven: `audit_record.go` twice, `budget.go`, `run_status.go`,
`internal/engine/resume.go` twice, and `review_status.go`.

**`internal/engine/resume.go`'s audit line is the one that gets missed.** It feeds `DetectPhase`'s `release`
branch, so leaving it on the shared `Converged` makes `tp resume` report `phase: release` on a streak
the reset just invalidated. **`review_status.go` must not move.**

## 4. The boundary against `--reconcile`

`--reconcile` records **why** a spec moved, preserving the operator's reason. This records only
**that** it moved, and only for the streak.

**They compose and neither implies the other.** A reset with no explanation tells an operator to redo
work without saying why; an explanation with no reset lets a stale claim stand. Two readers: the loop,
and the person.

## 5. Non-Goals

1. **No change to the review phase.** `review_status.go`'s `Converged` call stays on the shared
   function. Review already acts on spec movement by its own route.
2. **No reset of `spec_coverage_clean_rounds` or `role_streaks` in this release.** They are derived
   from the same rounds and the same argument applies, but each is a separate claim with its own
   consumers, and widening the blast radius of a reset is how a correct change becomes an incident.
3. **No repair of the 35 rounds whose snapshot and recorded hash disagree.** Whatever §2 decides,
   those rounds' hashes are not what their roles read, and this release does not pretend otherwise.
4. **No new workflow field.** The reset is not configurable; a spec that moved, moved.
5. **No `--reconcile` interaction.** A reconciliation explains a movement; it does not exempt it.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | two clean audit rounds either side of a spec change leave `consecutive_clean` at 1, not 2 | the shipped behaviour, which reports 2 |
| 2 | §3 *converged* | `converged` is false in the same fixture, not merely `consecutive_clean` reduced | move `ConsecutiveClean` alone, leaving `Converged` answering the pre-reset value |
| 3 | §3 *resume* | `tp resume` reports the audit phase, **not** `phase: release`, in that fixture | **move the `internal/cli` sites and leave `internal/engine/resume.go`'s audit line on the shared function** — every assertion three earlier drafts proposed still passes, and this is the section's own stated failure |
| 4 | §5.1 | the review phase's `consecutive_clean` and `converged` are byte-identical before and after | move `review_status.go` too, resetting a phase that already handles movement its own way |
| 5 | §2 | whichever option §2 takes, its stated cost is asserted: under (a) a pre-release round is unaffected; under (b) v0.35.0's recorded history reports `converged: false` | leave the vintage rule untested, so the release ships with (a)'s schema and (b)'s behaviour |
| 6 | §3 *no change* | an unchanged spec across five rounds leaves the streak at 5 | reset on every round, which makes convergence unreachable |

**Row 3 is recorded because no draft test caught it.** Three drafts proposed assertion lists and every
one passed under that mutant. It is first on this list rather than last for that reason.
