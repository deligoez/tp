# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **A land request does not carry who made it, so after a takeover the first request wins.** A request
  is stamped with the claim the holder's copy carries when it is written, not with the requesting
  agent's own spawn, which the worktree does not know. Since v1.29.1 a second worktree's `tp done`
  is refused while the first one's request waits, which protects the fresh agent when the old one
  asks late (`TestALateCloseAfterATakeoverLeavesTheFreshRequest`). In the reverse order (the agent a
  takeover replaced asks first, then the fresh agent), the old agent's work lands under the fresh
  claim and the fresh agent is refused. Closing it needs the request to carry the requester's
  identity (the spawn reaching the agent's worktree, for instance through the brief), a choice with
  user-visible consequences.

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
