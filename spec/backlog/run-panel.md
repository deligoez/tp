# tp — A panel that shows the run

**Decided** by the operator on 2026-11-25: the panel ships inside tp's plugin, in 1.32.0, once the two
measurements in §4 hold. Raised by the operator during the 1.31.0 run.

## 1. Context

`tp run` spawns its own runner processes, one per unit and up to `run_max_parallel` at once, each in
its own worktree. Claude Code lists only what its Agent tool started, so a person watching a run sees
one background shell and nothing of the units: which task each slot holds, how long it has run, which
exited and how. Today the only window is `tp run --status` typed by hand, and the unit logs under
`.tp/runs/<run_id>/`.

What Claude Code's documentation offers (read 2026-11-25): no API, environment variable, hook event
or manifest field registers an outside process as a task; a plugin may ship a **mod** (a `modules`
entry in `hooks/hooks.json`) that renders a panel, from Claude Code v2.1.287; a plugin **monitor**
streams a command's lines into the model's context, not the person's view; subagents started by the
Agent tool show in the panel, but a hook cannot make the session start the next one.

## 2. Decision to make

Ship a mod in tp's plugin that renders the run as a panel: one row per slot with the task id, the
attempt, the elapsed time and the last exit, plus the run's phase, units done and stop reason, read
from `tp run --status` in the session's project. Every project that installs the plugin gets it; the
driver and its gate, budget and retry logic do not change.

## 3. Alternatives

- **Coordinator mode**: the session spawns each unit with the Agent tool, so the units show natively.
  The next spawn is the model's choice, not the driver's, and the Agent tool's worktree starts from the
  default branch rather than `HEAD`.
- **A monitor**: costs the model's context per line and shows the person nothing.

## 4. Measured (2026-11-25, Claude Code 2.1.332)

- A mod refreshes on a timer: `$.clock.every` runs a callback on a period, and `$.process.run`
  runs `tp run --status` from it. The pane redraws when the value it reads changes.
- A module in an installed plugin loads with no setting (a probe plugin installed from a local
  marketplace ran its `session.start` hook). Claude Code 2.1.280 skips the module with a debug line
  (early access there), and 2.1.150 and 2.1.280 both pass `claude plugin validate` on tp's plugin
  with `modules` and `types` in it: the shell hooks run as before. Agents that use the skill without
  the plugin get no panel.
- `tp run --status` names only the last unit, so it gains `running`: the attempts the run holds now,
  each with its task, kind, attempt and start time.

## 5. Non-goals

- Changing how `tp run` spawns or tracks units.
- Controls in the panel (stop, retry): the panel reads, it does not drive.
