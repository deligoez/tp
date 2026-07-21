# tp v0.25.0 — User-defined reviewer & auditor roles

## 1. Overview

Review and audit roles are hardcoded in Go: `tp review` always emits the implementer / tester / architect trio (persona-swapped by domain), and `tp audit` always emits spec-coverage / security / maintainability-conventions. A project cannot add a role it needs (transaction-integrity, idempotency, pedagogy) or drop one that only produces overlap.

v0.25.0 makes the role corpus **project-owned data**. Each role is a file under `.tp/`; tp ships curated defaults and can eject them for editing. On top of that corpus tp scales the review panel across rounds (redundant lenses cost tokens without finding new issues), improves finding dedup so overlap is measurable, and retires the now-redundant `tp: lens` frontmatter. It also removes the `.tp-active` marker deprecated in v0.24.0.

This spec extends the v0.24.0 `.tp/` config mechanism; it does not replace it. `.tp/config.json` (workflow policy) and the role corpus live side by side.

## 2. Design Principles

1. **Separate data, share machinery.** Reviewers and auditors are distinct corpora (different input, question, lifecycle) but share one schema, parser, validator, dedup, and staleness code path. Duplication is the cost we refuse.
2. **tp owns the output contract; the user owns the input lens.** A role file customizes the prompt (persona, instructions, lens questions/focus); it can never redefine the finding schema (location, class, severity, PASS/PARTIAL/FAIL). Dedup, convergence, and mechanization depend on that contract being tp's.
3. **tp emits prompts; it never executes agents.** Role files are prompt specifications, not a runtime. The orchestrating agent spawns sub-agents exactly as today.
4. **Minimalism — only earned machinery.** No speculative config knobs, no fields "just in case." A threshold stays an internal constant until a real need forces it into config. Every kept surface is a maintenance cost.
5. **Excellent defaults, opt-in authoring.** A project with no role files runs the curated embedded corpus. Authoring roles is a power feature, not the default path, so aggregate review quality does not depend on every user writing good prompts.

## 3. Role Definition Schema

3.1 A role is a single JSON file. Reviewers and auditors share one schema; the **phase is inferred from the directory** (`.tp/reviewers/` vs `.tp/auditors/`) — no `phase` field is stored.

3.2 Fields:

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `id` | string | yes | Stable role id; unique within its directory; kebab-case |
| `title` | string | yes | Human label shown in prompts and progress |
| `instructions` | string | yes | The role's framing / system prompt body |
| `lens` | string[] | no | Focus questions appended to the prompt (review) or checklist-generation focus (audit) |
| `always_on` | bool | no (default false) | Never dropped by adaptive scaling (§8); e.g. regression |
| `domains` | string[] | no (default all) | Domains this role applies to; empty/absent = every domain |

3.3 The finding output schema is **not** part of a role file. tp injects the fixed NDJSON contract (review: free-form findings with `class`; audit: one row per checklist item with PASS/PARTIAL/FAIL and the category taxonomy) into every emitted prompt. A role file that attempts to declare output fields is a validation error.

3.4 `tp lint` validates role files: required fields present, `id` unique within its directory and kebab-case, `domains` a string array, no unknown top-level keys. An invalid role file aborts role-reading commands with exit 3 and a repair hint (consistent with malformed `.tp/config.json`).

## 4. Directory Layout & Discovery

4.1 Role files live at `.tp/reviewers/*.json` (spec review) and `.tp/auditors/*.json` (implementation audit), under the same repo-root `.tp/` directory as `config.json`.

4.2 Discovery reuses the v0.24.0 anchor: walk up from the current directory to the first `.git` boundary; the `.tp/` there supplies the corpus. A single deterministic anchor.

4.3 The corpus is **committed** to version control (like `config.json`): roles are shared team artifacts, and audit/review convergence enforcement must travel with the repo.

4.4 When a phase directory is absent or empty, tp uses its embedded default corpus for that phase (§5). A project therefore opts in per phase: custom reviewers with default auditors is valid.

## 5. Default Corpus & Eject

5.1 tp embeds a curated default corpus, selected by domain (§6):

| Phase | Domain `software` (default) | Domain `prose` |
|-------|-----------------------------|----------------|
| reviewers | implementer, tester, architect | coherence, soundness |
| auditors | spec-coverage, security, maintainability-conventions | spec-coverage, soundness |

5.2 A `regression` reviewer is a built-in `always_on` role, appended to every review regardless of corpus (§8 never taper it). It is not counted in the base panel sizes above.

5.3 `tp init --eject-roles [--domain <name>]` writes the selected default corpus into `.tp/reviewers/` and `.tp/auditors/` as editable files, so the previously hidden persona prompts become visible artifacts. It refuses to overwrite existing role files unless `--force`.

5.4 Ejecting is optional. It exists for inspection ("no black box") and customization; a project that is happy with defaults keeps zero role files and carries no maintenance burden.

## 6. Domain Selection

6.1 The spec-frontmatter `tp.domain` field (existing) selects which embedded default corpus applies when role files are absent, and which corpus `--eject-roles` scaffolds.

6.2 `domain` is **no longer a Go persona-swapper**. The only behavioral difference between domains is which default corpus is chosen; once role files exist, `domain` only filters roles via their `domains` field (§3.2).

6.3 Prose defaults to a leaner panel (2 reviewers + regression) because prose flaws surface from many angles at once, so additional lenses mostly overlap. Software keeps 3 because its lenses genuinely diverge.

## 7. Prompt Emission

7.1 `tp review <spec>` emits one prompt per active reviewer role (corpus filtered by domain, minus roles tapered by §8), plus the `always_on` regression role.

7.2 `tp audit <spec>` emits one prompt per active auditor role. Audit is **not** tapered (§8 is review-only): its roles are fixed-purpose and diverge, so the full panel always runs.

7.3 Emission is otherwise unchanged: same NDJSON output contract, same `--record` / `--status` / `--check` convergence machinery, same review/audit round separation.

## 8. Adaptive Review Scaling

8.1 Redundant reviewers cost tokens without finding new issues once a spec stabilizes. `tp review` reduces the **diversity panel** in later rounds based on measured inter-role overlap from the previous round.

8.2 Rule (deliberately simple): if the previous round's overlap ratio (§9.3) is at or above an internal threshold, tp drops the lowest-yield non-`always_on` reviewer from the next round's emission. The panel never falls below **one** diversity reviewer plus `always_on` roles.

8.3 `always_on` roles (regression) are never dropped. Audit is never tapered.

8.4 **Convergence safeguard:** the clean round(s) that establish convergence run the **full** corpus. tp never declares convergence on a tapered panel — a reduced round that comes back clean triggers one full-corpus confirming round before `consecutive_clean` can reach the required count.

8.5 A single knob governs this: `review_adaptive` (bool, default `true`) in `.tp/config.json` workflow. Setting it `false` always emits the full panel. The overlap threshold is an internal constant, not exposed until a concrete need justifies the config surface.

## 9. Finding Dedup by Location + Class

9.1 `tp review --merge` currently dedups by literal text similarity, which misses paraphrases and cannot measure overlap. v0.25.0 clusters findings by **(location, class)**.

9.2 Each cluster becomes one merged finding carrying `found_by`: the count of distinct roles that independently surfaced it.

9.3 The **overlap ratio** of a round is the fraction of merged findings with `found_by ≥ 2`. It feeds §8's taper decision.

9.4 `found_by ≥ K` (K an internal constant) marks a finding as a priority/mechanization candidate, extending the existing `mechanize_candidates` signal (a class independently found by many roles is a strong mechanization target).

9.5 Findings with no location, or with distinct classes, are never merged — clustering only collapses genuine duplicates.

## 10. Role Staleness

10.1 tp hashes the effective role corpus (all applicable role files' content) into `roles_hash`, stored in `state.json` next to `spec_hash`.

10.2 Editing any role file changes `roles_hash`, which marks recorded convergence **stale** — exactly as editing the spec marks it stale. Convergence cannot be declared against a corpus that changed since the last recorded round.

10.3 `tp review --status` / `tp audit --status` report `roles_stale` alongside the existing `stale` (spec) flag.

## 11. `tp: lens` Retirement & Migration

11.1 The standalone `tp: lens` frontmatter concept is retired. Its three jobs move:

1. Domain persona-swap → absorbed by domain-selected default corpora (§6).
2. Project-wide concerns → role files (§4), so they are shared once instead of copied into every spec's frontmatter.
3. Spec-local concerns → a spec-frontmatter **role override** that appends `lens` questions to a named role (or defines a spec-only role), resolved project-corpus ⊕ spec-override at read time.

11.2 Back-compat: an existing `tp: lens` block is auto-translated to the spec-layer role override with a deprecation warning. The `tp:` frontmatter and `domain` key are retained; only the standalone `lens` semantics are reframed.

11.3 The spec-layer override reuses the v0.24.0 resolve-at-read layering model (project defaults ⊕ local override); no second resolution mechanism is introduced.

## 12. `.tp-active` Removal (breaking)

12.1 The `.tp-active` marker, deprecated in v0.24.0, is removed. Discovery no longer reads it; the fallback and its deprecation warning are deleted from the discovery chain.

12.2 The root `.gitignore` line for `.tp-active` is removed, and every doc reference to it (README, SKILL, REFERENCE, CLAUDE) is deleted or updated.

12.3 Discovery precedence becomes: `--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect.

## 13. Configuration Surface

13.1 `.tp/config.json` workflow gains exactly one field: `review_adaptive` (bool, default `true`). It inherits the v0.24.0 layering (project default, no task-file override needed) and validation.

13.2 No other knob is added. Panel sizes derive from the corpus; thresholds are internal constants; staleness is automatic. Adding config beyond what a real use demands is a maintenance cost we decline (Principle 4).

## 14. Non-Goals

1. tp is not an agent runtime — it emits prompts; it does not spawn or execute agents.
2. No user-defined finding schema or category taxonomy — the output contract stays tp's.
3. No audit-side adaptive taper — audit roles diverge and the full panel always runs.
4. No parameterized overlap policy — one boolean (`review_adaptive`) only; thresholds stay internal until earned.
5. No speculative role fields — the schema (§3.2) is the whole surface; new fields require a demonstrated need.

## 15. Migration

15.1 Existing projects are unaffected until they opt in: no role files means the embedded default corpus, identical to today's hardcoded behavior. Existing `tp: lens` frontmatter keeps working via the §11.2 shim.

15.2 tp's own repo stays on the embedded default corpus — the three software reviewers and three auditors fit, and ejecting them would add role files with no benefit (Principle 4). v0.25.0's own review and audit loops dogfood the new emission/dedup/staleness code paths regardless of whether roles are ejected, because those paths run over the embedded corpus too.

15.3 The `.tp-active` removal requires no user action: v0.24.0 already migrated active pointers to `.tp/local.json`.

## 16. Role Authoring Guidance

16.1 A project-authored role is only as good as its prompt. tp ships **authoring guidance** in the skill (`skills/tp/SKILL.md`) teaching an orchestrator agent how to design a reviewer or auditor role prompt that produces high-signal, low-overlap, contract-conformant findings. This guidance is the mitigation for the quality-dilution risk of opening role authoring (Principle 5).

16.2 The guidance is grounded in researched prompt-design best practices (LLM-as-judge rubrics, adversarial critique, code-review automation) and covers, at minimum:

1. **Role = one distinct failure-lens** — a role must target a failure mode no other role covers; overlapping roles waste tokens and inflate the overlap ratio (§9).
2. **Adversarial framing** — "try to refute / find where this breaks" outperforms "check whether this is fine."
3. **Evidence demand** — every finding carries a location and a why, which is what makes dedup (§9) and audit PASS/FAIL meaningful.
4. **Scope boundaries** — state what the role does NOT cover, to keep lenses disjoint.
5. **Output-contract adherence** — the role customizes the lens, never the finding schema (§3.3).

16.3 The guidance includes worked example role sets for at least **code/software** (correctness, security, performance/contract lenses) and **prose** (narrative continuity vs expository derivability), plus a short catalog of other domains (e.g. legal/contract, product/PRD, data-schema, academic) with their characteristic diverging lenses.

16.4 The embedded default corpus (§5) is authored to exemplify this guidance, so an ejected default role is itself a worked example. A web-research pass precedes authoring the embedded corpus and this guidance section, so both reflect current best practice rather than assertion.
