# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **A coordinator cannot pause the pool hook short of holding every ready task.** From the v1.29.0
  review: a session told to stop spawning is asked once per turn while slots are free, and its only
  quiet answer is `tp set <id> waiting=...` per task, written to the shared task file and visible to
  every reader, `tp run` included. A per-session pause or cap kept out of the task file (under
  `.git/`, or a `tp pool` flag) is new surface.
- **A resumed session may not own its claims.** From the same review, not measured: if
  `claude --resume` gives a new `session_id`, the old session's claims are no session's, and the
  coordinator is asked nothing until it spawns again. Measure the id across a resume first.
- **A live agent spawned without a marker holds no pool slot.** `background_tasks` lists every
  running subagent, reviewers included, so counting each as a slot would count work that is not an
  implement unit; the marker is the way in (`spec/1.29.0-measurements.md`, *Left for later*).

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
