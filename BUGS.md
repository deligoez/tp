# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Silent loss of work, or output that lies

- **`tp review --check` converges with roles that never ran.** Test: a 3-role panel whose rounds
  hold one role's rows is not clean. Repro: record two rounds of only one role's rows → converged.
- **Named `--affected-files` beyond 10 are cut per code role with no notice.** Test: a named list
  is graded whole, or the payload says `graded 10 of 25`. Repro: name 25 files → `truncated:false`,
  empty stderr, prompts carry 10.
- **spec-coverage sees only the first 10 KB of the spec, cut mid-character, with no notice.**
  Test: the payload states the cut and the cut lands on a rune boundary. Repro: a 110 KB Turkish spec.
- **A `tp done --batch` row refused for a validation error has already claimed its task and leaves it
  wip.** Test: after a refused row the task's status and `started_at` are unchanged.
- **A registered check still suppresses its class when no task file resolves.** No check runs then,
  so nothing verifies the class and no reviewer reports it. Test: with no task file, the prompts carry
  no "do NOT report" sentence for a registered class. (The not-run fix covers the case where the check
  runs and fails to start.)

## Wrong or missing guidance

- **`--merge` blames JSON format when every row misses a required field.** Test: the error names
  the field and the lines. Repro: three valid rows without `evidence` → "trailing comma or wrapping array".
- **The ground pairing refusal names no accepted tier and cites "(§4.1)"**, tp's own design section.
  Test: it lists the accepted tiers and cites no section; the prompt glosses `query` as any read-only
  command over the corpus.
- **`--record` with `--round` is refused by value, not by flag.** Test: `--round 1` with `--record`
  is refused like `--round 9`, and `--help` states it.
- **The review carry lists `[WONTFIX]` rows under "UNRESOLVED — DO NOT re-report".** Test: accepted
  rows sit under their own header.
- **`--role <x>` drops the regression prompt with `skipped_roles: []`.** Test: it is listed.
- **The audit Prior Round block carries no disposition or evidence**, so an accepted row is reopened
  next round. Test: rows carry both. An acceptance invisible to the next round lasts one round.
- **`tp review --status`'s `next_action` withholds a class by its registration alone**, without
  running the check, so a class whose check cannot run is still hidden there. Test: a registered
  `exit 2` check leaves its class in `next_action`.
- **`spec-stale` names no command that clears it.** Test: the blocker names emit + record.
- **`--peek` ignores WIP.** Test: it previews what plain `tp next` returns.
- **A mistyped `tp.lens` frontmatter key is silently ignored by `tp review`** (lint reports it).
  Test: review warns in the payload.
- **Acceptance bullets are split again on `; ` and `. `.** Test: a `- ` list gives one criterion
  per bullet; `tp add`/`tp import` report the count.
- **The ground scratch file `ground-r<N>.ndjson` is not unique per spec.** Test: it carries the base.
- **`tp add` in a directory with several task files says "no task file found. Use --spec".**
  Test: it names the candidates.
