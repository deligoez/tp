# tp v1.49.0 — `next_action` recommends the delta pass

> **This file is decisions.** One recorded draft of this release added a flag *and* inverted a
> threshold; both errors are recorded below rather than quietly fixed, because each was the obvious
> move and the reason it is wrong is not visible from the change itself. **One draft, not two** —
> `git log --oneline --reverse -S'tp review <spec> --delta' -- spec/0.36.0.md` and the same command
> for `at most three` return the *same* commit list, and its earliest commit introduces both errors
> at once: `git show <c>~1:spec/0.36.0.md` contains neither string, `git show <c>:spec/0.36.0.md`
> contains both. Uncommitted drafts are a fact only the author holds; this file does not count them.

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
pass in `skills/tp/SKILL.md` — `git grep -n 'more than 3 sections' HEAD -- skills/tp/SKILL.md`
returns line 126 — or does not run it at all. The flag's own help does not lead there either:
`tp review --help` describes `--perspective` as *"documentation, testing, or code-audit"* and never
names `regression`, though `reviewPerspectives` (`internal/cli/review.go:512`) accepts it. That is a
defect in `--perspective`'s help string rather than in `next_action`; it is named here because it is
why the pass goes unrun, and it is deliberately **not** fixed by this release.

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

Outside that, `next_action` is unchanged — which is not the same as "names a counted round".
`ReviewNextAction` (`internal/engine/nextaction.go:48-61`) has four branches and the converged one
names no round at all. Measured in a probe: two clean recorded rounds give
`"decompose the spec into tasks, then tp import s.tasks.json"`, and one round with findings gives
`"revise the spec to address the blocking findings, then run the next review round"`.

**The threshold direction is `SKILL.md`'s, and the recorded draft inverted it.** `skills/tp/SKILL.md`
has carried *"when a fix batch touched **more than 3 sections**, run the standalone regression delta
pass"* since `f747c354` — `git log --reverse -S'more than 3 sections' -- skills/tp/SKILL.md` returns
that one commit, and `git tag --contains f747c354 --sort=creatordate | head -1` is **v0.23.0**. The
draft wrote *at most three*. The direction has a reason: **a large repair is what needs the cheap
look**, while a one-section repair is small enough that the next counted round covers it. The rule
now has one home and this release agrees with it rather than restating it.

**The added/removed clause is not a refinement.** `DiffSections` reports a section addition or removal
as a change with no counterpart to compare against, so a regression pass over it has nothing to
regress. That is a different question from *did this repair drift*, and it belongs to a counted round.

**A delta pass can never move the loop toward convergence** — because it writes no state, not because
of anything about panels. Measured in an `rsync` copy: `tp review spec/0.37.0.md --perspective
regression` wrote its payload to stdout and touched zero files (`touch stamp`, run, then
`find . -type f -newer stamp | wc -l` → 0). The panel is **not** the mechanism, and an earlier form of
this paragraph said it was: `tp review s.md --record f.ndjson` on a findings file carrying **one**
role's row returned `{"round": 1}` and set `consecutive_clean`, so `--record` alone advances the
count. *"Counted rounds are always full-panel"* is policy prose in `skills/tp/SKILL.md:129`, not
something the mechanism enforces. That is already true of the shipped pass and this release does not
change it.

## 4. The branch is omitted while a run is driving

**When `TP_RUN_ID` is set, `next_action` names the counted round instead.**

Eight unit kinds are fixed in `internal/engine/unitkind.go` and none of them runs a delta pass. So an
unattended driver reaching this recommendation would be told to run work it has **no unit for** — the
same failure already recorded for a different branch, where `next_action` recommended a registration
the phase could not honour.

**A ninth unit kind is not on the table.** It would reopen a review-converged spec mid-implementation
to buy a recommendation a human can already read. The branch is for an operator at a terminal.

**Delta findings go to stdout and never into `$TP_ROUND_DIR`.** Both halves are the shipped
behaviour, measured in the run above: the payload is written to stdout by `output.JSON`, and no file
is touched. **`-o` is not the mechanism, and this file said it was until grounding ran it** —
`tp review spec/0.37.0.md --perspective regression -o /tmp/delta.json` exits **2** with
`{"error":"-o/--output requires --merge","code":2,…}`, writes nothing to stdout and creates no file.
`internal/cli/review.go:403` registers the flag as *"Output file path (for --merge)"* and
`review.go:311` is the refusal. An operator who wants the payload in a file redirects stdout. An
uncounted pass must not write the counted round's regression file, which is reserved for a role unit
the driver spawned.

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

Every row derives from a numbered decision. Two universals over these seven rows are **not** asserted,
because an earlier form of this preamble asserted each and grounding falsified both. The table is seven
rows long and the author could have counted; that is the whole of the lesson.

**Not "every row names a mutant that must fail it".** Six name a mutant of the *product*. **Row 2 alone
names a mutant of the test**, and a weakened test cannot fail an assertion, so the sentence was false
for exactly one row. Row 2 is kept as a different kind of row, deliberately: what it guards is not a
way the branch can be wrong but a way its own guard can be. Under §5's matrix the inclusive mutant
`n >= 3` already fails row 1 as written — row 1 asserts 4 fires and 3 does not, and `n >= 3` fires on
3 — so the boundary is closed by row 1, and row 2 is not closing it. Row 2 constrains **how row 1 is
implemented**: with 3 and 4 rather than with 1 and 10, which passes whichever way the bound goes.

**Not "every row names an artifact".** Counting a `path:line` citation anywhere in the row as naming
one, **exactly one of the seven does** — row 2's `internal/engine/lock_timeout_range_test.go:13-33`.
The count is what `grep -cE '^\| [1-7] \|.*\.go:[0-9]' spec/1.49.0.md` prints, against
`grep -cE '^\| [1-7] \|' spec/1.49.0.md` for the denominator. **Anchored on the row shape rather than
on line numbers, and that is not fastidiousness.** The first form of this derivation was
`sed -n '121,127p' … | grep -c`, written after the edit that pushed the table thirteen lines down: it
printed `1`, the right answer, off a sentence in this very paragraph rather than off the table. Adding
a second `.go:` citation to row 5 in a copy takes the anchored form to `2` and leaves the line-pinned
form at `1` — so only one of the two could ever have failed. The earlier sentence's *explanation* for
the exception was wrong too: it blamed rows whose `from` column is a bare section reference, and row
2's `from` is not bare while four rows' are — `grep -cE '^\| [1-7] \| §[0-9.]+ \|' spec/1.49.0.md`.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | a repair touching **four** sections after a round with findings recommends the delta pass; **three** does not | invert the comparator — the draft's own error, which recommends the cheap look for small repairs and the full panel for large ones |
| 2 | §3 *boundary* | exactly three sections does **not** fire, and exactly four does — both asserted | test with one and ten, which passes whether the bound is inclusive or exclusive; this repository has already checked a documented 1–60 range with the value 999 (`internal/engine/lock_timeout_range_test.go:13-33`) |
| 3 | §3 *added* | a repair that adds or removes a section does **not** fire the branch, even at ten sections changed | ignore the clause, recommending a regression pass over text with nothing to regress against |
| 4 | §3 *clean* | a last round with **no** findings does **not** fire the branch, whatever the section count — asserted on not-firing, because what `next_action` says instead depends on whether the spec converged | fire on section count alone, recommending a delta pass after a clean round |
| 5 | §4 | with `TP_RUN_ID` set, the branch never fires — asserted on the same input that fires without it | emit it unconditionally, telling a driver to run a unit kind that does not exist |
| 6 | §5.3 | the command's exit code and the recorded round are identical with and without the branch — `next_action` is a string in the payload and has no exit code of its own | let it record or gate, turning a recommendation into a step |
| 7 | §2 | no new flag is registered — asserted against the command's flag set, not its help text | add `--delta`, shipping a second spelling of `--perspective regression` |

**Row 2 is the one a single-case test would skip, and what it separates is the boundary, not the
direction.** Evaluate the three comparators against the two shapes row 1 could take — row 1 as
written asserts that four fires and three does not; row 1 with one and ten asserts that ten fires and
one does not:

| comparator | row 1 (4 fires, 3 does not) | row 1 (10 fires, 1 does not) |
|---|---|---|
| correct, `n > 3` | passes | passes |
| inverted, `n <= 3` | fails | **fails** |
| inclusive, `n >= 3` | fails | **passes** |

So the inverting draft would **not** have passed row 1 written with one and ten: inverting a
comparator inverts every verdict, and any pair straddling the threshold catches it. What one and ten
cannot separate is `> 3` from `>= 3` — the boundary — which is what row 2's own mutant column says and
what row 2 exists to close.
