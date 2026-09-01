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

Measured on v0.35.0's own audit corpus — **nine** recorded rounds, 42 non-`PASS` rows: **3 `error`,
22 `warning`, 17 `info`**. 93% of what kept that phase open could not have blocked a release.

**Two deliverables, and only the first is a no-op by default.** §2 ships `audit_converge_on` — its
resolution, its fence, its validation and its documentation — and changes nothing for a project that
does not set it. §3 changes `consecutive_clean` and `converged` for **any** project whose spec moves
between rounds, opted in or not. An earlier draft of this paragraph claimed the whole release was
behaviour-preserving; it is not, and §3 is why.

## 2. `audit_converge_on`

**The field.** A workflow field taking `all` or `blocking`, resolved through the precedence workflow
fields actually use — **task override > project config > built-in** — and rejected with the same hint
machinery when the value is neither. There is no CLI layer and no `TP_<FIELD>` environment layer:
`engine/configresolve.go:261` merges exactly two layers over the default, and a repo-wide search for
`TP_REVIEW_CONVERGE_ON` returns zero. An earlier draft of this section wrote the five-layer chain,
which `spec/0.35.0.md` §7 records being removed after its audit reported across two rounds that the
top two layers cannot be verified because nothing implements them. The five-layer form is still
written in `CLAUDE.md`, which is where this draft copied it from; that is fixed with this release. `engine/reviewconvergeon.go` is twenty-four lines
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
operator setting `blocking` is choosing to gate on the audited agent's own judgement. Making that
label checkable is `spec/0.46.0.md` §8's subject, not this release's.

**Which is why the fence is not a detail of this release — it is what the release buys.** This is the
**first workflow field whose non-default value relaxes a gate**; `review_converge_on`'s weak value is
its own default, so nothing has needed this before. Without a fence, a unit stuck in an audit phase
under `tp run` could set `blocking` and end its own phase, which is the failure the whole
unattended-mode design exists to prevent.

`--skip-gate` is the precedent for the *decision* and not for the *mechanism*: it exits 2 under
`TP_UNATTENDED=1`, but the only workflow-write fence is `fenceWorkflowWrite(field string, requested
float64)` in `internal/cli/unattended.go:56` — **numeric by signature**, covering caps and command
fields. A string-valued field does not fit it. So this release either widens that fence to string
fields or adds a sibling, and `tp set --workflow audit_converge_on=blocking` exits 2 under
`TP_UNATTENDED=1` with the escalation hint the other fenced decisions use. The field is opt-in,
human-only, and that combination is the release's actual content — the default changes nothing
precisely so that the *decision* is the deliverable.

**What it costs to build, measured rather than implied.** "Twenty-four lines" describes
`engine/reviewconvergeon.go` alone. `review_converge_on` as a shipped feature touches **16 production
`.go` files, 11 test files, and both `skills/tp/REFERENCE.md` and `skills/tp/SKILL.md`**. Two surfaces
in that list are load-bearing and were missed once already: v0.31.1 exists as a whole release because
v0.31.0 shipped `review_converge_on` without `tp config --extract` and `validate --project`. Both are
in this release's scope.

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

**But they are not why the rule is needed, and saying they were would misdirect the implementer.**
`ReviewRound.Clean` is stamped at record time, so a round recorded in 2026 is never re-graded under a
knob set later: those 67 rows are read by no predicate this release adds. **The live risk is
forward-looking** — nothing validates `severity` on the way in, so a future role can write anything —
and the corpus supports that reading rather than the historical one: across v0.33.0–v0.36.0 there
are **zero** out-of-enum severities in 238 non-`PASS` rows. §5 test 4 uses a legacy row because it is
convenient and real, not because the case is live.

## 3. A changed spec ends the streak

`consecutive_clean` is a claim about a spec. When the spec changes, rounds recorded against the old
text no longer support the claim. The review phase already acts on this — a `fixed` resolution forces
a further round, and `tp resume` raises a `spec-stale` blocker — and the audit phase does not.

**Mechanism.** `consecutive_clean` is not stored: it is derived by `engine.ConsecutiveClean` walking
`AuditRounds`. That function is shared with review, so this release **adds two functions rather than
changing either shared one**:

| added | stops at a `spec_hash` boundary | called from |
|---|---|---|
| `engine.ConsecutiveCleanSince(rounds []ReviewRound, current string) int` | yes | the audit path only |
| `engine.ConvergedSince(rounds []ReviewRound, need int, current string) bool` | yes | the audit path only |

The second exists because `engine.Converged` folds `ConsecutiveClean` internally, and a reset that
changes two printed integers while `converged` keeps its pre-reset answer is not a reset.

**Every call site, enumerated, because two enumerations of this list have already been short.**
`Converged` is reached from `audit_record.go` (twice), `run_status.go`, `budget.go`, **`resume.go:87`**
— which feeds `DetectPhase`'s `release` branch and `resumeblockers.go`, so omitting it leaves
`tp resume` reporting `phase: release` on a streak the reset just invalidated, this section's own
stated failure — and **`review_status.go:67`**, which is the one site Non-Goal 4 promises cannot move.
`ConsecutiveClean` is reached from `audit.go:352`, `audit_record.go:101` and `:339` on the audit side
and `review_status.go:91` on the review side. The audit sites move to the `Since` variants; the review
sites do not.

**Prospective only, and it needs a marker rather than a word.** The reset applies to rounds recorded
from this release forward. Nothing on `ReviewRound` currently records a version, and every round
already on disk carries a `spec_hash` the new walk would stop at — so "prospective" written as prose
is unimplementable and unwritable as a test, which is what an earlier draft did. tp already owns the
pattern: `ReviewRound.IDScheme` plus `engine.IsLegacyRound` (`engine/reviewstate.go:503-505`) marks a
round's vintage on the round itself. **`ConsecutiveCleanSince` treats a round `IsLegacyRound` reports
as pre-marker the way `ConsecutiveClean` does — no boundary — and applies the reset only from the
first marked round onward.**

Why the word alone will not do: an earlier draft made the reset retroactive, and simulated against
v0.35.0's nine rounds — which carry **seven** distinct `spec_hash` values, counted, r1=r2 and r7=r8 —
that moves `blocking` from round 3 to round 8 and `all` from round 9 to never, falsifying §2.1's own
table and reducing "six rounds saved" to one. Deleting the word "retroactive" did not stop the
mechanism being retroactive; the marker does.

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

6. **An audit-side `nonblocking_open`.** Under `blocking`, a clean round can have surviving
   `warning` and `info` rows and tp emits no count of them, so an agent reading `clean: true` cannot
   tell an empty round from a suppressed one. The review side emits `nonblocking_open`
   (`review_record.go:231`) and six places — `spec/0.31.0.md` §8.4, `skills/tp/REFERENCE.md:634`,
   `skills/tp/SKILL.md`, `reviewconvergeon_clean_test.go:185-205`, `harness_note_test.go:263` and
   0.31.0's test 18 — pin its absence on the audit side **by name**. Creating the counterpart means
   inverting a pinned guard, which is a decision this release does not make and an earlier draft
   assumed without noticing. `spec/0.46.0.md` §8 takes it. Until then an operator setting `blocking`
   reads the round files for what was suppressed.

**One shipped guard is deliberately reversed, and the reversal is exactly two of its eleven
assertions.** `internal/cli/audit_signal_test.go`'s `TestAuditGate_NoEscapeHatchFromTheGate` carries
eleven `assert`/`require` calls, counted. Lines 473–474 assert that `audit_converge_on` is an
*unknown* workflow field; shipping the field reverses those two, which is the point of the release.
**The other nine stand**, and three of them are named here because a unit with spec cover for
"reversing the guard" could otherwise delete them: that a `wontfix` disposition does not clear an
open finding, that `--check` stays shut over a disposed finding, and — `:498` —
`assert.Len(t, e, 3, "a streak entry carries role, consecutive_clean and open only")`, which
`spec/0.34.0.tasks.json`'s own sweep records as the mutation-killing assertion in this file and which
§3's reset touches. Non-Goal 2 keeps the first two true; §5 test 9 pins all nine.

## 5. Tests

Every test names the input that makes it fail, because four earlier drafts of this list contained a
test that passed against unmodified tp.

1. **Resolution.** `audit_converge_on` resolves task override > project config > built-in, and an
   invalid value is refused with the same hint shape `review_converge_on` uses. The mutation that
   kills it: a resolver reading only the project layer. It does **not** assert a CLI or environment
   layer, because none exists.
2. **`info` is advisory.** Under `all`, a round with one surviving `info` row is not clean; under
   `blocking`, it is.
3. **`error` blocks.** Under `blocking`, a round with one surviving `error` row is not clean.
4. **`warning` is advisory**, tested separately from `info` because `warning` is 158 of the corpus's
   360 non-`PASS` rows and carries most of §1's 93%.
5. **Unrecognised fails closed.** Under `blocking`, a non-`PASS` row is counted as blocking when its
   severity is (a) outside the enum, (b) `null`, and (c) absent. Three fixtures, because a legacy row
   supplies only (a) — all 67 carry a present, non-null `high`/`medium`/`low`.
6. **The replay.** Over committed testdata — the `(status, severity)` pairs of v0.35.0's nine
   recorded rounds, copied into `testdata/` with the commit cited — `blocking` converges at round 3
   and `all` at round 9, **at `audit_clean_rounds: 2`**, which is stated because at 1 the answers are
   2 and 8. The test does not read `spec/.tp-review/`: that directory is live state for cycles still
   in flight, and an acceptance criterion that moves when unrelated work is recorded is not one.
7. **The reset is isolated from staleness.** `Converged` is `ConsecutiveClean(...) >= n &&
   !StateStale(...)`, and `StateStale` compares only the last round's hash to the current spec — so a
   naive fixture satisfies test 8 through the pre-existing term. This fixture's **last** round's
   `spec_hash` equals the current spec hash, so only §3's reset can move the answer.
8. **The reset works.** With a `spec_hash` boundary between the only two clean rounds and test 7's
   isolation in place, `ConvergedSince` is false and `--status --check` exits 1 on the same input.
9. **The shared walk does not move.** `ConsecutiveCleanSince` on rounds at hashes A, A, B returns
   **1** — the trailing run — while `ConsecutiveClean` on the same input returns 3. Asserted on both
   the function and the surface: `review_status.go:91`'s `consecutive_clean` still reports 3.
10. **Prospectivity has a marker.** A round that `engine.IsLegacyRound` reports as pre-marker
    contributes no boundary, so a history recorded before the upgrade keeps the `consecutive_clean`
    it had. The mutation that kills it: dropping the `IsLegacyRound` check, which makes v0.35.0's
    recorded history report `all` as never-converged.
11. **The fence.** `tp set --workflow audit_converge_on=blocking` exits 2 under `TP_UNATTENDED=1`
    with the escalation hint, and exits 0 without it.
12. **The nine surviving assertions** of `TestAuditGate_NoEscapeHatchFromTheGate` still pass,
    including `:498`'s streak-entry shape.
