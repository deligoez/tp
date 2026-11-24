# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **On a committed tree, the audit commands `tp resume` and `tp run` hand out exit 4.** After implement
  commits everything, `tp resume`'s `next_action.command` is `tp audit <spec>` and the audit-role
  units are `tp audit <spec> --role <r>`. Both exit 4 with `no changed files detected` and a hint
  naming `--affected-from-tasks`, which works. The agent recovers from the hint, but each audit
  round costs one failed call per unit. The choice: should the emission fall back to the done tasks'
  files when the tree is clean, or should `brief_command` name `--affected-from-tasks`? Found by the
  v1.29.4 skill review. SKILL.md documents the re-run for now.
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
