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

## Widened on 2026-09-11 — what the body absorbed and what it changed

The body was rewritten on 2026-09-11 as *tp says when it could not read*. What it took, and from
where:

| body § | from | what changed on the way in |
|---|---|---|
| §2 frontmatter | this sidecar, *Routed here at the 2026-09-08 re-verification*, first bullet | promoted to the lead; `tp audit` measured too |
| §3 sites 1–2 | this spec's old §2 and this sidecar's walkDocTree bullet | old §2.1 chose a notice only, silenced by `--quiet`; the 2026-09-08 decision (`two-advisories-measurements.md`) is *a payload field always; the notice is additional*, and the body now follows it |
| §3 site 3 | this sidecar's over-long-line bullet | — |
| §3 site 4 | `an-invalid-task-file-is-reported-measurements.md`, *Routed here from v1.1.0's audit round 3* | taken because its subject is a read failure reported as a result |
| §4 | `an-invalid-task-file-is-reported.md` §2 | severity lowered from `error` to `warning`, for the reason §4 gives |
| §5 | `two-advisories-measurements.md`, *The raw-stderr sweep* bullet | the channel rule is the decided one |
| §6 item 1 | `context-is-cut-on-a-rune-boundary.md` §2 | unchanged |
| §6 item 2 | `a-findings-exits-agree-measurements.md`, `findingIdentityKey` bullet | taken because it is the same byte-for-character cut |

Two passages of the old body left it and are kept here as its reasoning. **Keeping the unreadable file
in the ranked list at score 0** was considered and not taken: it would change what the prompt carries,
since an unreadable file would occupy one of the fifteen ranking slots whenever fewer than fifteen
files outscore it. **The two reads do not read the same set**: the ranking reads only the rankable
files (`index.md` and `config.*` go to an always-include list it never reads), and the content read
receives the ranked list, from which the ranking's drops are already absent. The two-reads hazard in
Non-Goal 2 survives either way.

Not carried from `an-invalid-task-file-is-reported-measurements.md`: *`validate --project`
under-reports twice* and *`--report` reads a spec as a findings file*. Neither is a read failure: the
first is a payload key omitted when empty, the second a positional fence. They stay in that sidecar as
history.

## Re-verified at `18032abe` (2026-09-11)

Every probe ran with the `HEAD` binary (`tp version v1.1.2-0.20260910212136-18032abe405f`) in a scratch
directory outside the repository, each a fresh `git init`. No item of field report WB-3155 lands on
this spec; the field report's sidecar sections are in the specs its items were routed to.

**§2 — the mistyped lens key.** Spec: a frontmatter block `tp: lens: implementr: ["does the model
validate status?"]`, one `## 1. Models` section.

| command | exit | stderr | the question in a prompt |
|---|---|---|---|
| `tp lint spec.md --json` | 0 | empty; one finding `tp.lens key "implementr" is unknown (known: implementer, tester, architect, all); ignored` | — |
| `tp review spec.md --json` | 0 | **0 bytes** | no role |
| control, key spelled `implementer` | 0 | `tp: lens is deprecated; migrate to tp.review_roles …` | implementer only |

The review payload's top-level keys are `spec, spec_ref, spec_path, structured_elements, skipped_roles,
prompts, review_loop` — none carries a frontmatter warning. Source: `internal/engine/frontmatter.go`
`parseLens` puts the unknown key into `fm.Warnings` and never into `fm.Lens`;
`internal/engine/lens_shim.go` `TranslateLegacyLens` returns early on an empty `fm.Lens`, so not even
the deprecation notice fires; `internal/cli/lint.go` appends `fm.Warnings` and nothing in `review.go`
or `audit.go` reads them.

A frontmatter that fails to parse (`implementer: [oops`) gives `tp lint` an `error` finding
(`frontmatter YAML parse failed: …`, exit 1) and `tp review` exit 0 with 0 stderr bytes. On the audit
side, `tp.audit_roles.security.focus: 5` (an active default auditor) gives `tp lint`
`tp.audit_roles.security.focus is not a list (got int); ignored` and
`tp audit au.md --affected-files m.go` exit 0 with 0 stderr bytes. The control that decided the
fixture: the same block naming `go-safety`, which the embedded corpus lacks, draws the shipped notice
`tp.audit_roles override for "go-safety" matches no active auditors role; ignored` — so a fixture must
name an active role or it passes for the wrong reason (row 1c).

Adjacent, not taken (Non-Goal 6): a misspelled top-level key, `tp: review_role: implementer: focus:
[…]`, draws **no** finding from `tp lint` and no notice from `tp review`, and the question reaches no
prompt.

**§3 site 1 — the ranking.** Three `docs/*.md` whose text shares words with the spec's headings, one
`chmod 000` and asserted unreadable first; spec with `## 1. Models` and `## 2. Status fields`;
`tp review spec.md --perspective documentation --docs-path docs --no-state`:

| invocation | `docs_structure.reviewed_files` | stderr |
|---|---|---|
| plain | 3 | the content read's `warning: cannot read docs/locked.md; its contents were dropped from the prompt (…permission denied)` |
| `--spec-inline` | **2** | **0 bytes** |
| `--spec-inline`, file readable (control) | 3 | 0 bytes |

**§3 site 2 — the walk.** `docs/a.md` and `docs/sub/b.md`, same command: readable → exit 0 and the
payload names `sub/b.md`; `chmod 000 docs/sub` → exit 0, 0 stderr bytes, no `b.md` in the payload.
Source: `internal/cli/review.go` `_ = filepath.WalkDir(…)` with `if err != nil || d.IsDir() { return
nil }` in the callback.

**§3 site 3 — the long line.** A 16-line spec with three `##` headings and one 70,000-character line:
`tp lint big.md` exits 3 with `{"error":"bufio.Scanner: token too long", …, "hint":"check the spec
path — this command takes the spec markdown file, not the task file"}`;
`tp review big.md --verify --findings f.ndjson --no-state` exits 0 and its prompt reads
`Spec file: …/big.md (16 lines, 0 sections)`. Source: `headings, _ := engine.ParseHeadings(specPath)`
at `internal/cli/review_verify.go` and `internal/cli/review.go`.

**§3 site 4 — the findings file.** Two lines, each a valid object followed by a trailing comma:
`tp review --merge bad.ndjson` exits 1 with `no line parsed in bad.ndjson: every content line was
skipped, so that input contributed nothing to the merge`; `tp review spec.md --verify --findings
bad.ndjson --no-state` exits 0 with a prompt opening *"Previous review rounds produced 0 findings"*.
Both print the same two `warning: skipping malformed line (invalid JSON) in bad.ndjson` lines.

**§4 — the task file.** Spec `t.md`, task file `t.tasks.json` holding `{ this is not json`:
`tp lint t.md --json` exits 0, 0 stderr bytes, `errors 0 warnings 0 info 0`, no finding. On the same
file `tp status`, `tp validate` and `tp list` (each `--file t.tasks.json`) exit 3 with `parse task
file: invalid character 't' looking for beginning of object key string` — the ground for `warning`
rather than `error`. The `acceptance-quality` findings lint emits are `info` (short acceptance) and
`warning` (removal or completion verbs), in `internal/cli/lint.go` `checkTaskFileQuality`, whose
`os.ReadFile` and `json.Unmarshal` failures each `return nil`.

**§5 — `--merge --quiet`.** One valid row and one `{bad json` line: `tp review --merge a.ndjson --quiet
--json` exits 0, prints `warning: skipping malformed line (invalid JSON) in a.ndjson`, and its payload
carries `"inputs": [{"path": "a.ndjson", "parsed": 1, "skipped": 1}]`. `internal/output/output.go`
`Notice` returns early under `--quiet` and is not suppressed by JSON mode.

**§6 item 1 — the context cut.** A line of 78 `a` and `— tail`, as two identical paragraphs:
`duplicate-paragraph` fires, and its `context` is 84 bytes ending in two `U+FFFD`. Source:
`ctx = ctx[:80]` twice in `internal/engine/vague.go`.

**§6 item 2 — the identity window.** Three scratch repositories, a two-row findings file each, recorded as
round 1, then round 2 emitted:

| rows' `finding` | first 80 chars | first 80 bytes | round 2 `previous_findings` | |
|---|---|---|---|---|
| 30 `ş` + 20 `x`, then `ALPHA-tail` / `BETA-tail` | differ | identical | **1** | wrong |
| `A` / `B` + 120 `z` | differ | differ | 2 | correct |
| 80 `q`, then `ALPHA` / `BETA` | identical | identical | 1 | correct |

Source: `internal/cli/review.go` `findingIdentityKey`, whose comment says *"the first 80 characters of
the finding field"* and whose body is `len(prefix) > findingPrefixLen` / `prefix[:findingPrefixLen]`.

## The raw-stderr sweep

**The counting rule decides the number, so it is written first.** A literal search for `(os.Stderr,`
over `internal/` at `18032abe`, read line by line, then restricted to the ten files the 2026-09-08
sweep named (`internal/cli/review_merge.go`, `audit_merge.go`, `review_verify.go`, `config.go`,
`done.go`, `commit.go`, `commitstrategy.go`, `changewarn.go`, `internal/engine/configresolve.go`,
`discover.go`), returns **18** call sites — 3, 4, 1, 1, 2, 1, 2, 1, 2, 1 in that order, the figure
`two-advisories-measurements.md` recorded at `dd89c566`. Two refinements the implementing task needs:

- **One of the 18 is not an advisory.** `internal/cli/done.go`'s closure-verification failure writes a
  JSON error envelope to stderr and then exits with the validation code. §5 leaves it alone; so the advisories in the ten
  files are 17.
- **The ten files are not every raw write.** The same search also hits `audit_resolve.go`,
  `review_resolve.go` and `use.go` — confirmations such as `resolved finding N as <status>` — and
  `gate.go`, whose two hits are error envelopes. The confirmations are advisories under §5's
  definition; the implementing task enumerates the final set from the search rather than from either
  list.

Of the 17, the ones reporting an input dropped from the result and their payload today:
`review_merge.go` and `audit_merge.go` (skipped lines — payload carries `inputs[].skipped` on the
review side), `audit_merge.go` (an unmarshalable merged row), `review_verify.go` (skipped lines — no
payload count, §3 site 4), and `configresolve.go`/`discover.go` (a `.tp/local.json` `active` pointer
treated as unset — the stale-pointer notice is `a-task-file-write-names-its-target`'s subject).
