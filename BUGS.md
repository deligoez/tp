# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **After a land request leaves, nothing tells the agent's worktree or the coordinator's session what
  happened** (v1.29.0, spec/backlog/worktree-land.md item 4, "a failed landing reopens the task as
  solo with its claim cleared"). Found by the v1.29.0 review rounds, reproduced in scratch repos with
  `landRepo`'s fixture (`internal/cli/land_test.go`); the fix is a change to that decision, not a
  local repair:
  - A request `tp land` drops (the task was closed by hand, or a crash fell between the close and its
    outcome record) writes no outcome, so the agent's `tp done --auto-commit` then closes its own
    copy.
  - A second request the agent writes while `tp land` gates the first is deleted with it; the second
    commit never lands, and the agent is told "already landed".
  - `tp set <id> waiting=` in the agent's worktree after a landing writes the local copy (only `tp
    done` and `tp commit` read the outcome).
  - A coordinator whose only claim a landing reopened is no longer a coordinator: `tp pool
    --stop-hook` prints nothing, and only `tp land`'s own output names the fresh brief.
  - The child close's log, which `reopened[].log` names, still carries `tp done`'s `--skip-gate`
    hint.
  - A `landing` task holds no slot while its agent still runs, so the pool can briefly run one agent
    over `run_max_parallel`.
  - `tp done --batch` entries with `commit_shas` skip the off-branch refusal the single-id close has.

## Fix track

## Decided elsewhere, not a fix

- **An auto-detected file list beyond 10 is cut per code role, and an inline spec excerpt stops at
  10 KB.** A named `--affected-files` list is not cut since v1.15.0, and every code role's round says
  `graded N of M` in `--record`, `--status` and `next_action`. The spec cut drops prose only: each
  spec-coverage checklist item carries its own `text`, and the marker names the path to read the rest.
  Measured on tp's shipped specs (`spec/1.20.0-measurements.md`): 21 of 22 audited specs exceed the
  cut, 65% of spec-coverage rows grade a section past it, and those rows hold 15 FAILs and 78
  PARTIALs, so the rest was read. The code roles' cut was measured from history in
  `spec/1.21.0-measurements.md`: half of their findings sit beyond rank 10 and are real, and yield does
  not follow churn. The constructed full-list round ran in v1.21.1 and showed one long prompt skims;
  the depth now comes from v1.22.0's task audit of every closed task's own files, and the cut stays
  (`spec/undecided.md`, *The divisible round*, closed).
