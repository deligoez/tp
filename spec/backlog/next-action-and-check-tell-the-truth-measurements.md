# next-action-and-check-tell-the-truth — measurements

Supplemental material for `next-action-and-check-tell-the-truth.md`; the spec stands without it.

The blocks below moved verbatim from the delta-pass draft this spec absorbed. The grounding
measurement behind the `--check` gate — the recorded round with `FAIL`s standing that exits 0, and
the two-unit fixture reproducing it — is in `ground-command-friction-measurements.md` under
"§9 `--check` exits 0 with `FAIL`s standing". `git show <ref>:<path>` commands keep the path at
that ref; every other citation was rewritten to the file's current name.

## One draft, not two (the absorbed draft's front matter)

> **This file is decisions.** One recorded draft of this release added a flag *and* inverted a
> threshold; both errors are recorded below rather than quietly fixed, because each was the obvious
> move and the reason it is wrong is not visible from the change itself. **One draft, not two** —
> `git log --oneline --reverse -S'tp review <spec> --delta' -- spec/0.36.0.md` and the same command
> for `at most three` return the *same* commit list, and its earliest commit introduces both errors
> at once: `git show <c>~1:spec/0.36.0.md` contains neither string, `git show <c>:spec/0.36.0.md`
> contains both. Uncommitted drafts are a fact only the author holds; this file does not count them.

## §3 The branch — the panel is not the mechanism

**A delta pass can never move the loop toward convergence** — because it writes no state, not because
of anything about panels. Measured in an `rsync` copy: `tp review spec/0.37.0.md --perspective
regression` wrote its payload to stdout and touched zero files (`touch stamp`, run, then
`find . -type f -newer stamp | wc -l` → 0). The panel is **not** the mechanism, and an earlier form of
this paragraph said it was: `tp review s.md --record f.ndjson` on a findings file carrying **one**
role's row returned `{"round": 1}` and set `consecutive_clean`, so `--record` alone advances the
count. *"Counted rounds are always full-panel"* is policy prose in `skills/tp/SKILL.md`, not
something the mechanism enforces. That is already true of the shipped pass and this release does not
change it.

(The draft cited `skills/tp/SKILL.md:129` for that sentence; at the current tree it is the line
`grep -n 'Counted rounds are always full-panel' skills/tp/SKILL.md` prints. The `-o` refusal the
draft measured — `tp review spec/0.37.0.md --perspective regression -o /tmp/delta.json` exits **2**
with `{"error":"-o/--output requires --merge","code":2,…}`, writes nothing to stdout and creates no
file — still reproduces: `internal/cli/review.go:403` registers the flag as *"Output file path (for
--merge)"* and `review.go:311` is the refusal.)

## §6 Tests — what the absorbed draft's preamble measured

Every row derives from a numbered decision. Two universals over these seven rows are **not** asserted,
because an earlier form of this preamble asserted each and grounding falsified both. The table is seven
rows long and the author could have counted; that is the whole of the lesson.

**Not "every row names a mutant that must fail it".** Six name a mutant of the *product*. **Row 2 alone
names a mutant of the test**, and a weakened test cannot fail an assertion, so the sentence was false
for exactly one row. Row 2 is kept deliberately, and this preamble has twice described *why* wrongly.
The comparator table below shows the inclusive mutant `n >= 3` failing row 1 as written — row 1
asserts 4 fires and 3 does not, and `n >= 3` fires on 3 — so **rows 1 and 2 assert the same
proposition**, `{(3, no fire), (4, fire)}`, and both fall to that mutant. What separates them is not
the assertion but the *mutant column*: row 2's names an implementation of row 1 (with 1 and 10) that
passes whichever way the bound goes. So row 2 constrains **how row 1 is written**, and the table
asserts one thing twice.

**Not "every row names an artifact".** Counting a `path:line` citation anywhere in the row as naming
one, **exactly one does** — row 2's `internal/engine/lock_timeout_range_test.go:13-33`. The count is
what `grep -cE '^\| [0-9]+ \|.*\.go:[0-9]' spec/backlog/next-action-and-check-tell-the-truth.md` prints, against
`grep -cE '^\| [0-9]+ \|' spec/backlog/next-action-and-check-tell-the-truth.md` for the denominator, and the bare-`from` figure is
`grep -cE '^\| [0-9]+ \| §[0-9.]+ \|' spec/backlog/next-action-and-check-tell-the-truth.md`.

**The anchor is `[0-9]+` rather than `[1-7]`, and the difference was measured.** An earlier form used
`[1-7]`, which is a character class over the row *number*: it asserts a denominator of seven instead
of deriving one. Appending an eighth row carrying a second `.go:` citation leaves the numerator at
`1` where the truth is `2`, and the denominator at `7` where the truth is `8` — silently. Uncapping it
costs nothing and removes the assertion.

**And the paragraph that explained all this was itself wrong three ways, which is the lesson.** Its
first derivation was `sed -n 'A,Bp' … | grep -c`, written after an edit moved the table: it printed
`1`, the right answer, off a sentence in this paragraph rather than off the table. Its replacement
cited *"§5's matrix"* when the matrix is in §6, the section the sentence is already in. It said the
table had moved *"thirteen lines"* when `git show 16372d01:spec/1.49.0.md` and
`git show 196da662:spec/1.49.0.md` put row 1 at lines 121 and 139 — **eighteen**, and thirteen is
reachable only from an uncommitted draft this file's own rules say it does not count. **A paragraph
written to explain why line-pinning is unsafe re-committed the class one level up**, twice, and each
time a round caught it rather than a reader.

(The rows the preamble counts are now rows 5–11 of the spec's §7; the citations it counts moved with
them.)

## §6 Tests — the comparator matrix

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
