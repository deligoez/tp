# 0.23.0 — Audit Redesign, Enforcement, Round Memory, and Domain Lenses

One release, two feedback sources. First: restructure `tp audit` prompts around three research-backed pillars — multi-role checklist routing, finding categorization, per-item structured embedding — as a clean break from the v0.22.0 audit JSON schema. Second: close the "tp promises a control it does not enforce" gap reported from a 45-chapter bilingual book project that drove tp through six review rounds.

## Motivation

### Audit feedback (agent consuming v0.22.0)

> 1. Single role across multiple prompts — `tp audit --json` produced 2 prompts both with `role: implementation-auditor`. `tp review` already provides perspective diversity (implementer/tester/architect); audit needs the same.
> 2. `category: null` — prompt category is empty. `tp review` has a completeness/ambiguity/consistency taxonomy; audit needs analogous categories (security/idempotency/api-design/test-coverage proposed).
> 3. No code context embedded — `prompts.prompt` field carries spec excerpt only. `files` (50 paths) and checklist references live elsewhere. Sub-agent has to grep/read on its own → token waste + inconsistent audits.

Industry research (`tp audit` v0.22.0 round-trip findings):

- **Multi-role audits** (Panel of LLM Judges, arXiv 2404.18796) — 3 disjoint judges beat one GPT-4 at 7× lower cost. 5+ roles plateau or regress (role-boundary violations, arXiv 2503.13657).
- **Finding categorization** — CodeQL/Semgrep converge on 4-6 single-axis categories. SonarQube two-axis model adds cognitive overhead. User's proposed `idempotency`/`api-design`/`test-coverage` are too narrow / belong elsewhere.
- **Context embedding** — Anthropic ("send a librarian, not the library"), Aider repo map (PageRank + 1K token budget), Chroma context-rot research (degradation past 10K tokens). Per-checklist-item structured embedding is the single biggest reliability lever.

### Book-project feedback (nine items)

A bilingual book project (45 chapters, 1000+ tests) used tp's spec → review → task → audit loop for content work across six review rounds. Nine findings came out of that usage; each was verified against the tp codebase before this spec was written:

| # | Finding (condensed) | Verification result |
|---|---------------------|---------------------|
| 1 | `quality_gate` is never executed; a task closes even when the gate command fails | Confirmed. No exec path exists; worse, `--gate-passed` also disables all closure verification (`closure.go` early return) and SKILL.md recommends the bypass |
| 2 | Convergence enforcement is prose-only; nothing counts clean rounds | Confirmed. `review_clean_rounds` is written by `tp init` and read by nothing; `tp review` output hardcodes a weaker "no new high-severity findings" rule that contradicts SKILL.md |
| 3 | Acceptance verification is keyword matching; it fails on prose criteria, so agents bypass it | Confirmed. English stopwords + code-shaped term regex; the error hint itself recommends `--gate-passed` |
| 4 | Findings have no lifecycle (open/applied/rejected) | Capability shipped in v0.16.0 (`--resolve` with fixed/wontfix/duplicate + evidence); SKILL.md never teaches it |
| 5 | Round N+1 rediscovers the spec from scratch; previous findings and the spec diff never reach the prompt | `--findings` (v0.12.0) and `--diff-from` exist but are opt-in and absent from SKILL.md workflow steps |
| 6 | Regressions introduced by round-N fixes are found one round late | Confirmed missing; no regression concept anywhere in the codebase |
| 7 | Review prompts assume software engineering ("error handling", "backward compatibility", "scalability") | Confirmed. Personas and questions are hardcoded in the prompt generators |
| 8 | `source_lines` becomes meaningless on every spec rewrite | Confirmed. Line coverage reads `source_lines` only; section spans are never derived even though `source_sections` resolution exists |
| 9 | No finding-class concept; mechanizable bug classes are rediscovered round after round | Confirmed. `category` describes a finding's nature, not its mechanizability |

### Resolution map

| Feedback # | Resolution | Sections |
|-----------|------------|----------|
| 1 | Execute the gate at close time; recorded `--skip-gate` escape | §6 |
| 2 | Round state + `tp review --record` / `--status --check`; `tp import` refuses non-converged specs | §8, §9, §10 |
| 3 | Delete keyword matching; N criteria → N evidence lines | §7 |
| 4 | State directory owns round files; SKILL.md teaches the resolve lifecycle | §8, §16 |
| 5 | Auto previous-findings injection + auto spec diff from snapshots | §8 |
| 6 | `regression` perspective, auto-included from round 2 | §11 |
| 7 | Spec frontmatter: `domain` + `lens` questions injected into role prompts | §12 |
| 8 | Section-derived line coverage; `source_sections` becomes the primary anchor | §13 |
| 9 | `class` field on findings; recurring-class detection; `workflow.checks` executed by tp | §14, §15 |

## Goals

1. **Three audit roles** with disjoint checklist routing: `spec-coverage`, `security`, `maintainability-conventions`.
2. **Five audit finding categories** (orthogonal to severity): `correctness`, `error-handling`, `security`, `concurrency`, `contract`.
3. **Per-checklist-item embedding**: each item appears as a structured object with `{item_id, type, spec_line, section, text, expected_evidence}` — inline in the prompt body AND as `auditPrompt.ChecklistItems`.
4. **Audit file filtering**: max 20 files per prompt with role-specific selection rules (§5), replacing the 50-file dump.
5. **Audit output schema**: NDJSON, one row per checklist item, documented for sub-agent authors (§4).
6. **Quality gate execution**: `tp done` and `tp close` run `workflow.quality_gate` and refuse closure on failure; `--skip-gate <reason>` is the recorded escape hatch (§6).
7. **Closure verification redesign**: keyword matching deleted; a task with N≥2 acceptance criteria requires N evidence lines in the closure reason (§7).
8. **Review round memory**: `.tp-review/` state directory with auto round numbering, auto previous-findings injection, and auto spec-diff from snapshots (§8).
9. **Recorded convergence**: `tp review --record` / `--status --check` and `tp audit --record` / `--status --check` make the N-consecutive-clean-rounds rule mechanically checkable (§8, §10).
10. **Import enforcement**: `tp import` refuses a spec whose recorded review rounds have not converged (§9).
11. **Regression perspective**: a prompt that checks the spec diff against previously fixed findings (§11).
12. **Domain lenses**: spec frontmatter declares `domain` and per-role `lens` questions; tp injects them into review prompts (§12).
13. **Section-derived line coverage**: tasks anchored by `source_sections` get line coverage computed from heading spans; `source_lines` becomes optional precision (§13).
14. **Finding class**: optional `class` field on review findings; `--report` and `--record` surface recurring classes as mechanization candidates (§14).
15. **Mechanical checks**: `workflow.checks` registry; tp executes registered checks before generating review prompts and excludes their classes from prompts (§15).
16. **SKILL.md rewrite**: the review loop recipe, closure rules, gate behavior, and audit migration are documented in the skill (§16).

## Non-Goals (Deferred to Later Releases)

- **`tp audit --merge` command** — full NDJSON consumption with schema validation, deduplication, and summary aggregation. `tp audit --record` (§10) implements minimal row counting only; the full parser deserves its own design.
- **Token budget enforcement** — estimator algorithm, 30+ item splitting, cache breakpoint markers.
- **Cached prefix optimization** — Anthropic SDK cache breakpoints, prompt-caching benchmark.
- **Migration helper / deprecation flow** — the audit clean break is intentional; migration guidance lives in SKILL.md.
- **A 4th audit role or 6th audit finding category** — research evidence: 3-5 is the convergent zone; 5+ regresses.
- **Domain-aware audit roles** — the `lens`/`domain` frontmatter (§12) applies to `tp review` only in this release; audit prompts keep their three fixed roles.
- **Automatic mechanization** — tp never authors check commands; the agent writes them, tp executes them.
- **State directory collision handling** — the state directory is keyed by spec basename next to the spec file; two specs with the same basename in different directories each get their own state; cross-directory dedup is out of scope.

## 1. Three Audit Roles

### 1.1 Role definitions

| Role | Focus | Reference Material | File Selection (see §5) |
|------|-------|-------------------|------------------------|
| `spec-coverage` | Does each spec requirement (table row, numbered list item, task acceptance) have observable code? | Spec excerpt + filtered files | Files in diff AND referenced by any task's `source_lines` |
| `security` | Are these invariants enforced: input validation, error handling, lock discipline, file perms, secrets? | Filtered files only (NO spec excerpt) | Files in diff matching security heuristics (§5.2) |
| `maintainability-conventions` | Does code follow Go idioms + project conventions (CLAUDE.md): error wrapping `%w`, exported symbols documented, function length, naming patterns? | CLAUDE.md excerpt (first 50 lines, "Conventions" section) + filtered files | First 10 files in diff (alphabetical) |

### 1.2 Disjoint checklist routing

Each spec-derived checklist item is routed to exactly one role:

- All `task_acceptance`, `table_row`, `list_item` items → `spec-coverage`
- `security` and `maintainability-conventions` roles use **file-level checklists** (one synthetic checklist item per affected file), NOT spec items

This means there is no overlap: spec-coverage carries the spec checklist; the other two roles carry file-checklist items derived from `affectedFiles`. No deduplication needed (no item appears in two prompts).

### 1.3 Role-specific rules embedded in prompt

Each prompt embeds 3-5 role-specific bullet rules:

- **spec-coverage**: from current `tp audit` rules (state-dependent behaviors are PARTIAL; table rows require edge-case handling; numbered list items describing tests require corresponding test functions; task acceptance requires observable behavior; specific error handling/validation/edge cases must be verified)
- **security**: lock-acquire/release pairing, no `_ = err` swallowing, no string concatenation in queries/paths, file perms 0o600 not 0o644, validation before use
- **maintainability-conventions**: error wrapping with `%w`, exported symbols have doc comments, function length under ~80 lines, naming follows project patterns (lowercase package + camelCase symbols + descriptive names), no leftover TODO/FIXME without ticket

## 2. Audit Finding Categories

### 2.1 Five-category enum

| Category | Covers |
|----------|--------|
| `correctness` | Spec mismatch, off-by-one, wrong field/value, logic error |
| `error-handling` | Unchecked error, swallowed panic, missing validation, ignored return |
| `security` | Input validation, auth, secrets, injection, file permissions, lock discipline |
| `concurrency` | Races, lock order, missing locks, goroutine leaks |
| `contract` | API/CLI shape mismatch with spec: flag names, exit codes, JSON field names, schema |

### 2.2 Category resolution rules

- **Mutually exclusive**: each finding picks exactly one category. When multiple apply, use this **resolution precedence** to pick one (the agent does NOT split into two findings):

  `security` > `concurrency` > `error-handling` > `correctness` > `contract`

  Example: missing auth check → both `security` and `correctness` apply → resolve to `security`.

- **No `other` / `misc`**: a future parser will reject unknown values. If the agent cannot pick, the finding description is too vague — the agent rewrites it.

- **Severity orthogonal**: `severity: error|warning|info` is independent of category.

### 2.3 Category presence rule

In NDJSON output (§4):

- `category` field MUST be present in every row.
- For `status: PASS` → `category: null` (explicit null, not omitted).
- For `status: PARTIAL` or `FAIL` → `category` MUST be one of the 5 enum values (not null).

### 2.4 Prompt-level field naming (avoid confusion with finding category)

The `auditPrompt` struct introduces a NEW field for prompt-level focus:

- `Role` (string): one of `spec-coverage`, `security`, `maintainability-conventions`
- (REMOVED) `Category` (was `null` in v0.22.0): no longer present in v0.23.0 to avoid ambiguity with finding category in NDJSON output

Finding category lives ONLY in NDJSON output rows. There is no prompt-level category field.

## 3. Audit Prompt Structure

### 3.1 Body order

Each prompt body is built in this fixed order:

```
## Role
{role-name}

## Role Rules
- {rule 1}
- {rule 2}
- ...

## Spec Excerpt        (spec-coverage ONLY)
{spec content}

## Project Context     (CLAUDE.md excerpt for maintainability-conventions ONLY)
{first 50 lines of CLAUDE.md}

## Checklist
[
  {"item_id": "list-2-3", "type": "list_item", "spec_line": 42, "section": "## 2. Backend Migration", "text": "Plain text persists as canonical", "expected_evidence": "internal/cli/importcmd.go normalize call"},
  ...
]

## Affected Files (max 20)
- internal/engine/coverage.go (touches §1.3, lines 12-41; diff: +13/-48)
- ...

## Output Schema
{NDJSON schema spec — see §4}
```

### 3.2 Per-item embedding

Each checklist item is a JSON object emitted inline in the prompt body. The same items are also exposed as `auditPrompt.ChecklistItems` ([]ChecklistItem) for programmatic consumers.

```go
type ChecklistItem struct {
    ItemID           string `json:"item_id"`
    Type             string `json:"type"`              // list_item | table_row | task_acceptance | file_check
    SpecLine         int    `json:"spec_line"`         // 0 for file-level items
    Section          string `json:"section"`           // canonical heading or file path
    Text             string `json:"text"`
    ExpectedEvidence string `json:"expected_evidence"` // see §3.3
}
```

### 3.3 `expected_evidence` heuristic

Generated deterministically from item metadata:

- For spec items with `source_lines`: `"file paths from task source_lines: <comma-separated paths>"`
- For spec items WITHOUT `source_lines`: `"search code under section %q for keywords from item text"`
- For file-level items (security / maintainability roles): `"inspect file: <path>"`
- Empty fallback: `""` (never nil)

The match is advisory text for the sub-agent — no tool enforces it. It's a hint to reduce sub-agent search cost.

### 3.4 No spec excerpt for non-spec roles

`security` and `maintainability-conventions` roles do NOT include the spec excerpt (per §1.1 table). This keeps their prompts focused on file content + role rules. Spec-coverage retains the full spec excerpt.

`maintainability-conventions` includes the first 50 lines of CLAUDE.md (the "Conventions" section if present) so the sub-agent has project naming/style guidance.

## 4. Audit Output Schema (NDJSON)

### 4.1 One row per checklist item

Sub-agents return one NDJSON line per checklist item:

```jsonl
{"item_id": "list-2-3", "status": "PASS", "evidence_file": "internal/cli/importcmd.go", "evidence_lines": "127-131", "category": null, "severity": null, "notes": ""}
{"item_id": "task-foo", "status": "FAIL", "evidence_file": null, "evidence_lines": null, "category": "correctness", "severity": "error", "notes": "spec §2.1 says X, code returns Y"}
{"item_id": "list-3-1", "status": "PARTIAL", "evidence_file": "internal/engine/x.go", "evidence_lines": "10-25", "category": "error-handling", "severity": "warning", "notes": "validation present but error swallowed at line 22"}
```

### 4.2 Field requirements

| Field | Required | Constraint |
|-------|----------|------------|
| `item_id` | always | must match a checklist item id from the prompt |
| `status` | always | one of `PASS`, `PARTIAL`, `FAIL` |
| `evidence_file` | when status is `PASS` or `PARTIAL` | repo-relative path. For `FAIL`: `null` |
| `evidence_lines` | when status is `PASS` or `PARTIAL` | range `"42-58"` OR single line `"42"`. For `FAIL`: `null` |
| `category` | always present (field MUST exist) | `null` if status is `PASS`; one of 5 enum values if `PARTIAL` or `FAIL` |
| `severity` | always present (field MUST exist) | `null` if status is `PASS`; one of `error`, `warning`, `info` if `PARTIAL` or `FAIL` |
| `notes` | always | short string, max 500 chars; empty string `""` if no notes |
| `class` | optional | kebab-case slug naming a mechanically checkable pattern (§14); omit when not classifiable |

### 4.3 Single-line evidence_lines example

Both forms are valid: `"42-58"` (range) and `"42"` (single line). Implementations parsing this field MUST handle both.

### 4.4 No full parser bundled in v0.23.0

This release documents the schema as guidance for sub-agent authors. `tp audit --record` (§10) reads rows only to count non-PASS entries; the full parser (`tp audit --merge` with schema validation and dedup) is deferred per Non-Goals.

## 5. Audit File Filtering

### 5.1 Maximum cap

Each prompt's affected files list is capped at **20 entries**, regardless of role.

### 5.2 Per-role selection rules

- **`spec-coverage`**: files in `git diff base..HEAD` AND referenced by any task's `source_lines`. Ranked by `source_lines ∩ diff_lines` overlap (line count, descending). Tie-break: alphabetical path.
- **`security`**: files in `git diff base..HEAD` whose **path** contains any case-insensitive substring from `{lock, validate, auth, secret, perm}` OR whose **first 200 lines** contain any of those substrings (case-insensitive). Ranked alphabetical.
- **`maintainability-conventions`**: first 10 files in `git diff base..HEAD` (alphabetical), regardless of content. Capped at 10 (not 20) for this role.

### 5.3 Overlap metric (spec-coverage role)

`overlap = count of line numbers present in BOTH the union of all task source_lines AND the set of changed line numbers from git diff for that file`. Higher overlap ranks first. Sort is stable; tie-break alphabetical path.

### 5.4 File entry format

```
- internal/engine/coverage.go (touches §1.3 Example resolution table, lines 12-41; diff: +13/-48)
```

Format breakdown:
- Path (repo-relative)
- `(touches §X.Y <section>, lines A-B; diff: +N/-M)` annotation
- For roles where `touches` doesn't apply (`security`, `maintainability-conventions`): emit `(diff: +N/-M)` only

### 5.5 Drop rules

Across all roles, drop:
- Binary files (existing `isBinaryFile` check)
- Test fixtures (`testdata/**`, `*.golden`)

## 6. Quality Gate Execution

`workflow.quality_gate` stops being an annotation and becomes an executed contract. When the field is non-empty, closing a task runs the command; a failing command blocks the close.

### 6.1 Trigger

| Command | Gate runs |
|---------|-----------|
| `tp done <id> [...]` (single and multi-ID) | Once per invocation, before any task state mutation |
| `tp done --batch <file>` | Once per invocation, before any entry is processed |
| `tp done <id> --auto-commit` | Once, before the commit and the close |
| `tp close <id> <reason>` | Once per invocation |
| `tp commit <id>` | Never — commit is not closure |

The gate runs exactly once per process invocation regardless of how many tasks close (batch parity: closing 20 tasks costs one gate run, same as closing 1).

### 6.2 Execution environment

1. Command: `sh -c "<quality_gate>"` on Unix; `cmd /C "<quality_gate>"` on Windows (`GOOS=windows`).
2. Working directory: the directory containing the task file.
3. Environment: inherited from the tp process unchanged.
4. Timeout: `workflow.gate_timeout_seconds` (default 600, valid range 30-3600). A timed-out gate counts as a failure.
5. Output capture: stdout and stderr combined, streamed to an in-memory buffer; only the last 20 lines are reported on failure.

### 6.3 Success path

Every task closed by the invocation gets `gate_passed_at` set to the current UTC time — except tasks closed via `--skip-gate` / `skip_gate` (§6.5), which record the skip instead. The `--gate-passed` flag is no longer needed when a gate is configured.

### 6.4 Failure path

- Single and multi-ID `tp done`, and `tp close`: no task closes. Exit code 4. The JSON error object carries `gate_cmd`, `exit_code` (null on timeout), `output_tail` (last 20 lines), and a `hint`: "fix the gate failure and retry, or close with --skip-gate '<why>' (recorded on the task)". On timeout the message states "gate timed out after <N>s".
- `tp done --batch`: entries carrying `skip_gate` close with the skip recorded; every entry without `skip_gate` fails with the gate error in the `failures` array (existing partial-failure shape).

### 6.5 `--skip-gate <reason>` escape hatch

1. `--skip-gate` takes a mandatory non-empty reason string; an empty reason is a usage error (exit 2).
2. When present on `tp done` (single/multi) or `tp close`, the gate is not executed and every closed task records `gate_skipped_reason = <reason>` with `gate_passed_at` left null.
3. Batch entries opt in per entry via a `skip_gate` string field in the NDJSON.
4. `tp show <id>` and `tp report` display `gate_skipped_reason` when set.
5. `tp reopen <id>` clears `gate_skipped_reason` (alongside the existing `gate_passed_at` / `commit_sha` clearing).

### 6.6 `--gate-passed` compatibility

- `workflow.quality_gate` non-empty: the flag is ignored; an info message states "quality gate is executed by tp; --gate-passed ignored".
- `workflow.quality_gate` empty: the flag keeps its attestation behavior (sets `gate_passed_at`), preserving gate-less projects.
- In both cases the flag no longer affects closure verification (§7 removes that coupling).

### 6.7 Workflow field summary

| Field | Type | Default | Range | `tp set --workflow` |
|-------|------|---------|-------|---------------------|
| `quality_gate` | string | `""` | — | read-only (unchanged) |
| `gate_timeout_seconds` | int | 600 | 30-3600 | settable |
| `checks` | array of `{class, cmd}` | `[]` | — | settable (§15) |

## 7. Closure Verification Redesign

The keyword matcher produced false negatives on prose criteria and non-English specs, and its documented bypass (`--gate-passed`) disabled verification entirely. It is replaced by a deterministic, language-agnostic structural rule.

### 7.1 Removed checks

1. Per-criterion keyword matching (`ExtractKeywords`, `stopWords`, `technicalTermRegex` deleted from `closure.go`).
2. Minimum reason length (`len(reason) >= len(acceptance)/2`) deleted.
3. The `gatePassed` parameter and its skip-everything early return deleted from `VerifyClosure`.

### 7.2 Evidence-line rule

1. `ParseAcceptanceCriteria` (unchanged) yields N criteria.
2. N ≥ 2: the reason MUST contain at least N evidence lines. An evidence line is a line whose trimmed form starts with `- `.
3. N ≤ 1: any non-empty reason passes (subject to §7.3).
4. Multi-ID `tp done` with a shared reason: each task is checked independently — the shared reason must satisfy the largest criteria count among the tasks.

Example:

```
acceptance: "Model exists. Migration runs. Tests pass."
tp done task-1 "- Task model at internal/model/task.go:18
- migration 0007 applied, schema verified
- go test ./... green (312 tests)"
```

### 7.3 Retained checks

- Forbidden patterns, unchanged: single-word reason; "deferred" / "will be done later"; "covered by existing" without a path; "not needed" without "because".
- `--covered-by` closures skip all checks (the referenced task already carries verified evidence).

### 7.4 Error format

On failure, the error message states the criteria count and the evidence-line count found; the hint enumerates each parsed criterion so the agent can write one evidence line per criterion without re-reading the task.

### 7.5 Batch parity

Batch entries follow the same rule — the NDJSON `reason` string may contain `\n` characters, so multi-line evidence works identically in batch mode.

## 8. Review Round Memory

tp owns the review round lifecycle through a state directory. Rounds are numbered by tp, previous findings and the spec diff reach the prompts automatically, and convergence becomes a recorded, checkable fact instead of prose.

### 8.1 State directory layout

Location: `<spec-dir>/.tp-review/<spec-base>/` where `<spec-base>` is the spec filename without extension (e.g. `spec/.tp-review/0.23.0/` for `spec/0.23.0.md`).

| File | Content |
|------|---------|
| `state.json` | Round index: recorded review and audit rounds with counts and timestamps |
| `snapshot-round-<N>.md` | Byte copy of the spec at the moment round N prompts were generated |
| `review-round-<N>.ndjson` | Findings recorded for review round N (copied by `--record`) |
| `audit-round-<N>.ndjson` | Findings recorded for audit round N (copied by `tp audit --record`) |

### 8.2 state.json schema

```json
{
  "spec": "spec/0.23.0.md",
  "review_rounds": [
    {"round": 1, "findings": 4, "clean": false, "recorded_at": "2026-07-20T12:00:00Z", "file": "review-round-1.ndjson"}
  ],
  "audit_rounds": []
}
```

### 8.3 Round lifecycle at prompt generation

`tp review <spec>` (default mode) with state enabled:

1. R = number of recorded review rounds + 1. The state directory is created on first use.
2. `snapshot-round-<R>.md` is written (overwritten if the same round's prompts are regenerated before recording).
3. Previous findings: all rows from `review-round-1..R-1.ndjson`, deduplicated by the existing finding identity key, feed the findings summary injected into every prompt. An explicit `--findings` flag overrides this source.
4. When R ≥ 2: a section-level diff (`DiffSections`) between `snapshot-round-<R-1>.md` and the current spec is injected into every prompt as a "Changed sections since round R-1" block — changed/added/removed heading list plus per-section new content capped at 40 lines per section and 6000 characters total. An explicit `--diff-from` flag overrides the baseline.
5. The regression prompt (§11) is auto-appended when R ≥ 2 AND (the diff contains at least one changed, added, or removed section OR at least one recorded finding has `resolved.status == "fixed"`).
6. Flag interactions: `--round` conflicting with the state-derived R is a usage error (exit 2) directing to `--no-state`; `--no-state` disables all state reads and writes and restores pre-0.23.0 manual behavior; `--record` combined with `--no-state` is a usage error.

The `documentation`, `testing`, and `code-audit` perspectives neither read nor write state.

### 8.4 `tp review <spec> --record <findings.ndjson>`

1. Every non-empty line of the file MUST parse as a JSON object; a parse failure aborts with exit 1 and the offending line number. An empty or whitespace-only file records a clean round.
2. The file is copied to `review-round-<R>.ndjson` (R = recorded rounds + 1) and a round entry `{round, findings, clean, recorded_at, file}` is appended to `state.json`, where `findings` = row count and `clean` = (findings == 0).
3. Rounds are immutable once recorded; later `--resolve` edits to the round file change finding resolution status but never the recorded count.
4. Output JSON: `{round, findings, clean, consecutive_clean, required_clean_rounds, converged, mechanize_candidates}` (§14.3 defines `mechanize_candidates`).

### 8.5 `tp review <spec> --status [--check]`

- `--status` prints `{review_rounds, consecutive_clean, required_clean_rounds, converged, mechanical_checks}`.
- `--check` additionally runs every `workflow.checks` entry (§15) and exits 0 only when `converged` is true AND every check passes; otherwise exit 1.

### 8.6 Convergence resolution

- `required_clean_rounds` = `workflow.review_clean_rounds` read from `<spec-base>.tasks.json` next to the spec when that file exists and parses; otherwise the default 2.
- `consecutive_clean` = length of the trailing run of clean rounds in `review_rounds`.
- `converged` = `consecutive_clean >= required_clean_rounds`.

### 8.7 review_loop output schema change

| Field | v0.22.0 | v0.23.0 |
|-------|---------|---------|
| `round` | `--round` flag value | state-derived R (flag must agree or errors) |
| `max_rounds` | hardcoded 2 | REMOVED |
| `convergence` | "no new high-severity findings" | "zero findings (any severity) in `<N>` consecutive rounds" |
| `required_clean_rounds` | absent | resolved per §8.6 |
| `consecutive_clean` | absent | from state |
| `converged` | absent | from state |
| `instruction` | "Stop after 2 rounds or when no new high-severity findings" | spawn sub-agents → merge findings → `--record` → repeat until `--status --check` exits 0 |

This removes the contradiction between tp's own output and SKILL.md's convergence rule.

## 9. Import Convergence Enforcement

### 9.1 Enforcement rule

`tp import` locates the state directory for the resolved spec path. When the directory exists and contains at least one recorded review round: `required` = the workflow `review_clean_rounds` value that will govern the imported file (imported payload's value, or the preserved existing value per §9.3, or default 2). If `consecutive_clean < required`, import fails with exit 1: "review not converged: <X> consecutive clean rounds, <required> required" plus a hint naming `tp review --record` and `--force`. The existing `--force` flag bypasses the check.

### 9.2 No-state behavior

When no state directory exists or it contains zero recorded review rounds, import proceeds and emits an info line "review convergence not verified (no recorded rounds)" (suppressed by `--quiet`). Projects that skip the review loop entirely stay unaffected.

### 9.3 Workflow preservation

When the import target task file already exists and the imported document contains no top-level `workflow` key (raw-JSON key check, before struct defaulting), the existing file's `workflow` block is carried into the imported result. Bare-array imports (which cannot carry workflow) always preserve the existing workflow. This stops `tp import --force` from silently erasing `quality_gate`, convergence parameters, and `checks`.

## 10. Audit Round Recording

### 10.1 `tp audit <spec> --record <results.ndjson>`

1. Every non-empty line MUST parse as a JSON object (same strictness as §8.4).
2. A row counts as a finding when its `status` field is absent OR not equal to `"PASS"` (exact match). `findings` = finding count; `clean` = (findings == 0).
3. The file is copied to `audit-round-<N>.ndjson` and appended to `state.json.audit_rounds`.
4. Output JSON mirrors §8.4 with `required_clean_rounds` = `workflow.audit_clean_rounds`.

### 10.2 `tp audit <spec> --status [--check]`

Mirrors §8.5 for audit rounds: `--check` exits 0 only when audit `converged` is true. Audit `--check` does not run `workflow.checks` (they guard review rounds).

### 10.3 Scope note

`--record` performs row counting only — it validates JSON well-formedness and reads the `status` field. Schema validation of §4.2 field requirements belongs to the deferred `tp audit --merge`.

## 11. Regression Perspective

The most expensive finding class in the field feedback: a round-N fix silently reverting a decision established in an earlier round, discovered one full round later. A focused prompt catches this class before it ships into the next round.

### 11.1 Standalone invocation

`tp review <spec> --perspective regression` requires either (a) a state directory with R ≥ 2, or (b) explicit `--diff-from <old-spec>` plus `--findings <file>`. Missing inputs are a usage error (exit 2).

### 11.2 Auto-inclusion

In default review mode with state enabled, the regression prompt is appended as a 4th entry in `prompts` under the condition in §8.3 item 5. The `instruction` field tells the agent to process the regression prompt first and apply its findings before or together with the three role prompts.

### 11.3 Prompt content

Body order:

1. Persona: "You guard decisions this spec has already settled. Your only job is to find changes that undo them."
2. "Changed sections" block: the section-level diff (same construction as §8.3 item 4).
3. "Previously fixed findings" block: every finding across recorded rounds with `resolved.status == "fixed"`, rendered as `finding — resolution evidence` lines (capped at 50 entries, newest first).
4. Three checks, numbered: (1) does any changed section revert or weaken a fixed finding above? (2) does any changed section contradict an unchanged section? (3) does any change reintroduce a problem that a fixed finding had eliminated in a different section?
5. The standard finding output format (§14.1's extended schema).

### 11.4 Category enum extension

The review finding format's category enum becomes `completeness|ambiguity|consistency|feasibility|redundancy|regression`. The `regression` value is emitted by the regression prompt and accepted by merge/resolve/report.

## 12. Spec Frontmatter and Domain Lenses

Review prompt skeletons stay tp's; the domain questions become the project's. A spec declares its domain and extra review questions in YAML frontmatter; tp injects them into the role prompts.

### 12.1 Frontmatter schema

A spec whose first line is `---` carries a YAML frontmatter block terminated by the next `---` line. tp reads only the `tp:` mapping; every other top-level key is ignored.

```yaml
---
tp:
  domain: prose
  lens:
    all:
      - "Does any chapter summary leak a plot point ahead of its chapter?"
    implementer:
      - "Can each section be written without inventing facts not in the outline?"
    tester:
      - "Is every widget gate condition stated in a checkable form?"
    architect: []
---
```

- `tp.domain`: free string; default `software` when absent. Only the exact value `software` activates the software-specific prompt content (§12.3).
- `tp.lens`: mapping with keys `implementer`, `tester`, `architect`, `all`; each value is a list of strings. Unknown keys under `lens` produce a lint warning and are ignored by review.

### 12.2 Parser exclusion

The frontmatter block (opening `---`, body, closing `---`) is excluded from every spec parser while absolute line numbers stay untouched:

1. `countContentLines` (line coverage) skips frontmatter lines.
2. `ParseHeadings` never yields a heading from inside frontmatter.
3. All `tp lint` rules skip frontmatter lines (a `#` inside a YAML string is not a heading, `---` is not a duplicate line).
4. Structured element extraction (tables, numbered lists) skips frontmatter.
5. `DiffSections` treats frontmatter as belonging to no section.
6. Spec content embedded into or referenced by review prompts excludes the frontmatter block.

### 12.3 Prompt integration

| Element | `domain: software` (default) | any other domain |
|---------|------------------------------|------------------|
| Implementer persona | "senior engineer who must implement this spec tomorrow" (unchanged) | "You must execute this spec exactly as written, starting tomorrow" |
| Tester persona | "QA engineer who must write tests from this spec" (unchanged) | "You must verify every claim in this spec with a pass/fail procedure" |
| Architect persona | "senior architect reviewing this spec for approval" (unchanged) | "You review this spec for internal consistency and structural soundness" |
| Implementer question "What happens when the happy path fails? Where are the error handling gaps?" | emitted | dropped |
| Architect question "Is there a 'What doesn't change' or backward compatibility section?..." | emitted | dropped |
| Architect question "Are there performance or scalability implications not addressed?" | emitted | dropped |
| Structural table/list questions (all roles) | emitted | emitted (domain-neutral) |
| Tester vague-language and divergent-interpretation questions | emitted | emitted (domain-neutral) |

Lens injection: `lens.all` entries are appended to the numbered check list of every generated prompt (implementer, tester, architect, regression); `lens.<role>` entries are appended to that role's prompt only. Order: hardcoded questions first, then `all`, then role-specific.

### 12.4 Lint reporting

`tp lint --json` gains a `frontmatter` object: `{present, lines, domain, lens_roles}` (`lines` as `"1-K"`). Malformed YAML in the frontmatter is a lint error naming the YAML parse failure; unknown `lens` keys and non-list lens values are lint warnings.

### 12.5 Dependency

YAML parsing uses `gopkg.in/yaml.v3` — the first YAML dependency in tp; JSON stays the only output format.

## 13. Section-Derived Line Coverage

`source_lines` dies on every spec rewrite; heading anchors survive. Line coverage now derives from `source_sections` when explicit lines are absent, making sections the primary anchor and lines an optional precision layer.

### 13.1 Section span rule

For a heading at line L with level V, its span runs from L through the line before the next heading with level ≤ V, or through the last line of the file. A section's span therefore includes its subsections.

### 13.2 Covered-set construction

`ValidateLineCoverage` builds the covered set as the union of:

1. Every task's parsed `source_lines` ranges (unchanged behavior).
2. For every task, the span (§13.1) of each `source_sections` entry that resolves unambiguously via `ResolveSection`; ambiguous or unresolvable entries are skipped, matching `AutoFillCoverage`.

The "no coverage computable" warning now fires only when the union is empty.

### 13.3 Anchor requirement change

`tp validate` reports an error for a task that has neither a non-empty `source_sections` nor a `source_lines` value. A task with only `source_sections` is fully valid; `source_lines` remains supported and adds line-level precision where sections are too coarse.

### 13.4 Documentation rule change

SKILL.md decomposition rule 4 is rewritten: every task MUST have `source_sections` (canonical headings); `source_lines` is optional precision. CLAUDE.md's "Every task MUST have `source_lines`" rule is updated to match.

## 14. Finding Class

### 14.1 `class` field

The review finding NDJSON format gains an optional `class` field: a kebab-case slug naming a mechanically checkable pattern (example: `code-citation-drift`). The prompt format instruction reads: "add `class` when the finding is an instance of a pattern a script could check across the whole corpus; omit it otherwise". The `reviewFinding` struct gains `Class string` with `json:"class,omitempty"`; merge and resolve preserve it; the dedup identity key is unchanged.

### 14.2 Report grouping

`tp review --report` adds a `by_class` breakdown (rows with a non-empty `class` only) alongside the existing severity and category breakdowns.

### 14.3 Mechanization candidates

A class is a mechanization candidate when, across the rounds under analysis, it appears in ≥ 2 distinct rounds OR ≥ 5 times within a single round. Both `--report` and `--record` output `mechanize_candidates`: a list of `{class, rounds_seen, total}` sorted by `total` descending. The `--record` output hint states: "write a mechanical check for each candidate class and register it: tp set --workflow checks='[...]'".

## 15. Mechanical Checks

The closing move of the class loop: once the agent mechanizes a finding class, tp runs the check on every subsequent round and tells reviewers to stop looking for that class. Review rounds spend agent judgment only where scripts cannot reach.

### 15.1 `workflow.checks` schema

`workflow.checks` is an array of `{class, cmd}` objects. Validation on write: `class` non-empty kebab-case (`[a-z0-9]+(-[a-z0-9]+)*`) and unique within the array; `cmd` non-empty.

### 15.2 `tp set --workflow checks`

`tp set --workflow checks='<json-array>'` replaces the whole list after validating §15.1 (replace semantics — the agent computes the full desired list). An invalid entry rejects the whole write with exit 1 and names the offending index.

### 15.3 Execution at review time

In default review mode and the regression perspective, before prompt generation, tp runs every registered check sequentially: same execution environment as the gate (§6.2) with `gate_timeout_seconds` applying per check. The review result gains `mechanical_checks`: an array of `{class, cmd, passed, exit_code, output_tail}` where `output_tail` (last 10 lines) is present only for failed checks. Check failures never abort prompt generation.

### 15.4 Convergence integration

`tp review --status --check` (§8.5) runs the same check list and requires every check to pass before exiting 0. A converged round count with a failing mechanical check is not convergence.

### 15.5 Prompt exclusion

When at least one check is registered, every generated review prompt carries: "Mechanically checked classes — do NOT report findings of these classes: `<class list>`". The `instruction` field adds: "If any mechanical check failed, fix those failures before spawning sub-agents".

## 16. Documentation Rewrite

SKILL.md is the reason two shipped features (finding lifecycle, previous-findings injection) went unused in the field. It is rewritten alongside the code:

1. Workflow A order becomes: interview → `tp lint` → `tp init` + `tp set --workflow` (convergence, `quality_gate`, `checks`) → review loop → decompose → `tp import --force`. Running `tp init` before the review loop gives the loop its workflow parameters.
2. The review loop step becomes an explicit recipe: `tp review <spec>` → spawn sub-agents per prompt → merge findings (`--merge`) → fix spec → resolve findings (`--resolve`) → `tp review <spec> --record <merged.ndjson>` → repeat until `tp review <spec> --status --check` exits 0.
3. Closure rule 5 ("Use `--gate-passed` to relax keyword matching") is deleted. The closure rules describe the evidence-line format (§7.2) with one `- ` line per criterion.
4. A gate rule is added: the gate runs automatically at `tp done`; `--skip-gate` requires explicit user approval and is never the agent's own decision.
5. Workflow B examples drop `--gate-passed` and note the automatic gate run.
6. The audit loop step documents `tp audit --record` and `tp audit --status --check` as the convergence mechanism.
7. A "Migration to v0.23.0 audit" section documents the schema break (§1-§5) for downstream consumers.
8. Frontmatter authoring guidance: when to set `domain`, how to write `lens` questions.
9. Class guidance: when to fill `class`, when to mechanize a candidate, how to register `checks`.
10. Decomposition rule 4 rewritten per §13.4; REFERENCE.md documents every new flag, field, and file format from this spec.

README.md and CLAUDE.md follow the pre-release checklist: new commands, flags, workflow fields, state directory, and the updated self-development rules (source anchors, gate behavior) appear in both.

## 17. Backward Compatibility

**Audit: clean break.** v0.23.0 audit JSON schema is incompatible with v0.22.0:

| Field | v0.22.0 | v0.23.0 |
|-------|---------|---------|
| `prompts[].role` | always `"implementation-auditor"` | one of `spec-coverage`, `security`, `maintainability-conventions` |
| `prompts[].category` | always `null` | REMOVED — field no longer present |
| `prompts[].prompt` | paragraph text with checklist as bullets | structured per §3 with embedded JSON-array checklist + project context |
| `prompts[].checklist_items` | absent | array of `ChecklistItem` per §3.2 |
| `prompts[].affected_files` | absent | array of `{path, source_line_ranges, diff_summary}` per §5 |
| Sub-agent output format (advisory) | paragraph: `{ID} \| {STATUS} \| {evidence}` | NDJSON per item per §4 |

Downstream consumers (sub-agents, scripts parsing audit JSON) MUST update to the v0.23.0 schema. No `--legacy-format` flag.

**Review and closure changes:**

| Behavior | v0.22.0 | v0.23.0 |
|----------|---------|---------|
| Multi-criterion closure reason | keyword match against criteria | ≥ N `- ` evidence lines (breaking for free-text reasons) |
| `--gate-passed` | attestation + disables verification | attestation only when no gate configured; ignored otherwise |
| Gate on `tp done` / `tp close` | never runs | runs when `workflow.quality_gate` set; `--skip-gate` escape |
| `review_loop.max_rounds` | present, hardcoded 2 | REMOVED |
| Review round numbering | `--round` flag, agent-managed | state-derived; `--no-state` restores manual behavior |
| `tp import` on reviewed spec | no convergence check | fails when recorded rounds have not converged; `--force` bypasses |
| `tp import --force` over existing file | overwrites workflow with defaults | preserves existing workflow when payload has none |
| Task JSON | — | new optional field `gate_skipped_reason` |
| Workflow JSON | — | new fields `gate_timeout_seconds`, `checks` |
| Review finding NDJSON | 5 categories, no class | + `regression` category, optional `class` field (additive) |
| Spec files starting with `---` | frontmatter lines counted as content | frontmatter excluded from all parsers |
| Dependencies | no YAML | `gopkg.in/yaml.v3` added |

## 18. Implementation Order

1. **Model fields** — `Task.GateSkippedReason`; `Workflow.GateTimeoutSeconds`, `Workflow.Checks` with unmarshal defaults and validation.
2. **Engine: command runner** — shared `RunCommand(cmd, dir, timeoutSeconds)` used by gate and checks (§6.2), with combined-output capture and timeout.
3. **Engine: closure verification v2** — §7 rewrite of `VerifyClosure`; delete keyword machinery.
4. **CLI: gate execution** — wire the runner into `tp done` (all forms) and `tp close`; `--skip-gate`; `--gate-passed` compatibility; batch semantics.
5. **Engine: frontmatter** — YAML block detection + `tp:` parsing; exclusion in `countContentLines`, `ParseHeadings`, lint rules, structured elements, `DiffSections`.
6. **Engine: audit categories + file filtering** — `audit_categories.go` (enum + `ResolveAuditCategory`), `audit_files.go` (`FilterAffectedFilesForRole` per §5).
7. **CLI: audit prompt rewrite** — `auditPrompt` schema change, role routing in `buildChecklist`, `generateAuditPrompts` per §3.
8. **Engine: review state** — state directory layout, `state.json` read/write, consecutive-clean computation, snapshot management.
9. **CLI: review state integration** — auto round numbering, auto previous-findings, auto diff injection, flag interaction errors, `--no-state`.
10. **CLI: `--record` / `--status` / `--check`** — for both `tp review` and `tp audit`; mode routing and exclusivity validation.
11. **CLI: import enforcement** — convergence check (§9.1-9.2) and workflow preservation (§9.3).
12. **CLI: regression perspective** — standalone mode, auto-inclusion, prompt generator, category enum extension.
13. **CLI: domain lenses** — persona/domain switching and lens injection per §12.3.
14. **Engine: section-derived coverage** — §13 span derivation and covered-set union; validate anchor rule.
15. **CLI: finding class** — struct field, `by_class` report breakdown, `mechanize_candidates` in report and record output.
16. **CLI: mechanical checks** — `tp set --workflow checks`, execution at review time, `mechanical_checks` output, prompt exclusions, `--status --check` integration.
17. **Docs** — SKILL.md rewrite (§16), REFERENCE.md, README.md, CLAUDE.md.
18. **Dogfood** — audit this release with its own machinery: recorded rounds, executed gate, convergence check green before tagging.

## 19. Test Plan

### 19.1 Unit tests (engine)

- `TestAuditCategory_EnumValid`: each of 5 values passes `IsValidCategory`; `"other"`, empty, unknown all fail
- `TestAuditCategory_PrecedenceResolution`: `ResolveAuditCategory(["correctness", "security"])` → `"security"`; full precedence order verified
- `TestFilterFiles_SpecCoverage_RanksByOverlap`: larger `source_lines ∩ diff` overlap ranks first; equal overlaps tie-break alphabetically
- `TestFilterFiles_SpecCoverage_Cap20`: 50 input files yields ≤ 20 entries
- `TestFilterFiles_Security_HeuristicMatch`: path heuristic matches `internal/lock/lock.go`, rejects `internal/utils/x.go`; content heuristic tested via fixture
- `TestFilterFiles_Maintainability_Cap10`: maintainability cap is 10, not 20
- `TestFilterFiles_DropsBinaryAndFixtures`: `*.golden`, `testdata/**`, `*.png` all dropped
- `TestRunCommand_Success_Failure_Timeout`: exit 0 → passed; non-zero → failure with output tail; sleep past timeout → failure with timeout message
- `TestVerifyClosure_EvidenceLines`: 3 criteria + 3 `- ` lines passes; 3 criteria + 2 lines fails naming counts; 1 criterion + free text passes
- `TestVerifyClosure_ForbiddenPatternsRetained`: "deferred", single-word, "covered by existing" without path still rejected
- `TestVerifyClosure_KeywordMachineryGone`: prose criteria in any language pass with matching evidence-line count
- `TestFrontmatter_ParseAndExclusion`: `tp:` mapping parsed; content lines, headings, lint findings, structured elements all exclude the block; absolute line numbers preserved
- `TestFrontmatter_MalformedYAML`: parse failure surfaces as lint error
- `TestSectionSpan_Derivation`: span ends before next same-or-higher-level heading; last section spans to EOF; subsections included
- `TestLineCoverage_SectionDerived`: task with only `source_sections` covers its span; union with explicit `source_lines` verified
- `TestReviewState_RoundLifecycle`: record appends immutable entries; consecutive-clean over mixed sequences (clean, dirty, clean, clean → 2)
- `TestWorkflowChecks_Validation`: invalid slug, duplicate class, empty cmd each rejected with index

### 19.2 Unit tests (cli)

- `TestRouteChecklist_Disjoint`: each spec-derived item appears in exactly one role bucket; file-level items only in security/maintainability
- `TestGenerateAuditPrompts_ThreeRolesPresent`: exactly 3 prompts when all roles have items
- `TestGenerateAuditPrompts_StructuredItems`: JSON-array checklist in body; `ChecklistItems` populated identically
- `TestGenerateAuditPrompts_NoCategoryField`: `auditPrompt` JSON output has no `Category` key
- `TestGenerateAuditPrompts_SpecExcerptOnlyForSpecCoverage`: spec excerpt only in `spec-coverage` body
- `TestGenerateAuditPrompts_CLAUDEmdOnlyForMaintainability`: CLAUDE.md excerpt only in `maintainability-conventions` body
- `TestDoneGate_RunsOncePerInvocation`: batch of 3 closes runs gate once; failure closes nothing (non-batch) / only skip entries (batch)
- `TestDoneGate_SkipGateRecorded`: `gate_skipped_reason` set, `gate_passed_at` null, reopen clears it
- `TestDoneGate_GatePassedCompat`: flag ignored with info when gate set; attestation preserved when gate empty
- `TestReviewState_AutoRound`: second `tp review` reports round 2 with previous findings injected and diff block present
- `TestReviewRecord_CleanAndDirty`: empty file → clean round; parse error → exit 1 with line number
- `TestReviewStatus_Check`: converged + passing checks → exit 0; either failing → exit 1
- `TestImport_ConvergenceEnforced`: unconverged state → exit 1; `--force` bypasses; no state → info only
- `TestImport_WorkflowPreserved`: `--force` re-import without workflow key keeps existing `quality_gate` and `checks`
- `TestRegressionPrompt_AutoInclusion`: appended only when diff or fixed findings exist; standalone mode requires inputs
- `TestLensInjection_DomainSwitch`: non-software domain swaps personas and drops the three software questions; lens questions appended in order
- `TestReport_ByClassAndCandidates`: class grouping and the ≥2-rounds / ≥5-single-round candidate rules
- `TestChecksExecution_PromptExclusion`: registered classes listed in every prompt; failed check appears in `mechanical_checks` with output tail

### 19.3 Integration tests

- `TestAudit_FullShape`: end-to-end on a small spec; JSON contains 3 role-tagged prompts with required fields per §3, §5, §17
- `TestAudit_FileFilterCap`: 50-file spec yields `affected_files` ≤ 20 (≤ 10 maintainability)
- `TestAudit_NoLegacyCategoryField`: audit JSON has no `Category` key
- `TestSelfLoop_ReviewToImport`: lint → init → review → record dirty round → import fails → record clean rounds → import succeeds
- `TestGateLoop_EndToEnd`: temp project with failing gate blocks `tp done`, passing gate stamps `gate_passed_at`, `--skip-gate` records reason

## 20. Acceptance Criteria

- `tp audit` emits 3 distinct prompts (one per role) when spec + diff are non-empty; each prompt has `role` ∈ {`spec-coverage`, `security`, `maintainability-conventions`}
- `auditPrompt` JSON has no `Category` field; body order matches §3.1; checklist items embedded per §3.2 and exposed as `ChecklistItems`
- `expected_evidence` populated per §3.3 (never null; non-empty for items with source_lines)
- Audit `affected_files` capped and filtered per §5 role rules
- 5 audit categories defined as engine constants with `ResolveAuditCategory` precedence; NDJSON schema (§4) documented in SKILL.md
- A failing `quality_gate` command blocks `tp done` and `tp close` with exit 4 and the output tail; a passing gate stamps `gate_passed_at` on every closed task; the gate runs once per invocation
- `--skip-gate <reason>` closes without running the gate and records `gate_skipped_reason`, visible in `tp show` and `tp report`
- `VerifyClosure` contains no keyword extraction; a task with N ≥ 2 criteria requires ≥ N `- ` evidence lines; forbidden patterns still reject
- `tp review` derives the round number from `.tp-review/` state, injects previous findings and the snapshot diff automatically, and errors on a conflicting `--round`
- `tp review --record` appends an immutable round entry and reports `consecutive_clean`, `converged`, and `mechanize_candidates`
- `tp review --status --check` exits 0 only when rounds are converged AND every registered check passes
- `tp import` fails on an unconverged reviewed spec, proceeds with an info line when no rounds exist, and preserves the existing workflow block when the payload has none
- `tp audit --record` / `--status --check` mirror the review machinery using `audit_clean_rounds`
- The regression prompt auto-appears from round 2 when the spec changed or fixed findings exist, and is invocable standalone via `--perspective regression`
- A spec with `tp.domain` other than `software` gets neutral personas and loses the three software-specific questions; `lens` questions appear in the designated prompts; frontmatter lines are invisible to lint, coverage, structured elements, and diff
- A task anchored only by `source_sections` receives line coverage from heading spans; a task with neither anchor fails validation
- Review findings accept optional `class`; `--report` groups by class; candidate classes (≥ 2 rounds or ≥ 5 in one round) surface in report and record output
- Registered `workflow.checks` run before review prompt generation, their classes are excluded in every prompt, and their failures appear in `mechanical_checks`
- `review_loop` output carries `required_clean_rounds`, `consecutive_clean`, `converged`, and the zero-findings convergence text; `max_rounds` is gone
- SKILL.md contains the review loop recipe, the evidence-line closure format, the gate/skip-gate policy, the audit migration section, and the rewritten source-anchor rule; REFERENCE.md, README.md, and CLAUDE.md reflect every new command, flag, and field
- All new unit and integration tests pass; existing tests updated for the new schemas
