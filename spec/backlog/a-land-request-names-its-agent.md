# A land request names its agent

Decision note (Decision track). Operator decision of 2026-11-24: option (a).

## Problem

A land request is stamped with the claim the holder's copy carries when the request is written, not
with the agent that wrote it: the agent's worktree does not know the Agent call's `tool_use_id` it was
spawned under, which only the hooks see. After `tp claim <id> --takeover` and a fresh spawn, the
replaced agent's `tp done` is stamped with the fresh claim. v1.29.1 refuses a second worktree's close
while a first one's request waits, so when the replaced agent asks first, its work lands under the
fresh claim and the fresh agent is refused (`BUGS.md`).

## Decision

tp mints the identity itself, at the point the agent receives its brief.

1. `tp brief`, run where an Agent spawn will claim from it (not inside `tp run`, not in a worktree
   whose task another tree holds), carries a fresh random claim token on a line of its own,
   `tp-claim-token: <token>`, beside the `tp-task:` marker, and as `claim_token` in its JSON. The
   close recipe tells the agent to pass it to `tp done` in its own worktree.
2. The plugin's Agent hook records the token in the claim it makes (`claimed_by.token`).
3. In an agent's worktree, `tp done` with a token-bearing claim requires `--claim-token` to match it,
   before the gate runs; a missing token exits 2 and a different one exits 4, and neither writes a
   request.
4. A request carries the token. `tp land` lands it only while the holder's claim carries the same
   token, and a waiting request whose token is no longer the claim's does not block the fresh agent's
   close.
5. `tp claim --takeover` clears the spawn and retires the token: a spawn from the replaced brief,
   which would share the replaced agent's token, is refused and pointed at a fresh `tp brief`.

## Alternatives

- **The token as a third field of the `tp-task:` marker.** Rejected: the 1.29.1 hook reads the third
  field as part of the task file path and refuses the spawn, so a new binary beside an older plugin
  would stop every spawn. A line of its own is ignored by an older hook: the claim then has no token
  and behaves as in 1.29.1.
- **Takeover drops the waiting request.** Simpler, but it does not tell the agents apart: the first
  request after the takeover still wins.
- **Document the gap.** Rejected: the operator's principle is that tp runs without human input after
  the spec, and here the coordinator could not tell which work is right.

## Non-goals

- A claim without a token (a spawn from an older brief or plugin, a claim made by hand) keeps the
  1.29.1 behaviour; the leniency is keyed on the holder's claim, not on the request.
- `tp run` units: the driver accounts for its own tasks and the hook does not claim for them.
- A token is not a secret: it tells two agents apart, it does not authenticate one.
