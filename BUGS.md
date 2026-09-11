# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Silent loss of work, or output that lies

- **Recording an empty file with no fresh emission clears `spec-stale`.** A round nobody ran against
  the edited text then counts as the round that re-read it. Repro: converge, edit the spec, run
  `tp review <spec> --record empty.ndjson` without `tp review <spec>` first, and `stale` becomes
  false. The honest fix is `reconcile.md`'s emit-time hash (a round records the text it read), which is
  a decision; this line waits on it.

## Wrong or missing guidance

- **The audit carry's `changed_since` is second-granular.** It asks git for commits since the prior
  round's `recorded_at`, so a commit landing in the same second as that record counts as a change and
  the acceptance is not carried. Conservative (never a false carry) and rare outside scripted flows.
  Fix: when the round file is tracked, diff from the parent of the commit that added it instead of
  `--since`. Test: an init commit in the same second as round 1's record still lets an unchanged file
  carry.
- **`tp resume` can move to decompose while a registered review check fails.** Its phase and
  `next_action` read the loop verdict alone. Running the checks on every driver tick is a real cost, and
  doing it properly needs the check verdict stamped at `--record` (a new state field), so it is a
  decision. Bounded since v1.2.1: `tp import` refuses while a check fails and names it, so a run that
  moves on stops there. Test: a converged spec with a failing check is not reported as ready to import.
- **Review and audit role files collide across specs run by hand in one directory.** `tp review a.md`
  and `tp review b.md` both name `review-r1-<role>.ndjson` (the audit names follow the same shape).
  The names are documented and merge-globbed, so carrying the spec's base in them is a contract change.
  Test: two specs emitted in one directory name different role files.
- **The single-prompt modes carry no `skipped_roles`.** `--perspective regression --role implementer`
  (and `code-audit`, `documentation`, `testing`, `--verify`) drop a role with nothing saying so. Test:
  each names what `--role` narrowed away, as the panel does under `role-filter`.
- **`tp add` with a `--file`/`TP_FILE` naming a missing file gives the `--spec` advice and drops the
  path.** Test: the error names the path it could not open.
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
