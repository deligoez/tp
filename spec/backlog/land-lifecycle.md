# tp — What a land leaves behind

**Decided** by the operator on 2026-11-25, on the Decision track; ships in 1.32.0.

## 1. Context

In the coordinator's pool an agent commits in its own worktree, `tp done` records a land request, and
the coordinator's `tp land` picks the commits onto its tree and closes the task. A field report
(2026-11-25, 26 tasks) found two things left behind:

- The task file stays modified after every land, so under `hc` each land costs a separate close
  commit: about ten extra commits in a 13-task run.
- No worktree or branch is ever removed: 29 remained, one still locked by the coordinator's pid. The
  harness removes only a worktree with no changes, and every landed one has commits.

## 2. Decision

1. **Land commits the task file itself**, under the effective `commit_strategy`, the way
   `tp done --auto-commit` does: one commit per `tp land` call, however many tasks it closed.
2. **Land removes a landed task's worktree and branch when it is safe**: tp's own landed record says
   every commit was picked, the worktree has no uncommitted change, and no live process holds its lock.
   Otherwise it leaves both and names each one kept, with the reason, in its output. The check reads
   tp's landed record, not `git cherry`: a land that resolved a conflict changes the patch ids.

## 3. Alternatives

- Land only says the task file changed. It keeps a step the coordinator has to remember.
- A `--prune` flag. The same: removal happens only when someone remembers to ask.

## 4. Non-goals

- Removing a worktree that holds work, or one a live process holds.
- Changing what `tp done` records in a worktree.
