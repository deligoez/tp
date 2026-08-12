# v0.33.0 — audit handoff after round 6

The implementation is complete (14/14 tasks) and the audit has run six rounds. This session stopped
at the subagent budget, not at a verdict. A fresh session picks up from `tp resume` (phase: audit)
and this file, which carries what the recorded state cannot: what each open finding is, why the
earlier repairs were made, and where one of them was wrong.

## Where the numbers stand

| round | findings | spec-coverage | FAIL |
|---|---|---|---|
| 1 | 4 | 112/112 | 0 |
| 2 | 5 | 112/112 | 0 |
| 3 | 6 | 112/112 | 0 |
| 4 | 3 | 112/112 | 0 |
| 5 | 4 | 112/112 | 0 |
| 6 | 4 | 112/112 | 0 |

**No round of this audit has produced a single FAIL**, and `spec-coverage` — the only role measuring
conformance — has been 112/112 in every round, twice under an explicit instruction to distrust its
own streak and attack the least-visible items. `tp audit --status` reports
`spec_coverage_clean_rounds: 6` and emits `divergence` naming the roles that still hold rows.

The gate is honest: `converged: false`, `tp audit --status --check` exits 1. The signal never
reached convergence arithmetic — `gate-guard-test` exists to keep it that way and was
mutation-proved.

## What the audit actually found and fixed

Worth recording, because it is the argument for having run six rounds rather than two:

- `countAuditFindings` restated the PASS predicate that `engine.AuditRowIsPass` encodes, with a doc
  comment requiring them to stay identical and nothing enforcing it. Now one call.
- Both record paths panicked with SIGSEGV when the state directory vanished mid-lock. Reproduced by
  an auditor with an external flock holder, fixed, and re-verified by the same repro.
- The repaired guard then carried the wrong hint (`defaultHint(3)` — task-file advice) and a comment
  claiming a hint it never attached. Now a `StateCorruptError` with `repair or delete <dir>`.
- `LockTimeoutError` fell through `exitStateError`'s untyped branch, losing exit 4 and its hint —
  the more likely arm during a parallel `--record` fan-out.
- **The important one:** `tp audit --merge <dir>` exited 0 with `merged_count: 0`, and feeding that
  empty output to `--record` recorded a round `clean: true`. An input tp never read advancing the
  audit gate — the same class as the v0.32.0 `--findings` path typo. Three loaders now abort.
- `skills/tp/SKILL.md` documented the merge step as `tp audit <spec> --merge …`, which the binary
  rejects with exit 2.

## Open findings (3, all pre-existing, none spec-scoped)

### 1. The false-clean sweep was incomplete — two of five readers still swallow a failed read

This is the one that matters, and it is a mistake this session made: round 5 fixed three loaders
and missed two. Both were found independently by `go-safety` and `ax-contract` in round 6.

- `readVerifyFindings` (`internal/cli/review_verify.go`) — `os.ReadFile` failure returns an empty
  set with **no diagnostic at all**. `tp review --verify <spec> --findings <dir>` exits **0** with
  empty stderr and emits a verifier prompt reading *"Previous review rounds produced 0 findings …
  If verifier finds 0 issues, review is complete."* Its `scanner.Err()` is still a warning too: a
  three-finding file with one 70KB line reports `previous_findings = 1`, silently dropping the rest.
  The same file through `mustParseFindingsFile` aborts exit 3 — two `--findings` consumers with
  opposite error contracts.
- `readSpecContent` (`internal/cli/review.go`) — its own comment claims *"A read error is
  propagated, never swallowed… Same class as parseFindingsFile"*, but `scanner.Err()` only warns.
  Reachable: `tp review --verify <spec> --findings f --spec-inline` exits 0 emitting a spec whose
  tail section is silently absent.

The fix is the one already applied three times: propagate or abort, and attach `ndjsonInputFileHint`
where the failure is a path. CLAUDE.md's own rule names this shape — *guard the value at the sink,
not at the entry point.*

### 2. The `--report` empty-directory split is justified wrongly, and the contract is untested

Commit `8f0d301` routed a bad `--report` path to exit 3 with the NDJSON hint while leaving no-args
and empty-directory at exit 2, on the reasoning that an empty directory is "the same class as
`--merge`'s own *at least 1 file required*". `ax-contract` refuted that: **`--merge` accepts no
directory at all**, so handing both modes the identical empty directory gives `--merge` exit 3 and
`--report` exit 2. The round-5 divergence was relocated, not eliminated. Only the no-argument case
is a genuine analogue.

It also noted a dead branch at `review_report.go` carrying exactly the right text — *"provide file
paths or a directory containing \*.ndjson files"* — on a condition `resolveReportFiles` can never
produce, and that `resolveReportFiles`/`reportPathError` ship with **zero test coverage**.

Recommendation from the role: route the empty directory through `reportPathError` too, delete the
dead branch, and pin the exit-code contract with a test.

### 3. `review_merge.go` reads at 64KB while its sibling writes at 1MB

`loadMergeFindings` uses a bare `bufio.NewScanner` (64KB) while `review_resolve.go` reads the same
artifact at 1MB and `audit_merge.go` bumps to 1MB. Reproduced: a 60KB finding merges, a 70KB one now
exits 3 — a findings file `--resolve` will happily rewrite cannot be read back by `--merge`. Making
it fatal was right; the cap asymmetry is the remaining half, and the abort attaches the *path* hint
when the path is fine.

## Backlog, deliberately not counted against this release

Each was recorded by the role that raised it, with its reasoning:

- `engine/lock.go` keys the lock file on `filepath.Abs`, which does not resolve symlinks, so `/tmp`
  and `/private/tmp` produce two lock files for one directory (go-safety, round 6).
- `internal/cli/review_merge.go` carries two stacked doc comments on `loadMergeFindings`, the first
  stale and contradicting the second — pre-existing since v0.29.0 (maintainability + spec-coverage,
  round 6).
- Over-length functions in `review.go` (`newReviewCmd` 166 lines, `buildReviewPrompts` 156,
  `runReview` 137). All predate this release; v0.27.0's `runReview` 346→123 refactor is the
  precedent for doing this as its own work (maintainability, rounds 4–6).

## What a fresh session should do

1. `tp resume` → phase `audit`. The `spec-stale` blocker is expected and disclosed: the spec was
   revised after review round 12 was recorded. `spec-coverage` has read the current text six times.
2. Fix finding 1 (both readers), then finding 2, then finding 3. All three use shapes already in the
   tree; none needs a new abstraction.
3. Run audit round 7 and have `go-safety` and `ax-contract` re-run their own repros against the
   fixes rather than accepting them on description — that discipline is what made rounds 3–6 worth
   running.
4. Two consecutive clean rounds converge it. If the pattern of rounds 4–6 repeats instead — each
   repair producing the next round's finding, with `spec-coverage` still clean and no FAIL anywhere
   — that is the state CLAUDE.md's v0.32.0 lesson describes, and shipping over it is a deliberate
   operator decision to be recorded in a RELEASE-DECISION.md beside this file.

## Honest note on this session's own repairs

Four of the six rounds were repair rounds, and two of them produced the next round's findings:
round 2's nil guard produced round 3's duplicated-error-literal and misleading-hint findings, and
round 5's loader sweep produced round 6's incomplete-sweep finding. Every repair fixed something
real; none was cosmetic. But the audit did not converge, and the reason is visible in the record
rather than hidden in it — which is what this release was built to make possible.
