# Agent-bound claims and a filled pool

Status: decision candidate.

## Problem

A coordinator that spawns one subagent per task from hand-written briefs, outside `tp run`, depends
on agents remembering four steps, and a field report shows each one slipping:

- **Missed claims.** Subagents closed tasks they never claimed, so `tp done` claimed them
  implicitly and the task recorded about zero minutes.
- **Early claims.** A task was claimed for an agent that started it hours later, so its elapsed time
  counted the wait.
- **Unexplained wip.** Wip tasks did not say which were being worked and which were parked behind a
  merge, a long run or a decision.
- **Ready tasks left idle.** The coordinator only looked for ready tasks when an agent finished, so a
  task that became ready while others ran stayed unstarted.

A real gate the coordinator kept in its head, a tag the task's output had to be built from, was
recorded neither as a dependency nor as `waiting`.

**Git cannot stand in for claims.** A task committed through one `hc run` has the same author time
on its first and last commit. Work is not always done in a worktree. Other people's commits land in
between. So a task's commits say nothing about when it started.

## What the harness gives

Measured in a scratch repository, with `claude -p` and a probe hook:

- **Identity reaches only the hook.** Every `PreToolUse` input from a subagent carries `agent_id`;
  the main session's input carries none.
- **The command environment holds only the session.** It has `CLAUDE_CODE_SESSION_ID`, the same for
  main and subagents, and no agent id.
- **Rewriting the command is rejected.** A `PreToolUse` hook can rewrite a command through
  `updatedInput`, but the rewritten command no longer matches the user's allow rules: `touch x`
  under `Bash(touch:*)` ran unprefixed and was refused prefixed.
- **The agent's prompt is readable.** A subagent's transcript is
  `<session transcript dir>/subagents/agent-<agent_id>.jsonl`, and its first line is the prompt the
  agent was spawned with. A hook can read it from the agent's first tool call. This is an observed
  layout, not a documented contract.
- **When the stop events fire:**
  - `SubagentStop` fires for a foreground subagent, for a worktree subagent (its `cwd` is the
    worktree) and for a background subagent.
  - It does not fire for a subagent stopped with `TaskStop`.
  - `Stop` fires at the end of every main-session turn.
- **A stop can be blocked once.** `{"decision":"block","reason":...}` on `SubagentStop` made the
  subagent run the reason's instruction. The next `SubagentStop` came with `stop_hook_active: true`.

## Decision

1. **The brief carries the task; the hook claims it.**
   - `tp brief <id>` puts one marker line, `tp-task: <id>`, in the brief.
   - At a subagent's first tool call, the plugin's `PreToolUse` hook reads the agent's prompt. For
     each marker it finds, the hook runs `tp claim <id>` itself with the claimant
     `{session, agent}`, and records the agent in local state keyed by git's common dir, so every
     worktree sees the same state.
   - The claim time is the agent's first action. Nobody has to remember the claim.
   - A brief without the marker falls back to today's behaviour.
2. **An agent cannot stop with an unexplained claim.** The plugin's `SubagentStop` hook looks up the
   wip tasks this agent claimed that carry no `waiting`.
   - If it finds any, it blocks the stop once and names the two ways out: `tp done <id> ...`, or
     `tp set <id> waiting="<what>"`.
   - With `stop_hook_active` set, it lets the stop through.
   - The main session gets a non-blocking note instead, because its `Stop` ends every turn.
3. **An unclaimed close is dated by its agent.**
   - When an agent closes a task it never claimed, `started_at` is that agent's first recorded
     action, not now, and `duration_source` is `"agent"`.
   - When one agent works several tasks, this is an upper bound, and `tp report` says so.
   - An explicit claim is never overwritten.
4. **Waiting is measured.** Setting `waiting` stamps `waiting_since`, and clearing it adds the
   interval to `waited_minutes`. `tp report` adds `waited_minutes` and `active_minutes` next to
   `actual_minutes`.
5. **The pool stays full.** `run_max_parallel` becomes the coordinator's cap too.
   - When the coordinator's turn ends (`Stop`), the hook counts the live agents holding tp claims.
   - If that count is below the cap and `tp ready` lists tasks, the hook blocks the stop once. It
     names the ready tasks and the free slots, each with the `tp brief` command to spawn.
   - A task held back for a reason that is not a task is written as `waiting` on the task, or as a
     dependency when the reason is a task. Then `tp ready` no longer offers it, and the reason is on
     disk instead of in the coordinator's head.
6. **`tp resume` names abandoned claims.** A wip task without `waiting`, whose agent has no
   `SubagentStop` and whose transcript has not been written for longer than the task's estimate, is
   reported as possibly abandoned. A killed agent fires no hook, so this report is its only cover.

## Alternatives rejected

- **Refuse writes while no task is wip.** This blocks the coordinator's own claim commits and its
  measurement output.
- **Prefix commands with the identity.** It breaks the user's allow rules (see above).
- **Derive start from commits.** Commits carry no start time (see above).
- **Match commit files to `touches`.** Tasks share files, and measurement tasks touch none.
- **Bind claims to a worktree marker.** Work is not always in a worktree.
- **Move every coordinator to `tp run`.** That is the existing deterministic path, but a coordinator
  that briefs agents by hand and waits on other sessions' decisions is not one `tp run` drives today.

## Non-goals

- Guessing an agent's task when its brief carries no marker and it never closes.
- Time inside a background run beyond `waiting`.
- `tp run` units: the driver claims for its children already, and `stop-role-incomplete.sh` keeps
  its job.

## Consequences

- The hook change bumps `plugin.json`.
- REFERENCE.md gains `claimed_by`, `duration_source: "agent"`, `waiting_since`, `waited_minutes`
  and `active_minutes`.
- Board clients should prefer `active_minutes` and name `"agent"` durations as upper bounds.
- A coordinator now has its turn extended while slots are free and ready tasks remain. That is the
  cost of keeping the pool full.
- The prompt read depends on Claude Code's transcript layout. A test pins the layout, so a change
  fails loudly rather than silently claiming nothing.
