# tp — A refusal names each bad row, its cause and what it would have accepted

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `refusals-that-name-nothing-measurements.md` beside it, and this file stands
without them. On 2026-09-11 it widened from the ground row's pairing refusal to the class, and
absorbed `record-diagnoses-every-bad-row.md` and `the-guard-pins-the-whole-listing.md`, now
forwarding stubs, and the one doc-comment correction of the dropped
`guards-read-what-production-reads.md`. The forwarding table of its 2026-09-08 split into six specs
is in the sidecar under *Split 2026-09-08*.

Class: **tool** — every decision changes what a refusal, a hint or a prompt line says; the same
inputs are accepted and refused before and after, and no convergence signal moves.

## 1. The decision

**Context.** At each site below tp holds what the reader needs to repair a refused input and does not
say it. The sidecar carries each reproduction at `HEAD` under *Re-verified 2026-09-11* and the field
evidence under *Field report WB-3155, verified 2026-09-11*.

- The ground row's **pairing refusal** names no acceptable tier, and cites `§4.1` — a section of
  tp's own design document, which in a user's repository names the user's own `§4.1`. The unit that
  hits it is a conforming one: its `kind` and its `tier` are each legal, paired wrongly.
- **`--record` names one bad row per invocation** in ground, and on invalid JSON in audit and review,
  so a payload with several bad rows costs one round trip each.
- **`--merge`'s hint blames the format for a missing field.** A field report (WB-3155) spent its time
  looking for a format fault in valid NDJSON whose rows lacked `evidence`.
- **The ground prompt's evidence table drops the column that says why a kind refuses the other
  tiers**, and its `query` and `run` lines do not separate a command run over a git corpus from the
  product's own command. The same field report graded `git diff` measurements `corpus`/`run` and was
  refused; it called the refusal's wording good and the prompt the gap.
- **The empty-payload hint of `tp ground --record`** states a conditional whose antecedent is false
  on a document that has no unit at all.
- **The runner map's refusal** says *"the eight unit kinds"* and names none of them.

**Decision.** One rule: **a refusal names every row it refuses, the cause of each, and the values it
would have accepted, in words that stand in the user's repository.** Each section below applies it
at one site.

**Consequences.** Messages, hints and two prompt lines change. `skills/tp/REFERENCE.md` quotes the
pairing refusal and the tier glosses, and moves with them; the quote gains a binding (§2). The ground
prompt's per-kind table grows by one column. The guards over listing refusals change shape (§8).
Nothing changes which inputs are accepted (Non-Goal 1).

**Alternatives.** *Record the good rows and warn about the bad ones* — rejected: a partially valid
round would make coverage a lie, and `--record` stays atomic (Non-Goal 2). *Point at the prompt
instead of listing the set* — rejected: the recovery is then a re-read of the emitted prompt, the
cost v1.0.0 already refused for the four enum cells.

## 2. The pairing refusal names the set, and cites nothing of tp's own

At `HEAD`, a `{behaviour, read, PASS}` row is refused with:

> field "tier": "read" says nothing about a "behaviour" claim (§4.1), and a PASS row must be
> reached at a tier that does

**The message gains the kind's acceptable tiers and loses the section number:**

> "read" says nothing about a "behaviour" claim, and a PASS row must be reached at a tier that
> does — for this kind, per the prompt's evidence table: run, red-green

**One derivation, two sinks.** The set the refusal ends with and the set the ground prompt prints for
the same kind are produced once, in `spec/1.0.0.md` §4.1's table order, so the prompt and the
recorder cannot drift apart; a unit pays for such drift with a refused round.

**No message `tp ground` writes cites a section of tp's own design document.** In the user's
repository `§2.1` or `§7.2` names a section of the user's spec. Where a message needs a reference it
names the prompt block or the file it means. The anchor refusal is the one exception: its `§n`
describes the user's own section numbering.

**`skills/tp/REFERENCE.md` quotes the pairing refusal verbatim, and a test binds the quote**: it
produces the refusal for `{behaviour, read, PASS}` and requires REFERENCE.md to contain that exact
string, on `TestDocsCarryTheConvergenceSignalWording`'s precedent in `internal/cli/docs_contract_test.go`.

## 3. `--record` names every bad row

**In all three phases, `--record` validates the whole payload and refuses once, naming every refused
line with its reason** — invalid JSON, a cell outside its enum, a wrong pairing, a floor mismatch, a
missing field. The write stays atomic: a refused payload writes nothing (Non-Goal 2). Review's
missing-field refusal already names every offending line; `TestReviewRecord_RefusesEveryRowMissingARequiredField`
stays green, and the other row-level refusals take the same form.

**Why.** What is easy for one row must be equally easy for *N*. Each round trip is a unit
re-invocation, and an operator wrote a whole-file validator of their own to get around the
first-error-only diagnosis — when a caller reimplements the tool's validator to use the tool, the
validator is missing something.

**The same pass warns about an off-vocabulary `severity`.** In review and audit, a row whose
`severity` is outside its phase's vocabulary is recorded and named in one notice with its line and
value — a warning, not a refusal, because rounds already recorded carry such values and a refusal
would stop the loops that write them. Decided at the 2026-09-08 decision pass; tp still does not
judge the severity itself (Non-Goal 5).

## 4. `--merge` names the missing field

**When every skipped line of a dropped input was skipped for a missing required field, the refusal
names each such line with the fields it lacks, and the hint names the fields the phase requires**,
in place of the hint about trailing commas and wrapping arrays. When any skipped line failed to parse
as JSON, the format hint stays, because that is what it diagnoses, and the refusal still names each
line with its cause. Each per-line skip warning gains its line number. Both merges take the rule:
`tp review --merge` and `tp audit --merge` share the hint today.

**Why.** `--record` on the same rows already says `line 1: missing required field evidence`; the merge
that precedes it in the loop is the step that sends the operator after the wrong fault.

## 5. The ground prompt says what each tier is, and why a kind refuses the others

**The prompt's per-kind line carries the reason `spec/1.0.0.md` §4.1's table gives for refusing the
other tiers** — its *"and not the others, because"* column, which the prompt drops today — rendered
without that table's reference to tp's own design document, by §2's rule.

**The `query` and `run` glosses separate the two readings a git corpus invites:** `query` reads as
*any read-only command or search over the corpus — grep, git log, git diff*, and `run` as *ran the
product's own command*. The refusal's wording is §2's; the field report found the gap in the prompt,
not in the refusal.

## 6. The empty-payload hint names the round's state

**The hint states which state the round is in, from the round's own counts, instead of a conditional
over the cause**: a floor with no unit, a floor whose every unit was cut, or a floor with units owed.
The refusal and its exit code are unchanged. At `HEAD` the hint asks for at least one row and then
offers, as the one other case, a floor whose every unit was cut, citing `§2.1` — on a document with
no unit, nothing was cut. `a-round-can-be-driven-from-the-envelope` fenced this hint out and left it
to whichever release next opened `--record`'s hints; this is that release.

## 7. The runner map's refusal names the kinds

**A per-kind runner map keyed on a value that is not a unit kind is refused with the eight unit kinds
and `default` listed.** At `HEAD`, a map keyed `implemnt` makes `tp run` exit 2 with *"not a unit
kind; a per-kind runner map keys on the eight unit kinds plus default"*, and its hint lists runner
shapes, not kinds. An accessor returning the eight kinds already exists and has no production
caller; the sidecar names it under *`UnitKinds` finds its caller*.

## 8. A listing refusal's guard pins the whole listing

**Every refusal that ends in a listing is guarded on its tail**: the message must end with `": "`
followed by the listing the call site passed, joined with `", "`. That is exhaustive against
appending, prepending and insertion, and pins no word before the colon. It covers the four enum-cell
refusals in `internal/engine/groundrow.go`, the pairing refusal (§2) and the runner map refusal (§7).

**Why.** `TestAnEnumRefusalNamesTheValuesItWouldHaveAccepted` asserts one `Contains` per listed value,
a lower bound: truncating a listing reddens it, appending does not, so a refusal telling a unit that
`document` — a *kind* — is a legal tier passes the guard written to make the message informative. The
obvious repair, `Contains` over the joined string, is green on the same mutant. **The cost is the
correct direction**: a message that later adds a clause after its listing turns the guard red and
makes its author re-derive the assertion.

**The same test's doc comment says what the guard derives from.** It claims a value added to
`spec/1.0.0.md` §7.2's table reaches the assertion without an edit; the expectation derives from the
code's own listings, and nothing in the test relates them to the document. The comment says that, and
drops the byte figure it quotes for the prompt, which moves with every spec grounded.

## 9. Non-Goals

1. **Nothing changes what any refusal accepts.** `groundAcceptableTiers`, the enum listings, the
   runner shapes and each phase's required fields are unchanged; row 4 pins the ground half.
2. **`--record` stays atomic.** §3 changes what a refusal says, never what it writes.
3. **No general guard over message text.** Only messages ending in a listing get §8's tail guard;
   every other ground refusal keeps its typed field assertion, which is stronger than any assertion
   over a sentence.
4. **The required fields are unchanged.** Review's `evidence` requirement is `spec/1.1.0.md`'s and
   intended; §4 changes what the merge says about it.
5. **tp does not judge `severity`.** It stays self-declared; §3 names an off-vocabulary value and
   records it.
6. **The skip warnings' channel is `an-unreadable-file-is-named` §5's.** This spec changes their words
   only.
7. **`spec/1.0.0.md` is not edited, and its tables are not bound to the code's enums.** It is a
   shipped spec; the binding `guards-read-what-production-reads.md` proposed was dropped, and its stub
   says why.
8. **The hint of a refused batch `tp done` commit** is `a-task-file-write-names-its-target`'s.

## 10. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Where the mutant is
the shipped behaviour, that behaviour was reproduced at `HEAD` or read from the source on 2026-09-11;
rows 17–19 were measured on 2026-09-08. Rows 4, 5 and 18 already hold at `HEAD` and guard against a
regression; every other row asserts behaviour that does not exist at `HEAD`, so its post-change count
belongs to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | the pairing refusal for `{behaviour, read, PASS}` ends with `": run, red-green"` | the shipped message, which ends at `a tier that does` and names none of the kind's tiers |
| 2 | §2 *one derivation* | for each of the seven kinds, the set the refusal renders equals the set the ground prompt prints, compared as strings | the prompt keeps a filter of its own and renders `code-structure` as `query, read`: red on one kind of seven |
| 3 | §2 *no design section* | the pairing refusal, the unknown-key refusal, the `unit_id` refusal, the `ordinal` refusal, the empty-payload hint, the carry-source hint and the all-cut notice each contain no `§` followed by a digit; the anchor refusal is excluded by name | the shipped strings, each of which carries one |
| 4 | §2 *acceptance unchanged* | all 42 `kind × tier` pairings get the same answer before and after: 9 accepted, 33 refused | change a set in `groundAcceptableTiers` while rewording the message |
| 5 | §2 *the document* | `skills/tp/REFERENCE.md` contains the refusal string this build produces for `{behaviour, read, PASS}` | reword the format string and leave REFERENCE.md: at `HEAD` no test under `internal/` contains the message's text, so every package stays green |
| 6 | §3 *ground* | a ground payload of three rows broken three ways — `{behaviour, read}`, `{corpus, run}` and `tier: "squinted"` on a `document` row — is refused naming lines 1, 2 and 3 | the shipped first-error return, which names line 1 only |
| 7 | §3 *invalid JSON* | a three-line payload whose every line is invalid JSON is refused by `tp audit --record` and by `tp review --record` naming lines 1, 2 and 3 | the shipped invalid-JSON abort, which names line 1 only in both phases |
| 8 | §3 *atomic* | after each refusal in rows 6 and 7, the state directory is byte-identical to what the emission left | validate each row as it is appended, which writes the good rows before refusing |
| 9 | §3 *severity* | a review row and an audit row each carrying `severity: "bogus"` record at exit 0, and each command writes one notice naming line 1 and the value | the shipped record, which exits 0 with no notice naming severity; a refusal fails the row too |
| 10 | §4 | `tp review --merge` over a two-line file whose rows lack only `evidence` exits 1 naming lines 1 and 2 and `evidence`, with a hint that does not contain `trailing comma`; `tp audit --merge` over rows lacking only `status` likewise | the shipped constant hint, which names the format on both merges |
| 11 | §4 *control* | a merge input holding one invalid-JSON line and one line lacking a field keeps the format hint and names both lines with their causes | give every dropped input the missing-field hint |
| 12 | §4 *line number* | each per-line skip warning names its line number | the shipped warning, which names the file and the cause and no line |
| 13 | §5 | the ground prompt carries, for each of the seven kinds, the reason `spec/1.0.0.md` §4.1 gives for refusing the other tiers, and none of those reasons contains `§` followed by a digit | the shipped prompt, which prints kind, subject and set and no reason |
| 14 | §5 *glosses* | the prompt's `query` line names a read-only command over the corpus with `git diff` among its examples, and its `run` line names the product's own command | the shipped glosses `ran a query over the corpus` and `ran the shipped command` |
| 15 | §6 | on a spec holding one heading and no sentence, `tp ground --record` of an empty payload still exits 1, and its hint contains neither `every unit was cut` nor `§` | the shipped hint, which carries both |
| 16 | §7 | a `.tp/config.json` runner map keyed `implemnt` makes `tp run` exit 2 with a message ending in the eight unit kinds and `default` | the shipped message, which names no kind |
| 17 | §8 *appending* | appending a value to any of the four enum listings at its production call site reddens the guard | `append(GroundTiers(), "document", "corpus", "vibes")` at the `tier` call site, and the same append at all four: green in `internal/engine` and `internal/cli` under the shipped per-value `Contains` guard, measured 2026-09-08 |
| 18 | §8 *truncating* | truncating any of the four listings reddens the guard, each of the four subtests failing on its own | `GroundTiers()[:2]`, which the shipped guard already catches — the row keeps that direction |
| 19 | §8 *the refused repair* | row 17 holds for the guard this release ships — the tail assertion — and not merely for a containment assertion over the joined listing | ship `Contains(msg, ": "+joined)` as the guard: on row 17's appended message `Contains` is true and `HasSuffix` false, so the guard stays green and row 17 fails |
