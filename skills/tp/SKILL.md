---
name: tp
description: Spec-to-task lifecycle manager for AI coding agents. Interviews user to resolve ambiguities, decomposes specs into atomic tasks, manages execution order via dependency graph, and batch-closes with evidence. Use when user wants to implement a spec, plan tasks, decompose a feature, or a *.tasks.json file exists.
---

# tp — Task Plan Skill

Activates when: a `.tasks.json` file exists, user asks to implement a spec/plan/tasks, or user references tp commands.

## Workflow A: Decompose (spec exists, no .tasks.json)

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

Then collect convergence parameters:
- "How many consecutive clean review rounds? (default: 2)" — integer 1-10, re-ask once if invalid, announce "Invalid input — using default of 2" on second failure, use default on skip.
- "How many consecutive clean audit rounds? (default: 2)" — same rules.

Announce: "I will review until N clean rounds, audit until M clean rounds." Hold values in memory until `tp init`.

If new ambiguities arise during spec writing, pause and return to step 3. Do not re-ask convergence params.

### Step 1: Spec → Decompose

1. `tp lint <spec.md>` — fix issues, review `structured_elements`
2. Review loop — `tp review` with sub-agents until convergence (see Convergence Enforcement below)
3. Decompose into tasks — **you are the decomposer, tp validates your output**
4. Backward pass — every table row and numbered list item → task acceptance; `tp validate` for line coverage
5. `tp import tasks.json` — validates and stores
6. After `tp init`, run `tp set --workflow review_clean_rounds=N audit_clean_rounds=M` if non-default

### Decomposition Rules

**You are the decomposer — tp validates your output.**

1. **Atomicity**: Each task = 1 commit, 1 verb, 1-15 min estimated
   - ≤3 acceptance criteria, ≤8 word title (no conjunctions), ≤2 source_sections
   - If >3 criteria, split by concern axis
2. **Concern axes** for splitting: types/models → logic/engine → validation → CLI/wiring → tests → docs
3. **Structured elements** (from `tp lint`): every table row, numbered list item, code block → some task's acceptance
4. **Source lines**: every task MUST have `source_lines` as a range: `"15-42"` or `"15-42,50-60"`
5. **Dependencies**: types before logic, logic before CLI, CLI before tests
6. **Preview before import**: list proposed tasks and ask for confirmation

## Workflow B: Execute (tasks exist)

```
plan=$(tp plan --minimal --json)  # ONE call for full plan
# For each task: implement → quality gate → tp commit <id> "evidence" → tp done <id> "evidence" --gate-passed --commit <sha>
# Or: tp done <id> "evidence" --gate-passed --auto-commit
# Or: batch close via tp done --batch results.ndjson
```

After all tasks done, run audit loop — `tp audit spec.md --json`, spawn sub-agents — until convergence (see Convergence Enforcement below). `tp audit` generates prompts; you spawn sub-agents and collect results.

## Workflow C: Resume (some tasks done/wip)

Same as B. `tp plan` excludes done tasks, puts WIP first. Convergence enforcement applies equally — see below.

## Closure Rules

Before closing a task (`tp done`):

1. Re-read acceptance criteria from the plan output
2. Verify implementation matches the FULL spec (not just acceptance summary)
3. Write reason addressing EACH criterion with file paths as evidence
4. Never use: "deferred", "covered by existing" (without proof), single-word reasons
5. Use `--gate-passed` to relax keyword matching — evidence like "2559 tests pass" is accepted
6. Use `--covered-by <id>` when work IS done but in a different task (not a deferral)
7. `tp done` auto-claims open tasks — no separate `tp claim` needed
8. Code snippets in spec may be illustrative — validate against actual codebase before implementing

## Convergence Enforcement

**NON-NEGOTIABLE:** You MUST NOT proceed to decomposition until you have completed N consecutive review rounds with zero findings (any severity), where N = `workflow.review_clean_rounds` (default: 2). A single clean round is insufficient — consecutive clean rounds confirm the spec is stable. Do not skip rounds, summarize findings as "minor", or declare convergence prematurely.

**NON-NEGOTIABLE:** You MUST NOT declare implementation complete until you have completed N consecutive audit rounds with zero findings (any severity), where N = `workflow.audit_clean_rounds` (default: 2). This applies equally when resuming via Workflow C. Do not skip rounds or declare the audit passed based on your confidence in the implementation.

**NON-NEGOTIABLE:** You MUST NOT begin or continue writing the spec while unresolved questions remain. You must exhaust all questions and collect convergence parameters before starting. If you discover new ambiguities while writing the spec, pause and return to the interview phase.

## Reference

For command details, field aliases, NDJSON format, and batch operations: see [REFERENCE.md](REFERENCE.md)
