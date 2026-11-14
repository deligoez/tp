# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

## Fix track

Found by the audit trial of 2026-11-14 (`spec/1.21.1-measurements.md`) on an old commit and each
reproduced at v1.21.0 with a built binary.

- **`tp config` exits 0 with project/default values when the task file does not parse.** Repro: truncate a task file that sets `workflow.review_max_rounds: 7`, run `tp config --json --file <it>`: exit 0, `review_max_rounds` 3, no error, and the comment at config.go:171 claims a re-read that never happens; the failing test asserts exit 3 with the parse error, as `tp list` gives.
- **`tp resume <spec>` treats an unparseable adjacent task file as an empty one.** Repro: truncate `spec/s.tasks.json`, run `tp resume spec/s.md`: exit 0, phase `review`, next action `tp review spec/s.md`, while `tp resume` with no argument exits 3; the failing test asserts exit 3 naming the parse error, and that only `os.IsNotExist` gives the empty task set.
- **`tp config --quiet` still prints config warnings.** Repro: `.tp/config.json` with `"review_max_rounds":"seven"`, `tp config --json --quiet` prints `warning: workflow.review_max_rounds: …` to stderr, where `tp validate --project --quiet` prints nothing; the failing test asserts empty stderr under `--quiet`, with the warnings routed through `output.Notice` (config.go:186), and the duplicate doc comment at config.go:50-51 dropped.
- **The surface guard's constant and messages name releases it no longer pins.** `nongoals_surface_test.go` keeps `v100Surface` (line 54) and says "v0.34.2 shipped" in its failure messages (155, 160), though line 26 requires a rename to the pinned tag and the list holds v1.3.0+ surface; the fix renames the constant to the current tag and makes the messages name the baseline rather than a version (text only, no red test).
- **`tp epic review --history --of-epic` names no file when a slice's snapshot does not parse.** Repro: corrupt `.tp/epic-review/<key>/snapshot-round-1.json` of a recorded slice, run `tp epic review --history --of-epic e1`: exit 3 with `epic review snapshot of round 1: …`, while the hint says to repair "the file the message names"; the failing test asserts the message holds the snapshot's path (wrap in `epicRoundEdges`/`readEpicSnapshotFile`).
- **`tp epic export` names no file when a bound file's review or ground state does not parse.** Repro: bind a task file to an epic, write `not json` into its `spec/.tp-review/<base>/ground-round-1.ndjson`, run `tp epic export`: exit 3 with `line 1: row is not a JSON object …` naming no file or spec; the failing test asserts the message names the bound task file (wrap epicexportjson.go:84 and :88 as :79 does), and the doc at :53-54 drops "in bound's order".
- **`tp export` prints `"tags": null` for a task file without tags.** Repro: a task file whose task has no `tags`, `depends_on` or `source_sections` key, then `tp export --json`: all three print `null`, and a section with no paragraph prints `"paragraphs": null`; the failing test asserts `[]` for each, per §6.2, by normalising in `ParseTaskFile` (or in `exportTask`) as `WriteTaskFile` already does.
- **`tp export` names no file when a recorded ground round does not parse.** Repro: write `not json` into `spec/.tp-review/<base>/ground-round-1.ndjson` (or garbage into `floor-ground-round-1.txt`) and run `tp export`: exit 3 with `line 1: row is not a JSON object …`, no round and no path, though the hint says "repair the file the message names"; the failing test asserts the message holds the file's path, wrapped in `exportGroundRound` (taskexportloops.go:175-182).

## Decided elsewhere, not a fix

- **An auto-detected file list beyond 10 is cut per code role, and an inline spec excerpt stops at
  10 KB.** A named `--affected-files` list is not cut since v1.15.0, and every code role's round says
  `graded N of M` in `--record`, `--status` and `next_action`. The spec cut drops prose only: each
  spec-coverage checklist item carries its own `text`, and the marker names the path to read the rest.
  Measured on tp's shipped specs (`spec/1.20.0-measurements.md`): 21 of 22 audited specs exceed the
  cut, 65% of spec-coverage rows grade a section past it, and those rows hold 15 FAILs and 78
  PARTIALs, so the rest was read. The code roles' cut was measured from history in
  `spec/1.21.0-measurements.md`: half of their findings sit beyond rank 10 and are real, and yield does
  not follow churn. The next step is one constructed audit round with the full list, the operator's
  decision of 2026-11-14; the item is in `spec/undecided.md` (*The divisible round*).
