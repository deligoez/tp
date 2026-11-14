# tp — A round counts only what its roles graded

A backlog decision for 1.20.0, worked out with the tp-smart session. It closes the BUGS.md entry
"`tp review --check` converges with roles that never ran" and the audit half of the same class:
convergence certifies a reading that did not happen. Class: **decision**. The measurements are in
`spec/1.20.0-measurements.md`.

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
  wrote no rows at all happened twice, and today's `spec_coverage_clean_rounds` already reads that as
  `null` in the latest round.

## 2. Decision

1. **Audit: spec-coverage's asked items are recorded at emission.** The round's pools file, which
   holds the code roles' file items since v1.15.0, also holds the spec-coverage role with its emitted
   checklist item ids. `--record` reports it in `graded` like a code role (`spec-coverage 58 of 62`).
2. **A partly graded spec-coverage round is not clean.** A recorded round whose spec-coverage graded
   fewer items than it was asked counts as not clean for that role, as an open row does: its
   `consecutive_clean`, and so `spec_coverage_clean_rounds`, is 0 for that round. A round recorded
   without an emission has no asked set and counts as before.
3. **Review: `tp review <spec> --record-inbox`.** It records the collected round from its inbox: it
   merges every role file there and refuses, exit 3, naming each role the round emitted that has no
   file. An empty file is a role that ran and found nothing. The emitted roles are the emission's
   panel, regression prompt included. The review-record unit under `tp run` uses it in place of the
   merge-then-record chain.
4. **`--record <file>` stays.** An empty `--record` file over a round whose emission named roles is
   refused, exit 3, as epic review does. A non-empty hand-made file keeps the gap, and REFERENCE says
   so: `--record-inbox` is the form that proves every role ran.
5. **Old rounds are not re-judged.** A round emitted before v1.20.0 has no asked set and no recorded
   panel; its presence is unknown, reported as nothing, never blocking.

## 3. Consequences

- A hand-driven review that wants convergence it can trust records with `--record-inbox`.
- The audit's `graded` list grows by one entry per round; `next_action` leads with it.

## 4. Alternatives

- **A `ran: true` row per role.** Rejected: a hand-made file can hold that row too, while a missing
  file cannot be faked.
- **Blocking partial code-role grading.** Not taken: a code role's list is a cut of its pool by
  design; blocking it would make every large change unshippable.

## 5. Non-goals

- Cutting or chunking the audit's file list or the spec excerpt (`checklist-covers-what-changed.md`).
- Re-grading recorded rounds.
