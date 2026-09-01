# tp v0.37.0 — Audit convergence

## 1. Overview

The review phase knows the difference between a finding that must be fixed and one that was merely
worth saying. The audit phase does not. `review_converge_on` has taken `blocking` or `all` since
v0.31.0; there is no `audit_converge_on`, and `engine.AuditRowIsPass` makes every row whose status is
not exactly `PASS` a finding of equal weight.

Three consequences, all measured:

- **tp's own v0.35.0 audit.** Across nine recorded rounds the non-`PASS` rows were **3 `error`, 22
  `warning`, 17 `info`**. Two of the three `error` rows were the same defect seen by two roles. The
  loop ran to nine rounds on a population that was 93% advisory.
- **A field cycle on a PHP package.** Rounds 3 through 9 produced 15–30 findings each, *zero* `FAIL`
  in the last three, and never converged. Its `spec-coverage` role carried 4–7 `PARTIAL` rows that
  the operator had already accepted as backlog. Because an accepted row never becomes `PASS`, that
  role could never be clean, `spec_coverage_clean_rounds` stayed 0, and the `divergence` signal — the
  one mechanism built to detect exactly that situation — never fired.
- **The same cycle's spec edits went unnoticed.** `tp audit --status` reported the round as
  non-stale while the spec hash changed twice mid-audit (`b5eabd59` → `8cac418b` → `467e1e8f`). In
  the review phase a `fixed` resolution forces a re-review; audit has no equivalent, so an agent can
  amend the spec to match code it just wrote and keep its clean-round streak.

This release gives audit the two things review already has: a severity-aware convergence policy, and
a reason to distrust a streak that spans a changed spec.

## 2. `audit_converge_on`

**It is the twin of a shipped mechanism, not a new idea.** `engine/reviewconvergeon.go` is
twenty-four lines: two constants, a hint string, and a predicate shared by the setter and every
consumer. `audit_converge_on` takes the same two values on the same terms — `blocking` counts a round
clean when no surviving non-`PASS` row carries a blocking severity, `all` when no non-`PASS` row
survives whatever its severity. Both predicates are over non-`PASS` rows: measured across this
repository's v0.35.0 and v0.36.0 audit rounds, every `PASS` row carries `severity: null`, so a
predicate phrased over "any row of any severity" would read the wrong population.

**The field it reads already exists and is already validated.** Audit rows carry `severity` from the
same `error | warning | info` vocabulary the schema requires, and `audit_schema.go` rejects a row
that omits it. Nothing new is asserted by the agent being measured.

**This is not the deferred `scope` idea, and the distinction is the reason this ships.**
`spec/0.33.0.md` §1, `spec/0.34.0.md` §1–2 and `spec/0.35.0.md` §1 defer a `scope` field on audit
rows through three consecutive cycles, each time for the same sound reason: a self-declared label
that gates a release is strictly worse than over-counting, because the agent whose work is being
gated writes the label. That objection does not touch `severity` — it is an existing enum, validated
at the schema, carried by every row already recorded. The two were bundled under one heading and only
one of them was risky. `scope` stays deferred.

**Default.** `blocking`, matching `review_converge_on`, so the asymmetry disappears rather than
inverting. An operator who wants the old behaviour sets `all`.

### 2.0a A second cycle reached the same stop, in a different repository

The measurement below is from this repository. An independent one arrived from a Laravel package's
6-round Workflow D audit and lands in the same place, which is worth recording because a single
repository's numbers are a weak argument for a default.

Findings ran 15 → 3 → 1 → 2 → 1 → 1 and the run was stopped by operator decision, never reaching two
consecutive clean rounds. At the stop, `spec-coverage` — the only role measuring spec conformance —
was **55/55 for four consecutive rounds** and `src/` had seen no behavioural change since round 2.
Every round stayed unclean on info- and warning-severity prose findings in docs and comments.

**The reason it could not converge is structural and is the argument for this release.** Rewriting a
paragraph to fix one claim creates new surface for the next round; rounds 5 and 6 each found a defect
introduced by the *previous* round's fix. There is no fixed point for prose the way there is for
code. v0.36.0's own audit is the second instance of the identical shape — four consecutive fully
clean `spec-coverage` rounds while `consecutive_clean` never left zero, and four repairs each
falsified by the round after (`spec/0.46.0.md` §7).

**`divergence` did its job and that is precisely the point.** It fired from round 4 and its hint was
acted on correctly: it told the operator to surface and stop rather than decide. It gates nothing by
design, so `--status --check` still reports "not converged" indefinitely. `audit_converge_on=blocking`
would have converged that run at round 3–4 with the remaining rows surfaced as `nonblocking_open` —
nothing hidden, and the gate answering the question the operator was actually asking. §5 records the
same gap from the `--check` side.

**One caveat this release must carry, because the severity gate is what makes it safe to stop.** Both
cycles' late rounds looked like prose churn and one of them was not: v0.36.0's rounds 12 and 13
produced two `error`-severity findings — guards certifying a CI step that never runs — invisible to
the conformance role and found only because those rounds were asked to build a *new instrument*
rather than to re-read. A severity gate is right, and it must not become a default stop at round 3
that ends rounds which would have changed their own method. `spec/0.49.0.md` §1c carries the cheapest
known way to tell the two apart: ask the role whether it still holds its own prior reasoning.

### 2.1 What the default would have cost, measured in both directions

Replaying the two predicates over this repository's own recorded audit rounds, with
`audit_clean_rounds: 2`:

| Cycle | rounds recorded | converges under `blocking` | converges under `all` |
|---|---|---|---|
| v0.35.0 | 9 | **round 3** (r2, r3 carry zero `error`) | round 9 |
| v0.36.0 | 7 and open | **round 3** (r2, r3 carry zero `error`) | not yet |

v0.35.0 is the case for the release: six rounds of a nine-round phase spent on a population that was
93% advisory, and `all` reproduces the recorded history exactly, which is what makes the change
opt-in rather than a reinterpretation.

**v0.36.0 is the case against the default, and it is not hypothetical.** That cycle's round 7 — four
rounds after `blocking` would have closed the phase — recorded three `error` rows. Two are repair
artifacts of round 6 and would not exist in a cycle that had stopped at round 3. The third is not:
`engine.WriteSnapshotAtomic` builds its temporary file from a fixed `final + ".tmp"`, so role units
that `REFERENCE.md` runs parallel with their siblings collide and the loser's rename fails with
`ENOENT` — measured on the literal brief commands at 0/20 sequential and 5/40 parallel. It is a
defect in shipped code that predates every repair in that cycle, and `blocking` would have shipped
over it.

The honest reading is not that `blocking` is wrong but that **the two cycles disagree, and one
sample each way is not a policy.** What the release owes is that the disagreement is visible: the
field is opt-in either way, the default is stated with this measurement beside it, and whichever
default ships, a later cycle can settle it from data rather than from preference. Decomposition
should treat the default as the one decision in this release with a recorded argument against it.

## 3. A disposition that says "accepted" stops blocking

v0.35.0 shipped `tp audit --resolve`, so an audit row can be marked. What it cannot do is stop
counting: a resolved row is still not `PASS`, and `AuditRowIsPass` is what convergence reads.

That is what stranded the field cycle. Its accepted backlog items were correctly recorded and
correctly re-reported, round after round, as reasons the phase was not done.

**Mechanism.** A row carrying a `wontfix` or `duplicate` disposition is excluded from the round's
finding count on the same terms review already excludes a resolved finding. `fixed` is not excluded
— a fix changes the code, and the next round is what checks it.

**Bound.** This is the counting rule only. Where an accepted finding *goes* so that it becomes
maintenance pressure rather than archived prose is `spec/0.41.0.md` — this release stops it blocking,
that one gives it somewhere to live. Recording the disposition without the durable home is the
smaller half and the one convergence needs.

## 4. A changed spec ends the streak

`consecutive_clean` is a claim about a spec. When the spec changes, rounds recorded against the old
text no longer support the claim, and the review phase already acts on this: a `fixed` resolution
forces a further round, and `tp resume` raises a `spec-stale` blocker.

**Mechanism.** When `tp audit --record` sees a `spec_hash` different from the previous round's, it
resets `consecutive_clean` to zero and says so in the payload. The rounds themselves are not
discarded — a recorded round stays a frozen fact — only the streak they were feeding.

**What this is not.** `spec/0.45.0.md` §3's `--reconcile` records *why* a spec moved, preserving
the hash the round actually read. This section resets *what the movement invalidates*. They compose:
reconcile explains the change for a reader, the reset makes the loop re-earn its streak. Neither
substitutes for the other, and neither is a prerequisite for the other.

## 5. The gate tp reports and the gate this project ships on are not the same gate

v0.36.0 shipped with `tp audit spec/0.36.0.md --status --check` returning **exit 1**, and shipping was
still correct. That gap is a defect in this release's subject, not in that decision.

`consecutive_clean` counts rounds with zero findings **across every role**, and
`required_clean_rounds` is compared against it. CLAUDE.md's shipping rule is phrased on a different
quantity — `spec_coverage_clean_rounds`, plus "no round has produced a FAIL" — because the
2-clean-rounds rule "is about SPEC CONFORMANCE, not about the codebase at large". v0.36.0's audit is
the worked case:

| round | findings | `spec_coverage_clean_rounds` | `consecutive_clean` |
|---|---|---|---|
| 10 | 2 | 1 | 0 |
| 11 | 1 | 2 | 0 |
| 12 | 2 | 3 | 0 |
| 13 | 3 | 4 | 0 |

Four consecutive fully clean rounds for the role that measures conformance, zero open spec-scoped
findings, and a counter that never left zero — because a second role kept returning rows about CI
guard machinery no checklist item covers. `--check` cannot reach exit 0 while any role finds
anything, so on the rule this project actually uses it was answering a different question.

**What this release must decide.** `audit_converge_on` already separates which rows block; the same
distinction has to reach `--check`, or `--check` stays an instrument nobody can ship on and every
cycle re-derives the judgement by hand from `role_streaks`. The candidate shapes, neither chosen
here: a role-scoped convergence set (`converge_roles`, defaulting to the conformance role) so
`--check` measures the streak the rule reads, or a `--check` that reports the shipping predicate
itself — conformance-role streak plus no-FAIL — with the all-roles count demoted to advisory.

Whichever it is, the test is the one v0.36.0 would have failed: a fixture where the conformance role
is clean for `required_clean_rounds` running while another role has open non-FAIL rows must reach
exit 0, and must not reach it if the conformance role has an open row.

## 5a. A PASS row can carry a defect in its note, and the counter cannot see it

**Measured in two repositories, and the stronger case is not this one.** A Laravel package's audit
had `spec-coverage` score **55/55 PASS in rounds 4, 5 and 6**, and in each of those three rounds the
same role's prose named a real spec defect that the operator then fixed with a commit
(`e7c8eb45`, `6debebfb`, `6be7bde5`). The role labelled its own rows *"recorded, not scored as gaps"*
and *"PASS with the discrepancy in the note"* — it knew the tick and the observation disagreed and
had no field to say so in. Three ticks, three defects, one role. That cycle's
`spec_coverage_clean_rounds` reached 4 and a `divergence` decision was brought to a human resting on
a number three notes contradicted.

**This repository has one instance, and it is weaker.** v0.36.0 audit round 13, item
`task-role-mode-test`, status **PASS**, note:

> "The hardcoded `modeFlags` map at :150-154 is a second, weaker guard **whose doc comment at
> :139-144 claims more than its body delivers**, but no checklist clause asks it to derive from the
> registered flag set."

A real defect of the class this cycle repaired three times elsewhere, sitting inside a PASS row, in a
round counted toward the `spec_coverage_clean_rounds: 4` that v0.36.0 shipped on. It went unfixed
until it was found by re-reading the recorded rows rather than the round's report. The reason it is
weaker than the three above: the role's PASS was **correct** under tp's own scope rule — it checked
five clauses for a derivation requirement and none imposed one — so nothing was mis-scored. The row
was right and still lost the observation.

**That is the finding, and it is not that roles score leniently.** In both repositories the reading
happened and was accurate; what failed is that a checklist row has one field for a verdict and no
field for *"my prose knows something my verdict cannot carry"*. The related measurement in
`spec/0.49.0.md` §1a is the same shape one level down: a summarising model asked to tick four claims
confirmed a claim and its negation, while the same model asked one claim at a time answered both
correctly. **A row per item invites a tick per row.**

**Two cheap changes, neither of which touches the checklist.** The checklist is what makes coverage
checkable and it stays. (1) The brief asks each role to state anything its prose knows that its row
cannot carry, **and to say what should be done about it**. (2) A PASS carrying such a statement is an
unresolved item for convergence, not a clean one. On the second rule none of the four rounds named
above was clean.

**The second clause of (1) is not politeness, and a measured instance says a field alone is not
enough.** A fifth case, `test-isolation` at 10/10 PASS, carried a note labelled *"One structural note,
not a finding"*: two `FIRE_AND_FORGET` assertions built from independent string literals, so a typo in
only the second would leave the positive assertion passing and the negative one passing vacuously.
Real, correctly outside the checklist, and it **sat unactioned for a round**. Asked in the next round
to give an actual recommendation either way, the same role **narrowed its own note against itself**:
the hazard is not the duplication but what it is paired with, so three of the four files it had
flagged were already safe by construction and only three call sites in one file were exposed. It
retracted its own earlier framing — *"the redundancy buys nothing here that a local variable doesn't
buy more cheaply"* — and the operator committed the change.

A field would have captured the note in the first round. It would not have produced the narrowing, and
**three quarters of what was first flagged did not need fixing.** `spec/0.49.0.md` §1c states the
general form: an unforced record is not usable evidence, and forcing a commitment is what converts a
reading into something actionable.

## 5b. A round recorded with one role erases the counter the ship rule reads

Measured against tp's own state at v0.36.0 audit round 12. `spec-coverage` stood at
`consecutive_clean: 3` and `spec_coverage_clean_rounds: 3`. Recording a round whose merged file
carried only `maintainability-conventions`' rows produced:

```
spec_coverage_clean_rounds   None
role_streaks   [{"role": "maintainability-conventions", "consecutive_clean": 1, "open": 0}]
```

The role is not reported as skipped or as stale. It is absent, its streak is gone, and the round is
`clean: true` — so a narrow round reads as progress while deleting the only signal that measures
spec conformance. `spec/0.35.0.md` §8a names "a dropped role merging clean" as a hazard an
unattended driver amplifies; this is the sharper form, because CLAUDE.md's own shipping rule is
phrased on `spec_coverage_clean_rounds` and a narrow round silently sets it to null.

This is what makes it a defect rather than a preference: v0.36.0 shipped `--role`, which makes a
one-role round easy to run for the first time, and the interactive fallback runs exactly one role
per unit. The cheap answer is that a recorded round should carry forward the roles it did not
receive, marked as not-run rather than dropped, so a streak can neither advance nor vanish on their
behalf. It was measured during v0.36.0's audit; `spec/0.46.0.md` §8 records where it came from.

## 6. Non-Goals

1. **`scope` on audit rows.** Deferred a fourth time, with the reasoning above, and now separated
   from the field that made the deferral look expensive.
2. **A durable home for accepted findings.** `spec/0.41.0.md`.
3. **Changing severity's meaning or vocabulary.** `error | warning | info` is unchanged; this release
   only reads it.
4. **Touching review convergence.** `review_converge_on` keeps its current behaviour and default.
5. **A fourth row status.** Whether audit needs a way to say "I could not verify this" is
   `spec/0.49.0.md` §5, and it is open there for four recorded reasons.

## 7. Tests

1. `audit_converge_on` resolves through the documented precedence with default `blocking`, and an
   illegal literal is refused with the hint naming both legal values.
2. Under `blocking`, a round whose only non-`PASS` rows are `warning` and `info` counts clean; the
   same round under `all` does not.
3. Under `blocking`, one `error` row keeps the round dirty however many advisory rows accompany it.
4. Replayed against this repository's nine recorded v0.35.0 audit rounds, `blocking` converges at
   round 3 and `all` at round 9, reproducing the recorded history exactly — the second half is what
   proves the change is opt-in rather than a silent reinterpretation. The replay reads `status` and
   `severity` off recorded rows and needs no spec snapshot, which is why it is runnable where
   `spec/0.43.0.md` §6's corpus gates are not.
5. A row dispositioned `wontfix` or `duplicate` is excluded from the finding count; a row
   dispositioned `fixed` is not.
6. `tp audit --record` with a `spec_hash` differing from the previous round resets
   `consecutive_clean` to zero and reports the reset on the machine-readable surface.
7. The reset does not delete or rewrite any recorded round.
8. A cycle whose spec never changes records an identical streak to today's, so the reset costs
   nothing when nothing moved.
