# next-action-and-check-tell-the-truth — measurements

Supplemental material for `next-action-and-check-tell-the-truth.md`; the spec stands without it.

## Cut on 2026-09-11: the delta-pass branch

The spec was rewritten on 2026-09-11. Its former §4 — an advisory branch in review's `next_action`
recommending `tp review <spec> --perspective regression` when a repair touched more than three
sections, with none added or removed, suppressed under `TP_RUN_ID` — was cut with its seven test rows
(the former rows 5–11). Three reasons, from the backlog survey of that date: nothing measured what the
missing recommendation costs; it needed a three-condition branch plus a carve-out for `tp run`, which
has no unit kind for the pass; and `skills/tp/SKILL.md`'s review loop, step 6, already states the rule
it would have recommended. The cut text is `git show 18032abe:spec/backlog/next-action-and-check-tell-the-truth.md`.

The sections *One draft, not two*, *§3 The branch — the panel is not the mechanism* and both
*§6 Tests* sections below belong to that branch and stay as its history; the rows they count are no
longer in the spec. The former §5 and its row survive as the spec's §5 and row 14.

Promoted at the same rewrite: the exit contract decided at the 2026-09-08 pass (below) is now the
spec's §4, the `{spec}`/`{round}` substitution decided beside it is §4.1 — the fix for the routed
class `check-grades-wrong-spec` (`.tp/routed-classes.json`) and for the wrong-spec item
`spec/1.1.0-release-notes.md` routes here — and WB-3155's #10 is §3's `ungraded` count.

## Re-verified 2026-09-11

Measured against a binary built from `18032abe`, each fixture in a fresh directory outside the
repository.

- **§2, row 1.** A two-section spec of three sentences, each naming a number, in a fresh git repo;
  `tp init`, `tp ground spec.md`, then `--record` of a payload deciding the floor's three units as two
  `FAIL` and one `PASS`. `tp ground spec.md --status` reports `by_verdict` `{"FAIL": 2, "PASS": 1, …}`
  with `emitted: 3`, `dispositioned: 3`, and no `next_action` key; `--status --check` exits **0**.
- **§2, row 2.** The same fixture with one unit decided `PARTIAL` (`partial_kind: true-when-written`,
  `held_at` set) and the rest `PASS` exits **0** from `--status --check`; with one unit decided
  `QUESTION` (three `{cause, prediction}` objects) it exits **0** as well.
- **§3, the missing count.** On row 1's fixture, after `--record`, editing one `FAIL` sentence's number
  leaves `--status` byte-identical to its pre-edit output and `--status --check` at exit 0. The next
  emission then reports `floor_size: 3`, `carried: 2` — the count exists, but only once emitted.
- **§4, rows 7 and 9.** A one-section spec with `tp set --workflow checks=` registering three entries
  across two fixtures — `exit 1`, `exit 2`, and a command the shell cannot find. The emission's
  `mechanical_checks` reports `exit_code` 1, 2 and 127, every one `passed: false`, and the suppression
  line *"Mechanically checked classes — do NOT report findings of these classes:"* appears in all
  three role prompts naming every registered class. `tp review spec.md --status --check` exits 1.
  Source: the prefix is `mechanizedExclusionPrefix` in `internal/cli/review.go`; the exit code and
  `Passed` come from `engine.RunCommand` in `internal/engine/runcmd.go`, which sets `Passed` only on a
  nil error and reports a timeout as a failure.
- **§4.1, row 10.** One repository, two specs `a.md` and `b.md`, both `tp init`-ed, a marker string in
  `b.md` only, and `tp use b.tasks.json`. Two project-level checks: one reaching its spec the way this
  repository's own registrations do — `grep -q <marker> "$(tp resume … | python3 … ["spec"])"` — and
  one written `grep -q <marker> {spec}`. `tp review a.md --status --check` reports the first
  `exit_code: 0`, `passed: true` — a pass about `b.md` while `a.md` lacks the marker — and the second
  `exit_code: 2` with `grep: {spec}: No such file or directory`: the token reaches the shell as
  written. The emission of `a.md` names both classes in its suppression line. With the pointer left
  naming a spec path instead of a task file, `tp resume` exits 3 and the first check greps an empty
  path, exit 2 — the silent failure the 2026-09-08 decision names.

## Field report WB-3155, verified 2026-09-11

**#10 — "a repair's own grading takes three rounds, and the tool does not say so."** The reporter
counted `FAIL`s of 5, 2 and 1 across three ground rounds before a clean one, and asked that `--record`
say how many units the next round will ask fresh. **Verdict: PARTLY.** The round counts are the
reporter's and were not re-derived. The reporter already saw `carried` at emission, and the number
they want is the floor less that; what reproduces is that nothing reports it *before* the emission,
when the operator decides whether the repair is done, and that `--check` cannot tell them either —
at `HEAD` a `FAIL`, repaired or not, never moves its exit code (reproductions above, *§2, row 1* and
*§3, the missing count*). The proposed channel cannot work as stated: at `--record` the repair has
not been written yet, so a count taken there describes none of the text the repair will write. The
spec takes the count at `--status` (§3) and relies on §2 for the gate. Source: the ground status payload is the struct in
`internal/cli/ground.go` carrying `emitted`, `dispositioned`, `reader_added`, `off_floor` and `cut`.

The blocks below moved verbatim from the delta-pass draft this spec absorbed. The grounding
measurement behind the `--check` gate — the recorded round with `FAIL`s standing that exits 0, and
the two-unit fixture reproducing it — is in `what-the-record-does-not-say-measurements.md` under
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

## Decided at the 2026-09-08 decision pass

From `spec/undecided.md`, *A registered check that outlives its release*. It lands on this spec
because its subject is checks telling the truth.

**Decided: `checks[].cmd` gains `{spec}` and `{round}` substitution and nothing else.** No other
placeholder is introduced. The substitution replaces the subshell workaround that reaches the spec
path through `tp resume`, which fails silently when `tp resume` cannot answer.

**Decided: a check's exit code is a contract tp defines for its own registrations** — `0` passed, `1`
found violations, `2` or higher cannot run. **A check that cannot run neither passes nor suppresses
its class**: it is reported as `ran: false`. Defining the status this way is legitimate here where it
was not for `gocognit` — `CLAUDE.md` records that gocognit's exit code cannot discriminate a result
from a failure, so its guard reads stderr instead — because `checks[].cmd` entries are tp's own
registrations rather than a third-party tool's convention tp merely observes.

The measured behaviour this ends is in `spec/undecided.md` under that entry: at `c75e5c3d`, a
schema-valid check registered as `exit 2` is reported `"passed": false` and still stamps
`do NOT report findings of these classes:` into all four role prompts of the same emission.

## Routed here from v1.1.0's audit round 3 (2026-09-08)

This sidecar carried no routed section before — it had *Decided at the 2026-09-08 decision pass*
above, but was not part of the sweep that wrote `## Routed here at the 2026-09-08 re-verification`
into other sidecars, so this heading names where the items came from instead of claiming membership
in that sweep. The items are recorded here; the spec body is not edited.

- **A registered check resolves its subject through the active pointer, not the spec on the command
  line — the motivating instance for the `{spec}` substitution decided above.**
  `internal/cli/review_status.go:199` is
  `res := engine.RunCommand(c.Cmd, dir, timeout, gateOutputTailLines)`: the spec path is not an
  argument, and there is no environment seam through which the command could learn it. So a check
  runs against whatever `tp resume` currently points at. Measured in a clone: the script run directly
  on `spec/1.0.1.md` exits **1** naming a real violation, while
  `tp review spec/1.0.1.md --status --check` reports `passed: true` — a PASS about one spec, reported
  under another. The second half is what makes it more than a wrong number: tp stamps
  `do NOT report findings of these classes:` into the role prompts of the spec named on the command
  line, so the class is suppressed for the spec the check never read. Root predates v1.1.0.
  (Citation read at `e8477464`; the two exit codes are quoted from the audit round's measurement.)
- **`scripts/check-test-rows-cite-sidecar.py:87` collides a crash with a result.** The line is
  `lines = spec.read_text(encoding="utf-8").splitlines()`, unguarded, inside `check_spec`. A
  non-UTF-8 or unreadable spec raises there, nothing catches it, and Python exits **1** with a
  traceback — the same code the docstring reserves for two *findings* meanings (lines 34–35: "1 when
  a cell cites it, and 1 when nothing was scanned at all"), while `main()`'s own I/O failures exit
  **2** (lines 148 and 156). The script therefore already keeps the contract on the paths its author
  wrote and breaks it on the path nobody wrote. An instance of the exit contract decided above: `2`
  or more means cannot run.
- **The zero-row guard is global, so a degraded table is caught only when it is alone.**
  `check_spec` returns a per-spec row count, `main()` sums it across every argument
  (`rows += n`, line 159), and the guard is `if rows == 0` (line 169). So a Tests table that has
  emptied itself — the plural header, the suffixed heading, or the unclosed fence its own message
  names — exits 1 when it is the only table, and **0** the moment any sibling argument contributes a
  single scanned row. Its citing cells then reach nobody while tp goes on telling every reviewer that
  `method-only-in-ungraded-sidecar` is mechanically checked. Read at `e8477464`.
