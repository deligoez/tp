# refusals-that-name-nothing — measurements

Supplemental material for `refusals-that-name-nothing.md`; the spec stands without it. Every block
below was moved here verbatim from the spec on 2026-09-08, except that two filename citations to
renumbered specs were repointed at slug paths. Every mutant was built and run in an
`rsync -a --exclude .git ./ <copy>/` copy outside the repository.

## Preamble

> **This file is decisions.** What v1.0.0's audit measured and did not repair — the split
> `spec/candidates.md` files them under, and §1 keeps it. Each of v1.0.0's defects was re-run
> against `HEAD` while writing this file rather than
> carried forward from the handover text, because a deferred finding is a claim about a tree that has
> since moved. **§6 is the exception and is not a re-derivation**: `spec/candidates.md` already
> carries that routing correction, in the same words and with the same search, and already re-routes
> the finding to this release. §6 says so.
>
> Every mutant below was built and run in `rsync -a --exclude .git ./ <copy>/`, outside the
> repository. **The check that the repository is unchanged has to have the repository as its
> subject**, and the one this preamble first carried did not:
> `diff -r --brief <copy>/internal ./internal` compares the copy against the tree, so it goes red
> whenever the *copy* still holds a mutant — and it did, on `internal/cli/review.go`, when it was
> re-run. What is asserted instead is `git status --porcelain -- internal/` printing nothing, beside
> a control on a path that *is* edited, which prints ` M <path>`: the empty output is then a reading
> rather than a silence.
>
> **No line number is cited anywhere below.** Round 1 found three line-anchored citations in this
> document pointing at the wrong text — one off by eleven lines, one at a doc comment rather than the
> assertion it names, one naming the wrong test entirely — so every citation here names a symbol or a
> quoted string and the file it lives in.

## §2 — the cost of the round trip

**The recovery costs a re-read of the emitted prompt, which is the argument v1.0.0 already accepted
for the other four cells.** The prompt does carry the per-kind sets — `groundPromptEvidence` in
`internal/cli/ground.go` renders them from `TierAcceptableFor` — so the information is reachable and
the cost is the round trip. **How many bytes that round trip is, this file quotes nowhere**, because
the figure is a property of the spec being grounded and not of tp. Derive it in the copy, since
`tp ground` writes a round snapshot beside the spec:
`tp ground spec/1.0.0.md --json | python3 -c 'import sys,json; print(len(json.load(sys.stdin)["prompt"].encode()))'`
— and note the path, because `tp ground 1.0.0.md`, the form this paragraph first carried, exits 3 with
`spec not found` and prints no number at all. The value moved at each of the six most recent commits
touching `spec/1.0.0.md` while `git log <that range> -- internal/` is empty, so the renderer never
changed and the figure tracks the spec's own growth. `internal/engine/groundrow_test.go`'s
enum-refusal doc comment states a **third** figure for the same quantity, measured at a third moment;
§4 edits that comment, and the stale figure is deleted with it rather than re-measured.

## §3 — measured, both directions

Probing the copy for `len(GroundVerdicts())`, `len(GroundKinds())`, `len(GroundTiers())` and
`len(GroundPartialKinds())` gives `6 7 6 3`, so no cell gets four. The next table is the same fact
from the other side — `GroundTiers()[:2]` costs the `tier` cell four of its six values, hence four
failures.

**Measured, both directions, on the shipped guard:**

| mutant at the production call site | `internal/engine` | `internal/cli` |
|---|---|---|
| `GroundTiers()[:2]` — truncate the listing | **red**, four failures naming `run`, `probe`, `red-green`, `break-and-control` | — |
| the same truncation on **all four** call sites at once | **red**, and *counted*: each of the four subtests fails on its own | — |
| `append(GroundTiers(), "document", "corpus", "vibes")` | **green** | **green** |
| the same append on **all four** call sites at once | **green** | **green** |

The mutated refusal reads, in full:

```
field "tier": "squinted" is not one of the values the spec lists: read, query, run, probe, red-green, break-and-control, document, corpus, vibes
```

A refusal telling a unit that `document` is a legal tier — while `document` is a *kind*, and the row
that pairs it with a tier is exactly what §2 is about — and the guard written to make this message
informative passes it.

The obvious repair does not work, and it was measured before being rejected:

| assertion | shipped message | listing appended | listing prepended |
|---|---|---|---|
| `Contains(msg, ": " + strings.Join(names, ", "))` | pass | **pass** | fail |
| `HasSuffix(msg, ": " + strings.Join(names, ", "))` | pass | **fail** | fail |

## §4 — §7.2's `meaning` column

Reading the four `meaning` cells of §7.2's field table in `spec/1.0.0.md` — the rows whose first
cell is `verdict`, `kind`, `tier` and `partial_kind`, named rather than cited by line because the
range this paragraph first carried was eleven lines off and pointed at §7.2's heading instead of its
table:

| cell | what §7.2's `meaning` column says | enum values present there |
|---|---|---|
| `verdict` | *one of §3's six* | none |
| `kind` | *one of §4.1's seven*, plus `` `NOT-A-CLAIM` ``, `` `tier` ``, `` `evidence` ``, `` `note` `` | **none** — the backticked tokens are one *verdict* value and three field names |
| `tier` | *one of §4.1's tier table* | none |
| `partial_kind` | `two-readings`, `reason-not-conclusion`, `true-when-written` | all three |

## §5.1 — how far the drop reaches through the shipped command

**The function-level table above does not say how far it reaches, and the answer bounds the finding.**
`rankFilesBySpecTerms` returns every candidate **unread** when the spec content carries no `## `/`### `
heading, so the drop site is never entered; and
`resolveReviewSpecContent`'s default arm returns `buildSpecRefContent`, whose headings are rendered
as bullets. An ordinary `tp review` therefore finds zero terms and cannot drop. Measured on one
fixture — three `docs/*.md`, one `chmod 000`, asserted unreadable first — with
`tp review spec.md --perspective documentation --docs-path docs --no-state`:

| invocation | `reviewed_files` | stderr |
|---|---|---|
| as written (spec-ref content) | 3 | `readFilesContent`'s warning |
| `--spec-inline` | **2** | empty |
| `--diff-from base.md` | **2** | empty |

The control is counted, not assumed: the matrix is 3 × 2 — all three invocations were run again with
`locked.md` readable, and every one reports 3 with empty stderr. **So the silent drop is reachable
through `tp review` only under `--spec-inline` or `--diff-from`**, whose rendered sections carry real
`## `/`### ` headings. That narrows the blast radius and does not withdraw the finding: those are the
two modes in which a role is handed the spec's own text, and they are the runs whose count is wrong.

**The fix is the sibling's channel, at the drop site.** Measured in the copy: with an
`output.Notice` at the `continue`, the same input gives `in=3 ranked=2 kept=2 drops=1 notices=1` —
exactly one notice, because a file dropped from the ranking never reaches `readFilesContent`, so
there is no double report. The full `internal/cli` suite stays green.

## §6 — a routing correction, and who actually made it (deleted from the spec)

This section was deleted from the spec on 2026-09-08 because its subject, `spec/candidates.md`, is
now a forwarding stub. It is kept here as the record of the routing argument.

`spec/candidates.md` routes §5 to **the release that reworks the audit checklist**, *"which already
owns the file-selection channel"*. **That routing is wrong, the check is one search — and
`candidates.md` has already run it.** The correction is in the same file, in the same words and with
the same search, in its table of judgements that did not survive checking; the same file's
where-each-finding-lives table already re-routes this one here. So what follows is a restatement with
its derivation attached, not a discovery — and it is not one of the three findings §0 says were
re-run against `HEAD`. It is worth restating because the *wrong* routing is still in the file too, in
the table of defects — sitting between the correction above it and the re-route below it, so a reader
who stops at the defect table gets the wrong answer with nothing to warn them.

`rankFilesBySpecTerms` has exactly two call sites, both in `internal/cli/review.go`:
`runReviewDocPlan`, which walks `.md` files, and `runReviewTestPlan`, which walks `_test.go` files.
Both are **review**-phase perspectives, dispatched from `runReview`'s `perspective` switch. They
select docs and tests.

The audit checklist is a different function in a different package: `selectCodeFiles`
(`internal/engine/auditfiles.go`), bounded by `CodeFileCap`, reached through `SelectAuditFiles`,
whose only consumer outside its own tests is `internal/cli/audit.go`. A search for
`SelectAuditFiles|AuditFileInputs` across `internal/` returns three files —
`cli/audit.go`, `engine/auditfiles.go`, `engine/auditfiles_test.go` — and `internal/cli/review.go` is
not among them.

**Zero overlap.** The two channels share the word "file selection" and nothing else, and a release
scoped to the audit checklist would not touch the function this finding is about. The finding belongs
here, with the other three, because what is wrong with it is what is wrong with them: the code holds
the answer and does not say it.

**Why this is worth a section rather than a footnote — and the reason it first gave was falsified by
its own subject.** `candidates.md` is where a finding waits, and a routing that names the wrong
release is one way a finding waits forever: the release it was routed to ships without it. The
mechanism this paragraph first named — *"nobody re-derives the routing because the file already says
where it goes"* — is exactly what did **not** happen here. Somebody did re-derive it, in
`candidates.md` itself, before this spec was written. What survives is narrower and still worth a
section: one file states this routing three times, wrong once and corrected twice, and nothing in the
wrong statement points at either correction. That is the argument for naming a release by its subject
in one place only — the roadmap — and §6 is what the rule costs where it was not followed. The same
file's own record carries the general form: a deferred finding *"read as resolved for exactly the
round it was absent"*.

## §6 of the spec (formerly §9.1) — `checkTaskFileQuality`, measured

Introduced at `8c2555a7`, `2026-04-02`, which `git merge-base --is-ancestor 8c2555a7 v1.0.0` confirms
is an ancestor of `v1.0.0` — so it predates every release that could have been expected to catch it.
First measured against `6d9be576`; re-run on 2026-09-08 against the dev binary at `HEAD` with the same
result.

Measured with a control and a precondition, on one spec with one task whose acceptance is under ten
words, so the check has something to report when it runs:

| task file | `acceptance-quality` findings | stderr | exit |
|---|---|---|---|
| valid (**control**) | 1 | 0 bytes | 0 |
| unparseable (`{ this is not json`) | **0** | 0 bytes | 0 |
| unreadable (`chmod 000`) | **0** | 0 bytes | 0 |

The control is what makes the two zeros readable: without it, zero findings is equally consistent
with a task file that has nothing wrong. The unreadable arm asserts the file is genuinely unreadable
before concluding anything, for the reason the spec's row 8c gives.

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

## From the guard spec

The guard-helper spec (formerly `13a-guards-read-what-production-reads.md`, absorbed into the refusals
spec as tasks on 2026-09-08 and then deleted) carried the measurements below. Its §2 table and its §8 meta-paragraphs are
copied verbatim; the line numbers in them were measured against `HEAD` at the time and are not
re-derived here.

### The guard spec's §2 — what is true today, measured

Every row is an edit to `spec/1.0.0.md` in a copy, then `go test -count=1`. The recipe is one line:

```
rsync -a --exclude .git . /tmp/probe && cd /tmp/probe   # then edit spec/1.0.0.md, then:
go test -count=1 ./internal/engine ./internal/cli
```

**Every row starts from a pristine `spec/1.0.0.md`.** The recipe above does not say so and the rows
require it: keep a copy of the file beside the probe and restore it between rows, or the mutants
stack and no row after the first measures what its cell claims.

"13th verb" below means `probed` appended to §2.1's real arms row — the defect a reader is trying to
catch. A **silent** row is the dangerous one: a real defect present, suite green.

| # | the input | packages run | result at `HEAD` |
|---|---|---|---|
| 1 | nothing changed | engine + cli | green |
| 2 | 13th verb alone | engine | **red** — the guard works when nothing is hiding |
| 3 | an unfenced decoy `\| **verb** \|` row **above** §2.1, plus the 13th verb | engine | red — the file-wide-first-match door is **closed** |
| 4 | an unfenced decoy row **inside** §2.1, plus the 13th verb | engine | red at `floor_test.go:1298` |
| 5 | a **fenced** decoy row inside §2.1, plus the 13th verb | engine | red at `floor_test.go:1298` — **closed, and the handover said open** |
| 6 | a **fenced** decoy row inside §2.1, spec otherwise **correct** | engine | **red — a false failure**: §2.1 step 1 rules a fenced block non-content, and quoting the arms table reddens the suite |
| 7 | a fenced quotation of the line `### 2.1 The floor` **above** §2.1 carrying a 12-verb decoy row, plus the 13th verb | engine + cli | **green — silent** |
| 8 | a fenced block inside §2.1 carrying a 12-verb decoy row and then a `### ` line, plus the 13th verb | engine + cli | **green — silent** |
| 9 | a fenced `### ` line inside §2.1, spec otherwise **correct** | engine | **red — a false failure**, and the message says *"a second one, fenced or not"* when there are **zero** |
| 10 | a 13th verb spelled `` `Re-ran` `` | engine + cli | **green — silent** |
| 11 | 13th and 14th verbs spelled `` `re‑ran` `` (U+2011) and `` `ran2` `` | engine + cli | **green — silent** |
| 12 | a fenced `### 7.3 …` line inside §7.2 above its table | engine | red in **three** tests, at **two** locations — `TestTheAllowedKeySetIsExactlySection72sTable` and `TestEveryFieldSection72NamesHasARejectionCase` at `groundrow_test.go:624`, and `TestThePerVerdictTableIsTheOneSection72States` at `groundtierrule_test.go:161`, which is the third helper in another file — a false failure |
| 13 | a fenced decoy field table inside §7.2 above the real one | engine | red in both §7.2 field callers — a false failure |
| 14 | a real 14th field `` `bogus` `` in §7.2's table | engine | red — the field guard works when nothing is hiding |
| 15 | a real 14th field `` `Bogus_field` `` in §7.2's table | engine + cli | **green — silent** |

**Two of these contradict what this release was handed, and both corrections matter to the design.**

Row 5: a fenced in-window decoy is **not** an open door. `require.Len(rows, 1)` counts the fenced row
too, so the helper refuses rather than choosing. What the same fence-blindness *does* buy is row 6 —
the guard reddens on a **correct** document. The hole is in the other direction from the one recorded.

Row 15: `groundSection72Fields`' first-cell class is `` `([a-z][a-z_]*)` ``, and a field it cannot
spell is dropped from the list while the count of thirteen still matches. That is exactly its twin's
character-class hole, in a helper the handover described as failing loud because *"both callers pin
hard — a count of thirteen, set equality with the code's key set"*. They do pin hard, and it does not
help: the pin is on a list the parser silently shortened. **The two helpers are the same defect, not
one strong and one weak**, and a release that fixed only §2.1's cell would leave §7.2's standing.

### The guard spec's §8 — what its test table learned about itself

Every row derives from a numbered decision. **Nine of the twelve name a mutant that their own stated
fixture kills, both directions measured.** Row 9's is marked **not built** — reasoned, not run, and
says so. Rows 10 and 12 name none, and the fourth cell says why: row 10's fixture is red under `HEAD`
and under the seam alike, so no mutant separates anything, and row 12 is the control. The counting
rule is that a row *names a mutant* when its fourth cell describes a change to the guard, not when it
explains why no change is needed.

**Rows 5 and 6 claimed to name one and did not.** Grounding round 1 built both and neither mutant dies
on the fixture its row names. Row 5's named mutant was `HEAD`'s `slices.Index` locate; on the
absent-§2.1 input that mutant *fails*, at `floor_test.go:1289`, with `"-1" is not greater than or
equal to "0"` under *"§2.1 must be findable by its heading"* — it names the anchor, so it **satisfies**
row 5's assertion. Row 6's named fixture, §2's row 4, is red with the mutation and red without it.
Both rows now carry a mutant their fixture kills, with the run in the cell; §3 item 3's own
continuation had named row 5's correctly all along, and the two now agree.

One line on where that came from, because it is this release's own subject looking back at it. The
false claim lived in this section's opening sentence, and a sentence introducing a table is exactly
what §2.1's arms cut — so grounding filed it against a **cut** unit rather than a floor one.
**A release about guards that re-parse a document lost its own strongest claim to the part of the
document its splitter drops.**

## Routed here at the 2026-09-08 re-verification

Four items from the candidates files land on this spec's subject — a refusal or an omission that
names nothing the caller can act on — and v1.1.0's audit round 2 added two more. They are recorded in
this sidecar; the spec body is not edited.

- **A mistyped override key is minted and then dropped.** `internal/engine/frontmatter.go` puts a
  mistyped `tp:` frontmatter key into `fm.Warnings`, and `internal/cli/lint.go` is the only consumer
  — three reads, against zero in `review.go` and `audit.go`. So `tp lint` reports the typo and
  `tp review` / `tp audit` run the whole round under the default the typo silently left in place.
  Source: `spec/0.33.0-candidates.md` item 4.
- **No guard covers the invalid-check sink.** v0.33.0 test 34 says a registered check that cannot run
  must surface in `mechanize_candidates`; at `HEAD` no test registers an invalid check and asserts
  that it does. The citation half of the item shipped — `review_suppression_test.go` now names
  v0.33.0 — and the sink half did not. Source: `spec/0.35.0-candidates.md` item 6.
- **The hint guard's two blind spots.** `internal/cli/hint_coverage_test.go` exempts four files by
  name — `config.go`, `config_extract.go`, `set_local.go`, `set_project.go` — with the reason
  recorded in the comment above `taskFileCommands`, and it says nothing at all about bare
  `os.Exit(ExitValidation)` sites, of which there are **51** across `internal/` at `dd89c566`
  (`faster_search 'os.Exit(ExitValidation)'`, counting call sites). The exemption is honest; the
  second gap is unnamed. Source: `spec/0.35.0-candidates.md` item 8.
- **`validate --project` under-reports twice.** `skipped` is omitted from the payload when empty, so a
  consumer cannot distinguish "nothing skipped" from "this build does not report skips", and
  `--strict` promotes deviations only, leaving the other advisory classes at their default severity.
  Source: `spec/0.35.0-candidates.md` item 10.
- **One over-long line blinds the heading parse, and the verify prompt reports the blindness as a
  result.** `engine.ParseHeadings` (`internal/engine/lint.go:28`) hands a default `bufio.NewScanner`
  to `ParseHeadingsFromScanner` (`internal/engine/lint.go:32`), so a line past the scanner's 64KB
  default token cap ends the parse with `bufio.Scanner: token too long`. Two call sites discard that
  error — `internal/cli/review_verify.go:111` and `internal/cli/review.go:1921`, both
  `headings, _ := engine.ParseHeadings(specPath)` feeding `buildSpecRefContent`. Measured at
  `c75e5c3d` on a 15-line spec carrying three `##` headings and one 70,000-character line:
  `tp review <spec>` and `tp lint <spec>` both refuse at exit **3** with `bufio.Scanner: token too
  long` (under a hint about the spec path, which names the wrong cause), while
  `tp review <spec> --verify --findings f.ndjson` exits **0** and emits
  `Spec file: /…/big-probe.md (16 lines, 0 sections)` above an empty `Focus your review on:` list. The
  reviewer is told the spec has no sections and is given no reason to doubt it. A silent wrong answer
  older than v1.1.0. Source: v1.1.0 audit round 2.
- **`--report` reads a spec as a findings file where `--merge` and `--resolve` refuse one.**
  `isSpecLookingPath` (`internal/cli/mode_positionals.go:19`) rejects a `.md`/`.markdown` positional
  at entry, and §4.1 states that fence for `--merge` and `--resolve`/`--resolve-all` only, so
  `--report` is outside it by design rather than by oversight. Measured at `c75e5c3d`:
  `tp review probe-spec.md --report` exits **0** and prints a report whose single round is
  `{file: probe-spec.md, in_file: 0, new: 0, resolved: 0, unresolved: 0}`, while
  `tp review --merge probe-spec.md` exits **2** naming the spec. The number a caller reads from the
  first is a count of nothing, and no channel says so. Source: v1.1.0 audit round 2.
