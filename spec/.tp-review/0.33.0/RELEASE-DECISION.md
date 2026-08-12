# v0.33.0 — release decision

`tp audit spec/0.33.0.md --status --check` exits 1 and `converged` is `false`. This release ships
anyway. The decision was the operator's, taken on the record below, and this file exists so that a
reader who finds the non-zero exit code can see what it was traded for.

## The numbers

| | |
|---|---|
| Audit rounds | 13 |
| **FAIL rows, all rounds** | **1** (round 10; fixed, and verified by the role that raised it) |
| `spec-coverage` | **112/112 in every one of the 13 rounds** |
| Open findings at ship | 4, all general-lens, all recorded below |
| Gate | `go test ./...` → 0, `golangci-lint run` → 0, `go test -race` clean |

`spec-coverage` is the only role that measures conformance to `spec/0.33.0.md`. Asked in round 13
for a verdict rather than another data point, it signed off: every table row, numbered item and task
acceptance criterion maps to real behaviour with an assertion that would fail the shortcut it was
written against. `ax-contract` closed round 13 at 10/10 — its first fully clean round — and
recommended shipping.

## Why `converged` is false

The audit never reached two consecutive clean rounds, and the reason is visible in the record rather
than hidden by it: **every round's repairs created the next round's findings.** Rounds 7–13 each
opened between three and nine new items, and a majority of the later ones were defects in the
repairs themselves. That is not a loop failing to terminate; it is a loop working on a surface that
each fix enlarges. The v0.32.0 lesson in CLAUDE.md describes this state exactly, and reserves the
call to the operator.

What made stopping defensible here is not fatigue but the shape of the last round: the two roles
that measure conformance and the agent-facing contract both came back clean, and the remaining four
findings are in the general lenses, on code paths the spec does not describe.

## What the audit found that had nothing to do with this release

The strongest argument for having run 13 rounds is that four genuine defects surfaced that predate
v0.33.0 entirely, each reproduced before it was fixed:

- **`tp commit` deleted every tracked `*.lock` in the repository.** `git rm --cached -- "*.lock"`
  matches across directories, so one commit recorded `yarn.lock`, `Gemfile.lock` and
  `sub/Cargo.lock` as deleted. The files stayed on disk, so the damage was invisible until a clone.
  Hidden for four releases because tp's own `commit_strategy` is `hc`, which never calls that path.
- **The file lock did not exclude.** `WithFileLockTimeout` removed the lock file on release, and
  flock binds to an inode: the next waiter opened the same path, got a new inode, and entered the
  critical section concurrently. Measured as 3 silently lost rounds per 100 concurrent records —
  and it affected *every* locked write in tp, task files included.
- **Torn task-file reads.** `WriteTaskFile` wrote in place while every query command reads lock-free:
  8 bad reads in 1115 concurrent `tp list` calls, each an exit 3 telling the agent to repair a file
  that was never damaged.
- **A fresh spec's first audit round could not be recorded at all** — emission wrote a snapshot with
  no index, and `--record` read that as corruption, advising deletion of a healthy directory.

## Open findings, accepted as v0.34.0 backlog

None is spec-scoped; none corrupts state, loses data, or blocks a documented workflow.

1. **`tp add --bulk` drops rows past a 64KB line and reports success** (`{"added":["b1"]}`, exit 0,
   warning on stderr only). The review/audit family was swept for this class; the bulk readers keep
   their own warn-and-continue contract, which feeds no gate. Named in `hint_coverage_test.go`.
2. **`tp import` writes the task file with no lock** — a silent lost update in 1 of 20 concurrent
   `import` + `add` runs. Atomicity is not mutual exclusion; the write needs `WithFileLock`.
3. **The `ExitValidation` channel inherits task-file advice.** A malformed `--record` NDJSON, and
   `tp <unknown-command>`, both exit 1 with `run 'tp validate' to audit the task file`; an unknown
   command should be exit 2. The hint guard covers `ExitFile` only.
4. **`lint.go` and `init.go` exit-3 sites want `specFileMissingHint`** — both take a spec, not a task
   file. Recorded in `taskFileCommands` with that reason rather than as a clean exemption.

`spec/0.23.0.md` §8 still says "tp never silently rebuilds the index", which this release
deliberately made false for the snapshot-only case. It is a historical spec and was left alone; the
living docs (`SKILL.md`, `REFERENCE.md`) were corrected.

## Honest note on this session's own work

Of the findings in rounds 11–13, most were defects in repairs made one or two rounds earlier — the
`os.Link` fallback that broke FAT volumes, an `O_EXCL` create that reintroduced the torn read, a
`.gitignore` call that clobbered users' custom entries, a lock pathspec whose glob matched nothing.
Three separate guard tests looked like guards and were not: one failed 1 run in 10 and was labelled
a non-detector, one passed against a nonsense pathspec because `.gitignore` satisfied its only
assertion, and one was the sole data race in the repo, invisible because the gate does not run
`-race`. Every one of those was caught by an independent role re-deriving the claim by mutation,
not by the author checking his own work. That is the argument for the loop, and it is the reason
the round count is not something to apologise for.
