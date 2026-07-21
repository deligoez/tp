# tp v0.25.0 — User-defined reviewer & auditor roles

## 1. Overview

Review and audit roles are hardcoded in Go: `tp review` always emits the implementer / tester / architect trio (persona-swapped by domain), and `tp audit` always emits spec-coverage / security / maintainability-conventions. A project cannot add a role it needs (transaction-integrity, idempotency, pedagogy) or drop one that only produces overlap.

v0.25.0 makes the role corpus **project-owned data**. Each role is a file under `.tp/`; tp ships curated defaults and can eject them for editing. On top of that corpus tp scales the review panel across rounds (redundant lenses cost tokens without finding new issues), improves finding dedup so overlap is measurable, and retires the now-redundant `tp: lens` frontmatter. It also removes the `.tp-active` marker deprecated in v0.24.0.

This spec extends the v0.24.0 `.tp/` config mechanism; it does not replace it. `.tp/config.json` (workflow policy) and the role corpus live side by side.

## 2. Design Principles

1. **Separate data, share machinery.** Reviewers and auditors are distinct corpora (different input, question, lifecycle) but share one file schema, one parser, and one validator. The finding pipeline (dedup, staleness, convergence) is phase-parameterized, not duplicated. Duplication is the cost we refuse.
2. **tp owns the output contract; the user owns the input lens.** A role file customizes the prompt (persona, instructions, focus questions); it can never redefine the finding schema (`role`, `location`, `class`, `severity`, and — for audit — `status` ∈ PASS/PARTIAL/FAIL). Dedup, convergence, and mechanization depend on that contract being tp's.
3. **tp emits prompts; it never executes agents.** Role files are prompt specifications, not a runtime. The orchestrating agent spawns sub-agents exactly as today.
4. **Minimalism — only earned machinery.** No speculative config knobs, no fields "just in case." A threshold is a specified constant, not a config surface, until a real need forces it. Every kept surface is a maintenance cost.
5. **Excellent defaults, opt-in authoring.** A project with no role files runs the curated embedded corpus. Authoring roles is a power feature, not the default path, so aggregate review quality does not depend on every user writing good prompts.

## 3. Role Definition Schema

3.1 A role is a single JSON file. Reviewers and auditors share one schema; the **phase is inferred from the directory** (`.tp/reviewers/` vs `.tp/auditors/`) — no `phase` field is stored.

3.2 A role file's **`id` MUST equal its filename stem** (`security.json` → id `security`). This makes ids unique within a directory by construction and removes any id/filename ambiguity. `id` MUST match `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase kebab-case).

3.3 Fields:

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `id` | string | yes | Role id; MUST equal filename stem and match the kebab-case pattern (§3.2) |
| `title` | string | yes | Human label shown in prompts and progress |
| `instructions` | string | yes | The role's framing / system prompt body |
| `focus` | string[] | no | Focus questions appended to the prompt (review) or checklist-generation focus (audit) |
| `domains` | string[] | no (default: all) | Domains this role applies to; empty/absent = every domain |

3.4 The finding output schema is **not** part of a role file. tp injects the fixed output contract (Principle 2) into every emitted prompt. A role file MUST NOT contain any key outside §3.3; an unknown top-level key (including any attempt to declare output fields) is a validation error.

3.5 The field name is `focus`, not `lens` — "lens" is reserved for the retired frontmatter (§11) and the general "failure-lens" concept, so the schema avoids the collision.

3.6 `tp lint` validates the role corpus whenever a `.tp/reviewers/` or `.tp/auditors/` directory exists: every file parses, has the required fields, `id` equals the filename stem and matches the pattern, `domains` is a string array, and no unknown keys are present. A malformed or invalid role file aborts every role-reading command (`tp review`, `tp audit`, `tp lint`, `--eject-roles`) with exit 3 and a `repair or delete <path>` hint — consistent with malformed `.tp/config.json`.

## 4. Directory Layout & Discovery

4.1 Role files live at `.tp/reviewers/*.json` (spec review) and `.tp/auditors/*.json` (implementation audit), under the same repo-root `.tp/` directory as `config.json`.

4.2 Discovery reuses the v0.24.0 anchor: walk up from the current directory to the first `.git` boundary; the `.tp/` there supplies the corpus. A single deterministic anchor.

4.3 The corpus is **committed** to version control (like `config.json`): roles are shared team artifacts, and audit/review convergence enforcement must travel with the repo.

4.4 A phase directory is **empty** when it contains no `*.json` file (a non-JSON file is ignored). When a phase directory is absent or empty, tp uses its embedded default corpus for that phase (§5). A project therefore opts in per phase: custom reviewers with default auditors is valid.

## 5. Default Corpus & Eject

5.1 tp embeds a curated default corpus, selected by domain (§6):

| Phase | Domain `software` (default) | Domain `prose` |
|-------|-----------------------------|----------------|
| reviewers | implementer, tester, architect | coherence, soundness |
| auditors | spec-coverage, security, maintainability-conventions | spec-coverage, soundness |

5.2 A `regression` reviewer is a **built-in role**, not a corpus file: it is always appended to review emission, is never tapered (§8), and cannot be ejected or overridden. The id `regression` is reserved — a `.tp/reviewers/regression.json` file is a validation error (§3.6). Its prompt is tp's existing regression prompt.

5.3 `tp init --eject-roles [--domain <name>]` writes the selected default corpus into `.tp/reviewers/` and `.tp/auditors/` as editable files, so the previously hidden persona prompts become visible artifacts. `--domain` accepts only a domain tp ships a corpus for (§5.1); an unknown domain is a usage error (exit 2) listing the known domains. Eject refuses to overwrite an existing role file unless `--force`.

5.4 Ejecting is optional. It exists for inspection ("no black box") and customization; a project happy with defaults keeps zero role files and carries no maintenance burden.

## 6. Domain Selection

6.1 The spec-frontmatter `tp.domain` field (existing) selects which embedded default corpus applies when role files are absent, and which corpus `--eject-roles` scaffolds. An unknown domain falls back to the `software` corpus with a lint warning.

6.2 `domain` is **no longer a Go persona-swapper**. Its only effects are: (a) which embedded corpus is used when no role files exist, and (b) filtering the active corpus by each role's `domains` field (§3.3) — a role whose `domains` omits the spec's domain is not emitted. A role with no `domains` applies to every domain.

6.3 Prose defaults to a leaner panel (2 reviewers + regression) because prose flaws surface from many angles at once, so additional lenses mostly overlap. Software keeps 3 because its lenses genuinely diverge.

## 7. Prompt Emission

7.1 `tp review <spec>` emits one prompt per active reviewer role (corpus filtered by domain, minus roles tapered by §8), plus the built-in `regression` role.

7.2 `tp audit <spec>` emits one prompt per active auditor role. Audit is **not** tapered (§8 is review-only): its roles are fixed-purpose and diverge, so the full panel always runs.

7.3 Every emitted prompt instructs the sub-agent to stamp its `role` id on every finding it returns (this is what makes §9 attribution possible). Emission is otherwise unchanged: same output contract, same `--record` / `--status` / `--check` convergence machinery, same review/audit round separation.

## 8. Adaptive Review Scaling

8.1 Redundant reviewers cost tokens without finding new issues once a spec stabilizes. `tp review` may reduce the **diversity panel** in a round based on the previous round's measured inter-role overlap. Only `tp review` tapers; `tp audit` never does (§7.2).

8.2 **Yield** of a role in a recorded round = the number of merged findings (§9) whose `found_by_roles` set is exactly that one role — i.e., findings that would be lost if the role were dropped. A role's yield is its unique contribution, not its total findings.

8.3 **Taper rule.** At emission of round N, tp reads round N−1's stored per-role yields and overlap ratio (§9.5, §8.6). If the overlap ratio ≥ `OVERLAP_TAPER_THRESHOLD` (a specified constant, 0.5), tp drops the single lowest-yield reviewer (ties broken by lexicographic id) from round N. The active diversity panel never falls below `MIN_DIVERSITY_ROLES` (a specified constant, 1); the built-in `regression` role is always additional and never dropped.

8.4 **Convergence safeguard, enforced at emission.** tp owns the taper decision, so it guarantees the confirming rounds are full-panel by *not tapering them*: tp emits the full corpus for round N whenever `consecutive_clean ≥ 1`, or whenever a clean round N would reach `review_clean_rounds`. Thus every round that can advance `consecutive_clean` toward convergence ran the full diversity panel.

8.5 **Convergence counting.** tp records, per round, the emitted diversity role set and a `full_panel` boolean. A clean round advances `consecutive_clean` only if `full_panel` is true. A clean *tapered* round does not itself advance the count; §8.4 guarantees the next round is full-panel, so convergence is never declared on a reduced panel.

8.6 **Persisted taper state.** Each recorded review round in `state.json` gains: `emitted_roles` (string[]), `full_panel` (bool), `per_role_yield` (map id→int), and `overlap_ratio` (float). Round N reads round N−1's entry; the first round and any round with zero prior findings emit the full panel (no taper signal).

8.7 A single knob governs tapering: `review_adaptive` (bool, default `true`) in `.tp/config.json` workflow. `false` always emits the full panel. The two constants in §8.3 are specified values, not config surface, until a concrete need justifies exposing them (Principle 4).

## 9. Finding Dedup by Location + Class

9.1 `tp review --merge` currently dedups by literal text similarity, which misses paraphrases and cannot measure overlap. v0.25.0 clusters findings by **(normalized location, class)**.

9.2 **Location normalization.** A finding's `location` is normalized to its leading section anchor: the first `§<n>(.<n>)*` token, or — absent that — the nearest preceding spec heading id. Two findings cluster only if their normalized locations and their `class` values are both equal.

9.3 **Empty class is never clustered.** A finding with an empty or absent `class` is kept as its own cluster (never merged), so an empty key cannot collapse unrelated findings. Findings with different classes, or different normalized locations, never merge.

9.4 **Attribution.** Every input finding carries a `role` (stamped per §7.3). A merged cluster records `found_by_roles` (the sorted set of distinct contributing roles) and `found_by` (its cardinality). This is the sole source of §8.2 yield and §9.5 overlap.

9.5 The **overlap ratio** of a round = (merged clusters with `found_by ≥ 2`) / (total merged clusters), or `0.0` when there are zero clusters. It feeds §8.3.

9.6 A cluster with `found_by ≥ MECHANIZE_QUORUM` (a specified constant, 2) is a priority/mechanization candidate, extending the existing `mechanize_candidates` signal: a class independently surfaced by multiple roles is a strong mechanization target. It appears in `--merge` and `--report` output alongside the existing class-frequency candidates.

## 10. Role Staleness

10.1 tp derives a corpus hash per phase: **`review_roles_hash`** and **`audit_roles_hash`**, each the sha256 over that phase's user role files sorted by path, hashing each file's path and content. When a phase has no user files (embedded corpus), its hash is the fixed sentinel `"builtin"` — so upgrading tp does not falsely invalidate a project's convergence; a change to embedded prompts is a release-notes concern, not per-project staleness.

10.2 Both hashes are stored in `state.json`. Editing a review role file changes `review_roles_hash`, marking recorded **review** convergence stale; editing an auditor file changes `audit_roles_hash`, marking **audit** convergence stale. The two sequences are independent — editing a reviewer never staleness-invalidates audit convergence.

10.3 A spec-frontmatter role override (§11) lives in the spec, so it is already covered by the existing `spec_hash`; it is not counted in the role hashes (no double-count).

10.4 `tp review --status` reports `roles_stale` (from `review_roles_hash`); `tp audit --status` reports `roles_stale` (from `audit_roles_hash`), each alongside the existing spec `stale` flag.

## 11. `tp: lens` Retirement & Migration

11.1 The standalone `tp: lens` frontmatter concept is retired. Its three jobs move:

1. Domain persona-swap → absorbed by domain-selected default corpora (§6).
2. Project-wide concerns → role files (§4), shared once instead of copied into every spec's frontmatter.
3. Spec-local concerns → a spec-frontmatter **role override** (§11.2).

11.2 A spec may carry `tp.review_roles` and `tp.audit_roles` maps keyed by role id, each value `{ focus: [string...] }`. The listed `focus` questions are **appended** (additive) to that role's `focus` at emission. An override id that matches no active role is ignored with a lint warning (never an error). Defining a brand-new role in frontmatter is out of scope — new roles are files (§4); the override only extends existing roles.

11.3 Resolution reuses the v0.24.0 read-time layering: effective focus for a role = project-corpus `focus` ⊕ spec-override `focus` (concatenated, project first). No second resolution mechanism is introduced.

11.4 **Back-compat shim.** A legacy `tp: lens` block auto-translates with a deprecation warning: `lens.all` appends to every active review role's focus (and regression); `lens.<role-id>` appends to that role. The `tp:` frontmatter and its `domain` key are retained; only the standalone `lens` semantics are reframed as §11.2 overrides.

## 12. `.tp-active` Removal (breaking)

12.1 The `.tp-active` marker, deprecated in v0.24.0, is removed. Discovery no longer reads it; the fallback and its deprecation warning are deleted from the discovery chain.

12.2 The root `.gitignore` line for `.tp-active` is removed, and every doc reference to it (README, SKILL, REFERENCE, CLAUDE) is deleted or updated.

12.3 Discovery precedence becomes: `--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect.

## 13. Configuration Surface

13.1 `.tp/config.json` workflow gains exactly one field: `review_adaptive` (bool, default `true`). It is the first boolean workflow field; the resolver and `tp set --workflow` / `--project` gain boolean parsing (accepting `true` / `false`, rejecting other values with exit 1). It inherits the v0.24.0 layering and `tp config --resolved` provenance.

13.2 No other knob is added. Panel sizes derive from the corpus; the §8/§9 constants are specified values; staleness is automatic. Adding config beyond a demonstrated need is a maintenance cost we decline (Principle 4).

## 14. Non-Goals

1. tp is not an agent runtime — it emits prompts; it does not spawn or execute agents.
2. No user-defined finding schema or category taxonomy — the output contract stays tp's.
3. No audit-side adaptive taper — audit roles diverge and the full panel always runs.
4. No parameterized overlap policy — one boolean (`review_adaptive`); the §8/§9 constants stay fixed until earned.
5. No speculative role fields — §3.3 is the whole schema; new fields require a demonstrated need.
6. No frontmatter-defined new roles — spec overrides only extend existing roles (§11.2); new roles are files.
7. No cross-project or global role corpus — discovery stops at the `.git` boundary (§4.2), one project.

## 15. Migration

15.1 Existing projects keep working: no role files means the embedded default corpus, whose prompts match today's hardcoded personas. The one behavioral change is adaptive scaling, which defaults on; a project that wants the pre-v0.25.0 fixed panel sets `review_adaptive=false`. The §8.4 safeguard means convergence is still only ever declared on a full panel.

15.2 tp's own repo stays on the embedded default corpus — the three software reviewers and three auditors fit, and ejecting them would add role files with no benefit (Principle 4). v0.25.0's own review and audit loops dogfood the new emission / dedup / staleness code paths regardless, because those paths run over the embedded corpus too.

15.3 The `.tp-active` removal requires no user action: v0.24.0 already migrated active pointers to `.tp/local.json`.

## 16. Role Authoring Guidance

16.1 A project-authored role is only as good as its prompt. tp ships **authoring guidance** in the skill (`skills/tp/SKILL.md`) teaching an orchestrator agent how to design a reviewer or auditor role prompt that produces high-signal, low-overlap, contract-conformant findings. This guidance is the mitigation for the quality-dilution risk of opening role authoring (Principle 5).

16.2 The guidance covers, at minimum:

1. **Role = one distinct failure-lens** — a role must target a failure mode no other role covers; overlapping roles waste tokens and inflate the overlap ratio (§9).
2. **Adversarial framing** — "try to refute / find where this breaks" outperforms "check whether this is fine."
3. **Evidence demand** — every finding carries a location and a why, which is what makes dedup (§9) and audit PASS/FAIL meaningful.
4. **Scope boundaries** — state what the role does NOT cover, to keep lenses disjoint.
5. **Output-contract adherence** — the role customizes the focus, never the finding schema (§3.4).

16.3 The guidance includes worked example role sets for **code/software** (correctness, security, performance/contract lenses) and **prose** (narrative continuity vs expository derivability), plus a short catalog of other domains (legal/contract, product/PRD, data-schema, academic) with their characteristic diverging lenses.

16.4 The embedded default corpus (§5) is authored to exemplify this guidance, so an ejected default role is itself a worked example.

> Non-normative: the authoring guidance and the embedded default prompts are informed by a web-research pass on reviewer/auditor prompt design, performed during implementation before the corpus and §16 skill section are written. This is a process note, not a testable requirement.
