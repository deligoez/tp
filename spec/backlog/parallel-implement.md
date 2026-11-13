# tp — Implement units run side by side in worktrees under tp run

A backlog decision for 1.19.0. The operator chose the shape (a worktree per unit, `touches` for
eligibility, off by default); the invariants below were settled with a second opinion from the
tp-smart session. Measurements: `spec/1.19.0-measurements.md`.

Class: **decision**.

## 1. Context

`tp run` spawns one implement unit at a time: implement is a non-concurrent kind, so a spec of
independent tasks still runs them in series. Replaying the DAGs of tp's own shipped task files with
each task's recorded `commit_files` as its file set, the ready tasks whose file sets are pairwise
disjoint would have finished the same work in fewer steps (the ratio at each cap is in the
measurements): worth having, and bounded by how well a task names what it touches.

## 2. Decision

1. **`touches` names what a task may change.** An optional task field of repo-relative path globs,
   written at decompose. A task without it always runs alone. `tp validate` refuses a malformed glob;
   `tp lint` warns on a task without `touches` when `run_max_parallel` is above 1.
2. **`run_max_parallel` caps a batch.** A workflow field, default 1, so nothing changes until the
   operator raises it, an operator decision as every `run_max_*` is. In the implement phase the oracle
   returns up to that many ready tasks whose `touches` are pairwise disjoint, the first ready task
   leading; two globs overlap unless their literal prefixes diverge, case-insensitively, and an empty
   literal prefix overlaps everything. Each unit's brief names its task id.
3. **A worktree per unit, outside the tree.** For a batch of more than one, the driver adds a git
   worktree per unit beside the run state, on a branch from the main branch's HEAD; the unit's tp
   commands read the main task file through `TP_FILE`, which the foreign-tree refusal accepts for a
   linked worktree of the same repository and no other tree.
4. **The unit asks; the driver closes.** In a worktree, `tp done` validates the commits are on the
   unit's branch and touch no tp-owned path (the task file, spec state, `.tp/`), writes a pending close
   and runs no gate. After the batch the driver, in task order, cherry-picks each unit's commits onto
   the main branch, runs the quality gate there, and on a pass closes the task with the picked shas.
   A conflict or a failing gate restores the main branch to before the pick and reopens the task with
   a durable `solo` mark the oracle reads, so it runs alone next. A diff that leaves its `touches` is
   recorded and warned, not reopened: the pick and the gate are the safety, `touches` the schedule.
5. **Gates and tests are one at a time.** The driver gates one pick at a time, and a unit's brief
   tells it to take tp's test lock around any test run, since parallel test load once exhausted the
   machine.
6. **A crash is reconciled at start.** A run that finds pending closes, unit branches or worktrees from
   an earlier run integrates or reopens them before its first batch, and `tp resume` names them.
7. **Stops.** When a unit escalates, its finished siblings are integrated before the run stops. A cap
   is checked between batches, so a batch may overshoot it by up to its size less one.

## 3. Consequences

- A project whose tests need ignored files (dependencies, `.env`, build output) cannot run units from a
  fresh worktree; the dry run says so once, and such a project keeps `run_max_parallel` at 1.
- A failed integration discards the unit's work and redoes it alone; the measurements count it.
- Without the driver, an agent fanning out by hand gets the same pieces: `touches`, the batch the
  oracle returns, and the pending close.

## 4. Alternatives

- **The unit also gates in its worktree.** Fewer discarded units, about twice the gates. The
  operator's call, put with the measured discard rate; not built.
- **Reopen a unit whose diff left its `touches`.** Rejected: it discards work that integrated cleanly
  and passed the gate.
- **Same tree, disjoint files.** Rejected by the operator: the gate would run over another unit's
  half-written change.

## 5. Non-goals

- Parallel review, audit or decompose beyond today's sibling roles.
- Merging by anything but cherry-pick; conflict resolution by an agent.
