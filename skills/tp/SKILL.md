---
name: tp
description: Spec-to-task lifecycle manager for AI coding agents. Interviews user to resolve ambiguities, decomposes specs into atomic tasks, manages execution order via dependency graph, and batch-closes with evidence. Use when user wants to implement a spec, plan tasks, decompose a feature, a *.tasks.json file exists, or user says 'continue with tp' to resume work after a context clear.
---

# tp — Task Plan Skill

Activates when: a `.tasks.json` file exists, user asks to implement a spec/plan/tasks, or user references tp commands.

> **English only — no foreign-language trigger seeding.** This skill is written in English. Trigger matching is semantic, so a user typing the resume intent (or any other trigger) in any language activates the skill. Do not seed literal foreign-language phrases — they add no value and violate the repo's English-only convention for committed artifacts.

## "continue with tp" — zero-friction resume (any project)

A short resume intent — "continue with tp", "resume tp" — is a sufficient standalone prompt in ANY tp project after a context clear. `tp resume` rebuilds the full picture from durable state (task file, spec, `.tp-review/`, `.tp/local.json`, git); nothing load-bearing lives in the context window. On the trigger:

1. **Orient**: `tp resume` → `{phase, blockers, next_action}`.
2. **Clear agent-clearable blockers**: `unexplained-changes` → commit the changes via the project's commit tool, or `tp keep <path> "<reason>"` for intentionally-uncommitted files; then re-run `tp resume`. Every other blocker is class `escalate`: `no-ready-task`, `epic-blocked`, `review-carried-blocking`, `review-budget-exhausted`, `audit-budget-exhausted` and `spec-stale`. Of these, `spec-stale` carries agent-runnable commands in `data.commands` (emit the review round, then `--record-inbox` it) and `review-carried-blocking` names the `--resolve` that clears its row (accepting a critical or high row without a change stays the operator's); run those and re-run `tp resume`. The rest, an empty `data.commands` and everything under `data.operator` are the operator's: stop and report the blocker's code, its message and the exact operator command its data or hint names.
3. **Work ONLY the reported phase** — do not run the next phase in the same session:
   - **review** → Workflow A Step 2 (review loop until `tp review <spec> --status --check` exits 0 — converged, or at the round cap with every finding of the latest round dispositioned; `wontfix`/`duplicate` are terminal and may be set pre-record, `fixed` implies a spec change and forces a re-review). An `ungrounded` key in `tp review`'s envelope says the latest ground round left floor units undispositioned; a spec never grounded shows no key, and no phase reports either — so on the Full track run the ground loop (Step 1.5) before the counted rounds whether or not the key appears.
   - **decompose** → Workflow A Step 3 (atomic tasks: 1 verb, ≤8-word title, ≤3 acceptance, `source_sections` for EVERY task; backward-pass every table row + numbered list item into some task's acceptance; `tp validate` until clean; `tp import tasks.json` plain, no `--force`).
   - **implement** → Workflow B/C (one fresh subagent PER task; strictly serial work tells it to run `tp next --brief`, and when more than one unit runs at a time — a pool or worktree flow — each spawn's prompt holds `tp brief <id>`'s output, whose marker claims the task; the brief carries the close recipe, so inject only runtime setup and live lessons; a task an agent spawn holds is not resumed — `tp resume` points at `tp pool --session <session>` or `tp land` instead; close per `commit_strategy`: `hc` → `hc run` then `tp done <id> --commit <sha>`; `builtin` → `tp done <id> --auto-commit -- "<evidence>"`; `tp commit` alone commits and records the sha but leaves the task `wip`).
   - **audit** → Workflow D (until `tp audit <spec> --status --check` exits 0).
   - **release** → report and stop.
4. **Stop at the phase boundary**: print (1) what changed on disk, (2) the next `tp resume` next_action. The driver — a human who clears context and re-sends the trigger, a coordinator session, or `tp run` — hands the next phase to a fresh unit.

This is the reset-native contract: `tp resume` is the single source of truth between context resets.

## Triage: pick the track first

**Triage every piece of work before starting it; the operator confirms the track.**

| Track | The work is | What runs |
|---|---|---|
| **Fix** | a defect with a reproduction | Workflow F |
| **Decision** | a choice between alternatives with user-visible consequences | a note of at most two pages — the decision, the alternatives, the non-goals — with the change built in a clone first; then Workflow A without the ground loop, 2 reviewer roles (`enabled: false` on the rest) and at most 2 review rounds (`review_max_rounds=2`). A finding about detail the code will settle is deferred to task acceptance |
| **Full** | a new capability too large to build in a clone first | Workflow A with the ground loop, `review_max_rounds`/`audit_max_rounds` set at `tp init`, and the spec's growth checked between rounds (a registered check that fails when the spec outgrows its round-1 snapshot) |

On both spec tracks a repair to the spec after its first round deletes or narrows; it does not add
requirements.

## Workflow F: Fix (a defect with a reproduction)

The input is a reproduction: a `BUGS.md` entry, a field report, a red test. No spec, no ground, no
review rounds.

1. Write the failing test from the reproduction; run it and watch it fail for the stated reason.
2. Fix.
3. Run the quality gate.
4. One review of the diff.
5. Commit.

A fix that turns out to need a design choice stops and becomes a Decision.

## Workflow A: Decision or full track (spec exists, no .tasks.json)

Order: **interview → write the spec body → `tp lint` → `tp init` → `tp set --workflow` → ground loop (Full track only) → review loop → decompose → `tp import`**.

Running `tp init` **before** the review loop creates the spec-adjacent task file that supplies the loop's workflow parameters (convergence counts, round budgets, checks). The quality gate is authored once at the project layer with `tp set --workflow --project quality_gate="<cmd>"` (writes `.tp/config.json`), not with `tp init --quality-gate`: a task-file override masks the project gate, so a later change to the project gate never reaches the task files carrying one. A spec that genuinely deviates sets its own with `tp set --workflow quality_gate="<cmd>"` (attended only; `quality_gate=` removes the override). Switching a spec's gate off is `--skip-gate` with a recorded reason, never a blank value.

### Step 0: Interview

Before writing or editing a spec, resolve all ambiguities:

1. **Locate material** — read draft spec (if provided) or ask user to describe the problem.
2. **Explore codebase** — read CLAUDE.md/README and affected files. Limit to files directly referenced.
3. **Identify ambiguities** — list all unclear, under-specified aspects.
4. **Ask one at a time** — for each ambiguity, ask one question. Derive follow-ups from answers.
5. **Prefer codebase** — if answerable by reading code, explore (≤5 files) instead of asking. Architectural/product decisions always go to user.
6. **Recommend answers** — provide a recommended answer for each question based on codebase context.
7. **Handle non-answers** — if user says "skip"/"whatever"/empty, accept recommended answer.
8. **Termination** — complete when every decision with user-visible impact (CLI output, file format, command behavior) is decided and no new questions arise.

Then collect workflow parameters (hold in memory until `tp init` / `tp set --workflow`):
- Quality gate command (e.g. `"go test -race ./... && golangci-lint run"`) — authored at the project layer in Step 1. For Go, run the suite under `-race` (a data race cannot fail a gate without it), and note that `golangci-lint run` (v2) checks `gofmt` only when a `formatters:` section enables it.
- Consecutive clean **review** rounds (default 2) — integer 1-10; re-ask once if invalid, then use default.
- Consecutive clean **audit** rounds (default 2) — same rules.
- Round budgets `review_max_rounds` / `audit_max_rounds` (default 3; an explicit 0 = uncapped) — after that many counted rounds a loop ends by disposition rather than by clean rounds (Gate, Budget & Escalation Policy).

Announce: "I will review until N clean rounds, audit until M clean rounds." If new ambiguities arise during spec writing, pause and return to step 3. Do not re-ask parameters.

### Step 0.5: Writing the spec body

`tp lint` checks form and the ground loop checks claims; neither writes the spec. Each rule below is
answerable while writing, so none costs a grading round.

- **The body is ADR-shaped**: decision, why, consequences, a link to supplemental material. The
  decision must stand without the material.
- **A sentence that would change if the implementation changed is not spec.** What stays is a
  decision, a rationale, a non-goal, or an acceptance row observable from outside. Function names,
  packages and exit-code branches for behaviour the release creates go to the implementing task.
- **A sentence about behaviour the release does not change is a test name, or it is not in the
  spec.** What the tool *already* does is a claim the author must have run; its explanation belongs
  in the project's reference docs.
- **A number does not live in a spec; a reference does.** Derivations go to
  `<base>-measurements.md`, which grounding does not grade. A count of a named artifact's items, or a
  number that *is* the decision (an exit code, a round count), stays.
- **Every fenced command runs under `bash -c '<block>'` at `HEAD` and prints something** — run each
  block before the round. Name the binary meant when it is not the `tp` on `PATH`, and record the
  `tp --version` the output came from.
- **A test row's mutant and its two counts belong in the implementing task's acceptance** —
  Decomposition Rules, rule 8.
- **A grading brief names the record and repeats no figure from it**; a check prototype says which
  hits it read one by one and which it judged by class.
- **The reviewed document is a decision record.** Review judges the decisions; the audit judges the
  behaviour, on code.
- **A repair subtracts before it adds**: after the first round it deletes or narrows, never adds a
  requirement; a new behaviour sentence it must write is run before committing.
- **Citations rot when what they name moves**: register a check that verifies them every round
  (Class & Checks Guidance) rather than trusting the convention.

A review loop ending at its cap is by design: at `review_max_rounds` the loop is done once every
finding of the latest round is dispositioned (`done_by: cap`), and open findings about behaviour the
release will create go into task acceptance.

### Step 1: Init the task file and workflow

1. `tp lint <spec.md>` — fix issues; review `structured_elements` and the `frontmatter` object, and read `floor_size`, `cut` and `review_panel`: what a round-1 grading of this spec will read, and which reviewer roles a round-1 emission would carry. They are reports, not gates — no threshold, no warning, nothing fails on them. REFERENCE.md defines each and says which text it counts.
2. `tp init <spec.md>` — creates the spec-adjacent `<base>.tasks.json` shell (zero tasks) with an empty `workflow` block. Author the gate one layer up: `tp set --workflow --project quality_gate="<cmd>"` (writes `.tp/config.json`, resolved by every task file); passing `--quality-gate` here writes a task-file override that masks it. `tp init` and `tp import` print a notice when the resolved gate's first command does not resolve (not a builtin, not on `PATH`, not an executable from the project root); the exit code is unchanged.
3. `tp set --workflow review_clean_rounds=N audit_clean_rounds=M` — convergence counts (only if non-default). `review_converge_on`/`audit_converge_on` belong here too if either is non-default: an audit round is graded under the policy it was recorded with, so `audit_converge_on` has to resolve **before audit round 1** to be worth anything (Workflow D).
4. `tp set --workflow review_max_rounds=R audit_max_rounds=A` — round budgets (only if not the default 3; `0` uncaps).
5. `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'` — register mechanical checks (see Class & Checks Guidance).

**Multi-spec repos:** put the shared gate/convergence policy in a repo-root `.tp/config.json` once (see [Project configuration](#project-configuration-tpconfigjson)) and leave each `tp init` shell's `workflow` block empty except where a spec genuinely deviates — that keeps one source of truth instead of copying policy into every `<base>.tasks.json`.

### Step 1.5: Ground loop — after `tp lint`, before the first review round

**Full track only.** Every review role is told the spec is "complete and authoritative"; `tp ground`
checks its claims against the world first, so review argues about premises that hold. It is a
command you run, not a phase: `tp resume` reports no ground phase and `tp run` schedules none.

Repeat until `tp ground <spec> --status --check` exits 0 — it exits 1 while a floor unit has no
disposition, while the floor is empty but `cut` is positive, or while the latest round holds a
`FAIL`. Each round:

1. `tp ground <spec>` — emits **one** `prompt` (not `prompts[]`: no panel, no role), writes the
   snapshot and floor, and names the inbox file the unit writes.
2. Spawn **one** sub-agent on it. `tp ground <spec> --units` prints each unit's full text. Units may
   copy what a probe needs **outside** the repository; give each concurrent unit its own `mktemp -d`.
3. `tp ground <spec> --record <file>` — one bad row records nothing.
4. Repair the spec against `FAIL` **and** `PARTIAL` rows (read `PARTIAL` from `by_verdict`: `--check`
   does not gate on it), then run the next round. `QUESTION` and `UNVERIFIABLE` do not block.

From round 2 a unit whose text did not change carries its disposition (`carried_from`) and is not
asked again, so an empty payload is legal, and an unrepaired `FAIL` carries as long as its sentence
stands. `--status` has no `next_action`; branch on `--check` and `by_verdict`. Verdicts, kinds,
tiers and payloads: REFERENCE.md, "Grounding".

**Re-emitting an unrecorded round is idempotent while the spec is unchanged** (round numbers are
consumed by `--record`). If the spec changed, the emit exits 3: record the round, or discard it with
`--force` (an operator decision under `TP_UNATTENDED`, `discard-emission`). `tp review` and
`tp audit` behave the same; `tp epic review` spells the discard `--discard`.

### Step 2: Review loop

**A spec that changes a validation rule, a gate or a record schema is sized by the fixtures it
breaks.** Before the first counted round, make the minimal change in a `git clone --no-hardlinks`
outside the repository and run the suite; count `--- FAIL:` lines and `[build failed]` packages
separately. If fixtures break, the fixture migration is its own task, and the rule change depends on
it.

Repeat until `tp review <spec> --status --check` exits 0:

1. `tp review <spec>` — auto-numbers the round, snapshots the spec, and injects previous findings
   and the changed-sections diff. From round 2 a **regression** prompt is added when the spec
   changed; process it first.
2. Spawn one sub-agent per prompt; each writes NDJSON findings to its `output_path` (an empty file,
   or a line holding only `[]`, when it finds nothing). **Every row needs `severity`, `finding`,
   `location` and `evidence`, non-empty**; `--record` refuses the whole file otherwise. Row contract:
   REFERENCE.md, "Finding `class` and Report".
3. `tp review <spec> --record-inbox` — records the round from every role's file and refuses (exit
   3) while a role has none. It is the only way to record an emitted round; `--record <file>` is for
   a round nobody emitted. If you merge role files to read them, use `-o` (never `>`) and `&&`, and
   never record a merged file of an emitted round.
4. Fix the spec, and dispose of every finding in **the recorded round file** `--status` names
   (`review_rounds[].file`, not a merge output): `tp review --resolve <file> <idx> fixed|wontfix|duplicate "evidence"`
   (0-based), or `--resolve-all <file> <disposition> "evidence"` for many.
   - `fixed` does not clean its round; the next recorded round verifies it (so a row arriving at
     `--record` already `fixed` aborts the record). `wontfix`/`duplicate` with evidence do clean it.
   - **A finding leaves a round as a spec change or as a disposition, never as new prose.** When the
     answer is *why the text is already right*, it goes in the `wontfix` evidence: prose written to
     satisfy a reviewer becomes the next round's surface.
   - Under the default `review_converge_on=blocking` a round is clean when no critical or high
     survives; accept surviving medium/low as `wontfix` under one justification. Accepting a critical
     or high without a change is the operator's (`accept-finding`).
5. When a fix batch touched **more than 3 sections**, run `tp review <spec> --perspective regression`
   as an uncounted check before the next counted round.
6. Repeat. `--status` reports the verdict (`done`, `done_by`: `converged`, or `cap` once the cap is
   reached with every latest finding dispositioned) and `next_action`; its other fields are in
   REFERENCE.md, "State Directory".

**A blocking finding nobody fixed or accepted is never converged away.** Later rounds are told not
to re-report it, so the loop stays open while the carried set holds an unresolved critical or high
(`carried_blocking`, `carried_blocking_rows`); a blocking `fixed` counts once a later round is
recorded. `next_action` names the way out.

**A rewritten spec starts an epoch**: after a rewrite (not a repair), `tp review <spec> --rewritten
"<evidence>"` stops rounds of the old text feeding carried findings in both phases. An epoch that
cuts an unresolved blocking row is the operator's `rewrite-past-blocking`. Details: REFERENCE.md,
"Epochs".

**Convergence is a recorded fact, not a judgment.** Do not skip rounds, call findings "minor", or
declare convergence before `--status --check` exits 0. Counted rounds are full-panel.

### Step 2a: The order inside a round

**Grade → record → repair → emit.** Findings judge one text; once it changes they cannot be recorded
against it. Do not delete a round's findings file by hand, and copy the tree only after the grading
unit has finished.

### Step 3: Decompose and import

1. Decompose into tasks — **you are the decomposer, tp validates your output.**
2. Backward pass — every table row and numbered list item → some task's acceptance; `tp validate` for line coverage.
3. `tp import tasks.json` — **plain, no `--force`.** The init shell holds zero tasks, so no overwrite flag is needed and the convergence checks stay armed: a review loop with recorded rounds that is not done, or a registered review check that fails, blocks the import (exit 1). A loop the cap ended passes. `--force` (overwriting real tasks, skipping those checks) is an operator decision.

### Decomposition Rules

**You are the decomposer — tp validates your output.**

1. **Atomicity**: Each task = 1 commit, 1 verb, 1-15 min estimated
   - ≤3 acceptance criteria, ≤8 word title (no conjunctions), ≤2 source_sections
   - `estimate_minutes` is required on every task: `tp import` and `tp add` exit 1 without it
   - If >3 criteria, split by concern axis
2. **Concern axes** for splitting: types/models → logic/engine → validation → CLI/wiring → tests → docs
3. **Structured elements** (from `tp lint`): every table row, numbered list item, code block → some task's acceptance
4. **Example tables are the recommended acceptance shape for behavioural tasks**: `input → expected output` rows taken from a table the spec already carries. A value fixed before the code exists is what keeps the test from being tautological; a value read off the code passes the gate just the same.
5. **Source anchors**: every task MUST have `source_sections` (canonical headings, e.g. `"## 4. Backend Migration"`). `source_lines` (`"15-42"` or `"15-42,50-60"`) is **optional precision** — sections are the primary anchor because line numbers die on every spec rewrite while heading anchors survive. A task with neither anchor is a validation error. When `source_lines` is absent, `spec_excerpt` (in `tp plan`/`show`/`next`/`next --peek`) is assembled from the section text — so the mandatory anchor also carries the spec content and removes a round trip.
6. **Dependencies**: types before logic, logic before CLI, CLI before tests
7. **A change and the test it invalidates belong to the SAME task.** The quality gate runs at every `tp done`, so "a later task fixes that test" is impossible — the earlier close is already red. A dependency graph that separates them looks correct and cannot be executed. Decompose by *what the gate sees*, not by concern.
8. **A test task's acceptance names its mutant and both counts.** Write the mutant before the assertion, and drop the test if the mutant cannot make it red. The acceptance quotes the fixture and **both** counts — the value at `HEAD` and the value under the mutant — because a mutant paired with the wrong fixture reads exactly like one that works, and the two counts are the only place the difference shows. They are confirmable only once the code lands, which is why they live here and not in the spec.
9. **`touches` keeps a parallel pool clean.** Under `run_max_parallel` above 1 a free slot takes a ready task whose `touches` overlap no running task's; on a clash the unit that integrates second is discarded and rerun alone. `touches` is a JSON array of globs from the project root (`path.Match`; `dir/` or `dir/**` for a subtree): `"touches": ["internal/engine/lock.go", "internal/engine/lock_test.go"]`. Name the files the commit will change, tests included. A wrong one costs a rerun, never a broken branch.
10. **Preview before import**: list proposed tasks and ask for confirmation

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
#   builtin: tp done <id> --auto-commit -- "evidence"   (tp commit <id> only commits and records the sha; the task stays wip until tp done)
#   hc:      commit with hc, then tp done <id> "evidence" --commit <sha> [--commit <sha> …]
# Batch close: tp done --batch results.ndjson  (each row carries commit — a sha or an array — or covered_by)
```

**The quality gate runs automatically at `tp done`** (and `tp close`): when `workflow.quality_gate` is set, closing a task runs the command once per invocation; a failing gate blocks the close (exit 4) and no task closes. A gate that exits 126 or 127 names the command it could not run and hints to fix the gate command, never `--skip-gate`. There is no `--gate-passed` step to perform — the flag is ignored when a gate is configured. On a gate-less project, `--gate-passed` still records an attestation.

A task audit that falls due mid-implement ("Task audits first" in Workflow D) runs as its units before the next implement unit. After all tasks are done, stop at the phase boundary: `tp resume` then reports `audit`, and the next unit runs Workflow D.

## Workflow C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first. A wip task an agent spawn holds (`claimed_by.spawn`) belongs to that agent: `tp next`, `tp brief` and `tp resume` do not offer it to resume, and `tp pool` counts it against `run_max_parallel`; when agents hold every task that could run, `tp next`/`tp brief` exit 4 naming them under `agent_held`, and `tp resume` reports no blocker, its next_action `tp pool --session <session>` (or `tp land` when the claim has no session). A wip task without a spawn claim is an interrupted unit and is resumed first. The phase boundary applies equally.

## Workflow D: Audit loop (convergence via record + status --check)

Repeat until `tp audit <spec> --status --check` exits 0:

**Task audits first (v1.22.0).** When `task_audit_roles` lists code roles, each closed task owes a task audit of its own files, in batches of at most ten files (REFERENCE, Task Audit). While one is owed the audit does not converge and `--status` names `task_audits_owed` (or `task_audits_unknown` when the task file cannot be read). Run each: `tp audit <spec> --task-audit` emits the batch due (null when none), one sub-agent per prompt writes its `output_path`, then `tp audit <spec> --task-audit --record-inbox` records it. A full batch is due mid-phase, the last partial one once every task is done. Its rows join the carried set below; a FAIL is fixed and `--resolve`d `fixed` in its `task-audit-<seq>.ndjson`, which makes one re-grade due before the next release round. `tp run` schedules all of this itself.

1. `tp audit <spec>` — emits one prompt per active auditor role from the corpus (defaults: `spec-coverage`, `security`, `maintainability-conventions`) with an embedded JSON-array checklist and its affected files. **spec-coverage is the only auditor id that changes routing** — it alone takes the spec-derived checklist and its own file selection; every other role, built-in or user-defined, receives one `file_check` item per file over the same shared code-file list: ranked by churn and cut to ten when auto-detected, whole when named. Each cut, and a universe that looks wrong (merge-only files, files no task touched), is named on stderr and in `notices`. Auto-detects changed files via git diff; `--affected-files` overrides, or `--affected-from-tasks` audits exactly the files touched by done tasks' `commit_shas`. Either named form goes uncut into each code role's one prompt, and a long one is read shallowly: in a measured trial a role given 121 files graded every one by pattern-scanning and missed most of what ten-file prompts found on the same files (v1.21.1, `named-list-one-prompt` in `notices`; `spec/1.21.1-measurements.md`). `graded N of M` counts rows returned, not files read. When no audit-able file is found it exits 4 with `suggested_files` (the same commit-derived list) and a hint. On a committed tree — the normal end of implement — `tp audit <spec>` and `tp audit <spec> --role <r>` find no changed files and exit 4 (`no changed files detected`); re-run them with `--affected-from-tasks`.
2. Spawn one sub-agent per role prompt; each returns one NDJSON line per checklist item (`status` ∈ PASS/PARTIAL/FAIL).
3. Merge the per-role files: `tp audit --merge r1.ndjson r2.ndjson ... -o results.ndjson` (no spec positional — `--merge` takes NDJSON inputs only and rejects a spec with exit 2). Rows sharing `role`+`item_id` collapse only when their verdict agrees; disagreeing rows are all kept and named under `conflicts`, with a stderr warning. The payload carries a status/role breakdown and the same per-input `inputs` accounting, exiting 1 on an input whose content rows all failed to parse. Then record: `tp audit <spec> --record results.ndjson` — a row counts as a finding when `status` is absent or ≠ `PASS`, under either `audit_converge_on` value; whether a round holding findings is nonetheless **clean** is that field's answer, not the finding count's (see below). The record also stores what each code role graded of its pool (`graded`, "graded N of M" in `next_action`). The audit round sequence is independent of review rounds.
4. Fix the code for every non-PASS item, then record how each was closed in the recorded round file `--status` names (`audit_rounds[].file`): `tp audit <round file> --resolve <0-based index|role:item_id> <fixed|wontfix|duplicate> "<evidence>"`. A `role:item_id` selector disposes every non-PASS row under the key and reports how many as `disposed`; `--resolve-all` disposes every undisposed non-PASS row and skips PASS rows; `--force` overwrites a disposition already there. A resolve into a file no round lists changes nothing and says `recorded_round: false`. **An evidenced `wontfix` or `duplicate` clears its round**, live on `--status` and `--check`; `fixed` does not — the next round re-reads the repair. **A non-PASS row stays carried until a later round grades it PASS or it is accepted (v1.18.0):** empty rounds do not clear it, so it holds convergence open, at the cap too, and every later prompt's Prior Round block lists it, a row older than the previous round with the round it was graded in. The acceptance lasts: the Prior Round block shows each row's disposition, and the next `--record` carries it onto the same `(role, item_id)` non-PASS row while the row's `evidence_file` is unchanged (reported as `carried`; `fixed` is never carried). `--status` names carried rows as `carried_blocking_rows`, and `next_action` how each clears (REFERENCE, Audit convergence). A `wontfix`/`duplicate` with blank evidence exits 2. Accepting an audit finding without a code change is the operator's decision: under `TP_UNATTENDED` it exits 2 and escalates as `accept-finding`, while `fixed` stays unfenced.
5. Repeat. `tp audit <spec> --status` shows `consecutive_clean`, `converged`, `done`/`done_by`, `stale`, `max_rounds`/`rounds_remaining`/`budget_exhausted` (`max_rounds` null and `budget_exhausted` absent only when uncapped), `in_flight_round`, `epochs` (present only once a `tp review --rewritten` epoch exists), and (with `--merge`/`--status`) an `overlap_report` — the audit-side signal for trimming a redundant auditor; with task audits, `task_audits` (the count recorded), `task_audits_owed` (task ids) or `task_audits_unknown` (an unreadable task file's path), `task_audits_nothing_to_read` and `task_audit_roles_outside_panel` (v1.22.0).
6. **Read the divergence signal (v0.33.0).** `tp audit <spec> --status` and `tp audit <spec> --record <file>` also carry `role_streaks` (each role's `consecutive_clean`/`open` in the latest recorded round — the streak lengths show whether the remaining findings are backlog or a regression), `spec_coverage_clean_rounds` (that role's streak, or `null` when the latest round measured no conformance at all — `null` ≠ `0`, so an absent `divergence` beside it proves nothing), and `divergence`, emitted only when spec-coverage has reached the clean-round threshold while other roles still hold open findings. All three survive `--compact`; the emission conditions and field shapes are in [REFERENCE.md](REFERENCE.md).

   **`divergence` gates nothing — it is reporting only.** Emitting it changes no convergence arithmetic, no stored per-round `clean` flag, no `next_action` and no `--status --check` exit code: audit convergence counts every non-PASS row the resolved policy does not accept, less the rows accepted `wontfix`/`duplicate` with evidence — so `--check` still exits 1 and `next_action` still names the next audit step. When `divergence` appears, **surface it and stop** — accepting findings outside spec conformance is a user-approved decision, never the agent's.

**Scope the audit, or it will not converge.** Neither value of `audit_converge_on` reads whether a
row is about the spec or about the codebase at large: under `all` every non-`PASS` row counts against
convergence, and under `blocking` a general lens's `error` counts exactly as a spec-coverage `error`
does. General lenses (`go-safety`, `maintainability-conventions`, `ax-contract`) always find
*something* in a real codebase, so there is no fixed point, and every repair round creates fresh
surface for the next. **Watch `spec-coverage` separately** — tp reports it as
`spec_coverage_clean_rounds` and names the split as `divergence`. Once it is clean for two rounds and
no round has ever produced a FAIL, findings outside the spec's surface are backlog, not a release
gate: the operator accepts them with `tp audit <round file> --resolve <role:item_id> wontfix
"<evidence>"`, naming the version that will take them, and ships. Keep audit repairs minimal for the
same reason — a repair that introduces a new abstraction is the next version's task, not this
audit's.

**Run the round cheaply — four brief rules, none of them tp code.** A role's minutes buy probes —
clones, mode matrices, mutants, repetition loops — and the probes are what find every `FAIL`, so the
saving is in not repeating them, never in cutting them.

1. **One clone, one binary, built by the orchestrator before the role stage.** `git clone
   --no-hardlinks` HEAD once, `go build -o` inside it once, and hand every role the path with "do not
   clone, do not build". A role that wants a mutant copies that tree with `rsync`.
2. **One repair unit per independent item.** Two items share a unit only when they edit the same lines
   of the same file.
3. **Delta re-grade.** On a round with no repairs, the brief lists the previous round's `PASS` rows and
   the role re-records them verbatim — `evidence_file` and `evidence_lines` carried — and re-measures
   only the non-`PASS` rows. On a round after repairs, only rows whose `evidence_file` is untouched by
   `git -c diff.external= diff --name-only <previous record sha>..HEAD` are carried, where the
   previous record sha is the commit that first added the round file, followed across renames
   (`git log --follow --diff-filter=A`). The orchestrator derives that list, the role does not guess
   it; tp does not derive it yet.
4. **The class-to-slug table goes into the brief before round 1.** A finding matching a class already
   routed is recorded `PARTIAL` with its slug in the note and expects no repair; only a class the table
   lacks reaches the orchestrator.

Rules 1, 3 and 4 are mechanized in tp's own repository by `scripts/audit-round-prep.py <spec>`: it
finds the previous recorded round, derives the carried rows from the record commit's diff, clones and
builds once, reads the class-to-slug table from `.tp/routed-classes.json`, and prints one brief
fragment per role. It exits 2 with nothing to carry when the previous round holds `file_check` ids of
the earlier derivation (no `.<16 hex>` digest), since such an id names no item of the next round.

### `audit_converge_on` — what an audit round has to be clean of (v0.37.0)

A workflow field taking `all` or `blocking`, resolving **task override > project config > built-in**
(there is no CLI flag and no `TP_<FIELD>` environment layer for a workflow field). Its built-in
default is **`all`** — deliberately the opposite of `review_converge_on`'s `blocking`, and the
behaviour tp shipped before the field existed, so a project that does not set it sees no change in
what any gate decides.

| value | an audit round is clean when |
|---|---|
| `all` (default) | the round has no non-`PASS` row |
| `blocking` | no non-`PASS` row in the round carries a blocking severity |

**Blocking severity is `error`** — audit rows use `error`/`warning`/`info`, a different vocabulary
from review's `critical`/`high`, and neither field is read by the other phase. **A severity tp cannot
grade is blocking, not ignored**: `severity: null`, an absent key, a non-string value, or a string
outside the enum all keep the round unclean under `blocking`. A round is graded under the policy
it was **recorded with** (stored as `converge_on`), so setting the knob governs rounds recorded
afterwards and never re-grades recorded history — and symmetrically, one approved round of
`blocking` keeps its `clean: true` after the knob returns to `all`. A disposition written later
still clears a round, live. `role_streaks[].open`, `consecutive_clean` and `spec_coverage_clean_rounds` stay
severity-blind, so under `blocking` a round can read `clean: true` beside a non-zero `open`; that is
correct — `open` counts findings, `clean` answers whether the phase may end.

An illegal literal at a write sink is a usage error (exit 2); an illegal value already stored, read
by `tp audit --record`/`--status`, is a validation error (exit 1). Both carry
`must be one of: blocking, all`.

**`blocking` is opt-in and human-only.** Its four write paths — `tp set --workflow`, its `--project`
form, `tp import`, and `tp config --extract` — are all fenced under `TP_UNATTENDED=1`, and the rule
differs by sink because the sinks differ in shape. At `tp set --workflow`, `tp import` and
`tp config --extract` a write that **changes the resolved value** to `blocking` exits 2; a write of
`all` never trips it, and an import carrying an already-resolved `blocking` forward passes. At
`tp set --workflow --project` the rule is the **value**: a `--project` write of `blocking` exits 2
whatever any single base resolves today, because it lands in the layer every base resolves through —
including bases tp cannot enumerate, such as one named with `--file` from outside the project root or
one under `vendor/`. So that refusal names no base, and a `--project` write of `all` lands even in a
tree holding a directory the scan cannot read. The other three keep the change rule because
`tp import` carries the existing block forward, so a value rule there would refuse every import an
opted-in project makes. An **attended** operator writes `blocking` at any sink
with no refusal. Under a run, if the refusal is an authoring error the unit fixes the document itself
(omit the imported document's top-level `workflow` key, or carry `"audit_converge_on": "<resolved>"`
in it); if the unit
**intends** the relax it records `tp escalate --decision audit-converge-on --evidence "<what you
found>"` and the run stops for the operator.

**`tp audit --merge` emits `by_severity`** — the round's non-`PASS` rows bucketed as
`error`/`warning`/`info`/`unrecognised` — on any round holding a non-`PASS` row, under either
policy, and it survives `--compact`. Under `blocking` it is how you see what a `clean: true` round
accepted. Both audit sinks' `next_action` names that accepted count as a numeral on the converged
branch and on the clean-but-not-converged branch, so an operator reading either learns the round
closed over open rows.

**Decide the default from the previous cycle's severity mix, before round 1.** The saving is only
reachable when the field resolves before the first round is recorded — the moment you have none of
this cycle's evidence — and `by_severity` is computed per round rather than persisted, so the
previous cycle's mix is recoverable only by counting `severity` across its `audit-round-*.ndjson`.
No flag derives it; this one-liner does:

```bash
python3 -c 'import json,sys,collections;print(collections.Counter((r.get("severity") if r.get("severity") in ("error","warning","info") else "unrecognised") for f in sys.argv[1:] for l in open(f) if l.strip() for r in [json.loads(l)] if r.get("status")!="PASS"))' <spec-dir>/.tp-review/<previous-version>/audit-round-*.ndjson
```

The glob is `audit-round-*.ndjson` on purpose: it is the recorded round, and it excludes the raw
per-role files a round may leave in `audit-inbox/` beside it, which would double-count.
Run against tp's own corpus it reports
`Counter({'warning': 22, 'info': 17, 'error': 3})` for v0.35.0 and
`Counter({'warning': 44, 'info': 32, 'error': 15})` for v0.36.0.

**Read the `error` column, not the round saving.** On those two cycles `blocking` would have
converged at round 3 where `all` took 9 and never — and it would have **shipped over an `error` row
in both**, because v0.35.0's round 4 and v0.36.0's rounds 7, 12 and 13 each carry one *after* the
point `blocking` halts. That is why `all` is the default. Setting `blocking` is choosing to gate on
the audited agent's own self-declared `severity`, which nothing on the audit path validates — take
it with the numbers above in front of you, per cycle, never as a standing repo default.

## Workflow E: Epic graph (plan above the spec, v1.3.0)

The input is a project's plan of epics, kept in `.tp/epics.json` beside `config.json` and committed
like it. The file is primary: tp lints, reviews and writes it, and `tp epic export --md` prints the
table a prose document embeds. Build it one topic at a time, each topic through five steps:

1. **Interview the topic** — its milestones, the epics in them, and for each edge *why* that
   dependency exists. `id`s stay the project's own; `title`, `acceptance` and `why` keep their
   language.
2. **`tp epic set milestones='<json>'`** — only when the topic needs a milestone the file does not
   have: the value is every track, each an ordered list of milestone names, and it **replaces the
   whole list**, so start from the current one (`tp epic export --md` or the file) and insert the new
   name. `tp epic set opens_after=<json>`, `gates=<json>` and `tag_checks=<json>` replace whole the
   same way; `gates.<m>.<field>=` edits one gate. The first `set` creates an absent file; a topic that
   fits existing milestones skips this step. An edge that must never exist — a track that may not
   reach into another milestone, say — is a `forbidden_edges` rule, `[{from, to, why}]` at the top of
   the file, each selector one of `epic`, `milestone`, `tag` or `track_of` (the track holding the named
   milestone); it has no `tp epic set` key, so it is written by hand or through `tp epic import`.
   `forbidden-dependency` then refuses every write that adds a direct edge or any-of member the rule
   forbids, naming the rule's `why`.
3. **`tp epic add --bulk <ndjson>`** — the topic's epics, one per line, every row or none. The lint
   check runs on the file as the whole batch leaves it, so rows may depend on each other in any
   order; a single `tp epic add` needs every target it names to exist already. A write whose
   after-state holds a lint error the before-state did not exits 1 and writes nothing, naming each
   error by its epic ids.
4. **`tp epic lint`** — read the warnings (`no-dependencies`, `thin-why`, `partial-parallel`,
   `gate-threshold-unwritten`, `deferrable-required`) the writers do not refuse; exit 0 means no error-severity finding.
5. **`tp epic review --milestone <m>` (or `--epic <id>`) over the touched slice** — a milestone
   slice asks every epic in it, not only the topic's, and its gate, so a topic spread over several
   milestones costs a round per milestone; for a topic, review the topic itself with
   `tp epic review --epics <ids> --name <topic>`, which keeps one slice as the topic grows. Several
   milestones emit in one call with `--milestones <m,m,...>` (or `--all`), each slice still recorded
   on its own. `--all --status --check` reads only the slices that have state, so on a graph whose
   milestones were never reviewed it passes with `slices: []`; the completion gate is
   `--milestones <every milestone> --status --check`. Spawn one
   sub-agent per role prompt, each writing its `output_path` (empty when it finds nothing), record the
   round with `tp epic review <slice> --record-inbox`, which stamps each row with its role, fix the graph or
   `--resolve` each finding before emitting the next round — while a round is out its dispositions
   are frozen, and changing one then takes the operator's `tp epic review <slice> --discard`, which
   emits nothing — and repeat until `tp epic review <slice> --status --check` exits 0.
   A class raised across slices (`tp epic review --report --all`) points at a lint rule or a
   `direct` tag check, the operator's to add.
   A gate with a threshold or a `purpose` (`tp epic set gates.<m>.purpose=<text>`) adds a `scope:<m>`
   unit that the built-in `gate-scope` role judges against both, over `tp epic scope <m>`; it is
   re-asked when that measurement's fingerprint moves, and a changed purpose re-asks the roles that
   take it, so after a gate edit read `tp epic review --milestones <m,...> --status --check` for `stale_units` and
   `purpose_stale_units` (REFERENCE.md, Epic Graph). The purpose is what scopes missing work: one
   that names what the milestone releases gets that release's obligations checked, and without one
   the role judges only against the threshold. Writing a purpose re-asks `phase-economist` over the
   whole milestone.
   Unchanged units carry from round to round, so a round after a small edit asks only what moved,
   and a re-asked unit's prompt lists its earlier `wontfix`/`duplicate` decisions so they are not
   raised again. Every round that resolves a finding `fixed` makes one more round necessary, so a cap
   of 3 leaves room for two repair rounds and one clean one; on a large slice the operator may choose
   `review_max_rounds` 4–5 before round 1 (`tp set --workflow --project`); a slice whose graph changed
   after its cap reads `grantable: true`, and the operator's `--grant-round` buys it one verification
   round rather than a raised cap for every slice. A round after one that
   fixed only non-blocking findings and left nothing open runs the regression role alone, which reads
   each change — shown before and after, with the epics it hands work to — at a quarter of a panel's
   cost. The default `review_converge_on: blocking` lets a slice finish with medium and low findings
   open (`--status` lists them under `open`); `all` makes every finding hold the slice open. **Stop
   rule:** when the finding count has not fallen for two rounds and the new findings ask design
   questions an edge cannot settle (how an operator decision is built, not which epic owns it), stop
   the loop rather than raise the cap — a larger cap will not converge it. Resolve those findings
   `wontfix` with a pointer to where the design is taken up, or leave them open, and decide outside
   the graph.
   **Descriptions** (v1.11.0). An epic may carry an optional markdown `description` — its intent,
   scope, non-goals, decisions and open questions; behaviour belongs in its spec, since text written
   here is re-reviewed on every edit. Write long text through `tp epic apply` set rows
   (`{"op":"set","id":"<id>","field":"description","value":"<json string>"}`, no shell quoting) and
   short text with `tp epic set <id> description=<text>`. A description is in its own epic's block
   only: editing it re-asks that epic, not its dependents, and `dod-satisfiability`, `why-auditor`
   and `phase-economist` read it as the epic's primary text. Every description edit is a round against
   the slice's cap, and describing a whole milestone re-asks all of it at once, so add descriptions a
   set of epics at a time and review that set as its own slice (`tp epic review --epics <ids> --name
   <set>`); read the slice's `--status` `asked_block_bytes` before emitting, and each prompt's
   `prompt_bytes` after. On a large slice, spawn one sub-agent per role and have each emit
   `tp epic review <slice> --role <id>` for its own prompt rather than reading the whole emission:
   an identical re-emission is idempotent. A description-only edit appends no regression role;
   a finding fixed through a description still gets one.

Then take the next topic. Every graph writer (`add`, `set`, `remove`, `import`, `link`, `unlink`,
`milestone add`, `apply`, its `--dry-run` included) and `--record` exits 2 under `TP_UNATTENDED`: changing the plan is the operator's,
so a unit that finds the need records it with `tp escalate --decision other`. The file's schema, the
lint rules, the state directory and the exit codes are in [REFERENCE.md](REFERENCE.md).

**Tracking the work (v1.5.0).** An epic's status is derived when read, never stored: `planned`,
`claimed`, `specced`, `in_progress` or `done`, from its bound files' `tp resume` phases or, with none,
from a done record.

1. **Read what can start.** `tp epic ready` lists the work epics that can start now, ranked by
   `downstream` — `longest_chain`, then `dependents`: start from the top. `dependents_open` counts
   every open milestone, every open track head included; for one phase read
   `dependents_by_milestone`. Claimed epics with their holder and what waits on the operator
   (conditions, gates whose threshold needs a met record) are apart; `--all` lifts the open-milestone
   rule. `tp epic status [--milestone <m>] [--open]` adds the graph's `critical_path` and, per
   milestone, `opens_after`, `waiting_on`, the counts, the gate, the blocked list, `throughput` and
   the estimate roll-up. `tp epic export` prints all of it as one JSON contract, with each bound
   file's phase and loop states: read it rather than tp's state files.
2. **Claim to reserve.** `tp epic claim <id> [--by <name>]` before you plan an epic, so a parallel
   agent sees it taken; `--release` gives it back. A blocked epic or one in a closed milestone can be
   claimed. It runs under `TP_UNATTENDED`: an agent reserves its own work. A claim says who is on an
   epic now; whose job it is, or who decides a condition, is its `owner` (v1.23.0): handles from the
   graph's declared `owners` set, written with `tp epic set <id> owner='["ana"]'` and never matched
   against `--by`. `tp epic ready --owner <h>` and `tp epic status --owner <h>` keep one owner's
   entries, `--unowned` the ones naming no declared handle.
3. **Bind when you start.** `tp init <spec> --epic <id>`, or `tp set --epic <id>` on an existing task
   file, once you write the epic's spec: binding is the decision to start it, its status follows the
   file from then on, and its claim is ignored. Write one spec per epic (a family shares a design
   note) so a later epic's dependencies never hold back an earlier one's tasks. `tp set --epic ""`
   removes the binding.
4. **The epic block.** While a direct dependency of a bound file's epic is not done, `tp next`,
   `tp next --peek`, `tp ready` and `tp claim` offer no task and carry `blocked_by_epics`; `tp next`
   exits 4, `tp claim` 1, and `tp done` refuses a task that is not already `wip`. A `wip` task stays
   resumable and closable. With no `wip` task, `tp resume` reports the `epic-blocked` blocker and
   offers no implement unit: stop and report it. When every blocker is a condition, `tp next` adds
   `operator: {decision: "epic-done", commands}` and the blocker names `tp epic done <c> --evidence`:
   that record is the operator's decision, never yours. An any-of group holds the file until one
   member is done: the payloads add `blocked_by_any_of`, one member list per unsatisfied group, the
   message names it `one of {A, B}`, and `operator` adds `choices`, the group's records as
   alternatives, any one of which satisfies it. A milestone not yet open does not hold a bound file
   back.
5. **Gates and thresholds.** A milestone opens when every milestone it opens after is open and has a
   green gate: every member done and, when the gate carries a `threshold` (the exit condition in the
   operator's words, written before the work), a met record. A gate naming no `members` holds every
   epic of its milestone that is neither a condition nor `deferrable`. `tp epic gate <m>` reports it
   and exits 0 only when green. Once the members are done, the operator measures and records
   `tp epic gate <m> --met --evidence "<text or URL>"`. List under `deferrable` the work that waits on
   a trigger that may never come, so it never holds the milestone. Edit one gate with
   `tp epic set gates.<m>.threshold=<text>` (or `.deferrable=`, `.members=`), which keeps its `met`.
   **What a gate pulls in** is `tp epic scope <m>`: the closure its members reach, the epics that
   carry most of it, and earlier gates' members no other gate needs. It writes nothing and runs
   unattended; its `fingerprint` changes only when the members, the graph's shape under them or the
   threshold do, so compare it to tell whether a scope pass is due.
   **A placeholder threshold** (`[threshold]`, `<exit condition>`, `TBD`; `.tp/config.json`'s
   `epic.threshold_placeholder` replaces the default pattern) is not one the operator can measure:
   `tp epic lint` warns `gate-threshold-unwritten` on an unmet gate carrying one, and `tp epic ready`,
   `tp epic status` and `tp epic export` leave that gate out of `operator.gates` and list it once under
   `operator.threshold_rewrites` with its `tp epic set gates.<m>.threshold=<threshold>` command. Writing
   the exit condition is the operator's; a unit reports the rewrite rather than inventing the text.
6. **Opening order.** A milestone opens after the previous one of its track by default, the first
   after none. `tp epic set opens_after=<json>` changes that per milestone: `[]` on each of a family
   of siblings keeps them one track, always open; `{"R": ["P"]}` opens R after P, in parallel with
   Q. After every `opens_after` or `milestones` write, read `ancestry_changed`: a one-key edit
   re-routes the later milestones of the track, so confirm each listed `lost`/`gained` is meant.
   `partial-parallel` names epics that depend on a milestone theirs no longer opens after; the block
   still makes them wait. It is `info` when the milestone's own entry chose that, a `warning` when it
   follows from defaults, which is the case to check.
   `dependency-opens-later` is the error on milestones that wait on each other in a cycle: an opening
   waits on the gates it opens after, a gate on each milestone a member reaches an undone epic of.
7. **Conditions.** An external precondition is an epic with `condition: true`
   (`tp epic set <id> condition=true`) whose `acceptance` says what it waits for; work that must not
   start before it depends on it. It takes no task file and no claim, and is done only by
   `tp epic done <id> --evidence "<text or URL>"`, which also closes an unbound work epic finished
   outside tp; `--undo` removes the record. `tp epic ready` asks the operator for it only when it is
   `actionable`: its dependencies done, its milestone open, and it is an explicit member of an open
   milestone's gate, or nothing depends on it, or some undone dependent, in an open milestone or with
   a bound file and not `deferrable` in its gate, waits on conditions alone, and the condition is not
   `deferrable` itself; `--all` lists the rest with `actionable: false` and `not_actionable`, every
   reason in the order `milestone`, `dependencies`, `dependents`, `deferrable`. A `deferrable` gate
   member is never listed as waiting: `tp epic status`'s and `export`'s `blocked` leave it out. An
   unsatisfied any-of group counts as a condition when one of its condition members could be
   decided now. An epic only a lost alternative needs is moot (no gate holds it, every group naming
   it is satisfied, every plain edge to it comes from a moot epic): `ready` lists it nowhere, and
   status marks it `moot: true`; it counts again once something live depends on it.
   `gate-held-by-condition` (info) names, per gate and condition outside the gate's own milestone, the
   members waiting on it, with one path.
   **A milestone that opens on something outside the plan** (a vendor signs, a date passes) keeps
   `opens_after: []` and holds its trigger as a condition epic of its own milestone, which every epic
   of the milestone with no other dependency depends on. The milestone reads open, but its epics are
   held: `tp epic ready` offers none of them and asks the operator for the trigger, and a bound file's
   `tp next` exits 4 naming it. Firing it is `tp epic done <trigger> --evidence "<text or URL>"`. To
   make lint refuse an epic that forgets the edge, tag the milestone's epics and add `tag_checks:
   {"<tag>": {"provided_by": "<trigger>"}}`. Do not put the trigger alone in a milestone the triggered
   one opens after: a gate names no condition by default, so that gate is green with no members, and
   `gate-empty` (warning) says so.
8. **Long-lead epics.** Place an epic that takes long in the earliest milestone where it can start,
   its real dependencies as edges, not in the milestone it must finish by: `tp epic ready` offers it
   as soon as they are done, and `critical_path` shows the chain it lengthens.

Under `TP_UNATTENDED` the operator's records exit 2, the removals included: `tp epic done` and its
`--undo` escalate as `epic-done`, `tp epic gate --met` as `gate-met`, and `tp init --epic` and
`tp set --epic` (an empty value too) as `epic-binding`.

## Reset-native workflow & commit strategy (v0.28.0)

Every unit — a review round, decomposition, one task, one audit round — is designed to run in a **fresh context**; tp is the durable state machine between resets. The reset is the orchestrator's job (a CLI cannot clear its caller's context); tp guarantees resumability.

**`tp resume [spec]`** — the phase-agnostic oracle. From durable state alone (task file, spec, `.tp-review/`, `.tp/local.json`, git) it returns `{phase, spec, changes, kept, bookkeeping, next_units, round, last_failure, next_action, blockers}` (plus `guidance` in the implement phase, and `running_units` beside a live rolling pool, v1.25.0: then `next_units` is empty and `next_action` carries no command, since the run starts the next units itself). Read-only. `next_units`, `round` and `phase` are the machine surface a driver parses (v0.35.0): `next_units` is the ordered `{kind, id, brief_command}` list to execute now, `round` the round they belong to (`null` outside a round-based phase), and `last_failure` the advisory `{unit_kind, unit_id, phase, exit_code, summary, at}` record of the wall the previous attempt hit, or `null` — it never changes which unit runs next. Phase is task-first: an open task reads `implement` even when the spec is stale (a `spec-stale` blocker fires, the phase does not revert). `next_action.payload` embeds the next unit's work, so a fresh agent needs one call rather than a probe across `tp status`/`tp next`/`--status`. `bookkeeping` lists the tp-owned dirty files a close legitimately leaves modified under `commit_strategy: hc` — commit them; they never count as an `unexplained-changes` blocker. A `spec-stale` blocker clears only through the round its message names — emit, run the roles, record: a `--record` with no emission of its own certifies no reading and no longer clears it (v1.2.2; since v1.12.0 also in a phase none of whose rounds was ever emitted). The field-by-field output table and the blocker vocabulary are in [REFERENCE.md](REFERENCE.md).

**The loop** — what a driver does, whether tp runs it or you do:

```
r = tp resume
for each blocker: agent-clearable (unexplained-changes) → reconcile (commit or tp keep), re-run resume
                  escalate (no-ready-task, epic-blocked, review-carried-blocking, *-budget-exhausted, spec-stale)
                    → stop, hand to a human: report the code, its message and the operator command in its data or hint
                    (next_action.summary names the blocker; command and brief_command are null; by hand, the
                    agent-runnable data.commands of spec-stale and review-carried-blocking may run first)
agents hold every runnable task (next_action.command tp pool / tp land) → land what they finish, refill the pool
running_units set (next_action.command null) → a tp run is driving the pool: stop and report; do not poll
phase decompose (next_action.command and brief_command null, no blocker) → Workflow A Step 3: decompose, tp import
blockers empty  → run next_action.command in a FRESH unit (sub-agent / session / process)
repeat until phase == release
```

**`tp run [spec]` — tp now ships that driver (v0.35.0).** It reads the cycle through the same path `tp resume` uses, stops if the cycle is releasable, takes `next_units`, spawns one runner process per unit — the three role kinds (`review-role`, `audit-role`, `task-audit-role`) concurrently, and with `run_max_parallel` above 1 a pool of implement units side by side, each in its own git worktree, integrated as it exits while its slot is refilled (rolling slots since v1.25.0; REFERENCE.md, "Implement pool"); every other kind alone; every prompt a round emits has a unit, the built-in `regression` prompt included — re-reads the state **from disk**, checks the caps, and loops. A unit's result is whatever it wrote to disk: the driver reads a child's exit code and one spend number, and nothing else it said. It exits **0** only on `stop_reason: converged` and **4** on every other stop reason, and holds a run-scoped lock at `.tp/locks/run-<key>.lock` for the whole run (`<key>` is the task file's base, or its directory and base escaped into one name, `x%2Fa`, for one below the root; v1.12.0), so a second `tp run` over the same task file exits 4. `tp run --dry-run` prints the units it would spawn without spawning anything or taking the lock (beside a live pool, none, with the pool's under `running_units`, as `tp resume` names them too); `tp run --status` reports the current or last run. The ten unit kinds, the ten stop reasons, the run state file, the child environment, the `runner` field and `notify_cmd` are in [REFERENCE.md](REFERENCE.md).

**Unattended units cannot take the user's decisions.** Every child is spawned with `TP_UNATTENDED=1`, and under it the user-only decisions fail closed with exit 2 and a hint naming `tp escalate`: `tp done --skip-gate` (and its other sinks — a `--batch` row's `skip_gate`, `tp close --skip-gate`), `tp import --force`, a `tp set --workflow` **raise** of `review_max_rounds`/`audit_max_rounds` or of any `run_max_*` cap, accepting (`wontfix`/`duplicate`) a critical or high review finding or any audit finding (`accept-finding`), an emission's `--force` that would discard an unrecorded round, and `tp epic review <slice> --discard` (`discard-emission`), and the epic operator records: `tp epic done` and its `--undo` (`epic-done`), `tp epic gate --met` (`gate-met`), and `tp init --epic` and `tp set --epic`, an empty value too (`epic-binding`), and `tp epic review <slice> --grant-round` (`grant-round`), and `tp review <spec> --rewritten` whose cut holds an unresolved blocking review row (`rewrite-past-blocking`), and setting or changing `worktree_setup_cmd` at either layer (`other`, v1.24.0); every epic graph writer escalates as `other`, while `tp epic claim` runs. An equal or lower value is accepted; `runner`, `notify_cmd` and `worktree_setup_cmd` name commands the driver executes and are refused at **every** layer rather than only above the resolved value. A change of `quality_gate` is refused too, escalating as `skip-gate` — a unit that rewrote the gate skipped it: any `tp set --workflow` write of it, and at `tp import`, `tp init --quality-gate` and `tp config --extract` a write that changes the gate that resolves. A unit that reaches such a decision records it instead of taking it — `tp escalate --decision <name> --evidence <text> [--option <text>]…` writes `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` and exits 2, the run stops with `stop_reason: escalation`, and the operator makes the decision and runs `tp run` again. Nothing is replayed, and an escalating unit is not counted as a failed attempt.

**The plugin.** tp publishes a Claude Code plugin at the repository root: `.claude-plugin/plugin.json` beside `marketplace.json`, this `skills/tp`, plus `hooks/` and `agents/`. **The Go binary is not in it** — installation stays Homebrew or `go install`, and the `SessionStart` hook preflights `tp`'s presence and version and fails with the install command rather than degrading quietly. `hooks/hooks.json` registers four events. Two hooks hold the unit fence: `PreToolUse` on the write tools denies hand-writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json`, and the first `Stop` hook refuses a role unit's stop once, when its findings file is still missing and it wrote no escalation record. Three serve a coordinator (below): `PreToolUse` on `Agent` claims a brief's task for the spawn, `SubagentStop` refuses a sub-agent's stop while its claimed task is neither `done` nor `waiting`, and the second `Stop` hook asks a coordinator to fill its free slots. `agents/` declares `tp-implementer`, `tp-reviewer` and `tp-auditor`, carrying **tool restrictions only** — the role's content stays in the corpus and reaches the unit through the prompt `tp review`/`tp audit` emits. Every hook declares a `timeout` of 10 seconds. A plugin update reaches a session only when the session starts: a running session keeps the skill text and hooks it loaded, so after `claude plugin update tp@tp` start a new session. A runtime that cannot load a plugin runs the same units without them: the restrictions are defence in depth, and the brief plus the tp commands are the durable contract.

**Interactive fallback — subagent-per-unit.** Where no runner is configured, or a human belongs in the loop, drive the same loop by hand:

**The orchestrator's job is to brief the unit, not remember what to inject (v0.30.0).** A fresh unit does not inherit the orchestrator's session history, so run `tp brief` (read-only — produce the brief, then decide whether to spawn) or, for strictly serial work, tell the unit to run `tp next --brief` (claims the task and returns the brief in one call). When more than one unit runs at a time, or in a pool or worktree flow, each spawn's prompt holds `tp brief <id>`'s output instead, whose marker claims the task (below). The brief assembles everything the unit needs from durable state: **identity** and the one-unit-then-stop rule, the **scope fence** (acceptance-only — no refactor/rename/cleanup, no hand-editing the task file or `.tp-review/`; report an out-of-fence finding as an `Out of scope:` line instead of fixing it), **prior work** (each dependency's evidence and the files its commits touched), the verbatim **acceptance** + `spec_excerpt`, and the exact **close recipe** for the effective `commit_strategy` (the `--` separator before a dash-leading reason, the N-evidence-line format, and under `hc` that `tp commit`/`--auto-commit`/a bare `tp done` are rejected). `tp resume`'s `next_action.brief_command` names the right command per phase — `tp next --brief` in implement, `tp review <spec> --round N` in review, `tp audit <spec>` in audit (on a committed tree it exits 4 with `no changed files detected`: run it with `--affected-from-tasks`), null while an escalate blocker stands and in decompose — so an orchestrator following `next_action` reaches a full brief without knowing which phase it is in. The injection duty now survives ONLY for what tp cannot know: runtime-specific setup, such as which tools the sub-agent should use (e.g. native Read/Edit are hook-blocked in some repos → use the project's MCP toolset) and any live operational gotcha not yet carried by the brief.

**Realizing the reset in Claude Code.** The "FRESH unit" is a Claude Code **subagent** (Agent/Task tool): clean context, one unit, work persisted to disk (commit, `tp done`, `.tp-review`); the orchestrator re-orients via `tp resume` between units. Subagents don't nest (one level), so the orchestrator does each review/audit round's per-role fan-out itself. For a *full* reset of the driver too, use the `/clear` + `tp resume` loop or drive tp with headless `claude -p` per unit.

**Spawn from the brief, so the spawn claims (v1.29.0).** `tp brief <id>` carries the marker `tp-task: <id> <task file>`: as the `marker` key of its JSON when piped (how a coordinator usually runs it), and as the first line in a terminal. Put the brief's output in the Agent prompt, one task per spawn: the plugin's Agent hook reads the marker line or the pasted JSON's `marker` and claims that task for the spawn before the agent starts (`claimed_by`, `started_at`), and its SubagentStop hook refuses the agent's first stop while the task is neither `done` nor `waiting`, naming the `tp done` and `tp set ... waiting=` commands with their `--file`. A refused claim (done, blocked, another spawn holds it) refuses the spawn itself; when the holder is gone, `tp claim <id> --takeover` (frees the claim from the dead spawn, keeps `started_at`; spawn again from a fresh `tp brief <id>`, since the old brief's claim token is retired) or `tp unclaim <id>` is the way out. The brief also carries a fresh claim token (`tp-claim-token: <token>` under the marker, `claim_token` in its JSON), which the hook records in the claim (v1.29.2). An agent in its own worktree commits its work with git and runs `tp done <id> --commit <sha> --claim-token <token> -- "<evidence>"`, as its brief's close recipe says: a claim that carries a token refuses a close without it, and a close from an agent a takeover replaced, before its gate runs. That gates its code and leaves a land request, and the coordinator's `tp land` picks the commits onto its branch and closes the task there, so a dependent starts only once the code is in. `tp commit` and `--auto-commit` are refused there with those commands. Then the agent stops: once the task is landed or reopened, a close in that worktree exits 4 with the outcome. A park (`tp set waiting=`) goes to the copy that holds the claim. The close lands through the coordinator whether or not the claim was committed with the task file before the agent's worktree was cut (v1.29.4), provided the coordinator works in the main worktree; a coordinator in a linked worktree leaves the claim uncommitted until `tp land`. Inside `tp run` both hooks stand aside: the driver accounts for its units.

**Keep the pool full (v1.29.0).** A session that claimed through spawns is a coordinator, and the plugin's second `Stop` hook runs `tp pool --stop-hook` when its turn ends: while fewer agents run than `run_max_parallel` allows and a task is ready (or a claim lost its agent), the stop is refused once with the tasks to start, each with its `tp brief <id> --file <file>`. A worktree agent's finished task comes first: while a land request waits, the stop is refused with `tp land --file <file>`, slot or no slot; a landing that fails reopens the task, and `tp land` names the brief to spawn a fresh agent from. The tasks are the ones `tp run` would start beside the running ones, so a task whose `touches` overlap a running task's waits. Spawn them in the background, each with its brief. A claim whose agent is gone is freed with `tp claim <id> --takeover --file <file>` and spawned again. A task that must not start yet for a reason that is not a task (a release tag, a person's decision) is held: `tp set <id> waiting="<why>"` on the open task takes it out of the ready set until a claim or `waiting=""` lifts it, and the hook stops asking about it. An agent waiting on its own long job parks its claim the same way, on its wip task, and keeps its slot. The second stop in a row passes, and an ordinary session that never claimed is never asked. Set `run_max_parallel` at the project layer (`tp set --workflow --project run_max_parallel=<n>`) to the number of agents first: it is the hand pool's cap too, default 1. Spawn each agent in its own worktree (Claude Code's Agent tool with `isolation: "worktree"`, or `git worktree add`), its prompt holding the brief whole. When a worktree agent's `tp done` exits 2 for a missing `--claim-token`, it reruns the close with the token its brief names; when it exits 4 (taken over, or already landed or reopened), it stops, and the coordinator respawns the task from a fresh `tp brief <id>`.

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

Every command and flag tp registers, one line each. Each form's full contract — payload keys, every
refusal and its exit code — is under the same form in [REFERENCE.md](REFERENCE.md), "Command forms".

### Primary workflow
| Command | Purpose |
|---------|---------|
| `tp plan` | Full execution plan (THE primary command) |
| `tp plan --minimal` | id + acceptance only |
| `tp plan --compact` | Plan without description, source_lines, tags |
| `tp plan --from <id>` | Plan from a task onward |
| `tp plan --level 0,1` | Filter by parallelism level |
| `tp commit <id> [reason]` | Stage, commit, record the SHA; the task stays `wip` (close with `tp done`) |
| `tp commit <id> --files "*.go"` | Selective staging |
| `tp done <id> "reason"` | Close with implicit claim; runs the quality gate |
| `tp done <id> --skip-gate "why"` | Skip the gate (operator decision) |
| `tp done <id> --gate-passed` | Gate-less projects: record an attestation |
| `tp done <id> --auto-commit` | Commit + close in one call (`builtin`) |
| `tp done <id> --auto-commit --files "*.go"` | Selective staging + commit + close |
| `tp done <id> --covered-by <id>` | Close as covered by another done task |
| `tp done <id> --commit <sha>` | Record the implementing commit |
| `tp done id1 id2 "reason"` | Multi-ID close |
| `tp done <id> --commit a --commit b` | Several commits (`hc`); repeatable |
| `tp done <id> --commit <sha> --claim-token <token>` | In an agent's worktree: the claim token its brief names |
| `tp done --batch file.ndjson` | Batch close from NDJSON |
| `tp done <id> --reason-file reason.md` | Reason from a file |
| `tp done <id> --stdin` | Reason from stdin |
| `tp resume [spec]` | Phase, blockers and next action from durable state (read-only) |
| `tp resume --epic <id>` | The same on the epic's bound task file; `tp next` and `tp ready` take it too |
| `tp gate` | Run the quality gate on this checkout and close nothing; exit 0 pass, 4 fail |
| `tp brief [id]` | The unit brief (read-only): scope, prior work, task, close recipe |
| `tp brief <id> --prior <n>` | Prior-work cap (0-20) |
| `tp run [spec]` | Drive the cycle unattended |
| `tp run --status` | The current or last run's state |
| `tp run --dry-run` | The units the driver would run next; spawns nothing |
| `tp escalate --decision <name> --evidence <text>` | Record an operator decision and stop the unit (exit 2); names in "Gate, Budget & Escalation Policy" |
| `tp escalate ... --option <text>` | A way forward the unit saw; repeatable |

### Incremental
| Command | Purpose |
|---------|---------|
| `tp next` | Resume WIP or claim the next ready task; never resumes an agent-held task |
| `tp next --minimal` | {id, acceptance} only |
| `tp next --peek` | Preview without claiming |
| `tp next --brief` | Claim and return the brief |
| `tp next --epic <id>` | The same on the epic's bound task file |

### Task state
| Command | Purpose |
|---------|---------|
| `tp claim <id> [id...]` | open -> wip |
| `tp claim --all-ready` | Claim every ready task |
| `tp claim <id> --spawn <tool_use_id> [--session <id>] [--claim-token <token>]` | Claim for an agent spawn (the plugin's Agent hook runs it); `--takeover` frees another spawn's task |
| `tp claims --spawn <tool_use_id>` | A spawn's claimed tasks and which are `unaccounted` |
| `tp pool --session <id> [--live <spawn>]... [--unresolved <n>]` | A coordinator's slots and what to spawn next; `--stop-hook` prints only the Stop decision |
| `tp land` | Land worktree agents' requests onto this branch and close their tasks |
| `tp unclaim <id> [id...]` | wip -> open |
| `tp close <id> <reason>` | wip -> done (low-level; prefer `tp done`) |
| `tp close <id> --reason-file reason.md` / `--stdin` | Reason from a file / stdin |
| `tp close <id> --skip-gate "why"` | Skip the gate (operator decision) |
| `tp reopen <id>` | done -> open |
| `tp remove <id>` | Remove a task (`--force` cleans deps) |
| `tp set <id> field=value` | Update a field; `waiting=<text>` parks a wip task or holds an open one |
| `tp set --workflow field=value` | Update a workflow field (REFERENCE.md, "Workflow Fields") |
| `tp set --workflow task_audit_roles='["go-safety"]'` | The code roles of the task audit |
| `tp set --workflow runner=…` / `notify_cmd` | **Not settable**: hand-write them (REFERENCE.md, "The `runner` field") |
| `tp set --workflow checks='[{"class":"s","cmd":"c"}]'` | Replace the mechanical checks |
| `tp set --workflow --project <field>=<value>` | Write a field to `.tp/config.json` |
| `tp set --workflow --project worktree_setup_cmd=<command>` | Pool worktree setup command (operator decision) |
| `tp set --local defaults.<flag>=<bool>` | A CLI flag default |
| `tp set --bulk sets.ndjson` | Bulk update |
| `tp set --epic <id>` | Bind the task file to an epic (`""` unbinds; operator decision under `TP_UNATTENDED`) |
| `tp keep <path> "<reason>"` | Keep-list an uncommitted file (`--remove`, `--list`) |

### Query
| Command | Purpose |
|---------|---------|
| `tp list` | All tasks (--status, --tag, --ids, --compact) |
| `tp ready` | Ready tasks (--first, --count, --ids) |
| `tp ready --epic <id>` | The same on the epic's bound task file |
| `tp show <id>` | Full details + spec_excerpt + blocks |
| `tp status` | Progress summary |
| `tp report` | Per-task duration + estimation accuracy |
| `tp blocked` | Tasks waiting on deps |
| `tp graph` | Dependency tree (--tag, --from) |
| `tp stats` | Parallelism analysis |
| `tp config` | Effective configuration (`--resolved`; `--extract [--dry-run\|--force]` hoists shared policy into `.tp/config.json`) |
| `tp export` | The task file's read model as JSON |
| `tp export --compact` | The same, trimmed |
| `tp export --list` | One row per task file in the repository |
| `tp dashboard [--port N]` | Serve the read-only dashboard |

### Spec & validation
| Command | Purpose |
|---------|---------|
| `tp lint spec.md` | Spec form checks; reports `floor_size`, `cut`, `review_panel` |
| `tp review spec.md` | Review prompts, one per active reviewer role |
| `tp review spec.md --perspective code-audit --affected-files src/a.go` | Code audit with source injection |
| `tp review spec.md --round N --findings file.ndjson` | Exclude previous findings; N must match the state round |
| `tp review spec.md --round N --final-round --affected-files src/a.go` | Final round with code read-through |
| `tp review spec.md --affected-files a.go --affected-files b.go` | Inject source files (repeat per file) |
| `tp review --merge r1.ndjson r2.ndjson -o merged.ndjson` | Merge + dedup findings (`--output`) |
| `tp review spec.md --perspective documentation --docs-path docs/` | Documentation perspective |
| `tp review spec.md --perspective testing --test-path internal/` | Testing perspective |
| `tp review --resolve findings.ndjson <idx> <disposition> "evidence"` | Dispose one finding (0-based) |
| `tp review --resolve-all findings.ndjson <fixed\|wontfix\|duplicate> "evidence"` | Dispose many findings |
| `tp review --resolve ... --force` | Re-resolve; on an emission, discard a changed unrecorded round |
| `tp review --verify spec.md --findings all.ndjson` | Lightweight verification |
| `tp review --report r1.ndjson r2.ndjson` | Cross-round convergence report |
| `tp review spec.md --diff-from old-spec.md` | Diff-based review |
| `tp review spec.md --spec-inline` | Embed the spec inline |
| `tp review spec.md --record-inbox` | Record a round from its inbox |
| `tp review spec.md --record merged.ndjson` | Record a round from a file |
| `tp review spec.md --record ... --harness-note "<text>"` | Record the orchestrator's framing |
| `tp review spec.md --status` / `--status --check` | Loop state / exit 0 when done |
| `tp review spec.md --rewritten "<evidence>"` | Start an epoch after a rewrite |
| `tp review spec.md --perspective regression` | Standalone regression pass |
| `tp review spec.md --no-state` | No state reads or writes |
| `tp review spec.md --role <name>` | One role's prompt |
| `tp audit spec.md` | Post-implementation audit prompts |
| `tp audit spec.md --affected-files src/a.go` | Manual file selection |
| `tp audit spec.md --affected-from-tasks` | The files of done tasks' commits |
| `tp audit spec.md --role <name>` | One role's prompt |
| `tp audit spec.md --findings review.ndjson` | Also verify review findings were addressed |
| `tp audit spec.md --record results.ndjson` | Record an audit round |
| `tp audit spec.md --task-audit [--role <name>]` | Emit the task audit due now |
| `tp audit spec.md --task-audit --record-inbox` | Record the task audit |
| `tp audit spec.md --base <git-ref>` | Detect audited files against a ref |
| `tp audit spec.md --record ... --harness-note "<text>"` | Record the orchestrator's framing |
| `tp audit --merge r1.ndjson r2.ndjson -o results.ndjson` | Merge per-role results (`--output`) |
| `tp audit spec.md --status` / `--status --check` | Loop state / exit 0 when done |
| `tp audit results.ndjson --resolve <selector> <disposition> "<evidence>"` | Dispose rows by index or `role:item_id` |
| `tp audit results.ndjson --resolve-all <disposition> "<evidence>"` | Dispose every undisposed row (`--force` re-resolves) |
| `tp ground spec.md` | Emit a ground round |
| `tp ground spec.md --force` | Discard a changed unrecorded round |
| `tp ground spec.md --record rows.ndjson` | Record a ground round |
| `tp ground spec.md --units` | Each floor unit's full text |
| `tp ground spec.md --status` | Latest emitted round's coverage and verdicts |
| `tp ground spec.md --status --check` | Exit 1 on an undispositioned unit or a `FAIL` |
| `tp validate` | Task file validation + coverage + atomicity |
| `tp validate --strict` | Warnings become errors |
| `tp validate --project` | Cross-spec workflow drift |
| `tp epic lint` | Lint `.tp/epics.json` |
| `tp epic export` | The plan as JSON |
| `tp epic export --md` | The plan as Markdown tables |
| `tp epic layout [--view plan\|milestones\|scope\|neighbourhood] [--milestone <m>] [--epic <id>] [--depth <n>] [--pack rows\|tree]` | One laid-out view of the map |
| `tp epic add '<json>'` / `tp epic add --stdin` | Add one epic |
| `tp epic add --bulk <ndjson>` | Add or replace many, all or none |
| `tp epic set <id> <field>=<value>` | Set one epic field |
| `tp epic set <milestones\|opens_after\|gates\|tag_checks\|owners>=<json>` | Replace a top-level object |
| `tp epic set gates.<m>.<threshold\|purpose\|deferrable\|members>=<value>` | Edit one gate |
| `tp epic remove <id>` | Remove an epic nothing depends on |
| `tp epic remove <id> --force` | Also drop its edges, memberships and groups |
| `tp epic import <file>` | Write a graph into an empty file |
| `tp epic import <file> --force` | Replace a file that holds epics |
| `tp epic add <epic> --if-revision <hex>` | Write only at this revision, else exit 4 |
| `tp epic set <id> <field>=<value> --if-revision <hex>` | The same guard |
| `tp epic remove <id> --if-revision <hex>` | The same guard |
| `tp epic import <file> --if-revision <hex>` | The same guard |
| `tp epic link <from> <to> <why>` | Add or re-explain an edge |
| `tp epic link <from> <to> <why> --if-revision <hex>` | The same guard |
| `tp epic link <from> --any-of <csv> <why>` | Add an any-of group |
| `tp epic unlink <from> <to>` | Remove an edge |
| `tp epic unlink <from> <to> --if-revision <hex>` | The same guard |
| `tp epic unlink <from> --any-of <csv>` | Remove an any-of group |
| `tp epic apply <ndjson>` | Many graph writes under one lock, all or none |
| `tp epic apply <ndjson> --dry-run` | Preview the write |
| `tp epic apply <ndjson> --if-revision <hex>` | The same guard |
| `tp epic milestone add <name> --after <m>` / `--before <m>` | Insert a milestone in `<m>`'s track |
| `tp epic milestone add <name> --new-track` | A new track |
| `tp epic milestone add <name> --if-revision <hex>` | The same guard |
| `tp epic gate <m>` | A gate's state; exit 0 only when green |
| `tp epic gate <m> --met --evidence "<text or URL>"` | Write the met record (operator decision) |
| `tp epic gate <m> --met --evidence <e> --if-revision <hex>` | The same guard |
| `tp epic scope <m>` | What a gate pulls in |
| `tp epic done <id> --evidence "<text or URL>"` | Done record on an unbound epic (operator decision) |
| `tp epic done <id> --undo` | Remove it |
| `tp epic done <id> --if-revision <hex>` | The same guard |
| `tp epic claim <id> [--by <name>]` | Reserve an epic |
| `tp epic claim <id> --release` | Release it |
| `tp epic claim <id> --if-revision <hex>` | The same guard |
| `tp epic ready` | What can start, and the operator's items |
| `tp epic ready --all` | Ignoring closed milestones |
| `tp epic ready --owner <h>` / `--unowned` | Filter by owner |
| `tp epic status` | Each milestone's progress |
| `tp epic status --milestone <m>` | One milestone |
| `tp epic status --open` | Open milestones only |
| `tp epic status --owner <h>` / `--unowned` | Filter by owner |
| `tp epic review --milestone <m>` / `tp epic review --epic <id>` | Emit a graph review round over a slice |
| `tp epic review --epics <ids>` | Over a set of epics |
| `tp epic review --epics <ids> --name <name>` | A named set that may grow |
| `tp epic review <slice> --role <name>` | One role's prompt |
| `tp epic review <slice>` re-emitted | Idempotent while unchanged; changed exits 3 |
| `tp epic review <slice> --discard` | Remove an unrecorded emission (operator decision) |
| `tp epic review <slice> --record <ndjson>` | Record a round from a file |
| `tp epic review <slice> --record-inbox` | Record a round from its inbox |
| `tp epic review <slice> --status` | The slice's loop status |
| `tp epic review <slice> --grant-round` | One round past the cap for a stale slice (operator decision) |
| `tp epic review <slice> --status --check` | Exit 0 when done |
| `tp epic review <slice> --history` | The slice's round-by-round graph history |
| `tp epic review <slice> --history --from <a> --to <b>` | Two rounds compared |
| `tp epic review --history --of-epic <id>` | One epic's history across slices |
| `tp epic review <slice> --resolve <0-based index>[,<index>...] <fixed\|wontfix\|duplicate> [evidence]` | Dispose findings (`--force` re-resolves) |
| `tp epic review <slice> --resolve-all <fixed\|wontfix\|duplicate> [evidence]` | Dispose every finding |
| `tp epic review --milestones <m,m,...>` / `tp epic review --all` | Emit many milestone slices |
| `tp epic review --milestones <m,m,...> --status` / `tp epic review --all --status` | One status row per slice |
| `tp epic review --report --all` / `tp epic review --report --milestones <m,m,...>` | Finding classes across slices |

### Data
| Command | Purpose |
|---------|---------|
| `tp init spec.md` | Create the task file shell |
| `tp init spec.md --epic <id>` | The same, bound to an epic |
| `tp init --eject-roles [--domain <name>] [--force]` | Write the default role corpus |
| `tp init --eject-roles --graph [--force]` | Write the graph-review roles |
| `tp init spec.md --quality-gate "<cmd>"` | Task-level gate override (masks the project gate) |
| `tp init spec.md --commit-strategy <builtin\|auto\|hc>` | Task-level `commit_strategy` |
| `tp add <json>` | Add a task (`--stdin` for piped input) |
| `tp add <json> --spec spec.md` | Spec path when the file must be created |
| `tp add --bulk tasks.ndjson` | Bulk add |
| `tp import file.json` | Import + validate (`--force` is an operator decision) |
| `tp import tasks.json --spec spec.md` | Import a bare JSON array |
| `tp use <file>` / `--clear` / bare | Set / clear / show the active task file |

### Global flags
| Flag | Purpose |
|------|---------|
| `--file <path>` | Explicit task file; its project is found from the file's own directory |
| `--json` | Force JSON output (default when piped) |
| `--compact` / `--no-compact` | Minimal JSON / force full output |
| `--quiet` / `--no-quiet` | Suppress / force info messages |
| `--no-color` / `--color` | Disable / force color |
| `-v` / `--version` | Print the tp version |

## Closure Rules

Before closing a task (`tp done`):

1. Re-read acceptance criteria from the plan output.
2. Verify implementation matches the FULL spec (not just the acceptance summary).
3. Write the reason in **evidence-line format**: for a task with N ≥ 2 acceptance criteria, the reason MUST contain at least N lines each starting with `- ` at column 0 (indented sub-bullets do not count) — one top-level evidence line per criterion, with file paths. A single-criterion task accepts any non-empty reason.
4. Never use: "deferred", "will be done later", "covered by existing" (without a path), single-word reasons.
5. Use `--covered-by <id>` when the work IS done but in a different task (not a deferral).
6. `tp done` auto-claims open tasks — no separate `tp claim` needed.
7. Code snippets in a spec may be illustrative — validate against the actual codebase before implementing.

> A reason starting with `- ` looks like a cobra flag; use the `--` separator, with real newlines (`"\n"` inside double quotes stays literal in bash and zsh): `tp done <id> --commit <sha> -- $'- line 1\n- line 2'`, or `--reason-file <file>`.

## Gate, Budget & Escalation Policy — user-approval gates

- **The gate runs automatically at `tp done`.** `--skip-gate "<reason>"` skips it and records `gate_skipped_reason` on each closed task. **`--skip-gate` requires explicit user approval — it is never the agent's own decision.**
- **The round cap (`review_max_rounds` / `audit_max_rounds`, default 3; 0 = uncapped):** once the cap is reached, prompt generation and `--record` refuse with exit 4. The loop is **done** (`done_by: cap`) when every finding of the latest round carries a disposition in its recorded round file — `fixed`, or `wontfix`/`duplicate` with evidence — and no unresolved or unverified-fixed blocking row is carried from an earlier round (v1.16.0; the refusal's hint is then that row's way out — an acceptance, or the operator's raise of the cap, never `fixed`). A blocking finding marked `fixed` at the cap keeps the loop open (`blocking_fixed_at_cap`): no round will re-read the repair, so the operator accepts it with evidence or raises the cap by one for a verification round. A done loop needs no `--force` at `tp import`. Raising a cap and importing with `--force` stay user-approved decisions — never the agent's own.
- **Convergence criteria differ by phase (v0.28.0+).** A **spec review** is converged only when a counted round surfaces **no blocking findings** — the blocking severities are **critical and high** (the built-in `review_converge_on=blocking` policy; `review_converge_on=all` opts into the strict any-severity rule) — and, since v1.16.0, no blocking finding of an earlier round stands unresolved in the carried set, since the next round is told not to re-report it: a blocking finding leaves the loop only by a fix, an acceptance or an epoch, never by silence. Accepting a critical or high finding without a spec change is the operator's decision, and so is an epoch that cuts one (`rewrite-past-blocking`) (medium/low findings may be accepted with recorded justification once none blocking remain). An **implementation audit** ends on `audit_clean_rounds` consecutive clean rounds (default 2), or at `audit_max_rounds` with every finding dispositioned, and since v1.18.0 only while no blocking row of an earlier round stands carried: a non-PASS row leaves the loop by a later round's PASS, an acceptance or an epoch, never by an empty round; accepting any audit finding without a code change is the operator's decision. What an audit round must be clean *of* is `audit_converge_on` (v0.37.0), whose built-in default `all` counts every non-`PASS` row — deliberately the opposite of `review_converge_on`'s default. Setting it to `blocking`, so only `error` (and any severity tp cannot grade) blocks, is a **user-approved decision**, taken per cycle on the previous cycle's severity mix and never as a standing repo default; see "`audit_converge_on`" in Workflow D for the fence, the escalation name and the one-liner that derives the mix.
- **Under `tp run` the policy is enforced, not merely stated (v0.35.0).** Every unit a run spawns carries `TP_UNATTENDED=1`, and the decisions above stop being available to it: `--skip-gate` at any of its sinks, `tp import --force`, a `tp set --workflow` raise of `review_max_rounds`/`audit_max_rounds` or any `run_max_*` cap, — since v0.37.0 — `audit_converge_on` reaching `blocking`, at all four of its sinks, a change of `quality_gate` at any of its write paths, — since v1.24.0 — setting or changing `worktree_setup_cmd` at either layer (escalated as `other`), accepting a critical/high review finding or any audit finding, an emission's discarding `--force` and, since v1.10.0, `tp epic review --discard`, — since v1.5.0 — an epic's done record, a gate's met record and a task file's epic binding, removals included, and — since v1.7.0 — the grant of one epic review round past the cap (`tp epic review <slice> --grant-round`), and — since v1.16.0 — an epoch whose cut holds an unresolved blocking review row (`tp review <spec> --rewritten`), each exit 2 with a hint naming `tp escalate`. By hand, outside a run, that epoch runs with no flag, as an acceptance by hand does, and says on stderr which rows it cut; `--force` stays the discard of an unrecorded emission. The rule differs by sink and the difference matters: `tp set --workflow --project` refuses **any** write of `blocking`, on its value alone, because it writes the layer every base resolves through including bases tp cannot enumerate; `tp set --workflow` (task layer), `tp import` and `tp config --extract` refuse a write that **changes what resolves** to `blocking`. See the `audit_converge_on` section of Workflow D. Record the decision with `tp escalate --decision <skip-gate|raise-review-cap|raise-audit-cap|import-force|audit-converge-on|accept-finding|discard-emission|epic-done|gate-met|epic-binding|grant-round|rewrite-past-blocking|other> --evidence "<what you found>" [--option "<a way forward>"]` — the run stops with `stop_reason: escalation`, the operator decides, and `tp run` is started again. **An escalation is a normal outcome, not a crash**, and it is neither a failed attempt nor a recorded round.
- **A unit runs a full suite only through `tp gate` (v1.26.0).** Inside `tp run`, run the targeted package tests your change needs directly, and a whole suite only as `tp gate`, which takes one of the run's gate slots (`run_max_parallel_gates`) as `tp done`'s gate does, so pooled units overlap on agent time rather than on test load. This is a rule for the agent, not a fence: tp cannot see an agent's own `go test`. Slots are polled, not queued fairly, so a gate can wait longer than one that asked after it; the main-branch gate always goes first, and the wait never counts against `gate_timeout_seconds`.
- **Verify the gate with separate exit codes, never a `&&` chain.** `<tests> && <linter> && echo OK` prints nothing when it fails, and a missing `OK` does not catch the eye. Run each command on its own and read each exit code (`<tests>; echo $?`, then `<linter>; echo $?`) before you close the task.

## Class & Checks Guidance

- **Fill `class`** on every review finding: a kebab-case slug naming its failure class, the key `--merge` clusters on. A class that is a pattern a script could check across the whole corpus (example: `code-citation-drift`) is what the mechanization rule below looks for.
- **Mechanization candidate:** a class that appears in ≥ 2 distinct rounds OR ≥ 5 times in a single round (`tp review --report` and `--record` output list `mechanize_candidates`). When one appears, write a detector command and register it: `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'`. A check is **only worth registering when the artifact it measures already exists in the review phase** — registered checks run in the review phase only, so a check whose subject a later phase writes can never verify it, while tp still tells every reviewer to stop reporting that class. When the subject is written later, make running the check the acceptance of the task that creates it instead.
- Once registered, tp runs the check every review round, reports it in `mechanical_checks`, and tells reviewers to stop reporting that class. A check's exit code is its contract: **0** passed, **1** found violations, anything else — 2 or higher, 126/127, a signal, a start failure, a timeout — could not run and is reported `ran: false`. A prompt suppresses a class only when its check reported `ran: true` in that emission, so a check that could not run — or never started, because no task file resolves — suppresses nothing. `tp review --status --check` requires every check to pass before exiting 0.
- **A check reads its spec from `TP_SPEC` (v1.20.1).** tp runs every check at the task file's repository root with `TP_SPEC` naming the spec the command is about: relative to that root when the spec lies under it, absolute otherwise. A `TP_SPEC` or `TP_ROUND` the shell or a `tp run` unit carried is dropped first, and with no spec `TP_SPEC` is unset, never empty. Write the argument as `"${TP_SPEC:?}"`, so a run with no spec fails the check rather than grading an empty path. Never derive the spec from `tp resume`: it names the active task file's spec, so in a repository with two specs the check grades the wrong one — tp's own three checks did that until v1.20.1.
- **Registration retires the suggestion (v0.33.0):** a registered check retires its mechanize candidate — the class is dropped from `mechanize_candidates`, from the register-a-check `hint`, and from `next_action`'s mechanize branch, and `tp review <spec> --record <file>` names the withheld classes in `mechanized_classes`. Under `--record` and `--status --check` only a check that ran (exit 0 or 1) retires its class; plain `--status`, which runs no check, keeps registration as the trigger. `--record` runs the checks only when their result can change its output, and then carries `mechanical_checks`. A failing check still blocks `tp review --status --check`. Class matching is byte-for-byte.
- `over-specification` is the one exception, scoped to a single sink: registering a check for it suppresses it from the candidates like any other class, but it never joins the `Mechanically checked classes — do NOT report findings of these classes:` sentence, because tp's own review prompts ask reviewers to raise it.

## Role Corpus & frontmatter overrides (v0.25.0)

Review and audit roles are **project-owned data** — one JSON file per role under the repo-root `.tp/`:

- `.tp/reviewers/*.json` drives `tp review`; `.tp/auditors/*.json` drives `tp audit` (phase = directory). Schema `{id, title, instructions, focus[], domains[], context}` (`context` is read only by `tp epic review`); `id` MUST equal the filename stem (lowercase kebab-case); `regression` is reserved. Commit the corpus to VCS.
- A populated phase directory **replaces** the embedded default corpus for that phase; absent/empty keeps tp's curated defaults (software: implementer/tester/architect + spec-coverage/security/maintainability-conventions; prose: coherence/soundness + spec-coverage/soundness). A project happy with defaults keeps **zero role files**.
- `tp init --eject-roles [--domain software|prose] [--force]` writes the defaults as editable, byte-identical files, and **ejected role files are not rewritten on upgrade** — re-eject with `--force` to adopt newer default prompts. tp's own repo dogfoods a custom 4+4 corpus (adopted v0.26.0: implementer/tester/architect/ax-economist reviewers + spec-coverage/go-safety/maintainability-conventions/ax-contract auditors).

Emission is corpus-driven: `tp review` emits one prompt per active reviewer role plus the built-in `regression` role (appended, never a corpus file); `tp audit` emits one per active auditor role. Every prompt stamps a five-key output contract: `role, location, class, severity`, plus `evidence` on a review prompt (v1.1.0) and `status` on an audit prompt. tp still only emits prompts — it never executes agents.

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
- **`enabled` (v0.32.0)** is the override object's second permitted key beside `focus`. enabled: false deactivates a role for one spec — no prompt, and that override's `focus` applied nowhere; `enabled: true` is a no-op. Two refusals fire **on prompt emission only** and exit 2: emptying a phase, and deactivating `spec-coverage`. Both are emission-only: they exit before any prompt is emitted and before any state is written — so `--record`, `--status`, `--merge` and the rest are unaffected, and a refused run leaves no `.tp-review/<spec>/` behind. Keep `domains` as the **durable** axis — a role's standing scope across every spec — and use `enabled: false` as the **one-spec exception** to it.
- **Overlap and staleness:** `tp review --merge`/`--report`/`--status` carry a per-role `overlap_report` flagging a `trim_candidate` (a reviewer that found only what others also found), and `--status` reports `roles_stale` when the corpus changed since the last recorded round. tp reports; trimming a role is your decision.

Corpus validation and its exit code, the exact warning and refusal strings, the cluster keys, the hash
rules and the two trim levers are in [REFERENCE.md](REFERENCE.md).

## Role authoring guidance (v0.25.0)

Opening role authoring is a power feature — a project-authored role is only as good as its prompt. Design each role for **high-signal, low-overlap, contract-conformant** findings:

1. **One distinct failure-lens per role.** A role must target a failure mode no other role covers. Overlapping roles waste tokens and get flagged as trim candidates (`overlap_report`) — diversity of lenses beats count.
2. **A reviewer reports what would make the implementation wrong** — a requirement that contradicts another, contradicts the spec's stated goal, or cannot be satisfied. Detail the code will settle (exact exit codes, JSON field names, error hints, helper names) is not a finding: the implementation decides it and its tests pin it. Adversarial probing belongs in the audit, where code exists.
3. **Evidence demand.** Every finding carries a `location` (a `§`-anchor) and a why — this is what makes dedup (the overlap report) and audit PASS/FAIL meaningful. A `high` review finding carries a counterexample in `evidence`: two quoted spec lines that contradict each other, or a command and its output.
4. **Finding nothing is a normal result.** A role that returns no rows has done its job; tell it so, or it reaches for a minor point.
5. **Output-contract adherence.** The role customizes only its `focus`, never the finding schema — tp injects the fixed contract (`role, location, class, severity`, plus `evidence` on a review prompt and `status` on an audit prompt).
6. **Altitude.** A spec review should push toward **decidable invariants** — behavior a task's acceptance can verify against the quality gate — and away from implementation prose that pins mechanism (SQL, an index, a field layout) whose correctness only code can establish. The canonical finding `class` `over-specification` names exactly this smell: *a detail whose correctness can only be established against code, prescribed in the spec where it belongs in task acceptance instead.* Any reviewer may raise it; it is usually **low or medium** severity (an altitude judgment, not a blocking defect), so it never blocks convergence under `review_converge_on=blocking` unless a reviewer stamps it critical/high — convergence reads severity, not the class. tp does not detect over-specification and never deletes spec content on its own claim of it; the remedy is to revise the over-specified detail down into a task's acceptance where the gate can check it.

**Worked example role sets:**

- **code/software** — reviewers: *buildability* (`implementer`: a requirement that cannot be built as written), *testability* (`tester`: two correct implementations that would fail each other's tests), *consistency* (`architect`: section against section, and against existing behaviour the spec does not say it changes). Auditors, on code: `spec-coverage`, `security` (trust boundaries, injection, unsafe defaults, unpaired locks), `maintainability-conventions`.
- **prose** — *narrative continuity* (coherence: does one part contradict, duplicate, or pre-empt another?) vs *expository derivability* (soundness: can each claim be derived from what precedes it without inventing facts?). Prose defaults to the leaner two-reviewer panel because prose flaws surface from many angles at once.

**Other domains** and their characteristic diverging lenses (for custom corpora): **legal/contract** — obligation completeness vs. ambiguity/loophole; **product/PRD** — user-journey completeness vs. measurable acceptance; **data-schema** — referential integrity vs. migration/compat; **academic** — claim support vs. methodology soundness.

The embedded default reviewers follow rules 1–4 — run `tp init --eject-roles` to read them. A corpus ejected before v1.2.0 keeps the older, adversarial reviewer text until it is re-ejected with `--force`.

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
| in the emitted set | exit **0**, exactly one `prompts[]` entry, byte-identical to that role's entry in the same invocation without the flag; `skipped_roles` names every other emitted role with reason `role-filter` and, when non-empty, survives `--compact` |
| recognised but not emitted this round | exit **0**, `prompts: []`. A name *this* phase skipped keeps its own `skipped_roles` entry; a name recognised only through the other phase's corpus is in none |
| recognised nowhere | exit **2**, with a hint naming what the invocation would have emitted; nothing is written to the state directory |

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
| `tp review` | default, `--perspective`, `--diff-from`, `--verify` | `--merge`, `--record`, `--status`, `--report`, `--resolve`, `--resolve-all`, `--rewritten` |
| `tp audit` | default only | `--merge`, `--record`, `--status`, `--resolve`, `--resolve-all` |

`tp audit` registers no `--perspective`, `--diff-from`, `--verify` or `--report` at all.

**`review_loop.instruction` narrows with the payload.** That key is addressed to a caller holding
the whole panel — spawn a sub-agent per prompt, merge, record the round, order the regression prompt
against the others. Under `--role` it follows the payload it now describes: a payload holding **one**
prompt gets a sentence-subset that directs no action that payload cannot support; a payload holding
**none** gets an empty key, because nothing at all is supportable there. Without `--role` the key is
unchanged.

## Where judgement-shaping text belongs (v0.31.0)

Standing instructions that shape a reviewer's judgement — what counts as a real defect, what to treat as intentional, what altitude to hold — do **not** belong in the orchestrator's prompt wrapper. tp cannot see that wrapper, so framing hidden there makes two rounds look comparable when their instructions differed materially. Standing framing has two sanctioned homes, both recorded via `roles_hash`:

- **Per-spec focus** → `tp.review_roles` / `tp.audit_roles` frontmatter (append `focus` to a reviewer or auditor role).
- **Project-owned roles** → `.tp/reviewers/*.json` / `.tp/auditors/*.json`.

The wrapper is only for what tp cannot know — runtime setup (e.g. hook-blocked native file tools → use the MCP toolset). When the wrapper must carry standing framing anyway, declare it with `--harness-note "<text>"` on `tp review`/`tp audit --record` so it is recorded on the round instead of staying invisible (the flag requires `--record`; supplying it alone is a usage error, exit 2). `--status` then surfaces `harness_stale` — true only when the two most recently recorded rounds **both** carry non-empty notes that differ — and, when stale, the latest `harness_note`. The note is opt-in, gates nothing, and both `harness_note`/`harness_stale` are stripped under `--compact`. Review and audit surface these symmetrically.

## `next_action` — the single next step (v0.31.0)

`tp review --status`/`--record` and `tp audit --status`/`--record` return a `next_action` naming the one next step the current state calls for, by a fixed precedence (retained under `--compact`):

1. **Done** — converged, or the round cap reached with every finding of the latest round dispositioned → the forward step: review names the directive `decompose the spec into tasks, then tp import <base>.tasks.json`; audit names the terminal `converged — implementation verified, proceed to release`, and a loop the cap ended says so instead. Convergence wins even when non-blocking findings remain open. Under `audit_converge_on: blocking` the audit string also **names the accepted count as a numeral** (`converged over N accepted rows — …`), so the round that closed over open advisory rows says so.
2. **The cap reached with a finding undispositioned, or a blocking finding open** → at the cap, disposition each remaining finding in the recorded round file it names; below it, revise the spec where the finding is a defect or disposition it there (audit: fix the code, or disposition a finding that needs no code change). Disposition is named as an exit equal to a change; accepting a blocking finding is named as the operator's decision, and `--resolve-all` is never advised.
3. **A `mechanize_candidates` class recurs, none blocking** (review only) → register a check (`tp set --workflow checks='[…]'`), then run the next round. The directive carries the phase qualifier: a check is **only worth registering when the artifact it measures already exists in the review phase**, because registered checks run in that phase alone. The un-mechanizable `over-specification` class never triggers this — it falls through to step 4. A class whose registered check ran falls through too (under plain `--status`, any registered class), so the driver is never told to write a check that already exists; a class whose check could not run is named with that check to fix.
4. **Clean but not yet converged** → run the next round: `tp review <spec>`, then `tp review <spec> --record-inbox` (audit: `tp audit <spec> --record <file>`). Under `audit_converge_on: blocking` the audit string is prefixed with the same numeral (`N accepted rows carried forward — run the next audit round: …`) — this is the branch a `blocking` cycle takes on *every* clean round, where the converged branch fires once.

**Registered checks override the review import steps.** Under `--record` and `--status --check`, at
done or at the cap, a registered check that failed or could not run replaces every step that names
import with that check to fix, ending `tp review <spec> --status --check exits 1 until every
registered check passes`. Plain `--status` runs no check, so while any check is registered its import
step starts `run tp review <spec> --status --check (--status alone runs no check) and on exit 0`.

`next_action` is advisory and read-only; it gates no exit code.

## State directory (`.tp-review/`)

- **Commit `.tp-review/` to version control.** Import convergence enforcement holds across clones and CI only when the recorded rounds travel with the repo: ignoring the directory makes every `tp import` in CI behave as "no recorded rounds", and convergence is then unverifiable.
- **Prunable:** only snapshot files older than the newest MAY be deleted (the diff falls back gracefully).
- **Do not delete `.tp/locks/<file name>-<hash>.lock` (e.g. `spec.tasks.json-76b49627.lock`, `local.json-d8122ba9.lock`) while tp may be running.** It is a zero-byte, git-ignored marker that persists after its lock is released, and that is what makes the lock exclude: flock binds to an inode, so unlinking it lets the next waiter lock a different inode at the same path and run concurrently.

## Tail protocol (when a round drops to one or two low/medium findings)

1. **Verify disputed findings:** route each through `tp review --verify <spec> --findings all.ndjson`. A verifier-rejected finding is resolved `wontfix` with the verifier's reasoning **and written into the role's inbox file before `--record-inbox`**, the row carrying `"resolved":{"status":"wontfix","evidence":"..."}` (an emitted round is recorded only from its inbox, and `--record` over it exits 3; the round entry never recomputes) — a round whose surviving rows are all pre-resolved `wontfix` records as clean.
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
