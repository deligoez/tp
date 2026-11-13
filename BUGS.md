# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **Empty audit rounds converge past an earlier FAIL.** CLAUDE.md rule 13 says an audit ships with no
  FAIL in any round, but audit convergence counts only the trailing clean rounds. Repro (v1.15.0,
  scratch git repo): record audit round 1 with one `FAIL`/`error` row; round 2's prompt shows it under
  Prior Round to re-check; record rounds 2 and 3 empty; `tp audit spec.md --status --check` exits 0,
  `converged: true`, and round 3's prompt no longer mentions the FAIL. The record does say "graded 0
  of N". The fix is a convergence rule, the audit counterpart of `spec/backlog/carried-rows-block.md`;
  it gets its own note and measurement.

## Fix track

- **A hidden dashboard tab keeps its event stream open, and enough tabs starve the page's fetches.**
  Each tab holds one `EventSource` to `/api/events` for as long as it is open, hidden or not, and a
  browser allows six HTTP/1.1 connections per host. Repro (v1.16.0, the operator's report, not
  rerun here): with several tabs open, `/api/layout` and `/api/history` stay pending. Failing
  test: the page closes its stream on `visibilitychange` to hidden and opens it again, with the
  current state, when it is visible.
- **The first push waits for all three derivations, one after another.** A cold `/api/events`
  stream sends nothing until `Live()` has derived the epic export, the task file list and the
  summary in turn. Repro (v1.16.0, the synthetic graph of the graph-card prototype): the
  three take ~58, ~43 and ~54 ms, `Live()` 164–206 ms, and the first byte of a cold stream arrives
  at 215 ms on an idle machine; under load the operator saw ~2 s of blank page. Failing test: the
  live derivations run concurrently and keep their order.
- **A doubled slash in the address breaks the page.** Loading `http://127.0.0.1:7377//` makes
  `syncURL` call `history.replaceState` with `//#tab=…`, a protocol-relative URL naming another
  origin, which throws a `SecurityError`. Repro (v1.16.0): open the dashboard at `//`; the console
  shows the error and the address never follows the page. Failing test: the address the page writes
  stays on the page's origin under a `//` path.

## Decided elsewhere, not a fix

- **`tp review --check` converges with roles that never ran.** A role that found nothing writes no
  rows, and `[]` records as no rows, so "ran and found nothing" cannot be told from "never ran" without
  a recorded per-role presence marker: a new state field and a contract, so a decision. The lax half was
  seen only in fixtures; schedule it on a field case. For epic review, `--record-inbox` (v1.10.0)
  now catches a role that never ran, since it refuses a role's missing file; `--record <file>` still
  has the gap for a non-empty hand-made file.
- **Named `--affected-files` beyond 10 are cut per code role, and spec-coverage reads the spec only to
  10 KB.** The prompt header says `10 of 25` and, since v1.2.1, the spec cut is named in the payload
  and lands on a rune boundary. Whether to cut at all is `spec/backlog/checklist-covers-what-changed.md`.
