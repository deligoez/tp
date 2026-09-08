# an-unreadable-file-is-named — measurements

Supplemental material for `an-unreadable-file-is-named.md`; the spec stands without it. Every block
below was moved here on 2026-09-08 from `refusals-that-name-nothing.md` and its sidecar, which had in
turn taken most of them verbatim from the spec body earlier the same day. The conventions every
mutant below was run under — the `rsync -a --exclude .git ./ <copy>/` copy outside the repository,
the `git status --porcelain -- internal/` check with its control, and the rule that no line number is
cited in the spec body — are in `refusals-that-name-nothing-measurements.md`'s **Preamble** and are
unchanged here.

## Implementation notes from the original body

The drop site, quoted from `rankFilesBySpecTerms` in `internal/cli/review.go`:

```go
content, err := os.ReadFile(f)
if err != nil {
    continue
}
```

## §5 — the two siblings, measured

Measured, both on the same input — three files in a temp dir, one `chmod 000`, spec lines carrying
two headings — with the probe asserting the locked file is genuinely unreadable before it concludes
anything:

| function | in | out | stderr |
|---|---|---|---|
| `rankFilesBySpecTerms` | 3 | **2** | `""` |
| `readFilesContent` | 3 | 2 | `warning: cannot read …/locked.md; its contents were dropped from the prompt (… permission denied)` |

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

## Routed here at the 2026-09-08 re-verification

Two of the items routed to `refusals-that-name-nothing-measurements.md` at that re-verification land
on this spec's subject — a value the code holds and drops without a channel. They are recorded in
this sidecar; the spec body is not edited.

- **A mistyped override key is minted and then dropped.** `internal/engine/frontmatter.go` puts a
  mistyped `tp:` frontmatter key into `fm.Warnings`, and `internal/cli/lint.go` is the only consumer
  — three reads, against zero in `review.go` and `audit.go`. So `tp lint` reports the typo and
  `tp review` / `tp audit` run the whole round under the default the typo silently left in place.
  Source: `spec/0.33.0-candidates.md` item 4.
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

## Routed here from v1.1.0's audit round 3 (2026-09-08)

One further item lands on this spec's subject. It is recorded in this sidecar; the spec body is not
edited.

- **`walkDocTree` refuses the coarse failure and stays silent on the partial one.** Second named
  instance of the swallow class. `internal/cli/review.go:1426-1437` discards `filepath.WalkDir`'s
  return (`_ = filepath.WalkDir(...)`) *and* returns `nil` from the callback on the per-entry error
  (`if err != nil || d.IsDir() { return nil }`), so an unreadable subtree is skipped rather than
  reported. **Run at `e8477464`** on a `docs/` holding `a.md` and `sub/b.md`, via
  `tp review spec.md --perspective documentation --docs-path docs`: readable → exit **0**, tree
  `docs/ ├ a.md └ sub/b.md`; `chmod 000 docs/sub` → exit **0**, **empty stderr**, tree `docs/ └ a.md`;
  a missing docs root → exit **3** with
  `docs path not found or not a directory` and a hint. So the failure that removes everything refuses,
  and the failure that removes half of it is indistinguishable from a smaller `docs/`. The reviewer is
  handed a doc tree with a file missing and no reason to doubt it.

### Not findings — one withdrawal and one pre-empted class

Recorded in this sidecar because both are about the swallow class this spec owns, and both cost a
round to settle. Neither is a defect; the point of writing them down is that the next reader of
`internal/cli/` does not re-derive them.

- **Withdrawn: `internal/cli/review_merge.go:47-50`'s bare `continue`.** The lines are
  `line, err := json.Marshal(f); if err != nil { continue }` — a discarded error with no channel, so
  it matches the class on its face, and it was reported as a finding at round 2. **Two roles
  independently withdrew it at round 3**, on the same ground: neither could construct an input that
  reaches the branch. The rows being marshalled are `reviewFinding` values that tp itself built by
  unmarshalling each input line, so they are marshalable by construction, and the one candidate that
  would not be — a float literal like `1e400`, which `encoding/json` refuses to marshal — never
  survives to that point, because it is rejected at **unmarshal** and the row is dropped earlier. Two
  independent withdrawals of one row is worth more than the row was: it is the *unreachable* case of
  the swallow class, and a spec that fences the class should say so rather than leaving a reader to
  find the same branch and file it a third time.
- **Pre-empted, so it is not filed: the `ResolveWorkflow` and `runMechanicalChecks` call sites are
  not swallowed errors.** Both functions return a second value that is discarded at most call sites,
  which reads exactly like `x, _ := f()` over an `error` — and neither second value is an error.
  `engine.ResolveWorkflow` (`internal/engine/workflow_resolve.go:16`) is
  `func ResolveWorkflow(specPath, explicitFile string) (wf model.Workflow, source string)` — the
  discarded value is the **layer name**, for diagnostics. `runMechanicalChecks`
  (`internal/cli/review_status.go:186`) is
  `func runMechanicalChecks(wf *model.Workflow, taskFilePath string) (results []map[string]any, allPass bool)`
  — the discarded value is a **bool**. The signature is the anchor here and the count is deliberately
  not: an attempt to state one produced 14 non-test call sites against the 13 the finding had claimed,
  and 10 under the narrower "discards with `, _`" rule, so the number depends on a counting rule
  nobody had fixed. Read the signature, not a tally.
