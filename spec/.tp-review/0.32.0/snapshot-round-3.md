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

`Frontmatter.ReviewRoles` and `Frontmatter.AuditRoles` change from `map[string][]string` to `map[string]RoleOverride`, where `RoleOverride` carries `Focus []string` and `Enabled *bool` (nil meaning unset, which is today's behavior). Two sites move with the type, and an earlier draft named a third that does not exist. The real readers are `ResolveOverrideFocus` (`internal/engine/overrides.go`), which both layers the focus and emits the "matches no active role" warning itself — there is no separate lint pass for that — and `TranslateLegacyLens`, whose declared return type is `map[string][]string` and whose result is assigned to the same `overrides` variable as `fm.ReviewRoles`; it must return the new type or be adapted at the call site, or the package will not compile. `parseRoleOverrides` in `internal/engine/frontmatter.go` is the writer and changes with the field.

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

Deactivating every role of a phase would produce a review or audit round that emits nothing and records nothing while still counting as a round. **On prompt emission**, when the `enabled: false` set would leave a phase with no active role, the command exits 2 — a usage error, because the spec's own declaration is what is wrong — naming the phase and the deactivated ids, with a hint to re-enable at least one role. The built-in `regression` role does not count as an active reviewer for this check, because it is appended to emission separately and cannot carry the phase alone.

Two scoping rules make this buildable and non-deadlocking, and both are load-bearing:

1. **The refusal fires only in the prompt-emission mode of each command**, which is the only mode that resolves the role corpus. The modes it must not fire in are per command, because their flag sets differ: for `tp review` those are `--record`, `--status` (with or without `--check`), `--merge`, `--resolve`, `--resolve-all`, `--verify` and `--report`; for `tp audit` they are `--record`, `--status` (with or without `--check`) and `--merge` — `tp audit` registers no `--resolve`, `--resolve-all`, `--verify` or `--report`. A refusal that fired on `--status --check` would break the convergence gate the whole loop runs on, and one that fired on `tp review --resolve` would break §2.6's own escape hatch.
2. **Role resolution moves ahead of every write the emission path performs.** Today both commands write before resolving roles, and the snapshot is not the first write: `tp review`'s `loadReviewRoundState` calls `engine.EnsureReviewState`, which creates `.tp-review/<spec>/` and saves a fresh `state.json` before `WriteSnapshotAtomic` runs, and `tp audit`'s `loadAuditSpec` snapshots likewise. Corpus resolution and override resolution move ahead of `EnsureReviewState` and of the audit snapshot, so both refusals are decidable while nothing — directory, state file, or snapshot — has been written. §6 item 7 asserts the absence of all three, not only the snapshot.

### 2.6 A role with unresolved findings cannot be deactivated

The prior-round carry re-presents a role's unresolved work in its next-round prompt. Deactivating that role would drop the work out of the loop while the recorded state still holds it — a spec could reach a clean round by removing the lens that was failing. tp refuses this on prompt emission, with the same mode scoping as §2.5, and exits **4** rather than 2: the trigger is recorded state under `.tp-review/`, not a malformed invocation, and tp's exit-code table reserves 4 for state.

The two phases do not share a definition, a scan range, or an escape hatch, and an earlier draft's attempt to unify them was the source of its errors. They are specified separately.

**Review.** A role is blocked when a recorded finding stamped with that role would count against convergence — that is, when it survives `engine.reviewFindingResolvedAway` *and* passes the severity filter `ReviewRoundClean` applies under the effective `review_converge_on`. Tying the guard to the same predicate as `clean` is the point: under the default `blocking` policy a medium finding does not keep a round from being clean, so it must not block deactivation either, or the guard would be stricter than convergence. The scan covers **every recorded round** of the phase, matching `loadReviewRoundState`, which carries findings from all recorded rounds rather than only the previous one. Role attribution is read from the `role` field the output contract stamps on each recorded row — not from the round-carry's dedup key, which is `(location, class)` and carries no role at all. The escape hatch is `tp review --resolve` **against the recorded round file** (`.tp-review/<spec>/review-round-N.ndjson`), and the refusal names that file per unresolved item: `--resolve` takes an arbitrary NDJSON positional, and the documented workflow resolves a merged working file, which never touches recorded state.

**Audit.** A role is blocked when the **latest** recorded audit round holds a row stamped with that role whose `status` is not `PASS`. The range is the latest round alone because that is exactly what `loadAuditPriorRound` carries, and because an audit row is not resolved but re-derived: there is no `tp audit --resolve`, and a row stops blocking when a later recorded round reports `PASS` for it. The escape hatch is therefore the ordinary audit loop — fix the code, run the round, record it — and the refusal says so rather than pointing at a resolve command that does not exist.

Both arms require recorded state to read. `tp review --no-state` disables state reads and writes entirely and skips `loadReviewRoundState`, so the guard cannot run under it; in that mode the guard is skipped and a warning says so, because silently allowing the deactivation would be the dishonest half of the choice. A phase with no recorded round has nothing unresolved and deactivation proceeds.

### 2.7 Precedence when more than one rule applies

Three outcomes can apply to one invocation, and the order is fixed: domain filtering first (§2.3), then the unknown-id warning (§2.3), then §2.6's unresolved-findings refusal (exit 4), then §2.5's empty-phase refusal (exit 2). §2.6 precedes §2.5 because it names a specific role and a specific thing to do about it, which is more actionable than "you disabled everything"; a spec that trips both gets the finding-bearing role named first, and re-running after clearing it then reports the empty phase if one remains.
## 3. Corpus staleness needs no change

`ComputeRolesHash` hashes the phase's role **files**, and a spec-frontmatter override is deliberately excluded from it because the override lives in the spec and is already covered by `spec_hash`. A per-spec `enabled: false` is therefore covered for free: editing it changes the spec, which changes `spec_hash`, which the existing staleness check already reports. No hashing change is part of this release, and a test pins it: toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` untouched.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and the role-corpus rule in `CLAUDE.md` describe the frontmatter override shape. Every one of the four must contain the substring `enabled: false deactivates a role for one spec` and must state the `disabled-by-spec` skip reason, the two refusals of §2.5 and §2.6 with their emission-only scope, and the relationship to `domains`: `domains` expresses what kind of work a role is for and is the right tool when the distinction is durable, while `enabled: false` expresses a one-spec exception and is the right tool when it is not.

`REFERENCE.md` additionally documents the overlap report, whose `trim_candidate` flag advises trimming a role repository-wide. It must state that a trim candidate has two levers — `enabled: false` removes the role from one spec, and deleting its file removes it from every spec **unless it is the phase's last role file**, in which case `ResolveActiveCorpus` falls back to the embedded default corpus rather than leaving the phase empty — and that a role carrying unresolved findings must be cleared before either.

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
7. Deactivating every role of a phase exits 2 on emission, names the phase and the deactivated ids, emits nothing, and leaves no `.tp-review/<spec>/` directory, no `state.json` and no round snapshot on disk — asserting all three is what pins the §2.5 ordering change, since the snapshot is not the first write.
8. The same spec that exits 2 on emission still exits normally under every other mode of each command, enumerated per command because their flag sets differ: `tp review` under `--record`, `--status`, `--status --check`, `--merge`, `--resolve`, `--resolve-all`, `--verify` and `--report`; `tp audit` under `--record`, `--status`, `--status --check` and `--merge` only. Without this the refusal would break the convergence gate and its own escape hatch.
9. **Audit arm.** Deactivating a role with a non-`PASS` row in the latest recorded audit round exits 4, names the role, the unresolved count, and the round file, and its hint points at the audit loop rather than a resolve command. Recording a later round in which that role's rows are `PASS` lets the same deactivation succeed — there is no `tp audit --resolve` to reach for.
10. **Review arm.** Deactivating a role blocks on a recorded finding that survives `reviewFindingResolvedAway` *and* the severity filter of the effective `review_converge_on`, and exits 4. All five decisive branches are covered: not resolved at all blocks; `fixed` blocks; `wontfix` with evidence does not; `duplicate` with evidence does not; `wontfix` or `duplicate` with empty or whitespace-only evidence blocks. Under the default `blocking` policy a surviving `medium` finding does **not** block, matching what `ReviewRoundClean` counts.
11. A blocking review finding recorded in round 1, with the role contributing nothing in rounds 2 and 3, still blocks deactivation — the every-round scan the review arm requires. The audit arm is the opposite by design: a non-`PASS` row in round 1 that the role's round-3 rows have superseded with `PASS` does not block, because the audit scan is latest-round-only.
12. A spec whose only `review_roles` entry is an `enabled: false` suppresses the legacy `tp: lens` shim, matching a spec whose only entry adds focus.
13. Toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` unchanged.
14. `--compact` still omits `skipped_roles`, including entries carrying the `disabled-by-spec` reason.
15. `tp review --no-state` on a spec that deactivates a role carrying a blocking finding skips the §2.6 guard and warns that it did, rather than allowing the deactivation silently — the guard needs recorded state and `--no-state` denies it.
16. A spec that trips both §2.5 and §2.6 reports the §2.6 refusal first, per §2.7.
17. A guard test asserts `README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md` and `CLAUDE.md` each contain `enabled: false deactivates a role for one spec`, and that `REFERENCE.md` states both trim-candidate levers.
