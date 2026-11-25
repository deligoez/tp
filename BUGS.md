# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **The next-round text cannot name a named inbox.** After a round recorded with `--record-inbox
  --inbox <dir>`, review's and audit's `next_action` name `tp review <spec>` and `--record-inbox`
  without `--inbox <dir>`, so following it writes into the checkout's default inbox. Which inbox a
  round came from is stored nowhere. Choice: record the inbox path on the recorded round (one
  optional field; recommended), or leave the text generic. (`spec/1.34.0-measurements.md` §5)

## Fix track

- **`tp brief` with no id hides overlaps.** When every ready task overlaps a held task's `touches`,
  `tp brief` (no id) says only "No ready tasks" (`internal/cli/brief.go`, the no-id path). Test: two
  ready tasks with overlapping `touches`, one claimed with `--spawn` in a worktree; `tp brief` should
  name the other under `overlapping {id, held_by}`, as `tp next` does since v1.34.0.
- **Task-audit `--record-inbox` gives no flat-layout hint.** A round emitted by v1.33 left its role
  files flat in `<inbox>/task-audit-inbox/`; `tp audit --task-audit … --record-inbox --inbox <dir>`
  exits 3 telling the person to re-run every role, while review and audit name the folder to move the
  files into (`d2491c5b`). Test: flat `round-N-<role>.ndjson` files under a named inbox; the hint
  names the `<base>-<hash>` folder.
- **A set-aside unit branch leaves `conflict_branch` stale.** Under `tp run`,
  `engine.setAsideUnitBranch` (`internal/engine/driver_pool.go`) renames an earlier unit branch with
  `git branch -m` but does not update the task's `conflict_branch`, so the recorded name can point at
  the new attempt's branch. No work is lost (`conflict_sha` no longer matches, so `tp land` keeps it),
  but the kept branch it names is the wrong one. Test: a conflict recorded, then a run sets the branch
  aside; the task's `conflict_branch` names the renamed branch.

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
