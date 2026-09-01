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
`engine/configresolve.go:261` merges exactly two layers over the default, and a search for
`TP_REVIEW_CONVERGE_ON` under `internal/` returns zero — scoped deliberately, since an unscoped
search now returns this sentence and its copies. An earlier draft of this section wrote the five-layer chain,
which `spec/0.35.0.md` §7 records being removed after its audit reported across two rounds that the
top two layers cannot be verified because nothing implements them. The five-layer form was copied
into this draft from `CLAUDE.md` and corrected there in the same batch as this sentence, but **it
still stands in two other places this release must sweep**: `skills/tp/REFERENCE.md:519-522` states it
as *the* rule for workflow fields, and `spec/0.31.0.md:41` carries it for the twin. `REFERENCE.md:327-328`
additionally names `TP_RUN_MAX_UNITS` and `TP_REVIEW_MAX_ROUNDS`, neither of which exists in any
non-test source. `engine/reviewconvergeon.go` is twenty-four lines
— two constants, a hint string, and a predicate shared by the setter and every consumer — and this
is its twin.

| value | a round is clean when |
|---|---|
| `all` | no non-`PASS` row survives, whatever its severity |
| `blocking` | no non-`PASS` row in the round carries a blocking severity |

Both predicates are over the round's non-`PASS` rows. **"Surviving" is deliberately not the word:**
it would imply a disposition that removes a row from the count, which Non-Goal 2 declines to create. Measured across v0.35.0's and v0.36.0's audit rounds,
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

**`blocking` governs `clean` and nothing else, and that boundary is stated because Non-Goal 1's
argument depends on it.** `role_streaks[].open`, `role_streaks[].consecutive_clean` and
`spec_coverage_clean_rounds` are computed by `auditRoundOpenByRole` (`engine/rolestreaks.go:128-142`)
through `AuditRowIsPass` alone. Under `blocking` a round can therefore record `clean: true` beside a
non-zero `open` — correct rather than defective: `open` counts findings, `clean` answers whether the
phase may end. Making the streak surfaces severity-aware is the same role-scoping question Non-Goal 1
defers, and goes with it.

**`severity` on a non-`PASS` row is a self-declared label and this release does not pretend
otherwise.** `audit_schema.go` renders the requirement into every prompt and validates nothing;
`audit_record.go` validates `category` alone; nothing on the audit path reads `severity` at all. An
operator setting `blocking` is choosing to gate on the audited agent's own judgement. Making that
label checkable is `spec/0.46.0.md` §8's subject, not this release's.

**Which is why the fence is not a detail of this release — it is what the release buys.** This is the
**first *string-valued* workflow field whose non-default value relaxes a gate** — the cap fields are
fenced already, which is the point of the comparison rather than a counterexample to it; `review_converge_on`'s weak value is
its own default, so nothing has needed this before. Without a fence, a unit stuck in an audit phase
under `tp run` could set `blocking` and end its own phase, which is the failure the whole
unattended-mode design exists to prevent.

`--skip-gate` is the precedent for the *decision* and not for the *mechanism*: it exits 2 under
`TP_UNATTENDED=1`, but the only workflow-write fence is `fenceWorkflowWrite(field string, requested
float64)` in `internal/cli/unattended.go:56` — **numeric by signature**, covering caps and command
fields. A string-valued field does not fit it. `fenceWorkflowWrite` has exactly four call sites — `set.go:414`, `set.go:428`,
`set_project.go:98`, `set_project.go:108` — all inside branches that have already run
`ParseFloat`/`Atoi`, so a string field reaches none of them. Widening it is **two new call sites in
two files**, and **both write paths are fenced**: `tp set --workflow audit_converge_on=blocking` and
`tp set --workflow --project audit_converge_on=blocking`. The second is not optional — it is the form
`audit_signal_test.go:472` drives, so a fence on `set.go` alone leaves `--project` an unattended
escape hatch that the release's own reversed guard exercises.

**And the escalation needs a name that exists.** `--decision` is a closed, validated enum
(`skip-gate, raise-review-cap, raise-audit-cap, import-force, other` — `escalate.go:41`, enforced by
`IsEscalationDecision`), and `escalationDecision(field)` maps only the two cap fields, returning
`other` otherwise. A refused unit would stop the whole run under `decision: other` with the reason
recoverable only from free text. This release adds `audit-converge-on` to that enum and maps the
field to it. The field is opt-in,
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
are **zero** out-of-enum severities in 238 non-`PASS` rows. §5 test 5 uses a legacy row because it is
convenient and real, not because the case is live.

## 3. What this release deliberately does not contain

**A spec-hash reset for `consecutive_clean` was in this release through five review rounds and is
not in it now.** The reasoning is recorded because the cut is the release's most consequential
decision and the next reader will otherwise re-propose it.

The mechanism was sound in outline — a changed spec means rounds recorded against the old text no
longer support the streak — and three drafts of it were falsified in three consecutive rounds, each
time on the same class:

| round | finding | what changed | what survived |
|---|---|---|---|
| 3 | `mechanism-falsifies-its-own-replay` | the retroactive clause | the mechanism stayed retroactive |
| 4 | `prospectivity-without-a-marker` | "prospective only" added as prose | nothing implements it |
| 5 | `prospectivity-marker-does-not-discriminate` | `IsLegacyRound` named as the marker | measured inert in 15 of 15 recorded histories |

`engine.IsLegacyRound` is `r.IDScheme == ""`, and `audit_record.go:213` has stamped the slug on every
audit round recorded since **v0.30.0**. It separates v0.30.0 from v0.29.0, not this release from the
last one. Under it the reset still applies to v0.35.0's whole history, taking a shipped, converged
cycle to `converged: false` on install — the outcome the marker was named to prevent.

**The class survived three prose corrections, which this repository's own rule says means the fourth
will not work either.** The reason is structural and is the same one the header records for rounds
1–3: what the section needs is a **decision** — store a new per-round byte marking a round's vintage,
or accept a retroactive reset and re-measure §2.1 against it — and a review round cannot make a
decision, only observe that none was made. Five rounds have now observed it.

It is `spec/0.46.0.md` §8.4, with the measurements. The decision belongs before that spec's round 1,
not during it.

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
   (`review_record.go:231`) and **four** places pin its absence on the audit side by
   name, counted: `spec/0.31.0.md` §8.4 and its test 18, `skills/tp/REFERENCE.md:634`, and
   `reviewconvergeon_clean_test.go:185-205`. (`harness_note_test.go:263` is a comment carrying no such
   assertion and `SKILL.md` never names the field in an audit context; an earlier draft counted six
   and the count was copied into `spec/0.46.0.md` before anyone checked it.) Creating the counterpart means
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
§3's reset touches. Non-Goal 2 keeps the first two true; §5 test 12 pins all nine. One dependency is stated because it is
otherwise invisible: those nine hold only because `signalRow` (`audit_signal_test.go:40-42`) emits no
`severity` key, so §2's fail-closed rule keeps the round unclean.

## 5. Tests

Every test names the input that makes it fail. Five earlier drafts of this list each contained a test
that passed against unmodified tp, so the naming is not decoration: it is the check that the test has
a failing arm at all.

1. **The built-in default is `all`.** Asserted directly, because five drafts of this list never
   asserted it and an implementation shipping `blocking` as the built-in passed all of them while
   inverting §2.1's whole argument.
2. **Resolution.** `audit_converge_on` resolves task override > project config > built-in, and an
   invalid value is refused with the same hint shape `review_converge_on` uses. Killing mutation: a
   resolver reading only the project layer. It asserts no CLI or environment layer, because none
   exists.
3. **The two surfaces v0.31.1 exists for.** `tp config --extract` emits the field and
   `tp validate --project` accepts and rejects it, tested because v0.31.0 shipped the twin without
   them and needed a whole release to add them.
4. **`info` is advisory.** Under `all`, a round with one surviving `info` row is not clean; under
   `blocking`, it is.
5. **`warning` is advisory**, tested separately from `info` because `warning` is 158 of the corpus's
   360 non-`PASS` rows and carries most of §1's 93%.
6. **`error` blocks.** Under `blocking`, a round with one `error` row is not clean.
7. **Unrecognised fails closed.** Under `blocking`, a non-`PASS` row is counted as blocking when its
   severity is (a) outside the enum, (b) `null`, and (c) absent — three fixtures, because a legacy row
   supplies only (a): all 67 carry a present, non-null `high`/`medium`/`low`.
8. **The replay, over committed testdata.** The rows of v0.35.0's nine recorded rounds and v0.36.0's
   thirteen, copied into `testdata/` with the commit cited, reproduce §2.1's table: `blocking` at 3
   and `all` at 9 for the first, `blocking` at 3 and `all` never for the second, **at
   `audit_clean_rounds: 2`** — stated because at 1 the first answers are 2 and 8. Both rows are pinned
   because the v0.36.0 row is the one that argues against the default. The test does not read
   `spec/.tp-review/`: that is live state for cycles still in flight, and an acceptance criterion that
   moves when unrelated work is recorded is not one.
9. **Both fence paths.** `tp set --workflow audit_converge_on=blocking` and
   `tp set --workflow --project audit_converge_on=blocking` each exit 2 under `TP_UNATTENDED=1` and 0
   without it. Two assertions, not one: `fenceWorkflowWrite`'s four existing call sites are all in
   `set.go` and `set_project.go` numeric branches, so an implementation that fences one file passes a
   single-path test while leaving the other open.
10. **The escalation is nameable.** A fenced refusal records `--decision audit-converge-on`, not
    `other`, which requires the enum entry §2 adds.
11. **The nine surviving assertions** of `TestAuditGate_NoEscapeHatchFromTheGate` still pass,
    including `:498`'s streak-entry shape.

**What is not tested here, named so its absence is deliberate.** `clean` is this release's only
gated surface; `role_streaks[].open` and `spec_coverage_clean_rounds` stay severity-blind by §2's
stated boundary, and `spec/0.46.0.md` §8 owns changing that.
