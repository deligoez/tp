---
name: tp
description: Spec-to-task lifecycle manager for AI coding agents. Interviews user to resolve ambiguities, decomposes specs into atomic tasks, manages execution order via dependency graph, and batch-closes with evidence. Use when user wants to implement a spec, plan tasks, decompose a feature, a *.tasks.json file exists, or user says 'continue with tp' to resume work after a context clear.
---

# tp — Task Plan Skill

Activates when: a `.tasks.json` file exists, user asks to implement a spec/plan/tasks, or user references tp commands.

> **English only — no foreign-language trigger seeding.** This skill is written in English. Trigger matching is semantic, so a user typing the resume intent (or any other trigger) in any language activates the skill. Do not seed literal foreign-language phrases — they add no value and violate the repo's English-only convention for committed artifacts.

## "continue with tp" — zero-friction resume (any project)

A short resume intent — "continue with tp", "resume tp" — is a sufficient standalone prompt in ANY tp project after a context clear. `tp resume` rebuilds the full picture from durable state (task file, spec, `.tp-review/`, `.tp/local.json`, git); nothing load-bearing lives in the context window. On the trigger:

1. **Orient**: `tp resume` → `{phase, blockers, next_action}`. (PATH `tp`; in tp's own self-development use the freshly-built dogfood binary instead.)
2. **Clear agent-clearable blockers**: `unexplained-changes` → commit the changes via the project's commit tool, or `tp keep <path> "<reason>"` for intentionally-uncommitted files; then re-run `tp resume`. Stop and report human-only blockers (`*-budget-exhausted`, `no-ready-task`).
3. **Work ONLY the reported phase** — do not run the next phase in the same session:
   - **review** → Workflow A Step 2 (review loop until `tp review <spec> --status --check` exits 0; `wontfix`/`duplicate` are terminal and may be set pre-record, `fixed` implies a spec change and forces a re-review).
   - **decompose** → Workflow A Step 3 (atomic tasks: 1 verb, ≤8-word title, ≤3 acceptance, `source_sections` for EVERY task; backward-pass every table row + numbered list item into some task's acceptance; `tp validate` until clean; `tp import tasks.json` plain, no `--force`).
   - **implement** → Workflow B/C (one fresh subagent PER task, injecting a durable-state pointer `tp next`, the close recipe for the effective `commit_strategy`, and live lessons; close per `commit_strategy`: `hc` → `hc run` then `tp done <id> --commit <sha>`; `builtin` → `tp commit`/`tp done --auto-commit`).
   - **audit** → Workflow D (until `tp audit <spec> --status --check` exits 0; ALWAYS 2 consecutive clean rounds, never cut short by a cap).
   - **release** → report and stop.
4. **Stop at the phase boundary**: print (1) what changed on disk, (2) the next `tp resume` next_action. The human clears context and re-sends the trigger for the next phase.

This is the reset-native contract: `tp resume` is the single source of truth between context resets.

## Workflow A: Decompose (spec exists, no .tasks.json)

Order (v0.23.0): **interview → `tp lint` → `tp init` → `tp set --workflow` → review loop → decompose → `tp import`**.

Running `tp init` **before** the review loop creates the spec-adjacent task file that supplies the loop's workflow parameters (convergence counts, round budgets, checks). The quality gate is authored once at the project layer with `tp set --workflow --project quality_gate="<cmd>"` (writes `.tp/config.json`), not with `tp init --quality-gate`: a task-file override masks the project gate, so a later change to the project gate never reaches the task files carrying one. The task-level setter stays read-only (`tp set --workflow quality_gate=…` exits 2).

### Step 0: Interview

Before writing or editing a spec, resolve all ambiguities:

1. **Locate material** — read draft spec (if provided) or ask user to describe the problem.
2. **Explore codebase** — read CLAUDE.md/README and affected files. Limit to files directly referenced.
3. **Identify ambiguities** — list all unclear, under-specified aspects.
4. **Ask one at a time** — for each ambiguity, ask one question. Derive follow-ups from answers.
5. **Prefer codebase** — if answerable by reading code, explore (≤5 files) instead of asking. Architectural/product decisions always go to user.
6. **Recommend answers** — provide a recommended answer for each question based on codebase context.
7. **Handle non-answers** — if user says "skip"/"whatever"/empty, accept recommended answer.
8. **Termination** — complete when: (a) every behavioral claim is verified or confirmed, (b) every design choice with user-visible impact (CLI output, file format, command behavior) is decided, (c) no new questions arise.

Then collect workflow parameters (hold in memory until `tp init` / `tp set --workflow`):
- Quality gate command (e.g. `"go test -race ./... && golangci-lint run"`) — authored at the project layer in Step 1. For a Go project, run the suite under `-race`: the race detector is off by default, so a data race can never fail a gate without it, and on a suite this size it costs between a tenth and a half again of the wall time. Note: `golangci-lint run` (v2) checks formatters like `gofmt` only when a `formatters:` section enables them, so enable one in `.golangci.yml` (or add `gofmt -l .` to the gate) or a gofmt-dirty file slips through.
- Consecutive clean **review** rounds (default 2) — integer 1-10; re-ask once if invalid, then use default.
- Consecutive clean **audit** rounds (default 2) — same rules.
- Optional round budgets `review_max_rounds` / `audit_max_rounds` (default 0 = no cap) — a hard ceiling on counted rounds before escalation.

Announce: "I will review until N clean rounds, audit until M clean rounds." If new ambiguities arise during spec writing, pause and return to step 3. Do not re-ask parameters.

### Step 1: Init the task file and workflow

1. `tp lint <spec.md>` — fix issues; review `structured_elements` and the `frontmatter` object.
2. `tp init <spec.md>` — creates the spec-adjacent `<base>.tasks.json` shell (zero tasks) with an empty `workflow` block. Author the gate one layer up: `tp set --workflow --project quality_gate="<cmd>"` (writes `.tp/config.json`, resolved by every task file); passing `--quality-gate` here writes a task-file override that masks it.
3. `tp set --workflow review_clean_rounds=N audit_clean_rounds=M` — convergence counts (only if non-default).
4. `tp set --workflow review_max_rounds=R audit_max_rounds=A` — round budgets (only if capping).
5. `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'` — register mechanical checks (see Class & Checks Guidance).

**Multi-spec repos:** put the shared gate/convergence policy in a repo-root `.tp/config.json` once (see [Project configuration](#project-configuration-tpconfigjson)) and leave each `tp init` shell's `workflow` block empty except where a spec genuinely deviates — that keeps one source of truth instead of copying policy into every `<base>.tasks.json`.

### Step 2: Review loop (explicit recipe)

Repeat until `tp review <spec> --status --check` exits 0:

1. `tp review <spec>` — tp auto-numbers the round (R = recorded rounds + 1), snapshots the spec, and injects previous findings + the changed-sections diff into every role prompt. A 4th **regression** prompt is auto-appended from round 2 when the spec changed or fixed findings exist — process it first.
2. Spawn one sub-agent per prompt; collect NDJSON findings.
3. `tp review --merge r1.ndjson ... -o merged.ndjson` — dedup across prompts. An all-empty (converged) input set still merges cleanly: empty inputs exit 0 and write a zero-byte `-o` file, so the merge → record chain works unchanged on a clean round (no manual file creation). The payload's `inputs` array reports `{path, parsed, skipped}` per input file, and an input with content lines of which **none** parsed exits **1** (v0.35.0) — that is a role whose whole file was mis-emitted, not a clean round, and recording it would freeze an undercounted round.
4. `tp review <spec> --record merged.ndjson` — record the round. Under the default `review_converge_on=blocking` a round is **clean** when no surviving finding is **critical or high** (medium/low are surfaced as `nonblocking_open`, not gated); `review_converge_on=all` keeps the strict any-severity rule. `--record` returns a `next_action` (the single next step); add `--harness-note "<text>"` to record, on this round, any standing framing the orchestrator's wrapper carried (see "Where judgement-shaping text belongs").
5. Fix the spec; mark each addressed finding with `tp review --resolve merged.ndjson <idx> fixed "evidence"` — indices are **0-based** (read them from `--merge ... -o` output or `--status`; a non-numeric index exits 2).
   - **Dispose of MANY findings in one call** with `tp review --resolve-all merged.ndjson <fixed|wontfix|duplicate> "evidence"` (evidence optional; add `--force` to also re-resolve already-resolved findings). This is how you **accept all surviving non-blocking findings as `wontfix` under one shared justification** once no critical/high remains — the severity-aware convergence permits accepting low/medium with recorded justification (see Gate, Budget & Escalation Policy).
6. When a fix batch touched **more than 3 sections**, run the standalone regression delta pass (`tp review <spec> --perspective regression`) as an uncounted check before the next counted round.
7. Repeat. `tp review <spec> --status` shows `consecutive_clean`, `converged`, `stale`, `budget_exhausted`, plus `max_rounds`/`rounds_remaining` (null when uncapped), `in_flight_round` (a snapshot with no recorded round file — `tp resume` then points at `record-round` to complete it), `next_action` (the single next step), `nonblocking_open` (count of accepted-open medium/low findings, only on a clean round that has them), and `harness_stale` (+ the latest `harness_note` when the wrapper framing changed between the last two recorded rounds). Prompt emission also reports `skipped_roles` (`[{role, reason}]`) naming every corpus role it did not emit.

**Convergence is a recorded fact, not a judgment.** Do not skip rounds, summarize findings as "minor", or declare convergence before `--status --check` exits 0. Counted rounds are always full-panel; the regression delta pass and the tail class-sweep (below) are uncounted.

### Step 3: Decompose and import

1. Decompose into tasks — **you are the decomposer, tp validates your output.**
2. Backward pass — every table row and numbered list item → some task's acceptance; `tp validate` for line coverage.
3. `tp import tasks.json` — **plain, no `--force`.** The init shell holds zero tasks, so the overwrite needs no `--force` and the §9.1 convergence checks stay armed (an unconverged or stale spec blocks the import with exit 1). Reserve `--force` for overwriting a file that already has real tasks — and only with explicit user approval.

### Decomposition Rules

**You are the decomposer — tp validates your output.**

1. **Atomicity**: Each task = 1 commit, 1 verb, 1-15 min estimated
   - ≤3 acceptance criteria, ≤8 word title (no conjunctions), ≤2 source_sections
   - If >3 criteria, split by concern axis
2. **Concern axes** for splitting: types/models → logic/engine → validation → CLI/wiring → tests → docs
3. **Structured elements** (from `tp lint`): every table row, numbered list item, code block → some task's acceptance
4. **Example tables are the recommended acceptance shape for behavioural tasks.** State behavioural acceptance as `input → expected output` rows, taking the values from a table the spec already carries — `tp lint` extracts spec tables, rule 3 maps every row to a task, and `tp validate` checks the coverage. A table authored in the interview and hardened by the review panel predates the implementation, so the unit writing the test transcribes the expected value instead of deriving it from the code under test. That source is the defence against a tautological test, not ordering: tp runs test work after implementation, and a test whose expected value was read off the code passes the gate exactly like a real one. Secondary benefit — committing to a concrete expected value forces out the ambiguity prose hides, which is friction that belongs in the interview (Step 0), not in a test written later.
5. **Source anchors**: every task MUST have `source_sections` (canonical headings, e.g. `"## 4. Backend Migration"`). `source_lines` (`"15-42"` or `"15-42,50-60"`) is **optional precision** — sections are the primary anchor because line numbers die on every spec rewrite while heading anchors survive. A task with neither anchor is a validation error. When `source_lines` is absent, `spec_excerpt` (in `tp plan`/`show`/`next`/`next --peek`) is assembled from the section text — so the mandatory anchor also carries the spec content and removes a round trip.
6. **Dependencies**: types before logic, logic before CLI, CLI before tests
7. **A change and the test it invalidates belong to the SAME task.** The quality gate runs at every `tp done`, so "a later task fixes that test" is impossible — the earlier close is already red. A dependency graph that separates them looks correct and cannot be executed. Decompose by *what the gate sees*, not by concern.
8. **Preview before import**: list proposed tasks and ask for confirmation

### source_sections format

Each `source_sections` entry MUST match a heading in the spec, in canonical form:
`"## Heading Text"` (heading marker prefix + space + heading text).

Example: spec contains `## 4. Backend Migration` → use `"## 4. Backend Migration"` in source_sections.

`tp import` and `tp add` are lenient — `"4. Backend Migration"` (without prefix) is also accepted when unambiguous and is auto-normalized. Use the full canonical form when the same text appears at multiple heading levels (both `## Setup` and `### Setup` exist) — otherwise the entry is ambiguous. A `tp validate` warning (error under `--strict`) fires for every ambiguous or unresolvable entry, so a typo'd anchor is never silently equivalent to no anchor.

### Coverage block: context_only vs unmapped

Each task file's `coverage` block tracks how spec headings relate to tasks:

- **`coverage.mapped_sections`**: headings referenced by at least one task's `source_sections` (after canonical resolution)
- **`coverage.context_only`**: spec headings NOT referenced by any task — treated as "context only" (intro, motivation, examples). Auto-fill marks all unreferenced headings here.
- **`coverage.unmapped`**: spec headings that should map to a task but do not. `tp validate` treats these as errors. Normally empty after auto-fill.

Arithmetic invariant: `mapped_sections + len(context_only) + len(unmapped) == total_sections`.

## Workflow B: Execute (tasks exist)

```
plan=$(tp plan --minimal --json)  # ONE call for full plan
# For each task: implement, then commit per the effective commit_strategy:
#   builtin: tp commit <id> "evidence"   (or tp done <id> "evidence" --auto-commit)
#   hc:      commit with hc, then tp done <id> "evidence" --commit <sha> [--commit <sha> …]
# Batch close: tp done --batch results.ndjson  (each row carries a commit_shas array or covered_by)
```

**The quality gate runs automatically at `tp done`** (and `tp close`): when `workflow.quality_gate` is set, closing a task runs the command once per invocation; a failing gate blocks the close (exit 4) and no task closes. There is no `--gate-passed` step to perform — the flag is ignored when a gate is configured. On a gate-less project, `--gate-passed` still records an attestation.

After all tasks done, run the audit loop (see Workflow D) until convergence.

## Workflow C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first. The audit loop applies equally.

## Workflow D: Audit loop (convergence via record + status --check)

Repeat until `tp audit <spec> --status --check` exits 0:

1. `tp audit <spec>` — emits one prompt per active auditor role from the corpus (defaults: `spec-coverage`, `security`, `maintainability-conventions`) with an embedded JSON-array checklist and its affected files. **spec-coverage is the only auditor id that changes routing** — it alone takes the spec-derived checklist and its own file selection; every other role, built-in or user-defined, receives one `file_check` item per file over the same shared, ranked, capped code-file list. Auto-detects changed files via git diff; `--affected-files` overrides, or `--affected-from-tasks` audits exactly the files touched by done tasks' `commit_shas` (the common post-implementation case needs no manual list). When no audit-able file is found it exits 4 with `suggested_files` (the same commit-derived list) and a hint.
2. Spawn one sub-agent per role prompt; each returns one NDJSON line per checklist item (`status` ∈ PASS/PARTIAL/FAIL).
3. Merge the per-role files: `tp audit --merge r1.ndjson r2.ndjson ... -o results.ndjson` (no spec positional — `--merge` takes NDJSON inputs only and rejects a spec with exit 2) (dedups by `role`+`item_id`, reports a status/role breakdown and the same per-input `inputs` accounting, exiting 1 on an input whose content rows all failed to parse), then record: `tp audit <spec> --record results.ndjson` — a row counts as a finding when `status` is absent or ≠ `PASS`; a clean round has zero findings. The audit round sequence is independent of review rounds.
4. Fix the code for every non-PASS item, then record how each was closed: `tp audit results.ndjson --resolve <0-based index|role:item_id> <fixed|wontfix|duplicate> "<evidence>"` (`--resolve-all` for the whole file, `--force` to overwrite a disposition already there). A row closed with no code change at all — a `wontfix` or a `duplicate` — is recorded the same way, which is the only way that outcome becomes durable. **A disposition is not an escape hatch from the gate:** `tp audit --record` counts every row whose `status` is not exactly `PASS` and reads no disposition at all, so parking a finding leaves the round's finding count, the role streak and `--status --check` exactly where they were.
5. Repeat. `tp audit <spec> --status` shows `consecutive_clean`, `converged`, `stale`, `budget_exhausted`, `max_rounds`/`rounds_remaining`/`in_flight_round`, and (with `--merge`/`--status`) an `overlap_report` — the audit-side signal for trimming a redundant auditor.
6. **Read the divergence signal (v0.33.0).** `tp audit <spec> --status` and `tp audit <spec> --record <file>` also carry `role_streaks` (each role's `consecutive_clean`/`open` in the latest recorded round — the streak lengths show whether the remaining findings are backlog or a regression), `spec_coverage_clean_rounds` (that role's streak, or `null` when the latest round measured no conformance at all — `null` ≠ `0`, so an absent `divergence` beside it proves nothing), and `divergence`, emitted only when spec-coverage has reached the clean-round threshold while other roles still hold open findings. All three survive `--compact`; the emission conditions and field shapes are in [REFERENCE.md](REFERENCE.md).

   **`divergence` gates nothing — it is reporting only.** Convergence arithmetic, the stored per-round `clean` flag, `next_action` and the `--status --check` exit code are unchanged: audit convergence still counts every non-PASS row, so `--check` still exits 1 and `next_action` still reads fix-and-re-audit. When `divergence` appears, **surface it and stop** — accepting findings outside spec conformance is a user-approved decision, never the agent's.

**Scope the audit, or it will not converge (v0.32.0 lesson, the hard way).** A row counts against
convergence whether it is about the spec or about the codebase at large — tp has no audit-side
equivalent of `review_converge_on` yet. General lenses (`go-safety`, `maintainability-conventions`,
`ax-contract`) always find *something* in a real codebase, so there is no fixed point: tp's own
v0.32.0 audit ran 11 rounds while `spec-coverage` — the only role measuring spec conformance — was
55/55 clean from round 2 onward, and every repair round created fresh surface for the next round to
audit. **Watch `spec-coverage` separately — since v0.33.0 tp reports it as `spec_coverage_clean_rounds` and names the split as `divergence`, so this is no longer hand-tracking.** Once it is clean for two rounds and no round has ever
produced a FAIL, findings outside the spec's surface are backlog, not a release gate: record them
with `tp review --resolve`-style evidence, name the version that will take them, and ship. Keep
audit repairs minimal for the same reason — a repair that introduces a new abstraction is the next
version's task, not this audit's.

**Both refusals are emission-only.** `enabled: false` that empties a phase, or names `spec-coverage`,
exits 2 before any prompt is emitted and before any state is written — so `--record`, `--status`,
`--merge` and the rest are unaffected, and a refused run leaves no `.tp-review/<spec>/` behind.

## Reset-native workflow & commit strategy (v0.28.0)

Every unit — a review round, decomposition, one task, one audit round — is designed to run in a **fresh context**; tp is the durable state machine between resets. The reset is the orchestrator's job (a CLI cannot clear its caller's context); tp guarantees resumability.

**`tp resume [spec]`** — the phase-agnostic oracle. From durable state alone (task file, spec, `.tp-review/`, `.tp/local.json`, git) it returns `{phase, spec, changes, kept, bookkeeping, next_units, round, last_failure, next_action, blockers}` (plus `guidance` in the implement phase). Read-only. `next_units`, `round` and `phase` are the machine surface a driver parses (v0.35.0): `next_units` is the ordered `{kind, id, brief_command}` list to execute now, `round` the round they belong to (`null` outside a round-based phase), and `last_failure` the advisory `{unit_kind, unit_id, phase, exit_code, summary, at}` record of the wall the previous attempt hit, or `null` — it never changes which unit runs next. Phase is task-first: an open task reads `implement` even when the spec is stale (a `spec-stale` blocker fires, the phase does not revert). `next_action.payload` embeds the next unit's work, so a fresh agent needs one call rather than a probe across `tp status`/`tp next`/`--status`. `bookkeeping` lists the tp-owned dirty files a close legitimately leaves modified under `commit_strategy: hc` — commit them; they never count as an `unexplained-changes` blocker. The field-by-field output table and the blocker vocabulary are in [REFERENCE.md](REFERENCE.md).

**The loop** — what a driver does, whether tp runs it or you do:

```
r = tp resume
for each blocker: agent-clearable (unexplained-changes) → reconcile (commit or tp keep), re-run resume
                  escalate (no-ready-task, *-budget-exhausted, spec-stale) → stop, hand to a human
blockers empty  → run next_action.command in a FRESH unit (sub-agent / session / process)
repeat until phase == release
```

**`tp run [spec]` — tp now ships that driver (v0.35.0).** It reads the cycle through the same path `tp resume` uses, stops if the cycle is releasable, takes `next_units`, spawns one runner process per unit — the two role kinds concurrently, every other kind alone — re-reads the state **from disk**, checks the caps, and loops. A unit's result is whatever it wrote to disk: the driver reads a child's exit code and one spend number, and nothing else it said. It exits **0** only on `stop_reason: converged` and **4** on every other stop reason, and holds a run-scoped lock at `.tp/locks/run-<base>.lock` for the whole run, so a second `tp run` over the same task file exits 4. `tp run --dry-run` prints the batch it would spawn without spawning anything or taking the lock; `tp run --status` reports the current or last run. The eight unit kinds, the nine stop reasons, the run state file, the child environment, the `runner` field and `notify_cmd` are in [REFERENCE.md](REFERENCE.md).

**Unattended units cannot take the user's decisions.** Every child is spawned with `TP_UNATTENDED=1`, and under it the four user-only decisions fail closed with exit 2 and a hint naming `tp escalate`: `tp done --skip-gate` (and its other sinks — a `--batch` row's `skip_gate`, `tp close --skip-gate`), `tp import --force`, and a `tp set --workflow` **raise** of `review_max_rounds`/`audit_max_rounds` or of any `run_max_*` cap. An equal or lower value is accepted; `runner` and `notify_cmd` name commands the driver executes and are refused at **every** layer rather than only above the resolved value. A unit that reaches such a decision records it instead of taking it — `tp escalate --decision <name> --evidence <text> [--option <text>]…` writes `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` and exits 2, the run stops with `stop_reason: escalation`, and the operator makes the decision and runs `tp run` again. Nothing is replayed, and an escalating unit is not counted as a failed attempt.

**The plugin.** tp publishes a Claude Code plugin at the repository root: `.claude-plugin/plugin.json` beside `marketplace.json`, this `skills/tp`, plus `hooks/` and `agents/`. **The Go binary is not in it** — installation stays Homebrew or `go install`, and the `SessionStart` hook preflights `tp`'s presence and version and fails with the install command rather than degrading quietly. The other two hooks hold the unit fence: `PreToolUse` denies hand-writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json`, and `Stop` refuses a role unit's stop once, when its findings file is still missing and it wrote no escalation record. `agents/` declares `tp-implementer`, `tp-reviewer` and `tp-auditor`, carrying **tool restrictions only** — the role's content stays in the corpus and reaches the unit through the prompt `tp review`/`tp audit` emits. Every hook declares a `timeout` of 10 seconds. A runtime that cannot load a plugin runs the same units without them: the restrictions are defence in depth, and the brief plus the tp commands are the durable contract.

**Interactive fallback — subagent-per-unit.** Where no runner is configured, or a human belongs in the loop, drive the same loop by hand:

**The orchestrator's job is to brief the unit, not remember what to inject (v0.30.0).** A fresh unit does not inherit the orchestrator's session history, so run `tp brief` (read-only — produce the brief, then decide whether to spawn) or tell the unit to run `tp next --brief` (claims the task and returns the brief in one call). The brief assembles everything the unit needs from durable state: **identity** and the one-unit-then-stop rule, the **scope fence** (acceptance-only — no refactor/rename/cleanup, no hand-editing the task file or `.tp-review/`; report an out-of-fence finding as an `Out of scope:` line instead of fixing it), **prior work** (each dependency's evidence and the files its commits touched), the verbatim **acceptance** + `spec_excerpt`, and the exact **close recipe** for the effective `commit_strategy` (the `--` separator before a dash-leading reason, the N-evidence-line format, and under `hc` that `tp commit`/`--auto-commit`/a bare `tp done` are rejected). `tp resume`'s `next_action.brief_command` names the right command per phase — `tp next --brief` in implement, `tp review <spec> --round N` in review, `tp audit <spec>` in audit — so an orchestrator following `next_action` reaches a full brief without knowing which phase it is in. The injection duty now survives ONLY for what tp cannot know: runtime-specific setup, such as which tools the sub-agent should use (e.g. native Read/Edit are hook-blocked in some repos → use the project's MCP toolset) and any live operational gotcha not yet carried by the brief.

**Realizing the reset in Claude Code.** The "FRESH unit" is a Claude Code **subagent** (Agent/Task tool): clean context, one unit, work persisted to disk (commit, `tp done`, `.tp-review`); the orchestrator re-orients via `tp resume` between units. Subagents don't nest (one level), so the orchestrator does each review/audit round's per-role fan-out itself. For a *full* reset of the driver too, use the `/clear` + `tp resume` loop or drive tp with headless `claude -p` per unit.

**Driving tp from a non-Claude runtime (v0.29.0).** tp is a CLI subprocess and the loop above is runtime-neutral — `tp run`'s `runner` field ships an `opencode` template beside `claude`, and a runner object drives any other command — or fan out each unit yourself (a review/audit round's per-role prompts, or one implementation task) with whatever sub-agent primitive the runtime provides: a Task/subagent tool, a headless `agent -p` invocation, a forked process, or a fresh session. Produce the brief with `tp brief` (or have the unit run `tp next --brief`) and inject only runtime-specific setup — the brief carries everything else. Three runtime hazards to configure up front:
1. **Permission prompts truncate the loop.** tp runs non-interactively, but a review/audit unit fans out to sub-agents that often need to **read** files the runtime gates behind a permission prompt. A headless runtime that **auto-denies** prompts truncates the round mid-way — sub-agents return empty findings instead of reading the blocked files, producing a **false clean round**. Configure the runtime to allow the file reads tp's units need (a read-tool allow-list), or run the units where prompts can be answered, *before* driving the loop.
2. **Fan-out depth.** tp's fan-out is one level: the orchestrator spawns a round's role reviewers/auditors. If the runtime's sub-agent primitive does not nest, the orchestrator runs that round's fan-out inline — correct and expected; `tp resume` still re-orients between rounds.
3. **A runtime that merges the child's streams corrupts `--json`.** tp keeps them separate by contract: every advisory, error and hint goes to **stderr**, and only the payload reaches stdout, so a piped `tp … --json` is parseable. A runtime that captures the child as one combined stream breaks that guarantee *outside* tp, and `2>/dev/null` on the command does not help because the merge happens above it. The symptom is an advisory line — `round N file … is missing; skipping its rows` — appearing in front of the JSON and surviving redirection, which reads like tp writing diagnostics to stdout. Capture the two streams separately, or run tp with `--quiet`, which suppresses the advisories at the source.

A unit returns to the loop only at a clean checkpoint; a crashed unit is recovered on the next `tp resume`, never lost.

**`commit_strategy`**:

- `builtin` — tp commits (`tp commit`, `tp done --auto-commit`, `tp done --commit`).
- `hc` — the agent commits with `hc`, then records via `tp done --commit <sha> [--commit <sha> …]`.
- `auto` — `hc` when on `PATH`, else `builtin`.

`commit_strategy` is authored at `tp init`; the project default is settable with `tp set --workflow --project commit_strategy=<builtin|auto|hc>` (writes `.tp/config.json`), while the task-file setter stays read-only (`tp set --workflow commit_strategy=…` exits 2 with a hint naming the project setter).

A task closed with commit(s) records `commit_shas` (ordered; `commit_sha` mirrors `[0]`). `tp done --commit` is repeatable; a duplicate sha exits 1.

**`tp keep`** — the durable, git-ignored (`.tp/local.json`) memory of files kept uncommitted, so `tp resume` classifies them as `kept` not `changes`. Feed `tp resume`'s `kept[].path` into `hc`'s `allow_unplanned`.

## Command & flag inventory

Every command and flag tp registers, in its exact form. Field ranges, exit codes and schemas are in
[REFERENCE.md](REFERENCE.md).

### Primary workflow
| Command | Purpose |
|---------|---------|
| `tp plan` | Full execution plan (THE primary command) |
| `tp plan --minimal` | Minimal plan: id + acceptance (~80% fewer tokens) |
| `tp plan --compact` | Stripped plan: no description, source_lines, tags (~40% fewer) |
| `tp plan --from <id>` | Start plan from a specific task onward |
| `tp plan --level 0,1` | Filter by parallelism levels (multi-agent) |
| `tp commit <id> [reason]` | Stage + structured commit + record SHA |
| `tp commit <id> --files "*.go"` | Selective file staging |
| `tp done <id> "reason"` | Single close with implicit claim + verification; runs the quality gate |
| `tp done <id> --skip-gate "why"` | Skip gate execution, record `gate_skipped_reason` (needs user approval) |
| `tp done <id> --gate-passed` | Gate-less projects only: record an attestation; ignored when a gate is set |
| `tp done <id> --auto-commit` | Commit + close in one call |
| `tp done <id> --auto-commit --files "*.go"` | Selective staging + commit + close |
| `tp done <id> --covered-by <id>` | Close as covered by another done task |
| `tp done <id> --commit <sha>` | Record implementing commit SHA |
| `tp done id1 id2 "reason"` | Multi-ID close (shared reason) |
| `tp done <id> --commit a --commit b` | Record multiple commits (hc flow); repeatable, duplicate exits 1; `commit_sha` mirrors `commit_shas[0]` |
| `tp done --batch file.ndjson` | Batch close from NDJSON |
| `tp done <id> --reason-file reason.md` | Read the closure reason from a file instead of an argument |
| `tp done <id> --stdin` | Read the closure reason from stdin |
| `tp resume [spec]` | Report phase + next action from durable state (reset-native, read-only; `--compact`) |
| `tp brief [id]` | The unit brief (read-only): identity + scope fence + prior work + the task + the close recipe; claims nothing |
| `tp brief <id> --prior <n>` | Override the prior-work recency cap (0-20; 0 = dependency entries only) |
| `tp run [spec]` | Drive the cycle unattended, one unit at a time |
| `tp run --status` | Report the current or last run: phase, units done, the accrual against each cap, the last unit's exit code and log path, `stop_reason`, and `run_state` (`in_flight`/`crashed`/`stopped`); takes no run lock; exits 3 when no run state exists |
| `tp run --dry-run` | List the units the driver would execute next as `{phase, round, next_units}` and exit 0; spawns nothing, writes no run state, takes no run lock |
| `tp escalate --decision <name> --evidence <text>` | Record a decision only the operator can take and stop the unit: writes `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` and exits 2; `--decision` is one of `skip-gate`, `raise-review-cap`, `raise-audit-cap`, `import-force`, `other`; outside a run (no `TP_RUN_DIR`) it is a usage error |
| `tp escalate ... --option <text>` | Add a way forward the unit saw; repeatable, and `options` is `[]` when none is given |

### Incremental
| Command | Purpose |
|---------|---------|
| `tp next` | Resume WIP or claim next ready |
| `tp next --minimal` | Minimal output: {id, acceptance} only |
| `tp next --peek` | Preview without claiming |
| `tp next --brief` | Claim the task and return its brief (identity, scope, prior work, close recipe); `--brief` + `--minimal` exit 2 |

### Task state
| Command | Purpose |
|---------|---------|
| `tp claim <id> [id...]` | open -> wip (batch: multiple IDs) |
| `tp claim --all-ready` | Claim all ready tasks at once |
| `tp close <id> <reason>` | wip -> done (low-level, prefer tp done) |
| `tp close <id> --reason-file reason.md` / `--stdin` | Read the closure reason from a file / from stdin |
| `tp close <id> --skip-gate "why"` | Skip gate execution, recording `gate_skipped_reason` on the closed task (needs user approval) |
| `tp reopen <id>` | done -> open (clears timestamps + SHAs) |
| `tp remove <id>` | Remove task (--force cleans deps) |
| `tp set <id> field=value` | Update field (managed fields protected) |
| `tp set --workflow field=value` | Update workflow fields: `review_clean_rounds`/`audit_clean_rounds`, `review_converge_on`, `gate_timeout_seconds`, `review_max_rounds`/`audit_max_rounds`, `lock_timeout_seconds`, and the five run caps `run_max_units`, `run_max_wall_clock_seconds`, `run_max_budget_usd`, `run_max_unit_budget_usd`, `run_max_unit_retries` |
| `tp set --workflow runner=…` / `notify_cmd` | **Not settable.** `runner` is `unknown workflow field` (exit 2) — hand-write it under `workflow` in `.tp/config.json` or a task file; `notify_cmd` is a top-level key of `.tp/local.json` only. Both name a command the driver executes |
| `tp set --workflow checks='[{"class":"s","cmd":"c"}]'` | Replace the mechanical-checks list (JSON array; `class` kebab-case unique, `cmd` non-empty) |
| `tp set --workflow --project <field>=<value>` | Edit a project-level workflow field (writes `.tp/config.json`) |
| `tp set --local defaults.<flag>=<bool>` | Set a CLI flag default (`compact`/`quiet`/`no_color`) |
| `tp set --bulk sets.ndjson` | Bulk update from NDJSON {id, field, value} |
| `tp keep <path> "<reason>"` | Keep-list a deliberately-uncommitted file (`--remove`, `--list`) |

### Query
| Command | Purpose |
|---------|---------|
| `tp list` | All tasks (--status, --tag, --ids, --compact) |
| `tp ready` | Deps-satisfied tasks (--first, --count, --ids) |
| `tp show <id>` | Full details + spec_excerpt + blocks |
| `tp status` | Progress summary (open/wip/done counts) |
| `tp report` | Per-task duration + estimation accuracy |
| `tp blocked` | Tasks waiting on unsatisfied deps |
| `tp graph` | Dependency tree (--tag, --from) |
| `tp stats` | Parallelism analysis |
| `tp config` | Effective configuration as JSON (`--resolved`; `--extract [--dry-run\|--force]` hoists policy shared by ALL task files into `.tp/config.json`) |

### Spec & validation
| Command | Purpose |
|---------|---------|
| `tp lint spec.md` | Spec quality + structured elements + duplicate lines/paragraphs + numbering gaps + orphan list items + broken cross-refs |
| `tp review spec.md` | Adversarial review prompts (one per active reviewer role) |
| `tp review spec.md --perspective code-audit --affected-files src/a.go` | Code audit with source file injection (never reads `--findings`; passing it exits 2) |
| `tp review spec.md --round N --findings file.ndjson` | Multi-round with previous findings exclusion |
| `tp review spec.md --round N --final-round --affected-files src/a.go` | Final round with mandatory code read-through |
| `tp review spec.md --affected-files a.go b.go` | Inject source files into every default review prompt |
| `tp review --merge r1.ndjson r2.ndjson -o merged.ndjson` | Merge + dedup findings (`-o` is short for `--output`); reports `inputs` as `{path, parsed, skipped}` per file. All-empty inputs (a converged round) exit 0 and write a zero-byte `-o` file; an input with content lines and zero parsed exits 1; a missing input exits 3; no inputs exit 2 |
| `tp review spec.md --perspective documentation --docs-path docs/` | Documentation perspective; `--docs-path` is required with it |
| `tp review spec.md --perspective testing --test-path internal/` | Testing perspective; `--test-path` is required with it |
| `tp review --resolve findings.ndjson <idx> <disposition> "evidence"` | Mark one finding fixed/wontfix/duplicate; `<idx>` is **0-based** |
| `tp review --resolve-all findings.ndjson <fixed\|wontfix\|duplicate> "evidence"` | Dispose of **many** findings in one call (evidence optional) — the way to accept all surviving non-blocking findings under one justification |
| `tp review --resolve ... --force` | Force re-resolve already resolved findings |
| `tp review --verify spec.md --findings all.ndjson` | Lightweight verification (verifier role) |
| `tp review --report r1.ndjson r2.ndjson` | Cross-round convergence report |
| `tp review spec.md --diff-from old-spec.md` | Diff-based review; overrides the snapshot baseline and forces the inline diff block at any round |
| `tp review spec.md --spec-inline` | Embed full spec inline (default is reference mode) |
| `tp review spec.md --record merged.ndjson` | Record a review round; auto-numbers R, freezes the count + clean flag, returns `next_action` |
| `tp review spec.md --record ... --harness-note "<text>"` | Record the round's orchestrator-wrapper framing (requires `--record`; alone → exit 2) |
| `tp review spec.md --status` / `--status --check` | Convergence state / exit 0 only when converged AND every check passes |
| `tp review spec.md --perspective regression` | Standalone regression pass (needs state R≥2, or `--diff-from` + `--findings`) |
| `tp review spec.md --no-state` | Disable all state reads/writes; restores pre-0.23.0 manual `--round` numbering |
| `tp review spec.md --role <name>` | Emit only that role's prompt. A name the round does not emit but tp recognises exits 0 with an empty `prompts[]`; a name it recognises nowhere exits 2 |
| `tp audit spec.md` | Post-implementation audit: verify code matches spec. No audit-able file → exit 4 with `suggested_files` + hint |
| `tp audit spec.md --affected-files src/a.go` | Manual file selection (comma or repeated) |
| `tp audit spec.md --affected-from-tasks` | Audit exactly the files touched by done tasks' `commit_shas` |
| `tp audit spec.md --role <name>` | Emit only that role's prompt; same three name classes as `tp review --role` |
| `tp audit spec.md --findings review.ndjson` | Also verify review findings were addressed (routed to spec-coverage) |
| `tp audit spec.md --record results.ndjson` | Record an audit round (non-PASS rows = findings); independent sequence |
| `tp audit spec.md --base <git-ref>` | Diff against a git ref to detect the audited files (omit for staged + unstaged) |
| `tp audit spec.md --record ... --harness-note "<text>"` | Record the round's orchestrator-wrapper framing (requires `--record`) |
| `tp audit --merge r1.ndjson r2.ndjson -o results.ndjson` | Merge + dedup per-role audit results by `role`+`item_id` (`-o` is short for `--output`); same `inputs` accounting and same exit 1 on an input that parsed nothing |
| `tp audit spec.md --status` / `--status --check` | Audit convergence state / exit 0 only when converged |
| `tp audit results.ndjson --resolve <selector> <disposition> "<evidence>"` | Dispose one audit row — selector is a 0-based index or the row's `role:item_id` key; disposition is fixed/wontfix/duplicate (results NDJSON is the positional, a spec → exit 2) |
| `tp audit results.ndjson --resolve-all <disposition> "<evidence>"` | Dispose every undisposed audit row; add `--force` to re-resolve rows already carrying a disposition |
| `tp validate` | Task file validation + line coverage + atomicity |
| `tp validate --strict` | Atomicity warnings become errors |
| `tp validate --project` | Cross-spec workflow drift (informational; `--strict` → exit 1) |

### Data
| Command | Purpose |
|---------|---------|
| `tp init spec.md` | Create the task file shell (also writes `.tp/.gitignore`, which covers `.tp/locks/` and the run artifacts) |
| `tp init --eject-roles [--domain <name>] [--force]` | Write the default role corpus into `.tp/reviewers` and `.tp/auditors` |
| `tp init spec.md --quality-gate "<cmd>"` | Write a task-file-level `quality_gate` override (otherwise the project default in `.tp/config.json` applies) |
| `tp init spec.md --commit-strategy <builtin\|auto\|hc>` | Write a task-file-level `commit_strategy` override |
| `tp add <json>` | Add task (`--stdin` for piped input); entry rules reject missing id/title/acceptance/anchor and unknown deps |
| `tp add <json> --spec spec.md` | Spec path, required when `tp add` has to create the task file |
| `tp add --bulk tasks.ndjson` | Bulk add from NDJSON |
| `tp import file.json` | Import + validate (`--force` to overwrite + relax atomicity) |
| `tp import tasks.json --spec spec.md` | Import bare JSON array (auto-wraps into a TaskFile) |
| `tp use <file>` / `--clear` / bare | Set / clear / show the active task file (`.tp/local.json`) |

### Global flags
| Flag | Purpose |
|------|---------|
| `--file <path>` | Explicit task file path |
| `--json` | Force JSON output (default when piped) |
| `--compact` / `--no-compact` | Minimal JSON (~40% smaller) / force full output |
| `--quiet` / `--no-quiet` | Suppress info messages / force info output |
| `--no-color` / `--color` | Disable / force colored output |
## Closure Rules

Before closing a task (`tp done`):

1. Re-read acceptance criteria from the plan output.
2. Verify implementation matches the FULL spec (not just the acceptance summary).
3. Write the reason in **evidence-line format**: for a task with N ≥ 2 acceptance criteria, the reason MUST contain at least N lines each starting with `- ` at column 0 (indented sub-bullets do not count) — one top-level evidence line per criterion, with file paths. A single-criterion task accepts any non-empty reason.
4. Never use: "deferred", "will be done later", "covered by existing" (without a path), single-word reasons.
5. Use `--covered-by <id>` when the work IS done but in a different task (not a deferral).
6. `tp done` auto-claims open tasks — no separate `tp claim` needed.
7. Code snippets in a spec may be illustrative — validate against the actual codebase before implementing.

> A reason starting with `- ` looks like a cobra flag; use the `--` separator: `tp done <id> --commit <sha> -- "- line 1\n- line 2"`.

## Gate, Budget & Escalation Policy — user-approval gates

- **The gate runs automatically at `tp done`.** `--skip-gate "<reason>"` skips it and records `gate_skipped_reason` on each closed task. **`--skip-gate` requires explicit user approval — it is never the agent's own decision.**
- **Round-budget exhaustion (`review_max_rounds` / `audit_max_rounds`):** when the cap is reached and the sequence is not converged, `tp review` / `tp audit` prompt generation and `--record` refuse with exit 4 and an escalation hint. **The agent STOPS and escalates.** Raising the cap with `tp set --workflow`, and importing with `--force`, are user-approved decisions — never the agent's own.
- **Convergence criteria differ by phase (v0.28.0+).** A **spec review** is converged only when a counted round surfaces **no blocking findings** — the blocking severities are **critical and high** (the built-in `review_converge_on=blocking` policy; `review_converge_on=all` opts into the strict any-severity rule): never declare review convergence or accept a round cap while a critical or high finding is open (medium/low findings may be accepted with recorded justification once none blocking remain). An **implementation audit** always runs to the full **2 consecutive clean rounds and is never cut short by an early cap** — a hit `audit_max_rounds` means fix the findings and continue (with a user-approved cap raise), never ship with them open. Implementation correctness is not negotiable.
- **Under `tp run` the policy is enforced, not merely stated (v0.35.0).** Every unit a run spawns carries `TP_UNATTENDED=1`, and the four decisions above stop being available to it: `--skip-gate` at any of its sinks, `tp import --force`, and a `tp set --workflow` raise of `review_max_rounds`/`audit_max_rounds` or any `run_max_*` cap each exit 2 with a hint naming `tp escalate`. Record the decision with `tp escalate --decision <skip-gate|raise-review-cap|raise-audit-cap|import-force|other> --evidence "<what you found>" [--option "<a way forward>"]` — the run stops with `stop_reason: escalation`, the operator decides, and `tp run` is started again. **An escalation is a normal outcome, not a crash**, and it is neither a failed attempt nor a recorded round.
- **Verify the gate with separate exit codes, never a `&&` chain.** `<tests> && <linter> && echo OK` prints nothing when it fails, and a missing `OK` does not catch the eye. Run each command on its own and read each exit code (`<tests>; echo $?`, then `<linter>; echo $?`) before you close the task.

## Class & Checks Guidance

- **Fill `class`** on a review finding when it is an instance of a pattern a script could check across the whole corpus (example: `code-citation-drift`); omit it otherwise.
- **Mechanization candidate:** a class that appears in ≥ 2 distinct rounds OR ≥ 5 times in a single round (`tp review --report` and `--record` output list `mechanize_candidates`). When one appears, write a detector command and register it: `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'`. A check is **only worth registering when the artifact it measures already exists in the review phase** — registered checks run in the review phase only, so a check whose subject a later phase writes can never verify it, while tp still tells every reviewer to stop reporting that class. When the subject is written later, make running the check the acceptance of the task that creates it instead.
- Once registered, tp runs the check every review round, reports pass/fail in `mechanical_checks`, and tells reviewers to stop reporting that class. `tp review --status --check` requires every check to pass before exiting 0.
- **Registration retires the suggestion (v0.33.0):** a registered check retires its mechanize candidate — the class is dropped from `mechanize_candidates`, from the register-a-check `hint`, and from `next_action`'s mechanize branch, and `tp review <spec> --record <file>` names the withheld classes in `mechanized_classes`. Registration is the trigger, not the check's result — a failing check is still reported in `mechanical_checks` and still blocks `tp review --status --check`. Class matching is byte-for-byte.
- `over-specification` is the one exception, scoped to a single sink: registering a check for it suppresses it from the candidates like any other class, but it never joins the `Mechanically checked classes — do NOT report findings of these classes:` sentence, because tp's own review prompts ask reviewers to raise it.

## Role Corpus & frontmatter overrides (v0.25.0)

Review and audit roles are **project-owned data** — one JSON file per role under the repo-root `.tp/`:

- `.tp/reviewers/*.json` drives `tp review`; `.tp/auditors/*.json` drives `tp audit` (phase = directory). Schema `{id, title, instructions, focus[], domains[]}`; `id` MUST equal the filename stem (lowercase kebab-case); `regression` is reserved. Commit the corpus to VCS.
- A populated phase directory **replaces** the embedded default corpus for that phase; absent/empty keeps tp's curated defaults (software: implementer/tester/architect + spec-coverage/security/maintainability-conventions; prose: coherence/soundness + spec-coverage/soundness). A project happy with defaults keeps **zero role files**.
- `tp init --eject-roles [--domain software|prose] [--force]` writes the defaults as editable, byte-identical files, and **ejected role files are not rewritten on upgrade** — re-eject with `--force` to adopt newer default prompts. tp's own repo dogfoods a custom 4+4 corpus (adopted v0.26.0: implementer/tester/architect/ax-economist reviewers + spec-coverage/go-safety/maintainability-conventions/ax-contract auditors).

Emission is corpus-driven: `tp review` emits one prompt per active reviewer role plus the built-in `regression` role (appended, never a corpus file); `tp audit` emits one per active auditor role. Every prompt stamps the output contract (`role, location, class, severity`; audit adds `status`). tp still only emits prompts — it never executes agents.

Spec frontmatter steers the corpus without new files:

```yaml
---
tp:
  domain: prose          # selects & filters the corpus (no persona swap)
  review_roles:          # append focus questions to an existing role
    implementer:
      focus: ["Can each section be written without inventing facts not in the outline?"]
  audit_roles:
    spec-coverage:
      focus: ["Is every outline element present and fully developed?"]
---
```

- `domain` selects the embedded corpus when no role files exist and filters a user corpus by each role's `domains`; an unknown domain falls back to `software` with a lint warning.
- `tp.review_roles`/`tp.audit_roles` **append** focus to an existing role (project focus first); an unknown override id is a lint warning; `regression` takes no overrides. The standalone `tp: lens` is retired.
- **`enabled` (v0.32.0)** is the override object's second permitted key beside `focus`. enabled: false deactivates a role for one spec — no prompt, and that override's `focus` applied nowhere; `enabled: true` is a no-op. Two refusals fire **on prompt emission only** and exit 2: emptying a phase, and deactivating `spec-coverage`. Keep `domains` as the **durable** axis — a role's standing scope across every spec — and use `enabled: false` as the **one-spec exception** to it.
- **Overlap and staleness:** `tp review --merge`/`--report`/`--status` carry a per-role `overlap_report` flagging a `trim_candidate` (a reviewer that found only what others also found), and `--status` reports `roles_stale` when the corpus changed since the last recorded round. tp reports; trimming a role is your decision.

Corpus validation and its exit code, the exact warning and refusal strings, the cluster keys, the hash
rules and the two trim levers are in [REFERENCE.md](REFERENCE.md).

## Role authoring guidance (v0.25.0)

Opening role authoring is a power feature — a project-authored role is only as good as its prompt. Design each role for **high-signal, low-overlap, contract-conformant** findings:

1. **One distinct failure-lens per role.** A role must target a failure mode no other role covers. Overlapping roles waste tokens and get flagged as trim candidates (`overlap_report`) — diversity of lenses beats count.
2. **Adversarial framing.** "Try to refute this / find where it breaks / enumerate every X and verify each" outperforms "check whether this is fine". LLM reviewers have a leniency bias and underweight negation, so tell the role to actively hunt flaws and test the spec's "DO NOT" constraints.
3. **Evidence demand.** Every finding carries a `location` (a `§`-anchor) and a why — this is what makes dedup (the overlap report) and audit PASS/FAIL meaningful. A finding with no location is unverifiable.
4. **Scope boundaries.** State what the role does NOT cover (name the sibling roles' territory), so the panel tiles the problem space with disjoint lenses.
5. **Output-contract adherence.** The role customizes only its `focus`, never the finding schema — tp injects the fixed contract (`role, location, class, severity`, plus `status` for audit).
6. **Altitude.** A spec review should push toward **decidable invariants** — behavior a task's acceptance can verify against the quality gate — and away from implementation prose that pins mechanism (SQL, an index, a field layout) whose correctness only code can establish. The canonical finding `class` `over-specification` names exactly this smell: *a detail whose correctness can only be established against code, prescribed in the spec where it belongs in task acceptance instead.* Any reviewer may raise it; it is usually **low or medium** severity (an altitude judgment, not a blocking defect), so it never blocks convergence under `review_converge_on=blocking` unless a reviewer stamps it critical/high — convergence reads severity, not the class. tp does not detect over-specification and never deletes spec content on its own claim of it; the remedy is to revise the over-specified detail down into a task's acceptance where the gate can check it.

**Worked example role sets:**

- **code/software** — *correctness* (does the change actually work: error paths, edge cases, happy-path gaps), *security* (trust boundaries, injection, unsafe defaults, unpaired locks), *performance/contract* (backward-compat, complexity, interface consistency). The ejectable `implementer`/`tester`/`architect` reviewers and `spec-coverage`/`security`/`maintainability-conventions` auditors are worked examples of this guidance.
- **prose** — *narrative continuity* (coherence: does one part contradict, duplicate, or pre-empt another?) vs *expository derivability* (soundness: can each claim be derived from what precedes it without inventing facts?). Prose defaults to the leaner two-reviewer panel because prose flaws surface from many angles at once.

**Other domains** and their characteristic diverging lenses (for custom corpora): **legal/contract** — obligation completeness vs. ambiguity/loophole; **product/PRD** — user-journey completeness vs. measurable acceptance; **data-schema** — referential integrity vs. migration/compat; **academic** — claim support vs. methodology soundness.

The embedded default corpus is authored to exemplify this guidance, so an ejected default role is itself a worked example — run `tp init --eject-roles` to read them.

## Recording language — read in any language, record in English

**A unit reads the spec in whatever language its author wrote, and records in English.** The two
halves are separate rules and collapsing them either way is wrong.

Forcing the *input* to English would mean nobody could write a spec in their own language, and a
reviewer told to "work in English" against a Turkish spec may translate it in its head before
judging it — losing exactly the precision a review is for. So the spec, and the spec content the
prompt embeds, are the author's.

The *recorded output* is English without exception: finding text, audit row text, `class` slugs,
closure reasons passed to `tp done`, and commit messages. Three reasons, in the order they bite:

1. **They are committed artifacts.** `review-round-N.ndjson` and the audit results are checked in;
   a repo's English-only convention for committed files applies to them like any other file.
2. **The next round reads them.** `--merge` clusters by `(location, class)` and hands the
   representative to every role. A corpus with two languages splits one idea across two slug
   families, and `mechanize_candidates` — which matches slugs byte-for-byte — stops seeing it. One
   cycle in this repository already produced **20 distinct slugs for a single idea** in one
   language; a second language multiplies that, it does not add to it.
3. **A human reads the round later.** The recorded rounds are the only durable account of why a
   spec says what it says.

**This is a convention, not yet a fence.** Nothing in the emitted prompt states it today — measured:
the role prompt carries zero mentions of language. Until a release puts it in the output contract,
an orchestrator that spawns units itself should say it when it spawns them.

## `--role` — one prompt at a time (v0.36.0)

`tp review <spec> --role <name>` and `tp audit <spec> --role <name>` narrow an emission to a single
role's prompt. It is what a **unit** runs as its own first command: the emission the whole panel
would produce is many times the size of the one prompt the unit will act on, and a run that loses a
sub-agent mid-round loses only that role's work rather than the round's.

`--role` takes exactly one value and is not repeatable. `--role ""` is refused as an unknown role
rather than treated as an absent flag, so "flag given empty" and "flag absent" are never the same
command.

**Three name classes, three outcomes.** The set that produces a prompt is what *that invocation*
would emit — not the corpus, and not the phase's active role set: the built-in `regression` role is
emitted and belongs to no corpus, and `--perspective testing` emits `test-planner`, which is in
neither.

| The name is… | Outcome |
|---|---|
| in the emitted set | exit **0**, exactly one `prompts[]` entry, byte-identical to that role's entry in the same invocation without the flag |
| recognised but not emitted this round | exit **0**, `prompts: []`. A name *this* phase skipped keeps its own `skipped_roles` entry; a name recognised only through the other phase's corpus is in none |
| recognised nowhere | exit **2**, with a hint naming what the invocation would have emitted |

"Recognised" spans **both phases**: the user corpus and the embedded default corpus for reviewers
*and* auditors, plus `regression`. So an auditor running `tp audit <spec> --role spec-coverage` and
one running `tp review <spec> --role spec-coverage` both exit 0 — the second emits nothing, because
`tp review` never emits an auditor.

The exit-0-empty class is not a courtesy, it is forced: the unit set and the emitted set are
computed by different filters. A corpus role with an empty checklist is spawned as a unit and emits
no prompt, so under an exit-2 rule its own brief would fail before it did any work.

**Where the flag is legal.** It is legal wherever prompts are emitted and refused with exit 2 where
none are, and the refusal is evaluated *before* the mode's own argument validation, so the hint
names the flag conflict rather than a missing `--findings`. The two commands do not have the same
flag sets:

| Command | `--role` legal in | `--role` refused in |
|---|---|---|
| `tp review` | default, `--perspective`, `--diff-from`, `--verify` | `--merge`, `--record`, `--status`, `--report`, `--resolve`, `--resolve-all` |
| `tp audit` | default only | `--merge`, `--record`, `--status`, `--resolve`, `--resolve-all` |

`tp audit` registers no `--perspective`, `--diff-from`, `--verify` or `--report` at all.

**`review_loop.instruction` narrows with the payload.** That key is addressed to a caller holding
the whole panel — spawn a sub-agent per prompt, merge, record the round, order the regression prompt
against the others. Under `--role` the payload holds one prompt and no regression prompt, so tp
emits a sentence-subset of the key that directs no action the payload cannot support. Without
`--role` the key is unchanged.

## Where judgement-shaping text belongs (v0.31.0)

Standing instructions that shape a reviewer's judgement — what counts as a real defect, what to treat as intentional, what altitude to hold — do **not** belong in the orchestrator's prompt wrapper. tp cannot see that wrapper, so framing hidden there makes two rounds look comparable when their instructions differed materially. Standing framing has two sanctioned homes, both recorded via `roles_hash`:

- **Per-spec focus** → `tp.review_roles` / `tp.audit_roles` frontmatter (append `focus` to a reviewer or auditor role).
- **Project-owned roles** → `.tp/reviewers/*.json` / `.tp/auditors/*.json`.

The wrapper is only for what tp cannot know — runtime setup (e.g. hook-blocked native file tools → use the MCP toolset). When the wrapper must carry standing framing anyway, declare it with `--harness-note "<text>"` on `tp review`/`tp audit --record` so it is recorded on the round instead of staying invisible (the flag requires `--record`; supplying it alone is a usage error, exit 2). `--status` then surfaces `harness_stale` — true only when the two most recently recorded rounds **both** carry non-empty notes that differ — and, when stale, the latest `harness_note`. The note is opt-in, gates nothing, and both `harness_note`/`harness_stale` are stripped under `--compact`. Review and audit surface these symmetrically.

## `next_action` — the single next step (v0.31.0)

`tp review --status`/`--record` and `tp audit --status`/`--record` return a `next_action` naming the one next step the current state calls for, by a fixed precedence (retained under `--compact`):

1. **Converged** → the forward step: review names the directive `decompose the spec into tasks, then tp import <base>.tasks.json`; audit names the terminal `converged — implementation verified, proceed to release`. Convergence wins even when non-blocking findings remain open.
2. **Blocking (critical/high) findings open** → `revise the spec to address the blocking findings, then run the next review round` (audit: fix the findings, then re-audit). It never steers toward `--resolve`/`--resolve-all`: disposing a blocking finding is an operator decision, never auto-advised.
3. **A `mechanize_candidates` class recurs, none blocking** (review only) → register a check (`tp set --workflow checks='[…]'`), then run the next round. The directive carries the phase qualifier: a check is **only worth registering when the artifact it measures already exists in the review phase**, because registered checks run in that phase alone. The un-mechanizable `over-specification` class never triggers this — it falls through to step 4. Since v0.33.0 a class with a registered check falls through too: it is suppressed from the candidate list, so the driver is never told to write a check that already exists.
4. **Clean but not yet converged** → run the next round: `tp review <spec> --record <file>` (audit: `tp audit <spec> --record <file>`).

`next_action` is advisory and read-only; it gates no exit code.

## State directory (`.tp-review/`)

- **Commit `.tp-review/` to version control.** Import convergence enforcement holds across clones and CI only when the recorded rounds travel with the repo: ignoring the directory makes every `tp import` in CI behave as "no recorded rounds", and convergence is then unverifiable.
- **Prunable:** only snapshot files older than the newest MAY be deleted (the diff falls back gracefully).
- **Do not delete `.tp/locks/<base>-<hash>.lock` while tp may be running.** It is a zero-byte, git-ignored marker that persists after its lock is released, and that is what makes the lock exclude: flock binds to an inode, so unlinking it lets the next waiter lock a different inode at the same path and run concurrently.

## Tail protocol (when a round drops to one or two low/medium findings)

1. **Verify disputed findings:** route each through `tp review --verify <spec> --findings all.ndjson`. A verifier-rejected finding is resolved `wontfix` with the verifier's reasoning **and written into the findings file before `--record`** (the round entry never recomputes) — a round whose surviving rows are all pre-resolved `wontfix` records as clean.
2. **Class-sweep:** derive the class of each surviving tail finding and run one exhaustive class-sweep prompt per class ("enumerate every `<pattern>` in the spec; verify each") before the next counted round, so a single class cannot drip one finding per round. The class-sweep is uncounted.

## Project configuration (`.tp/config.json`)

Multi-spec repos keep **one** workflow policy in a repo-root `.tp/` instead of copying it into every
`<base>.tasks.json` (tp's own "derive, don't maintain a parallel list" principle applied to policy).
A task file's `workflow` block then holds only **explicit overrides**, and effective values resolve
**at read time** — so read the effective value, never the file. The commands are in the inventory
above; the file layout, the resolution order and the precedence rules are in
[REFERENCE.md](REFERENCE.md).

## Reference

Exhaustive field, exit-code and schema detail — workflow field ranges, the `tp resume` output table,
the audit JSON schema, the finding contract, the state-directory file formats — is in
[REFERENCE.md](REFERENCE.md).
