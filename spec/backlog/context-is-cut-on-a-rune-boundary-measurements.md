# context-is-cut-on-a-rune-boundary — measurements

Supplemental material for `context-is-cut-on-a-rune-boundary.md`; the spec stands without it. The
block below was moved here on 2026-09-08 from `refusals-that-name-nothing-measurements.md`, which had
in turn taken it verbatim from the spec body earlier the same day. The conventions every mutant below
was run under — the `rsync -a --exclude .git ./ <copy>/` copy outside the repository, the
`git status --porcelain -- internal/` check with its control, and the rule that no line number is
cited in the spec body — are in that file's **Preamble** and are unchanged here.

## §7 of the spec (formerly §9.2) — `Context` truncated by bytes, measured

Built and run: a line of 78 ASCII characters followed by an em dash, duplicated so `duplicate-line`
fires. Byte 79 begins `\xe2\x80\x94`; the cut lands after `\xe2\x80`. `tp lint --json` emits a
`context` **ending in `U+FFFD`** — a character in no document, which is exactly the class round 1 of
that release repaired one function up, where `blankInlineCode`'s working copy was reaching
`Finding.Context`. Re-run on 2026-09-08 at `HEAD`: the emitted `context` is 84 bytes and ends in two
`U+FFFD`.

**The two tests that pin the cap cannot see it.** `internal/engine/lint_test.go` asserts
`assert.LessOrEqual(t, len(f.Context), 80)` for `duplicate-line` and again for
`duplicate-paragraph`. `len` on a Go string is bytes, so the corrupt value is exactly 80 and both
assertions pass. Note the emitted JSON is **84 bytes / 80 characters** — the encoder renders each of
the two orphaned bytes as `U+FFFD` — so the shipped output does not even honour the cap the guard
believes it is checking.

**Scope, so the finding is not oversold: it fires on no document this repository has.** Swept all
67 files under `spec/` and `spec/backlog/` at `6d9be576` for a `duplicate-line` or
`duplicate-paragraph` context containing `U+FFFD`: **0**. The defect needs a duplicated line whose
80th byte falls inside a rune, which is why it survived. What makes it worth a row is the guard, not
the frequency: a byte-length assertion over a character-length claim passes identically whether the
truncation is correct or not, so nothing in the suite would notice the day a spec produced one.
