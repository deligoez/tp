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
- **An unresolved high review finding is converged away by empty rounds.** Convergence counts only the
  trailing clean rounds, each judged on its own rows, while the next round's prompts list the open
  finding under "UNRESOLVED findings from previous rounds — DO NOT re-report". So a high nobody fixed
  or disposed of is not raised again and `--status --check` exits 0. Repro (v1.12.0, scratch git repo,
  two-section spec, `tp init` and commit): emit round 1, record one row
  `{"role":"r","severity":"high","finding":"Rule 1 is ambiguous","location":"§2","evidence":"line 9","class":"ambiguity"}`;
  emit round 2 (its prompt carries the row under DO NOT re-report) and record an empty file; same for
  round 3; the spec is unchanged throughout. `tp review spec.md --status --check` exits 0,
  `converged: true`, `consecutive_clean: 2`, `next_action` "decompose the spec…". Failing test:
  that sequence must not converge. The fix changes a convergence rule (an open blocking row in any
  round of the current text blocks, or the carry list stops telling roles not to re-raise unresolved
  rows), so it is the operator's choice, not a fix-track change.

## Fix track

- **A gate with no effective members and no threshold is green, and nothing says so.** A milestone
  that holds only a condition epic, with no `gates` entry, has an empty gate; `tp epic gate <m>`
  reports `{"green": true, "members": []}` and every milestone opening after it is open at once, so
  `tp epic ready` offers their epics before the condition is done. Repro (v1.15.0, scratch git repo):
  milestones `[["A"],["TG","T1"]]`, `opens_after` `{"TG": [], "T1": ["TG"]}`, TG holding only
  `{"id":"trigger","condition":true,…}`, T1 one epic; `tp epic lint` reports 0 errors and 0 warnings,
  `tp epic ready` offers the T1 epic. Failing test: lint warns when a gate has no effective members and
  no threshold while a milestone opens after it.
- **`tp epic set gates.<m>.members=` takes a comma-separated list, not JSON.** `members=["x"]` is
  refused as a dangling reference to the literal `["x"]`, while `tags=` and `tag_checks=` take JSON.
  Failing test: the JSON array form is accepted, or the refusal names the expected form.

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
