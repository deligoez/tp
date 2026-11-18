# Rolling implement slots

A decision note (Decision track). Operator decisions, 2026-11-18: rolling replaces the batch barrier
as the only schedule above `run_max_parallel` 1, and the barrier code is deleted; the work is a
Decision, not a Full cycle.

## 1. Today

Read in the code at `3158f7d8`. `runIteration` (`internal/engine/driver.go`) runs one implement
batch as a barrier:

1. `startBatch` claims up to `run_max_parallel` ready tasks whose `touches` do not overlap, adds
   one worktree each and runs `worktree_setup_cmd` in each;
2. `attemptUnits` spawns them through `spawnAll`, which waits for **every** unit, and retries the
   failed ones inside the iteration as a group;
3. `integrateBatch` cherry-picks every pending close onto the main branch in id order, one
   `tp done` per pick, so the main gates run one at a time;
4. the loop re-reads `tp resume` and builds the next batch.

A unit that finishes early leaves its slot idle until the slowest sibling and its retries end, and a
task its close unblocks waits for the whole batch's integration.

## 2. Decision

Above `run_max_parallel` 1, the implement phase keeps its slots filled.

**The serial rule.** One loop owns the main branch. While a pick or a main gate is running, nothing
is spawned, refilled or retried, and no worktree is added or set up. Every unit that has exited is
integrated before any slot is refilled. So a worktree never branches from a half-picked `HEAD`, and
"disjoint from the running units" is the whole overlap rule. A child's process is started on the
loop itself, so a spawn or a retry never lands inside a later integration; only the wait for it runs
beside the loop. A refill that frees several slots starts their units off one oracle read: safe,
because nothing is integrated between that read and those starts, so `HEAD` does not move, and the
only claims made meanwhile are of units that same read returned.

**The loop.** Whenever a unit's child exits:

1. its pending close, if it asked for one, is integrated as today: pick, `tp done` on the main
   branch, and on a conflict or a red gate, `reset --keep`, reopen, `solo`;
2. the refill asks the oracle, not a second scheduler in the driver: the oracle's implement units
   with the running ids left out, then the units whose `touches` overlap no running unit's, up to
   the free slots. `tp run --dry-run` and the run therefore name the same units;
3. each new unit gets a fresh worktree off the current `HEAD` and its `worktree_setup_cmd`.

**Live claims.** The pool's claims are wip on the main branch, and a wip task today means "resume
alone". Two readers need to tell a live claim from an abandoned one, and they answer it apart:

- **The driver infers nothing from disk.** It knows its pool in memory and passes the running ids to
  the oracle explicitly (one function, a `running` parameter), so the decision path never guesses
  liveness. When `tp run` takes its lock, every open unit row left by an earlier run is abandoned
  by definition: reconcile closes it, and the pool starts empty. A test: a run that starts over a
  crashed run's open rows treats those tasks as wip resumes, run alone, not as pool members. The
  one exception is a unit the dead run left live, which the new run waits for before it integrates
  or removes anything. A unit is live only when all three hold: (a) the dead run's state lists it in
  `pool`, so it was never integrated; (b) the pid of the unit's own child, recorded on its row at
  spawn, is alive (`kill(pid, 0)`, EPERM counting as alive); (c) its unit lock, which its child
  inherits, is held. A daemon the unit left behind holds the lock after the child exited and fails
  (b); a clean stop empties the pool and fails (a). While it waits, the new run carries those units
  and their pids into its own `pool`, and its notice names each unit and pid. Windows takes no
  lock, so a restarted run there cannot tell a live unit.
- **An outside `tp resume`** (the operator, a hook, the dashboard, `tp run --dry-run`) reads
  identity from the run state's `pool` (a unit is a member from its claim through its setup and
  exit until it is integrated) and liveness from whether the run lock is held, or, with the lock
  free, from the same three-part rule a restarted run waits on, so the reader and the driver agree: the
  kernel releases a lock on a crash, and rows stay. Probing must not race a starting run: a
  `tp run` that starts while `tp resume` is called in a loop does not exit 4 as a second run, and a
  test shows it. During a live run, `tp resume` names the running units as running, and
  `tp run --dry-run` says a run is in progress rather than naming units. This reader's answer is
  guidance only, never a decision: nothing in the driver reads it.

**Retry** keeps today's shape: same worktree, same branch, in the unit's own slot, while other slots
keep running. The serial rule applies to it.

**Runs alone.** A resumed wip task, a `solo` task, and a full task-audit batch (v1.22.0: it comes
before more implement units and never mixes with them) each drain the pool first. While one is
waiting, no slot is refilled, so it cannot starve behind a pool that keeps filling.

**Stopping.** A signal, an escalation, a cap or a driver error stops new spawns and retries. The
running units finish, their closes are integrated, and then the run stops with today's reason and
precedence. A second escalation during the drain is recorded by the lowest seq, as
`lowestEscalation` does today. Caps are checked before every spawn, so `run_max_units` is no longer
overshot; the spend and wall-clock caps can still be passed during the drain, and the docs say so.

**Setup failure.** There is no batch to unwind. The failing unit's claim, worktree and branch are
released, new spawns stop, the running units drain and are integrated, and the run stops with
`worktree-setup-failed`. A failed `git worktree add` does the same and stops with `driver-error`.

**Unchanged.** Cap 1 is the same path: one unit at a time, in the main tree, no worktree. Review,
audit and every other phase are unchanged, and so are the integration refusals and the crash
artifacts: pending closes, worktrees and `tp-unit/` branches.

## 3. Alternatives

- **Keep the barrier and document it.** Rejected by the operator.
- **An opt-in `run_schedule` field.** Rejected: nobody has run `tp run` above cap 1 in the field,
  and a second schedule would be kept forever.
- **Refill without integrating first.** Rejected: a new unit would branch from a main that lacks the
  finished work, and the readiness of that work's dependents would be wrong.
- **Rebase a unit's branch before its pick.** Rejected: the cherry-pick already replays the commits
  onto the current main, and a rebase rewrites the commits after their gate, which breaks the
  `gated_tree` link. A conflict still means `solo`.

## 4. What must be shown

- **Cap 1 is identical.** Cap 1's existing tests pass unchanged, and the barrier code is gone: no
  second scheduler remains.
- **The serial rule.** A test: between a pick's start and its close or reset, no worktree is added
  and no child is spawned.
- **Kill points.** For each of these a test kills the run there and shows the next `tp run`
  recovers cleanly:
  - during a pick;
  - after a close, before its request is deleted;
  - during a setup;
  - during a refill, with a worktree added and no child spawned yet.
- **Readers of the batch boundary.** Swept and corrected: `tp run --dry-run`'s `next_units`,
  `tp resume`'s guidance (v1.24.0's chain note), the run state, the dashboard, `reconcileBatches`,
  `notify_cmd`, REFERENCE's "Implement batches" and every doc sentence that says batch for the
  implement phase.
- **Measurement.** A fixture whose gate takes real time and whose units differ in length. The
  expected numbers are written before the run. Measured at cap 1, cap 2 and cap 3, against the
  barrier binary of `3158f7d8`:
  - wall time;
  - idle slot-seconds;
  - the peak number of gates running at once;
  - the gate-reuse rate;
  - the count of `solo` reopens;
  - how often a task-audit batch drains the pool while `task_audit_roles` is set.

  The fixture proves the mechanism; the gain in a compiling project is the field's to measure.
- **Load.** Rolling can run N unit gates beside one main gate. That is a new load shape on a machine
  a parallel test load has crashed once, so the measurement reports the peak and the docs name it.

## 5. Non-goals

- Parallel scheduling for review or audit.
- More units than `run_max_parallel`.
- Changes to how a pick or a main gate runs.
