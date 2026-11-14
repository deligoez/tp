# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

## Fix track


## Decided elsewhere, not a fix

- **A hand-recorded `tp review --record <file>` converges with roles that never ran.** Since v1.20.0
  (`spec/backlog/role-presence.md`) `tp review <spec> --record-inbox` is the proving form: it refuses
  while a role of the round, the ones a `--role` filter skipped included, has no file. Under `tp run`
  the oracle schedules the record unit only once every role's file parses. What stays open is
  `--record <file>`, empty or not, which cannot tell a role that found nothing from one that never
  ran. Closing it costs refusing an empty `--record` over an emitted round, or a `--status` field
  showing presence unproven: the operator's call.
- **An auto-detected file list beyond 10 is cut per code role, and an inline spec excerpt stops at
  10 KB.** A named `--affected-files` list is not cut since v1.15.0, and every code role's round says
  `graded N of M` in `--record`, `--status` and `next_action`. The spec cut drops prose only: each
  spec-coverage checklist item carries its own `text`, and the marker names the path to read the rest.
  Measured on tp's shipped specs (`spec/1.20.0-measurements.md`): 21 of 22 audited specs exceed the
  cut, 65% of spec-coverage rows grade a section past it, and those rows hold 15 FAILs and 78
  PARTIALs, so the rest was read. No audit round since v1.15.0 exists to count `M > 10`; whether to cut
  at all waits for a field round that shows one, in `spec/backlog/checklist-covers-what-changed.md`.
