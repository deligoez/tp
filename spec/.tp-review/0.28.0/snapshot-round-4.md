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
5. **Agent-agnostic core, minimal surface.** The resume oracle, the commit-record model, and the keep-list reuse tp's existing state files and add no dependency on any agent runtime in tp's own code path. `hc` is an optional, auto-detected peer tool, never invoked by tp and never a hard requirement of tp itself.

## 3. Reset boundaries and the checkpoint invariant

3.1 **The checkpoint invariant.** A reset is safe when the finished unit's own work is durable — a task closed (with its commit(s), or `--covered-by` when another task's commits cover it), or a round merged and recorded to `.tp-review/`. A task closed with a commit carries `commit_shas` (§6); a `--covered-by` close carries none, because its work lives in the covering task's commits. Under `builtin` the commit is made by `tp commit`/`--auto-commit`; under `hc` it is made by the agent and recorded with `tp done --commit`. The working tree need not be pristine: files a project deliberately keeps uncommitted (§7) are legitimate, and tp never sweeps them into a commit or discards them. What `tp resume` guarantees is recovery of anything that reached durable state (task file, `.tp-review/`, git). Work that never reached disk before a mid-unit reset is the **incomplete checkpoint** the driver avoids by returning a unit to the orchestrator only at a clean boundary (§9.2); when it does happen, `tp resume` surfaces the trace (a `wip` task with no commit, or an unexplained change) so the next agent reconciles it rather than building over it.

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

4.1 **Command, discovery, and spec resolution.** `tp resume` resolves the active task file by the existing discovery order (`--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect) and derives the spec from that task file's `spec` field. `tp resume <spec>` takes the spec argument as authoritative and uses the `<base>.tasks.json` adjacent to it; when the argument and a discovered task file's `spec` disagree, the **argument wins**. When no task file is found and no spec argument is given, `tp resume` exits 3 (`no task file found`) with a hint to run `tp init <spec>` or pass a spec. When a spec argument names a spec whose adjacent task file does not exist, tp treats the task set as empty and computes the phase by §4.3 (which yields `decompose` or `review` from review state). `tp resume` is read-only (§4.8). Output is JSON when piped or `--json`, a short human summary in a TTY.

4.2 **Output schema.** `tp resume` returns exactly these fields:

1. `phase`: one of `review`, `decompose`, `implement`, `audit`, `release`.
2. `spec`: the resolved spec path (string).
3. `changes`: an array of repo-root-relative paths (each a single file) with uncommitted changes **not** covered by the keep-list (§7) — the unexplained changes to reconcile — sorted by byte-wise codepoint order. Empty when none; empty when the directory is not a git repository (git state cannot be read).
4. `kept`: an array of `{path, reason}` — one entry per uncommitted file matched by a keep-list entry (§7), `path` the matched file (never the glob), `reason` from the first matching entry (§7.3) — sorted by byte-wise codepoint order on `path`. Empty when none match.
5. `next_action`: an object `{command, summary, payload}`. `command` is the literal next tp command as a string, or `null` for the `decompose` and `release` phases (agent work with no single tp command). `summary` is a one-line human gloss. `payload` carries the immediate work (§4.4).
6. `blockers`: an array of blocker objects (§4.6), sorted by the fixed code order; empty when the agent may run `next_action` directly.

`--compact` omits the human-facing fields — `next_action.summary`, each `kept[].reason`, and each `blockers[].message` — keeping the machine-actionable fields (including every `blockers[].data`). A clean, ready state is `changes: []` and `blockers: []`.

4.3 **Phase detection.** The phase is computed from durable state by testing task-file state first, so review convergence (which reads as false whenever the spec is stale) can never pull an in-progress project back to `review`. First match wins:

1. The task file holds tasks and at least one is not `done` → `implement`.
2. The task file holds tasks and all are `done`, and the audit sequence is not converged (or no audit round is recorded yet) → `audit`.
3. The task file holds tasks and all are `done`, and the audit sequence is converged → `release`.
4. The task file holds zero tasks (the init shell or an absent file), and the review sequence is converged → `decompose`.
5. The task file holds zero tasks, and the review sequence is not converged (or no review round is recorded yet) → `review`.

Convergence for rules 2–5 is read from the existing `tp review --status`/`tp audit --status` state and is unchanged by this release. Because a stale spec reads as unconverged, a spec edited during the zero-task stage naturally routes to `review` (rule 5) for re-review. Once tasks exist (rules 1–3), a stale spec does not change the phase — an in-progress project stays in `implement`/`audit` — and the staleness is surfaced as an `escalate`-class `spec-stale` blocker (§4.6) so a mid-implementation spec edit stops the driver for a human decision rather than silently reverting the phase.

4.4 **Embedded next-action payload.** To keep a fresh agent to one round-trip, `payload` carries pinned keys per phase. For `review`/`audit`, `round` is the 1-based number of the **next** round to record (recorded rounds + 1), and `unresolved_findings` is the count of the previous round's findings whose `resolved.status` is absent or is a value other than `wontfix` — `0` for the first round, which has no previous round:

1. `review` → `{"round": <int>, "unresolved_findings": <int>}`; `next_action.command` is the string `tp review <spec>`.
2. `decompose` → `{}` (the spec is the top-level `spec` field); `next_action.command` is `null`; `tp import` closes the phase.
3. `implement` → `{"task": {"id": "<id>", "acceptance": "<text>"} | null, "wip": <bool>}`. If any `wip` task exists the first one in task-file order is surfaced with `wip: true`; otherwise the first ready task with `wip: false`; if no task is ready but open tasks remain, `task` is `null` and the `no-ready-task` blocker (§4.6) carries the unmet dependencies. `next_action.command` is the string `tp next` (which claims and returns the full task).
4. `audit` → `{"round": <int>, "unresolved_findings": <int>}`; `next_action.command` is the string `tp audit <spec>`.
5. `release` → `{}`; `next_action.command` is `null`.

4.5 **Working-tree classification.** `tp resume` reads `git status --porcelain=v1 -z -uall` — staged and unstaged tracked changes, and every non-ignored untracked file enumerated individually (not collapsed to a directory). Every path it reports is repo-root-relative (git's `--porcelain` base), which is the base the keep-list also uses (§7.1). For each entry it derives the changed file's path: an unstaged modification, a staged change, and an untracked file each yield their one path; a **staged** rename yields the destination path only (the `-z` record's origin path is ignored); an **unstaged** rename is not a git rename status and appears as its constituent delete and untracked-add paths. Each derived path is matched against the keep-list (§7.3): a match sends the file to `kept` (with the winning entry's `reason`); a non-match sends it to `changes`. tp classifies and reports; it never commits or discards either set (§8). When the directory is not a git repository, both arrays are empty.

4.6 **Blockers.** `blockers` is an array of objects `{code, class, message, data}`: `code` is a stable slug, `class` is `agent-clearable` (the agent resolves it and re-runs `tp resume`) or `escalate` (the driver stops and hands to a human), `message` is a human gloss, and `data` is an object carrying the same facts machine-readably (never only in the prose `message`). The full vocabulary, in the fixed order tp emits them:

| code | class | when | `data` |
|------|-------|------|--------|
| `unexplained-changes` | agent-clearable | `changes` is non-empty | `{"count": <int>}` |
| `no-ready-task` | escalate | `implement`, open tasks remain but none is ready | `{"blocked_by": [<id>…]}` |
| `review-budget-exhausted` | escalate | recorded review rounds reached `review_max_rounds` without convergence | `{"cap": <int>}` |
| `audit-budget-exhausted` | escalate | recorded audit rounds reached `audit_max_rounds` without convergence | `{"cap": <int>}` |
| `spec-stale` | escalate | the review `stale` flag is set while phase is `implement`/`audit` | `{"spec": "<path>"}` |

Multiple blockers may coexist (for example `unexplained-changes` with `spec-stale`); they are emitted together in the table's order. A machine consumer keys on `code`, `class`, and `data`; the `message` is a human convenience only and is dropped under `--compact` (§4.2).

4.7 **Raising a cap is a user decision.** A `review-budget-exhausted`/`audit-budget-exhausted` blocker is `escalate`, never `agent-clearable`: the driver stops, and raising `review_max_rounds`/`audit_max_rounds` (or importing with `--force`) is a user-approved decision, consistent with tp's existing round-budget escalation policy. The reference driver (§9.2) must not clear an `escalate` blocker autonomously.

4.8 **Read-only.** `tp resume` writes no file. Running it leaves the task file, `.tp/`, and `.tp-review/` byte-identical, so it is safe to call at any point, including inside a driver loop.

## 5. Commit strategy

5.1 **Three values.** `commit_strategy` resolves (by the v0.24.0 precedence, task override > project config > built-in) to one of:

1. `builtin` — tp performs the commit itself (the pre-0.28.0 behavior). `tp commit`, `tp done --auto-commit`, and `tp done --commit <sha>` all work.
2. `hc` — tp performs no commit. The agent produces the task's commit(s) with `hc`, then records them via `tp done --commit <sha> [--commit <sha> …]` (§6). `tp commit` and `tp done --auto-commit` are rejected (§5.4).
3. `auto` — resolves to `hc` when `hc` is on `PATH`, otherwise to `builtin`. The auto-detect value: "use hc when it is installed, fall back otherwise".

5.2 **Default, resolution report, and unknown values.** The built-in default — used when no layer sets `commit_strategy` — is `auto`: on a machine with `hc` on `PATH` a project uses the hc flow, and on one without it falls back to `builtin`, so tp stays usable with no `hc` present and needs no configuration to use `hc` when it is there. A project overrides the default in `.tp/config.json`: `builtin` to always self-commit even when `hc` is installed, or `hc` to require the hc flow. Default `auto` is a deliberate behavior change from pre-0.28.0 on machines that have `hc`, gated on its presence and overridable per repo. `tp config --resolved` keeps the uniform v0.24.0 shape `{value, source}` for `commit_strategy` — `value` is the resolved name (`builtin`/`auto`/`hc`), `source` the layer that set it (or `default`). The `auto` detection result is not a config layer: plain `tp config` (non-`--resolved`) adds a top-level `commit_strategy_effective` key holding the concrete behavior (`builtin` or `hc`) after `auto` resolution (equal to `value` when `value` is `builtin`/`hc`). A `commit_strategy` **present** at a resolution layer but outside the three names (including pre-0.28.0 free-form placeholders such as `squash`) resolves to `builtin` (distinct from an absent value, which resolves to the `auto` default), and any command that reads the effective strategy (`tp commit`, `tp done`, `tp config`) prints a one-line stderr warning naming the unrecognized value; the command still exits 0.

5.3 **hc delegation is a deferral, not a shell-out.** Under an effective `hc` strategy tp does not build a commit plan and does not invoke `hc`; because tp never runs `hc`, tp needs no `hc` on `PATH` (only the `auto` resolution probes for it, §5.2). Deciding how to split the change into commits is a reasoning task the agent performs with `hc` (read `hc diff`, form the plan, run `hc run`); tp records the result — the commit SHAs (§6). tp authors no commit message under `hc`: the agent authors messages through `hc`, and the authoritative task↔commit link lives in the task's `commit_shas`, not in a message trailer. This keeps commit granularity with `hc` and the agent, and keeps tp deterministic and free of a runtime dependency in its own code path. When the effective strategy is `hc` and `hc` is not on `PATH`, tp prints a one-line stderr courtesy warning on strategy-reading commands (the agent, not tp, will fail to run `hc`); this warning changes no exit code.

5.4 **Closing a task under each strategy.** Recording a SHA is a task-file write and needs no `hc`; only *making* a commit does, and tp never makes one under `hc`. So there is a single rule, independent of whether `hc` is installed:

1. **`builtin` (including `auto` with `hc` absent).** `tp commit`, `tp done --auto-commit`, `tp done --commit <sha>`, `tp done --covered-by <id>`, and `tp done --batch <file>` all work as before.
2. **Effective `hc`.** `tp commit` and `tp done --auto-commit` are rejected with **exit 2** (usage: the command is not valid under this strategy) and the hint `commit_strategy is hc: commit with hc, then tp done --commit <sha>`, whether or not `hc` is installed. A task closes with `tp done --commit <sha> [--commit <sha> …]`, or with `tp done --covered-by <id>` for a task that needs no commit. A bare `tp done <id> "<reason>"` (no `--commit`, no `--covered-by`) is rejected with exit 2 and the same hint. In `tp done --batch`, each NDJSON row supplies its own `commit_shas` (array) or `covered_by`; a row with neither is rejected (exit 2) and the batch fails per its existing partial-failure semantics. There is no exit-4 path: tp never invokes `hc`, so `hc`'s absence never fails a tp command (it surfaces when the agent runs `hc`).

## 6. Multi-commit tasks — `commit_shas`

6.1 **`commit_shas` is canonical; `commit_sha` is a compatibility mirror.** `hc` may split one task's change into more than one commit. The task gains `commit_shas` (`[]string`, `omitempty`): the ordered commits that implement the task, the authoritative record. `tp done <id> --commit <sha>` is repeatable — `--commit a --commit b` records `["a","b"]`. Repeating the **same** sha in one close (`--commit a --commit a`) is rejected with exit 1 (`duplicate commit sha`). When `commit_shas` is non-empty, `commit_sha` is set to `commit_shas[0]` (the primary commit) so existing readers keep working unchanged; a close that records no commit (`--covered-by`) leaves both `commit_shas` and `commit_sha` empty/absent. This mirror is a deliberate, documented backward-compatibility redundancy, not an oversight. A reader that wants the full set reads `commit_shas` and falls back to `commit_sha` for a pre-0.28.0 task that predates the list (such a task has `commit_sha` and no `commit_shas`).

6.2 **`tp commit` records one.** Under `builtin`, `tp commit` and `tp done --auto-commit` make exactly one commit and record a single-element `commit_shas`. A multi-commit task arises only from the agent-driven `hc` flow (§5.3) recorded through repeated `--commit`.

6.3 **Managed field.** `commit_shas` is a managed field: `tp set` rejects it (exit 2) exactly as it already rejects `commit_sha`. `tp reopen` clears `commit_shas` alongside the fields it already clears (`gate_passed_at`, `gate_skipped_reason`, `commit_sha`).

## 7. The keep-list — durable intentional-uncommitted memory

7.1 **Storage.** `.tp/local.json` (the existing repo-scoped, git-ignored local-state file that already holds the active pointer and CLI flag defaults) gains a `keep_uncommitted` array of `{path, reason}`: `path` is a repo-root-relative file path or glob, `reason` is a short string — the note that tells a fresh agent why the file is left. It is repo-scoped, not spec-scoped, because a deliberately-uncommitted file is a property of the working tree, not of one spec's lifecycle; and git-ignored, because it describes this checkout's working state, which does not travel to a clean checkout or CI (where there is no such change to classify). Paths are stored and compared repo-root-relative — the same base `tp resume` normalizes `git status` output to (§4.5).

7.2 **`tp keep`.** tp owns writes to the list: `tp keep <path> "<reason>"` adds or updates an entry (a repeated `path` overwrites its `reason`); `tp keep --remove <path>` drops an entry (a `path` not present is a no-op, exit 0); `tp keep --list` prints the array as JSON, emitting `[]` (never `null`) when empty. The `<path>` argument is normalized to repo-root-relative before storage (so `tp keep` run from a subdirectory stores the same form `tp resume` compares against). `tp keep <path>` with no reason is a usage error (exit 2). The agent never hand-edits `.tp/local.json`. An empty or absent `keep_uncommitted` is the common case; the list fills only when a real intentional leftover exists.

7.3 **Glob dialect and how `tp resume` uses it.** A `path` entry matches by Go `filepath.Match` semantics: `*` and `?` do not cross `/`, character classes are supported, and there is no `**` recursive wildcard (a directory of kept files is named by its explicit paths or one pattern per directory level). `tp resume` matches each uncommitted file path (§4.5) against the keep-list entries in stored order; the **first** entry that matches wins and supplies the `reason` (so a file covered by several patterns yields exactly one `kept` entry). A matched file goes to `kept`; an unmatched file goes to `changes` and raises the `unexplained-changes` blocker. This classification lets the intent survive a reset: the fresh agent sees which changes are known-and-fine and which need reconciling, without asking the human again.

7.4 **Synergy with hc.** `keep_uncommitted` is the durable, cross-reset memory of `hc`'s per-run `allow_unplanned`. When the agent commits a task with `hc`, it feeds the `kept[].path` values already present in the `tp resume` output (no extra call) into the `hc` plan's `allow_unplanned` so the same files stay out of the commit. `hc` remains a stateless committer (its own soul); tp persists the intent; the agent carries it from one to the other.

7.5 **Self-healing.** An unexplained change (a path in `changes`, raising the `unexplained-changes` blocker) is resolved one of two ways: the agent commits it (it was the unit's work) or runs `tp keep <path> "<reason>"` (it is intentional). Either empties it from `changes` on the next `tp resume`, so the classification converges without a persistent nag.

## 8. Uncommitted changes are surfaced, never swept

8.1 **tp reports, it does not gate.** The checkpoint decision (§3.1) is the operator's, made from `tp resume`'s `changes`/`kept`/`blockers` and from `tp done`'s post-close warning. tp adds no hard clean-tree gate: a hard gate would break the legitimate case of files a project intends to keep uncommitted (§7).

8.2 **tp never commits or discards a change it was not told to.** Under `builtin`, `tp commit`/`--auto-commit` stage the task's files (all, or `--files`) and commit only those. Under an `hc` strategy tp makes no commit at all; `hc`'s plan-and-coverage model governs what each commit contains, so any change the agent did not plan stays in the working tree. In neither strategy does tp sweep an unrelated change into a task's commit or delete it.

8.3 **`tp done` warns, does not block.** When the working tree still carries uncommitted changes not covered by the keep-list after a successful close, `tp done`/`tp close` print a one-line stderr warning naming the count of unexplained changes; the close exits 0. A change covered by the keep-list produces no such warning. The warning informs the reset decision; the agent (via the skill) reconciles unexplained changes — commit or `tp keep` — before returning the unit.

## 9. Reference driver (external, agent-agnostic)

9.1 **tp ships no driver.** The loop that reads `tp resume`, runs a unit in a fresh context, and repeats is the orchestrator's, not tp's. Embedding it would bind tp to one agent runtime and break §2.5. tp ships the oracle and the record; the driver stays outside.

9.2 **A reference driver is documented.** `skills/tp/SKILL.md` and `README.md` describe a reference loop in runtime-neutral pseudocode: call `tp resume`; for each blocker, if its `class` is `agent-clearable` resolve it (reconcile the unexplained changes) and re-run `tp resume`, and if any blocker's `class` is `escalate` stop and hand to a human (a mid-implementation spec change, an exhausted round budget); when `blockers` is empty, run `next_action` in a fresh unit context (a sub-agent, a fresh session, or a new process — whichever the runtime provides); repeat until `phase` is `release`. The document states plainly that the reset between units is the runtime's mechanism, that a unit returns to the loop only at a clean checkpoint, and that a crashed unit is recovered on the next `tp resume` (§3.1) rather than lost.

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
8. **No `**` recursive glob.** Keep-list matching is `filepath.Match` (§7.3); recursive globbing is out of scope for this release.
9. **No `done` lifecycle marker.** The `release` phase is terminal for this release; a persisted `done` marker after a tagged release is a later feature.
10. **No new agent-runtime coupling.** tp ships no driver binary and no runtime-specific integration; the reference driver is documentation (§9).
11. **No change to review/audit convergence semantics.** `tp resume` reads the existing `--status` state; it does not change how convergence is computed or recorded.
12. **No `tp set --workflow` change for `commit_strategy`.** It stays authored by `tp init --commit-strategy` and by `.tp/config.json`; this release gives it behavior, not a new settable path.

## 12. Tests / Acceptance

12.1 **Phase detection (task-first ordering).** `tp resume` reports `implement` whenever the task file holds an open task, **even when the spec is stale** (review reads unconverged) — the stale case adds a `spec-stale` blocker but does not change the phase. With all tasks `done` and audit unconverged (or no audit round yet) → `audit`; with audit converged → `release`. With zero tasks and review converged → `decompose`; with zero tasks and review unconverged or unstarted → `review`. Each transition is asserted from durable state with no reliance on prior in-memory context.

12.2 **Discovery and spec resolution.** `tp resume` with no task file and no spec argument exits 3 with `no task file found`. `tp resume <spec>` uses the argument's adjacent task file; when a discovered task file's `spec` disagrees with the argument, the argument wins. A spec argument whose adjacent task file is absent yields `decompose` or `review` per §4.3.

12.3 **Embedded payload and command literals.** In `implement`, `payload.task` is the first WIP task (file order) with `wip: true` if any, else the first ready task with `wip: false`, else `null` with a `no-ready-task` blocker whose `data.blocked_by` lists the unmet deps; `next_action.command` is `tp next`. In `review`/`audit`, `payload` is `{round: recorded+1, unresolved_findings: <count>}` with `unresolved_findings` `0` for round 1, and `next_action.command` is `tp review <spec>` / `tp audit <spec>`. In `decompose`/`release`, `payload` is `{}` and `next_action.command` is `null`.

12.4 **Working-tree classification.** A modified tracked file, a staged file, and an untracked file each not on the keep-list appear individually in `changes` (byte-sorted) with an `unexplained-changes` blocker whose `data.count` equals `len(changes)`; a staged rename reports the destination path. Recording a path via `tp keep <path> "<reason>"` moves it to `kept` (with `reason`), out of `changes`, clearing the blocker. A glob keep-entry (`*.log`) matching two files yields two `kept` entries (the matched files, not the pattern); a file matching two entries yields one `kept` entry with the first entry's reason. A non-git directory yields `changes: []` and `kept: []`.

12.5 **Blockers.** `blockers` entries are `{code, class, message, data}`. `unexplained-changes` is `agent-clearable` with `data.count`; `no-ready-task` (`data.blocked_by`), `review-budget-exhausted`/`audit-budget-exhausted` (`data.cap`), and `spec-stale` (`data.spec`) are `escalate`. Coexisting blockers (for example `unexplained-changes` with `spec-stale` in `implement`) are emitted together in the fixed code order. A recorded review round count equal to `review_max_rounds` without convergence yields `review-budget-exhausted`.

12.6 **`tp keep`.** `tp keep a.txt "local config"` adds `{path, reason}`; a repeated `tp keep a.txt "new"` overwrites the reason; `tp keep --remove a.txt` drops it and a `--remove` of an absent path exits 0; `tp keep --list` prints the array as JSON (`[]` when empty, never `null`); `tp keep a.txt` with no reason exits 2; a `<path>` given from a subdirectory is stored repo-root-relative. `.tp/local.json` stays git-ignored and no entry is written into any task file or `.tp-review/` file.

12.7 **`--compact` and read-only.** `tp resume --compact` omits `next_action.summary`, every `kept[].reason`, and every `blockers[].message`, and keeps the other fields (including `blockers[].data`). `tp resume` writes no file: task file, `.tp/`, and `.tp-review/` are byte-identical before and after, in every reachable phase, and it produces JSON when piped.

12.8 **Strategy resolution.** `commit_strategy` resolves `builtin`/`auto`/`hc` by task-override > project > built-in precedence; an absent value at every layer resolves to the built-in default `auto`. `tp config --resolved` reports `commit_strategy` as `{value, source}` (uniform with other fields); plain `tp config` adds a top-level `commit_strategy_effective` of `hc` when `auto` and `hc` is present, `builtin` when absent. A **present** unrecognized value resolves to `builtin` (not `auto`), and any strategy-reading command prints a stderr warning and exits 0.

12.9 **Closing under each strategy.** Under `builtin` (and `auto` with `hc` absent), `tp commit <id>` and `tp done <id> --auto-commit` commit and close. Under effective `hc` — whether or not `hc` is installed — `tp commit`, `tp done --auto-commit`, and a bare `tp done <id> "reason"` are each rejected with exit 2 and the hc hint, while `tp done <id> --commit <sha>` and `tp done <id> --covered-by <other>` close; a `tp done --batch` row with neither `commit_shas` nor `covered_by` is rejected with exit 2. No commit-strategy path returns exit 4.

12.10 **`commit_shas`.** `tp done <id> --commit a --commit b` records `commit_shas: ["a","b"]` and `commit_sha: "a"`; a single `--commit a` records `commit_shas: ["a"]` and `commit_sha: "a"`; `--commit a --commit a` exits 1 with `duplicate commit sha`; a `--covered-by` close records neither `commit_shas` nor `commit_sha`. `tp set <id> commit_shas=...` is rejected with exit 2. `tp reopen <id>` clears `commit_shas`, `commit_sha`, `gate_passed_at`, and `gate_skipped_reason`. A pre-0.28.0 task with `commit_sha` and no `commit_shas` reads back its `commit_sha` unchanged.

12.11 **`tp done` warning on unexplained changes.** With an uncommitted change not on the keep-list present after a successful close, `tp done` exits 0 and prints a stderr warning naming the unexplained-change count; the change remains in the working tree unmodified (tp neither commits nor discards it), and a keep-listed change produces no such warning.
