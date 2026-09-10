# The decomposition is built before it is imported — dropped

Dropped 2026-09-11. This file stays as a forwarding stub so that existing citations still resolve.

| what moved | where |
|---|---|
| building the change in a clone and running the suite before the first counted round | shipped as `skills/tp/SKILL.md` *Step 2: Review loop*, its opening rule (`4aa3e12d`) |
| an unrunnable quality gate found before the first close (field item #2) | `gate-sequence` — the gate is checked once at `init`/`import` |
| acceptance criteria split in ways the author did not see (field item #5) | `a-task-file-write-names-its-target` — a `- ` bullet is one criterion, and the count is shown at `add`/`import` |

**Dropped:** the per-task probe round — a prompt per task, a recorded probe, and three `tp validate`
warnings. What it had left to offer was finding tasks whose criteria already held at `HEAD`. That
does not pay for one subagent plus a full suite run per task.

Its `-measurements.md` sidecar stays as history. It has no recorded rounds under `.tp-review/`. The
old body cited its sidecar as `decompose-build-probe-measurements.md`, a name no file in this
repository's history ever had. The sidecar's own header still carries it, and is left as it is.
