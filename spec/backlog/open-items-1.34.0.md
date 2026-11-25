# Close the items 1.32.0 and 1.33.0 left open

The open lists in `spec/1.32.0-measurements.md` §4 and `spec/1.33.0-measurements.md` §5, in one
release. The parked converged-payload decision stays parked.

## Decision

- **`tp now`.** `next_action` stays the agent's line, documented; a task waiting to land is listed
  under `landing` only; `--epic` narrows `epics`, and done epics stay on the map; several task files
  and none active is a need, not an exit 3; bound task files are read once per poll.
- **Worktrees and land.** `tp claim` and `tp next` screen `touches` as `tp brief` does; a landed
  task's earlier conflict branch is pruned when nothing is left in it.
- **Audit, review and run.** A named `--inbox` keeps two specs apart; review's latest-round blocking
  `next_action` names the repair; `audit-round-prep.py` diffs without renames; `tp run --status` gains
  `last_finished`.
