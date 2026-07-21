# tp v0.25.0 — User-defined reviewer & auditor roles

## 1. Overview

Review and audit roles are hardcoded in Go: `tp review` always emits the implementer / tester / architect trio (persona-swapped by domain), and `tp audit` always emits spec-coverage / security / maintainability-conventions. A project cannot add a role it needs (transaction-integrity, idempotency, pedagogy) or drop one that only produces overlap.

v0.25.0 makes the role corpus **project-owned data**. Each role is a file under `.tp/`; tp ships curated defaults and can eject them for editing. It also improves finding dedup so that inter-role overlap is measurable and **reported** — letting the orchestrator trim redundant roles or mechanize a repeated class — retires the now-redundant `tp: lens` frontmatter, and removes the `.tp-active` marker deprecated in v0.24.0.

tp does **not** auto-reduce the panel per round. It surfaces the overlap signal; the human or orchestrator decides whether to trim the corpus. This is tp's P4 (agent plans, tool executes) applied to review scaling, and it keeps convergence a simple, unchanged contract.

This spec extends the v0.24.0 `.tp/` config mechanism; it does not replace it. `.tp/config.json` (workflow policy) and the role corpus live side by side.

## 2. Design Principles

1. **Separate data, share machinery.** Reviewers and auditors are distinct corpora (different input, question, lifecycle) but share one file schema, one parser, and one validator. The finding pipeline (dedup, staleness) is phase-parameterized, not duplicated. Duplication is the cost we refuse.
2. **tp owns the output contract; the user owns the prompt.** A role file customizes the prompt (persona, instructions, focus questions); it can never redefine the finding schema. Every finding carries `role`, `location`, `class`, `severity`, and — for audit — `status` ∈ PASS/PARTIAL/FAIL. Dedup and mechanization depend on that contract being tp's.
3. **tp emits prompts; it never executes agents.** Role files are prompt specifications, not a runtime. The orchestrating agent spawns sub-agents exactly as today.
4. **Minimalism — only earned machinery.** No speculative config knobs, no fields "just in case," no automation that a report plus a human decision covers. Every kept surface is a maintenance cost.
5. **Excellent defaults, opt-in authoring.** A project with no role files runs the curated embedded corpus. Authoring roles is a power feature, not the default path, so aggregate review quality does not depend on every user writing good prompts.

## 3. Role Definition Schema

3.1 A role is a single JSON file. Reviewers and auditors share one schema; the **phase is inferred from the directory** (`.tp/reviewers/` vs `.tp/auditors/`) — no `phase` field is stored.

3.2 A role file's **`id` MUST equal its filename stem** (`security.json` → id `security`). This makes ids unique within a directory by construction and removes any id/filename ambiguity. `id` MUST match `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase kebab-case). The id `regression` is **reserved** (§5.2): a role file named `regression.json` is a validation error.

3.3 Fields:

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `id` | string | yes | Role id; MUST equal filename stem, match the kebab-case pattern, and not be the reserved `regression` (§3.2) |
| `title` | string | yes | Human label shown in prompts and progress |
| `instructions` | string | yes | The role's framing / system prompt body |
| `focus` | string[] | no | Focus questions appended to the prompt (review) or checklist-generation focus (audit) |
| `domains` | string[] | no (default: all) | Domains this role applies to; empty/absent = every domain |

3.4 The finding output schema is **not** part of a role file. tp injects the fixed output contract (Principle 2) into every emitted prompt. A role file MUST NOT contain any key outside §3.3; an unknown top-level key (including any attempt to declare output fields) is a validation error.

3.5 The schema field is named `focus`, not `lens`, to avoid colliding with the retired `tp: lens` frontmatter (§10). ("Lens" still appears in prose as the informal term for a role's failure-lens; only the JSON key is renamed.)

3.6 `tp lint` validates the role corpus whenever a `.tp/reviewers/` or `.tp/auditors/` directory exists. Each file must: parse as JSON; contain all required fields (§3.3); have `id` equal to the filename stem, matching the kebab-case pattern, and not equal to the reserved `regression`; carry `domains` as a string array if present; and contain no unknown top-level key. A malformed or invalid role file aborts every role-reading command (`tp review`, `tp audit`, `tp lint`, `tp init --eject-roles`) with exit 3 and a `repair or delete <path>` hint — consistent with malformed `.tp/config.json`.

## 4. Directory Layout & Discovery

4.1 Role files live at `.tp/reviewers/*.json` (spec review) and `.tp/auditors/*.json` (implementation audit), under the same repo-root `.tp/` directory as `config.json`.

4.2 Discovery reuses the v0.24.0 anchor: walk up from the current directory to the first `.git` boundary; the `.tp/` there supplies the corpus. A single deterministic anchor.

4.3 The corpus is **committed** to version control (like `config.json`): roles are shared team artifacts, and review/audit convergence must travel with the repo.

4.4 A phase directory is **populated** when it contains at least one `*.json` file (non-JSON files are ignored). A populated phase directory **replaces** — does not merge with — the embedded default corpus for that phase: the active corpus for that phase is exactly its files. When a phase directory is absent or empty, tp uses its embedded default corpus for that phase (§5). A project therefore opts in per phase: custom reviewers with default auditors is valid.

## 5. Default Corpus & Eject

5.1 tp embeds a curated default corpus, selected by domain (§6):

| Phase | Domain `software` (default) | Domain `prose` |
|-------|-----------------------------|----------------|
| reviewers | implementer, tester, architect | coherence, soundness |
| auditors | spec-coverage, security, maintainability-conventions | spec-coverage, soundness |

5.2 A `regression` reviewer is a **built-in role**, not a corpus file: it is always appended to review emission and cannot be ejected. The reserved id `regression` (§3.2) keeps a user file from shadowing it. Its prompt is tp's existing regression prompt. A spec-frontmatter override (§10) does **not** apply to `regression` — its lens is fixed.

5.3 `tp init --eject-roles [--domain <name>]` writes the selected default corpus into `.tp/reviewers/` and `.tp/auditors/` as editable files, so the previously hidden persona prompts become visible artifacts. `--domain` accepts only a domain tp ships a corpus for (§5.1); because the value is typed by the user, an unknown `--domain` is a usage error (exit 2) listing the known domains. Eject refuses to overwrite an existing role file unless `--force`; `--force` overwrites regardless of the existing file's validity.

5.4 Ejecting is optional and behavior-neutral: an ejected default is byte-equivalent in effect to the embedded one, so it does not change emission or staleness (§9.1). It exists for inspection ("no black box") and as an editing baseline. A project happy with defaults keeps zero role files and carries no maintenance burden.

## 6. Domain Selection

6.1 The spec-frontmatter `tp.domain` field (existing) selects which embedded default corpus applies when role files are absent, and which corpus `--eject-roles` scaffolds. An unknown domain falls back to the `software` corpus with a lint warning (frontmatter is data, so it degrades softly rather than erroring).

6.2 `domain` is **no longer a Go persona-swapper**. Its only effects are: (a) which embedded corpus is used when no role files exist, and (b) filtering the active corpus by each role's `domains` field (§3.3) — a role whose `domains` omits the spec's domain is not emitted. A role with no `domains` applies to every domain. If domain filtering leaves a phase's active panel empty, tp falls back to that phase's embedded default corpus (a project never reviews with zero roles).

6.3 Prose defaults to a leaner panel (2 reviewers + regression) because prose flaws surface from many angles at once, so extra reviewers mostly overlap. Software keeps 3 because its concerns genuinely diverge.

## 7. Prompt Emission

7.1 `tp review <spec>` emits one prompt per active reviewer role (the corpus filtered by domain per §6.2), plus the built-in `regression` role. The full active panel runs every round — tp does not drop roles automatically (§8.5).

7.2 `tp audit <spec>` emits one prompt per active auditor role. Audit and review are symmetric here: the full active panel runs each round.

7.3 Every emitted prompt instructs the sub-agent to stamp, on every finding it returns, its `role` id and a `location` normalized to a section anchor (§8.2). These two fields are part of the output contract (Principle 2) and are what make dedup and overlap reporting possible. Emission is otherwise unchanged: same `--record` / `--status` / `--check` convergence machinery, same review/audit round separation.

## 8. Finding Dedup & Overlap Reporting

8.1 `tp review --merge` currently dedups by literal text similarity, which misses paraphrases and cannot measure overlap. v0.25.0 clusters findings by **(location, class)**.

8.2 **Location.** Reviewers stamp `location` as a section anchor — a `§<n>(.<n>)*` token (e.g. `§8.2`). tp's clustering key is the exact stamped anchor; there is no fuzzy heading lookup. A finding whose `location` contains no `§` token clusters only with findings carrying the byte-identical `location` string.

8.3 **Class.** Two findings cluster only if both their `location` and their `class` are equal. A finding with an empty or absent `class` is kept as its own cluster (never merged), so an empty key cannot collapse unrelated findings.

8.4 **Attribution.** Every input finding carries a `role` (§7.3). A merged cluster records `found_by_roles` (the sorted set of distinct non-empty contributing roles) and `found_by` (its cardinality). A finding with a missing or blank `role` still clusters by (location, class) but contributes no role to `found_by_roles`.

8.5 **Overlap reporting (no auto-taper).** `--merge`, `--report`, and `--status` report: the round's **overlap ratio** = (clusters with `found_by ≥ 2`) / (total clusters), or `0.0` when there are zero clusters; and **mechanize candidates** — clusters with `found_by ≥ MECHANIZE_QUORUM` (a specified constant, 2), merged with the existing class-frequency candidates by taking the union keyed on `class` (a class flagged by either signal appears once, annotated with which signals fired). tp **does not** change the emitted panel based on this signal; a high overlap ratio is a recommendation to the orchestrator to trim a redundant role file (§4) or register a mechanical check, not an action tp takes.

8.6 Findings with different classes, or different locations, never merge. Clustering only collapses genuine duplicates.

## 9. Role Staleness

9.1 tp derives a corpus hash per phase from the **effective role set's content**, not file paths: **`review_roles_hash`** and **`audit_roles_hash`**, each the sha256 over that phase's active roles sorted by id, hashing each role's canonical `(id, title, instructions, focus, domains)`. Because the hash is over content, an ejected default corpus (byte-equivalent to the embedded one) produces the same hash as the embedded corpus — ejecting is staleness-neutral (§5.4) — and the hash is identical across clones regardless of checkout path.

9.2 Both hashes are stored in `state.json`. Editing a review role's effective content changes `review_roles_hash`, marking recorded **review** convergence stale; editing an auditor changes `audit_roles_hash`, marking **audit** convergence stale. The two sequences are independent — editing a reviewer never staleness-invalidates audit convergence.

9.3 A spec-frontmatter role override (§10) lives in the spec, so it is already covered by the existing `spec_hash`; it is not counted in the role hashes (no double-count).

9.4 `tp review --status` reports `roles_stale` (from `review_roles_hash`); `tp audit --status` reports `roles_stale` (from `audit_roles_hash`), each alongside the existing spec `stale` flag. A pre-v0.25.0 `state.json` with no stored role hash treats the current hash as the baseline (no retroactive staleness) on first read.

## 10. `tp: lens` Retirement & Migration

10.1 The standalone `tp: lens` frontmatter concept is retired. Its three jobs move:

1. Domain persona-swap → absorbed by domain-selected default corpora (§6).
2. Project-wide concerns → role files (§4), shared once instead of copied into every spec's frontmatter.
3. Spec-local concerns → a spec-frontmatter **role override** (§10.2).

10.2 A spec may carry `tp.review_roles` and `tp.audit_roles` maps keyed by role id, each value an object whose only permitted key is `focus` (a string array); any other key is a lint warning and is ignored. The listed `focus` questions are **appended** (additive) to that role's `focus` at emission. An override whose id matches no active role in that phase is ignored with a lint warning (never an error). The built-in `regression` role does not accept overrides (§5.2). New roles are files (§4); frontmatter only extends existing roles.

10.3 Resolution reuses the v0.24.0 read-time layering: effective focus for a role = project-corpus `focus` ⊕ spec-override `focus` (concatenated, project first). No second resolution mechanism is introduced.

10.4 **Back-compat shim.** A legacy `tp: lens` block auto-translates with a deprecation warning: `lens.all` appends to every active **review** role's focus (not `regression`, not auditors); `lens.<role-id>` appends to that review role if it is active (else the warning notes the unknown id). The shim covers review only, matching legacy `lens` semantics. The `tp:` frontmatter and its `domain` key are retained; only the standalone `lens` key is reframed as §10.2 overrides.

## 11. `.tp-active` Removal (breaking)

11.1 The `.tp-active` marker, deprecated in v0.24.0, is removed. Discovery no longer reads it; the fallback and its deprecation warning are deleted from the discovery chain.

11.2 The root `.gitignore` line for `.tp-active` is removed, and every doc reference to it (README, SKILL, REFERENCE, CLAUDE) is deleted or updated.

11.3 Discovery precedence becomes: `--file` > `TP_FILE` > `.tp/local.json` active pointer > auto-detect.

## 12. Non-Goals

1. tp is not an agent runtime — it emits prompts; it does not spawn or execute agents.
2. No user-defined finding schema or category taxonomy — the output contract stays tp's.
3. **No auto-taper.** tp reports overlap (§8.5); it never changes the emitted panel by itself. Panel size is a durable corpus decision (edit role files), not a per-round guess.
4. No new workflow config field — v0.25.0 adds no `.tp/config.json` knob (removing the need for a boolean workflow field entirely).
5. No speculative role fields — §3.3 is the whole schema; new fields require a demonstrated need.
6. No frontmatter-defined new roles — spec overrides only extend existing roles (§10.2); new roles are files.
7. No cross-project or global role corpus — discovery stops at the `.git` boundary (§4.2), one project.

## 13. Migration

13.1 Existing projects keep working unchanged. No role files means the embedded default corpus, whose prompts match today's hardcoded personas, and the emitted panel is identical to today's. The only visible change is for `domain: prose` specs, whose default reviewer panel becomes the two prose lenses (§6.3) instead of three swapped personas — a project can eject and add a third role if it wants the old count.

13.2 tp's own repo stays on the embedded default corpus — the three software reviewers and three auditors fit, and ejecting them would add role files with no benefit (Principle 4). v0.25.0's own review and audit loops dogfood the new emission / dedup / staleness code paths regardless, because those paths run over the embedded corpus too.

13.3 The `.tp-active` removal requires no user action: v0.24.0 already migrated active pointers to `.tp/local.json`.

## 14. Role Authoring Guidance

14.1 A project-authored role is only as good as its prompt. tp ships **authoring guidance** in the skill (`skills/tp/SKILL.md`) teaching an orchestrator agent how to design a reviewer or auditor role prompt that produces high-signal, low-overlap, contract-conformant findings. This guidance is the mitigation for the quality-dilution risk of opening role authoring (Principle 5).

14.2 The guidance covers, at minimum:

1. **Role = one distinct failure-lens** — a role must target a failure mode no other role covers; overlapping roles waste tokens and raise the overlap ratio (§8.5).
2. **Adversarial framing** — "try to refute / find where this breaks" outperforms "check whether this is fine."
3. **Evidence demand** — every finding carries a `location` and a why, which is what makes dedup (§8) and audit PASS/FAIL meaningful.
4. **Scope boundaries** — state what the role does NOT cover, to keep lenses disjoint.
5. **Output-contract adherence** — the role customizes the focus, never the finding schema (§3.4).

14.3 The guidance includes worked example role sets for **code/software** (correctness, security, performance/contract lenses) and **prose** (narrative continuity vs expository derivability), plus a short catalog of other domains (legal/contract, product/PRD, data-schema, academic) with their characteristic diverging lenses.

14.4 The embedded default corpus (§5) is authored to exemplify this guidance, so an ejected default role is itself a worked example.

> Non-normative: the authoring guidance and the embedded default prompts are informed by a web-research pass on reviewer/auditor prompt design, performed during implementation before the corpus and §14 skill section are written. This is a process note, not a testable requirement.
