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
   changed or fixed findings exist; process it first.
2. Spawn one sub-agent per prompt; each writes NDJSON findings to its `output_path` (an empty file,
   or a line holding only `[]`, when it finds nothing). **Every row needs `severity`, `finding`,
   `location` and `evidence`, non-empty**; `--record` refuses the whole file otherwise, and `tp review --merge` drops such a row with a stderr
   warning (`inputs[].skipped`), so it never becomes a finding. Row contract:
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
3. `tp import tasks.json` — **plain, no `--force`.** The init shell holds zero tasks, so no overwrite flag is needed and the convergence checks stay armed: a review loop with recorded rounds that is not done, or a registered review check that fails, blocks the import (exit 1); with no recorded round it passes unverified. A loop the cap ended passes, and the payload says so (`done_by`, `fixed_at_cap`, `stale_waived`). `--force` (overwriting real tasks, skipping those checks) is an operator decision.

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

`tp import` and `tp add` are lenient — `"4. Backend Migration"` (without prefix) is also accepted when unambiguous and is auto-normalized. Use the full canonical form when the same text appears at multiple heading levels (both `## Setup` and `### Setup` exist) — otherwise the entry is ambiguous. A `tp validate` warning (error under `--strict`) fires for every ambiguous or unresolvable entry, and `tp add` refuses (exit 1, v1.31.0) an entry that names no heading of the spec, so a typo'd anchor is never silently equivalent to no anchor.

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

**Task audits first.** When `task_audit_roles` lists code roles, each closed task owes a task audit
of its own files; while one is owed the audit does not converge (`--status` names
`task_audits_owed`). Run each: `tp audit <spec> --task-audit` emits the batch due (null when none),
one sub-agent per prompt writes its `output_path`, then `tp audit <spec> --task-audit --record-inbox`.
A FAIL is fixed and `--resolve`d `fixed` in its `task-audit-<seq>.ndjson`. `tp run` schedules these
itself. Details: REFERENCE.md, "Task Audit".

1. `tp audit <spec>` — one prompt per active auditor role, each with a JSON-array checklist and its
   files. **spec-coverage is the only auditor id that changes routing**: it takes the spec-derived
   checklist; every other role gets one `file_check` item per file of the shared code-file list
   (ranked by churn and cut to ten when auto-detected; cuts are named in `notices`). When nothing
   changed in the working tree or since the last tag — the normal end of implement on an untagged
   repository — it audits the files the done tasks' commits touched, cut the same way (a
   `tasks-fallback` notice). `--affected-from-tasks` and `--affected-files` name the list instead;
   either named form goes uncut into each code role's one prompt, and a long one is read shallowly.
2. Spawn one sub-agent per role prompt; each writes one NDJSON line per checklist item (`status` ∈
   PASS/PARTIAL/FAIL) to its `output_path`.
3. Merge and record: `tp audit --merge r1.ndjson ... -o results.ndjson && tp audit <spec> --record results.ndjson`.
   Rows sharing `role`+`item_id` collapse only when their verdicts agree (`conflicts` names the
   rest). A row not `PASS` is a finding; whether the round is **clean** is `audit_converge_on`'s
   answer (below).
4. Fix the code for every non-PASS row, then dispose of each in the recorded round file `--status`
   names (`audit_rounds[].file`): `tp audit <round file> --resolve <0-based index|role:item_id> <fixed|wontfix|duplicate> "<evidence>"`,
   or `--resolve-all`. An evidenced `wontfix`/`duplicate` clears its round; `fixed` waits for the
   next round's re-read. **A non-PASS row stays carried until a later round grades it PASS or it is
   accepted** — empty rounds do not clear it, at the cap too; an acceptance carries onto the same row
   while its `evidence_file` is unchanged. Accepting an audit finding without a code change is the
   operator's (`accept-finding`).
5. Repeat. `--status` reports the verdict, `next_action`, `carried_blocking_rows` and the task-audit
   counts; field list in REFERENCE.md, "State Directory".
6. **Read the divergence signal.** `--status` and `--record` carry `role_streaks`,
   `spec_coverage_clean_rounds` (`null` when the latest round measured no conformance — not `0`) and
   `divergence`, emitted when spec-coverage reached its clean-round threshold while other roles still
   hold open findings (REFERENCE.md, "Honest Convergence Signals").

   **`divergence` gates nothing — it is reporting only.** It changes no convergence arithmetic, no stored per-round `clean` flag, no `next_action` and no `--status --check` exit code: audit convergence counts every non-PASS row the resolved policy does not accept, less the rows accepted `wontfix`/`duplicate` with evidence. When `divergence` appears, **surface it and stop** — accepting findings outside spec conformance is the operator's decision.

**Scope the audit, or it will not converge.** Neither value of `audit_converge_on` reads whether a
row is about the spec or the codebase at large, and general lenses (`go-safety`,
`maintainability-conventions`, `ax-contract`) always find *something*. Watch `spec-coverage`
separately (`spec_coverage_clean_rounds`, `divergence`): once it is clean for two rounds and no
round ever produced a FAIL, findings outside the spec's surface are backlog — the operator accepts
them with `tp audit <round file> --resolve <role:item_id> wontfix "<evidence>"`, naming the version
that will take them, and ships. Keep audit repairs minimal: a new abstraction is the next version's
task.

**Run the round cheaply; the probes are what find every `FAIL`, so save on repeating them, never on
cutting them.**

1. **One clone, one binary**, built by the orchestrator before the role stage and handed to every role
   with "do not clone, do not build"; a mutant copies that tree.
2. **One repair unit per independent item**; two items share one only when they edit the same lines.
3. **Delta re-grade**: the brief lists the previous round's `PASS` rows whose `evidence_file` the
   repairs since that round's record commit did not touch; the role re-records those verbatim and
   re-measures the rest. The orchestrator derives the list.
4. **Give the brief the class-to-slug table before round 1**: a finding of an already-routed class
   is recorded `PARTIAL` with its slug and expects no repair.

**`audit_converge_on`** (`all` by default, or `blocking`) decides what a round must be clean of:
under `all`, every non-PASS row; under `blocking`, only rows whose severity is `error` or that tp
cannot grade. A round is graded under the policy it was recorded with, so the field must resolve
**before audit round 1**. Setting `blocking` is an operator decision (fenced under `TP_UNATTENDED`;
escalate `audit-converge-on`), because it gates on the auditor's own self-declared severity.
`tp audit --merge` reports `by_severity`. Rules, sinks and how to choose: REFERENCE.md, "Workflow
Fields".

## Workflow E: Epic graph (plan above the spec)

A project's plan of epics lives in `.tp/epics.json` beside `config.json`, committed like it. tp lints,
reviews and writes the file; `tp epic export --md` prints the table a prose document embeds. Schema,
lint rules, payloads, state directory and exit codes: REFERENCE.md, "Epic Graph".

**Build it one topic at a time:**

1. **Interview the topic** — its milestones, their epics, and for each edge *why* it exists. `id`s
   stay the project's own; `title`, `acceptance` and `why` keep their language.
2. **`tp epic set milestones='<json>'`** only when a new milestone is needed. `milestones`,
   `opens_after`, `gates` and `tag_checks` **replace the whole value**, so start from the current one;
   `gates.<m>.<field>=` edits one gate. An edge that must never exist is a `forbidden_edges` rule
   (no `set` key: by hand or `tp epic import`), which then refuses every write that adds it.
3. **`tp epic add --bulk <ndjson>`** — the topic's epics, all rows or none, in any order; a single
   `tp epic add` needs its targets to exist. A write that adds a lint error exits 1 and writes nothing.
4. **`tp epic lint`** — read the warnings the writers do not refuse; exit 0 means no error.
5. **Review the touched slice** — `tp epic review --epics <ids> --name <topic>` (one slice that grows
   with the topic), or `--milestone <m>` / `--epic <id>`. A milestone slice asks every epic in it and
   its gate, so a topic spread over several milestones costs a round per milestone; `--milestones <m,...>` or `--all` emit
   several, each recorded on its own. Spawn one sub-agent per role prompt writing its `output_path`
   (empty when it finds nothing), record with `tp epic review <slice> --record-inbox`, fix the graph
   or `--resolve` each finding, and repeat until `tp epic review <slice> --status --check` exits 0.
   The completion gate is `--milestones <every milestone> --status --check`: `--all --status --check`
   reads only slices that have state.
   - While a round is out its dispositions are frozen; changing one is the operator's `--discard`.
   - A gate's `threshold` or `purpose` adds a `scope:<m>` unit judged by `gate-scope`; after a gate
     edit read `stale_units` and `purpose_stale_units`. A purpose naming what the milestone releases
     gets that release's obligations checked.
   - Unchanged units carry; a re-asked unit sees its earlier `wontfix`/`duplicate` decisions. After a
     round that fixed only non-blocking findings and left nothing open, the next runs the
     regression role alone.
   - Each `fixed` costs one more round, so a cap of 3 is two repairs and one clean round; on a large
     slice the operator may set `review_max_rounds` 4–5 before round 1. A slice whose graph changed
     after its cap reads `grantable: true`, and the operator's `--grant-round` buys one round.
   - `review_converge_on: blocking` (default) lets a slice finish with medium/low findings open.
   - **Stop rule:** when the finding count has not fallen for two rounds and the new findings ask
     design questions an edge cannot settle (how an operator decision is built, not which epic owns
     it), stop rather than raise the cap — a larger cap will not converge it. Resolve them `wontfix`
     with a pointer to where the design is taken up, or leave them open, and decide outside the graph.
   - A class raised across slices (`tp epic review --report --all`) points at a lint rule or a
     `direct` tag check, the operator's to add.
   - **Descriptions** (an epic's intent, scope, non-goals, open questions; behaviour belongs in its
     spec): long text through `tp epic apply` set rows, short with `tp epic set <id>
     description=<text>`. An edit re-asks only that epic and is a round against the cap, so describe a
     set of epics at a time as its own slice, and read `asked_block_bytes` before emitting. On a large
     slice each role sub-agent emits its own prompt with `tp epic review <slice> --role <id>`.

Then take the next topic. Every graph writer (`add`, `set`, `remove`, `import`, `link`, `unlink`,
`milestone add`, `apply`, `--dry-run` included) and `--record` exits 2 under `TP_UNATTENDED`: a unit that needs the plan changed
records `tp escalate --decision other`.

**Tracking the work.** An epic's status is derived when read: `planned`, `claimed`, `specced`,
`in_progress` or `done`, from its bound files' `tp resume` phases or a done record.

1. **Read what can start:** `tp epic ready` lists startable work epics ranked by `downstream`, and,
   apart, claimed epics and what waits on the operator; `--all` lifts the open-milestone rule.
   `tp epic status [--milestone <m>] [--open]` adds `critical_path`, per-milestone gates, blocked
   lists and throughput; `tp epic export` is all of it as one JSON contract — read it rather than
   tp's state files.
2. **Claim to reserve:** `tp epic claim <id> [--by <name>]` before planning an epic (`--release`
   gives it back); it runs under `TP_UNATTENDED`. Whose job an epic is, is its `owner` (handles from
   the graph's `owners`, `tp epic set <id> owner='["ana"]'`); `--owner <h>` / `--unowned` filter
   `ready` and `status`.
3. **Bind when you start:** `tp init <spec> --epic <id>` or `tp set --epic <id>` once the epic's spec
   exists. Write one spec per epic (a family shares a design note), so a later epic's dependencies
   never hold back an earlier one's tasks; the status follows the file and the claim is ignored. `--epic ""`
   unbinds.
4. **The epic block:** while a direct dependency of a bound file's epic is not done, `tp next`,
   `tp ready` and `tp claim` offer no task (`blocked_by_epics`; `tp next` exits 4, `tp claim` 1) and
   `tp done` refuses a task not already `wip`. With no `wip` task, `tp resume` reports `epic-blocked`:
   stop and report it. When every blocker is a condition, `operator` names the
   `tp epic done <c> --evidence` records — the operator's decision, never yours. An any-of group holds
   until one member is done (`blocked_by_any_of`, `operator.choices`).
5. **Gates:** a milestone opens when every milestone it opens after is open with a green gate — every
   member done and, for a gate with a `threshold`, a met record. `tp epic gate <m>` exits 0 only when
   green; the operator records `tp epic gate <m> --met --evidence "<text or URL>"`. List under
   `deferrable` work that waits on a trigger that may never come. `tp epic scope <m>` shows what a
   gate pulls in; its `fingerprint` tells whether a scope pass is due. A placeholder threshold is
   listed under `operator.threshold_rewrites`: writing the exit condition is the operator's, so a
   unit reports it rather than inventing the text.
6. **Opening order:** a milestone opens after the previous one of its track; `tp epic set
   opens_after=<json>` changes that. After every `opens_after` or `milestones` write, confirm each
   `ancestry_changed` `lost`/`gained` entry is meant, and read `partial-parallel` warnings.
7. **Conditions:** an external precondition is an epic with `condition: true` whose `acceptance` says
   what it waits for; it takes no task file and no claim and is done only by
   `tp epic done <id> --evidence "<text or URL>"` (`--undo` removes the record), which also closes an
   unbound work epic finished outside tp. `tp epic ready` asks for it only when it is `actionable`
   (`--all` lists the rest with `not_actionable` reasons). A milestone that opens on something outside
   the plan keeps `opens_after: []` and holds its trigger as a condition epic of its own milestone,
   which its dependency-free epics depend on; a `tag_checks` `provided_by` rule makes lint refuse an
   epic that forgets the edge. Never put the trigger alone in an earlier milestone: an empty gate is
   green (`gate-empty`).
8. **Long-lead epics:** place one in the earliest milestone where it can start, with its real
   dependencies, not in the milestone it must finish by.

Under `TP_UNATTENDED` the operator's records exit 2, removals included: `tp epic done` and `--undo`
escalate as `epic-done`, `tp epic gate --met` as `gate-met`, `tp init --epic` and `tp set --epic` as
`epic-binding`.

## Reset-native workflow & commit strategy

Every unit — a review round, decomposition, one task, one audit round — runs in a **fresh context**;
tp is the durable state machine between resets. The reset is the orchestrator's job (a CLI cannot
clear its caller's context); tp guarantees resumability. A unit returns to the loop only at a clean
checkpoint; a crashed unit is recovered on the next `tp resume`, never lost.

### The oracle and the loop

**`tp resume [spec]`** — the phase-agnostic oracle, read-only. From durable state alone (task file, spec, `.tp-review/`, `.tp/local.json`, git) it returns `{phase, spec, changes, kept, bookkeeping, next_units, round, last_failure, next_action, blockers}` (plus `guidance` in the implement phase, and `running_units` beside a live rolling pool: then `next_units` is empty and `next_action` carries no command, since the run starts the next units itself).

- `next_units` (the ordered `{kind, id, brief_command}` list to execute now), `round` (`null` outside a round-based phase) and `phase` are the machine surface a driver parses. `last_failure` is the advisory `{unit_kind, unit_id, phase, exit_code, summary, at}` record of the previous attempt's wall, or `null`; it never changes which unit runs next.
- Phase is task-first: an open task reads `implement` even when the spec is stale (a `spec-stale` blocker fires, the phase does not revert). A `spec-stale` blocker clears only through the round its message names — emit, run the roles, record: a `--record` with no emission of its own certifies no reading and does not clear it.
- `next_action.payload` embeds the next unit's work, so a fresh agent needs one call. `next_action.brief_command` names the unit's command per phase — `tp next --brief` in implement, `tp review <spec> --round N` in review, `tp audit <spec>` in audit — and is null while an escalate blocker stands and in decompose.
- `bookkeeping` lists the tp-owned dirty files a close legitimately leaves under `commit_strategy: hc` — commit them; they never count as `unexplained-changes`.

The field-by-field output table and the blocker vocabulary are in [REFERENCE.md](REFERENCE.md). **The loop** — what a driver does, whether tp runs it or you do:

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

### `tp run` — tp's own driver

**`tp run [spec]`** reads the cycle through the same path `tp resume` uses, stops if the cycle is releasable, takes `next_units`, spawns one runner process per unit, re-reads the state **from disk**, checks the caps, and loops. The three role kinds (`review-role`, `audit-role`, `task-audit-role`) run concurrently, every prompt a round emits having a unit (the built-in `regression` prompt included); with `run_max_parallel` above 1 implement units run as a pool, each in its own git worktree, integrated as it exits while its slot is refilled (REFERENCE.md, "Implement pool"); every other kind runs alone.

- A unit's result is whatever it wrote to disk: the driver reads a child's exit code and one spend number, nothing else it said.
- It exits **0** only on `stop_reason: converged` and **4** on every other stop reason. A run-scoped lock at `.tp/locks/run-<key>.lock` (`<key>` the task file's base, or its directory and base escaped into one name, `x%2Fa`, for one below the root) makes a second `tp run` over the same task file exit 4.
- `tp run --dry-run` prints the units it would spawn without spawning or locking (beside a live pool, none, with the pool's under `running_units`); `tp run --status` reports the current or last run.
- **Unattended units cannot take the user's decisions.** Every child carries `TP_UNATTENDED=1`, so each decision in the Gate, Budget & Escalation Policy table fails closed with exit 2 and a hint naming `tp escalate`; every epic graph writer escalates as `other`, while `tp epic claim` runs. The unit records the decision (`tp escalate --decision <name> --evidence <text> [--option <text>]…`, which writes `$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json` and exits 2), the run stops with `stop_reason: escalation`, and the operator decides and runs `tp run` again. Nothing is replayed, and an escalating unit is not counted as a failed attempt. Per-sink rules: REFERENCE.md, "`TP_UNATTENDED`".

The ten unit kinds, the ten stop reasons, the run state file, the child environment, the `runner` field (templates `claude` and `opencode`, or a runner object for any other command) and `notify_cmd` are in [REFERENCE.md](REFERENCE.md).

### Coordinating by hand

Where no runner is configured, or a human belongs in the loop, drive the same loop yourself: each "FRESH unit" is a sub-agent (in Claude Code, the Agent tool: clean context, one unit, work persisted to disk), and the orchestrator re-orients via `tp resume` between units. Sub-agents do not nest, so the orchestrator does each review/audit round's per-role fan-out itself. For a full reset of the driver too, use the `/clear` + `tp resume` loop or headless `claude -p` per unit.

**Brief the unit; do not remember what to inject.** A fresh unit does not inherit the orchestrator's history. For strictly serial work tell it to run `tp next --brief` (claims the task and returns the brief); otherwise run `tp brief <id>` (read-only) and put its output in the spawn's prompt. The brief carries **identity** and the one-unit-then-stop rule, the **scope fence** (acceptance-only — no refactor/rename/cleanup, no hand-editing the task file or `.tp-review/`; an out-of-fence finding is reported as an `Out of scope:` line), **prior work** (each dependency's evidence and the files its commits touched), the verbatim **acceptance** + `spec_excerpt`, and the **close recipe** for the effective `commit_strategy` (the `--` separator before a dash-leading reason, the evidence-line format, and under `hc` that `tp commit`/`--auto-commit`/a bare `tp done` are rejected). Inject only what tp cannot know: runtime setup, such as which tools the sub-agent should use (native Read/Edit are hook-blocked in some repos → the project's MCP toolset), and any live gotcha the brief does not carry.

**Spawn from the brief, so the spawn claims.** `tp brief <id>` carries the marker `tp-task: <id> <task file>` (the `marker` key of its JSON when piped, the first line in a terminal) and a fresh claim token (`tp-claim-token: <token>`, `claim_token`). One task per spawn: the plugin's Agent hook reads the marker and claims the task for the spawn before the agent starts (`claimed_by`, `started_at`), recording the token, and a refused claim (done, blocked, another spawn holds it) refuses the spawn. Its SubagentStop hook refuses the agent's first stop while the task is neither `done` nor `waiting`, naming the `tp done` and `tp set ... waiting=` commands with their `--file`. When the holder is gone, `tp claim <id> --takeover` (frees the claim, keeps `started_at`, retires the old brief's token — spawn again from a fresh `tp brief <id>`) or `tp unclaim <id>` is the way out. Inside `tp run` both hooks stand aside: the driver accounts for its units.

**Worktree agents land through the coordinator.** Spawn each agent in its own worktree (Claude Code's Agent tool with `isolation: "worktree"`, or `git worktree add`), its prompt holding the brief whole. The agent commits with git and runs `tp done <id> --commit <sha> --claim-token <token> -- "<evidence>"`, as its close recipe says; `tp commit` and `--auto-commit` are refused there. A claim that carries a token refuses a close without it (exit 2: rerun with the token the brief names) and a close from an agent a takeover replaced (exit 4), before its gate runs. The close gates the code and leaves a land request; the coordinator's `tp land` picks the commits onto its branch and closes the task there, so a dependent starts only once the code is in. Then the agent stops: once the task is landed or reopened, a close in that worktree exits 4, and the coordinator respawns the task from a fresh `tp brief <id>`. A park (`tp set waiting=`) goes to the copy that holds the claim. The close lands whether or not the claim was committed before the worktree was cut, provided the coordinator works in the main worktree; a coordinator in a linked worktree leaves the claim uncommitted until `tp land`.

**Keep the pool full.** A session that claimed through spawns is a coordinator. Set `run_max_parallel` at the project layer first (`tp set --workflow --project run_max_parallel=<n>`): it is the hand pool's cap too, default 1. The plugin's second `Stop` hook runs `tp pool --stop-hook` when the coordinator's turn ends and refuses the stop once, a finished task first:

- while a land request waits, with `tp land --file <file>`, slot or no slot; a landing that fails reopens the task, and `tp land` names the brief to spawn a fresh agent from;
- while fewer agents run than `run_max_parallel` allows and a task is ready (or a claim lost its agent), with each task's `tp brief <id> --file <file>` — the tasks `tp run` would start beside the running ones, so one whose `touches` overlap a running task's waits. Spawn them in the background; a claim whose agent is gone is freed with `tp claim <id> --takeover --file <file>` first.

A task that must not start yet for a reason that is not a task (a release tag, a person's decision) is held with `tp set <id> waiting="<why>"` until a claim or `waiting=""` lifts it, and the hook stops asking about it; an agent waiting on its own long job parks its wip task the same way and keeps its slot. The second stop in a row passes, and a session that never claimed is never asked.

**The plugin.** tp publishes a Claude Code plugin at the repository root (`.claude-plugin/plugin.json` beside `marketplace.json`, this `skills/tp`, `hooks/`, `agents/`); **the Go binary is not in it** — installation stays Homebrew or `go install`, and the `SessionStart` hook preflights `tp`'s presence and version and fails with the install command. Besides the coordinator hooks above, `PreToolUse` on the write tools denies hand-writes to `.tp-review/` contents, `*.tasks.json`, `.tp/config.json` and `.tp/local.json`, and the first `Stop` hook refuses a role unit's stop once while its findings file is missing and it wrote no escalation record. `agents/` declares `tp-implementer`, `tp-reviewer`, `tp-auditor` and `tp-decider` (the `decision` unit) with **tool restrictions only**; the role's content reaches the unit through the emitted prompt. Every hook declares a 10-second `timeout`. A plugin update reaches a session only when it starts: after `claude plugin update tp@tp` start a new session. A runtime that cannot load a plugin runs the same units without it: the restrictions are defence in depth, and the brief plus the tp commands are the durable contract.

**From a non-Claude runtime** the loop is the same: `tp run`'s `runner`, or your own fan-out with whatever sub-agent primitive the runtime has (a Task tool, headless `agent -p`, a forked process, a fresh session), each unit given its brief. Configure three hazards up front:

1. **Permission prompts truncate the loop.** A headless runtime that auto-denies a read prompt leaves a review/audit sub-agent with empty findings — a **false clean round**. Allow the file reads tp's units need, or run them where prompts can be answered.
2. **Fan-out depth.** If the runtime's sub-agents do not nest, the orchestrator runs a round's fan-out inline; `tp resume` still re-orients between rounds.
3. **Merged streams corrupt `--json`.** tp writes every advisory, error and hint to **stderr** and only the payload to stdout. A runtime that captures one combined stream breaks that above tp, where `2>/dev/null` cannot help: an advisory such as `round N file … is missing; skipping its rows` then appears before the JSON. Capture the streams separately, or run tp with `--quiet`.

### Commits

**`commit_strategy`**:

- `builtin` — tp commits (`tp commit`, `tp done --auto-commit`, `tp done --commit`).
- `hc` — the agent commits with `hc`, then records via `tp done --commit <sha> [--commit <sha> …]`.
- `auto` — `hc` when on `PATH`, else `builtin`.

It is authored at `tp init`; the project default is set with `tp set --workflow --project commit_strategy=<builtin|auto|hc>`, while the task-file setter is read-only (`tp set --workflow commit_strategy=…` exits 2 naming the project setter). A task closed with commit(s) records `commit_shas` (ordered; `commit_sha` mirrors `[0]`); `tp done --commit` is repeatable, and a duplicate sha exits 1.

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
| `tp found add --location <file:line> --reproduction <cmd> --expected <text> --observed <text>` | Record one request for a defect outside the caller's task; changes no task, prints the request's id; exit 2 on a bad shape |
| `tp found add ... --question <text> --alternative <text>` | A request that needs a choice, in place of `--expected`/`--observed`; `--alternative` repeatable |
| `tp found add ... --blocks [--task <id>]` | The caller's task cannot pass without it; needs a wip task the caller works (an implement unit's own, or `--task`) |
| `tp found add ... --audit-row <role:item_id>` | From an audit-role unit: a Fix-shaped finding on an item the in-flight round did not ask; recording the round disposes that row naming the request, which is applied only then; an asked item exits 2 (it takes the audit-fix path) |
| `tp found answer <request> task --expected <text> --observed <text> --cite <file> --quote <text>` | The decision unit's answer to an undecided request: applied like a Fix-shaped request (never a duplicate of the one it answers; past `found_max` an open queue entry); `--location`/`--reproduction` default to the request's; exit 2 when the quote is not in the cited file |
| `tp found answer <request> conforms --reproduction <cmd> --cite <file> --quote <text>` | The expected behaviour already holds: no task, a decided queue entry; `--cite` is the spec, a `CLAUDE.md`, `CONVENTIONS.md` or `REFERENCE.md`, quoted verbatim (whitespace-normalised) |
| `tp found answer <request> human` | Everything else: an open queue entry with the request's question and alternatives |
| `tp found settle --task <entry> --reject <entry>=<reason>` | The person settles open queue entries in one batch: a task in the spec's task file (not counted against `found_max`; the entry's finders now depend on it) or a rejection with its reason (its finders are ready again); each flag repeatable; an entry without expected/observed also takes `--expected <entry>=<text> --observed <entry>=<text>`; exit 2 and nothing written on an unknown, non-open or repeated entry |

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

**Operator decisions are never the agent's own.** Under `tp run` every unit carries
`TP_UNATTENDED=1`, and each of these exits 2 with a hint naming `tp escalate`; by hand, outside a
run, stop and ask. The `--decision` name to record is in brackets:

| Decision | Name |
|---|---|
| `--skip-gate` on `tp done`/`tp close`, or changing `quality_gate` (a rewritten gate is a skipped one) | `skip-gate` |
| raising `review_max_rounds` / `audit_max_rounds` | `raise-review-cap` / `raise-audit-cap` |
| `tp import --force` | `import-force` |
| `audit_converge_on` resolving to `blocking` | `audit-converge-on` |
| accepting a critical/high review finding, or any audit finding, without a change | `accept-finding` |
| discarding an unrecorded emission (`--force` on an emit, `tp epic review --discard`) | `discard-emission` |
| an epic's done record, a gate's met record, a task file's epic binding | `epic-done`, `gate-met`, `epic-binding` |
| one epic review round past the cap (`--grant-round`) | `grant-round` |
| an epoch whose cut holds an unresolved blocking row (`--rewritten`) | `rewrite-past-blocking` |
| raising a `run_max_*` cap, setting `worktree_setup_cmd`, `runner` or `notify_cmd` | `other` |

Record it with `tp escalate --decision <name> --evidence "<what you found>" [--option "<a way forward>"]`;
the run stops with `stop_reason: escalation` for the operator. **An escalation is a normal outcome**,
neither a failed attempt nor a recorded round.

- **The round cap** (default 3; `0` uncaps): at the cap emission and `--record` refuse (exit 4), and
  the loop is **done** (`done_by: cap`) once every finding of the latest round is dispositioned and
  no blocking row is carried from an earlier round. A blocking finding marked `fixed` at the cap
  keeps the loop open (`blocking_fixed_at_cap`): no round re-reads the repair, so the operator
  accepts it or raises the cap.
- **Convergence differs by phase.** Review: no surviving critical or high (`review_converge_on`,
  default `blocking`), medium/low acceptable with evidence. Audit: `audit_clean_rounds` clean rounds
  under `audit_converge_on` (default `all`: every non-PASS row counts). In both, a blocking row
  leaves only by a fix a later round verifies, an acceptance, or an epoch — never by an empty round.
- **A unit runs a full suite only through `tp gate`.** Inside `tp run`, run targeted package tests
  directly and the whole suite as `tp gate`, which takes one of the run's gate slots
  (`run_max_parallel_gates`).
- **Verify the gate with separate exit codes, never a `&&` chain**: `<tests>; echo $?`, then
  `<linter>; echo $?`.

## Class & Checks Guidance

- **Fill `class`** on every review finding: a kebab-case slug naming its failure class, the key `--merge` clusters on.
- **Mechanization candidate:** a class that appears in ≥ 2 distinct rounds OR ≥ 5 times in a single round (`mechanize_candidates` in `--report` and `--record`). Write a detector and register it: `tp set --workflow checks='[{"class":"<slug>","cmd":"<detector>"}]'`. A check is **only worth registering when the artifact it measures already exists in the review phase** — checks run in the review phase only; when the subject is written later, make running the check the acceptance of the task that creates it.
- Once registered, tp runs the check every review round (`mechanical_checks`) and tells reviewers to stop reporting that class. Exit **0** passed, **1** found violations, anything else could not run (`ran: false`) and suppresses nothing. `tp review --status --check` requires every check to pass.
- **A check reads its spec from `TP_SPEC`**: tp runs every check at the task file's repository root with `TP_SPEC` naming the spec, relative to that root when the spec lies under it and absolute otherwise; a `TP_SPEC` or `TP_ROUND` the caller carried is dropped first, and with no spec `TP_SPEC` is unset, never empty. Write the argument as `"${TP_SPEC:?}"`, never derive the spec from `tp resume`, which names the active task file's spec.
- **Registration retires the suggestion:** a registered check retires its mechanize candidate — the class leaves `mechanize_candidates`, the hint and `next_action`, and `--record` names it in `mechanized_classes`. Under `--record` and `--status --check` only a check that ran (exit 0 or 1) retires its class; plain `--status` runs no check. `over-specification` is never added to the prompts' do-not-report list, since tp's own prompts ask for it.

## Role Corpus & frontmatter overrides

Review and audit roles are **project-owned data** — one JSON file per role under the repo-root `.tp/`:

- `.tp/reviewers/*.json` drives `tp review`; `.tp/auditors/*.json` drives `tp audit`. Schema `{id, title, instructions, focus[], domains[], context}`; `id` equals the filename stem; `regression` is reserved. Commit the corpus.
- A populated phase directory **replaces** the embedded defaults for that phase; absent or empty keeps them (software: implementer/tester/architect + spec-coverage/security/maintainability-conventions; prose: coherence/soundness + spec-coverage/soundness).
- `tp init --eject-roles [--domain software|prose] [--force]` writes the defaults as editable files, and **ejected role files are not rewritten on upgrade** — re-eject with `--force` to adopt newer prompts.

`tp review` emits one prompt per active reviewer role plus the built-in `regression`; `tp audit` one per active auditor role. tp only emits prompts — it never executes agents. Spec frontmatter steers the corpus:

```yaml
---
tp:
  domain: prose          # selects & filters the corpus
  review_roles:          # append focus questions to an existing role
    implementer:
      focus: ["Can each section be written without inventing facts not in the outline?"]
  audit_roles:
    spec-coverage:
      focus: ["Is every outline element present and fully developed?"]
---
```

- `domain` selects the embedded corpus and filters a user corpus by each role's `domains`; an unknown domain falls back to `software` with a lint warning.
- `tp.review_roles`/`tp.audit_roles` **append** focus to a role; an unknown id is a lint warning.
- enabled: false deactivates a role for one spec (`enabled` is the override's other key). Emptying a phase or deactivating `spec-coverage` is refused at emission (exit 2). Keep `domains` as a role's standing scope and `enabled: false` as the one-spec exception.
- `overlap_report` flags a `trim_candidate` (a role that found only what others found) and `roles_stale` a corpus changed since the last round; trimming is your decision.

Validation, refusal strings, hashes and the two trim levers: REFERENCE.md, "Role Corpus".

## Role authoring guidance

1. **One distinct failure-lens per role**; overlapping roles waste tokens and show up as trim candidates.
2. **A reviewer reports what would make the implementation wrong** — requirements that contradict each other or the goal, or cannot be satisfied. Detail the code will settle (exit codes, field names, helper names) is not a finding; adversarial probing belongs in the audit.
3. **Evidence demand**: every finding has a `§`-anchored `location`; a `high` carries a counterexample in `evidence` (two contradicting spec lines, or a command and its output).
4. **Finding nothing is a normal result** — say so, or the role reaches for a minor point.
5. **The role customizes only its `focus`**, never the finding schema tp injects.
6. **Altitude**: push toward decidable invariants a task's acceptance can verify. The class `over-specification` names a detail only code can establish, prescribed in the spec; it is usually low or medium, and its remedy is moving the detail into task acceptance.

Example lenses — software: buildability (`implementer`), testability (`tester`), consistency (`architect`); auditors `spec-coverage`, `security`, `maintainability-conventions`. Prose: coherence vs soundness. Legal: obligation completeness vs loopholes; PRD: journey completeness vs measurable acceptance; data schema: referential integrity vs migration; academic: claim support vs methodology.

## Recording language — read in any language, record in English

A unit reads the spec in its author's language and judges it there; translating first loses the
precision a review is for. Everything it **records** is English: finding and audit row text, `class`
slugs, closure reasons and commit messages — they are committed, the next round clusters on them
byte-for-byte, and a second language splits one idea across two slug families. The emitted prompt
does not state this, so an orchestrator that spawns units says it when it spawns them.

## `--role` — one prompt at a time

`tp review <spec> --role <name>` and `tp audit <spec> --role <name>` emit one role's prompt; it is a
unit's own first command. A name the invocation emits gives exactly that prompt; a name tp
recognises in either phase's corpus but does not emit this round exits 0 with `prompts: []`; an
unknown name exits 2 and writes nothing. Legal modes and the payload: REFERENCE.md, "`--role`
payload contract".

## Where judgement-shaping text belongs

Standing instructions that shape judgement go in per-spec `focus` frontmatter or the project's role
files, both recorded via `roles_hash` — never only in the orchestrator's wrapper, which tp cannot
see. The wrapper is for runtime setup. Framing it must carry anyway is declared with
`--harness-note "<text>"` on `--record`, and `--status` reports `harness_stale` when it changed.

## `next_action` — the single next step

`tp review`/`tp audit` `--status` and `--record` return a `next_action` by a fixed precedence; it is advisory and gates no exit code:

1. **Done** (converged, or the cap reached with every latest finding dispositioned) → review: decompose and `tp import`; audit: proceed to release.
2. **The cap reached with a finding undispositioned, or a blocking finding open** → disposition it, or (below the cap) revise the spec or code. Accepting a blocking finding is named as the operator's decision.
3. **A `mechanize_candidates` class recurs, none blocking** (review only) → register a check, then run the next round. A check is **only worth registering when the artifact it measures already exists in the review phase**. A class whose check is registered and ran falls through to step 4.
4. **Clean but not yet converged** → run the next round.

Under `--record` and `--status --check`, a registered check that failed or could not run replaces any step that names import with that check to fix. Plain `--status` runs no check, so while a check is registered its import step starts with `tp review <spec> --status --check`.

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
