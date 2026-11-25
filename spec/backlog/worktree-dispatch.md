# tp — Which work goes to a worktree

**Decided** by the operator on 2026-11-25, on the Decision track; ships in 1.32.0.

## 1. Context

Three things went wrong when tasks ran side by side in worktrees (the 1.31.0 run and a field report):

- A task whose work was `tp set --workflow --project ...` ran in a worktree, and `tp done` refused its
  commit because `.tp/config.json` is written only by tp.
- Three 1.31.0 tasks did not cherry-pick onto main. Each reopened and ran again from scratch, so
  the unit's work was thrown away.
- In the coordinator's pool, `tp brief` let two tasks whose `touches` overlap be claimed at once (both
  touched a manifest and its lockfile). `tp run` already keeps such tasks apart.

## 2. Decision

1. **A task whose `touches` include `.tp/config.json` runs in the main tree**, never in a worktree,
   under `tp run` and in the coordinator's pool.
2. **A conflict is resolved, not redone.** When a unit's commits do not apply to main, the branch is
   kept and the next unit for that task is told to bring the branch onto main and resolve the
   conflict, rather than to start the task again.
3. **`tp brief` claims under the same `touches` rule `tp run` uses**: a ready task whose `touches`
   overlap a claimed task's is not handed out while that claim stands.

## 3. Alternatives

- Accept a worktree commit whose only `.tp/config.json` change is the one `tp set` produced. Proving
  that is harder than not dispatching the task.
- Keep redoing a conflicting task. It cost three units' work in 1.31.0.

## 4. Non-goals

- Inferring `touches` a task did not declare.
- Resolving conflicts inside the driver: the unit resolves them.
