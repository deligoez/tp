# tp — A record clears staleness only through its own emission

A backlog decision, decided by the operator; it replaces the BUGS.md entry "A phase whose entire
recorded history was hand-recorded, with no emission, still clears `spec-stale` at `--record`".

Class: **decision**. One contract sentence.

## 1. The decision

**Context.** With no round that ever read text, staleness falls back to comparing the last recorded
round's stored hash, the pre-v1.2.2 answer, so recording without emitting clears `spec-stale` in a
phase that has never emitted. Since v1.2.2 a round carries the hash of its own emission snapshot.

**Decision.** One rule: a record clears staleness only through the snapshot of its own emission. A
record with no emission is still accepted, but it does not clear `spec-stale`, and the output says
why and names the emission that would bind a round to the text. The implementation is removing the
fallback, not adding a case for one kind of phase.

**Verify before building.** What else the fallback serves. A spec that converged before v1.2.2 has
rounds with a stored hash and no emission-snapshot hash. If such a history cannot be told apart from
"hand-recorded, never emitted", removing the fallback turns every spec converged before v1.2.2 stale
on upgrade — tp's own repository holds such specs, so dogfooding shows it at once — and that cost
goes back to the operator before the build. Also list what each test pinning the fallback asserts,
and whether SKILL describes a workflow that records without emitting.

**Checked before the build (2026-11-12, at 59975e14).**

- *The two histories can be told apart.* `hash_scheme` is written from v1.2.2 on; a round recorded
  before it carries none. `latestRoundThatReadText` (`internal/engine/reviewstate.go`) skips only
  rounds marked `no-emission`, so an unmarked round already counts as one that read text. The
  fallback is reached only when *every* recorded round of the phase is `no-emission`, which only a
  v1.2.2+ tp writes, and only for a `--record` with no snapshot on disk. A history that converged
  before v1.2.2 has unmarked rounds and never reaches it. What cannot be told apart is a pre-v1.2.2
  round recorded by hand from a pre-v1.2.2 round that was emitted — both unmarked — and both
  already count as read text; removing the fallback does not change that.
- *tp's own specs.* The clone holds 23 state directories: 22 specs, 44 phase histories, plus one
  under `spec/backlog/` whose spec file is gone. 38 histories are wholly unmarked, 6 wholly
  `snapshot`, none wholly or partly `no-emission`. `tp review|audit <spec> --status` (`stale`,
  `converged`) and `tp resume --file <spec>.tasks.json` (`phase`, `blockers`) were run on all 22
  with the freshly built binary and with a prototype whose fallback returns stale: the outputs are
  identical, both runs over a clean working tree. No pre-v1.2.2 spec turns stale, so the stop rule
  does not fire.
- *The tests that pin the fallback.* One asserts it directly:
  `TestStateStale_ARoundThatReadNoTextAnswersForNone` (engine) says an all-`no-emission` history is
  fresh when the text is unchanged and stale when edited. 35 cli tests fail with the prototype
  because their fixtures reach a converged state by `--record` with no emission, through
  `recordRound` or a direct `--record`. Two of them assert staleness itself
  (`TestReviewState_Staleness`, `TestReviewStatus_RoundsAndConvergence`: fresh after hand records);
  the rest assert what convergence unlocks — `next_action` naming import, import enforcement, the
  check gate, the round budget at the cap, the audit divergence signal, `--compact` keys, the
  record lifecycle. BUGS.md counted seven; the run counts 36. They keep what they assert once the
  fixture stands in for the emission it skipped, by writing the round's snapshot from the current
  text; running the emission itself would also run their registered checks, which several of them
  count.
- *SKILL.* No workflow records without emitting. `tp resume`'s paragraph already says a `--record`
  with no emission of its own no longer clears `spec-stale` (v1.2.2), which this decision makes
  true for every history; REFERENCE's state section describes the fallback and changes.

**Consequences.** Someone who only records by hand sees `spec-stale` until they emit once per text
version.

## 2. Alternatives

- **Refuse the record.** Rejected: it turns a ledger-only use into an error for no gain.
- **Leave it.** Rejected by the operator.

## 3. Non-goals

- Changing how an emitted round's snapshot hash is computed.
