# tp v0.28.0 — Reset-native workflow: resume oracle + hc commit delegation

## 1. Overview

tp's target user is an AI agent whose context window is finite and degrades over long runs. The most reliable way to keep every unit of work in the high-success zone is to **reset the agent's context between units** — decompose, then run each atomic task in a fresh context, and start each review/audit round fresh. A context reset only works if nothing lives in the agent's memory that is not also on disk: after a reset, a brand-new agent must reconstruct exactly where the work stands and what the single next action is, losing nothing.

This release makes tp the **durable state machine** that carries that state across resets. It adds two layers:

1. **The resume oracle (`tp resume`).** One phase-agnostic command that, from durable state alone (the spec, the task file, the `.tp-review/` directory, and git), reports which lifecycle phase the project is in and the concrete next action — the note a finishing agent leaves for the next one.
2. **Commit delegation (`commit_strategy`).** A commit is the save-point that makes a reset safe: a reset boundary is durable when the finished unit's own work is committed and tp state is recorded. This release gives the existing `commit_strategy` field real behavior so a project can delegate the actual committing to `hc` (the hunk-commit tool) while tp keeps ownership of the task↔commit record.

tp does **not** trigger the reset itself — a CLI subprocess cannot clear its caller's context, and trying would couple tp to one agent runtime. tp guarantees the property that makes an externally-triggered reset safe: **resumability**. The reset trigger is the orchestrator's job (§8); the guarantee that no reset loses work is tp's.

## 2. Design Principles

1. **The tool is resumable; the orchestrator resets.** tp never controls agent context. It makes every unit small and every unit's state reconstructible from disk, so an orchestrator may reset at any checkpoint boundary with no loss. Whether a reset happens, and how, is outside tp.
2. **Every reset boundary is a checkpoint.** A checkpoint is durable when the finished unit's own work is committed and tp state (task file, `.tp-review/`) reflects it. The commit is the save-point; `hc` (or the built-in committer) is the save mechanism.
3. **Durable state is the note to the next agent.** Commit messages, `closed_reason` evidence lines, resolve evidence, recorded rounds, and `commit_shas` are the record a finishing agent leaves behind. `tp resume` is the reader of that note. No planning lives only in a context window.
4. **Report, do not force.** tp surfaces phase, next action, and uncommitted-change status. It does not spawn agents, run `hc`, block a reset, or commit and discard changes on its own. Enforcement of the reset discipline lives in the orchestrator and the skill, informed by tp's report.
5. **Agent-agnostic core.** The resume oracle and commit-record model add no dependency on any agent runtime. `hc` is an optional, auto-detected peer tool, never a hard requirement of tp itself.

## 3. Reset boundaries and the checkpoint invariant

3.1 **The checkpoint invariant.** A reset is safe when the finished unit's own work is committed and recorded — a task closed with its commit(s) (§6), or a round merged and recorded to `.tp-review/`. The working tree need not be pristine: changes unrelated to the unit that a project deliberately keeps uncommitted are legitimate, and tp never sweeps them into a commit or discards them (under an `hc` strategy, `hc`'s own coverage model decides what a commit contains, §5.3, §7.2). `tp resume` surfaces any remaining uncommitted changes (§4.5, §7) so the operator can confirm they are intentional leftovers rather than the unit's forgotten work before resetting — the surfacing informs the decision, it does not block it. A reset taken while a unit's own work is still uncommitted is an **incomplete checkpoint** and is recovered on the next `tp resume` (§4.5), never silently lost.

3.2 **Canonical boundaries.** A unit of work is one of: a review round, the decomposition step, one implementation task, or one audit round. The boundaries at which a reset is expected are:

| Boundary | Unit that just finished | Reset |
|----------|-------------------------|-------|
| Spec review converged, before decomposition | the review loop | yes |
| Between review rounds | one review round | yes |
| Decomposition imported | decomposition | yes |
| Between implementation tasks | one task | yes |
| All tasks done, before audit | the implementation loop | yes |
| Between audit rounds | one audit round | yes |
| Audit converged, before release | the audit loop | no (human-approved release) |

3.3 **Round-level resets are safe by construction.** A review or audit round already fans out into fresh per-role sub-agents and records its findings to `.tp-review/` before the round is closed; the round's context does not carry into the next round. Resetting between rounds therefore discards nothing tp did not already record. The one unsafe point is **inside** a round — after sub-agents return findings but before `tp review --merge`/`--record` writes them — so a reset is taken at a round boundary, never mid-round.

3.4 **The gate makes every code checkpoint green.** The quality gate runs at `tp done`/`tp close`. Every implementation-task and audit-round checkpoint is therefore a green-gate state: a fresh agent resuming after such a boundary always starts from a build that last passed. Review-round checkpoints carry no code and no gate; their checkpoint is the committed spec plus the recorded round.

## 4. `tp resume` — the phase-agnostic resume oracle

4.1 **Command and discovery.** `tp resume` (optionally `tp resume <spec>`) resolves the active task file by the existing discovery order (`--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect), derives the spec path from the task file's `spec` field, and reads the `.tp-review/` state beside the spec. It is a read-only command; it never writes state. Output is JSON when piped or `--json`, a short human summary in a TTY.

4.2 **Output schema.** `tp resume` returns a single object:

1. `phase`: one of `review`, `decompose`, `implement`, `audit`, `release`.
2. `spec`: the resolved spec path.
3. `tree_clean`: boolean — whether `git status` reports a clean working tree.
4. `next_action`: an object `{command, summary, payload}` — the literal next tp command to run, a one-line description, and an embedded work payload the next agent can act on without a second call (§4.4).
5. `blockers`: an array of strings — conditions the operator must weigh before `next_action` proceeds (empty when none). Uncommitted changes at a boundary and an incomplete checkpoint (§4.5) appear here.

4.3 **Phase detection.** The phase is computed from durable state, in this order (first match wins):

1. The review sequence for the spec is not converged (`tp review --status` reports `converged: false`) → `review`.
2. Review is converged and the task file holds zero tasks (the init shell) → `decompose`.
3. The task file holds tasks and at least one is not `done` → `implement`.
4. All tasks are `done` and the audit sequence is not converged → `audit`.
5. All tasks are `done` and the audit sequence is converged → `release`.

A spec with no `.tp-review/` state and no tasks resolves to `review` by rule 1 (the loop has not started). A converged review with a taskless file, and a partially-imported file, each resolve deterministically by the ordered rules above.

4.4 **Embedded next-action payload.** To keep a fresh agent to one round-trip, `next_action.payload` carries the immediate work for the phase:

1. `review` → the pending-round context: the round number that will be recorded next, the count of unresolved findings, and the `tp review <spec>` invocation that emits the round's prompts.
2. `decompose` → the converged spec path and the reminder that decomposition is the agent's step, closed by `tp import`.
3. `implement` → the next unit exactly as `tp next --peek --minimal` reports it: the WIP task if one exists, otherwise the first ready task, as `{id, acceptance}`. When no task is ready but tasks remain open, the blocker list names the unmet dependencies.
4. `audit` → the pending audit-round context, mirroring the `review` payload for the audit sequence.
5. `release` → the release-checklist reference; no automatic action.

4.5 **Surfacing residual work and recovery.** `tp resume` surfaces conditions the next agent must weigh; it reports, it does not repair, sweep, or block:

1. **Uncommitted changes.** `tree_clean` is `false` and the changed paths are listed. `next_action` tells the agent to decide per file — commit changes that are the finished unit's forgotten work, and keep changes that are unrelated (the reference driver and skill instruct the agent to ask the human when a change is unexplained). tp neither commits nor discards them.
2. **In-progress task.** A `wip` task with no recorded commit is surfaced in the `implement` payload as a resume of that task (the existing `tp next` WIP behavior), not a new claim.
3. **Unrecorded round.** Loose per-role NDJSON in `.tp-review/` with no matching recorded round is named in the review/audit payload so the agent merges and records it rather than re-running the round.

## 5. Commit strategy

5.1 **Three values.** `commit_strategy` resolves (by the v0.24.0 precedence, task override > project config > built-in) to one of:

1. `builtin` — tp performs the commit itself (the pre-0.28.0 behavior). `tp commit`, `tp done --auto-commit`, and `tp done --commit <sha>` all work.
2. `hc` — tp performs no commit. The agent produces the task's commit(s) with `hc`, then records them via `tp done --commit <sha> [--commit <sha> …]` (§6). `tp commit` and `tp done --auto-commit` are rejected with a hint pointing to the `hc` flow. This value requires `hc` on `PATH`; its absence is an error at commit time (§5.4).
3. `auto` — resolves to `hc` when `hc` is on `PATH`, otherwise to `builtin`. This is the auto-detect value: "use hc when it is installed, fall back otherwise".

5.2 **Default and unknown values.** The built-in default is `auto`: on a machine with `hc` on `PATH` a project uses the hc flow, and on one without it falls back to `builtin` — so tp stays usable with no `hc` present and needs no configuration to use `hc` when it is there. A project overrides the default in `.tp/config.json`: `builtin` to always self-commit even when `hc` is installed, or `hc` to require the hc flow. Default `auto` is a deliberate behavior change from pre-0.28.0 on machines that have `hc`, gated on its presence and overridable per repo. A `commit_strategy` value outside the three names (including pre-0.28.0 free-form placeholders such as `squash`) resolves to `builtin` with a one-line stderr warning; the command exits 0. `tp config --resolved` reports the resolved `commit_strategy` and, for `auto`, the effective value detected (`auto -> hc` or `auto -> builtin`).

5.3 **hc delegation is a deferral, not a shell-out.** Under an effective `hc` strategy tp does not build a commit plan and does not invoke `hc`. Deciding how to split the change into commits is a reasoning task the agent performs with `hc` (read `hc diff`, form the plan, run `hc run`); tp records the result — the commit SHAs (§6). tp authors no commit message under `hc`: the agent authors messages through `hc`, and the authoritative task↔commit link lives in the task's `commit_shas`, not in a message trailer. This keeps commit granularity with `hc` and the agent, and keeps tp deterministic and free of a runtime dependency in its own code path.

5.4 **`tp commit` and `--auto-commit` under each strategy.** Under `builtin`, `tp commit` and `tp done --auto-commit` commit as before. Under an effective `hc` strategy both are rejected with exit 2 and a hint (`commit_strategy is hc: commit with hc, then tp done --commit <sha>`), and `tp done --commit <sha> …` is the sole closing path. When the effective strategy is `hc` because a project set `commit_strategy: hc` explicitly and `hc` is not on `PATH`, a commit attempt fails with exit 4 and a hint to install `hc` or set `commit_strategy: builtin` — the project opted in, so the miss is loud, not a silent fallback. Under `auto` with `hc` absent, all three behave as `builtin`.

## 6. Multi-commit tasks — `commit_shas`

6.1 **A task records a list of commits.** `hc` may split one task's change into more than one commit. The task gains `commit_shas` (`[]string`, `omitempty`): the ordered commits that implement the task. `tp done <id> --commit <sha>` is repeatable — `--commit a --commit b` records `["a","b"]`. The single `commit_sha` field is retained and set to the first element of `commit_shas` (the primary commit) for backward-compatible readers; when only one commit is recorded, `commit_sha` and `commit_shas[0]` are identical.

6.2 **`tp commit` records one.** Under `builtin`, `tp commit` and `tp done --auto-commit` make exactly one commit and record a single-element `commit_shas`. A multi-commit task arises only from the agent-driven `hc` flow (§5.3) recorded through repeated `--commit`.

6.3 **Reopen clears the list.** `tp reopen` clears `commit_shas` alongside the fields it already clears (`gate_passed_at`, `gate_skipped_reason`, `commit_sha`).

## 7. Uncommitted changes are surfaced, never swept

7.1 **tp reports, it does not gate.** The checkpoint decision (§3.1) is the operator's, made from `tp resume`'s `tree_clean` flag and path list and from `tp done`'s post-close warning. tp adds no hard clean-tree gate: a hard gate would break the legitimate case of unrelated changes a project intends to keep uncommitted.

7.2 **tp never commits or discards a change it was not told to.** Under `builtin`, `tp commit`/`--auto-commit` stage the task's files (all, or `--files`) and commit only those. Under an `hc` strategy tp makes no commit at all; `hc`'s plan-and-coverage model governs what each commit contains, so any change the agent did not plan stays in the working tree. In neither strategy does tp sweep an unrelated change into a task's commit or delete it.

7.3 **`tp done` warns, does not block.** When the working tree still carries uncommitted changes after a successful close, `tp done`/`tp close` print a one-line stderr warning that the tree is not clean; the close exits 0. The warning informs the reset decision; the operator (via the skill) asks the human when the residual changes are unexplained and keeps them when they are intentional.

## 8. Reference driver (external, agent-agnostic)

8.1 **tp ships no driver.** The loop that reads `tp resume`, runs a unit in a fresh context, and repeats is the orchestrator's, not tp's. Embedding it would bind tp to one agent runtime and break §2.5. tp ships the oracle and the record; the driver stays outside.

8.2 **A reference driver is documented.** `skills/tp/SKILL.md` and `README.md` describe a reference loop in runtime-neutral pseudocode: call `tp resume`; if `blockers` is non-empty, clear them; otherwise run `next_action` in a fresh unit context (a sub-agent, a fresh session, or a new process — whichever the runtime provides); repeat until `phase` is `release`. The document states plainly that the reset between units is the runtime's mechanism (sub-agent teardown, a new session, or a new process), that a unit must reach a clean checkpoint before it returns, and that a crashed unit is recovered on the next `tp resume` via §4.5 rather than lost.

8.3 **Round-trip minimization is a driver property.** Because `next_action.payload` embeds the immediate work (§4.4), the reference driver needs one `tp resume` call per unit, not a probe across `tp status`, `tp review --status`, and `tp next`. The single entry point is what makes the driver short.

## 9. Migration and dogfood (tp's own repo)

9.1 **tp adopts the strategy it ships.** With the built-in default now `auto` and `hc` installed in tp's development environment, tp's own task commits go through the hc flow without extra configuration; `.tp/config.json` may state `commit_strategy: auto` explicitly for clarity. The remaining implementation tasks of v0.28.0 are committed through the hc flow and closed with `tp done --commit <sha> …`, dogfooding §5–§6.

9.2 **`tp resume` drives the rest of v0.28.0.** Once `tp resume` exists, the remaining implementation and the audit loop are entered from a `tp resume` call, so the oracle dogfoods its own phase detection and payloads on tp's own task file before release.

## 10. Non-Goals

1. **tp does not reset context.** No command clears, compacts, or spawns an agent context. Resetting is the orchestrator's, exclusively (§2.1).
2. **tp does not run or plan `hc`.** It neither invokes `hc` nor computes hunk assignments; commit granularity stays with the agent and `hc` (§5.3).
3. **No auto-splitting of a task by tp.** tp records the commits the agent produced; it does not itself divide a task's diff. `commit_shas` accepts a list, it does not generate one.
4. **tp authors no commit message under `hc`.** The task↔commit link is `commit_shas` (§5.3), not a message trailer; there is no message-scaffold command.
5. **No hard clean-tree gate and no sweeping.** tp neither enforces a pristine working tree nor commits or discards changes unrelated to a unit; excluding an unrelated change from a commit is `hc`'s coverage model, and keeping it is the operator's choice (§7).
6. **No `done` lifecycle marker.** The `release` phase is terminal for this release; a persisted `done` marker after a tagged release is a later feature.
7. **No new agent-runtime coupling.** tp ships no driver binary and no runtime-specific integration; the reference driver is documentation (§8).
8. **No change to review/audit convergence semantics.** `tp resume` reads the existing `--status` state; it does not change how convergence is computed or recorded.
9. **No `tp set --workflow` change for `commit_strategy`.** It stays authored by `tp init --commit-strategy` and by `.tp/config.json`; this release gives it behavior, not a new settable path.

## 11. Tests / Acceptance

11.1 **Phase detection.** With a task file whose review sequence is unconverged, `tp resume` reports `phase: "review"`; after review converges but before any task is imported, `phase: "decompose"`; with tasks present and one open, `phase: "implement"`; with every task `done` and audit unconverged, `phase: "audit"`; with audit converged, `phase: "release"`. Each transition is asserted from durable state with no reliance on prior in-memory context.

11.2 **Embedded payload, one round-trip.** In `implement` phase, `tp resume` payload equals what `tp next --peek --minimal` reports (WIP task if any, else first ready task as `{id, acceptance}`); when no task is ready but open tasks remain, `blockers` names the unmet dependencies. In `review`/`audit` phase, the payload carries the next round number and unresolved-finding count and the `tp review`/`tp audit` command that emits prompts.

11.3 **Uncommitted-change reporting.** With uncommitted working changes, `tp resume` reports `tree_clean: false` and lists the changed paths and a `dirty working tree` blocker, and `next_action` tells the agent to decide per file. With a clean tree, `tree_clean: true` and no such blocker.

11.4 **Incomplete-checkpoint recovery.** A `wip` task with no recorded commit is surfaced as a resume (not a new claim) in the `implement` payload. Loose per-role NDJSON in `.tp-review/` with no matching recorded round is named in the review/audit payload as a merge-and-record recovery.

11.5 **Strategy resolution.** `commit_strategy` resolves `builtin`/`auto`/`hc` by task-override > project > built-in precedence; the built-in default is `auto`. With `auto` effective and `hc` present, `tp config --resolved` reports `auto -> hc`; with `hc` absent, `auto -> builtin`. An unknown value resolves to `builtin` with a stderr warning and exit 0.

11.6 **`--auto-commit` and `tp commit` gating.** Under `builtin`, `tp commit <id>` and `tp done <id> --auto-commit` commit and close. Under an effective `hc` strategy both are rejected with exit 2 and a hint, and `tp done <id> --commit <sha>` closes. Under an explicit `commit_strategy: hc` with `hc` absent, a commit attempt fails with exit 4 and an install/switch hint. Under `auto` with `hc` absent, all behave as `builtin`.

11.7 **`commit_shas`.** `tp done <id> --commit a --commit b` records `commit_shas: ["a","b"]` and sets `commit_sha` to `"a"`; a single `--commit a` records `commit_shas: ["a"]` and `commit_sha: "a"`. `tp reopen <id>` clears `commit_shas`, `commit_sha`, `gate_passed_at`, and `gate_skipped_reason`.

11.8 **Unrelated changes are kept.** With uncommitted changes unrelated to the closed task present, `tp done` succeeds (exit 0) with a stderr warning that the tree is not clean; the unrelated changes remain in the working tree unmodified — tp neither commits nor discards them — and `tp resume` then reports `tree_clean: false` with the paths listed.

11.9 **Read-only oracle.** `tp resume` writes no file: running it leaves the task file, `.tp/`, and `.tp-review/` byte-identical. It exits 0 in every reachable phase and produces JSON when piped.
