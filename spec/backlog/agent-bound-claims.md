# Agent-bound claims

Status: decision, building.

## Problem

A coordinator that spawns one subagent per task from hand-written briefs, outside `tp run`, depends
on remembering two steps, and a field report shows both slipping:

- **Claims slip.** Subagents closed tasks nobody had claimed, so `tp done` claimed them implicitly
  and the task recorded about zero minutes. Other tasks were claimed hours before their agent
  started.
- **Agents stop with tasks still wip and no reason.** Nothing on disk separates a task being worked
  from one parked behind a merge, a long run or a decision.

**Git cannot stand in for claims.** A task committed through one `hc run` has the same author time
on its first and last commit. Work is not always done in a worktree. Other people's commits land in
between.

## What the harness gives

Measured in a scratch repository with `claude -p` and a logging hook:

- **The spawn is visible.** A `PreToolUse` hook with matcher `Agent` sees `tool_input.prompt` (a
  documented field) and `tool_use_id`. This held for a general-purpose agent, a background agent
  and a fork.
- **The stop is linked to its spawn.**
  - `SubagentStop` fires for a foreground subagent, a worktree subagent and a background subagent,
    and it carries `agent_transcript_path`.
  - Beside that transcript, `agent-<id>.meta.json` records the spawn's `toolUseId` and whether the
    agent `finished`. This is an observed layout, not a documented contract.
- **A stop can be blocked once.** `{"decision":"block","reason":...}` made the subagent act on the
  reason. Its next stop came with `stop_hook_active: true`.
- **Some events fire no hook, or fire twice.**
  - A subagent stopped with `TaskStop` fires no `SubagentStop`.
  - A background agent can fire `SubagentStop`, then resume and stop again.
- **The command environment has no agent id**, only `CLAUDE_CODE_SESSION_ID`. Rewriting a command to
  add one breaks the user's allow rules.

## Decision

1. **The brief carries the task.** `tp brief <id>` prints, on its own line, the marker
   `tp-task: <id> <task file>`, where the task file is relative to the repository root.
2. **The spawn claims it.** The plugin's `PreToolUse` hook with matcher `Agent` reads the marker in
   `tool_input.prompt` (a marker line, or the `marker` field of a brief's JSON pasted whole) and runs
   `tp claim <id> --file <task file> --spawn <tool_use_id>`, passing values as argv after checking
   each against the id and path shapes. A spawn carries one task; two markers refuse the spawn.
   - **An open task** is claimed now: `started_at` is the spawn, `duration_source` is `claimed`, and
     `claimed_by` is `{session, spawn}`.
   - **A wip task** keeps its `started_at`, and `claimed_by` moves to this spawn. This covers a
     coordinator that claimed before spawning.
   - **A refusal** (done, blocked, held by another spawn, unknown id, a lock timeout) exits 2 and the
     spawn does not happen. A holder that is gone is freed with `tp claim <id> --takeover`.
   - **A prompt without a marker** passes untouched.
3. **The agent cannot stop with an unexplained claim.** The plugin's `SubagentStop` hook reads
   `toolUseId` from the agent's `meta.json` and asks `tp claims --spawn <id>` for the tasks claimed
   by that spawn.
   - A task counts as accounted for when a copy of its task file that carries this spawn's claim has
     it done, or wip with `waiting` set. An agent in its own worktree writes its park to the copy
     holding the claim.
   - If any task is unaccounted, the hook blocks the stop once, naming `tp done <id> ...` and
     `tp set <id> waiting="<what>"`.
   - With `stop_hook_active` set, the stop passes.
   - With no `meta.json`, or no claims, the hook exits 0.
4. **Who commits the claim does not change.** The claim is a change to the task file, which the
   coordinator commits as it does today.

## Alternatives rejected

- **Claim at the agent's first tool call by reading its transcript.** It needs a hook on every tool
  call of every session, and a fork's transcript holds its parent's context.
- **Prefix commands with the identity.** It breaks allow rules.
- **Date an unclaimed close by its agent.** `tp done` cannot learn the agent. A claim at spawn makes
  the question moot.
- **Derive start from commits.** Commits carry no start time.
- **Refuse writes while no task is wip.** It blocks the coordinator's own claim commits.

## Non-goals

- A brief without a marker.
- A killed agent: no hook fires for it. Liveness and filling free slots belong to the decision on
  keeping the pool full.
- A resumed background agent that finishes after `waiting` was set: its stop passes, as `waiting`
  says the task is parked.
- `tp run` units: the driver claims for its children, and `stop-role-incomplete.sh` keeps its job.
- Measuring waiting time.

## Consequences

- There are two new plugin hooks. `plugin.json` is bumped, and the release check reads Hooks 4
  (SessionStart, PreToolUse, Stop, SubagentStop).
- REFERENCE.md gains `claimed_by` and the `tp claim --spawn` and `tp claims` forms. SKILL.md's
  parallel workflow says to spawn from `tp brief` output.
- A test pins the `meta.json` layout, so a change in Claude Code fails loudly rather than letting
  every stop through.
