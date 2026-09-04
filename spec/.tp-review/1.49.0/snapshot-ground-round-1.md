# tp v1.49.0 — `next_action` recommends the delta pass

> **This file is decisions.** Two drafts of this release added a flag and inverted a threshold; both
> are recorded below rather than quietly fixed, because each was the obvious move and the reason it
> is wrong is not visible from the change itself.

## 1. Overview

Any `fixed` disposition voids a round — `--record` refuses it, correctly, because a fix means the
spec changed and the round's findings were read against older text. The cost is that a **one-section
repair forces a full five-role panel**.

**The pass that solves this already ships.** `tp review <spec> --perspective regression` emits the
regression role alone, derives its scope from the newest earlier snapshot through `DiffSections`
(`internal/engine/diff.go:41`), and reports `Round: 0`, *"This pass records no state"* and
`Convergence: "uncounted delta pass — counted rounds stay full-panel"`
(`internal/cli/review_regression.go:126`).

**This release adds one thing: `next_action` learns to recommend it.** Today an operator finds the
pass in `skills/tp/SKILL.md:75` or does not run it at all.

## 2. No flag is added

An earlier draft proposed `--delta`, justified as *"generalizing the existing pass by deriving the
scope instead of taking it"*. **The derivation is already the shipped default**, and taking the scope
(`--diff-from` + `--findings`) is the shipped opt-in. The flag would have been a second spelling of a
shipped feature, and its emitted output — round zero, no state, that convergence line — is verbatim
what `--perspective regression` produces today.

**What is missing is a recommendation, not a capability**, and the two are easy to confuse because
both end in the operator running something new.

## 3. The branch

It fires when **all three** hold:

- the last recorded round produced findings, and
- the repairs since touched **more than three sections**, and
- no section was added or removed.

Outside that, `next_action` names a full counted round as it does today.

**The threshold direction is `SKILL.md`'s, and an earlier draft inverted it.** `skills/tp/SKILL.md:75`
has carried *"when a fix batch touched **more than 3 sections**, run the standalone regression delta
pass"* since v0.28.0; the draft wrote *at most three*. The direction has a reason: **a large repair is
what needs the cheap look**, while a one-section repair is small enough that the next counted round
covers it. The rule now has one home and this release agrees with it rather than restating it.

**The added/removed clause is not a refinement.** `DiffSections` reports a section addition or removal
as a change with no counterpart to compare against, so a regression pass over it has nothing to
regress. That is a different question from *did this repair drift*, and it belongs to a counted round.

**A delta pass can never move the loop toward convergence.** It is a cheap look at a large repair,
never a substitute for a round; only a counted full-panel round advances the count. That is already
true of the shipped pass and this release does not change it.

## 4. The branch is omitted while a run is driving

**When `TP_RUN_ID` is set, `next_action` names the counted round instead.**

Eight unit kinds are fixed in `internal/engine/unitkind.go` and none of them runs a delta pass. So an
unattended driver reaching this recommendation would be told to run work it has **no unit for** — the
same failure already recorded for a different branch, where `next_action` recommended a registration
the phase could not honour.

**A ninth unit kind is not on the table.** It would reopen a review-converged spec mid-implementation
to buy a recommendation a human can already read. The branch is for an operator at a terminal.

**Delta findings go where the invoking operator sends them** — `-o`, defaulting to stdout as today —
and never into `$TP_ROUND_DIR`. An uncounted pass must not write the counted round's regression file,
which is reserved for a role unit the driver spawned.

## 5. Non-Goals

1. **No new flag.** §2.
2. **No new unit kind.** §4.
3. **No convergence effect.** The recommendation gates nothing, counts nothing, and records nothing;
   `next_action` has never gated an exit code and does not begin here.
4. **No change to `--perspective regression` itself.** Its scope derivation, its round-zero reporting
   and its output destination are unchanged. This release reads it; it does not touch it.
5. **The threshold is not configurable.** A workflow field for it would give the rule a second home,
   which is the condition that let an earlier draft invert it undetected.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | a repair touching **four** sections after a round with findings recommends the delta pass; **three** does not | invert the comparator — the draft's own error, which recommends the cheap look for small repairs and the full panel for large ones |
| 2 | §3 *boundary* | exactly three sections does **not** fire, and exactly four does — both asserted | test with one and ten, which passes whether the bound is inclusive or exclusive; this repository has already checked a documented 1–60 range with the value 999 |
| 3 | §3 *added* | a repair that adds or removes a section names the counted round even at ten sections changed | ignore the clause, recommending a regression pass over text with nothing to regress against |
| 4 | §3 *clean* | a last round with **no** findings names the counted round, whatever the section count | fire on section count alone, recommending a delta pass after a clean round |
| 5 | §4 | with `TP_RUN_ID` set, the branch never fires — asserted on the same input that fires without it | emit it unconditionally, telling a driver to run a unit kind that does not exist |
| 6 | §5.3 | `next_action`'s exit code and the recorded round are identical with and without the branch | let it record or gate, turning a recommendation into a step |
| 7 | §2 | no new flag is registered — asserted against the command's flag set, not its help text | add `--delta`, shipping a second spelling of `--perspective regression` |

**Row 2 is the one a single-case test would skip.** The threshold is the only number in this release
and the draft that inverted it would have passed row 1 written with one and ten; only asserting both
sides of the boundary separates the two directions.
