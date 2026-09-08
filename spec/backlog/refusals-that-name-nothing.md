# tp — The refusals that name nothing

Class: **tool** — no command, no flag, no workflow field; three seams in what a refusal says, plus
three test-only tasks. Its measurements are in `refusals-that-name-nothing-measurements.md` beside it;
this file stands without them. Every citation below names a symbol or a quoted string and the file it
lives in, never a line number.

## 1. Overview

The places where tp holds the information a reader needs and does not say it, and one guard that was
written to close the first of them and closes a weaker claim instead:

1. **The pairing refusal names no acceptable set** (§2). `groundEnumCell` was raised to list its legal
   values; `validateGroundRowTier` — the *other* refusal of the same field — was not.
2. **The guard that raised the four is a lower bound, not equality** (§3). Truncating a listing
   reddens it; appending to one does not.
3. **That guard's doc comment claims a binding to the document that does not exist** (§4), and the
   sentence is wrong in two independent ways rather than one.
4. **`rankFilesBySpecTerms` drops an unreadable file in silence** (§5) while the next function in the
   same file documents the opposite convention as a convention.
5. **`checkTaskFileQuality` swallows both of its failures** (§6) — an unreadable task file and an
   unparseable one each produce zero findings, empty stderr and exit 0, indistinguishable from clean.
6. **`vague.go` truncates a finding's `Context` by bytes** (§7), so a multi-byte rune on the
   boundary reaches the reader as `U+FFFD` — text that is in no document. The two tests pinning the
   cap assert a byte length, which the corruption satisfies.

They share a subject and not a mechanism: in each, the failing party already holds the answer. §2 has
`groundAcceptableTiers[kind]` one lookup away, §3 has the listing it was handed, §4 has a spec-reading
helper in the same file, §5 has the file path and the `error`, §6 has the `error` twice over, and
§7 has the rune boundary `utf8.DecodeLastRuneInString` would give it.

**§3 exists because a guard was written whose mutant nobody ran in the other direction.** The four
enum refusals were repaired and a test was written to hold them; the test was checked by *removing*
values and never by *adding* them. That is the shape this release is about one level up, and §9 says
so as a rule: every row of the test table names the mutant that must fail it, and the §3 rows carry
one in each direction.

§10 carries three test-only tasks absorbed from the guard-helper spec, which shared this release's
subject: a guard that re-parses a document by hand holds a weaker answer than production does.

## 2. The pairing refusal names the set §4.1 grants

`internal/engine/groundrow.go` refuses a `tier` cell twice, and only one of the two says what it would
have taken. Both messages, produced by a probe in the copy against `parseGroundRows`:

```
field "tier": "squinted" is not one of the values the spec lists: read, query, run, probe, red-green, break-and-control
field "tier": "read" says nothing about a "behaviour" claim (§4.1), and a PASS row must be reached at a tier that does
```

The first is `groundEnumCell`, repaired in v1.0.0. The second is `validateGroundRowTier`, in the same
`internal/engine/groundrow.go`, untouched — while `groundAcceptableTiers[KindBehaviour]` holds
`{run, red-green}` in the file next door.

**The second refusal is the one a *conforming* unit hits.** The first fires on a value that is in no
enum — a typo. The second fires on two values §4.1 itself lists, paired wrongly. The census, run over
the shipped predicate rather than counted by eye:

```go
// probe: for each kind × tier ask TierAcceptableFor; and which verdicts bind the rule
kinds=7 tiers=6 total=42 acceptable=9 refused_by_tier_rule=33
binds=PASS,PARTIAL,FAIL
```

So **33 of the 42 pairings a unit can write using only §4.1's own values are refused by the message
that names no set — on the three verdicts that bind the rule** — and the nine that pass are the whole
of the rule. The qualifier belongs in the sentence rather than under it: `groundTierRuleBinds` is true
for `PASS`, `PARTIAL` and `FAIL` and false for `UNVERIFIABLE`, `QUESTION` and `NOT-A-CLAIM`, and
`validateGroundRowTier` returns `nil` for the latter three, so a `QUESTION` row carrying
`{behaviour, read}` is accepted and never sees this message at all. A unit that reads the enum listing
and picks a legal tier is *more* likely to land here than one that mistypes.

The recovery costs a re-read of the emitted prompt, which is the argument v1.0.0 already accepted for
the other four cells; the prompt does carry the per-kind sets, rendered by `groundPromptEvidence` in
`internal/cli/ground.go` from `TierAcceptableFor`. How many bytes that round trip is, this file quotes
nowhere, because the figure is a property of the spec being grounded and not of tp — the sidecar's
*§2 — the cost of the round trip* gives the derivation. `internal/engine/groundrow_test.go`'s
enum-refusal doc comment states a figure for that quantity; §4 edits that comment, and the stale
figure is deleted with it rather than re-measured.

### 2.1 The message, and the one derivation behind it

**The message gains the set, rendered from the same predicate the prompt renders from:**

```
"read" says nothing about a "behaviour" claim (§4.1), and a PASS row must be reached at a tier that does: run, red-green
```

**One derivation, two sinks.** `engine.AcceptableTiersFor(kind)` returns the kind's tiers filtered out
of `GroundTiers()`, so the order is §4.1's table order; `validateGroundRowTier` and
`groundPromptEvidence` both call it, and the per-kind filter loop currently inlined inside
`groundPromptEvidence` — the one walking `GroundKinds()` × `GroundTiers()` and asking
`engine.TierAcceptableFor` — is replaced by that call. This is a move, not a new abstraction: the sets
already exist, the filter already exists in `cli`, and putting it beside `TierAcceptableFor` is what
makes "the prompt and the refusal agree"
a fact rather than a hope. `groundPromptEvidence`'s own doc already gives the reason — a prompt
stating the rule in its own words can drift from the recorder, and the unit pays for the drift with a
refused round.

**The empty-set branch is written rather than defaulted.** `AcceptableTiersFor` on a kind outside the
seven returns nothing, and a message ending in `does: ` with nothing after it is worse than the
message it replaces. All seven kinds have at least one acceptable tier (the census above: 9 across 7),
so this is unreachable through `ParseGroundRow`, where `ParseGroundKind` closes the enum — but it is
**not** unreachable through a direct call. **No shipped test reaches it, and the release adds the
caller rather than riding on one.** The tree's only direct calls to `validateGroundRowTier` outside
`ParseGroundRow` are the two in `TestAVerdictOutsideTheSixIsHeldToTheTierRule`
(`internal/engine/groundtierrule_test.go`), and both pass `Kind: KindBehaviour`, whose acceptable set
is `{run, red-green}`; the out-of-enum value there is the *verdict*, not the kind. The branch says
that no tier is acceptable for that kind, and §9 pins it.

**`skills/tp/REFERENCE.md` quotes this message verbatim and nothing binds the quote to the code.** It
sits there in §7.2's per-verdict table, in the `PASS`, `PARTIAL` or `FAIL` row. Measured: rewording
the format string to `"%q is not evidence about a %q claim (§4.1), and a %s row must be reached at a
tier that is"` and running `go test ./... -count=1` leaves **every package green** while REFERENCE.md
documents a sentence tp no longer produces. So this release updates REFERENCE.md and adds the binding,
on `TestDocsCarryTheConvergenceSignalWording`'s precedent — `internal/cli/docs_contract_test.go`,
`assert.Contains(reference, engine.DivergenceHint, …)`, which requires REFERENCE.md to quote the
shipped constant. **In package `cli`, not `engine`:** `internal/engine/divergence_test.go` only
*names* that test, inside a doc comment, and this paragraph first cited the doc comment as though it
were the assertion. The test here *produces* the refusal for `{behaviour, read, PASS}` and requires
REFERENCE.md to contain that exact string.

**The limit of that binding, stated rather than implied.** It establishes that the quote is present
and current; it cannot establish that no *other* paragraph of REFERENCE.md contradicts it, because a
`Contains` is a local assertion inside an unbounded text and the complement is free. The replacement
is not a rewording — it is the whole-artifact shape §3 uses, and REFERENCE.md is not a bounded
artifact.

## 3. The enum-refusal guard asserts the listing, not its members

`TestAnEnumRefusalNamesTheValuesItWouldHaveAccepted` (`internal/engine/groundrow_test.go`) loops over
the four enum cells and, for each, asserts `assert.Contains(err.Error(), want)` **once per value of
that cell's listing** — the rule, because there is no one per-cell count to state. Nothing is asserted
about what else the message says.

Measured in both directions on the shipped guard, truncating any listing at its production call site
reddens it and appending values to any listing leaves it green, so a refusal telling a unit that
`document` — a *kind* — is a legal tier passes the guard written to make the message informative; the
runs are in the sidecar under *§3 — measured, both directions*.

**The immune shape is available, because the message is a bounded artifact.** One string, one
listing, no elsewhere. But the obvious repair does not work, and it was measured before being
rejected: `Contains` over the joined string is green on the exact mutant it would be written for,
because appending leaves the joined string a substring (the sidecar's second §3 table). **The
assertion is on the message's tail**: the refusal must *end* with `": "` followed by the listing the
call site passed, joined with `", "`. That is exhaustive against appending, prepending and insertion,
and it pins no word of the sentence before the colon.

**Its one cost, named because it is the correct direction.** A future message that adds a clause
*after* the listing turns the guard red and forces whoever adds it to re-derive the assertion. That is
loud. The alternative — a containment check that stays green while the listing rots — is the failure
this section exists to close.

**§2's message gets the same guard**, against `AcceptableTiersFor(kind)`, by the same shape.

## 4. The guard's doc comment, and the one binding it promised

The same test's doc comment says:

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
  The three helpers are the subject of §10's first task, and the reader this bullet adds goes through
  the helper that task introduces.
- The comment then says what is true: three of the four listings are pinned to §3 and §4.1 by the
  code's own tables and by nothing in this test, and `partial_kind`'s is pinned to §7.2.

**Binding `GroundVerdicts()`, `GroundKinds()`, `GroundTiers()` and `groundAcceptableTiers` to §3's and
§4.1's tables is available and deliberately not taken here** (Non-Goal 4). It is a different claim —
*the code's enum equals the document's table* — from the one this guard makes — *the refusal names the
code's enum* — and a guard that asserts both is a guard whose failure does not say which broke.

## 5. An unreadable file is named, not dropped in silence

`rankFilesBySpecTerms` (`internal/cli/review.go`) scores each candidate file by how many spec
heading terms it contains, and on a read error:

```go
content, err := os.ReadFile(f)
if err != nil {
    continue
}
```

The **next function in the same file**, `readFilesContent`, hits the same condition and its doc comment
states the opposite convention as a convention:

> A file it cannot read is named on stderr rather than dropped in silence: the caller's paths came
> from a directory walk, so an unreadable one is an anomaly, and a role that never sees the body would
> otherwise judge the file from its absence.

**One condition, two siblings, opposite channels.** Measured, both on the same input — three files in
a temp dir, one `chmod 000`, spec lines carrying two headings — with the probe asserting the locked
file is genuinely unreadable before it concludes anything:

| function | in | out | stderr |
|---|---|---|---|
| `rankFilesBySpecTerms` | 3 | **2** | `""` |
| `readFilesContent` | 3 | 2 | `warning: cannot read …/locked.md; its contents were dropped from the prompt (… permission denied)` |

**The ranking drop is the worse of the two, because it is the silent one.** `docStructure.ReviewedFiles`
is `len(ranked)` in both `runReviewDocPlan` and `runReviewTestPlan`, so the emitted JSON says fewer
files were reviewed and says nothing about why. **What the drop removes is the body and the fact of
the drop — not the path**, and this paragraph first claimed the opposite. The emitted prompt falsifies
it: `walkDocTree` builds the `Documentation structure:` tree *upstream* of the ranking and hands it to
the prompt generator, so the dropped filename survives.

### 5.1 The decision

**The drop site gets the sibling's channel, `output.Notice`, and exactly one notice per
unreadable file** — a file dropped from the ranking never reaches `readFilesContent`, so there is no
double report. The silent drop is reachable through `tp review` only under `--spec-inline` or
`--diff-from`, whose rendered sections carry real `## `/`### ` headings — a plain `tp review` finds
zero terms and never enters the drop site — which narrows the blast radius and does not withdraw the
finding, since those are the two modes in which a role is handed the spec's own text; the sidecar's
*§5.1 — how far the drop reaches through the shipped command* is the run.

**Two limits, both stated rather than fixed here.** `output.Notice` returns early under `--quiet`
(`internal/output/output.go`), so under a quiet run the notice is suppressed — the sibling has the
identical limit, and diverging would recreate the asymmetry this section closes. And each of the two
functions issues its own `os.ReadFile`, so a file readable at one and not at the other is possible;
merging the two reads is a refactor, not this release (Non-Goal 5). They do **not** read the same
*set*, and this paragraph first said they did: ranking reads only the rankable files (`index.md` and
`config.*` are diverted into an always-include list it never reads), and `readFilesContent` receives
the ranked list, from which ranking's own drops are already absent. The two-reads hazard survives
either reading; the set claim does not.

**Keeping the file in the ranked list at score 0 was considered and not taken.** It would fix
`ReviewedFiles` as well as the channel, but it changes *what the prompt carries* — an unreadable file
would occupy one of the fifteen slots whenever fewer than fifteen files outscore it — and this release
is about what a refusal says, not about what a selection returns. The count stays wrong; the operator
now learns why.

## 6. An invalid task file is reported by `tp lint`, not swallowed

`checkTaskFileQuality` (`internal/cli/lint.go`) resolves the spec's task file and returns `nil` on
both of its failure paths — `os.ReadFile` erroring and `json.Unmarshal` erroring. A missing file
returning `nil` is correct and is a third, separate branch; the other two are not the same case.
Measured against a control at `HEAD`: an unparseable task file and an unreadable one each yield zero
`acceptance-quality` findings, zero bytes on stderr and exit 0, against the control's one finding —
the sidecar's *§6 of the spec — `checkTaskFileQuality`, measured* is the table, with the precondition
that the unreadable file is asserted unreadable first.

Nothing in the payload, on stderr or in the exit code separates a swallowed failure from a clean
lint, so an operator who corrupts a task file is told their spec is fine. It is §5's mechanism in a
different command: the failing party holds the `error` and drops it. The defect predates `v1.0.0`.

**The decision.** Both failure paths become one `error`-severity finding, rule `task-file-invalid`,
naming the task file's path and the underlying error, so `tp lint` exits non-zero on it; the
`acceptance-quality` walk is skipped for that file, since there is nothing to walk. The missing-file
branch is unchanged: a spec with no task file is the ordinary state before decomposition, and it stays
silent. The check's own read error is reported through the same finding rather than through stderr,
because `--json` consumers read findings and not stderr, and a lint failure that only one channel
carries is the class §5 closes.

## 7. `Context` is truncated on a rune boundary

`internal/engine/vague.go` caps a finding's `Context` at 80 with `ctx = ctx[:80]` in two rules,
`duplicate-line` and `duplicate-paragraph`. The slice is by **byte**, so a multi-byte rune straddling
the boundary is cut in half and the reader receives `U+FFFD` — measured at `HEAD` with a duplicated
line of 78 ASCII characters and an em dash, whose emitted `context` ends in `U+FFFD` and, once the
encoder has rendered the orphaned bytes, no longer honours the cap the guard believes it is checking.
The two tests pinning the cap in `internal/engine/lint_test.go` assert `len(f.Context) <= 80`, and
`len` on a Go string is bytes, so the corrupt value passes both. It fires on no document this
repository has; the sidecar's *§7 of the spec — `Context` truncated by bytes, measured* is the run and
the sweep. What makes it worth a row is the guard, not the frequency: a byte-length assertion over a
character-length claim passes identically whether the truncation is correct or not.

**The decision.** The cap stays 80 bytes, and the cut lands on the last rune boundary at or before
it, so the emitted `Context` is always valid UTF-8 and never longer than 80 bytes in the payload.
`utf8.DecodeLastRuneInString`, or slicing on a rune boundary, is one call away in both places, and
the two guards gain a `utf8.ValidString` assertion beside the length they already check.

## 8. Non-Goals

1. **No general validation of message text.** Two messages get a guard, both because they are bounded
   artifacts ending in a listing derived from a shipped table. Every other refusal in `groundrow.go`
   keeps the assertion it has — `rowErr.Field` as a typed value, which is the field-naming contract
   and is stronger than any assertion over a sentence.
2. **Nothing changes what any refusal accepts.** `groundAcceptableTiers`, `groundTierRuleBinds`,
   `TierAcceptableFor` and the four enum listings are unchanged; the same rows are rejected before and
   after. §2.1 changes what a rejection *says*, and §9 pins that the accepted and rejected sets are
   identical across the change.
3. **The audit-side file selection is untouched.** `rankFilesBySpecTerms`'s two call sites are the
   review-phase `runReviewDocPlan` and `runReviewTestPlan` in `internal/cli/review.go`; the audit
   checklist is `selectCodeFiles` in `internal/engine/auditfiles.go`, reached through
   `SelectAuditFiles`, and the two share nothing but the words "file selection". `selectCodeFiles`,
   `CodeFileCap`, `SelectAuditFiles` and `internal/engine/auditfiles.go` are not edited here.
4. **§3's and §4.1's tables are not bound to the code's enums.** Available, precedented, and a
   different claim from the one §3's guard makes (§4). A guard asserting both fails without saying
   which broke.
5. **The two file reads are not merged.** `rankFilesBySpecTerms` and `readFilesContent` each read the
   candidate files; collapsing them into one pass is a refactor with its own cap and ordering
   decisions, and CLAUDE.md's rule is that a repair introducing a new abstraction belongs to the next
   version.
6. **`ReviewedFiles` is not corrected.** §5 makes the drop audible; the count still reports the
   post-drop list. Correcting it means deciding whether an unreadable file was "reviewed", which is a
   contract question about the emitted JSON and not a channel question.
7. **`tp lint` does not validate the task file's schema.** §6 reports a file that cannot be read or
   parsed as JSON; a file that parses and violates `tp validate`'s rules is `tp validate`'s to report,
   as today.
8. **The `Context` cap is not raised, and is not made a character count.** §7 keeps 80 bytes and
   makes the cut honest; what the cap should be is a contract question this release does not open.
9. **The other findings v1.0.0's audit carried are not here.** The `FAIL`-not-permanent-in-the-
   deletion-direction claim is `spec/backlog/what-the-carry-can-promise.md`'s, and the empty-floor
   ask's collapsed zeros are `spec/backlog/ground-command-friction.md`'s; neither is a refusal that
   names nothing. The guard/production parser split is §10's, as tasks.
10. **§10's tasks change no production code.** `floorBlocks`, `floorAnchorsByLine` and the fence
    regexp are read, not rewritten; nothing is exported; the diff of those tasks is `*_test.go` only.

## 9. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. **§3 gets three rows — 3a, 3b, 3c — and the first two are one mutant in each
direction**, because §3 exists precisely because the guard it replaces was only ever mutated one way;
3c pins the repair that would otherwise be reached for instead.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 | the pairing refusal for `{behaviour, read, PASS}` **ends** with `": "` followed by `run, red-green` — the tiers `AcceptableTiersFor(KindBehaviour)` returns, joined with `", "` | the shipped message, which ends at `a tier that does` and names no set — and no test in the tree asserts on its text at all: **measured, rewording the format string leaves `go test ./... -count=1` green in every package** |
| 2 | §2.1 *one derivation* | the set the refusal renders and the set `groundPromptEvidence` prints for the same kind are produced by the same call, asserted by comparing the two rendered strings for all seven kinds | re-inline the filter loop in `ground.go`, which is the shipped shape and lets prompt and recorder drift silently |
| 3a | §3 *truncation* | truncating any of the four enum listings at the production call site reddens the guard | `GroundTiers()[:2]`, which the shipped guard already catches — and the *any of the four* is **counted rather than assumed**: the truncation was applied at each of the four call sites and each of the four subtests fails on its own. This row records that the existing direction is not lost |
| 3b | §3 *appending* | appending a value to any of the four listings reddens the guard | `append(GroundTiers(), "document", "corpus", "vibes")`, and the same append on all four call sites at once: **measured green in `internal/engine` and `internal/cli` under the shipped guard** |
| 3c | §3 *the refused repair* | `Contains(msg, ": "+joined)` is **not** the fix — the guard must still redden under the appending mutant with that assertion in place | ship the containment form, which is green on the appended message because the joined string is still a substring (measured: `Contains`=true, `HasSuffix`=false) |
| 4 | §2.1 *the empty set* | `validateGroundRowTier` called directly with a kind outside the seven produces a message that states no tier is acceptable, and does not end in `": "` with nothing after it | render the empty set through the same join, producing a refusal ending in a colon — **and the caller is new work, not a ride: the tree's only direct callers are `TestAVerdictOutsideTheSixIsHeldToTheTierRule`'s two, which both pass `Kind: KindBehaviour`, so no shipped test reaches this branch** |
| 5 | §2 *acceptance unchanged* | every one of the 42 `kind × tier` pairings gets the same accept/reject answer before and after, with 9 accepted and 33 refused | change a set in `groundAcceptableTiers` while raising the message, which is the way a "message-only" change stops being one |
| 6 | §2.1 *the document* | `skills/tp/REFERENCE.md` contains the refusal string this build produces for `{behaviour, read, PASS}`, the needle derived from the shipped format rather than restated | reword the format string without touching REFERENCE.md — **measured: `go test ./... -count=1` is green in every package with the doc quoting a sentence tp no longer produces** |
| 7 | §4 *the binding* | `GroundPartialKinds()` equals the backticked values in §7.2's `partial_kind` cell of `spec/1.0.0.md`, in order, and the extraction fails loudly rather than returning an empty set | add a fourth value to that cell only — **measured: `internal/engine` and `internal/cli` both green today** |
| 7b | §4 *the extraction* | the helper must find the `partial_kind` row and a non-empty value list, asserted before the comparison | an extractor that returns nothing on a reworded table, which makes the equality vacuous and green |
| 8 | §5 | on a file set containing one unreadable file, `len(in) - len(out)` equals the number of notices written to stderr | the shipped bare `continue` — **measured: `in=3 out=2 stderr=""`, so 1 ≠ 0; after the fix, `drops=1 notices=1`** |
| 8b | §5 *no double report* | exactly one notice per unreadable file across the ranking and the read that follows it | notice at both sites, which reports twice for one anomaly and makes the count in row 8 wrong in the other direction |
| 8c | §5 *the fixture is valid* | the probe asserts the locked file is unreadable **before** it concludes anything from an empty stderr | run the probe as a user that can read a `chmod 000` file, under which the silent-drop finding cannot occur and the test passes for the wrong reason |
| 8d | §5 *the branch is entered* | the fixture's spec lines carry at least one `## `/`### ` heading, asserted before the drop is counted, so `rankFilesBySpecTerms` does not take its zero-terms early return | a fixture whose spec content is `buildSpecRefContent`'s bullet headings, under which zero terms are found, every candidate is returned **unread**, the drop site is never entered, and the test passes with no drop and no notice |
| 9 | §6 *unparseable* | on a spec whose task file is `{ this is not json`, `tp lint` reports one `task-file-invalid` finding at `error` severity naming the task file's path, and exits non-zero | the shipped `return nil` after `json.Unmarshal`, under which the run is indistinguishable from the control |
| 9b | §6 *unreadable* | on a spec whose task file is `chmod 000` — asserted unreadable first, as in row 8c — `tp lint` reports the same finding naming the read error, and exits non-zero | the shipped `return nil` after `os.ReadFile` |
| 9c | §6 *the control and the missing file* | a valid task file with one short acceptance still yields exactly one `acceptance-quality` finding and no `task-file-invalid`; a spec with no task file yields neither | report the missing-file branch too, which turns every undecomposed spec's lint red |
| 10 | §7 *the cut* | on a duplicated line of 78 ASCII characters and an em dash, both `duplicate-line` and `duplicate-paragraph` emit a `Context` that is valid UTF-8 (`utf8.ValidString`) and at most 80 bytes | the shipped `ctx[:80]`, whose `Context` on that input is 80 bytes of which the last two are an orphaned rune prefix |
| 10b | §7 *the guard* | the two existing cap assertions in `lint_test.go` are joined by `utf8.ValidString` on the same fixture, and the fixture's 80th byte is asserted to fall inside a rune before the assertion runs | a fixture of 80 ASCII characters, under which byte and rune truncation agree and the guard proves nothing |

**Row 5 is the one an implementer will be tempted to skip.** §2 is described everywhere above as a
message change, and a message change is exactly what an accidental edit to `groundAcceptableTiers`
would hide inside. The 42-pairing census is one loop and it is what makes "nothing changes what any
refusal accepts" (Non-Goal 2) a checked claim instead of an intention.

**Row 8c is not decoration.** The `chmod 000` fixture decides the result of §5's entire measurement,
and under a user that can read the file the probe returns three files, empty stderr, and a green test
that has established nothing. The property the verdict rests on is asserted, not assumed. Rows 9b and
10b carry the same rule for their own fixtures.

## 10. Tasks taken from the guard spec

Three test helpers in `internal/engine` — `floorSection21Verbs` (`floor_test.go`),
`groundSection72Fields` (`groundrow_test.go`) and `groundSection72VerdictRule`
(`groundtierrule_test.go`) — re-parse a Markdown section of `spec/1.0.0.md` by hand, and none of
them knows what a fence is, while production parses the same document with a block scanner that
toggles on fences. The guard-helper spec measured fifteen inputs against them, four silent and three
false failures; the table and what it taught are in the sidecar under *From the guard spec*. Its
decisions land here as three tasks, all `*_test.go` only (Non-Goal 10):

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
