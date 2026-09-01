# tp v0.37.0 — Audit convergence

> **Rewritten after review round 7, which diagnosed why six rounds of repair did not converge.**
> §2 and §4 had become a survey of the repository — line-number citations, counted enumerations,
> which of eleven assertions survive, which documented sentence this release falsifies. Each round
> rewrote the survey and the next round re-falsified it against the tree, so the reviewed surface
> grew with every repair and the finding count stayed flat while the *kind* narrowed to "this
> citation is wrong". Nine of round 7's twelve architect findings were of that kind.
>
> A survey is checked by reading, and reading is what a review round does — so it can be re-falsified
> forever. **The decisions are what a spec owes; the survey belongs in task acceptance, where it is
> checked by running.** This file is the decisions. The measurements that justify them stay; the
> forensics that would guide an implementer go to `spec/0.37.0.tasks.json`.

## 1. Overview

`review_converge_on` lets the review loop stop counting advisory findings against convergence. The
audit loop has no equivalent: `--record` counts every row whose `status` is not exactly `PASS`, at
any severity. Two cycles in this repository and one field cycle in another project each ran long
past the point where the only surviving findings were advisory.

Measured on v0.35.0's audit corpus — nine recorded rounds, 42 non-`PASS` rows: **3 `error`,
22 `warning`, 17 `info`**. 93% of what kept that phase open could not have blocked a release.

**Three deliverables.**

1. **`audit_converge_on`** (§2) — the field, its resolution, its validation, its documentation.
   Default `all`, which is the behaviour tp ships today, so a project that does not set it sees no
   change.
2. **The fence** (§3) — the field is opt-in *and human-only*. This is what the release buys.
3. **`by_severity` on `tp audit --merge`** (§4) — the breakdown that makes a `blocking` round's
   verdict readable. Emitted unconditionally, so it is the one part every project sees.

A fourth, a spec-hash reset for `consecutive_clean`, was in this release through five rounds and is
now `spec/0.46.0.md` §8.4. §5 records why, so the next reader does not re-propose it.

## 2. `audit_converge_on`

**The field.** A workflow field taking `all` or `blocking`, resolved *task override > project config
> built-in* — the precedence workflow fields actually use; there is no CLI layer and no `TP_<FIELD>`
layer for one. Invalid values are refused at both the write sink (a usage error) and the consuming
sink (a validation error), on the same terms and with the same hint as `review_converge_on`.

| value | a round is clean when |
|---|---|
| `all` | no non-`PASS` row in the round carries any severity |
| `blocking` | no non-`PASS` row in the round carries a blocking severity |

Both predicates are over the round's non-`PASS` rows. Measured across v0.35.0's and v0.36.0's audit
rounds, every one of 2,913 `PASS` rows carries `severity: null`, so a predicate phrased over "any row
of any severity" would read the wrong population. *Surviving* is deliberately not the word: it would
imply a disposition that removes a row from the count, which §6 declines to create.

**Blocking severity is `error`.** Audit rows use `error | warning | info`; `review_converge_on`'s set
is `critical | high` over the review vocabulary. The two vocabularies do not overlap, so the audit
predicate does not reuse the review ranker — one that did would rank every audit severity as unknown.

**An unrecognised severity is treated as blocking.** Not rejected, not ignored. A row tp cannot grade
is a row tp must not stop counting. Stated normatively because two earlier drafts left it to be
inferred and both inferred it wrong.

**`clean` is stamped at record time, not recomputed live.** This is a decision and the alternative was
real: the review twin recomputes live, by design, "so a later `--resolve` or converge-on switch
re-evaluates it without re-recording". Copied here, that would re-grade every recorded round the day
the knob is set — and 67 non-`PASS` rows across v0.29.0–v0.32.0 carry severities outside the enum, in
fifteen round files, which the fail-closed rule above would then make blocking. Converged historical
cycles would report `converged: false` on install. **Setting the knob governs rounds recorded after
it is set**, which is the same prospective property that cut the fourth deliverable.

**Default `all`,** because §2.1's missing column says so.

**`blocking` governs `clean` and nothing else.** `role_streaks[].open`, its `consecutive_clean` and
`spec_coverage_clean_rounds` stay severity-blind. Under `blocking` a round can record `clean: true`
beside a non-zero `open`, and that is correct: `open` counts findings, `clean` answers whether the
phase may end. Making the streak surfaces severity-aware is the role-scoping question §6 defers.

**`severity` on a non-`PASS` row is self-declared and this release does not pretend otherwise.**
Nothing on the audit path validates or reads it today. An operator setting `blocking` is choosing to
gate on the audited agent's own judgement — which is exactly why §3 exists and why the default is
not `blocking`. Making the label checkable is `spec/0.46.0.md` §8.2.

### 2.1 Why the default is `all`

| cycle | audit rounds | converges under `blocking` | converges under `all` |
|---|---|---|---|
| v0.35.0 | 9, converged | round 3 | round 9 |
| v0.36.0 | 13, never converged | round 3 | never — no round reaches zero |

The saving is real: v0.35.0 spent six of nine rounds on a population that was 93% advisory.

**The column this table did not have is what decides the default.** It asks *when the loop stops*, and
never asked *what was still to be found after it stopped*. That column reads `error` in both rows —
v0.35.0's round 4 carries `{error 1, warning 2, info 1}` one round after `blocking` would have halted,
and v0.36.0's rounds 7, 12 and 13 each carry `error` rows. **`blocking` would have shipped over an
`error` row in 2 of 2 measured cycles.** That is decisive against it as a default and is not an
argument against the field: an operator who sets it accepts that trade with the numbers in front of
them.

## 3. The fence

**This is the first *string-valued* workflow field whose non-default value relaxes a gate.** The cap
fields are fenced already; `review_converge_on`'s weak value is its own default, so nothing has
needed this before. Without a fence, a unit stuck in an audit phase under `tp run` could set
`blocking` and end its own phase — the failure unattended mode exists to prevent.

**Both values are refused, and every write path is fenced.** The fence is on the field, not the
direction: a unit that can set `all` can set `blocking` on the next line, so a direction-scoped fence
is one a unit walks around. Every path that writes the field under `TP_UNATTENDED=1` exits 2 —
including `tp import`, which reaches the same layer and whose fence today covers only `--force`.

**The refusal is nameable.** `--decision` is a closed, validated enum, and a field that is not in it
stops a run under `other` with the reason recoverable only from free text. This release adds
`audit-converge-on` to that enum, maps the field to it, and refuses with a message that describes
*this* field rather than reusing one written for command fields.

## 4. `by_severity` on `tp audit --merge`

Under `blocking` a clean round can carry `warning` and `info` rows. The *count* of them is already
emitted — `role_streaks[].open` is severity-blind and unconditional on both `--status` and
`--record` — so what is missing is the **breakdown**, and without it an agent reading `clean: true`
cannot see what was suppressed.

`--merge` already loops the round's rows building `by_status` and `by_role`, on the command the loop
runs immediately before `--record`. A `by_severity` counter over non-`PASS` rows goes in that loop.
It is emitted unconditionally and **survives `--compact`**, on the same grounds the review side gives
for `nonblocking_open`: a field whose absence changes a decision is not explanatory. A row with an
unrecognised severity is counted under the literal it carries, and one with none under `null`, so the
breakdown never hides a row the predicate would block on.

This is the audit-side visibility this release ships. `nonblocking_open` on `--record` and `--status`
is a different surface with four pinned guards against it and stays deferred (§6).

## 5. What this release deliberately does not contain

A spec-hash reset for `consecutive_clean` was here through five review rounds. Rounds 3, 4 and 5 each
falsified a different draft and each time the class survived: the retroactive clause went and the
mechanism stayed retroactive; "prospective only" arrived as prose with nothing implementing it; then
`IsLegacyRound` was named as the marker and is inert — it has marked every audit round since v0.30.0,
and across all fifteen recorded audit histories the check changes the streak in **0 of 15**.

The section needed a **decision** — store a per-round vintage byte, or accept a retroactive reset and
re-measure §2.1 against it — and a review round cannot make one. It is `spec/0.46.0.md` §8.4 with both
options priced.

## 6. Non-Goals

1. **The `--check` divergence.** `consecutive_clean` counts every role while the shipping rule reads
   `spec_coverage_clean_rounds` plus no-FAIL. That is role scoping, not severity scoping, and severity
   parity does not close it: on v0.36.0's rounds 12–13 every `error` row belongs to one role while the
   conformance role was clean four rounds running. `spec/0.46.0.md` §8.
2. **A disposition that stops blocking.** `spec/0.46.0.md` §8.1, with the record-time/live choice and
   the merge-ordering defect that breaks it either way.
3. **Making `severity` checkable.** `spec/0.46.0.md` §8.2.
4. **`nonblocking_open` on `--record` and `--status`.** Four places pin its absence there by name.
   `spec/0.46.0.md` §8.3. §4's `--merge` breakdown is a different surface and none of the four
   reaches it.
5. **Review's convergence.** Unchanged, and nothing here touches the walks review's `--status` reads.
6. **A fourth row status.** `AuditRowIsPass` stays byte-exact on `status == "PASS"`.

**Three shipped assertions are deliberately reversed, and the count is three because two earlier
drafts said one and then two.** The gate guard's assertion that `audit_converge_on` is an unknown
workflow field; the docs-contract guard pinning a `SKILL.md` sentence that says audit convergence
counts every non-`PASS` row; and `engine.DivergenceHint`, a shipped constant emitted verbatim as
`divergence.hint`, which ends with that same sentence and is therefore false under `blocking` in
exactly the situation it fires for. Every other assertion in those guards stands.

## 7. Tests

Each test names the input that makes it fail. Six earlier drafts of this list each contained a test
that passed against unmodified tp.

1. **The built-in default is `all`.** Asserted directly: six drafts never asserted it, and an
   implementation shipping `blocking` as the built-in passed all of them while inverting §2.1.
2. **Resolution.** Task override beats project config beats built-in. No CLI or environment layer is
   asserted, because none exists.
3. **Both refusal sinks.** An invalid literal at the write sink is a usage error; an invalid stored
   value reaching a consuming command is a validation error. Two exit codes, two commands — the twin
   has both and one test covering one of them would pass while the other path shipped ungraded.
4. **The config surfaces.** `tp config --extract` emits the field and `tp validate --project` accepts
   and rejects it. v0.31.0 shipped the twin without these and needed v0.31.1 to add them.
5. **`info` is advisory** — not clean under `all`, clean under `blocking`.
6. **`warning` is advisory**, tested separately from `info`: it is 158 of the corpus's 360 non-`PASS`
   rows and carries most of §1's 93%.
7. **`error` blocks.**
8. **Unrecognised fails closed** — three fixtures, for a severity outside the enum, `null`, and
   absent. A legacy row supplies only the first.
9. **`clean` is stamped, not recomputed.** Recording rounds under `all`, then switching to `blocking`
   without re-recording, leaves every stored round's verdict unchanged. The mutation that kills it is
   the review twin's live re-grade, which is the natural thing to copy.
10. **The replay, by re-recording.** Each of v0.35.0's nine and v0.36.0's thirteen recorded rounds is
    re-recorded from committed testdata against a fixture spec, reproducing §2.1: `blocking` at 3 and
    `all` at 9; `blocking` at 3 and `all` never — at `audit_clean_rounds: 2`, stated because at 1 the
    first pair is 2 and 8. Re-recording rather than synthesising round entries is what makes the
    answer reachable: `Converged` also folds a staleness term, and a synthesised history carries the
    original spec's hashes. The test reads committed testdata, never `spec/.tp-review/`, which is live
    state for cycles still in flight.
11. **Every write path is fenced.** Each of `tp set --workflow`, `tp set --workflow --project` and
    `tp import` exits 2 under `TP_UNATTENDED=1` for both `all` and `blocking`, and 0 without it. One
    assertion per path: they are separate guards and a test covering one passes while another stays
    open.
12. **The refusal names this field.** The message describes `audit_converge_on` and the escalation
    records `--decision audit-converge-on` rather than `other`.
13. **`by_severity` on `--merge`.** Present, correct over a mixed round, and still present under
    `--compact`.
14. **The three reversals and nothing more.** The two surviving assertions of the gate guard still
    pass, and both `SKILL.md` and `engine.DivergenceHint` state a rule true under both values of the
    field — asserted against the shipped docs-contract guard, so the documentation cannot drift from
    the constant.
