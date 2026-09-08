# tp — The guard pins the whole listing

Class: **tool** — no command, no flag, no workflow field; one test-shape decision over the four enum
refusals in `internal/engine/groundrow.go`. Its measurements are in
`the-guard-pins-the-whole-listing-measurements.md` beside it; this file stands without them. Every
citation below names a symbol or a quoted string and the file it lives in, never a line number.

## 1. Overview

The subject is the guard that holds tp's four ground enum refusals to naming the values they would
have accepted: it is a lower bound rather than an equality, so truncating a listing reddens it and
appending to one does not, and a refusal telling a unit that a *kind* is a legal tier passes the
guard written to make that message informative. This spec was split out of
`refusals-that-name-nothing.md` on 2026-09-08, where it was §3 of that file, whose forwarding table
maps every moved section to its new slug.

## 2. The enum-refusal guard asserts the listing, not its members

**This spec exists because a guard was written whose mutant nobody ran in the other direction.** The
four enum refusals were repaired and a test was written to hold them; the test was checked by
*removing* values and never by *adding* them. That is the shape this release is about one level up,
and §4 says so as a rule: every row of the test table names the mutant that must fail it, and the
rows here carry one in each direction.

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

**`refusals-that-name-nothing.md`'s pairing refusal gets the same guard**, against the tiers
acceptable for the kind, by the same shape.

## 3. Non-Goals

1. **No general validation of message text.** Two messages get a guard, both because they are bounded
   artifacts ending in a listing derived from a shipped table. Every other refusal in `groundrow.go`
   keeps the assertion it has — `rowErr.Field` as a typed value, which is the field-naming contract
   and is stronger than any assertion over a sentence.

## 4. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. **§2 gets three rows — 1a, 1b, 1c — and the first two are one mutant in each
direction**, because §2 exists precisely because the guard it replaces was only ever mutated one way;
1c pins the repair that would otherwise be reached for instead.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1a | §2 *truncation* | truncating any of the four enum listings at the production call site reddens the guard | `GroundTiers()[:2]`, which the shipped guard already catches — and the *any of the four* is **counted rather than assumed**: the truncation was applied at each of the four call sites and each of the four subtests fails on its own. This row records that the existing direction is not lost |
| 1b | §2 *appending* | appending a value to any of the four listings reddens the guard | `append(GroundTiers(), "document", "corpus", "vibes")`, and the same append on all four call sites at once: **measured green in `internal/engine` and `internal/cli` under the shipped guard** |
| 1c | §2 *the refused repair* | `Contains(msg, ": "+joined)` is **not** the fix — the guard must still redden under the appending mutant with that assertion in place | ship the containment form, which is green on the appended message because the joined string is still a substring (measured: `Contains`=true, `HasSuffix`=false) |
