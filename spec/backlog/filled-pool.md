# A filled pool for a hand-driven coordinator

Status: decision, building after agent-bound-claims.md.

## Problem

`tp run` fills its pool deterministically. Its rolling slots start the next ready task as soon as a
unit exits, up to `run_max_parallel`.

A coordinator that spawns subagents by hand has no such loop. A field report showed:

- **Ready tasks left idle.** The coordinator looked for ready tasks only when an agent finished, so
  tasks that became ready while others ran stayed unstarted.
- **Gates kept only in the coordinator's head.** A gate that was not a task stayed unrecorded, so
  every reader saw the task as ready.

## What the harness gives

Measured in a scratch repository with `claude -p` and a logging hook:

- **The main session's `Stop` fires at the end of every turn.** Its input carries `background_tasks`,
  the session's live background work. A running subagent is listed as
  `{"id": <agent_id>, "type": "subagent", "status": "running", ...}`.
- **A subagent stopped with `TaskStop` drops out of `background_tasks`.** Its `meta.json` has no
  `finished` key, and neither has a running agent's, so `background_tasks` is the liveness signal.
- **`agent-<agent_id>.meta.json` holds the spawn's `toolUseId`.** This links a live agent to the
  `claimed_by.spawn` set by agent-bound-claims.md.
- **A `Stop` block makes the session act on the reason.** The next `Stop` comes with
  `stop_hook_active: true`.

## Decision

1. **A live claim is one whose spawn is running.** A wip task holds a slot when its
   `claimed_by.spawn` is the `toolUseId` of a subagent that `background_tasks` lists as running.
   - A wip task with no running spawn is reported as orphaned and holds no slot, whether its agent
     finished without closing the task or was killed.
2. **The coordinator is asked to fill free slots.** The plugin's `Stop` hook runs
   `tp pool --session <session_id> --live <spawn ids>`, which answers with the cap
   (`run_max_parallel`), the live claims, the orphans and the ready tasks.
   - It blocks the stop once when all three hold:
     - the session holds or has held a claim in this repository (it is a coordinator);
     - free slots exist;
     - `tp ready` lists tasks.
   - The reason names each ready task with its `tp brief` command, up to the free slots, plus any
     orphans.
   - With `stop_hook_active` set, or for any session that never claimed, it exits 0.
   - A `tp run` unit never claims through a spawn, so the hook never fires for one.
3. **A gate that is not a task is written down.** `waiting` becomes legal on an open task, where it
   means held: `tp ready`, `tp next` and `tp run`'s pick skip the task. `tp export` gives it state
   `held`, and `tp validate` stops rejecting it. A gate that is a task stays a `depends_on`. A held
   task is the coordinator's on-disk answer to "why is this ready task not started?", and the hook
   stops asking about it.

## Alternatives rejected

- **Move the coordinator to `tp run`.** That is the existing deterministic path. A coordinator that
  briefs agents by hand and waits on other sessions' decisions is not one `tp run` drives today.
- **Count live agents from `meta.json` or transcript age.** A killed agent and a running one look
  alike in `meta.json`, and a long command writes nothing to the transcript.
- **Block every main session with ready tasks.** Ordinary sessions in a tp project would be stopped
  on every turn.

## Non-goals

- Foreground subagents: the coordinator's turn does not end while one runs.
- Choosing which ready task to start: the hook lists them in `tp ready`'s order.
- Raising the cap: `run_max_parallel` stays the operator's (rule 9).

## Consequences

- REFERENCE.md gains `tp pool`, the `held` export state, and `waiting` on open tasks.
- The dashboard and board clients show `held` as a waiting column, not a ready one.
- Each turn a coordinator ends with free slots and ready tasks costs one more turn.
