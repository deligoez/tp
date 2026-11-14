# tp — Spec-coverage and the review panel count only when they answered

A backlog decision for 1.20.0, worked out with the tp-smart session. It narrows the BUGS.md entry
"`tp review --check` converges with roles that never ran" and closes the audit half of the same
class: convergence certifies a reading that did not happen. Class: **decision**. The measurements are
in `spec/1.20.0-measurements.md`.

## 1. Context

- Under `tp run` the class is already closed: a role unit succeeds only when its role file parses, and
  the record unit is scheduled only once every emitted role's file does (`roundBasedUnits`, pinned by
  `TestBuildNextUnits_RecordUnitOnceEveryRoleAnswered`).
- By hand it is open. `tp review <spec> --record <file>` records whatever rows the file holds, so a
  role that never ran reads as a role that found nothing. Epic review closed the same gap in v1.10.0
  with `--record-inbox`.
- Audit rows carry a verdict per checklist item, PASS included, so absence is visible. A round in
  which spec-coverage graded part of what it was asked, all PASS, counts as a clean spec-coverage
  round today. On tp's own history this never happened (0 of 23 clean rounds); a spec-coverage that
  wrote no rows at all happened twice, and today's `spec_coverage_clean_rounds` reads that as `null`
  in the latest round.

## 2. Decision

1. **Audit: spec-coverage's asked items are recorded at emission.** The round's pools file, which
   holds the code roles' file items since v1.15.0, also holds the spec-coverage role with its emitted
   checklist item ids, whatever `--role` filter the emission ran under. `--record` reports it in
   `graded` like a code role (`spec-coverage 58 of 62`). The asked set is the checklist only; carried
   Prior Round keys stay under the 1.18.0 rule, so one row is never held by two rules.
2. **A partly graded spec-coverage round is not clean.** The predicate is containment: every asked id
   has a row, whatever its verdict; extra or repeated rows make up for none. A round that fails it is
   not clean, as a round with an open row is not, and its spec-coverage `consecutive_clean` is 0. A
   round whose spec-coverage wrote no rows is now `0 of N`, so not clean, where it read `null` before;
   a round asked nothing (`no-checklist-items`) is clean as today.
3. **An audit round recorded without its emission asks nothing it can show.** From 1.20.0 such a
   round is stamped `unasked: true` and is not a clean spec-coverage round: v1.12.0 already holds that
   a record with no emission of its own certifies no reading.
4. **Review: `tp review <spec> --record-inbox`.** It records the collected round from its inbox: it
   merges every role file there and refuses, exit 3, naming each required role with no file. The
   required roles are every role the round would have asked without a `--role` filter: the emitted
   ones plus those `skipped_roles` lists with reason `role-filter`; the other skip reasons stay
   exempt. An empty file is a role that ran and found nothing. The review-record unit under `tp run`
   uses it in place of the merge-then-record chain.
5. **Old rounds are not re-judged.** A round recorded before v1.20.0 has no asked set and no `unasked`
   stamp; it counts as before.

## 3. Consequences

- A hand-driven review that wants convergence it can trust records with `--record-inbox`.
  `--record <file>` is unchanged, an empty file included, and REFERENCE says it cannot show presence.
- The audit's `graded` list grows by one entry per emitted round, and `next_action` leads with it.
- An audit recorded by hand without an emission no longer converges on spec-coverage.

## 4. Alternatives

- **A `ran: true` row per role.** Rejected: a hand-made file can hold that row too, while a missing
  file cannot be faked.
- **Refusing an empty `--record` over an emission that named roles**, as epic review does. Not taken:
  it changes how a clean round is recorded by hand in the workflows, the agent text and the fixtures,
  and `--record-inbox` already gives the proof to whoever wants it.
- **Blocking partial code-role grading.** Not taken: a code role's list is a cut of its pool by
  design; blocking it would make every large change unshippable.

## 5. Non-goals

- Epic review's other inbox machinery: the re-emit guard, frozen dispositions, `--discard`.
- Showing presence as unproven in `--status` for a round recorded from a hand file.
- Cutting or chunking the audit's file list or the spec excerpt (`checklist-covers-what-changed.md`).
