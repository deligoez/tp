# gate-sequence — measurements

Supplemental material for `gate-sequence.md`; the spec stands without it.

## Why the restatement goes: the five-round record

> Its central one is not the array — it is that **CI stops restating the
> gate**. v0.36.0's audit came back to that restatement in **five** of its thirteen rounds and closed
> none of it, and §1.1's five inputs are why. The five carries its rule, because a bare count here
> reproduced under none: rounds whose recorded rows name `TestCIRunsEveryStepOfTheProjectGate` — 1, 5,
> 6, 7, 8 of `spec/.tp-review/0.36.0/audit-round-*.ndjson`. Rows merely mentioning `ci.yml` return
> thirteen; repair commits on the two guard files return one contiguous block after round 8's record.
> But a repaired guard *is* available — §1.1 records two, built and run — so the reason for removing
> the restatement is not that guarding it is impossible. It is that a guarded restatement is still two
> definitions of the gate, now with a third artifact keeping them in step.

## What the shipped guard measures, and what it does not

Every row below was built and run against the guards as they ship, in an `rsync -a --exclude .git`
copy outside the repository. **The third row was re-run against `HEAD` with its control**, because an
inherited measurement is a claim about a tree that has since moved:

```
baseline (untouched ci.yml)                                          ok
run: |  echo "temporarily skipping ./scripts/check-suite-state.sh (flaky on CI)"
        go test ./...                                                ok      <- green
delete ONLY the echo, changing nothing CI runs                       FAIL    <- red
                                                 TestCIRunsEveryStepOfTheProjectGate
```

Same CI behaviour in both of the last two — a bare `go test ./...`, no race detector, no state check —
and the guard's verdict flips on the presence of a string inside an `echo`. Note what that line is
**not**: it carries no `#` and it is executed, so *restricting a substring search to executable lines
does not help* — the mention is itself executable. That repair was written during v0.36.0's audit, in
a commit making the guard "search executable lines, not the whole file", and reverted two commits
later. **The guard tracks text, not execution**, and the control is what proves it rather than merely
suggesting it. What does help is dropping the *substring* comparison as well as the whole-file scope
— assertion A below.

**Counting rule for "gate guard", because two counts of this set have already disagreed:** every test
function in `internal/cli/ci_gate_test.go` plus `internal/cli/quality_gate_config_test.go` —
`cat internal/cli/ci_gate_test.go internal/cli/quality_gate_config_test.go | grep -cE '^func Test'`.
The `cat` is not decoration: `grep -hcE` over the two paths prints a count **per file** — `1` and
`5`, two numbers a reader has to know to add — and a derivation that does not yield the figure it
states is the failure this paragraph exists to prevent. There are **six** at `HEAD` (`771b2ece`).
Counting the same way at every commit `git log -S'func Test'` returns for the two files puts the set
at **1 → 2 → 3 → 4 → 6** (`2120dcc2`, `2f2271fb`, `86f987d2`, `ed49776b`, `39d87f0c`), so a
five-guard state never existed. An earlier revision wrote that progression as 3 → 4 → 6, dropping its
first two states silently — in the paragraph fixing a counting rule.

| input | result |
|---|---|
| the deadcode step deleted, `# TODO: re-enable ./scripts/check-deadcode.sh` left behind | all six gate guards green, CI runs no deadcode |
| `run: ./scripts/check-suite-state.sh` replaced by `run: go test ./...`, plus a comment naming the wrapper **by its full `./scripts/` path** | all six green, CI runs neither the race detector nor the state check |
| `echo "temporarily skipping ./scripts/check-suite-state.sh (flaky on CI)"` above a bare `go test ./...` | all six green; deleting only the `echo` turns the guard red |
| `if: false` on the deadcode step | all six green |
| `continue-on-error: true` on the suite-state step | all six green |

**The second row's full path is load-bearing.** `ci.yml`'s comment names the wrapper as
`check-suite-state.sh` while the gate step is `./scripts/check-suite-state.sh`, and the substring does
not match: a reproduction that keeps the bare basename reddens the guard and looks like a refutation
of this table. Rows four and five are measured on one side only — that Actions skips an `if: false`
step, and that `continue-on-error: true` keeps a job green while a step fails, is documented Actions
behaviour rather than something observed here; what was observed is that all six guards stay green.

**A repaired guard exists. It was built and run, and that is why this section no longer claims
otherwise.** Two assertions, both in the same test file:

- **A — equality over parsed run-blocks.** Collect the argument of each inline `run:` plus each
  more-indented line under `run: |`, comments excluded; require each `&&`-split gate step to **equal**
  one of them, rather than to be a substring of the file. ~60 lines.
- **B — a two-item prefix blacklist over the whole file.** No non-comment line in `ci.yml` begins
  with `if:` or `continue-on-error:`. ~15 lines. It reddens rows four and five as the table below
  records, and it is *not* of a shape that closes the class — see the paragraph after the table.

| input | the shipped six | A | B |
|---|---|---|---|
| untouched `ci.yml` | green | green | green |
| deadcode step deleted, TODO comment left | green | **red** | green |
| wrapper → bare `go test`, comment names the full path | green | **red** | green |
| `echo` naming the wrapper above a bare `go test` | green | **red** | green |
| *control:* only the `echo` deleted, CI unchanged | **red** | red | green |
| `if: false` on the deadcode step | green | green | **red** |
| `continue-on-error: true` on the suite-state step | green | green | **red** |

**So the honest claim is narrower than "the guard cannot work".** What cannot work is an
unstructured `strings.Contains` over the file's whole text, because its subject is a text that any
mention satisfies. That is a defect in the assertion's **scope and its comparator**, not proof that
no assertion exists.

**And the rule is about scope, not polarity.** `CLAUDE.md` records this, measured twice: absence is
not the safe direction. A windowed `NotContains` is a presence assertion over a finite blacklist, and
both it and `Contains` are *local* assertions inside an *unbounded* text, so a negation simply moves
elsewhere. Two shapes survive — an assertion whose subject is the whole of a **bounded artifact**
(§4's replacement guard), and a verdict resting on **bounded read-backs** rather than on matched
text. A is the first shape, reached by narrowing the subject from the file's text to the set of
commands the file runs.

**B is not of either shape, and this file classified it as the first for one revision.** B is a
two-item prefix blacklist over lines — exactly the finite blacklist the paragraph above rejects — and
the negation was built by quoting the YAML key. A step carrying `"if": false` leaves **all eight**
assertions green (the shipped six, A and B); the same step written `if: false` reddens B, which is
the control that makes the pair mean something. The two files are the same step:
`ruby -ryaml -e 'p YAML.load_file("q.yml")["jobs"]["test"]["steps"][0].keys'` reports
`["name", "if", "run"]` with `if` false under either spelling, so the disabled step is invisible to
B. B's column in the table above still reproduces cell for cell; what does not stand is its
classification. The sound form of B's intent is the **second** shape — parse the file and read each
step's key set back — because a parser reports the key whichever way it is spelled.

**The release still removes the restatement, and only the reason changes.** Not that the restatement
cannot be guarded — A and B redden every one of §1.1's five recorded inputs — but that a guarded
restatement is still two definitions of the gate, now with a third artifact keeping them in step.
Deleting one of the two definitions is the only option on the table that removes work instead of
adding it.

## A load-sensitive gate test

`TestRoleWriteHookStaysCheapOnADeepMissingPath` asserts `deep < 3*shallow` on wall-clock
(`internal/cli/role_write_symlink_test.go`). It **failed one of four full gate runs** during v0.36.0's
implementation, at deep 56.95 ms against shallow 17.82 ms — a ratio of **3.196**, not the 3.198 first
recorded (`python3 -c 'print(56.95/17.82)'`).

**The two timings have a named artifact, which is why a constant may be built on them.** They are in
the `closed_reason` of task `instruction-test` in `spec/0.36.0.tasks.json`: the failing run, both
figures, the 3.198 corrected above, the diagnosis *"FLAKY under load"*, and the judgement that the
constant "sits too close to the noise floor" — which is the reasoning this section goes on to reject.
Nothing else carries either number (`git ls-files`, `spec/.tp-review/` excluded), and no commit
message on any branch mentions them, so that one closure is the whole provenance.

**That rate is unreproduced, and this section states it as unreproduced rather than as measured.**
**104** repetitions, zero failures, across three samples: 32 in this file's first ground round (24
solo, 4 under `-race`, 4 inside a full `go test -count=1 -race ./...` — the condition the original
sentence names) plus 24 after it, both in an `rsync` copy —
`go test ./internal/cli -run TestRoleWriteHookStaysCheapOnADeepMissingPath -count=20 -v` (ratios
1.208–1.417) and `go test ./internal/cli -race -count=4 -v` (1.258–1.433); 24 in ground round 2; and
24 in a `git clone --no-hardlinks` copy at `771b2ece`, `-count=24 -v`. Under p = 0.25,
P(0 failures in 104) ≈ 1e-13.

**The absolute timings do not carry between machines, and an earlier revision quoted them as if they
did.** It gave the shallow side as 15.1–18.9 ms and the deep as 20.6–23.9 ms; neither interval
reproduced in either later sample — round 2 measured shallow 16.3–33.8 and deep 25.0–39.8, the clone
at `771b2ece` shallow 18.8–28.5 and deep 22.0–52.9. What does carry is the **shape**. The deep side
alone reaches the original's magnitude: the clone's worst deep reading, 52.9 ms, is within 8% of
56.95. But it arrives beside a shallow reading that rose with it — 28.5 ms — so the ratio stays near
1.9. Both sides are measured back to back and share whatever load exists, which is what the ratio is
for. So the original run is a deep spike **unaccompanied** by a shallow one, and no repetition here
has produced that.

**What produced it is not established, and the mechanism this section first named is impossible.**
That sentence read: *"one of four full gate runs" describes the four-step gate, whose later steps
load the machine while the suite is still finishing.* The gate is one `&&`-joined string, and §2
measures above what `&&` means — `sh -c '(exit 3) && (exit 9)'` returns 3, and the second command
never runs — so `golangci-lint run` cannot start until the wrapper has exited. Within
one gate invocation no later step overlaps the suite. What the recorded closure says is only *"FLAKY
under load"*, without naming the source. The remaining candidate inside a single gate run is the
suite itself: `go test ./...` runs package binaries concurrently and this test is `t.Parallel()`
inside a package of them — which is precisely the condition 4 of the 104 repetitions exercised,
without a failure. This section therefore claims a **tail**, and names no cause for it.

**The design stays and the constant moves.** A ratio rather than a deadline, fastest-of-N each side,
is right. What was wrong is the reason first given for widening: the constant does **not** sit at the
noise floor. What was also wrong is the margin claimed for it. An earlier revision said the observed
ratio never exceeded 1.44, so 3 sat at more than twice the highest ratio seen — but the highest ratio
has risen with every fresh sample: 1.433, then **1.651** in ground round 2 (deep 34.83 ms, shallow
21.10 ms), then **1.857** in the clone at `771b2ece` (deep 52.87 ms, shallow 28.47 ms). Against 1.857,
3 is 1.6×, not 2×, and the pattern is the section's own point in miniature — a sample maximum is not
a bound, because the maximum is exactly the statistic a tail moves. The problem is a **tail**, not a
floor, and that decides how far to widen. The constant must clear the recorded 3.196 outlier and
still redden an unbounded walk, which the test's own comment records at ~6× shallow. **It moves to
4** — above the recorded outlier and 2.2× the highest ratio yet observed — and the sample grows from
three iterations per side to nine, because a fastest-of-N estimate is what a tail defeats and N is
the only knob that answers it.

**It is fixed here because a gate that randomly reddens is a gate people learn to re-run.** That
habit is what makes §1.1's third row — *temporarily skipping (flaky on CI)* — a thing someone writes.

## Notes on rows 5 and 8

**Row 8 cannot be implemented alone**, which is why it names its dependency. Built in isolation — the
counter moved inside the per-file loop, `goInstallRef` untouched — it returns the identical verdict
`release.yml contributed zero matches` on a fully floated *and* on the shipped, fully pinned
`release.yml`. The two halves of §5 are one change.

**Row 5 is the acceptance and its fixture is a recorded input, not an invented one.** §1.1's rows were
built and run against the shipping guards; using the one that turned all six green is what separates a
fix from a reword. Reproducing it needs the input stated exactly: the `echo` must spell the full
`./scripts/check-suite-state.sh`. The guard is a substring match, so a bare `check-suite-state.sh`
reddens it — and a naive reproduction then looks like a refutation of §1.1 rather than a
misconstruction of its fixture.
