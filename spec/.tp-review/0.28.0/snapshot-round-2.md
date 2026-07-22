# tp v0.28.0 — Reset-native workflow: resume oracle + hc commit delegation

## 1. Overview

tp's target user is an AI agent whose context window is finite and degrades over long runs. The most reliable way to keep every unit of work in the high-success zone is to **reset the agent's context between units** — decompose, then run each atomic task in a fresh context, and start each review/audit round fresh. A context reset only works if nothing lives in the agent's memory that is not also on disk: after a reset, a brand-new agent must reconstruct exactly where the work stands and what the single next action is, losing nothing.

This release makes tp the **durable state machine** that carries that state across resets. It adds three coordinated pieces:

1. **The resume oracle (`tp resume`).** One phase-agnostic command that, from durable state alone (the spec, the task file, the `.tp-review/` directory, `.tp/local.json`, and git), reports which lifecycle phase the project is in and the concrete next action — the note a finishing agent leaves for the next one.
2. **Commit delegation (`commit_strategy`).** A commit is the save-point that makes a reset safe: a reset boundary is durable when the finished unit's own work is committed and tp state is recorded. This release gives the existing `commit_strategy` field real behavior so a project can delegate the actual committing to `hc` (the hunk-commit tool) while tp keeps ownership of the task↔commit record.
3. **The keep-list (`keep_uncommitted`).** A durable, cross-reset memory of the files a project deliberately leaves uncommitted, so a fresh agent distinguishes an intentional leftover from a unit's forgotten work without re-asking the human each reset.

tp does **not** trigger the reset itself — a CLI subprocess cannot clear its caller's context, and trying would couple tp to one agent runtime. tp guarantees the property that makes an externally-triggered reset safe: **resumability**. The reset trigger is the orchestrator's job (§9); the guarantee that no reset loses work is tp's.

## 2. Design Principles

1. **The tool is resumable; the orchestrator resets.** tp never controls agent context. It makes every unit small and every unit's state reconstructible from disk, so an orchestrator may reset at any checkpoint boundary with no loss. Whether a reset happens, and how, is outside tp.
2. **Safety lives in existing state; the keep-list adds clarity, not safety.** Forgotten work is already detectable from task state (a `wip` task with no recorded commit) and prevented by `hc`'s coverage model (every hunk is committed or explicitly left, never dropped by accident). The keep-list names deliberate leftovers so a fresh agent is not puzzled by them; it is a clarity mechanism, not the thing that keeps work from being lost.
3. **Durable state is the note to the next agent.** Commit messages, `closed_reason` evidence lines, resolve evidence, recorded rounds, `commit_shas`, and each keep-entry's `reason` are the record a finishing agent leaves behind. `tp resume` is the reader of that note. No planning lives only in a context window.
4. **Report, do not force.** tp surfaces phase, next action, and the classified working-tree state. It never spawns agents, runs `hc`, commits, or discards a change on its own. Enforcement of the reset discipline lives in the orchestrator and the skill, informed by tp's report.
5. **Agent-agnostic core, minimal surface.** The resume oracle, the commit-record model, and the keep-list reuse tp's existing state files and add no dependency on any agent runtime. `hc` is an optional, auto-detected peer tool, never a hard requirement of tp itself.

## 3. Reset boundaries and the checkpoint invariant

3.1 **The checkpoint invariant.** A reset is safe when the finished unit's own work is committed and recorded — a task closed with its commit(s) (§6), or a round merged and recorded to `.tp-review/`. The working tree need not be pristine: files a project deliberately keeps uncommitted (§7) are legitimate, and tp never sweeps them into a commit or discards them. What `tp resume` guarantees is recovery of anything that reached durable state (task file, `.tp-review/`, git). Work that never reached disk before a mid-unit reset is the **incomplete checkpoint** the driver avoids by returning a unit to the orchestrator only at a clean boundary (§9.2); when it does happen, `tp resume` surfaces the trace (a `wip` task with no commit, or unexplained changes) so the next agent reconciles it rather than building over it.

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

3.4 **The gate constrains, it does not certify freshness.** The quality gate runs at `tp done`/`tp close`, so the last close ran the gate green. This does not mean the tree is green now: an uncommitted edit made after that close is not re-gated. This is exactly why the checkpoint invariant (§3.1) requires the unit's work **committed**, not merely a past green gate — a committed, closed task is a state a fresh agent can trust; an uncommitted edit is not.

## 4. `tp resume` — the phase-agnostic resume oracle

4.1 **Command, discovery, and spec resolution.** `tp resume` resolves the active task file by the existing discovery order (`--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect) and derives the spec from that task file's `spec` field. `tp resume <spec>` takes the spec argument as authoritative and uses the `<base>.tasks.json` adjacent to it; when the argument and a discovered task file's `spec` disagree, the **argument wins** (it names the spec to resume). When no task file is found and no spec argument is given, `tp resume` exits 3 (`no task file found`) with a hint to run `tp init <spec>` or pass a spec. When a spec argument names a spec whose adjacent task file does not yet exist, the phase is computed from review state alone (§4.3, rule 1). `tp resume` is read-only (§4.7). Output is JSON when piped or `--json`, a short human summary in a TTY.

4.2 **Output schema.** `tp resume` returns exactly these fields:

1. `phase`: one of `review`, `decompose`, `implement`, `audit`, `release`.
2. `spec`: the resolved spec path (string).
3. `changes`: an array of repo-relative paths with uncommitted changes that are **not** covered by the keep-list (§7) — the unexplained changes to reconcile. Empty when the tree carries no such change. When the directory is not a git repository, `changes` is empty (git state cannot be read).
4. `kept`: an array of `{path, reason}` — the keep-list entries (§7) that currently match an uncommitted change, echoed so the next agent knows they are intentional. Empty when none match.
5. `next_action`: an object `{command, summary, payload}`. `command` is the literal next tp command as a string, or `null` when the step is agent work with no single tp command (the `decompose` and `release` phases). `summary` is a one-line human gloss. `payload` carries the immediate work (§4.4).
6. `blockers`: an array of strings from the fixed vocabulary (§4.6); empty when the agent may run `next_action` directly.

`--compact` omits the human-facing `next_action.summary` and each `kept[].reason`, keeping the machine-actionable fields. A clean, ready state is `changes: []` and `blockers: []`.

4.3 **Phase detection.** The phase is computed from durable state, first match wins:

1. The review sequence for the spec is not converged (`tp review --status` reports `converged: false`), or no review state exists yet → `review`.
2. Review is converged and the task file holds zero tasks (the init shell) → `decompose`.
3. The task file holds tasks and at least one is not `done` → `implement`.
4. All tasks are `done` and the audit sequence is not converged → `audit`.
5. All tasks are `done` and the audit sequence is converged → `release`.

Detection reads only committed/recorded state, so it is reset-stable. When the review sequence is converged but the spec has since changed (the recorded `stale` flag is set), phase stays `decompose`/`implement` per the task-file state and the staleness is surfaced as a blocker (§4.6, item 4) rather than reverting the phase — reverting would be a judgment tp does not make.

4.4 **Embedded next-action payload.** To keep a fresh agent to one round-trip, `payload` carries pinned keys per phase:

1. `review` → `{"round": <int>, "unresolved_findings": <int>}`; `next_action.command` is `tp review <spec>` (emits the round's prompts).
2. `decompose` → `{"spec": "<path>"}`; `next_action.command` is `null` (decomposition is agent work; `tp import` closes it).
3. `implement` → `{"task": {"id": "<id>", "acceptance": "<text>"}, "wip": <bool>}` — the WIP task if one exists (`wip: true`), otherwise the first ready task (`wip: false`), matching `tp next --peek --minimal`. `next_action.command` is `tp next` (claims it). When no task is ready but tasks remain open, `task` is `null` and the unmet dependencies appear in `blockers` (§4.6, item 2).
4. `audit` → `{"round": <int>, "unresolved_findings": <int>}`; `next_action.command` is `tp audit <spec>`.
5. `release` → `{}`; `next_action.command` is `null` (human-approved release).

4.5 **Working-tree classification.** `tp resume` reads `git status --porcelain` (tracked modifications, staged changes, and non-ignored untracked files each count as an uncommitted change) and partitions the paths against the keep-list (§7): a path matched by a `keep_uncommitted` entry (exact path or glob) goes to `kept` with its `reason`; every other uncommitted path goes to `changes`. tp classifies and reports; it never commits or discards either set (§8).

4.6 **Blocker vocabulary.** `blockers` is a fixed set of machine-readable strings, each naming a condition that must be cleared before `next_action` is run:

1. `unexplained changes: <n> path(s)` — `changes` is non-empty; the agent commits them (forgotten work) or records them with `tp keep` (intentional) before continuing.
2. `no ready task: blocked by <id>[, <id>...]` — in `implement`, open tasks remain but none is ready.
3. `review budget exhausted` / `audit budget exhausted` — the recorded round count reached `review_max_rounds`/`audit_max_rounds` without convergence; raising the cap is a user-approved decision.
4. `spec changed since review converged` — the review `stale` flag is set (§4.3).

4.7 **Read-only.** `tp resume` writes no file. Running it leaves the task file, `.tp/`, and `.tp-review/` byte-identical, so it is safe to call at any point, including inside a driver loop.

## 5. Commit strategy

5.1 **Three values.** `commit_strategy` resolves (by the v0.24.0 precedence, task override > project config > built-in) to one of:

1. `builtin` — tp performs the commit itself (the pre-0.28.0 behavior). `tp commit`, `tp done --auto-commit`, and `tp done --commit <sha>` all work.
2. `hc` — tp performs no commit. The agent produces the task's commit(s) with `hc`, then records them via `tp done --commit <sha> [--commit <sha> …]` (§6). `tp commit` and `tp done --auto-commit` are rejected (§5.4). This value expects `hc` on `PATH`; its absence surfaces at commit time (§5.4).
3. `auto` — resolves to `hc` when `hc` is on `PATH`, otherwise to `builtin`. The auto-detect value: "use hc when it is installed, fall back otherwise".

5.2 **Default, resolution report, and unknown values.** The built-in default is `auto`: on a machine with `hc` on `PATH` a project uses the hc flow, and on one without it falls back to `builtin` — so tp stays usable with no `hc` present and needs no configuration to use `hc` when it is there. A project overrides the default in `.tp/config.json`: `builtin` to always self-commit even when `hc` is installed, or `hc` to require the hc flow. Default `auto` is a deliberate behavior change from pre-0.28.0 on machines that have `hc`, gated on its presence and overridable per repo. `tp config --resolved` reports `commit_strategy` as `{"value": <resolved>, "source": <layer>, "effective": <builtin|hc>}` where `effective` is the concrete behavior after `auto` detection (`value` and `effective` are equal for `builtin`/`hc`). A `commit_strategy` value outside the three names (including pre-0.28.0 free-form placeholders such as `squash`) resolves to `builtin`, and any command that reads the effective strategy (`tp commit`, `tp done`, `tp config`) prints a one-line stderr warning naming the unknown value; the command still exits 0.

5.3 **hc delegation is a deferral, not a shell-out.** Under an effective `hc` strategy tp does not build a commit plan and does not invoke `hc`. Deciding how to split the change into commits is a reasoning task the agent performs with `hc` (read `hc diff`, form the plan, run `hc run`); tp records the result — the commit SHAs (§6). tp authors no commit message under `hc`: the agent authors messages through `hc`, and the authoritative task↔commit link lives in the task's `commit_shas`, not in a message trailer. This keeps commit granularity with `hc` and the agent, and keeps tp deterministic and free of a runtime dependency in its own code path.

5.4 **Closing a task under each strategy.** Recording a SHA is a task-file write and never needs `hc`; only *making* a commit does. The paths are:

1. **`builtin`.** `tp commit`, `tp done --auto-commit`, and `tp done --commit <sha>` all work as before.
2. **Effective `hc`, hc present.** `tp commit` and `tp done --auto-commit` are rejected with **exit 2** (usage: the command is not valid under this strategy) and the hint `commit_strategy is hc: commit with hc, then tp done --commit <sha>`. A task closes with `tp done --commit <sha> [--commit <sha> …]`, or with `tp done --covered-by <id>` for a task that needs no commit. A bare `tp done <id> "<reason>"` (no `--commit`, no `--covered-by`) is rejected with exit 2 and the same hint.
3. **Explicit `commit_strategy: hc`, hc absent.** `tp commit`/`tp done --auto-commit` — which would need to *make* a commit — fail with **exit 4** (state: the strategy requires `hc`, which is not installed) and a hint to install `hc` or set `commit_strategy: builtin`. `tp done --commit <sha>` (recording an already-made commit) and `tp done --covered-by <id>` still succeed — they make no commit.
4. **`auto`, hc absent.** Resolves to `builtin`; every path in (1) works.

## 6. Multi-commit tasks — `commit_shas`

6.1 **A task records a list of commits.** `hc` may split one task's change into more than one commit. The task gains `commit_shas` (`[]string`, `omitempty`): the ordered commits that implement the task. `tp done <id> --commit <sha>` is repeatable — `--commit a --commit b` records `["a","b"]`. Repeating the **same** sha in one close (`--commit a --commit a`) is rejected with exit 1 (`duplicate commit sha`). The single `commit_sha` field is retained and set to the first element of `commit_shas` (the primary commit) for backward-compatible readers; when one commit is recorded `commit_sha` and `commit_shas[0]` are identical. The redundancy is deliberate: `commit_sha` keeps existing readers (reports, tooling) working unchanged while `commit_shas` carries the full list.

6.2 **`tp commit` records one.** Under `builtin`, `tp commit` and `tp done --auto-commit` make exactly one commit and record a single-element `commit_shas`. A multi-commit task arises only from the agent-driven `hc` flow (§5.3) recorded through repeated `--commit`.

6.3 **Managed field.** `commit_shas` is a managed field: `tp set` rejects it (exit 2) exactly as it already rejects `commit_sha`. `tp reopen` clears `commit_shas` alongside the fields it already clears (`gate_passed_at`, `gate_skipped_reason`, `commit_sha`).

## 7. The keep-list — durable intentional-uncommitted memory

7.1 **Storage.** `.tp/local.json` (the existing repo-scoped, git-ignored local-state file that already holds the active pointer and CLI flag defaults) gains a `keep_uncommitted` array of `{path, reason}`: `path` is a repo-relative file path or glob, `reason` is a short string — the note that tells a fresh agent why the file is left. It is repo-scoped, not spec-scoped, because a deliberately-uncommitted file is a property of the working tree, not of one spec's lifecycle; and git-ignored, because it describes this checkout's working state, which does not travel to a clean checkout or CI (where there is no such change to classify).

7.2 **`tp keep`.** tp owns writes to the list: `tp keep <path> "<reason>"` adds or updates an entry; `tp keep --remove <path>` drops one; `tp keep --list` prints the list as JSON. The agent never hand-edits `.tp/local.json`. An empty or absent `keep_uncommitted` is the common case; the list fills only when a real intentional leftover exists.

7.3 **How `tp resume` uses it.** `tp resume` partitions the uncommitted paths against `keep_uncommitted` (§4.5): a matched path is reported in `kept` (with its `reason`) and does not count toward `changes` or raise a blocker; an unmatched path is reported in `changes` and raises the `unexplained changes` blocker (§4.6). This is the classification that lets the intent survive a reset: the fresh agent sees which changes are known-and-fine and which need reconciling, without asking the human again.

7.4 **Synergy with hc.** `keep_uncommitted` is the durable, cross-reset memory of `hc`'s per-run `allow_unplanned`. When the agent commits a task with `hc`, it feeds the `tp keep --list` paths into the `hc` plan's `allow_unplanned` so the same files stay out of the commit. `hc` remains a stateless committer (its own soul); tp persists the intent; the agent carries it from one to the other.

7.5 **Self-healing.** An unexplained change (a path in `changes`) is resolved one of two ways: the agent commits it (it was the unit's work) or runs `tp keep <path> "<reason>"` (it is intentional). Either empties it from `changes` on the next `tp resume`, so the classification converges without a persistent nag.

## 8. Uncommitted changes are surfaced, never swept

8.1 **tp reports, it does not gate.** The checkpoint decision (§3.1) is the operator's, made from `tp resume`'s `changes`/`kept`/`blockers` and from `tp done`'s post-close warning. tp adds no hard clean-tree gate: a hard gate would break the legitimate case of files a project intends to keep uncommitted (§7).

8.2 **tp never commits or discards a change it was not told to.** Under `builtin`, `tp commit`/`--auto-commit` stage the task's files (all, or `--files`) and commit only those. Under an `hc` strategy tp makes no commit at all; `hc`'s plan-and-coverage model governs what each commit contains, so any change the agent did not plan stays in the working tree. In neither strategy does tp sweep an unrelated change into a task's commit or delete it.

8.3 **`tp done` warns, does not block.** When the working tree still carries uncommitted changes not covered by the keep-list after a successful close, `tp done`/`tp close` print a one-line stderr warning naming the count of unexplained changes; the close exits 0. The warning informs the reset decision; the agent (via the skill) reconciles unexplained changes — commit or `tp keep` — before returning the unit.

## 9. Reference driver (external, agent-agnostic)

9.1 **tp ships no driver.** The loop that reads `tp resume`, runs a unit in a fresh context, and repeats is the orchestrator's, not tp's. Embedding it would bind tp to one agent runtime and break §2.5. tp ships the oracle and the record; the driver stays outside.

9.2 **A reference driver is documented.** `skills/tp/SKILL.md` and `README.md` describe a reference loop in runtime-neutral pseudocode: call `tp resume`; if `blockers` is non-empty, clear them (reconcile unexplained changes, raise a cap, and so on); otherwise run `next_action` in a fresh unit context (a sub-agent, a fresh session, or a new process — whichever the runtime provides); repeat until `phase` is `release`. The document states plainly that the reset between units is the runtime's mechanism, that a unit returns to the loop only at a clean checkpoint, and that a crashed unit is recovered on the next `tp resume` (§3.1) rather than lost.

9.3 **Round-trip minimization.** Because `payload` embeds the immediate work (§4.4), the reference driver needs one `tp resume` call per unit, not a probe across `tp status`, `tp review --status`, and `tp next`. The single entry point is what makes the driver short.

## 10. Migration and dogfood (tp's own repo)

10.1 **tp adopts the strategy it ships.** With the built-in default now `auto` and `hc` installed in tp's development environment, tp's own task commits go through the hc flow without extra configuration. The remaining implementation tasks of v0.28.0 are committed through the hc flow and closed with `tp done --commit <sha> …`, dogfooding §5–§6.

10.2 **`tp resume` drives the rest of v0.28.0.** Once `tp resume` exists, the remaining implementation and the audit loop are entered from a `tp resume` call, so the oracle dogfoods its own phase detection and payloads on tp's own task file before release.

10.3 **Documentation updates.** The self-development rule "task-closing commits always go through `tp commit`" (CLAUDE.md) and the equivalent SKILL.md/README.md guidance are updated for the `hc` strategy: under it, a task closes with `tp done --commit <sha>` after an agent-driven `hc` commit, and `tp commit`/`--auto-commit` are the `builtin`-strategy path. The `tp keep` command and the `tp resume` reference driver are documented in the same pass.

## 11. Non-Goals

1. **tp does not reset context.** No command clears, compacts, or spawns an agent context. Resetting is the orchestrator's, exclusively (§2.1).
2. **tp does not run or plan `hc`.** It neither invokes `hc` nor computes hunk assignments; commit granularity stays with the agent and `hc` (§5.3).
3. **No auto-splitting of a task by tp.** tp records the commits the agent produced; it does not itself divide a task's diff. `commit_shas` accepts a list, it does not generate one.
4. **tp authors no commit message under `hc`.** The task↔commit link is `commit_shas` (§5.3), not a message trailer; there is no message-scaffold command.
5. **No hard clean-tree gate and no sweeping.** tp neither enforces a pristine working tree nor commits or discards changes it was not told to; keeping a file uncommitted is recorded in the keep-list, not enforced by tp (§7, §8).
6. **No free-form notes store.** The cross-reset note is structured: each keep-entry's `reason`, plus tp's existing `closed_reason`/commit-message/resolve-evidence channels. There is no free-text hand-off file.
7. **The keep-list is not committed and not spec-scoped.** It lives in git-ignored `.tp/local.json` and describes the local working tree, not project history (§7.1).
8. **No `done` lifecycle marker.** The `release` phase is terminal for this release; a persisted `done` marker after a tagged release is a later feature.
9. **No new agent-runtime coupling.** tp ships no driver binary and no runtime-specific integration; the reference driver is documentation (§9).
10. **No change to review/audit convergence semantics.** `tp resume` reads the existing `--status` state; it does not change how convergence is computed or recorded.
11. **No `tp set --workflow` change for `commit_strategy`.** It stays authored by `tp init --commit-strategy` and by `.tp/config.json`; this release gives it behavior, not a new settable path.

## 12. Tests / Acceptance

12.1 **Phase detection.** With a task file whose review sequence is unconverged (or absent), `tp resume` reports `phase: "review"`; after review converges but before any task is imported, `phase: "decompose"`; with tasks present and one open, `phase: "implement"`; with every task `done` and audit unconverged, `phase: "audit"`; with audit converged, `phase: "release"`. Each transition is asserted from durable state with no reliance on prior in-memory context.

12.2 **Discovery and spec resolution.** `tp resume` with no task file and no spec argument exits 3 with `no task file found`. `tp resume <spec>` uses the argument's adjacent task file; when a discovered task file's `spec` disagrees with the argument, the argument wins. A spec argument whose adjacent task file is absent resolves the phase from review state (rule 1).

12.3 **Embedded payload, one round-trip.** In `implement`, `payload.task` equals what `tp next --peek --minimal` reports (WIP task with `wip: true` if any, else first ready task with `wip: false`); when no task is ready but open tasks remain, `payload.task` is `null` and `blockers` names the unmet dependencies. In `review`/`audit`, `payload` carries `round` and `unresolved_findings` and `next_action.command` is `tp review`/`tp audit <spec>`. In `decompose`/`release`, `next_action.command` is `null`.

12.4 **Working-tree classification.** With an uncommitted change to a path not on the keep-list, `tp resume` lists it in `changes` and raises the `unexplained changes` blocker. With that path recorded via `tp keep <path> "<reason>"`, the same change moves to `kept` (with its `reason`), leaves `changes` empty, and clears the blocker. A non-git directory yields `changes: []`.

12.5 **`tp keep`.** `tp keep a.txt "local config"` adds `{path, reason}` to `keep_uncommitted` in `.tp/local.json`; `tp keep --remove a.txt` drops it; `tp keep --list` prints the array. `.tp/local.json` stays git-ignored and the entry is not written into any task file or `.tp-review/` file.

12.6 **`--compact` and read-only.** `tp resume --compact` omits `next_action.summary` and every `kept[].reason` and keeps the other fields. `tp resume` writes no file: task file, `.tp/`, and `.tp-review/` are byte-identical before and after, in every reachable phase, and it produces JSON when piped.

12.7 **Strategy resolution.** `commit_strategy` resolves `builtin`/`auto`/`hc` by task-override > project > built-in precedence; the built-in default is `auto`. `tp config --resolved` reports `{value, source, effective}`; with `auto` and `hc` present `effective` is `hc`, with `hc` absent `effective` is `builtin`. An unknown value resolves to `builtin` and any strategy-reading command prints a stderr warning and exits 0.

12.8 **Closing under each strategy.** Under `builtin`, `tp commit <id>` and `tp done <id> --auto-commit` commit and close. Under effective `hc` with `hc` present, `tp commit` and `tp done --auto-commit` and a bare `tp done <id> "reason"` are each rejected with exit 2 and the hc hint, while `tp done <id> --commit <sha>` and `tp done <id> --covered-by <other>` close. Under explicit `commit_strategy: hc` with `hc` absent, `tp commit`/`--auto-commit` fail with exit 4, while `tp done --commit <sha>` and `--covered-by` still succeed. Under `auto` with `hc` absent, the `builtin` paths work.

12.9 **`commit_shas`.** `tp done <id> --commit a --commit b` records `commit_shas: ["a","b"]` and `commit_sha: "a"`; a single `--commit a` records `commit_shas: ["a"]` and `commit_sha: "a"`; `--commit a --commit a` exits 1 with `duplicate commit sha`. `tp set <id> commit_shas=...` is rejected with exit 2. `tp reopen <id>` clears `commit_shas`, `commit_sha`, `gate_passed_at`, and `gate_skipped_reason`.

12.10 **`tp done` warning on unexplained changes.** With an uncommitted change not on the keep-list present after a successful close, `tp done` exits 0 and prints a stderr warning naming the unexplained-change count; the change remains in the working tree unmodified (tp neither commits nor discards it), and a keep-listed change produces no such warning.
