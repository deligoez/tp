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
   written at decompose. It is a hint that avoids discards: a task without it joins a batch
   unscreened (v1.20.0, the operator's decision; until then it always ran alone), since safety rests on
   the pick and the gate. `tp validate` refuses a malformed glob and warns on an open task without
   `touches` when `run_max_parallel` is above 1.
2. **`run_max_parallel` caps a batch.** A workflow field, default 1, so nothing changes until the
   operator raises it, an operator decision as every `run_max_*` is. In the implement phase the oracle
   returns up to that many ready tasks whose `touches` are pairwise disjoint, the first ready task
   leading; two globs overlap unless their literal prefixes diverge, case-insensitively, and an empty
   literal prefix overlaps everything. Each unit's brief names its task id.
3. **A worktree per unit, outside the tree.** For a batch of more than one, the driver adds a git
   worktree per unit under the system's temporary directory, on a branch from the main branch's
   HEAD; the unit's tp commands read the main task file through an absolute `TP_FILE`. In a linked
   worktree of the same repository `tp done` writes the pending close below instead of meeting the
   foreign-tree refusal, which every other tree mode still meets there.
4. **The unit asks; the driver closes.** In a worktree, `tp done` validates the commits are on the
   unit's branch and touch no tp-owned path (the task file, spec state, `.tp/`), runs the quality gate
   in the worktree, and on a pass writes a pending close (v1.20.0, the operator's decision; until then
   it ran no gate). After the batch the driver, in id order, cherry-picks each unit's commits onto
   the main branch, runs the quality gate there again, and on a pass closes the task with the picked
   shas. The second gate stays even when the main branch has not moved since the unit's base: skipping
   it is a cost the operator has not taken.
   A conflict or a failing gate restores the main branch to before the pick and reopens the task with
   a durable `solo` mark the oracle reads, so it runs alone next. A diff that leaves its `touches` is
   warned on stderr, not reopened: the pick and the gate are the safety, `touches` the schedule.
5. **Integration gates are one at a time.** The driver gates one pick at a time. The units' own gates
   run side by side, up to `run_max_parallel`: the operator revised 1.19.0's "gates always
   serialized" for this in v1.20.0, choosing it over one lock for every gate with the modelled cost in
   `spec/1.20.0-measurements.md` before them. While working, a unit's brief tells it to run only the
   tests its change needs, never the whole suite, since parallel test load once exhausted the machine.
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

- **The unit also gates in its worktree.** Fewer discarded units, about twice the gates. Taken in
  v1.20.0 by the operator's decision: a red change is caught in the unit, before a pick.
- **Reopen a unit whose diff left its `touches`.** Rejected: it discards work that integrated cleanly
  and passed the gate.
- **Same tree, disjoint files.** Rejected by the operator: the gate would run over another unit's
  half-written change.

## 5. Non-goals

- Parallel review, audit or decompose beyond today's sibling roles.
- Merging by anything but cherry-pick; conflict resolution by an agent.
