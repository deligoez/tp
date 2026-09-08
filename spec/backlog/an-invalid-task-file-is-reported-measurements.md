# an-invalid-task-file-is-reported — measurements

Supplemental material for `an-invalid-task-file-is-reported.md`; the spec stands without it. Every
block below was moved here on 2026-09-08 from `refusals-that-name-nothing-measurements.md`, which had
in turn taken it verbatim from the spec body earlier the same day. The conventions every mutant below
was run under — the `rsync -a --exclude .git ./ <copy>/` copy outside the repository, the
`git status --porcelain -- internal/` check with its control, and the rule that no line number is
cited in the spec body — are in that file's **Preamble** and are unchanged here.

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
before concluding anything, for the reason the spec's row 1b gives.

## Routed here at the 2026-09-08 re-verification

Two of the items routed to `refusals-that-name-nothing-measurements.md` at that re-verification land
on this spec's subject — a command that exits 0 with a payload nothing in it distinguishes from
clean. They are recorded in this sidecar; the spec body is not edited.

- **`validate --project` under-reports twice.** `skipped` is omitted from the payload when empty, so a
  consumer cannot distinguish "nothing skipped" from "this build does not report skips", and
  `--strict` promotes deviations only, leaving the other advisory classes at their default severity.
  Source: `spec/0.35.0-candidates.md` item 10.
- **`--report` reads a spec as a findings file where `--merge` and `--resolve` refuse one.**
  `isSpecLookingPath` (`internal/cli/mode_positionals.go:19`) rejects a `.md`/`.markdown` positional
  at entry, and §4.1 states that fence for `--merge` and `--resolve`/`--resolve-all` only, so
  `--report` is outside it by design rather than by oversight. Measured at `c75e5c3d`:
  `tp review probe-spec.md --report` exits **0** and prints a report whose single round is
  `{file: probe-spec.md, in_file: 0, new: 0, resolved: 0, unresolved: 0}`, while
  `tp review --merge probe-spec.md` exits **2** naming the spec. The number a caller reads from the
  first is a count of nothing, and no channel says so. Source: v1.1.0 audit round 2.

## Routed here from v1.1.0's audit round 3 (2026-09-08)

One further item lands on this spec's subject. It is recorded in this sidecar; the spec body is not
edited.

- **`--verify` has no zero-parse refusal where `--merge` does — and the parse path is *not* silent,
  which is the correction.** `readVerifyFindings` (`internal/cli/review_verify.go:167-201`) makes the
  **read** error fatal (`output.Error(ExitFile, …)` at :170) and warns per malformed line at :184, but
  has no guard for *every* line failing to parse. **Run at `e8477464`** on one two-line file with a
  trailing comma on each line, through one binary: `tp review --merge bad.ndjson` exits **1** with
  `{"error":"no line parsed in bad.ndjson: every content line was skipped, so that input contributed
  nothing to the merge"}` and a re-emit hint; `tp review spec.md --verify --findings bad.ndjson` exits
  **0** with `previous_findings: 0` and a prompt opening *"Previous review rounds produced 0
  findings"*. **Both printed the identical two `warning: skipping malformed line (invalid JSON)`
  lines** — so the difference is the *refusal*, not the warning, and any description of the verify
  parse path as silent is wrong at this revision. What is missing is the guard that turns "every line
  dropped" into a refusal; without it the verifier is told 0 as a *result*. The function's own doc
  comment (:160-166) already forbids this — *"Neither failure may come back as an empty set"* — so the
  contract is stated and half-kept. `--merge` carries `droppedInputs`/`mergeInputCounts`; `--verify`
  populates no counts at all, so nothing in the payload distinguishes a genuinely empty findings file
  from one every line of which was dropped.
