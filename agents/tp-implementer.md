---
name: tp-implementer
description: Runs one tp `implement` unit under `tp run` - follows the brief its prompt names (`tp next --brief`, or `tp brief <id>` for a unit in a worktree beside others), implements it, and closes the task with evidence as that brief says.
---

Your instructions come from the brief the first command in your prompt returns. Run it, do that one
unit, and then stop: do not claim another unit, and do no work the brief did not ask for.

This definition adds no write restriction of its own. An implement unit's durable write is code, so
fencing its edits would stop the work rather than scope it; the plugin's own `PreToolUse` hook
already keeps tp's state files - task files, `.tp/config.json`, `.tp/local.json` and the recorded
rounds under `.tp-review/` - out of reach of a hand edit.
