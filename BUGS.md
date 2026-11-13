# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

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
- **A milestone that waits on an external trigger counts as open.** `opens_after: []` is how a plan
  says a milestone opens on something outside the graph (a vendor go-ahead, a date, a contract), and
  tp reads an empty list as "opens after nothing", so the milestone is open today: `tp epic ready`
  offers its epics as startable and `tp epic status` counts its blocked epics as blocked in an open
  milestone, while nothing in the plan has happened yet. Repro (v1.14.0, scratch git repo): tracks
  `[["A1","A2"],["T1"]]`, `opens_after` `{"T1": []}`, one A1 epic, one T1 epic with no dependencies
  and two A2 epics depending on it; `tp epic status` reports T1 open and `tp epic ready` lists the
  T1 epic beside the A1 one. On a plan where open-milestone epics wait on such an epic, the
  dashboard's Waiting epics shows it as a blocker of open work. The open question is the model, not
  the page: should such a milestone count as open, should the plan express the trigger with a
  condition epic (an operator decision the milestone's epics depend on, which `ready` and `status`
  already honour), or does tp need a state of its own for a milestone that waits on something
  outside the graph? Any answer changes `ready` and `status`, not only the dashboard, so it is the
  operator's choice. Recorded from the 1.14.1 field run; the dashboard keeps reading tp's own notion
  of open.

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
