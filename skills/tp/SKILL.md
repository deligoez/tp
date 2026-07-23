---
name: tp
description: Spec-to-task lifecycle manager for AI coding agents. Interviews user to resolve ambiguities, decomposes specs into atomic tasks, manages execution order via dependency graph, and batch-closes with evidence. Use when user wants to implement a spec, plan tasks, decompose a feature, a *.tasks.json file exists, or user says 'continue with tp' / 'tp ile devam et' to resume work after a context clear.
---

# tp — Task Plan Skill

Activates when: a `.tasks.json` file exists, user asks to implement a spec/plan/tasks, or user references tp commands.

## "continue with tp" / "tp ile devam et" — zero-friction resume (any project)

A short resume intent — "tp ile devam et", "continue with tp", "tp'ye devam", "resume tp" — is a sufficient standalone prompt in ANY tp project after a context clear. `tp resume` rebuilds the full picture from durable state (task file, spec, `.tp-review/`, `.tp/local.json`, git); nothing load-bearing lives in the context window. On the trigger:

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

Running `tp init` **before** the review loop creates the spec-adjacent task file that supplies the loop's workflow parameters (convergence counts, round budgets, checks). The quality gate is authored at init time with `--quality-gate` because `tp set` keeps it read-only.

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
- Quality gate command (e.g. `"go test ./... && golangci-lint run"`) — authored at `tp init --quality-gate`.
- Consecutive clean **review** rounds (default 2) — integer 1-10; re-ask once if invalid, then use default.
- Consecutive clean **audit** rounds (default 2) — same rules.
- Optional round budgets `review_max_rounds` / `audit_max_rounds` (default 0 = no cap) — a hard ceiling on counted rounds before escalation.

Announce: "I will review until N clean rounds, audit until M clean rounds." If new ambiguities arise during spec writing, pause and return to step 3. Do not re-ask parameters.

### Step 1: Init the task file and workflow

1. `tp lint <spec.md>` — fix issues; review `structured_elements` and the `frontmatter` object.
2. `tp init <spec.md> --quality-gate "<cmd>"` — creates the spec-adjacent `<base>.tasks.json` shell (zero tasks) and the gate.
3. `tp set --workflow review_clean_rounds=N audit_clean_rounds=M` — convergence counts (only if non-default).
4. `tp set --workflow review_max_rounds=R audit_max_rounds=A` — round budgets (only if capping).
5. `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'` — register mechanical checks (see Class & Checks Guidance).

**Multi-spec repos:** put the shared gate/convergence policy in a repo-root `.tp/config.json` once (see [Project configuration](#project-configuration-tpconfigjson)) and leave each `tp init` shell's `workflow` block empty except where a spec genuinely deviates — that keeps one source of truth instead of copying policy into every `<base>.tasks.json`.

### Step 2: Review loop (explicit recipe)

Repeat until `tp review <spec> --status --check` exits 0:

1. `tp review <spec>` — tp auto-numbers the round (R = recorded rounds + 1), snapshots the spec, and injects previous findings + the changed-sections diff into every role prompt. A 4th **regression** prompt is auto-appended from round 2 when the spec changed or fixed findings exist — process it first.
2. Spawn one sub-agent per prompt; collect NDJSON findings.
3. `tp review --merge r1.ndjson ... -o merged.ndjson` — dedup across prompts.
4. `tp review <spec> --record merged.ndjson` — record the round. Zero surviving findings records a **clean** round.
5. Fix the spec; mark each addressed finding with `tp review --resolve merged.ndjson <idx> fixed "evidence"`.
6. When a fix batch touched **more than 3 sections**, run the standalone regression delta pass (`tp review <spec> --perspective regression`) as an uncounted check before the next counted round.
7. Repeat. `tp review <spec> --status` shows `consecutive_clean`, `converged`, `stale`, and `budget_exhausted`.

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
4. **Source anchors**: every task MUST have `source_sections` (canonical headings, e.g. `"## 4. Backend Migration"`). `source_lines` (`"15-42"` or `"15-42,50-60"`) is **optional precision** — sections are the primary anchor because line numbers die on every spec rewrite while heading anchors survive. A task with neither anchor is a validation error.
5. **Dependencies**: types before logic, logic before CLI, CLI before tests
6. **Preview before import**: list proposed tasks and ask for confirmation

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

1. `tp audit <spec>` — emits one prompt per active auditor role from the corpus (defaults: `spec-coverage`, `security`, `maintainability-conventions`) with an embedded JSON-array checklist and per-role affected files. Auto-detects changed files via git diff; `--affected-files` overrides.
2. Spawn one sub-agent per role prompt; each returns one NDJSON line per checklist item (`status` ∈ PASS/PARTIAL/FAIL).
3. Merge the per-role files: `tp audit <spec> --merge r1.ndjson r2.ndjson ... -o results.ndjson` (dedups by `role`+`item_id`, reports a status/role breakdown), then record: `tp audit <spec> --record results.ndjson` — a row counts as a finding when `status` is absent or ≠ `PASS`; a clean round has zero findings. The audit round sequence is independent of review rounds.
4. Fix the code for every non-PASS item.
5. Repeat. `tp audit <spec> --status` shows `consecutive_clean`, `converged`, `stale`, `budget_exhausted`.

## Reset-native workflow & commit strategy (v0.28.0)

Every unit — a review round, decomposition, one task, one audit round — is designed to run in a **fresh context**; tp is the durable state machine between resets. The reset is the orchestrator's job (a CLI cannot clear its caller's context); tp guarantees resumability.

**`tp resume [spec]`** — the phase-agnostic oracle. From durable state alone (task file, spec, `.tp-review/`, `.tp/local.json`, git) it returns `{phase, spec, changes, kept, next_action, blockers}`. Read-only. Phase is task-first: an open task reads `implement` even when the spec is stale (a `spec-stale` blocker fires, the phase does not revert). `next_action.payload` embeds the next unit's work (one round-trip): `{round, unresolved_findings}` for review/audit, `{task, wip}` for implement. `--compact` drops `summary`/`reason`/`message`, keeps every `data`.

**Reference driver** (runtime-neutral — tp ships none; embedding it would bind tp to one runtime):

```
r = tp resume
for each blocker: agent-clearable (unexplained-changes) → reconcile (commit or tp keep), re-run resume
                  escalate (no-ready-task, *-budget-exhausted, spec-stale) → stop, hand to a human
blockers empty  → run next_action.command in a FRESH unit (sub-agent / session / process)
repeat until phase == release
```

**Realizing the reset in Claude Code.** The "FRESH unit" above is a Claude Code **subagent** (Agent/Task tool): it starts with a clean context, does exactly one unit, and its work reaches disk (commit, `tp done`, `.tp-review`); the orchestrator re-orients via `tp resume` between units. When you spawn a unit, **inject what it needs** (it does not inherit the orchestrator's session history): (1) a **durable-state pointer** — tell it to run `tp next`/`tp resume` for its exact unit; (2) the **close recipe** for the effective `commit_strategy`; (3) the project's **live gotchas**. Subagents don't nest, so the orchestrator does each review/audit round's fan-out itself. For a *full* reset of the driver too, use the `/clear` + `tp resume` loop or drive tp with headless `claude -p` per unit.

**`commit_strategy`** (task override > `.tp/config.json` > built-in default `auto`):

- `builtin` — tp commits (`tp commit`, `tp done --auto-commit`, `tp done --commit`).
- `hc` — the agent commits with `hc`, then records via `tp done --commit <sha> [--commit <sha> …]`; `tp commit`/`--auto-commit`/bare `tp done` are rejected with exit 2. Never exit 4 — tp never runs `hc`.
- `auto` — `hc` when on `PATH`, else `builtin`. `tp config` shows `commit_strategy_effective`; `--resolved` shows `{value, source}`.

A task closed with commit(s) records `commit_shas` (ordered; `commit_sha` mirrors `[0]`). `tp done --commit` is repeatable; a duplicate sha exits 1. **During self-development, tp's own repo runs `commit_strategy: auto` with `hc` installed → the hc close flow: implement → `hc run` (code) → `tp done --commit <sha> …` → `hc` (task-file closure).**

**`tp keep`** — the durable, git-ignored (`.tp/local.json`) memory of files kept uncommitted, so `tp resume` classifies them as `kept` not `changes`:

```bash
tp keep <path> "<reason>"   # add/update (a repeated path overwrites; filepath.Match globs, no **)
tp keep --remove <path>     # drop (an absent path is a no-op, exit 0)
tp keep --list              # print as JSON ([] when empty)
```

Paths store repo-root-relative from any subdirectory. Feed `tp resume`'s `kept[].path` into `hc`'s `allow_unplanned`. After a close, `tp done`/`tp close` warn on stderr about any unexplained change not on the keep-list (exit 0; tp never commits or discards it).

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
- **Convergence criteria differ by phase (v0.28.0+).** A **spec review** is converged only when a counted round surfaces **no critical/blocking findings**: never declare review convergence or accept a round cap while a critical finding is open (low/medium findings may be accepted with recorded justification once no critical ones remain). An **implementation audit** always runs to the full **2 consecutive clean rounds and is never cut short by an early cap** — a hit `audit_max_rounds` means fix the findings and continue (with a user-approved cap raise), never ship with them open. Implementation correctness is not negotiable.

## Class & Checks Guidance

- **Fill `class`** on a review finding when it is an instance of a pattern a script could check across the whole corpus (example: `code-citation-drift`); omit it otherwise.
- **Mechanization candidate:** a class that appears in ≥ 2 distinct rounds OR ≥ 5 times in a single round (`tp review --report` and `--record` output list `mechanize_candidates`). When one appears, write a detector command and register it: `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'`.
- Once registered, tp runs the check every review round, reports pass/fail in `mechanical_checks`, and tells reviewers to stop reporting that class. `tp review --status --check` requires every check to pass before exiting 0.

## Role Corpus & frontmatter overrides (v0.25.0)

Review and audit roles are **project-owned data** — one JSON file per role under the repo-root `.tp/`:

- `.tp/reviewers/*.json` drives `tp review`; `.tp/auditors/*.json` drives `tp audit` (phase = directory). Schema `{id, title, instructions, focus[], domains[]}`; `id` MUST equal the filename stem (lowercase kebab-case); `regression` is reserved. Commit the corpus to VCS.
- A populated phase directory **replaces** the embedded default corpus for that phase; absent/empty keeps tp's curated defaults (software: implementer/tester/architect + spec-coverage/security/maintainability-conventions; prose: coherence/soundness + spec-coverage/soundness). A project happy with defaults keeps **zero role files**.
- `tp init --eject-roles [--domain software|prose] [--force]` writes the defaults as editable, byte-identical files; an unknown `--domain` is a usage error (exit 2). tp's own repo dogfoods a custom 4+4 corpus (adopted v0.26.0: implementer/tester/architect/ax-economist reviewers + spec-coverage/go-safety/maintainability-conventions/ax-contract auditors); a project happy with the embedded defaults still keeps zero role files.
- `tp lint`, `tp review`, and `tp audit` validate the corpus; a malformed role file aborts that phase with **exit 3** and a `repair or delete <path>` hint (a broken auditor never blocks review).

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
- `tp.review_roles`/`tp.audit_roles` **append** focus to an existing role (project focus first); an unknown override id is a lint warning; `regression` takes no overrides. The standalone `tp: lens` is retired — a legacy `lens` auto-translates to review-role focus with a deprecation warning (the new form wins when both are present).

**Per-role overlap report** (`tp review --merge`/`--report`/`--status`): findings cluster by `(location, class)`; `overlap_report` lists each reviewer role's `unique`/`shared` cluster counts and flags a `trim_candidate` (`unique == 0 && shared >= 1`) — a reviewer that found only what others also found. tp reports; you decide whether to trim a role file. `mechanize_candidates` is unchanged.

**Role staleness**: each recorded round stores a per-phase `roles_hash` (`"builtin"` on the defaults); `--status` reports `roles_stale` beside the spec `stale` flag when the corpus changed since the last round (a single re-confirming round clears it). A pre-v0.25.0 round with no stored hash is treated as matching.

## Role authoring guidance (v0.25.0)

Opening role authoring is a power feature — a project-authored role is only as good as its prompt. Design each role for **high-signal, low-overlap, contract-conformant** findings:

1. **One distinct failure-lens per role.** A role must target a failure mode no other role covers. Overlapping roles waste tokens and get flagged as trim candidates (`overlap_report`) — diversity of lenses beats count.
2. **Adversarial framing.** "Try to refute this / find where it breaks / enumerate every X and verify each" outperforms "check whether this is fine". LLM reviewers have a leniency bias and underweight negation, so tell the role to actively hunt flaws and test the spec's "DO NOT" constraints.
3. **Evidence demand.** Every finding carries a `location` (a `§`-anchor) and a why — this is what makes dedup (the overlap report) and audit PASS/FAIL meaningful. A finding with no location is unverifiable.
4. **Scope boundaries.** State what the role does NOT cover (name the sibling roles' territory), so the panel tiles the problem space with disjoint lenses.
5. **Output-contract adherence.** The role customizes only its `focus`, never the finding schema — tp injects the fixed contract (`role, location, class, severity`, plus `status` for audit).

**Worked example role sets:**

- **code/software** — *correctness* (does the change actually work: error paths, edge cases, happy-path gaps), *security* (trust boundaries, injection, unsafe defaults, unpaired locks), *performance/contract* (backward-compat, complexity, interface consistency). The ejectable `implementer`/`tester`/`architect` reviewers and `spec-coverage`/`security`/`maintainability-conventions` auditors are worked examples of this guidance.
- **prose** — *narrative continuity* (coherence: does one part contradict, duplicate, or pre-empt another?) vs *expository derivability* (soundness: can each claim be derived from what precedes it without inventing facts?). Prose defaults to the leaner two-reviewer panel because prose flaws surface from many angles at once.

**Other domains** and their characteristic diverging lenses (for custom corpora): **legal/contract** — obligation completeness vs. ambiguity/loophole; **product/PRD** — user-journey completeness vs. measurable acceptance; **data-schema** — referential integrity vs. migration/compat; **academic** — claim support vs. methodology soundness.

The embedded default corpus is authored to exemplify this guidance, so an ejected default role is itself a worked example — run `tp init --eject-roles` to read them.

## State directory (`.tp-review/`)

- `tp` owns the review/audit round lifecycle in `<spec-dir>/.tp-review/<spec-base>/` (`state.json`, `snapshot-round-<N>.md`, `review-round-<N>.ndjson`, `audit-round-<N>.ndjson`).
- **Commit `.tp-review/` to version control.** Import convergence enforcement holds across clones and CI only when the recorded rounds travel with the repo. `state.json`, every round NDJSON, and the newest snapshot are load-bearing.
- **Prunable:** only snapshot files older than the newest MAY be deleted (the diff falls back gracefully).
- **CI implication:** ignoring the directory makes every `tp import` in CI behave as "no recorded rounds" (import proceeds with an info line) — convergence is then unverifiable. A corrupt or index-less directory aborts state-reading commands with exit 3 and a repair hint; tp never silently rebuilds the index.

## Tail protocol (when a round drops to one or two low/medium findings)

1. **Verify disputed findings:** route each through `tp review --verify <spec> --findings all.ndjson`. A verifier-rejected finding is resolved `wontfix` with the verifier's reasoning **and written into the findings file before `--record`** (the round entry never recomputes) — a round whose surviving rows are all pre-resolved `wontfix` records as clean.
2. **Class-sweep:** derive the class of each surviving tail finding and run one exhaustive class-sweep prompt per class ("enumerate every `<pattern>` in the spec; verify each") before the next counted round, so a single class cannot drip one finding per round. The class-sweep is uncounted.

## Migration to v0.23.0 audit (schema break)

`tp audit` JSON is a **clean break** from v0.22.0 — downstream consumers (sub-agents, scripts) MUST update; there is no `--legacy-format` flag:

- `prompts[].role` is now one of `spec-coverage` / `security` / `maintainability-conventions` (was always `implementation-auditor`).
- `prompts[].category` is **removed**.
- `prompts[].prompt` is structured (Role → Role Rules → Spec Excerpt → Project Context → JSON-array Checklist → Affected Files → Output Schema), not paragraph text.
- `prompts[].checklist_items` (array of `ChecklistItem`) and `prompts[].affected_files` (`{path, tasks, diff_summary}`) are new.
- Sub-agent output is NDJSON, one row per checklist item (`item_id`, `status`, `evidence_file`, `evidence_lines`, `category`, `severity`, `notes`, optional `class`).
- `--affected-files` survives (replaces the diff universe before per-role filtering); `--findings` survives (`finding` items route to `spec-coverage`).

## Project configuration (`.tp/config.json`)

Multi-spec repos keep **one** workflow policy in a repo-root `.tp/` instead of copying it into every `<base>.tasks.json` (tp's own "derive, don't maintain a parallel list" principle applied to policy):

- **`.tp/config.json`** (commit to VCS) — shared workflow defaults: `quality_gate`, `review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `review_max_rounds`, `audit_max_rounds`, `checks`. A task file's `workflow` block then holds only **explicit overrides**; effective values resolve **at read time** (precedence: CLI flag > env > task-file override > `.tp/config.json` > built-in default). Absent ≠ zero; `checks` uses replace semantics.
- **`.tp/local.json`** (git-ignored automatically) — the `active` task-file pointer (written by `tp use`, the sole active-file mechanism since the `.tp-active` marker was removed in v0.25.0) and CLI flag `defaults` (`compact`/`quiet`/`no_color`). Negating flags (`--no-compact`/`--no-quiet`/`--color`) override a default for a single run.
- Discovery walks up from the CWD to the `.git` boundary to find `.tp/` — a single deterministic anchor the agent never disambiguates.

Commands:
- `tp config` / `tp config --resolved` — effective settings; `--resolved` annotates each with its `{value, source}` layer (so the agent can see *why* a value is in force).
- `tp config --extract [--dry-run|--force]` — hoist policy shared by ALL task files into `.tp/config.json`.
- `tp set --workflow --project <field>=<value>` — edit a project-level workflow field; `tp set --local defaults.<flag>=<bool>` — set a flag default.
- `tp validate --project` — report cross-spec workflow drift (informational; `--strict` → exit 1).

## Reference

For command details, field aliases, NDJSON format, and batch operations: see [REFERENCE.md](REFERENCE.md)
