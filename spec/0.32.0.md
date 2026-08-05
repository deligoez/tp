# tp v0.32.0 — Per-spec role activation

## 1. Overview

The role corpus is repository-wide. A spec's frontmatter can append focus questions to a role, but it cannot switch a role off. Field feedback from a repository running six reviewers and four auditors reported the consequence: for a spec about multi-tenant authorization two lenses are the highest-value ones and the rest are noise, and each noise role still costs a full sub-agent round.

A second report from the same repository settles which problem this is. Its round-1 overlap report over 92 merged findings credited the three project-authored roles with 38 findings between them and exactly one cluster shared with the built-ins, and two of the round's most expensive defects came from roles the default corpus does not contain — an `auth-context` role that found a middleware registration list complete for route groups but missing `/broadcasting/auth`, and a `data-migration` role that found a column the spec proposed to drop carrying 146k historical rows plus a foreign key, making the spec's `ALGORITHM=INSTANT` claim false. So a large corpus is not a value problem: the roles earn their tokens on a spec in their domain. It is a **noise** problem — those same roles earn nothing on a spec outside their domain, and today there is no way to say so per spec. That is what this release adds, and it is also why the mechanism is deny-only: an allowlist would silently drop a role someone forgot to list, and this data says a dropped role can cost a critical finding.

A partial mechanism already exists and the report understates it: `domains` on a role plus `domain:` in the spec frontmatter does deactivate a role per spec, and the dropped role is named in `skipped_roles` with reason `domain-mismatch`. But `domains` is a taxonomy axis — it answers "what kind of work is this role for", not "this one refactor spec does not need it" — and a spec carries exactly one domain, so expressing an arbitrary per-spec subset through domains means inventing domains that describe nothing.

### 1.1 What a declined role costs

The saving is one sub-agent round per declined role per phase round, and v0.31.2 made that round more expensive: every shared-arm audit prompt now carries the `## Project Context` block (up to `claudeMDExcerptLineCap`, 50 lines), the `## Disposition` block, and up to `CodeFileCap` affected-file lines, on top of the role's own rules. Declining one auditor on a spec outside its domain therefore saves, per audit round: one sub-agent invocation, that prompt, the files the role reads itself, and one result row per file that the driver must merge and read. Multiplied by the required clean rounds, and again by each round the loop does not converge on.

The cost side is deliberately small: one optional frontmatter key, one new `skipped_roles` reason, and two refusals that only fire on a mistake. This release adds no flag, no command, and no field to any JSON payload.

This spec assumes the v0.31.2 routing, in which `spec-coverage` is the only auditor id that changes routing and every other role receives one item per affected code file.

## 2. `enabled` in the spec-frontmatter role overrides

### 2.1 The override value gains a second permitted key

`tp.review_roles` and `tp.audit_roles` keep their current shape — a mapping from role id to an override object. The override object accepts a second key, `enabled`, whose value is a boolean:

```yaml
tp:
  audit_roles:
    data-migration: { enabled: false }
    multi-tenant-isolation:
      focus:
        - "Does every query filter by the active tenant?"
```

`enabled` and `focus` may appear together on the same role. `enabled: false` with a `focus` list is not an error: the role is deactivated and the focus is never applied, because a deactivated role emits nothing. `enabled: true` is accepted and is a no-op, since a role reaching override resolution is already active; it exists so a reader can state intent without memorizing which polarity is meaningful. Any key other than `focus` and `enabled` remains a lint warning and is ignored, as today.

### 2.2 The parsed override becomes a struct

`Frontmatter.ReviewRoles` and `Frontmatter.AuditRoles` change from `map[string][]string` to `map[string]RoleOverride`, where `RoleOverride` carries `Focus []string` and `Enabled *bool` (nil meaning unset, which is today's behavior). Every reader of those two fields must move with them; the readers are `ResolveOverrideFocus` (`internal/engine/overrides.go`), the lint pass that reports unknown override ids, and `TranslateLegacyLens`'s caller, which compares against the new map only for emptiness. The parse warnings for a non-mapping value, a non-list `focus`, and a non-string focus element are unchanged; a non-boolean `enabled` is a new lint warning and is ignored, leaving the role active.

Because `enabled: false` is currently rejected with the "not a permitted override key" warning and ignored, a spec written for this release degrades safely on an older tp: the role stays active and the user is told why.

### 2.3 Resolution drops the role and reports what it dropped

`ResolveOverrideFocus` returns a third value: the ids it dropped for `enabled: false`. Its two callers (`internal/cli/review.go` and `internal/cli/audit.go`) pass that set to the `skipped_roles` assembly of §2.4 and to the refusals of §2.5 and §2.6; without it the drop would be invisible to the code that must report and guard it.

The drop happens after domain filtering and after the active-role check, so a role already removed by `domains` is reported once with `domain-mismatch` and not again, and `enabled: false` on an id matching no active role produces the existing "matches no active role" warning rather than a silent success. When both could apply to one id, `domain-mismatch` wins: it removed the role first, and the spec's `enabled: false` for it is then an override of a role that is not active, which is the warn-and-ignore case.

The drop is applied **outside** `ResolveActiveCorpus`, which is what keeps it from colliding with that function's documented empty-panel fallback: when domain filtering empties the user panel, `ResolveActiveCorpus` falls back to the embedded corpus so a project never reviews with zero roles. This release does not change that. The `enabled: false` drop is applied to whatever set `ResolveActiveCorpus` returned, and an empty result is refused by §2.5 rather than back-filled — the two rules describe different situations, one where the project's taxonomy accidentally matched nothing and one where the spec explicitly named every role.

The legacy `tp: lens` shim applies only when `tp.review_roles` and `tp.audit_roles` are both absent. A spec that uses `review_roles` solely to deactivate a role therefore suppresses the shim, exactly as one that uses it to add focus does; the spec has opted into the current mechanism.

### 2.4 A deactivated role stays visible

Every role deactivated by `enabled: false` is named in the phase's `skipped_roles` with the new reason `disabled-by-spec`, alongside the existing `no-checklist-items`, `no-spec-change`, `domain-mismatch`, and `no-baseline`. The reason is documented with the others.

`skipped_roles` remains omitted under `--compact`, as today. That is a real gap in the transparency claim and it is accepted rather than papered over: under `--compact` a driver cannot tell a deactivated role from any other absence. The mitigation is that deactivation is declared in the spec the driver already has, unlike the silent skips `skipped_roles` exists to surface, and `--compact` is opt-in. A driver that wants the distinction drops `--compact` for the emission call.

### 2.5 A phase cannot be emptied

Deactivating every role of a phase would produce a review or audit round that emits nothing and records nothing while still counting as a round. **On prompt emission**, when the `enabled: false` set would leave a phase with no active role, the command exits 2 with an error naming the phase and the deactivated ids, and a hint to re-enable at least one role. The built-in `regression` role does not count as an active reviewer for this check, because it is appended to emission separately and cannot carry the phase alone.

Two scoping rules make this buildable and non-deadlocking, and both are load-bearing:

1. **The refusal fires only in the prompt-emission mode of each command.** `tp review` and `tp audit` are single cobra commands with many modes, and every mode takes the same spec argument and parses the same frontmatter. `--record`, `--status` (with or without `--check`), `--merge`, `--resolve`, `--resolve-all`, `--verify`, and `--report` never resolve the role corpus and must be unaffected. A refusal that fired on `--status --check` would break the convergence gate the whole loop runs on, and one that fired on `--resolve` would break §2.6's own escape hatch.
2. **Role resolution moves ahead of the round-snapshot write.** Today both commands write the round-N spec snapshot before resolving roles — `tp review` at `loadReviewRoundState` and `tp audit` at `loadAuditSpec`, both via `WriteSnapshotAtomic`. Since the deactivation set is only known after resolution, a refusal at the current ordering would exit 2 with a snapshot already on disk, contradicting "no state is written". Corpus resolution and override resolution move before the snapshot write in both commands, so both refusals of §2.5 and §2.6 are decidable while nothing has been written.

### 2.6 A role with unresolved findings cannot be deactivated

The prior-round carry mechanism re-presents a role's unresolved rows in its next-round prompt. Deactivating a role would therefore drop its open findings out of the loop while the recorded state still holds them — a spec could reach a clean round by removing the lens that was failing. tp refuses this, on prompt emission, with the same scoping as §2.5.

The predicate is not invented here. It is the phase's own convergence predicate, so the guard and the definition of a clean round agree by construction:

- **Review**: a recorded finding attributed to that role which `engine.reviewFindingResolvedAway` does not treat as resolved away — that is, one whose `resolved.status` is not `wontfix` or `duplicate`, or whose `resolved.evidence` is empty. This is exactly the set that keeps a round from being clean.
- **Audit**: a recorded row attributed to that role whose `status` is not `PASS`. This is exactly the set `tp audit --record` counts.

The scan covers **every recorded round of that phase**, deduped by the same key the round-carry uses, not only the latest. Scoping it to the latest round would leave the guard trivially bypassable: a role that contributed no row to round N — because it was skipped, or reported nothing new — would hide its still-open rows from round N-1. It would also be wrong for review, where `loadReviewRoundState` already carries findings from every recorded round rather than just the previous one.

When the check fires, the command exits 2 naming the role, the count of unresolved items, and **the recorded round file each unresolved item lives in** (`.tp-review/<spec>/review-round-N.ndjson` or `audit-round-N.ndjson`). Naming the file is what makes the escape hatch reachable: `tp review --resolve` takes an arbitrary findings NDJSON as its positional, and the documented workflow resolves a merged working file, which does not touch recorded state. Resolving the merged file and re-running would hit the same refusal with no indication of why. A phase with no recorded round has nothing unresolved and deactivation proceeds.

### 2.7 Precedence when more than one rule applies

Three outcomes can apply to one invocation, and the order is fixed: domain filtering first (§2.3), then the unknown-id warning (§2.3), then §2.6's unresolved-findings refusal, then §2.5's empty-phase refusal. §2.6 precedes §2.5 because it names a specific role and a specific file to act on, which is more actionable than "you disabled everything"; a spec that trips both gets the finding-bearing role named first, and re-running after resolving it then reports the empty phase if one remains.

## 3. Corpus staleness needs no change

`ComputeRolesHash` hashes the phase's role **files**, and a spec-frontmatter override is deliberately excluded from it because the override lives in the spec and is already covered by `spec_hash`. A per-spec `enabled: false` is therefore covered for free: editing it changes the spec, which changes `spec_hash`, which the existing staleness check already reports. No hashing change is part of this release, and a test pins it: toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` untouched.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and the role-corpus rule in `CLAUDE.md` describe the frontmatter override shape. Every one of the four must contain the substring `enabled: false deactivates a role for one spec` and must state the `disabled-by-spec` skip reason, the two refusals of §2.5 and §2.6 with their emission-only scope, and the relationship to `domains`: `domains` expresses what kind of work a role is for and is the right tool when the distinction is durable, while `enabled: false` expresses a one-spec exception and is the right tool when it is not.

`REFERENCE.md` additionally documents the overlap report, whose `trim_candidate` flag advises trimming a role repository-wide. It must state that a trim candidate has two levers — deleting the role file removes it from every spec, `enabled: false` removes it from one — and that a role carrying unresolved findings must be resolved before either.

## 5. Non-Goals

1. **A list-shaped allowlist.** The reported suggestion of `review_roles: [implementer, tester, auth-context]` alongside the existing map form would require parsing two shapes for one key and would make "listed" mean "allowed" in one shape and "overridden" in the other. The map plus `enabled` expresses the same intent in the shape that already exists, and §1's field data argues against allowlist semantics specifically: a role omitted by accident can cost a critical finding.
2. **Changing the `trim_candidate` output.** An earlier draft made the trim-candidate *rendering* name both levers. That was wrong about the tree and about the audience: there is no rendered advice, only the literal `[trim candidate]` flag appended to a per-role line in `tp review --report`'s TTY block, the audit phase has no TTY rendering at all, and tp emits JSON whenever stdout is not a terminal — so the one surface the change would touch is the one no agent ever reads. Adding a machine-readable advice field would be a new payload key for a decision the driver can already make from `unique`/`shared`. The guidance goes in `REFERENCE.md` (§4) instead, where a human reads it.
3. **Re-enabling a filtered role.** `enabled: true` does not resurrect a role removed by `domains` or absent from the corpus. Domain filtering stays the coarse axis and frontmatter stays a per-spec refinement of the active set, never a way to widen it.
4. **Deactivating the built-in `regression` role.** It is tp's convergence machinery rather than a corpus lens, it accepts no overrides today, and switching it off would let a spec converge without the round-over-round diff that makes convergence honest.
5. **A project-level default deactivation.** Turning a role off for every spec is already expressible: delete or stop committing its file. A `.tp/config.json` disable list would be a second, redundant source of truth for corpus membership.
6. **Per-task role scoping.** A single spec whose tasks run in two repositories needs the corpus resolved per task, not per spec. That is a different mechanism and a separate release; `enabled: false` does not address it.

## 6. Tests

1. `enabled: false` on an active role removes it from the emitted panel, and the role appears in `skipped_roles` with reason `disabled-by-spec`.
2. `enabled: false` and `focus` on the same role deactivate it and apply no focus anywhere in the emitted output.
3. `enabled: true` is accepted, emits no warning, and leaves the role active with its focus layered as usual.
4. A non-boolean `enabled` produces a lint warning and leaves the role active.
5. A key other than `focus` and `enabled` still produces the existing "not a permitted override key" warning.
6. `enabled: false` on an id matching no active role produces the existing "matches no active role" warning and changes nothing; a role already dropped by `domains` is reported once, with `domain-mismatch`.
7. Deactivating every role of a phase exits 2 on emission, names the phase and the deactivated ids, emits nothing, and leaves no round snapshot on disk — the snapshot assertion is what pins the §2.5 ordering change.
8. The same spec that exits 2 on emission still exits normally under `--record`, `--status`, `--status --check`, `--merge`, `--resolve`, `--verify` and `--report`, for both `tp review` and `tp audit`. Without this the refusal would break the convergence gate and its own escape hatch.
9. Deactivating a role with a non-`PASS` row in a recorded audit round exits 2, names the role, the unresolved count, and the round file; resolving that row in the recorded file lets the same deactivation succeed.
10. Deactivating a role with a review finding that `reviewFindingResolvedAway` does not treat as resolved away exits 2 the same way; a finding resolved `wontfix` with evidence does not block, and one resolved `fixed` does — matching the convergence predicate exactly.
11. An unresolved row recorded in round 1, with the role contributing nothing in rounds 2 and 3, still blocks deactivation — the latest-round-only bypass.
12. A spec whose only `review_roles` entry is an `enabled: false` suppresses the legacy `tp: lens` shim, matching a spec whose only entry adds focus.
13. Toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` unchanged.
14. `--compact` still omits `skipped_roles`, including entries carrying the `disabled-by-spec` reason.
15. A spec that trips both §2.5 and §2.6 reports the §2.6 refusal first, per §2.7.
16. A guard test asserts `README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md` and `CLAUDE.md` each contain `enabled: false deactivates a role for one spec`, and that `REFERENCE.md` states both trim-candidate levers.
