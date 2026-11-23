# A worktree agent's close lands through the coordinator

Status: decision, building.

## Problem

A hand-driven coordinator claims a task on its own tree and spawns an agent in a worktree. The claim
is an uncommitted edit, so the agent's worktree has the task open. Two ways of closing were tried in
the v1.29.0 branch, and both failed review (`spec/1.29.0-measurements.md`, *The review rounds*):

- **The close on the agent's own copy.** The coordinator's stop guard cannot see it, and merging the
  branch meets the uncommitted claims.
- **The close written beside the claim.** A close is more than a record. The gate also ran on the
  coordinator's tree, `--auto-commit` committed the coordinator's files, `tp commit` bypassed it,
  and a dependent became ready before the agent's code was on the coordinator's branch.

tp's rule after the spec is written is that work runs with no human input. So whatever ends the
agent's task must be a command an agent can run, named by a hook or a hint, and every failure must
end in another such command.

## Decision

1. **The agent asks; it does not close.** `tp done <id> --commit <sha>... -- "<evidence>"` in a
   linked worktree, for a task a spawn holds the claim on in any copy, records a *land request*
   instead of a close. It checks, in this order: one id and `--commit` given; `--auto-commit`,
   `--skip-gate` and `--covered-by` refused; the evidence and dependencies, read from the copy that
   holds the claim; each commit is on the worktree's `HEAD` and touches no tp-owned path; then the
   gate runs in the worktree. `tp commit` and `tp done --auto-commit` there are refused with the exact
   commands to run instead. Inside `tp run` nothing changes: the driver's own pending close applies.
2. **Requests live in the git common directory**, under `tp/land/`, keyed by the task file's path
   relative to the repository's top level. Every worktree reads the same requests, and removing the
   agent's worktree removes none. `tp run`'s pending directory is not touched.
3. **The coordinator lands with one command.** `tp land` in the coordinator's tree takes each request
   for the task file: a task already done drops its request; commits already on `HEAD` (a hand merge
   with the same commits, or a crash after the pick) are not picked again; otherwise they are
   cherry-picked onto `HEAD`. Then `tp done <id> --commit <landed>... -- "<evidence>"` closes the
   task there, its gate running once, on the landed tree. Under `hc` and `builtin` alike the commits
   are the agent's, so nothing else is committed.
4. **A failed landing reopens the task.** A conflict, a commit that no longer exists, or a red gate
   takes the pick back (`git reset --keep`), reopens the task as solo with its claim cleared, drops
   the request, and names the cause and the brief to spawn a fresh agent from, on the current `HEAD`,
   as `tp run` does with a unit that failed to integrate. The agent's worktree is told the outcome and
   closes nothing. A pick refused by the coordinator's own state (anything staged, or uncommitted
   changes in the same paths) reopens nothing: `tp land` names the paths and asks for them to be
   committed first. No hint suggests `--skip-gate` or `--force`.
5. **The hooks sequence it.** `tp claims --spawn` counts a task with a land request as accounted. The
   pool reads requests as a class of its own, *landing*, ahead of every other: the coordinator's stop
   is refused while one waits, with `tp land` as the reason's first command, whether or not a slot
   is free. A landing task keeps its `touches` busy and holds no slot.

A task is done only once its code is on the coordinator's branch, so a dependent cannot start before
it.

## Alternatives

- **Close beside the claim** (the v1.29.0 branch's first repair). Rejected for the reasons above.
- **Merge instead of cherry-pick.** A merge brings in whatever the agent's branch was cut from, and a
  merge commit cannot be closed on as the task's commit. The pick is what `tp run` does.
- **Park a failed landing with `waiting`.** It leaves a task nobody resumes without a person; a
  reopened task is the pool's to refill.

## Non-goals

- A hand merge that rewrote the commits (squash, rebase): `tp land` does not recognise it, picks
  again, and on conflict reopens the task.
- A coordinator whose own tree fails the gate: the agent's task is reopened for it.
- A worktree whose own copy carries a spawn's claim on the task (cut after an older claim was
  committed, then taken over): it speaks for itself, and its close stays local. Only a copy that has
  the task open looks for a holder, and only a `wip` one under a spawn's claim, so neither the
  coordinator's own tree nor `tp run` ever writes into a stale copy.
- A new workflow field, an escalation, or an operator decision.

## Acceptance

One end-to-end test, run under both commit strategies: a gate that fails without the agent's file, no
`--gate-passed`, the agent in a real worktree, a dependent task, and every command after the spawn
copied from a hook reason or a tp hint. The dependent is not ready until `tp land` has run, and is
ready after it.
