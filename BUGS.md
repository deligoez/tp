# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Silent loss of work, or output that lies

- **A phase whose entire recorded history was hand-recorded, with no emission, still clears
  `spec-stale` at `--record`.** With no round that ever read text, staleness falls back to comparing
  the last recorded round's stored hash, which is the pre-v1.2.2 answer. The field case is closed: a
  converged loop always emits, and since v1.2.2 a round carries the hash of its own emission snapshot,
  so recording without emitting no longer clears staleness there. Closing this case too flips the
  seven tests that pin the staleness contract for hand-driven fixtures, so it is a decision.

- **The project config layer resolves from the process working directory, not from the task file.**
  `ProjectWorkflowOverride` discovers `.tp/` from `.`, so with `--file` (or `TP_FILE`) naming a task
  file in another project tree, that plan is driven by the *current* project's `quality_gate`, round
  caps, `commit_strategy` and registered checks: `tp done --file ../other/x.tasks.json` runs this
  project's gate command against that plan, the same silent-wrong-target family as the stale pointer
  fixed in v1.2.0. Seen from a test: with no `.tp/` beside the task file, resume still named the check
  gate, because discovery reached tp's own `.tp/config.json` from the test process. The option —
  resolve the project layer from the task file's directory — changes documented resolution precedence,
  so it is a decision.

## Wrong or missing guidance

- **Two specs with the same base in different directories share the ground scratch file** when
  grounded from one working directory (`x/a.md` and `y/a.md` both write `ground-a-r1.ndjson`; their
  state directories do not collide). Test: they get different scratch names.
- **`tp run` never spawns a regression unit.** A round the panel emits with a regression prompt is
  driven without it. Whether a run should grade regression, and as which unit kind, is the open question.

## Decided elsewhere, not a fix

- **`tp review --check` converges with roles that never ran.** A role that found nothing writes no
  rows, and `[]` records as no rows, so "ran and found nothing" cannot be told from "never ran" without
  a recorded per-role presence marker: a new state field and a contract, so a decision. The lax half was
  seen only in fixtures; schedule it on a field case.
- **Named `--affected-files` beyond 10 are cut per code role, and spec-coverage reads the spec only to
  10 KB.** The prompt header says `10 of 25` and, since v1.2.1, the spec cut is named in the payload
  and lands on a rune boundary. Whether to cut at all is `spec/backlog/checklist-covers-what-changed.md`.
