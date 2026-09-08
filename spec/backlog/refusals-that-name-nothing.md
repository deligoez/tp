# tp — The refusals that name nothing

Class: **tool** — no command, no flag, no workflow field; one seam in what a refusal says. Its
measurements are in `refusals-that-name-nothing-measurements.md` beside it; this file stands without
them. Every citation below names a symbol or a quoted string and the file it lives in, never a line
number.

## 1. Overview

The subject is the place where tp holds the information a reader needs and does not say it: the
ground row's *pairing* refusal — the one a conforming unit hits, because it fires on two values §4.1
itself lists, paired wrongly — names no acceptable set, while the sibling refusal of the same field
was raised to list its legal values and the acceptable set is one lookup away. On 2026-09-08 this
file was split into six specs, one per decision that can ship alone; this one keeps the slug because
it is the file's own headline, and §5 maps every moved section to the slug that took it.

## 2. The pairing refusal names the set §4.1 grants

`internal/engine/groundrow.go` refuses a `tier` cell twice, and only one of the two says what it would
have taken. Both messages, produced by a probe in the copy against `parseGroundRows`, read:

> field "tier": "squinted" is not one of the values the spec lists: read, query, run, probe,
> red-green, break-and-control

> field "tier": "read" says nothing about a "behaviour" claim (§4.1), and a PASS row must be reached
> at a tier that does

The first is `groundEnumCell`, repaired in v1.0.0. The second is `validateGroundRowTier`, in the same
`internal/engine/groundrow.go`, untouched — while `groundAcceptableTiers[KindBehaviour]` holds
`{run, red-green}` in the file next door.

**The second refusal is the one a *conforming* unit hits.** The first fires on a value that is in no
enum — a typo. The second fires on two values §4.1 itself lists, paired wrongly. The census, run over
the shipped predicate rather than counted by eye, is in the sidecar under *§2 — the census of the
`kind × tier` pairings*: most of the pairings a unit can write using only §4.1's own values are
refused by the message that names no set, on the three verdicts that bind the rule, and the ones that
pass are the whole of the rule. The qualifier belongs in the sentence rather than under it:
`groundTierRuleBinds` is true for `PASS`, `PARTIAL` and `FAIL` and false for `UNVERIFIABLE`,
`QUESTION` and `NOT-A-CLAIM`, and `validateGroundRowTier` returns `nil` for the latter three, so a
`QUESTION` row carrying `{behaviour, read}` is accepted and never sees this message at all. A unit
that reads the enum listing and picks a legal tier is *more* likely to land here than one that
mistypes.

The recovery costs a re-read of the emitted prompt, which is the argument v1.0.0 already accepted for
the other four cells; the prompt does carry the per-kind sets, rendered by `groundPromptEvidence` in
`internal/cli/ground.go` from `TierAcceptableFor`. How many bytes that round trip is, this file quotes
nowhere, because the figure is a property of the spec being grounded and not of tp — the sidecar's
*§2 — the cost of the round trip* gives the derivation. `internal/engine/groundrow_test.go`'s
enum-refusal doc comment states a figure for that quantity; `guards-read-what-production-reads.md`
edits that comment, and the stale figure is deleted with it rather than re-measured.

### 2.1 The message, and the one derivation behind it

**The message gains the set, rendered from the same predicate the prompt renders from:**

> "read" says nothing about a "behaviour" claim (§4.1), and a PASS row must be reached at a tier that
> does: run, red-green

**One derivation, two sinks.** The refusal and the emitted prompt name the same acceptable set for a
kind, in §4.1's table order, because both are rendered from one derivation rather than from two. This
is a move, not a new abstraction: the sets already exist, the filter already exists in `cli`, and
putting the two sinks on one derivation is what makes "the prompt and the refusal agree" a fact
rather than a hope; the mechanics are in the sidecar under *Implementation notes from the original
body*. `groundPromptEvidence`'s own doc already gives the reason — a prompt stating the rule in its
own words can drift from the recorder, and the unit pays for the drift with a refused round.

**The empty-set branch is written rather than defaulted.** A kind outside the seven has no acceptable
tier, and a message ending in `does: ` with nothing after it is worse than the message it replaces;
the refusal instead states that no tier is acceptable for that kind. All seven kinds have at least
one acceptable tier (the census in the sidecar), so this is unreachable through `ParseGroundRow`,
where `ParseGroundKind` closes the enum — but it is **not** unreachable through a direct call. **No
shipped test reaches it, and the release adds the caller rather than riding on one**; the sidecar's
*Implementation notes from the original body* names the two direct callers that exist and why neither
enters the branch. §4 pins it.

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
is not a rewording — it is the whole-artifact shape `the-guard-pins-the-whole-listing.md` uses, and
REFERENCE.md is not a bounded artifact.

## 3. Non-Goals

1. **No general validation of message text.** Two messages get a guard, both because they are bounded
   artifacts ending in a listing derived from a shipped table. Every other refusal in `groundrow.go`
   keeps the assertion it has — `rowErr.Field` as a typed value, which is the field-naming contract
   and is stronger than any assertion over a sentence.
2. **Nothing changes what any refusal accepts.** `groundAcceptableTiers`, `groundTierRuleBinds`,
   `TierAcceptableFor` and the four enum listings are unchanged; the same rows are rejected before and
   after. §2.1 changes what a rejection *says*, and §4 pins that the accepted and rejected sets are
   identical across the change.
3. **The other findings v1.0.0's audit carried are not here.** The `FAIL`-not-permanent-in-the-
   deletion-direction claim is `spec/backlog/what-the-carry-can-promise.md`'s, and the empty-floor
   ask's collapsed zeros are `spec/backlog/the-floor-names-what-it-cut.md` §4's; neither is a refusal that
   names nothing. The guard/production parser split is
   `spec/backlog/guards-read-what-production-reads.md`'s, as tasks.

## 4. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 | the pairing refusal for `{behaviour, read, PASS}` **ends** with `": "` followed by `run, red-green` — the tiers acceptable for `KindBehaviour`, joined with `", "` | the shipped message, which ends at `a tier that does` and names no set — and no test in the tree asserts on its text at all: **measured, rewording the format string leaves `go test ./... -count=1` green in every package** |
| 2 | §2.1 *one derivation* | the set the refusal renders and the set `groundPromptEvidence` prints for the same kind are produced by the same call, asserted by comparing the two rendered strings for all seven kinds | re-inline the filter loop in `ground.go`, which is the shipped shape and lets prompt and recorder drift silently |
| 3 | §2.1 *the empty set* | `validateGroundRowTier` called directly with a kind outside the seven produces a message that states no tier is acceptable, and does not end in `": "` with nothing after it | render the empty set through the same join, producing a refusal ending in a colon — **and the caller is new work, not a ride: the tree's only direct callers are `TestAVerdictOutsideTheSixIsHeldToTheTierRule`'s two, which both pass `Kind: KindBehaviour`, so no shipped test reaches this branch** |
| 4 | §2 *acceptance unchanged* | every one of the 42 `kind × tier` pairings gets the same accept/reject answer before and after, with 9 accepted and 33 refused | change a set in `groundAcceptableTiers` while raising the message, which is the way a "message-only" change stops being one |
| 5 | §2.1 *the document* | `skills/tp/REFERENCE.md` contains the refusal string this build produces for `{behaviour, read, PASS}`, the needle derived from the shipped format rather than restated | reword the format string without touching REFERENCE.md — **measured: `go test ./... -count=1` is green in every package with the doc quoting a sentence tp no longer produces** |

**Row 4 is the one an implementer will be tempted to skip.** §2 is described everywhere above as a
message change, and a message change is exactly what an accidental edit to `groundAcceptableTiers`
would hide inside. The 42-pairing census is one loop and it is what makes "nothing changes what any
refusal accepts" (Non-Goal 2) a checked claim instead of an intention.

## Split 2026-09-08

Six independent decisions were split out of this file, one per spec that can ship alone. The
sections it used to carry map to them as follows; the sidecar splits the same way, each new spec
carrying a `<slug>-measurements.md` beside it.

| was | subject | now |
|---|---|---|
| §1 bullet 1, §2, §2.1 | the pairing refusal names the tiers it would have accepted | this file |
| §1 bullets 2–3, §3 | the enum-refusal guard pins the whole listing, not its members | `the-guard-pins-the-whole-listing.md` |
| §4, §10 | the guard's doc comment, the §7.2 binding it promised, and the three hand-parsing test helpers | `guards-read-what-production-reads.md` |
| §1 bullet 4, §5, §5.1 | an unreadable file is named on stderr, not dropped in silence | `an-unreadable-file-is-named.md` |
| §1 bullet 5, §6 | an unreadable or unparseable task file is reported by `tp lint`, not swallowed | `an-invalid-task-file-is-reported.md` |
| §1 bullet 6, §7 | a finding's `Context` is cut on a rune boundary | `context-is-cut-on-a-rune-boundary.md` |
| §8 Non-Goals 1–2, 9 | — | this file, §3 |
| §8 Non-Goals 3, 5, 6 | — | `an-unreadable-file-is-named.md` |
| §8 Non-Goals 4, 10 | — | `guards-read-what-production-reads.md` |
| §8 Non-Goal 7 | — | `an-invalid-task-file-is-reported.md` |
| §8 Non-Goal 8 | — | `context-is-cut-on-a-rune-boundary.md` |
| §9 rows 1, 2, 4, 5, 6 | — | this file, §4 rows 1–5 |
| §9 rows 3a, 3b, 3c | — | `the-guard-pins-the-whole-listing.md`, rows 1a–1c |
| §9 rows 7, 7b | — | `guards-read-what-production-reads.md`, rows 1, 1b |
| §9 rows 8, 8b, 8c, 8d | — | `an-unreadable-file-is-named.md`, rows 1, 1b, 1c, 1d |
| §9 rows 9, 9b, 9c | — | `an-invalid-task-file-is-reported.md`, rows 1, 1b, 1c |
| §9 rows 10, 10b | — | `context-is-cut-on-a-rune-boundary.md`, rows 1, 1b |
