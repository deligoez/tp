# tp v0.32.0 — Per-spec role activation

## 1. Overview

The role corpus is repository-wide. A spec's frontmatter can append focus questions to a role, but it cannot switch a role off. Field feedback from a repository running six reviewers and four auditors reported the consequence: for a spec about multi-tenant authorization two lenses are the highest-value ones and the rest are noise, and each noise role still costs a full sub-agent round. As a corpus grows, per-spec waste grows with it, and the only lever tp offers is the overlap report's `trim_candidate`, which advises trimming the role **repository-wide** — the opposite of what a single noisy spec calls for.

A partial mechanism already exists and the report understates it: `domains` on a role plus `domain:` in the spec frontmatter does deactivate a role per spec, and the dropped role is named in `skipped_roles` with reason `domain-mismatch`. But `domains` is a taxonomy axis — it answers "what kind of work is this role for", not "this one refactor spec does not need it" — and a spec carries exactly one domain, so expressing an arbitrary per-spec subset through domains means inventing domains that describe nothing.

A second report from the same repository settles which problem this is. Its round-1 overlap report over 92 merged findings credited the three project-authored roles with 38 findings between them and exactly one cluster shared with the built-ins, and two of the round's most expensive defects came from roles the default corpus does not contain — an `auth-context` role that found a middleware registration list complete for route groups but missing `/broadcasting/auth`, and a `data-migration` role that found a column the spec proposed to drop carrying 146k historical rows plus a foreign key, making the spec's `ALGORITHM=INSTANT` claim false. So a large corpus is not a value problem: the roles earn their tokens on a spec in their domain. It is a **noise** problem — those same roles earn nothing on a spec outside their domain, and today there is no way to say so per spec. That is what this release adds, and it is also why the mechanism is deny-only: an allowlist would silently drop a role someone forgot to list, and this data says a dropped role can cost a critical finding.

This release adds an explicit per-spec off switch to the mechanism that already exists for focus, with the transparency and honesty guards tp applies everywhere else: a deactivated role stays visible in `skipped_roles`, a phase cannot be emptied, and a role carrying unresolved findings cannot be silently removed from the loop.

This spec assumes the v0.31.2 auditor routing, in which `spec-coverage` is the only id with a dedicated checklist source and every other role receives one item per affected code file.

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

`Frontmatter.ReviewRoles` and `Frontmatter.AuditRoles` change from `map[string][]string` to a map from role id to an override struct carrying the focus list and a three-valued `enabled` (unset, true, false). Unset is the current behavior. The parse warnings for a non-mapping value, a non-list `focus`, and a non-string focus element are unchanged; a non-boolean `enabled` is a new lint warning and is ignored, leaving the role active.

Because a value of `enabled: false` is currently rejected with the "not a permitted override key" warning and ignored, a spec written for this release degrades safely on an older tp: the role stays active and the user is told why.

### 2.3 Resolution drops the role before focus layering

`ResolveOverrideFocus` drops every role whose override sets `enabled: false` from the phase's effective corpus, and never applies focus to a dropped role. The drop happens after domain filtering, so a role already removed by `domains` is not reported twice, and after the active-role check, so `enabled: false` on an id matching no active role produces the existing "matches no active role" warning rather than a silent success.

The legacy `tp: lens` shim applies only when `tp.review_roles` and `tp.audit_roles` are both absent. A spec that uses `review_roles` solely to deactivate a role therefore suppresses the shim, exactly as one that uses it to add focus does; the spec has opted into the current mechanism.

### 2.4 A deactivated role stays visible

Every role deactivated by `enabled: false` is named in the phase's `skipped_roles` with the new reason `disabled-by-spec`, alongside the existing `no-checklist-items`, `no-spec-change`, `domain-mismatch`, and `no-baseline`. The reason is documented with the others, and `skipped_roles` remains omitted under `--compact` as it is today. A driver reading the happy path can therefore always tell an intentionally deactivated role from a silently dropped one.

### 2.5 A phase cannot be emptied

Deactivating every role of a phase would produce a review or audit round that emits nothing and records nothing while still counting as a round. `tp review` and `tp audit` refuse it: when the `enabled: false` set would leave a phase with no active role, the command exits 2 with an error naming the phase and the deactivated ids, and a hint to re-enable at least one role. Nothing is emitted and no state is written. The built-in `regression` role does not count as an active reviewer for this check, because it is appended to emission separately and cannot carry the phase alone.

### 2.6 A role with unresolved findings cannot be deactivated

The prior-round carry mechanism re-presents a role's own unresolved rows in its next-round prompt. Deactivating a role would therefore drop its open findings out of the loop while the recorded state still holds them — a spec could reach a clean round by removing the lens that was failing. tp refuses this.

When a role targeted by `enabled: false` has unresolved rows in the latest recorded round of that phase — a non-PASS audit row, or an unresolved review finding — the command exits 2 with an error naming the role and the count of unresolved items, and a hint pointing at `tp review --resolve` / `tp audit --record`. The escape hatch is the existing one: resolve the findings, with a disposition and recorded evidence, and then deactivate the role. A phase with no recorded round yet has nothing unresolved and deactivation proceeds.

## 3. The overlap report points at the right lever

`trim_candidate` marks a role that contributed no sole finding and at least one shared finding. Its rendered advice reads as a repository-wide instruction, which is wrong for a role that is valuable elsewhere and redundant on this spec. The TTY rendering of a trim candidate names both levers: deleting the role file removes it from every spec, and `enabled: false` in this spec's frontmatter removes it from this one. The `trim_candidate` field itself, its computation, and the JSON shape are unchanged — this is rendered advice only.

## 4. Corpus staleness needs no change

`ComputeRolesHash` hashes the phase's role **files**, and a spec-frontmatter override is deliberately excluded from it because the override lives in the spec and is already covered by `spec_hash`. A per-spec `enabled: false` is therefore covered for free: editing it changes the spec, which changes `spec_hash`, which the existing staleness check already reports. No hashing change is part of this release, and a test pins that: toggling `enabled` changes `spec_hash` and leaves `review_roles_hash` and `audit_roles_hash` untouched.

## 5. Documentation

`README.md`, `skills/tp/SKILL.md`, and the role-corpus rule in `CLAUDE.md` describe the frontmatter override shape. Each must state the two permitted keys, the `disabled-by-spec` skip reason, the two refusals of §2.5 and §2.6, and the relationship to `domains`: `domains` expresses what kind of work a role is for and is the right tool when the distinction is durable, while `enabled: false` expresses a one-spec exception and is the right tool when it is not.

## 6. Non-Goals

1. **A list-shaped allowlist.** The reported suggestion of `review_roles: [implementer, tester, auth-context]` alongside the existing map form would require parsing two shapes for one key and would make "listed" mean "allowed" in one shape and "overridden" in the other. The map plus `enabled` expresses the same intent in the shape that already exists.
2. **Re-enabling a filtered role.** `enabled: true` does not resurrect a role removed by `domains` or absent from the corpus. Domain filtering stays the coarse axis and frontmatter stays a per-spec refinement of the active set, never a way to widen it.
3. **Deactivating the built-in `regression` role.** It is tp's convergence machinery rather than a corpus lens, it accepts no overrides today, and switching it off would let a spec converge without the round-over-round diff that makes convergence honest.
4. **A project-level default deactivation.** Turning a role off for every spec is already expressible: delete or stop committing its file. A `.tp/config.json` disable list would be a second, redundant source of truth for corpus membership.
5. **Per-task role scoping.** A single spec whose tasks run in two repositories needs the corpus resolved per task, not per spec. That is a different mechanism and a separate release; `enabled: false` does not address it.

## 7. Tests

1. `enabled: false` on an active role removes it from the emitted panel, and the role appears in `skipped_roles` with reason `disabled-by-spec`.
2. `enabled: false` and `focus` on the same role deactivate it and apply no focus anywhere in the emitted output.
3. `enabled: true` is accepted, emits no warning, and leaves the role active and its focus layered as usual.
4. A non-boolean `enabled` produces a lint warning and leaves the role active.
5. A key other than `focus` and `enabled` still produces the existing "not a permitted override key" warning.
6. `enabled: false` on an id matching no active role produces the existing "matches no active role" warning and changes nothing.
7. Deactivating every role of a phase exits 2, names the phase and the deactivated ids, emits nothing, and writes no state.
8. Deactivating a role with an unresolved non-PASS row in the latest recorded audit round exits 2 and names the role and the unresolved count; resolving the row first lets the same deactivation succeed.
9. Deactivating a role with an unresolved review finding in the latest recorded review round exits 2 the same way; the same deactivation succeeds against a spec with no recorded round.
10. A spec whose only `review_roles` entry is an `enabled: false` suppresses the legacy `tp: lens` shim, matching a spec whose only entry adds focus.
11. Toggling `enabled` changes `spec_hash` and leaves `review_roles_hash` and `audit_roles_hash` unchanged.
12. `--compact` still omits `skipped_roles`, including entries carrying the `disabled-by-spec` reason.
