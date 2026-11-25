# tp — Audit rounds that carry decisions and fit the auditor

**Decided** by the operator on 2026-11-25, on the Decision track; ships in 1.32.0.

## 1. Context

A field report (2026-11-25, an audit run to its cap) and the 1.31.0 cycle found:

- A row resolved `wontfix` by the operator came back in the next round as an item to grade, though
  its cited code was untouched; one auditor graded it PASS to avoid a live finding at the cap.
- The audit prompt names a results inbox inside the repository, and auditors told the checkout is
  read-only had nowhere to write.
- With 26 affected files the coordinator sliced the checklist by hand and merged the slices with
  `tp audit --merge`, because past 10 files one prompt skims.
- A sentence added to a spec after its review cap raised `spec-stale` with only "raising the cap is
  the operator's decision" for an answer.
- Before the last audit round `next_action` said only "address the findings, then re-audit", and an
  agent concluded convergence was impossible.

## 2. Decision

1. **A `wontfix` carries** to the next round while the files its row cites are unchanged, the way a
   ground round carries an unchanged unit; the auditor is not asked about it again.
2. **The results inbox can be named**: an override for the path the emitted prompt tells a unit to
   write to. The default stays where it is.
3. **`--record-inbox` takes several part files per role** and merges them, so slicing by hand costs no
   `--merge` call. tp does not slice by itself (*The divisible round*, closed, stays closed).
4. **The `spec-stale` message after a review cap names both ways forward**: revert the sentence and
   record the decision in the measurements file, or raise the cap.
5. **Before the last allowed audit round, `next_action` says so**: the next round is the last, and the
   loop ends `done_by: cap` once its findings are dispositioned.

## 3. Alternatives

- Tell the auditor how to grade an accepted deviation. It relies on the auditor, which is what failed.
- Move the inbox outside the repository by default. It changes every project for one project's setup.
- Slice prompts inside tp. Closed before: depth comes from the per-task audit.
- A recorded addendum to a capped spec. A repair subtracts; decisions already live in measurements.

## 4. Non-goals

- Carrying a `wontfix` whose cited files changed.
- Changing what a clean round or convergence counts.
