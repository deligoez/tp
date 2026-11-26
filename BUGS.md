# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

None open.

## Fix track

- **Two next-step texts still name the default inbox after a named-inbox round.** `tp review <file>
  --resolve ... fixed` answers `next_step: run the next review round ... tp review <spec>, then tp
  review <spec> --record-inbox` (`reviewResolvedNextStep`, internal/cli/review_resolve.go), and `tp
  resume`'s blocker commands are built the same way (engine/resumeblockers.go, resumepayload.go).
  Repro: record a review round with `--record-inbox --inbox /x`, resolve its finding fixed: the
  step omits `--inbox /x`. The test asserts the step names the stored inbox. (`spec/1.36.0-measurements.md` §3)
- **The task-audit flat-layout hint's "every role missing" condition is untested.** A round where
  some role files sit in the flat layout and some in the spec folder still gets the plain refusal,
  but weakening `len(missing) == asked` to `len(missing) > 0` in `taskAuditRoleRows`
  (internal/cli/audit_task.go) keeps every test green. The test records a mixed round and asserts no
  move hint. (`spec/1.36.0-measurements.md` §3)

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
