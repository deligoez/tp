# tp — An unreadable file is named

Class: **tool** — no command, no flag, no workflow field; one channel decision in
`internal/cli/review.go`. Its measurements are in `an-unreadable-file-is-named-measurements.md`
beside it; this file stands without them. Every citation below names a symbol or a quoted string and
the file it lives in, never a line number.

## 1. Overview

The subject is the review phase's file ranking, which drops a file it cannot read in silence while
the next function in the same file states the opposite convention as a convention and writes the
dropped file's name to stderr. One condition, two siblings, opposite channels; the failing party
holds the path and the `error` and says neither. This spec was split out of
`refusals-that-name-nothing.md` on 2026-09-08, where it was §5 of that file, whose forwarding table
maps every moved section to its new slug.

## 2. An unreadable file is named, not dropped in silence

`rankFilesBySpecTerms` (`internal/cli/review.go`) scores each candidate file by how many spec
heading terms it contains, and on a read error drops the file with a bare `continue` — the source is
quoted in the sidecar under *Implementation notes from the original body*.

The **next function in the same file**, `readFilesContent`, hits the same condition and its doc comment
states the opposite convention as a convention:

> A file it cannot read is named on stderr rather than dropped in silence: the caller's paths came
> from a directory walk, so an unreadable one is an anomaly, and a role that never sees the body would
> otherwise judge the file from its absence.

**One condition, two siblings, opposite channels.** Measured, both on the same input — three files in
a temp dir, one `chmod 000`, spec lines carrying two headings — with the probe asserting the locked
file is genuinely unreadable before it concludes anything: the ranking drops one file and writes
nothing, the read drops the same file and names it on stderr. The per-function table is in the
sidecar under *§5 — the two siblings, measured*.

**The ranking drop is the worse of the two, because it is the silent one.** `docStructure.ReviewedFiles`
is `len(ranked)` in both `runReviewDocPlan` and `runReviewTestPlan`, so the emitted JSON says fewer
files were reviewed and says nothing about why. **What the drop removes is the body and the fact of
the drop — not the path**, and this paragraph first claimed the opposite. The emitted prompt falsifies
it: `walkDocTree` builds the `Documentation structure:` tree *upstream* of the ranking and hands it to
the prompt generator, so the dropped filename survives.

### 2.1 The decision

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
merging the two reads is a refactor, not this release (Non-Goal 2). They do **not** read the same
*set*, and this paragraph first said they did: ranking reads only the rankable files (`index.md` and
`config.*` are diverted into an always-include list it never reads), and `readFilesContent` receives
the ranked list, from which ranking's own drops are already absent. The two-reads hazard survives
either reading; the set claim does not.

**Keeping the file in the ranked list at score 0 was considered and not taken.** It would fix
`ReviewedFiles` as well as the channel, but it changes *what the prompt carries* — an unreadable file
would occupy one of the fifteen slots whenever fewer than fifteen files outscore it — and this release
is about what a refusal says, not about what a selection returns. The count stays wrong; the operator
now learns why.

## 3. Non-Goals

1. **The audit-side file selection is untouched.** `rankFilesBySpecTerms`'s two call sites are the
   review-phase `runReviewDocPlan` and `runReviewTestPlan` in `internal/cli/review.go`; the audit
   checklist is `selectCodeFiles` in `internal/engine/auditfiles.go`, reached through
   `SelectAuditFiles`, and the two share nothing but the words "file selection". `selectCodeFiles`,
   `CodeFileCap`, `SelectAuditFiles` and `internal/engine/auditfiles.go` are not edited here.
2. **The two file reads are not merged.** `rankFilesBySpecTerms` and `readFilesContent` each read the
   candidate files; collapsing them into one pass is a refactor with its own cap and ordering
   decisions, and CLAUDE.md's rule is that a repair introducing a new abstraction belongs to the next
   version.
3. **`ReviewedFiles` is not corrected.** §2 makes the drop audible; the count still reports the
   post-drop list. Correcting it means deciding whether an unreadable file was "reviewed", which is a
   contract question about the emitted JSON and not a channel question.

## 4. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | on a file set containing one unreadable file, `len(in) - len(out)` equals the number of notices written to stderr | the shipped bare `continue` — **measured: `in=3 out=2 stderr=""`, so 1 ≠ 0; after the fix, `drops=1 notices=1`** |
| 1b | §2 *no double report* | exactly one notice per unreadable file across the ranking and the read that follows it | notice at both sites, which reports twice for one anomaly and makes the count in row 1 wrong in the other direction |
| 1c | §2 *the fixture is valid* | the probe asserts the locked file is unreadable **before** it concludes anything from an empty stderr | run the probe as a user that can read a `chmod 000` file, under which the silent-drop finding cannot occur and the test passes for the wrong reason |
| 1d | §2 *the branch is entered* | the fixture's spec lines carry at least one `## `/`### ` heading, asserted before the drop is counted, so `rankFilesBySpecTerms` does not take its zero-terms early return | a fixture whose spec content is `buildSpecRefContent`'s bullet headings, under which zero terms are found, every candidate is returned **unread**, the drop site is never entered, and the test passes with no drop and no notice |

**Row 1c is not decoration.** The `chmod 000` fixture decides the result of this spec's entire
measurement, and under a user that can read the file the probe returns three files, empty stderr, and
a green test that has established nothing. The property the verdict rests on is asserted, not assumed.
`an-invalid-task-file-is-reported.md`'s row 1b and `context-is-cut-on-a-rune-boundary.md`'s row 1b
carry the same rule for their own fixtures.
