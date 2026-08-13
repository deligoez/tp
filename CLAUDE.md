# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Four Foundational Principles

| # | Principle | Definition |
|---|-----------|------------|
| P1 | AX First | Every decision optimizes for the AI agent |
| P2 | Batch Parity | What's easy for 1 task must be equally easy for N tasks |
| P3 | Minimal Tokens | Every output byte costs agent context |
| P4 | Agent Plans, Tool Executes | Agent produces decisions, tool deterministically executes |

**Always evaluate a change through the AX lens: does this reduce token overhead or round-trips for
the agent?** What tp is, how it installs, and what AX buys the reader are `README.md`'s job.

## Quick Reference

```bash
# Build
go build ./cmd/tp

# Test
go test ./...

# Lint
golangci-lint run

# Quality gate (the project gate in .tp/config.json; run after every task)
go test -race ./... && golangci-lint run

# Stripped binary (production, <10MB)
go build -ldflags="-s -w" -o tp ./cmd/tp
```

## Commands

The command index is in `README.md`; the exact form of every command and flag, plus the workflows
that use them, is the inventory in `skills/tp/SKILL.md`. Do not restate either here — a fourth copy
is a fourth thing to drift.

## Project Structure

```
cmd/tp/              Main entry point
internal/
  cli/               Cobra commands (plan, done, next, list, claim, close, commit, report, ...)
                     review.go          — core review + mode routing + prompt generators
                     review_merge.go    — --merge mode (dedup, sort, output)
                     review_resolve.go  — --resolve/--resolve-all mode (flock, in-place update)
                     review_verify.go   — --verify mode (lightweight verification prompt)
                     review_report.go   — --report mode (convergence analysis, TTY/JSON output)
                     audit.go           — tp audit (post-implementation spec verification)
  engine/            Core logic (toposort, closure, validate, lint, parallel, discover, lock, excerpt, linecoverage, structured)
                     diff.go            — section-level spec diff (for --diff-from)
                     fileio.go          — shared file I/O, budget-aware reading, affected summary
                     suggest.go         — task ID suggestion for covered_by did-you-mean hints
  model/             Data types (TaskFile, Task, Workflow, Coverage)
  output/            Formatting (JSON/TTY, compact, colors, hint errors)
spec/
  0.1.0.md           Original specification (1431 lines)
  <version>.md       New feature specs — one file per version/feature
skills/tp/
  SKILL.md           Claude Code skill (workflows, decomposition rules, commit format)
  REFERENCE.md       Exhaustive field, exit-code and schema detail
.claude-plugin/
  marketplace.json   Skill distribution manifest
```

## Self-Development: tp Uses tp

**tp develops itself using its own workflow.** When implementing new features:

1. **Write a spec** in `spec/<version>.md` describing the feature
2. **Lint the spec**: `tp lint spec/<version>.md`
3. **Init + workflow**: `tp init spec/<version>.md` (no `--quality-gate` — the repo gate lives in `.tp/config.json`, and a task-file override would mask it), then `tp set --workflow` for convergence counts / round budgets / `checks` (before the review loop, so the loop reads them)
4. **Review loop**: `tp review spec/<version>.md` → spawn sub-agents → `tp review --merge` → `tp review spec/<version>.md --record merged.ndjson` → resolve findings → repeat until `tp review spec/<version>.md --status --check` exits 0
5. **Decompose into tasks** with `source_sections` for every task (`source_lines` optional precision)
6. **Import**: `tp import <tasks.json>` (plain — the init shell holds zero tasks; convergence checks stay armed)
7. **Validate**: `tp validate` — check coverage gaps
8. **Implement each task**, then:
   - `tp done <id> "evidence" --commit <sha>` — the quality gate runs automatically
9. **Audit loop**: `tp audit spec/<version>.md` → spawn sub-agents → `tp audit spec/<version>.md --record results.ndjson` → fix code → repeat until `tp audit spec/<version>.md --status --check` exits 0
10. **Report**: Last `tp done` auto-includes report summary. Or: `tp report` for full details
11. **Release**: tag, push, `gh release edit` with notes

### Rules
- **Convergence, gate and budget policy live in `skills/tp/SKILL.md`** (Gate, Budget & Escalation Policy, plus Workflow A step 2 and Workflow D). It applies here unchanged: `--skip-gate`, raising `review_max_rounds`/`audit_max_rounds`, and `tp import --force` are **user-approved decisions, never the agent's own**.
- **The audit's 2-clean-rounds rule is about SPEC CONFORMANCE, not about the codebase at large (v0.32.0 lesson).** SKILL.md's Workflow D carries the rule and the signals that make it measurable (`spec_coverage_clean_rounds`, `role_streaks`, `divergence`). What it cost here: tp's own v0.32.0 audit ran **11 rounds with zero FAIL in any of them** while `spec-coverage`, the only role measuring conformance, was **55/55 clean from round 2 onward** — the rounds went to hint wording, advisory channels and git scoping the spec never mentions, and each repair round created fresh surface for the next. So once `spec-coverage` is clean two rounds running and no round has produced a FAIL, record the out-of-surface findings with justification, name the version that takes them, and ship — never over an open spec-scoped finding, and keep audit repairs minimal, because a repair that introduces a new abstraction (a channel move, a new helper) belongs to the next version rather than to this audit.
- Quality gate after every task: `go test -race ./... && golangci-lint run` — this is the project gate in `.tp/config.json`, and no task file carries a `quality_gate` override that would mask it. `-race` is part of it because Go's race detector is off by default, so without it a data race cannot fail the gate; it costs between a tenth and a half again of the suite's wall time. `golangci-lint run` (v2) only checks formatters (`gofmt`/`gofumpt`) when a `formatters:` section enables them, so tp's `.golangci.yml` enables `gofmt` (else a gofmt-dirty file passes the gate)
- **When `tp review --record` names the same `mechanize_candidates` class for a third round, write the check instead of the prose fix — but only if the thing the check measures already exists in the phase the check runs in.** A class that survived three prose corrections will survive the fourth, so the prose fix is not the answer; the phase is what decides whether a check can be the answer instead. Registered `workflow.checks` run in the **review phase only**, so a check whose subject is an artifact a later phase produces cannot verify that subject in any round. Registering it early is not a harmless head start: tp tells every reviewer to stop reporting a mechanized class, so a check registered before its subject exists suppresses that finding class for every reviewer while verifying only whatever part of its subject already exists — which may be nothing. v0.34.0 §7.1 is the worked instance: a check over a guard-test list that implementation writes was registered for eight rounds, could never compare the list against its derivation, and the first round after unregistering it surfaced three stale measured claims the suppression had hidden. So — subject already in the spec: register it with `tp set --workflow checks='[{"class":"...","cmd":"..."}]'` and `tp review` runs it every round (`scripts/check-test-inventory.py` is the worked example). Subject written by a later phase: keep it out of `checks`, and make running it the acceptance of the task that creates the subject. Either way, test the check itself before trusting it — check-test-inventory's first version mis-parsed the spec and its literal set was both too broad and too narrow.
- **Guard a dangerous value at the sink, not at the entry point.** A `tp done --commit` check does not protect `commit_shas` written by `tp import`/`tp add`; `engine.SafeGitRev` is applied wherever a caller-supplied string becomes a git argument. When an audit finding says a fix is incomplete, re-derive the claim rather than defending it.
- **This repo's role corpus (custom since v0.26.0)** — tp dogfoods a custom 4+4 corpus: `.tp/reviewers/{implementer,tester,architect,ax-economist}.json` + `.tp/auditors/{spec-coverage,go-safety,maintainability-conventions,ax-contract}.json` — the three tuned defaults plus two tp-specific lenses (`ax-economist` = token/round-trip/knob economy in spec review; `ax-contract` = the agent-facing JSON output contract in code audit). `domains` is omitted (tp is single-domain). Every `tp review`/`tp audit` therefore dogfoods corpus emission, `(location,class)` dedup, the per-role overlap report and per-phase `roles_hash` staleness. The corpus format, the frontmatter overrides and per-spec deactivation are documented in `skills/tp/SKILL.md`.
- **Dogfood the in-progress binary** — during self-development, always run tp against its OWN repo with a freshly built binary (`go build -o /tmp/tp-dev/tp ./cmd/tp`), never the PATH-installed release (it lags and hides new behavior). Rebuild after every implementing commit, and once a task adds a new command/flag, immediately exercise that new capability on tp's own spec/task files to surface bugs the unit tests miss (real dogfooding). When a version's new feature can manage tp's own workflow (e.g. a new config file), adopt it for the remaining development of that same version. This applies to the **review and audit loops too**: run every `tp review` / `tp audit` (including `--record` / `--status`) through the current, to-be-released binary so each round dogfoods the exact code being shipped — never a lagging PATH release. Once a version's own feature can drive review/audit (e.g. user-defined reviewer roles), the remaining rounds of that version use it. Before implementation starts (spec + review phase) the current version is simply the latest release — no behavioral difference — but still route every tp call (`lint` / `init` / `review`) through the freshly-built binary.
- **English everywhere in committed artifacts** — commit messages, `tp commit`/`tp done` closure reasons (they land in `commit_sha` bodies and `closed_reason`), code comments, docs, **and release notes** (they live on the GitHub release, an outward-facing artifact) are ALWAYS in English, regardless of the conversation language. Author notes/thinking may be in any language, but nothing committed to the repo (or posted to its releases) may be. If a Turkish (or other non-English) message slips into history, fix it with `hc rewrite` (commit messages) or by editing the artifact (closure reasons, docs, release notes) before release.
- **Task-closing commits follow `commit_strategy`** (v0.28.0) — under `builtin`, close through `tp commit` (records `commit_shas`) or `tp done --auto-commit`; under `hc` (tp's own default, since `hc` is installed here), the agent commits with `hc` and then records the SHA(s) via `tp done --commit <sha> [--commit <sha> …]` (stored in `commit_shas`, `commit_sha` mirrors `[0]`). Never raw `git commit`; and under `hc`, `tp commit`/`tp done --auto-commit`/a bare `tp done` are rejected (exit 2) — a close needs `--commit` or `--covered-by`
- Every other commit (spec progression, docs, tooling, changes outside a task) goes through the `hc` skill (hunk-based atomic commits) — never raw `git commit`
- **Update `skills/tp/SKILL.md` before every release** — new commands, flags, lint rules, and workflow changes MUST be reflected in the skill file before creating the release tag
- **Pre-release checklist** — before running `gh release create`, verify:
  1. `skills/tp/SKILL.md` reflects all new commands, flags, lint rules, and workflow changes
  2. `CLAUDE.md` Self-Development Rules reflect any new conventions or process changes
  3. `README.md` reflects all new commands, flags, and features
  4. All three files are committed and included in the release tag
  5. **Dogfood any migration the release introduces on tp's own repo.** When a version adds a config/layout that makes an existing file or duplicated data redundant, migrate tp's own repo as part of the release: adopt the new mechanism, delete the now-redundant files, and commit the result (so tp ships already using its own new feature). For v0.24.0: write `.tp/config.json` with the shared workflow policy, remove the duplicated `workflow` blocks from every `spec/*.tasks.json`, migrate the active pointer with `tp use` (writes `.tp/local.json`, git-ignored), and delete the deprecated `.tp-active`; commit `.tp/config.json` + `.tp/.gitignore` + the thinned task files. For **v0.25.0: no repo migration** — tp kept zero role files and stayed on the embedded default corpus (§13.3), which still dogfooded the new emission/dedup/staleness paths. For **v0.26.0** (presence-preserving workflow): thin `spec/0.25.0.tasks.json`'s materialized `workflow` block to `{}` (verify `tp config --resolved` reports identical values before/after), and adopt the tuned custom 4+4 role corpus under `.tp/reviewers/` + `.tp/auditors/` — so tp now **dogfoods user-defined roles** rather than the embedded defaults.
  6. **v0.34.0 only** — refresh `spec/0.34.0-release-counts.md` at the tag (re-run its four commands with `REF=v0.34.0`, re-derive the fact table under the same rule) and paste that section into the release notes. §2.3 of `spec/0.34.0.md` asks for the counts *in the release notes*; the file is where they are derived, not where they are published.
- **Post-release commands** — after `gh release create`, run these two commands (in order):
  ```bash
  go install github.com/deligoez/tp/cmd/tp@v<VERSION>
  npx skills update -g   # updates every installed global skill source, including deligoez/tp
  ```
  Use the exact version tag (e.g., `v0.17.0`), not `@latest` — the Go module proxy and skill registry may not update immediately.

### Reset-native self-development via subagent-per-unit (v0.28.0+)

Prefer running each **unit** in a **fresh subagent context** (Agent/Task tool), not inline in the orchestrator: one implementation task, or one review/audit round's per-role reviewers/auditors. The subagent's work reaches disk (commit, `tp done`, `.tp-review` record); the orchestrator re-orients from durable state (`tp resume`/`tp next`) between units, so nothing load-bearing lives only in a context window. (Verified: a fresh subagent implemented, gated, committed, and closed a task end-to-end from injected context alone.)

**Brief the unit, don't retype its context (v0.30.0).** `skills/tp/SKILL.md` describes what the brief carries and why the orchestrator produces it rather than remembering it. In this repo the unit's first call is `/tmp/tp-dev/tp next --brief`, and the orchestrator injects only what tp cannot know: native Read/Edit/Write/grep are hook-blocked → use codedbpro (same-file range/insert edits apply in **list order**); run the quality gate yourself before `tp done`; the `TP_HC` env seam gives tests a deterministic strategy.

**Honest boundaries.** Subagents don't nest (one level), so the orchestrator does each round's fan-out itself; the orchestrator's own context is NOT reset in this model — only the units are. For a full reset of the driver too, use the `/clear` + `tp resume` loop (human/harness-triggered, since an agent can't clear its own caller's context — §2.1) or drive tp externally with headless `claude -p` per unit.

**Budget the subagents before starting a long run (v0.32.0 lesson).** Claude Code caps subagents per *session* (`CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION`, 200 by default) and the cost is `rounds × roles`: a 4-role audit is 4 per round, a 5-role review 5 per round, so 23 rounds alone is ~100. v0.32.0 exhausted the cap at audit round 11 and the last repairs had to be done by the orchestrator itself — which still works, but trades away the independence that makes the loop worth running. Estimate `rounds × roles + tasks + repairs` up front; if it approaches the cap, either raise it or plan a `/clear` + `tp resume` handoff to a fresh session at a phase boundary.

### Continuous Improvement
- After each implementation cycle, note friction points and AX issues
- If a tp command is awkward to use during self-development, fix it immediately
- If a workflow step is error-prone, add tooling or guidance to prevent it
- Agent feedback from other projects is high-priority — real-world usage reveals blind spots
- Every improvement should be evaluated: does this reduce token overhead or agent friction?

### Deferred Ideas (evaluate when agent feedback warrants)
- 📋 **Next version's candidates live in `spec/0.35.0-candidates.md`** — read it before writing a new spec. It carries what the v0.34.0 cycle accepted with recorded justification because its own Non-Goals fenced it out, rather than judged harmless.
- ✅ **Full audit NDJSON parser** (`tp audit --merge`) — **shipped.** Merges + dedups per-role audit-result files (by `role`+`item_id`) with a status/role breakdown, mirroring `tp review --merge`; `--record` still counts non-PASS rows for convergence.
- ✅ **Broken cross-reference lint** (`broken-cross-ref`) — **shipped.** Flags `§X.Y step N` when section X.Y has fewer than N numbered steps. Kept conservative to hold the false-positive rate down: fires only when the section is a heading whose content holds a numbered list and N exceeds the largest such list (sized by both item count and highest literal number, so `1. 1. 1.` markdown numbering counts correctly); refs into listless or unknown sections, and refs inside code blocks, are never reported. Zero false positives across tp's own specs.
- ✅ **Duplicate paragraph lint** (`duplicate-paragraph`) — **shipped.** Flags two consecutive identical blank-line-separated paragraphs (a copy-paste artifact `duplicate-line` misses); a code block between two blocks breaks their adjacency, and single-line heading or horizontal-rule paragraphs are skipped to avoid double-reporting.
- ✅ **Project-level workflow config** (`.tp/config.json`) — **shipped in v0.24.0.** Repo-root `.tp/config.json` holds workflow **defaults** (committed); each `<base>.tasks.json` `workflow` block holds only explicit **overrides**; effective values **resolve at read time** (CLI > env > task override > project config > built-in). `.tp/local.json` (git-ignored) holds the `active` pointer + CLI flag `defaults`. See "Project Configuration" in `skills/tp/REFERENCE.md`.

## Tech Stack

- **Language:** Go
- **CLI:** spf13/cobra
- **Colors:** fatih/color (NO_COLOR + TTY detection built-in)
- **File locking:** gofrs/flock
- **Testing:** stretchr/testify
- **JSON:** encoding/json (stdlib)
- **Validation:** Manual struct validation (no JSON Schema library)

## Conventions

tp's own behavior — exit codes, managed fields, workflow field ranges, `--compact` semantics,
acceptance delimiters, field aliases, discovery order — is documented once, in
`skills/tp/SKILL.md` and `skills/tp/REFERENCE.md`; the lint rule table lives in `README.md`. What
follows is only what is a convention of *this codebase*:

- JSON output when piped or `--json`, colored text in TTY
- Pretty-printed JSON with 2-space indentation
- All write operations use flock; reads are lock-free
- Task status is stored as `open -> wip -> done` (three states only; blocked is computed from deps)
- `unused`, `unparam`, `gocritic` and `dupl` are enabled, so an orphaned symbol, an unused parameter, a range that copies a large struct, or a clone at or above `dupl`'s default 150-token threshold each fail the gate

## Manual QA Testing

When running manual QA, use this setup to avoid wasting time:

```bash
# 1. Build binary to temp dir
mkdir -p /tmp/tp-qa/project
go build -ldflags="-s -w" -o /tmp/tp-qa/tp ./cmd/tp
export TP=/tmp/tp-qa/tp
cd /tmp/tp-qa/project

# 2. Create a spec with multiple headings (for lint, coverage, excerpt tests)
cat > spec.md << 'SPEC'
# Todo App
## 1. Models
### 1.1 Task Model
Create a Task model with title, status, due_date fields.
### 1.2 User Model
Create a User model with email and password fields.
## 2. API
### 2.1 Create Task
POST /tasks endpoint that creates a new task.
### 2.2 List Tasks
GET /tasks endpoint that returns all tasks.
## 3. Testing
### 3.1 Unit Tests
Unit tests for Task and User models.
SPEC

# 3. Init + add tasks with dependency chain
$TP init spec.md
$TP add '{"id":"task-model","title":"Create Task model","estimate_minutes":5,"acceptance":"Task model exists with migration.","source_sections":["### 1.1 Task Model"],"source_lines":"4-4","depends_on":[]}'
$TP add '{"id":"user-model","title":"Create User model","estimate_minutes":5,"acceptance":"User model exists.","source_sections":["### 1.2 User Model"],"depends_on":[]}'
$TP add '{"id":"create-api","title":"Create task endpoint","estimate_minutes":8,"acceptance":"POST /tasks creates task.","source_sections":["### 2.1 Create Task"],"depends_on":["task-model"]}'
$TP add '{"id":"list-api","title":"List tasks endpoint","estimate_minutes":5,"acceptance":"GET /tasks returns all.","source_sections":["### 2.2 List Tasks"],"depends_on":["task-model"]}'
$TP add '{"id":"tests","title":"Write unit tests","estimate_minutes":8,"acceptance":"Task and User model tests pass.","source_sections":["### 3.1 Unit Tests"],"depends_on":["task-model","user-model"]}'

# Setup ready — 5 tasks, 2 ready (task-model, user-model), 3 blocked
```

This spec passes both `tp lint` (the container headings `## 1`/`## 2`/`## 3` no longer fire `empty-section` since v0.29.0) and `tp validate` (exit 0) on a clean run.

### QA Test Checklist

**All output is JSON when piped. Use `| python3 -c "import sys,json; ..."` to parse.**

**Note:** `tp add` prints `{"added":["<id>"]}` on success; check the exit code on failure.

| Area | Commands to test | What to verify |
|------|-----------------|----------------|
| **Basics** | `--version`, `--help`, `lint spec.md` | Version shows, help lists all commands, lint runs |
| **Status/Query** | `status`, `ready`, `ready --first`, `blocked`, `show <id>`, `list`, `list --status open`, `list --tag`, `list --ids`, `list --compact` | Correct counts, correct filtering, compact strips fields |
| **Plan** | `plan`, `plan --compact`, `plan --from <id>`, `plan --level 0` | Topo order, WIP first, excerpt present, compact strips excerpt |
| **Next** | `next` (claim), `next` again (WIP resume), `next --peek` | Same task returned twice, peek doesn't claim |
| **Done single** | `done <id> "reason"`, `done <id> "reason" --gate-passed --commit sha` | has_next correct, gate_passed_at/commit_sha set |
| **Done batch** | Write NDJSON, `done --batch file` | closed/failed counts, partial failure works |
| **Claim batch** | `claim id1 id2`, `claim --all-ready` | claimed array, failures with hint |
| **Close (low-level)** | `close <id> "reason"` on open task (should fail with hint) | Error mentions "use tp done" |
| **Reopen** | `reopen <id>`, then `show` | status=open, closed_at/gate_passed_at/commit_sha all null |
| **Remove** | `remove <id>` with dependents (should fail), `remove --force` | Force cleans deps to `[]` not null |
| **Set** | `set <id> estimate_minutes=3`, `set <id> status=done` (should fail) | Field updated, managed field rejected with hint |
| **Validate** | `validate`, `validate --strict` (add task with estimate>15) | Strict makes atomicity violations into errors |
| **Graph** | `graph`, `graph --tag`, `graph --from` | JSON adjacency list when piped |
| **Error cases** | Done already-done, done with "deferred", done single word, claim blocked | Correct exit codes (1/2/4), actionable hints |
| **Nil slices** | `show` on task with no dependents, `ready` when all done, `blocked` when none blocked | `[]` not `null` in JSON |
| **Import** | Create full task file JSON, `import file.json` | Status shows imported tasks |
| **Excerpt** | Add task with source_sections (no source_lines), check `plan` output | spec_excerpt carries the section text (v0.29.0) |

### Common nil-slice pattern to watch for

Any `var x []T` that reaches `output.JSON()` will serialize as `null` when empty. Always use `x := make([]T, 0)` for JSON-output slices. Grep: `grep -rn 'var .* \[\]' internal/ --include='*.go' | grep -v _test.go`
