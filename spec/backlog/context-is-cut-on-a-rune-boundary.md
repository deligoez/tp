# tp — Context is cut on a rune boundary

Class: **tool** — no command, no flag, no workflow field; one truncation decision in
`internal/engine/vague.go`. Its measurements are in
`context-is-cut-on-a-rune-boundary-measurements.md` beside it; this file stands without them. Every
citation below names a symbol or a quoted string and the file it lives in, never a line number.

## 1. Overview

The subject is the cap on a lint finding's `Context`: it is applied by byte, so a multi-byte rune
straddling the boundary is cut in half and reaches the reader as `U+FFFD` — text that is in no
document — while the two tests pinning the cap assert a byte length, which the corruption satisfies.
This spec was split out of `refusals-that-name-nothing.md` on 2026-09-08, where it was §7 of that
file, whose forwarding table maps every moved section to its new slug.

## 2. `Context` is truncated on a rune boundary

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

## 3. Non-Goals

1. **The `Context` cap is not raised, and is not made a character count.** §2 keeps 80 bytes and
   makes the cut honest; what the cap should be is a contract question this release does not open.

## 4. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *the cut* | on a duplicated line of 78 ASCII characters and an em dash, both `duplicate-line` and `duplicate-paragraph` emit a `Context` that is valid UTF-8 (`utf8.ValidString`) and at most 80 bytes | the shipped `ctx[:80]`, whose `Context` on that input is 80 bytes of which the last two are an orphaned rune prefix |
| 1b | §2 *the guard* | the two existing cap assertions in `lint_test.go` are joined by `utf8.ValidString` on the same fixture, and the fixture's 80th byte is asserted to fall inside a rune before the assertion runs | a fixture of 80 ASCII characters, under which byte and rune truncation agree and the guard proves nothing |
