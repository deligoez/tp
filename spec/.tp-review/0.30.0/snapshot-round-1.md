# tp v0.30.0 — The unit brief: tp briefs the unit, not the orchestrator

## 1. Overview

tp v0.28.0 made the workflow reset-native: every unit of work — a review round, one implementation task, an audit round — runs in a fresh context, and `tp resume` reconstructs the project from durable state. v0.29.0 made that state trustworthy. What neither release addressed is the moment a fresh unit actually starts: **what does the unit know when its context begins?**

Today the answer differs by phase, and measuring the emitted prompts of a live project shows the gap is wider than the phase split suggests. A review unit receives a near-complete brief. An audit unit receives the same shape minus its history. An implementation unit receives a task record: the acceptance text, the anchors, an excerpt, and a count of what remains. Everything else — how to close, what the previous units produced, what must not be touched — lives in the orchestrator's head and is re-typed, from memory, into every spawn.

| What a unit needs | review unit | audit unit | implementation unit |
|-------------------|-------------|------------|---------------------|
| Identity — what am I here to do | present | present | absent |
| Scope fence — what is out of bounds | present | present | absent |
| Input pointer — what to read | spec path, section tree | named files, contents | `spec_excerpt`, anchors |
| Prior work — what already happened | prior findings + changed sections | **absent** | absent |
| Output contract — how to finish | schema, no output path | schema, no output path | absent (no close recipe) |
| Quality rule — what "good" means | present | present | absent |
| Reset discipline — one unit, then stop | absent | absent | absent |
| Loop budget — which round, how many left | round number only | inconsistent | not applicable |

The consequences are observed, not hypothetical. Across two full external runs (opencode + GLM 5.2 driving tp end to end): a verification-style task had to be told by hand that the functions it was verifying already existed, or it would have reimplemented them; a unit with no scope fence "improved" a file outside its task and broke the quality gate; units began work without loading the workflow contract at all, because nothing in the durable state told them to; and every audit round re-derived its judgement from scratch, unaware of what the previous round had found or what had since been fixed.

This release closes the asymmetry in both directions: tp assembles a full brief for the implementation unit it currently under-serves, and gives review and audit units the history, the output path, and the stopping rule they are currently missing.

## 2. Design Principles

1. **The tool briefs the unit.** Assembling a brief from durable state is deterministic work, which makes it tp's job under P4 (*Agent Plans, Tool Executes*). An orchestrator that retypes context from memory is a lossy cache of the task file.
2. **A brief is self-sufficient, not a pointer to documentation.** The unit must be able to finish correctly having read only the brief. Instructions to "load the skill first" are advisory and get skipped; anything load-bearing belongs in the brief itself.
3. **Every phase gets the same shape.** A review unit, an implementation unit, and an audit unit differ in content, not in structure: identity, scope, input, prior work, output contract, quality rule.
4. **Prior work is what was produced, not that something happened.** A count of closed tasks carries no information. The evidence line and the files a closed unit's commits touched do.
5. **The fence is stated, and drift is recorded — never blocked.** tp states what is out of scope and records what each commit actually touched. It does not reject a commit for touching an unexpected file; it makes the drift visible to the next unit and to the audit.
6. **Every added byte replaces a byte the orchestrator was typing.** The brief is not new context; it is context that was already being sent, assembled correctly and once.

## 3. The unit brief

3.1 **Definition.** A *unit brief* is the complete, self-sufficient text a fresh agent needs to execute exactly one unit of work and finish it correctly. It is assembled by tp from the task file, the spec, `.tp-review/`, the effective workflow config, and git. It contains no information that exists only in an orchestrator's context.

3.2 **Ownership.** tp owns the brief's structure and its non-negotiable parts — the close recipe, the output contract, the scope fence, the prior-work record. A project customizes the phrasing of review and audit role identities through the role corpus (`.tp/reviewers`, `.tp/auditors`), exactly as today. The implementation brief has no role corpus in this release (§14).

3.3 **The reset contract.** A brief is valid at the moment it is produced. It is not a durable artifact and tp does not store it: re-running the command re-derives it from state. A unit that loses its context re-derives its own brief rather than asking the orchestrator to remember it.

## 4. Brief anatomy

4.1 An implementation brief has six sections, always in this order:

| Section | Content |
|---------|---------|
| Identity | What this unit is, and the one-unit-then-stop rule |
| Scope | The acceptance criteria as the boundary, plus what must not be touched (§7) |
| Prior work | What earlier units produced, selected per §5 |
| Your unit | Task id, acceptance, anchors, `spec_excerpt` |
| How to close | The exact close command for the effective `commit_strategy`, the gate, and the evidence format (§8) |
| What follows | The tasks this unit unblocks, and the phase this unit's completion moves toward |

4.2 **The identity section states the reset discipline explicitly**: this agent executes one unit and stops; it does not pick up a second task; work not on disk when it returns is lost. A fresh unit cannot infer this from a task record.

4.3 **The brief never restates the acceptance criteria in its own words.** The acceptance text in "Your unit" is verbatim from the task file, so the unit's success condition and tp's closure check are the same string.

4.4 **Default output is text**, ready to paste into a sub-agent prompt. `--json` returns `{brief, task, prior_work, close, scope, next}` where `brief` is the assembled text and the remaining fields are its machine-readable parts — mirroring how `tp review` returns both `prompts[].prompt` and structured fields.

## 5. Prior-work selection

5.1 A brief must answer "where did we get to" without paying for the project's entire history. tp selects, in this order, and deduplicates:

1. Every done task this unit transitively depends on — the work this unit builds on top of.
2. The most recently closed done tasks up to a total of five entries, for adjacent context the dependency graph does not capture.

5.2 Each selected entry carries: task id, title, the **first line** of `closed_reason` (the evidence summary the closing unit wrote), its `commit_shas`, and the files those commits touched (§6). Nothing else — a full evidence block per prior task would dominate the brief.

5.3 When a task's prior-work set is empty (the first unit of a project), the section states that this is the first unit rather than being omitted, so a fresh agent never wonders whether the section failed to render.

5.4 `--prior <n>` overrides the recency count (default 5, range 0–20). Dependency-derived entries are never dropped by this limit: a unit that builds on a prior task's output must see what that task produced. When the recency limit drops entries, the section says how many were omitted — a silent cap reads as "this is everything".

## 6. `commit_files` — what a closed unit produced

6.1 The files a task's commits touched are the most concrete statement of what it produced, and tp can record them at close time for free. On a close that records commits (`tp commit`, `tp done --auto-commit`, `tp done --commit`), tp resolves each sha's changed paths and stores them on the task as `commit_files`: a deduplicated, sorted array of repo-root-relative paths. A `--covered-by` close records none, because its work lives in the covering task.

6.2 `commit_files` is a managed field: `tp set` rejects it, `tp reopen` clears it alongside the other closure fields.

6.3 When git is unavailable or a sha cannot be resolved, the field is omitted rather than guessed, and the brief's prior-work entry falls back to the evidence line alone.

6.4 The array is capped at 50 paths per task; when a commit set exceeds it, tp stores the first 50 in sorted order and records the total in `commit_files_total`, so a fresh reader knows the list is partial.

## 7. The scope fence

7.1 **The fence is a statement in the brief, not a check in the commit path.** Every implementation brief states: implement only what this task's acceptance criteria require; do not refactor, rename, reformat, or "clean up" code outside them; do not hand-edit `<base>.tasks.json` or anything under `.tp-review/`; if a real problem is found outside the fence, report it in the closure evidence instead of fixing it.

7.2 **Out-of-fence findings have a home.** Because the fence forbids fixing, it must offer somewhere to put what was found. The evidence format (§8.3) accepts an optional trailing line beginning `Out of scope:` — a note the closing unit writes, preserved verbatim in `closed_reason`, and surfaced by `tp report` so it reaches a human rather than dying in a context window.

7.3 **Drift is visible, never blocked.** `commit_files` makes what a unit actually touched a matter of record: it appears in the next unit's prior work, in `tp show`, and in the audit phase's file selection. tp does not compare a commit's paths against the task's anchors — the mapping from a spec section to a source file is not derivable, and a wrong rejection at close time would be far more expensive than a visible record.

## 8. The close recipe

8.1 A unit that cannot close correctly has not done its job, so the brief carries the exact commands rather than a description of them. tp resolves the effective `commit_strategy` and emits the matching recipe:

| Strategy | Recipe in the brief |
|----------|---------------------|
| `builtin` | run the gate; then `tp done <id> -- "<evidence>" --auto-commit` |
| `hc` | run the gate; commit with `hc`; then `tp done <id> -- "<evidence>" --commit <sha>` (repeat `--commit` per sha) |

8.2 The recipe includes the resolved `quality_gate` command verbatim, and the rule that a red gate is never closed over — `--skip-gate` is a human decision, not the unit's.

8.3 **The evidence format is stated as a contract**, because `closed_reason` is what the next unit reads as prior work (§5.2): one line per acceptance criterion, each stating what was implemented and how it was verified; no bare "done"/"wip"/"deferred"; written in English; an optional trailing `Out of scope:` line (§7.2). The brief states that the first line becomes the next unit's summary, which is the reason to lead with the substantive claim.

8.4 The recipe names the `--` separator before the reason, and states that under `hc` a bare `tp done`, `tp commit`, and `--auto-commit` are all rejected. These are the two failures a fresh unit hits first.

## 9. Commands

9.1 `tp brief [id]` prints the brief for one unit. With no argument it targets the in-progress task, else the next ready task by the same ordering `tp next` uses. It is **read-only**: it claims nothing and mutates nothing, so an orchestrator may produce a brief before deciding to spawn. With no task available it exits 4 with the same `{done, message}` shape `tp next` uses.

9.2 `tp next --brief` claims the unit as `tp next` does today and returns the brief for it. This is the form a fresh unit runs itself: one call that both takes ownership and delivers everything needed. `--brief` composes with `--json`; `--minimal` and `--brief` are mutually exclusive (exit 2), since one strips context and the other assembles it.

9.3 `tp resume`'s `next_action` gains `brief_command`: the exact command that produces the brief for the action it is pointing at (`tp brief <id>` in the implement phase, `tp review <spec> --round N` in review, `tp audit <spec>` in audit). An orchestrator following `next_action` reaches a full brief without knowing which phase it is in.

## 10. Review and audit unit briefs

10.1 Role prompts are already briefs (§1). This release brings them to the same contract as the implementation brief without touching the role-authored prompt bodies a project has tuned: everything added here is tp-owned framing around the role's own text.

10.2 **The audit phase gets the memory the review phase already has.** A review prompt from round 2 onward embeds the previous round's findings, the changed-sections diff, and which findings were resolved. An audit prompt embeds none of these, so every audit round re-derives its judgement from zero: it cannot tell a defect it is seeing for the first time from one the previous round already reported and a later commit already fixed, and it cannot notice that something previously passing has regressed. Audit prompt emission from round 2 onward gains a **prior-round section** carrying, for each non-PASS row of the previous round: the role, the stable item id (§10.3), the status, the evidence location, and whether a commit touching that file landed after the round was recorded. The section states plainly that a prior finding is context, not a verdict to repeat — the auditor re-checks the code and records its own status.

10.3 **Audit item ids are stable across rounds.** Today an audit checklist item is identified positionally as `file-<role>-<n>`, so the same id names different subjects in different rounds — in a real run `file-go-safety-2` was one defect in round 1 and an unrelated one in round 2. That silently breaks `(role, item_id)` deduplication in `tp audit --merge`, makes cross-round comparison meaningless, and is a precondition for §10.2 to be useful at all. A file-derived checklist item's id becomes `file-<role>-<slug>`, where `slug` is derived from the item's own subject (the file path and the checklist text) rather than its position, so the same subject keeps the same id across rounds and a reordered checklist does not renumber unrelated items.

10.4 **tp names the output file.** A role prompt describes the NDJSON schema but not where to write it, so the orchestrator invents a filename and the merge glob depends on that invention. Each emitted prompt gains `output_path` — `review-r<N>-<role>.ndjson` / `audit-r<N>-<role>.ndjson`, relative to the working directory — and the prompt text names it. The merge step becomes a predictable glob rather than a convention.

10.5 **The reset discipline is stated in role prompts too**: produce findings for this round only, write them to the named file, then stop. No unit of any phase currently carries this rule.

10.6 **The loop budget is stated once, consistently.** Every emitted prompt names its round number, the required clean-round count, the consecutive clean rounds so far, and the remaining budget when a cap is set. Today the round number reaches the three diversity reviewer prompts but not the regression prompt, and reaches one audit role prompt but not the other, which leaves a fresh unit unable to tell a first pass from a convergence check.

10.7 **Per-round file reading is stated, not implied.** An audit prompt inlines file contents for the role whose checklist exhausts the reading budget first and only names the paths for the roles emitted after it. Whichever applies, the prompt says so explicitly — inlined contents are authoritative and complete, or the unit must read the named files itself before judging. An auditor that assumes it received the whole file when it received a summary produces confident wrong verdicts.

10.8 The `--record` path warns on a findings row missing `role` and names the file and line. tp already tolerates the row; the warning exists because a role-less row silently empties the per-role overlap report, which is how a real audit round lost its attribution.

10.9 **Rounds recorded before this release keep their positional ids.** tp does not rewrite recorded round files, so a project mid-loop when it upgrades holds rounds whose ids follow the old scheme. Those rounds are read without error, and a prior-round section built from one states that its ids are legacy and not comparable to the current round's — an honest gap is better than a false match between two ids that mean different things.

## 11. Duration provenance

11.1 `tp done` performs an implicit claim when a task is still open, which sets `started_at` to a moment before the close — so a unit that implements first and calls `tp done` once produces a duration near zero. In the external run, all 17 tasks were excluded from estimation accuracy for exactly this reason, and the report gave no way to tell an unmeasurable task from an instantaneous one.

11.2 Tasks gain `duration_source`: `claimed` when `started_at` came from an explicit claim (`tp claim`, `tp next`, `tp next --brief`), `implicit` when it came from `tp done`'s implicit claim. `tp report` carries it per task, excludes `implicit` tasks from `estimation_accuracy`, and reports the count separately from tasks excluded for rounding to zero.

11.3 The implementation brief's close recipe does not ask the unit to claim separately: `tp next --brief` (§9.2) already claimed it, which makes `claimed` the normal case for brief-driven units.

## 12. Token budget

12.1 A brief is only worth sending if it is smaller than what it replaces. `--compact` reduces it to the decision-critical core: prior work collapses to one line per entry (id and evidence summary, no file lists), `spec_excerpt` is omitted per the existing `--compact` rule, and the identity and scope sections shorten to a single line each. The close recipe, the acceptance text, and the scope fence's prohibitions are never dropped — they are the parts whose absence causes a wrong close.

12.2 `tp brief --json --compact` returns the structured fields with the same reductions, so a driver that assembles its own prompt pays only for what it uses.

## 13. Documentation

13.1 `skills/tp/SKILL.md` replaces its description of the orchestrator's injection duty with the brief commands: the orchestrator's job becomes running `tp brief` (or telling the unit to run `tp next --brief`) rather than remembering what to inject. The injection duty remains documented only for what tp cannot know — runtime-specific setup such as which tools the sub-agent should use.

13.2 `README.md` and `skills/tp/REFERENCE.md` document `tp brief`, `tp next --brief`, `--prior`, `brief_command`, `commit_files`/`commit_files_total`, `duration_source`, `output_path`, and the `Out of scope:` evidence line.

13.3 `CLAUDE.md`'s self-development rules adopt the brief: the subagent-per-unit recipe cites `tp next --brief` as the unit's first call, and the scope fence as the rule that prevents out-of-task changes.

## 14. Non-Goals

1. No customizable identity for the implementation brief. The role corpus stays a review/audit concept; a `.tp/brief.md` template is deferred until a project demonstrates it needs one.
2. No commit rejection on scope drift (§7.3), and no inference of which files a spec section "should" produce.
3. No stored brief artifacts. A brief is re-derived, never read back from disk, so it can never be stale.
4. tp still spawns nothing, resets no context, and runs no agent runtime. The brief is text; who receives it is the orchestrator's business.
5. No change to convergence criteria, phase detection, or the close checkpoint established in v0.29.0.

## 15. Tests / Acceptance

1. `tp brief` on a project with a ready task prints all six sections in order and claims nothing — task status is unchanged after the call (§4.1, §9.1).
2. `tp brief` with no available task exits 4 with the `{done, message}` shape (§9.1).
3. `tp next --brief` claims the task and returns the brief; `--brief` with `--minimal` exits 2 (§9.2).
4. The prior-work section lists every transitive done dependency plus recent closures to the default limit, each with its evidence first line, shas, and files; `--prior 0` keeps dependency entries and drops recency entries, stating how many were omitted (§5.1, §5.2, §5.4).
5. A project's first unit renders a prior-work section that says so (§5.3).
6. A close through each commit path records `commit_files`; a `--covered-by` close records none; `tp set commit_files=…` is rejected and `tp reopen` clears it (§6.1, §6.2).
7. A commit set larger than the cap stores 50 sorted paths plus `commit_files_total` (§6.4).
8. The brief's close recipe matches the effective strategy for `builtin` and for `hc`, names the resolved gate command, the `--` separator, and the rejected-command list (§8.1, §8.2, §8.4).
9. A closure reason with a trailing `Out of scope:` line is accepted, preserved verbatim in `closed_reason`, and surfaced by `tp report` (§7.2).
10. `tp resume` reports `brief_command` matching the phase in each of implement, review, and audit (§9.3).
11. An audit prompt for round 2 carries a prior-round section listing the previous round's non-PASS rows with role, item id, status, location, and whether the file changed since; a round-1 prompt carries none (§10.2).
12. The same audit subject keeps its item id when the checklist is regenerated after unrelated items are added or reordered, and `tp audit --merge` deduplicates two rounds' rows for that subject on `(role, item_id)` (§10.3).
13. Emitted review and audit prompts carry `output_path`, name that file in the prompt text, state the stop-after-this-round rule, and state the round number with the required and current clean-round counts (§10.4, §10.5, §10.6).
14. An audit prompt states whether the file contents it carries are complete or whether the unit must read the named files itself (§10.7).
15. `--record` warns, naming file and line, on a findings row missing `role`, and still records the round (§10.8).
16. A task claimed by `tp next` reports `duration_source: "claimed"`; one closed from open by a bare `tp done` reports `implicit` and is excluded from `estimation_accuracy` with its own count (§11.2).
17. `tp brief --compact` drops file lists and the excerpt while keeping the acceptance text, the close recipe, and the scope prohibitions (§12.1).
18. A round recorded before this release, whose item ids are positional, is read without error and produces a prior-round section that states its ids are legacy and not comparable (§10.9).
19. `go test ./...` and `golangci-lint run` are clean, and SKILL.md, README.md, REFERENCE.md, and CLAUDE.md carry the §13 updates.
