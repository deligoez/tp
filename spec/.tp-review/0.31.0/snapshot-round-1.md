# tp v0.31.0 — Honest convergence: severity-aware stopping, altitude findings, a recorded harness

## 1. Overview

A field report measured tp's own spec-review loop across four rounds of a real project (Laravel + Livewire, ~570-line spec, the embedded software corpus plus the auto-appended `regression` role). Most of what it found is praise the tool earned — the regression role, the overlap report, durable `.tp-review/` state. But three of its findings are not the reporter's project quirks: they are tp defects, confirmed against the code, and this release fixes them.

The through-line is **honesty of the review loop**: the moment tp decides a spec is done, the vocabulary a reviewer has to describe a defect, and the inputs to a recorded round that tp cannot currently see.

| Symptom the report measured | Root cause in tp | This release |
|---|---|---|
| Docs say "stop when no critical findings"; the tool grinds rounds until zero findings of any severity survive | The convergence predicate is severity-blind; the docs and even tp's own prompt strings disagree with it and with each other | §3–4: severity-aware convergence, one reconciled story |
| The spec kept accreting SQL and mechanism every round; nothing in the loop ever pushed detail back down to task acceptance | tp has no finding class for "this belongs in acceptance, not the spec" | §5: the `over-specification` finding class, reviewer-raised |
| The orchestrator's own prompt framing ("a clean round is legitimate") silently biased a recorded round; nothing recorded that it differed between rounds | The orchestrator wrapper is an unversioned, unrecorded input to a recorded process | §6: a recorded harness note, and where judgement-shaping text actually belongs |
| Regression findings silently dropped out of the per-role overlap report | One prompt builder omits the role-stamping contract | §7: every finding carries its role |
| `--resolve-all` and `--verify` existed but were read past as JSON fields | The loop surfaces capabilities as passive output, never as the next action | §8: `next_action` for the review and audit loops |

The scope is the spec-review phase and its shared machinery. **The audit phase is deliberately untouched** where it is already consistent (§4.4): audit convergence is status-based in both its code and its docs, and its role prompts already stamp `role`. The harness note (§6) and `next_action` (§8) extend to audit because their absence is symmetric; convergence and the finding-class vocabulary do not, because audit correctness is non-negotiable and stays at two clean rounds.

## 2. Design Principles

1. **The stop rule is one rule, stated once.** A convergence policy that lives one way in the code, another in the docs, and a third in the tool's own prompt strings is not a policy — it is three. tp enforces exactly what it documents, and documents exactly what it enforces.
2. **Severity is a decision input, not decoration.** tp already computes a severity breakdown and throws it away at the stop check. If severity is worth showing, it is worth deciding on.
3. **A spec review pushes toward decidable invariants, not implementation prose.** Detail whose correctness can only be established against code belongs in task acceptance, where the gate runs. The loop needs a way to say so — and it must be a reviewer saying it, never the orchestrator asserting it to make findings disappear (P4: *Agent Plans, Tool Executes* cuts both ways).
4. **An input to a recorded process is itself recorded.** tp already fingerprints the role corpus per round (`roles_hash`) and reports `roles_stale`. The orchestrator's prompt framing is no less load-bearing and no less deserving of a record.
5. **Every finding is attributable.** A finding with no role is invisible to the overlap report that justifies keeping the panel. Attribution is part of the output contract, for every perspective, with no exceptions.
6. **A capability the loop does not name is a capability that is not used.** `tp resume` names the next action per phase; the review and audit loops should too, rather than burying `--verify`, `--resolve-all`, and check registration in a large JSON response.

## 3. Severity-aware review convergence

3.1 **The defect.** `tp review <spec> --status --check` converges when a counted round is *clean*, and a clean round is one where zero findings survive verification — of any severity. Severity never enters the path. The documented policy (SKILL.md, CLAUDE.md) says the opposite: a review is converged when no **critical or blocking** finding remains, and lower-severity findings may be accepted with recorded justification. The enforced predicate and the written policy are different rules, and the enforced one creates the incentive the field report documented: grind rounds until reviewers happen to return nothing, which pressures the orchestrator into biasing the reviewers.

3.2 **The rule this release enforces.** A counted review round is *clean* when **no surviving finding is of a blocking severity** (§4.1). Findings below the blocking severities do not prevent a round from counting toward convergence; they are surfaced, not gated (§4.2). Convergence remains "`review_clean_rounds` consecutive clean rounds", unchanged — only the definition of *clean* becomes severity-aware.

3.3 **The setting.** A workflow field `review_converge_on` selects the policy:

| Value | A round is clean when… | Use |
|---|---|---|
| `blocking` (default) | no surviving finding is critical or high severity | the documented policy: stop when nothing blocking remains |
| `all` | no finding of any severity survives | the strict bright line: today's behavior, opt-in |

It resolves like every other workflow field (CLI > env > task override > project config > built-in default), carries a project-level default in `.tp/config.json`, and appears in `tp config --resolved`. Its built-in default is `blocking`. This is a behavior change from prior releases (which behaved as `all`), made deliberately: the tool now does what its own documentation always promised.

3.4 **Changing the setting re-evaluates history honestly.** A round's recorded state carries enough severity information that `--status` recomputes each round's cleanliness under the *current* `review_converge_on`, not the value in force when the round was recorded. Switching from `blocking` to `all` therefore un-cleans a round that had surviving medium findings, and `--status` reflects that immediately — a stored boolean that lied after a policy change would defeat the purpose. A round recorded before this release carries no severity breakdown; it is read without error and its stored clean flag is used as-is (the "empty means compatible" upgrade rule tp already uses for `roles_hash`).

3.5 **Resolution still exempts at any severity.** A finding resolved as `wontfix` (or `duplicate`) with evidence does not block a round under either setting, exactly as today — including a critical one, because an explicit, evidenced human acceptance is a stronger signal than a severity label. `review_converge_on` governs only whether an *unresolved* finding below the blocking severities blocks the round.

## 4. The blocking-severity set and recorded justification

4.1 **The blocking severities are critical and high.** Under `blocking`, these two severities prevent a clean round; medium and low do not. This is the union of what the two existing documented sources already imply — CLAUDE.md's "critical (blocking)" and the prompt string's "if any critical/high severity, revise spec" — reconciled into one set. The blocking set is fixed in this release, not itself a knob (§10.1).

4.2 **Non-blocking findings are surfaced, so accepting them is a visible decision, not a silent one.** When a round is clean only because its surviving findings are all below the blocking severities, `--status` and `--record` state this explicitly: they report the count of surviving non-blocking findings and that they were accepted open. "Accepted with recorded justification" is satisfied two ways, and the operator chooses: the findings remain recorded in `.tp-review/` and the surfaced count makes the acceptance auditable; or the operator resolves them as `wontfix` with per-finding evidence (§8.3) for an explicit justification. Neither is required for convergence under `blocking`; both leave a record.

4.3 **The severity breakdown already exists — this release consumes it.** tp computes a `by_severity` breakdown at merge time and has never read it at the stop check. The clean decision now reads surviving-finding severities. No new severity vocabulary is introduced: the review severities stay `critical | high | medium | low`.

4.4 **Audit convergence is unchanged.** An audit round is clean on a status count (rows whose `status` is not `PASS`), its docs describe exactly that, and its `error | warning | info` severity is a display annotation the stop check never reads — code and docs already agree. This release adds no severity-awareness to audit convergence and no `audit_converge_on` field. Audit runs to two consecutive clean rounds as before; implementation correctness is not negotiable against severity.

## 5. The `over-specification` finding class

5.1 **The failure mode.** Reviewers report under-specification; the orchestrator answers by adding precision; across rounds the spec accretes implementation mechanism (SQL clauses, transaction boundaries, column-by-column write tables). Prose is not type-checked, so every such addition is an unverified refactor of code written in a natural language, and the regression role dutifully finds the new inconsistencies with real call sites. The loop has no counter-pressure: nothing in it is ever licensed to say "this detail belongs in task acceptance, not the spec."

5.2 **The vocabulary.** tp documents a canonical finding `class`, `over-specification`, meaning: *a detail whose correctness can only be established against code, prescribed in the spec where it belongs in task acceptance instead.* The `class` field already exists and is free-form; this release gives it a documented, canonical value and describes it in the finding output contract and the role-authoring guidance, so a reviewer is licensed to raise it and a reader knows what it means. Its typical severity is low or medium — it is an altitude smell, not a blocking defect — which is why §3–4 matter for it: an over-specification finding does not, by itself, block convergence.

5.3 **The altitude principle is stated in the role-authoring guidance.** The guidance for review roles (SKILL.md and REFERENCE.md, and the `--eject-roles` default role docs) states that a spec review should push toward decidable invariants — behavior a task's acceptance can verify against the gate — and away from implementation prose. This is documentation and guidance, not a new default reviewer role (§10.2): the class is available to every existing reviewer.

## 6. The recorded harness note

6.1 **The invisible input.** tp emits a role prompt; the orchestrator wraps it before it reaches the sub-agent, and tp cannot see that wrapper. The wrapper legitimately carries runtime-specific setup (e.g. "native file tools are hook-blocked, use the MCP toolset"). But it is also where an orchestrator is tempted to put standing judgement-shaping instructions — and when it does, two rounds are recorded as comparable while their actual instructions differed materially, with nothing in `.tp-review/` showing it. tp already solved this shape for the role corpus; the wrapper gets none of it.

6.2 **The note.** `tp review <spec> --record <file> --harness-note "<text>"` stores the note verbatim on the recorded round; `tp audit <spec> --record <file> --harness-note "<text>"` does the same for an audit round. The flag is optional: omitted, nothing is recorded and behavior is exactly as today. It is free text, not a controlled value — its job is to state, in the orchestrator's own words, what standing framing the wrapper carried this round.

6.3 **Staleness is surfaced like `roles_stale`.** `--status` reports the latest round's harness note and a `harness_stale` flag: true when the two most recently recorded rounds carry different notes (a wrapper that changed mid-loop), false when they match or when a round predates the field (the same "empty means compatible" rule as `roles_hash`). This turns an invisible variable into a recorded, comparable one without tp needing to see or control the wrapper.

6.4 **Judgement-shaping text belongs in roles, not the wrapper — stated plainly.** SKILL.md states the rule the report found missing: standing instructions that shape a reviewer's judgement (what counts as a real defect, what to treat as intentional) do not belong in the orchestrator wrapper; their sanctioned homes are `tp.review_roles` frontmatter for per-spec focus and `.tp/reviewers/*.json` for project-owned roles, both of which tp records via `roles_hash`. The wrapper is for what tp cannot know — runtime setup — and when it must carry standing framing anyway, that framing is declared with `--harness-note` so it is recorded rather than invisible.

## 7. Every finding carries its role

7.1 **The defect.** Corpus reviewer prompts append the canonical output contract, which instructs stamping `role` on every finding; the `regression` prompt builder does not — it appends only the bare finding format. So regression findings carry no `role` and silently drop out of the per-role overlap report, in both the auto-appended regression pass of a counted round and the standalone `--perspective regression` path (they share the one defective builder). The `--record` path warns on a role-less row, but the round it was meant to attribute is already under-counted.

7.2 **The rule.** Every emitted finding — from any review perspective, including `regression`, `--verify`, and `code-audit` — carries a `role` field identifying the emitting perspective. Fixing the single shared builder fixes both regression call sites; the verify and code-audit prompts, which emit findings under their own informal schemas, also stamp `role`, so "every finding is attributable" holds with no per-perspective exception. Plan-emitting prompts (doc and test plans) are unaffected — they emit plan items, not findings.

7.3 **The audit side is already correct and stays untouched.** Every audit role prompt appends both the audit schema and the output contract, so audit findings already carry `role` and `item_id`. This release changes no audit prompt builder.

## 8. `next_action` for the review and audit loops

8.1 **The principle.** `tp resume` names the exact next command per phase; the review and audit loops surface their capabilities only as fields in a large response, so `--verify`, `--resolve-all`, and check registration get read past. `tp review --status`, `tp review --record`, and the audit equivalents gain a `next_action` naming the single next command the current state calls for.

8.2 **What it names.** Given the recorded state, `next_action` points to one command: unresolved findings present → resolve them (`tp review <file> --resolve-all <status> "<evidence>"` for the batch case, or `--resolve <idx>` for one) or verify disputed ones (`tp review --verify <spec> --findings <file>`); a `mechanize_candidates` class present → register a check (`tp set --workflow checks='[…]'`); converged → the phase's forward command (decompose / import, or in audit, proceed). It is advisory and read-only: it changes nothing and never gates an exit code.

8.3 **`--resolve-all` is documented where `--resolve` is.** The batch resolver (`tp review <file> --resolve-all <fixed|wontfix|duplicate> "<evidence>"`, `--force` to re-resolve) already exists but was undiscoverable next to the single-index `--resolve`. SKILL.md and REFERENCE.md document it adjacent to `--resolve`, named as the way to accept or dispose of many findings in one call — including the "accept all surviving non-blocking findings as `wontfix` with a shared justification" case that §4.2 relies on.

## 9. Documentation

9.1 `skills/tp/SKILL.md`: correct the convergence description to the enforced severity-aware rule and document `review_converge_on` and its default; document the `over-specification` class and the altitude principle in the role-authoring guidance; state the "judgement-shaping text belongs in roles, not the wrapper" rule and document `--harness-note` / `harness_stale`; document `--resolve-all` adjacent to `--resolve`; document `next_action` for the review and audit loops.

9.2 `CLAUDE.md`: the self-development convergence rule states the blocking-severity set explicitly (critical or high block; medium and low may be accepted with recorded justification once none blocking remain) and names `review_converge_on`, so the project's own rules match the enforced predicate rather than a policy the code never implemented.

9.3 `README.md` and `skills/tp/REFERENCE.md`: document `review_converge_on`, the `over-specification` class, `--harness-note` / `harness_stale`, `next_action`, and `--resolve-all`; correct any convergence wording that claims a severity-blind or a critical-only stop rule.

9.4 The tool's own prompt strings are corrected to state the enforced rule: the review-loop convergence and instruction strings describe "no surviving critical or high severity finding" (matching `review_converge_on: blocking`), and no string claims a stop rule the code does not enforce. The self-contradiction between the two existing convergence strings is removed.

## 10. Non-Goals

1. **The blocking-severity set is not itself configurable.** `review_converge_on` chooses between the documented policy (`blocking` = critical + high) and the strict bright line (`all`); a per-severity floor (e.g. critical-only, or high-and-above-including-medium) is deferred until a project shows it needs one. The two values cover the reconciled documented intent.
2. **No dedicated altitude reviewer role.** The `over-specification` class is available to every existing reviewer; a fifth default role whose sole lens is altitude is deferred. The overlap report can justify one later if the class proves to miss without it.
3. **tp does not detect over-specification, and the orchestrator does not assert it.** The class is a vocabulary a *reviewer* raises against the spec; tp neither flags it heuristically nor licenses the orchestrator to delete spec content on its own claim of over-specification. Altitude is a reviewer's finding, subject to the same P4 boundary as every other finding.
4. **No audit convergence change.** No `audit_converge_on`, no severity-aware audit stop rule; audit stays at two clean status-based rounds.
5. **The harness note is opt-in and never gates.** tp does not require it, cannot see the wrapper it describes, and never fails or blocks a round for its absence or its contents; `harness_stale` is reported, never enforced.
6. **No new severity vocabulary and no rewrite of recorded rounds.** Review severities stay `critical | high | medium | low`; rounds recorded before this release are read as-is under the upgrade rule, never rewritten.

## 11. Tests / Acceptance

1. With `review_converge_on: blocking` (the default), a counted round whose only surviving findings are medium and low is clean and counts toward convergence; a round with a surviving high (or critical) finding is not clean (§3.2, §4.1).
2. With `review_converge_on: all`, the same round with surviving medium findings is not clean — reproducing the prior release's behavior (§3.3).
3. `review_converge_on` resolves through the layer order and appears in `tp config --resolved`; its built-in default is `blocking` (§3.3).
4. Switching `review_converge_on` from `blocking` to `all` re-evaluates recorded rounds: a round previously clean with surviving medium findings becomes not clean in `--status` without re-recording (§3.4).
5. A round recorded before this release (no severity breakdown) is read without error and its stored clean flag is honored (§3.4).
6. A finding resolved `wontfix` with evidence does not block a round under either setting, including a critical one (§3.5).
7. When a round is clean only because all surviving findings are below the blocking severities, `--status` and `--record` report the count of surviving non-blocking findings and that they were accepted open (§4.2).
8. Audit convergence is unchanged: an audit round's cleanliness is a status count, unaffected by `review_converge_on` and by any `error|warning|info` severity; there is no `audit_converge_on` (§4.4).
9. A reviewer finding with `class: over-specification` is accepted, carried through merge, and documented in the finding contract and role-authoring guidance; its presence does not by itself block convergence under `blocking` (§5.2, §5.3).
10. `tp review <spec> --record <file> --harness-note "<text>"` stores the note on the round; `tp audit … --harness-note` does the same for an audit round; omitting the flag records nothing and changes no behavior (§6.2).
11. `--status` reports the latest harness note and `harness_stale`: true when the two most recent rounds' notes differ, false when they match or a round predates the field (§6.3).
12. A regression finding — from both the auto-appended pass and `--perspective regression` — carries `role: "regression"` and appears in the per-role overlap report; `--verify` and `code-audit` findings also carry `role` (§7.1, §7.2).
13. No audit prompt builder is changed; audit findings still carry `role` and `item_id` (§7.3).
14. `tp review --status`, `tp review --record`, and the audit equivalents carry a `next_action` naming one command matched to state (resolve/verify when unresolved findings exist; register a check when a mechanize candidate exists; the forward command when converged); it gates no exit code (§8.1, §8.2).
15. `--resolve-all` is documented adjacent to `--resolve` in SKILL.md and REFERENCE.md, including the accept-all-non-blocking-as-`wontfix` case (§8.3).
16. The tool's review-loop convergence and instruction strings state the enforced severity-aware rule with no internal contradiction; no prompt string claims an unenforced stop rule (§9.4).
17. `go test ./...` and `golangci-lint run` are clean, and SKILL.md, CLAUDE.md, README.md, and REFERENCE.md carry the §9 updates.
