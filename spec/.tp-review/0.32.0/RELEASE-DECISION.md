# v0.32.0 — release decision

`tp audit spec/0.32.0.md --status` reports `converged: false`. This release ships anyway, on the
reasoning below. Recorded so the decision is auditable rather than implicit.

## What the eleven rounds actually say

| | spec-coverage | FAIL, all roles |
|---|---|---|
| round 1 | 54/55 | 0 |
| rounds 2–11 | **55/55 every round** | **0 every round** |

`spec-coverage` is the only auditor role that measures conformance to the spec: it alone takes the
spec-derived checklist. It has been clean for ten consecutive rounds. No round in the entire audit
produced a single FAIL.

## Why `converged` is nevertheless false

tp counts **every** non-PASS row against audit convergence, whether the row concerns the spec or
the codebase at large. The other three roles in this repo's corpus (`go-safety`,
`maintainability-conventions`, `ax-contract`) are general code lenses; in a real codebase they
always find something, so under that predicate there is no fixed point. Rounds 3–11 were consumed
by error-hint wording, advisory channels, git scoping and flag hygiene — none of which
`spec/0.32.0.md` specifies. Each repair round also created fresh surface for the next round to
audit, which is why the finding count stopped falling monotonically.

This is a gap in tp, not a defect in the implementation: spec review has a severity-aware
`review_converge_on`; audit has no scope-aware equivalent. Closing it is v0.33.0's headline —
`audit_converge_on` plus a `scope` field (`spec` | `codebase`) on audit rows, so convergence counts
only spec-scoped rows while codebase-scoped rows accumulate as a visible backlog.

## The spec changed after review converged — disclosed, not hidden

`tp review spec/0.32.0.md --status` reports `stale: true` and `harness_stale: true`. The cause is
real and worth naming: review converged at round 12 (rounds 11 and 12 both free of blocking
findings), and **§2.6 was then rewritten during audit** (`ebfb3ea`). The recorded review rounds
therefore hash a spec text that no longer exists.

The rewrite reversed a decision the spec had made explicitly. §2.6 previously said the §2.3
warnings travel `output.Info` and that ordering them "would pin a channel no agent reads". Three
audit rounds of accumulating evidence showed the opposite: `output.Info` is silent whenever stdout
is not a terminal, so the feature's primary failure mode — a mistyped role id that leaves the role
active — was invisible to the only audience the feature has. The paragraph now says those warnings
travel `output.Notice`.

What makes this acceptable rather than a gap: the rewritten §2.6 was **audited five times against
the code** by `spec-coverage`, in rounds 7 through 11, 55/55 each round. The conformance role read
the new text, not the old. Re-running the review loop over one paragraph would have re-verified
prose that the audit had already verified against the implementation.

`nonblocking_open: 8` is the recorded set of low/medium review findings accepted with justification
under `review_converge_on=blocking`, which the policy permits once no critical or high remains.

## Findings accepted rather than fixed

Each is outside `spec/0.32.0.md`'s surface and is carried to v0.33.0:

1. **`output.Error` does not exit.** `runAuditStatus` emits `{"error":…,"code":3}` and still returns
   0 when `output.JSON` fails; `review_status.go` and `claim.go` share the shape. Reachable only on
   a stdout write failure, where EPIPE kills the process first. A three-file sweep, not an audit
   repair.
2. **~25 raw-stderr advisory sites outside the audit path** (`review_merge.go`, `set.go`, `add.go`,
   `commit.go`, `done.go`, `config.go`, `engine/configresolve.go`, `engine/discover.go`, …) still
   ignore `--quiet` while their converted siblings honour it. Judged acceptable by `ax-contract` in
   round 10: `audit.go`, `audit_roles.go` and both `--record` paths now hold zero raw stderr writes,
   so every advisory the audit loop emits per round honours `--quiet`; the remainder are one-shot
   errors on non-loop commands. Converting them changes six commands' stderr contract.
3. **A mistyped override *key*** (`enbaled:` for `enabled:`) is still silent in `tp review`/`tp audit`
   — `resolveRolePanel` discards `fm.Warnings`, and only `tp lint` consumes them. §2.1 calls it "a
   lint warning", so current behaviour matches the spec; surfacing it in the emission path is a new
   advisory, not a channel move.

## What the audit did find, and fix

Worth recording, because it is the argument for running the loop at all. Eleven rounds produced,
among others: a `--findings` path typo that made an audit round record as **clean** (convergence
integrity); roles told file contents were "complete and authoritative" when a read had failed;
`(diff: +0/-0)` presented to every role as measured fact on a committed tree; the wrong repository's
`CLAUDE.md` injected into prompts; a refused run leaving `.tp-review/<spec>/` on disk in **both**
commands; and `--compact` stripping 0.5% of the audit payload against a documented ~40%.
