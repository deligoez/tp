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
go test -race ./... && golangci-lint run && ./scripts/check-deadcode.sh && ./scripts/check-complexity.sh

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
- Quality gate after every task: `go test -race ./... && golangci-lint run && ./scripts/check-deadcode.sh && ./scripts/check-complexity.sh` — this is the project gate in `.tp/config.json`, and no task file carries a `quality_gate` override that would mask it. tp's `.golangci.yml` enables the `gofmt` formatter — `skills/tp/SKILL.md` says why that section is required. The third step needs `go install golang.org/x/tools/cmd/deadcode@latest`, the fourth `go install github.com/uudashr/gocognit/cmd/gocognit@latest`. **The gate and CI are the same list, and a test enforces it**: `TestCIRunsEveryStepOfTheProjectGate` splits `.tp/config.json`'s `quality_gate` on `&&` and requires every step to appear in `.github/workflows/ci.yml`, because CI had drifted to `go test ./...` — no race detector, no deadcode — while the local gate ran four steps, so a green badge meant less than it looked. Adding a step to the gate now fails two guard tests until CI and the resolved-gate literal in `internal/cli/quality_gate_config_test.go` are updated with it; that is the mechanism working, not an obstacle.
- **Complexity is a ratchet, not a threshold.** `scripts/check-complexity.sh` runs `gocognit` over production code (`_test.go` excluded) at `TP_COGNIT_THRESHOLD` (22) and compares the violations against `scripts/baseline-complexity.txt`, keyed on `package function` rather than line so the baseline survives edits elsewhere in the file. It fails on a **new** violation and equally on a **stale** baseline entry that no longer violates — so the list can only shrink, and a function that gets simplified cannot silently re-inflate later. Why a ratchet: the codebase had **54** production functions over the threshold when this was added, so a plain `gocognit` linter entry would have to fail all 54 on day one or sit at a threshold that measures nothing. Two rejected alternatives, recorded so they are not re-proposed: `golangci-lint --new-from-rev` weakens *every* linter on unchanged code, not just this one, and per-file exclusions were unusable because the 54 functions are spread across 38 files.
- **`unused` cannot see an exported symbol, so the gate runs `deadcode` too.** golangci-lint's `unused` skips exported identifiers by design; `deadcode` builds a call graph from every main package and test, so a function nothing reaches is reported whatever its case. It found three uncalled exported accessors in `root.go` that eight audit rounds and four roles had not, and — more usefully — it separates *dead* from *called only by tests*, which is how the audit-category sink gap surfaced: `IsValidCategory` had been written, tested, and never wired, while tp told every auditor that unknown categories are rejected. When a function shows up as test-only, ask whether the production caller is missing rather than whether the test is.
- **Run the tools before the review rounds, not after.** `deadcode`, `go fix`, `govulncheck` and mutation testing each found something in this codebase that a 17-round review and an 8-round audit had passed over, in under a minute apiece. The reason is structural rather than a failure of the loop: **review reads what is written, and these read what is reachable.** A validator that exists, is tested, and is never called reads perfectly well on the page; only a call graph notices. Every defect they catch first is a defect no reviewer spends a round on. `scripts/check-deadcode.sh` is in the gate; the rest are on triggers — `go fix ./...` after a large implementation push and before a release, `govulncheck ./...` before a release, mutation testing **at least once before any release** (see below). **`go fix`'s own output is not evidence on Go 1.26**: at v0.35.0 it reported "applied 6 of 7 fixes; 5 files updated" on every run while changing **zero bytes** — confirmed by running it in an `rsync` copy and diffing against the original, which is the only way to tell an applied fix from a claimed one. Check `git status`, not the summary line.
- **Mutation testing: cheap to run, expensive to read, so run it at decision points rather than in the gate.** `gremlins unleash ./internal/engine` (note the path form — gremlins appends `/...` itself) took **53 minutes for 1540 mutants** at v0.35.0 — budget an hour, not the five minutes this line used to claim; the engine package roughly doubled and the old figure sent two cycles into it expecting a coffee break. Interpretation is the cost: v0.34.1's ten survivors in one file were two equivalent mutants, three on an undocumented backoff schedule, and four on a documented contract with no boundary test — only the last group was worth acting on. **Do not treat the survivor count as a score to drive down**; classify them and say which you are deliberately leaving and why. Two rules earned the hard way: a run is **load-sensitive** (a second run on a busy machine turned 75 kills into 80 timeouts — a run full of `TIMED OUT` is not a result), and a **behavioural test can verify a rule and still miss its boundary** — v0.34.0's audit checked a documented 1–60 range with the value 999, which passes whether the bound is inclusive or exclusive. Run it in an `rsync` copy, never the repo. What it is worth: the files v0.34.0's manual sweep examined score 96–100% efficacy, the files it did not sit at 25–78%, so the manual work was real — and `internal/engine/validate.go` at **25% with 46 survivors** is the standing backlog. The two decision points, named here so they are not re-derived each cycle and do not die with the orchestrator's context: **when a version's new engine surface is complete, before its audit loop opens**, and **before the release tag** — never while task gates are running, for the load reason above. `spec/0.42.0.md` §8 is where this becomes a documented gate entry instead of a rule in this file.
- **Prove a fix by running it, not by reading it — write the test first and watch it fail.** A test
  that has never been observed failing proves nothing about the code; it may be asserting a
  tautology, and it passes identically either way. Write the assertion, run it against the unfixed
  code, confirm it fails *for the reason you expect*, then fix. The same rule at the sink: an
  auditor that keyword-searches for a term sees a `PASS` line and cannot see the gate discarding the
  exit status it was meant to report — a real field report, where round 1 returned 75 PASS while
  three of that project's critical gates were printing `PASS` on error. Reading confirms; only
  running can refute, so **the informative experiment is the one that could have failed**: inject the
  failure the code claims to catch, feed the boundary value, break the precondition. When a
  sink-level check cannot discriminate — the input never contained the case — say so rather than
  counting it as evidence. `spec/0.42.0.md` turns this into tp's evidence contract; until it ships it
  is a rule here.
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
  4b. **Bump `.claude-plugin/plugin.json`'s `version` BEFORE tagging, every release.** Claude Code resolves a plugin's version from that field first, and a set field *pins* it: users are offered an update only when the field changes, however many commits or tags ship. Anthropic's documented remedy — omit the field and let git tags drive it — is closed to tp, because `hooks/session-start.sh` reads the same field as the minimum tp version (§6.1). The field is load-bearing twice, so the bump is mandatory. `TestPluginVersionIsNotBehindTheLatestTag` fails the gate when a tag has overtaken the manifest, and the order is bump → commit → tag (the `>=` comparison exists so that window is legal). Bumping it also raises the minimum tp version, which is why the release must ship a binary at or above it.
  5. **Dogfood any migration the release introduces on tp's own repo.** When a version adds a config/layout that makes an existing file or duplicated data redundant, migrate tp's own repo as part of the release: adopt the new mechanism, delete the now-redundant files, and commit the result (so tp ships already using its own new feature). For v0.24.0: write `.tp/config.json` with the shared workflow policy, remove the duplicated `workflow` blocks from every `spec/*.tasks.json`, migrate the active pointer with `tp use` (writes `.tp/local.json`, git-ignored), and delete the deprecated `.tp-active`; commit `.tp/config.json` + `.tp/.gitignore` + the thinned task files. For **v0.25.0: no repo migration** — tp kept zero role files and stayed on the embedded default corpus (§13.3), which still dogfooded the new emission/dedup/staleness paths. For **v0.26.0** (presence-preserving workflow): thin `spec/0.25.0.tasks.json`'s materialized `workflow` block to `{}` (verify `tp config --resolved` reports identical values before/after), and adopt the tuned custom 4+4 role corpus under `.tp/reviewers/` + `.tp/auditors/` — so tp now **dogfoods user-defined roles** rather than the embedded defaults.
  6. **v0.34.0 only** — refresh `spec/0.34.0-release-counts.md` at the tag (re-run its four commands with `REF=v0.34.0`, re-derive the fact table under the same rule) and paste that section into the release notes. §2.3 of `spec/0.34.0.md` asks for the counts *in the release notes*; the file is where they are derived, not where they are published.
- **Post-release commands** — after `gh release create`, run these (in order):
  ```bash
  go install github.com/deligoez/tp/cmd/tp@v<VERSION>
  claude plugin update tp@tp   # Claude Code users: the plugin carries the hooks and agents
  npx skills update -g         # other agents: updates every installed global skill source
  ```
  Use the exact version tag (e.g., `v0.17.0`), not `@latest` — the Go module proxy and skill registry may not update immediately. Then verify by running rather than assuming: `claude plugin details tp@tp` must report the new version and the full inventory (Skills 1 / Agents 3 / Hooks 3). **A plugin that installs is not a plugin that loads** — v0.35.0 shipped a manifest that installed cleanly and then failed to load, so `claude plugin validate` passing is not evidence; only `install` + `details` is.
- **Two distribution channels, one source.** `skills/tp/` is the only copy of the skill in the repo: the plugin finds it by convention (`strict: false`, no component keys in the marketplace entry) and the `npx skills` package publishes the same directory. So the skill costs nothing extra to keep and reaches the 30+ agents that implement the Agent Skills standard, while the plugin adds the hooks and agents that only Claude Code can run. Ship both; install exactly one per machine, because a plugin skill and a standalone skill of the same name both load rather than one overriding the other.

### Reset-native self-development: `tp run` first, subagent-per-unit as the fallback (v0.35.0)

**The primary model is `tp run`.** tp ships the driver now, so the default way to advance a cycle here is `/tmp/tp-dev/tp run` — it reads the cycle through the same path `tp resume` uses, spawns one runner process per unit (role siblings concurrently, everything else alone), re-reads the state from disk, checks its caps, and loops until the oracle says `release`. Exit **0** means `stop_reason: converged` and exit **4** names one of the other eight stop reasons, so the orchestrator branches on the exit code rather than on the transcript. `tp run --dry-run` shows the batch it would spawn without spawning it or taking the lock, and `tp run --status` reports the run in flight or the last one that stopped. The five `run_max_*` caps, `runner` and `notify_cmd` configure it; the eight unit kinds, nine stop reasons, run state, child environment and the plugin's hooks are in `skills/tp/REFERENCE.md`.

**Under a run the policy is enforced, not merely stated.** Every child gets `TP_UNATTENDED=1`, and the four user-approved decisions this file has always reserved for the operator — `--skip-gate` at any sink, `tp import --force`, a raise of `review_max_rounds`/`audit_max_rounds`, a raise of any `run_max_*` cap — exit 2 under it. A unit that reaches one records it with `tp escalate --decision <name> --evidence <text> [--option <text>]…`, the run stops with `stop_reason: escalation`, and **the operator answers and starts `tp run` again**. That is the same rule as before; it is now a fence rather than an instruction.

**Budget a run before starting it.** The caps are the budget: `run_max_units` (default 100), `run_max_wall_clock_seconds` (28800) and `run_max_budget_usd` (0 = disabled). Caps are checked **between iterations**, so a run can overshoot wall-clock and budget by at most one iteration. A cap stop is a report to a human, never an acceptance — it never records a round or marks a phase converged.

**Interactive fallback — subagent-per-unit (v0.28.0+).** When a run is not appropriate — no runner configured, a unit that wants a human in the loop, or a one-off repair — run each **unit** in a **fresh subagent context** (Agent/Task tool), not inline in the orchestrator: one implementation task, or one review/audit round's per-role reviewers/auditors. (Verified: a fresh subagent implemented, gated, committed, and closed a task end-to-end from injected context alone.)

**Brief the unit, don't retype its context (v0.30.0).** `skills/tp/SKILL.md` describes what the brief carries and why the orchestrator produces it rather than remembering it. In this repo the unit's first call is `/tmp/tp-dev/tp next --brief`, and the orchestrator injects only what tp cannot know: native Read/Edit/Write/grep are hook-blocked → use codedbpro (same-file range/insert edits apply in **list order**); run the quality gate yourself before `tp done`; the `TP_HC` env seam gives tests a deterministic strategy.

**Honest boundaries.** The orchestrator's own context is NOT reset in this model — only the units are.

**Budget the subagents before starting a long run (v0.32.0 lesson).** Claude Code caps subagents per *session* (`CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION`, 200 by default) and the cost is `rounds × roles`: a 4-role audit is 4 per round, a 5-role review 5 per round, so 23 rounds alone is ~100. v0.32.0 exhausted the cap at audit round 11 and the last repairs had to be done by the orchestrator itself — which still works, but trades away the independence that makes the loop worth running. Estimate `rounds × roles + tasks + repairs` up front; if it approaches the cap, either raise it or plan a `/clear` + `tp resume` handoff to a fresh session at a phase boundary.

### Continuous Improvement
- After each implementation cycle, note friction points and AX issues
- If a tp command is awkward to use during self-development, fix it immediately
- If a workflow step is error-prone, add tooling or guidance to prevent it
- Agent feedback from other projects is high-priority — real-world usage reveals blind spots
- Every improvement should be evaluated: does this reduce token overhead or agent friction?

### Where the next work is (read this before starting anything)

**`spec/0.35.0.md`** (the unattended runner, `tp run`) is **complete and audit-converged** — 21
review rounds, 45/45 tasks done, 9 audit rounds with the last two clean, and both
`tp review spec/0.35.0.md --status --check` and `tp audit spec/0.35.0.md --status --check` exit 0.
`tp resume` reports `phase: release`. Everything before the tag is done; the release itself is the
operator's.

Two things that cycle deliberately did NOT fix, both recorded with reasons in
`spec/0.35.0-candidates.md` — read §16 (the mutation run) and §17 (the two degraded-scan repairs
choosing different channels) before opening the next release, so neither is re-derived from scratch.

**Seven releases are planned, and the order below is by measured value over size, not by authoring
order.** Three specs were renumbered to match it; a spec filename is the release that ships it, and
that convention is kept. None of the seven has been reviewed.

| # | Spec | What it does | Size |
|---|---|---|---|
| 0.36.0 | `spec/0.36.0.md` — the emitted round | The emitted prompt carries its own isolation and incremental-write constraints, and `--out-dir` gives the prompts somewhere to go | XS |
| 0.37.0 | `spec/0.37.0.md` — audit convergence | `audit_converge_on` (severity-aware, the twin of `review_converge_on`), accepted rows stop blocking, a changed spec ends the streak | S |
| 0.38.0 | `spec/0.38.0.md` — the backlog release | The defects v0.32.0–v0.35.0 deferred with reasons; cites `spec/0.35.0-candidates.md` rather than restating it | M |
| 0.39.0 | `spec/0.39.0.md` — the rules become checks | `quality_gate` as an ordered array; the binary advisory. Was `0.37.0.md` | S–M |
| 0.40.0 | `spec/0.40.0.md` — what the loop costs | Repair locality and class families, measured from data tp already stores. Was `0.36.0.md` | M |
| 0.41.0 | `spec/0.41.0.md` — the divisible round | Sharding a role's checklist, incremental rounds, a durable home for an accepted finding | L |
| 0.42.0 | `spec/0.42.0.md` — the evidence contract | Stop accepting an assertion where an experiment was meant. Was `0.38.0.md`. **Pre-design** — §4–§7 and §10 name no mechanism yet | ? |

**Why this order.** 0.36.0 is first because it is the smallest and its cost is paid by every cycle
after it — two cycles measured the same three manual additions per round, and losing a subagent
mid-round loses the round. 0.37.0 is second on this repo's own numbers: across v0.35.0's nine audit
rounds the non-`PASS` rows were 3 `error`, 22 `warning`, 17 `info`, so a severity-aware policy would
have closed the phase several rounds earlier. 0.38.0 is third because its items are live defects with
no design risk. 0.41.0 follows 0.40.0 deliberately: sharding is the expensive answer to a question
0.40.0's measurement should be allowed to ask first. 0.42.0 is last by readiness, not by preference —
it cannot be decomposed at all until its design pass.

**One earlier claim was checked and dropped.** This file used to say v0.36.0 must go first because it
measures the loop the other rules are validated in. The spec text does not support it — the
loop-costs spec makes no reference to the others. The ordering above rests on measured size and value
instead.

**Field feedback drives three of the seven.** `spec/0.42.0.md` came from a Rust NLP project; three of
their seven reports are answered there (audit evidence is a keyword search, no `UNVERIFIED` verdict,
closure evidence unchecked for kind); `spec/0.40.0.md` §6.1 answers the reconcile request and its
Non-Goals 7-8 record what was deliberately not taken. A second cycle, on a PHP package, produced
eleven findings that became `spec/0.36.0.md`, `spec/0.37.0.md` and `spec/0.41.0.md` — its measured
round times (44 min for one agent, unchanged when a role was dropped) are the whole argument for
sharding. Two more reports did
not survive verification against tp's own source and are worth remembering as a class: *diagnostics
poison `--json`* (false — `output.Notice` has written to stderr since v0.31.2; their runtime merges
the child's streams) and *`--record` accepts a missing file* (false — `review_record.go` exits
`ExitFile`). **Field feedback arrives with the reporter's environment baked in, so verify each claim
against the source before routing it to a spec.**

**Findings are already routed, so do not re-derive them.** `spec/0.35.0.md` §8a carries the five
things an unattended driver amplifies (an inflated convergence count from `--merge`'s dedup key,
`next_action` recommending a registration the phase cannot honour, a truncated audit reporting as
complete, a dropped role merging clean, exit codes that cannot separate a typo from a failure).
`spec/0.40.0.md` §2 carries the repair-verbosity rule with the measurements from v0.35.0's own loop —
43% of that cycle's findings sat in text the previous round had just written, and one idea arrived
under 25 distinct class slugs. `spec/0.35.0-candidates.md` §0 names what v0.34.1 and v0.34.2 already
closed and what moved to a spec — read it first so nothing gets re-opened or claimed twice.

**v0.36.0's own hazard is suppression, and its §8 is a release gate rather than a test appendix.**
Every mechanism in it hides, regroups or narrows what reviewers see, which is exactly what burned
v0.34.0 §7.1 for eight rounds. Nothing there ships until it is replayed against the recorded rounds
in `spec/.tp-review/` and shown not to lose a finding tp used to surface.

`spec/feedback.md` is gone: every finding it held now has an owner. Field feedback should land in the
spec that will answer it, not in a file nobody is required to read.

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
