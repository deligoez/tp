# tp — Guards read what production reads

Class: **tool** — no command, no flag, no workflow field; one binding between a guard's expectation
and the document it claims to derive from, plus three test-only tasks. Its measurements are in
`guards-read-what-production-reads-measurements.md` beside it; this file stands without them. Every
citation below names a symbol or a quoted string and the file it lives in, never a line number.

## 1. Overview

The subject is a guard that re-parses a document by hand and so holds a weaker answer than production
does: the enum-refusal guard's doc comment claims a binding to `spec/1.0.0.md` that does not exist,
and the three test helpers in `internal/engine` that read that document each re-implement a Markdown
section scan that does not know what a fence is. This spec was split out of
`refusals-that-name-nothing.md` on 2026-09-08, where it was §4 and §10 of that file — §10 having
itself absorbed the former guard-helper spec's tasks earlier the same day — and that file's
forwarding table maps every moved section to its new slug.

## 2. The guard's doc comment, and the one binding it promised

`TestAnEnumRefusalNamesTheValuesItWouldHaveAccepted`'s doc comment
(`internal/engine/groundrow_test.go`) says:

> The expectation is derived from those listings rather than restated, so a value added to §7.2's
> table reaches this assertion without anyone editing it

**Falsified by running.** In the copy, adding a fourth value to §7.2's `partial_kind` cell of
`spec/1.0.0.md` — `` `two-readings`, `reason-not-conclusion`, `true-when-written`, `scope-mismatch` ``
— and running `go test ./internal/engine/ ./internal/cli/ -count=1` leaves **both packages green**.
The expectation derives from `GroundPartialKinds()`, a Go listing, and nothing relates that listing to
the document.

**The sentence is wrong twice, and the second error is the one that decides the fix.** §7.2's table
does not hold the values for three of the four cells the test covers: reading the four `meaning`
cells of §7.2's field table in `spec/1.0.0.md`, only `partial_kind`'s cell lists its values; the
sidecar's *§4 — §7.2's `meaning` column* has the cell-by-cell reading.

**One cell of four.** The values for the other three live in §3's disposition table and §4.1's two
tables. So "a value added to §7.2's table" names a place that holds the values for a quarter of the
assertion, and a naive value-extractor pointed at the `kind` cell would bind `kind` to `NOT-A-CLAIM`.

**The decision: build the binding where it is constructible, correct the sentence where it is not.**
Not one or the other, because the sentence carries two errors and only one of them is a missing
binding.

- `partial_kind`'s expectation is read out of §7.2's cell and required to equal `GroundPartialKinds()`
  — same order, same values. This is the house pattern for this spec, not new surface: five readers in
  `internal/engine`'s tests already read `spec/1.0.0.md` — three helpers (`floorSection21Verbs`,
  `groundSection72Fields`, `groundSection72VerdictRule`) and two tests
  (`TestTheCauseBoundIsTheOneTheSpecStates`, `TestSection11Row21OnThisReleasesOwnSpec`). The precedent
  for reading a *value* out of the document rather than a field name is
  **`TestTheCauseBoundIsTheOneTheSpecStates`**, which extracts number words with two regexps and
  compares them to `groundCausesMin`/`groundCausesMax` — **not**
  `TestSection11Row21OnThisReleasesOwnSpec`, whose every assertion is structural, as its doc says.
  The three helpers are the subject of §3's first task, and the reader this bullet adds goes through
  the helper that task introduces.
- The comment then says what is true: three of the four listings are pinned to §3 and §4.1 by the
  code's own tables and by nothing in this test, and `partial_kind`'s is pinned to §7.2.

**Binding `GroundVerdicts()`, `GroundKinds()`, `GroundTiers()` and `groundAcceptableTiers` to §3's and
§4.1's tables is available and deliberately not taken here** (Non-Goal 1). It is a different claim —
*the code's enum equals the document's table* — from the one this guard makes — *the refusal names the
code's enum* — and a guard that asserts both is a guard whose failure does not say which broke.

## 3. Tasks taken from the guard spec

Three test helpers in `internal/engine` — `floorSection21Verbs` (`floor_test.go`),
`groundSection72Fields` (`groundrow_test.go`) and `groundSection72VerdictRule`
(`groundtierrule_test.go`) — re-parse a Markdown section of `spec/1.0.0.md` by hand, and none of
them knows what a fence is, while production parses the same document with a block scanner that
toggles on fences. The guard-helper spec measured fifteen inputs against them, four silent and three
false failures; the table and what it taught are in the sidecar under *From the guard spec*. Its
decisions land here as three tasks, all `*_test.go` only (Non-Goal 2):

1. **One shared test helper over production's block and anchor derivation replaces the three
   hand-parsing helpers** — `specTableRowsUnder(t, rel, anchor)` in a new
   `internal/engine/specsection_test.go`, built on `floorBlocks` and `floorAnchorsByLine`, returning
   the raw table-row lines production places under the anchor, and failing by naming the anchor when
   the section is absent or fenced.
2. **§2.1's verb row reads every backticked span** — the row is selected from the helper's rows,
   exactly one unfenced `**verb**` row may match, and the verbs are every `` `…` `` span in it rather
   than the spans a character class can spell.
3. **§7.2's field rows and verdict rows partition one row set, and an unspellable field fails
   loudly** — a row whose first cell is a single backticked span is a field row unless the span
   parses as a §3 verdict, and a first cell that is neither a verdict nor `^[a-z][a-z_]*$` is a
   failure with a message, not a silent omission from a list whose length is then asserted.

The three false sentences the guard spec named — the two helper doc comments and
`TestTheVerbArmIsExactlyTheTwelveListedVerbs`' *"a later release's work"* — are corrected by the
tasks that change them, not reworded around.

## 4. Non-Goals

1. **§3's and §4.1's tables are not bound to the code's enums.** Available, precedented, and a
   different claim from the one `the-guard-pins-the-whole-listing.md`'s guard makes (§2). A guard
   asserting both fails without saying which broke.
2. **§3's tasks change no production code.** `floorBlocks`, `floorAnchorsByLine` and the fence
   regexp are read, not rewritten; nothing is exported; the diff of those tasks is `*_test.go` only.

## 5. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *the binding* | `GroundPartialKinds()` equals the backticked values in §7.2's `partial_kind` cell of `spec/1.0.0.md`, in order, and the extraction fails loudly rather than returning an empty set | add a fourth value to that cell only — **measured: `internal/engine` and `internal/cli` both green today** |
| 1b | §2 *the extraction* | the helper must find the `partial_kind` row and a non-empty value list, asserted before the comparison | an extractor that returns nothing on a reworded table, which makes the equality vacuous and green |
