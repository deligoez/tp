# tp — `tp run` drives every prompt a round emits

A backlog decision, decided by the operator; it replaces the BUGS.md entry "`tp run` never spawns a
regression unit".

Class: **decision**.

## 1. The decision

**Context.** A round emitted after a fix can carry a regression prompt, appended by a predicate at
emission rather than read from the role corpus. `tp run`'s oracle builds one unit per role of the
corpus, so under a run that prompt is emitted and nobody answers it; the round records and can
converge with the check unread. Which phases can carry it is checked in the code before the build
and named here then.

**Decision.** Under a run, every prompt a round's emission prints has a unit. Regression is today's
instance: when the emission carries it, the run spawns a unit of the same kind as that phase's
other role units, reading a role prompt and writing a role file. The oracle and the emission decide
"does this round carry regression" through one shared function, not two copies. The acceptance is a
test over the whole panel, so the next role appended by a predicate is caught the same way.

Two assumptions are checked before the build and written down: that the run's record step merges
role parts by pattern, so a regression part is picked up; and that the plugin's write hook grants a
role unit its part path by pattern, covering the regression id. If the hook grants per corpus role,
the change is plugin content and a version bump.

**Consequences.** Rounds after a fix gain one unit under a run, so more tokens and time, and a run
that used to converge may find a regression and take another round. The budget a role unit counts
against is named here once checked; raising any `run_max_*` stays the operator's decision.

**Checked before the build (2026-11-12, at 59975e14).**

- *Which phases carry regression.* Only `tp review`'s default panel: from round 2, outside
  `--no-state`, when the changed-sections diff against the newest earlier snapshot is non-empty or a
  recorded finding is resolved `fixed` (`buildReviewPrompts`, `internal/cli/review.go`). `tp audit`
  never emits it. `--perspective regression` emits it alone, but no unit kind runs a perspective.
  `tp epic review` appends its own regression role, but `tp run` does not drive epic review: the
  oracle's phases are implement, decompose, review, audit and release.
- *The record step.* It merges `$TP_ROUND_DIR/role-*.ndjson` by glob (`recordBriefCommand`,
  `internal/engine/briefcommand.go`), and under `TP_ROUND_DIR` the regression prompt already names
  `$TP_ROUND_DIR/role-regression.ndjson.part` as its `output_path`, as every review prompt does. A
  regression part is picked up with no change.
- *The plugin's write hook.* `hooks/pre-tool-use-role-write-allow.sh` grants
  `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part`, and the Stop hook reads the same path. Both key on
  the unit id, not on the corpus, so the regression id is covered: no plugin content changes and no
  version bump for this decision.
- *The budget.* A regression unit is one spawned child, so it counts against `run_max_units` (a
  retry counts again) and is attempted `1 + run_max_unit_retries` times; its spend accrues to
  `run_max_budget_usd` and each attempt is bounded by `run_max_unit_budget_usd`; its time is inside
  `run_max_wall_clock_seconds`. `run_max_units` is the cap whose count grows by one per round
  after a fix.
- *Beyond the note.* Under a driver, `withSiblingUnits` (`internal/cli/rolefilter.go`) leaves out of
  a unit's `skipped_roles` the roles a sibling unit covers. It listed only corpus roles, so every
  role unit's payload named regression under `role-filter` as the one prompt no unit covered. Once
  regression has a unit, that entry is wrong, so the sibling set takes regression when the round
  carries it.

## 2. Alternatives

- **Do not emit regression under a run.** Rejected: consistent, but it drops a check silently.
- **Make regression an ordinary corpus role.** Rejected: it would run on rounds with nothing to
  verify, the cost epic review removed in 1.11.0.

## 3. Non-goals

- A new unit kind or agent.
