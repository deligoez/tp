# tp v0.29.0 — Loop integrity: clean-round merge, close checkpoint, entry validation, excerpt parity

## 1. Overview

tp v0.28.1 was driven end to end through a foreign agent runtime (opencode 1.17.5 + GLM 5.2) across the full lifecycle — spec review, decomposition, subagent-per-unit implementation, and audit — on three throwaway projects. tp completed every phase, and the run exposed a set of defects that tp's own dogfooding never hits because tp's driver knows the workarounds by heart.

Three of them break the workflow's *happy path*, not its edges:

1. **The clean round cannot be merged.** `tp review --merge` refuses an all-empty input set — which is exactly what a converged round produces — so the documented merge → record chain is unreachable at the moment the loop succeeds.
2. **A close never lands a clean checkpoint.** The task-closing commit records the task file in its pre-close state, so every close leaves tp's own bookkeeping uncommitted and the next `tp resume` reports it as an unexplained change. Under a reset-native driver this costs one manual reconciliation per unit.
3. **Bad tasks enter the file silently.** `tp add` accepts a task with no `id`, no `title`, and no source anchor, writes `null` where every other command writes `[]`, and never fills the coverage block — so the documented `init` + `add` path yields a file that can never pass `tp validate`.

This release fixes those, plus the transparency and conformance gaps the same run surfaced. It adds no workflow concepts and no new phases: every change is a correctness, transparency, or contract fix on a command that already exists.

## 2. Design Principles

1. **The goal state is a first-class path.** The state the loop is trying to reach — a clean round, a closed task, a converged phase — must be the best-tested path, not the one that errors.
2. **A close is a checkpoint or it is nothing.** After a close through tp's own commit path, tp-owned state is committed and the working tree is clean; a fresh agent inherits no cleanup.
3. **Reject at entry, not at validate.** A task that cannot be addressed, anchored, or executed never enters the file; `tp validate` reports drift, not damage tp itself allowed in.
4. **Silence is a bug.** Every omission tp makes on purpose — a skipped role, an excluded attribution, a filtered file type — is stated in the output that omits it.
5. **Exit codes are a contract.** 0 success, 1 validation, 2 usage, 3 file, 4 state. A caller branches on them; a wrong code is a wrong answer.
6. **No new knobs unless a knob removes a round-trip.** Each field added here replaces an inspection the agent would otherwise perform itself.

## 3. Clean-round merge

3.1 **Empty is a result, not an error.** `tp review --merge` and `tp audit --merge` succeed when the inputs contain zero findings: exit 0, an empty output file at `-o` (created, zero bytes), empty stdout in NDJSON mode, and a summary whose `merged_count` is 0. The merge → record chain therefore works unchanged on a clean round.

3.2 **Failure stays failure.** Only genuinely unusable input fails: a missing or unreadable input file exits 3, and zero input files exits 2. Malformed or incomplete lines inside a readable file continue to be skipped with a stderr warning and never change the exit code.

3.3 The resulting matrix:

| Input | Exit | Output file | Summary |
|-------|------|-------------|---------|
| Files with findings | 0 | findings NDJSON | `merged_count` > 0 |
| Files present, all empty | 0 | created, empty | `merged_count` 0 |
| Files present, only malformed lines | 0 | created, empty | `merged_count` 0, warnings on stderr |
| An input file missing or unreadable | 3 | not written | error object |
| No input files given | 2 | not written | error object |

3.4 **Recording a clean round is unchanged.** `tp review <spec> --record <empty-file>` already succeeds and counts the round as clean; §3.1 only removes the step that prevented producing that file.

## 4. Mode-scoped positionals

4.1 `tp review` and `tp audit` change meaning by mode, and the spec positional belongs only to the modes that address spec state. In `--merge` and `--resolve`/`--resolve-all` modes a spec-looking positional is **never** consumed as data: tp exits 2 with a usage error naming the correct form. Today `tp audit <spec> --merge a.ndjson b.ndjson` silently parses the spec as an input file (reporting `input_files: 3` and one warning per spec line), and `tp review <spec> --resolve f.ndjson 0 fixed "why"` reads the index as the disposition.

4.2 The spec positional stays **required** for `--record`, `--status`, `--report`, `--verify`, and plain prompt emission, which are all spec-scoped.

4.3 **Finding indices are 0-based**, and say so: the `--resolve` usage line, `--help` text, and the merged-findings output all state the base. A non-numeric index is a usage error (exit 2) naming the expected form rather than an "invalid status" error.

## 5. The close checkpoint

5.1 **A tp-made commit contains the task file in its post-close state.** Under `commit_strategy: builtin`, `tp commit` and `tp done --auto-commit` today commit the task file while the task is still `wip`, then write `status`, `closed_at`, `gate_passed_at`, and `commit_sha`/`commit_shas` afterwards — leaving the tree dirty after every close. After this release the closing commit contains the final state: tp creates the commit, writes the closure record, and folds it into that same commit via `git commit --amend --no-edit`, guarded by both (i) `HEAD` still being the commit tp just created and (ii) no path other than the task file differing from that commit. If either guard fails, tp instead creates a follow-up commit `chore(tp): record <id> closure`. Either way `git status` is clean for tp-owned paths when the command returns, and the recorded sha is the sha the task file carries.

5.2 **Under `hc`, tp classifies instead of committing.** tp never runs `hc`, so `tp done --commit <sha>` legitimately leaves tp-owned files modified. `tp resume` gains a `bookkeeping` array — uncommitted files that are tp's own state (`<base>.tasks.json`, anything under `.tp-review/`, anything under `.tp/`) — reported separately from `changes`, never counted as an `unexplained-changes` blocker, and each entry carrying the closure or round it records. `next_action` is unaffected; the human-facing summary names the pending bookkeeping so the agent commits it with the project's commit tool.

5.3 **The lock file leaves the working tree.** The task-file lock moves from the sibling path `<base>.tasks.json.lock` into `.tp/locks/<base>.lock`, which the `.tp/.gitignore` tp already writes covers. On first write tp removes a stale sibling lock left by an earlier version. tp never stages a path under `.tp/locks/`, and `tp commit`/`--auto-commit` refuse to stage any path ending in `.tasks.json.lock` even when the caller passes it via `--files`.

5.4 `.tp/.gitignore` is created by `tp init` rather than lazily by the first `tp keep`, so it is committable with the project's initial tp state instead of appearing mid-loop as a new uncommitted file.

## 6. Entry validation

6.1 `tp add` applies the same rules `tp import` applies, at entry:

| Rule | Behavior | Exit |
|------|----------|------|
| `id` missing, blank, or whitespace-only | rejected, hint names the field | 1 |
| `id` already present in the file | rejected, hint names the existing task | 1 |
| `title` missing or blank | rejected, hint names the field | 1 |
| `acceptance` missing or blank | rejected, hint names the field | 1 |
| Neither `source_sections` nor `source_lines` | rejected, hint names both anchors | 1 |
| `depends_on` referencing an unknown task | rejected, hint lists the unknown ids | 1 |
| Task JSON is not valid JSON | rejected as usage, decoder detail in `hint`, not in `error` | 2 |

6.2 **Slices are normalized on write.** Every task tp writes carries `[]` rather than `null` for `depends_on`, `source_sections`, and `tags`, whichever command wrote it. A file is never left with a mix of both.

6.3 The same normalization applies to command output: `tp lint`'s `structured_elements.tables` and `numbered_lists`, and `tp stats`'s `tags`, are `[]` when empty.

## 7. Coverage is computed on every write

7.1 `AutoFillCoverage` runs after any command that changes the task set or a task's anchors — `add`, `import`, `remove`, and `set` of `source_sections`/`source_lines` — whenever the spec file is readable. Today only `tp import` calls it, so the `tp init` + `tp add` path documented in the project's own QA setup produces a file that fails `tp validate` with `total_sections is 0 but spec has N headings` and no way to repair it short of a round trip through `import`.

7.2 When the spec cannot be read, the coverage block is left untouched and `tp validate`'s coverage finding gains a hint naming the unreadable spec path.

## 8. `spec_excerpt` parity for `source_sections`

8.1 `source_sections` is the mandatory anchor and `source_lines` is optional precision, yet only `source_lines` produces an excerpt today — so the typical task ships no spec text and the agent must open the spec itself, which is the round trip `tp plan` exists to remove. When a task has `source_sections` and no `source_lines`, `spec_excerpt` is the content of those sections: for each entry, its heading line plus the section body up to the next heading of the same or shallower level, in the order the task lists them, joined by a blank line.

8.2 The existing 2000-character cap applies to the assembled excerpt, truncating at the cap with the existing marker. A section name that matches no heading contributes nothing and is reported by `tp validate`'s existing reference check, not by the excerpt.

8.3 Excerpt parity applies everywhere an excerpt is emitted today: `tp plan`, `tp show`, and `tp next` (including `--peek`, which currently returns an empty string). `--compact` continues to omit `spec_excerpt` entirely.

## 9. Role and attribution transparency

9.1 **A corpus role that produces no prompt is named.** `tp review` and `tp audit` prompt emission adds `skipped_roles`: an array of `{role, reason}` covering every corpus role that was not emitted, with `reason` one of `no-checklist-items`, `no-spec-change`, or `domain-mismatch`. In the QA run the software auditor corpus ships three roles and tp emitted two, with nothing to distinguish an intentional skip from a misconfigured corpus.

9.2 **Attribution exclusions are stated.** `found_by_roles` deliberately excludes the built-in `regression` role, so a finding contributed only by regression carries no roles and vanishes from the overlap report while still counting in `merged_count`. The merge summary and `--status` output add `attribution_excludes: ["regression"]` so the absence reads as policy rather than data loss.

9.3 **Audit gains the overlap report.** `tp audit --merge` and `tp audit --status` emit `overlap_report` with the same `{role, unique, shared, trim_candidate}` shape as review, computed over non-PASS rows clustered by `(item_id, category)`. Review-only overlap analysis leaves audit panels with no signal for trimming a redundant role.

## 10. Loop budget and in-flight rounds

10.1 `tp review --status` and `tp audit --status` report `max_rounds` (the effective cap, `null` when uncapped) and `rounds_remaining` (`null` when uncapped) next to the existing `budget_exhausted`. A fresh orchestrator currently has to open the task file to learn how much budget is left.

10.2 **An interrupted round is visible.** `.tp-review/<base>/` holds both `snapshot-round-N.md` and `review-round-N.ndjson`; a snapshot with no round file means a round was started and never recorded. `--status` reports `in_flight_round: N` (`null` when none), and `tp resume`'s `next_action` for that state points at completing and recording round N rather than starting a new one.

10.3 **The regression prompt carries its baseline path.** The regression role's prompt names the snapshot it diffs against (`.tp-review/<base>/snapshot-round-<N-1>.md`), so a fresh sub-agent can read the baseline without the orchestrator injecting the path.

## 11. Audit file selection

11.1 When `tp audit` finds no audit-able file in the diff it exits 4, and the hint names `--base`/`--affected-files` without naming any file — leaving a fresh agent to derive the list from git itself. The exit-4 payload gains `suggested_files`: the union of paths touched by the commits recorded in `commit_shas` of done tasks, with the same type filtering the auto-detection applies.

11.2 `tp audit <spec> --affected-from-tasks` runs that derivation directly and audits the resulting file set, so the common post-implementation case needs no manual file list.

## 12. Lock contention

12.1 A write today fails immediately when another tp process holds the lock: two concurrent `tp set` calls leave one at exit 1 with no hint and no retry, which a driver running parallel units hits by accident. Write-lock acquisition retries with backoff until `lock_timeout_seconds` elapses — a workflow field, default 5, range 1–60, settable per project and per task file like the other workflow fields.

12.2 On timeout tp exits 4 (state, not validation) with a hint naming the lock path and the elapsed wait.

## 13. Exit-code conformance

13.1 The documented scheme is 0 success, 1 validation, 2 usage, 3 file, 4 state. These cases violate it today:

| Case | Today | Required |
|------|-------|----------|
| `tp add '{not valid json'` | 3, raw decoder error in `error` | 2, decoder detail in `hint` |
| Any cobra flag-parse failure (e.g. `tp done <id> "- evidence"`) | 1, bare cobra text | 2, tp error object |
| `tp done <id> "<reason starting with a dash>"` | 1, no hint | 2 with a hint naming the `--` separator |

13.2 Every error object tp emits for a rejected invocation carries a `hint` that names the next command to run, not only the rule that was broken.

## 14. Report accuracy

14.1 `tp report` divides the estimate by the measured duration whenever it is greater than zero, so a task closed within microseconds yields `actual_minutes: 0` beside `accuracy: 300000000`. When `actual_minutes` rounds to `0.0`, `accuracy` is `null` and the task carries `note: "duration below resolution"`.

14.2 Tasks with a `null` accuracy are excluded from the summary's `estimation_accuracy`, and the summary reports how many were excluded.

## 15. Lint

15.1 `empty-section` no longer fires on a container heading — one whose next heading is deeper than itself. A leaf heading with no content remains an error. tp's own documented QA spec currently fails `tp lint` with four such findings.

## 16. `commit_strategy` after init

16.1 `commit_strategy` is authored at `tp init` and cannot be changed afterwards through any documented command; `tp set --workflow commit_strategy=…` explains this clearly, while `tp set --workflow --project commit_strategy=…` answers `unknown workflow field` with a different exit code for the same concept.

16.2 The project default becomes settable: `tp set --workflow --project commit_strategy=<builtin|auto|hc>` writes `workflow.commit_strategy` in `.tp/config.json` and participates in the existing resolve-at-read order. The task-file value stays init-authored, and `tp set --workflow commit_strategy=…` keeps its current exit-2 refusal, with the hint now naming the project-level setter.

16.3 `tp config --resolved` annotates `commit_strategy` with its source layer like every other setting.

## 17. Documentation

17.1 `skills/tp/SKILL.md` documents driving tp from a non-Claude agent runtime: fan-out with whatever sub-agent primitive the runtime provides, the orchestrator's injection duty (durable-state pointer, close recipe for the effective `commit_strategy`, live operational lessons), and the rule that a headless runtime which auto-denies permission prompts will truncate a review or audit loop mid-round unless it is configured to allow them.

17.2 `README.md` and `skills/tp/REFERENCE.md` reflect every flag, field, and behavior change in §3–§16: `bookkeeping`, `skipped_roles`, `attribution_excludes`, `in_flight_round`, `max_rounds`/`rounds_remaining`, `suggested_files`, `--affected-from-tasks`, `lock_timeout_seconds`, and the project-level `commit_strategy` setter.

17.3 `CLAUDE.md`'s manual QA recipe is corrected to a path that passes `tp lint` and `tp validate` on a clean run.

## 18. Non-Goals

1. No new lifecycle phase, no change to review or audit convergence criteria, and no change to what counts as a clean round.
2. No agent-runtime-specific behavior inside tp: runtime setup guidance lives in the skill and README (§17), never in tp's code path.
3. Finding indices stay 0-based; §4.3 documents the base rather than renumbering.
4. tp still never runs `hc`, never commits on the `hc` strategy, and never discards or sweeps an uncommitted change.

## 19. Tests / Acceptance

1. `--merge` with all-empty inputs exits 0 and creates an empty `-o` file; with a missing input file exits 3; with no input files exits 2 (§3).
2. A full clean round runs `--merge` → `--record` with no manual file creation, and the round counts as clean (§3.4).
3. `tp audit <spec> --merge a b` exits 2; `tp review <spec> --resolve f 0 fixed "e"` exits 2; both name the correct form (§4.1).
4. After `tp done --auto-commit`, `git status` is clean for tp-owned paths and the committed task file shows `status: done` with the recorded sha; the amend guards fall back to a follow-up commit when `HEAD` moved (§5.1).
5. Under `commit_strategy: hc`, `tp resume` reports the modified task file in `bookkeeping`, not in `changes`, and raises no `unexplained-changes` blocker for it (§5.2).
6. No `*.tasks.json.lock` path exists after a write, a stale sibling lock is removed, and `tp commit --files <lock>` refuses it (§5.3).
7. `tp add` rejects each row of the §6.1 table with the stated exit code and a hint, and every task tp writes carries `[]` rather than `null` for its slice fields (§6.1, §6.2).
8. `tp init` + `tp add` + `tp validate` is clean on a spec whose sections are all anchored, with no `import` round trip (§7.1).
9. A task with only `source_sections` carries the section text in `spec_excerpt` in `plan`, `show`, `next`, and `next --peek`, capped at 2000 characters (§8).
10. Prompt emission lists every non-emitted corpus role with a reason; merge and status output carry `attribution_excludes` (§9.1, §9.2).
11. `tp audit --merge` and `tp audit --status` emit an `overlap_report` (§9.3).
12. `--status` reports `max_rounds`, `rounds_remaining`, and `in_flight_round`, and a snapshot without a round file makes `tp resume` point at recording that round (§10.1, §10.2).
13. `tp audit` on a fully committed tree returns `suggested_files`, and `--affected-from-tasks` audits that set without a manual list (§11).
14. Two concurrent writes both succeed within `lock_timeout_seconds`; a held lock past the timeout exits 4 with a hint (§12).
15. Each §13.1 row returns the required exit code and a `hint`.
16. A task closed in under a second reports `accuracy: null` with the note, and the summary excludes it from `estimation_accuracy` (§14).
17. `tp lint` reports no `empty-section` finding for a heading followed by a deeper heading, and still reports one for an empty leaf heading (§15).
18. `tp set --workflow --project commit_strategy=hc` writes the project default and `tp config --resolved` shows its source; `tp set --workflow commit_strategy=hc` still exits 2 and names the project setter (§16).
19. `go test ./...` and `golangci-lint run` are clean, and SKILL.md, REFERENCE.md, README.md, and CLAUDE.md carry the §17 updates.
