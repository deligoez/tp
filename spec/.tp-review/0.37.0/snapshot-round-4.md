# tp v0.37.0 — Audit convergence

> **Rewritten after review round 3.** Rounds 1–3 each reported the same class: a choice this spec
> owed had not been made. A review round cannot make a choice — it can only observe that none was
> made — so the operator made them, and this file is what remains once every undecided question is
> either decided or moved out. `spec/0.46.0.md` §8 carries what moved. The prose recording which
> drafts were withdrawn is gone with them: it had grown to 77 of the file's 306 lines, which is
> `spec/0.43.0.md` §2's repair-verbosity class measured on this file.

## 1. Overview

`review_converge_on` lets the review loop stop counting advisory findings against convergence. The
audit loop has no equivalent: `--record` counts every row whose `status` is not exactly `PASS`, at
any severity. Two cycles in this repository and one field cycle in another project each ran long
past the point where the only surviving findings were advisory.

Measured on v0.35.0's own audit corpus — 22 recorded rounds, 42 non-`PASS` rows: **3 `error`,
22 `warning`, 17 `info`**. 93% of what kept that phase open could not have blocked a release.

This release ships the field, its resolution, its validation and its documentation. **It does not
change any existing project's behaviour**, because the default is the behaviour tp has today.

## 2. `audit_converge_on`

**The field.** A workflow field taking `all` or `blocking`, resolved through the same precedence
every workflow field uses (CLI > env > task override > project config > built-in), and rejected with
the same hint machinery when the value is neither. `engine/reviewconvergeon.go` is twenty-four lines
— two constants, a hint string, and a predicate shared by the setter and every consumer — and this
is its twin.

| value | a round is clean when |
|---|---|
| `all` | no non-`PASS` row survives, whatever its severity |
| `blocking` | no surviving non-`PASS` row carries a blocking severity |

Both predicates are over non-`PASS` rows. Measured across v0.35.0's and v0.36.0's audit rounds,
**every one of 2,913 `PASS` rows carries `severity: null`**, so a predicate phrased over "any row of
any severity" would read the wrong population.

**Blocking severity is `error`, and that is stated here rather than left to a table.** Audit rows use
`error | warning | info`. `review_converge_on`'s set is `critical | high` over the review vocabulary;
the two vocabularies do not overlap and `engine.SeverityRank` knows only the review one, so **the
audit predicate does not reuse it**. An implementation that ranks audit severities through
`SeverityRank` ranks every audit severity as unknown, and ranks the legacy rows of §2.2 above a
current `error`.

**An unrecognised severity is treated as blocking.** Not rejected, not ignored. This is the rule
v0.31.0 §4.1 states for the review side and it is the only rule that fails closed: a row tp cannot
grade is a row tp must not stop counting. It is stated normatively here because two earlier drafts
of this section left it to be inferred, and both inferred it wrong.

**Default: `all`.** The behaviour tp ships today, so no existing project changes. §2.1 is why the
default is not `blocking`.

**`severity` on a non-`PASS` row is a self-declared label and this release does not pretend
otherwise.** `audit_schema.go` renders the requirement into every prompt and validates nothing;
`audit_record.go` validates `category` alone; nothing on the audit path reads `severity` at all. An
operator setting `blocking` is choosing to gate on the audited agent's own judgement, which is the
same standing decision as `--skip-gate` and is why the field is opt-in rather than the default.
Making that label checkable is `spec/0.46.0.md` §8's subject, not this release's.

### 2.1 Why the default is `all`, measured in both directions

| cycle | audit rounds | converges under `blocking` | converges under `all` |
|---|---|---|---|
| v0.35.0 | 9, converged | round 3 | round 9 |
| v0.36.0 | 13, never converged | round 3 (r2, r3 carry zero `error`) | never — no round reaches zero |

The saving is real: v0.35.0 spent six of nine rounds on a population that was 93% advisory.

**And the column this table did not have is what decides the default.** It asks *when the loop
stops*. It never asked *what was still to be found after it stopped*, and that column reads `error`
in both rows — v0.35.0's round 4 carries `{error 1, warning 2, info 1}` one round after `blocking`
would have halted, and v0.36.0's rounds 7, 12 and 13 each carry `error` rows. **`blocking` would
have shipped over an `error` row in 2 of 2 measured cycles.** That is decisive against it as a
default and is not an argument against the field: an operator who sets it is accepting that trade
with the numbers in front of them.

### 2.2 The 67 legacy rows

Sixty-seven non-`PASS` rows recorded across v0.29.0–v0.32.0 carry severities outside
`error | warning | info` — `high` 16, `medium` 15, `low` 36 — in fifteen round files. Under the
default they are counted as they always were. Under `blocking` §2's fail-closed rule counts them as
blocking, which is the conservative answer and needs no migration, no rewrite of recorded rounds and
no validation at the sink.

## 3. A changed spec ends the streak

`consecutive_clean` is a claim about a spec. When the spec changes, rounds recorded against the old
text no longer support the claim. The review phase already acts on this — a `fixed` resolution forces
a further round, and `tp resume` raises a `spec-stale` blocker — and the audit phase does not.

**Mechanism.** `consecutive_clean` is not stored: it is derived by `engine.ConsecutiveClean` walking
`AuditRounds`. That function is shared with review's `--status`, so this release **adds**
`engine.ConsecutiveCleanSince`, which stops the walk at the first `spec_hash` boundary, and the audit
path calls it. `engine.Converged` folds `ConsecutiveClean` internally and is reached by
`audit_record.go`, `run_status.go` and `budget.go`; it gains the same audit-side variant, because a
reset that changes two printed integers while `converged` keeps the pre-reset answer is not a reset.

**Prospective only.** The reset applies to rounds recorded from this release forward. An earlier
draft made it retroactive over rounds already on disk; simulated against v0.35.0's nine rounds, which
carry **eight distinct `spec_hash` values**, that moves `blocking` from round 3 to round 8 and `all`
from round 9 to never — falsifying §2.1's own table and reducing "six rounds saved" to one. A
mechanism that rewrites the history it is justified by is not a mechanism.

**What this is not.** `spec/0.45.0.md` §3's `--reconcile` records *why* a spec moved. This records
only *that* it moved, and only for the streak.

## 4. Non-Goals

1. **The `--check` divergence.** `consecutive_clean` counts every role while the shipping rule reads
   `spec_coverage_clean_rounds` plus no-FAIL. That is role scoping, not severity scoping, and
   severity parity does not close it: on v0.36.0's rounds 12–13 every `error` row belongs to
   `maintainability-conventions` while `spec-coverage` was clean four rounds running, so a
   severity-parity `--check` still exits 1 for exactly the rounds the gap is about. `spec/0.46.0.md`
   §8 owns it and this release does not claim to narrow it.
2. **A disposition that stops blocking.** `--resolve` marks a row and convergence still counts it.
   Fixing that needs a choice between recomputing `clean` live from rows and persisting the
   disposition into the next round's input, and it collides with `dedupAuditRows` keeping the first
   `(role, item_id)` row and comparing nothing. `spec/0.46.0.md` §8 records it.
3. **Making `severity` checkable.** §2 states plainly that it is not. `spec/0.46.0.md` §8.
4. **Review's convergence.** `review_converge_on` is unchanged. §3 adds a function rather than
   altering the shared one precisely so review's `--status` cannot move.
5. **A fourth row status.** `AuditRowIsPass` stays byte-exact on `status == "PASS"`.

**One shipped guard is deliberately reversed.** `internal/cli/audit_signal_test.go`'s
`TestAuditGate_NoEscapeHatchFromTheGate` asserts that `audit_converge_on` is an *unknown* workflow
field. Shipping the field reverses that assertion, which is the point of the release; the guard's
other assertions — that a `wontfix` disposition does not clear an open finding, and that `--check`
stays shut over a disposed finding — are untouched, because Non-Goal 2 keeps them true.

## 5. Tests

1. `audit_converge_on` resolves through the full precedence chain, and an invalid value is refused
   with the same hint shape `review_converge_on` uses.
2. Under `all`, a round with one surviving `info` row is not clean. Under `blocking`, it is.
3. Under `blocking`, a round with one surviving `error` row is not clean.
4. Under `blocking`, a row whose severity is absent, `null`, or outside the enum is counted as
   blocking. The fixture uses one of the 67 recorded legacy rows, so the case is not hypothetical.
5. Replaying v0.35.0's nine recorded rounds reproduces the §2.1 row — `blocking` at 3, `all` at 9 —
   and replaying v0.36.0's thirteen reproduces `blocking` at 3 and `all` never.
6. `engine.ConsecutiveCleanSince` stops at a `spec_hash` boundary and `engine.ConsecutiveClean` does
   not. Three clean review rounds at hashes A, A, B still report `consecutive_clean: 3` — the
   assertion that catches an implementation which changes the shared walk.
7. The audit-side `Converged` variant agrees with the reset: a hash boundary between the only two
   clean rounds leaves `converged` false, and `--status --check` exits 1 on the same input.
8. The reset is prospective — rounds already on disk at upgrade keep the `consecutive_clean` they
   had.
9. `TestAuditGate_NoEscapeHatchFromTheGate`'s two surviving assertions still pass.
