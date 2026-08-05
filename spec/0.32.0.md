# tp v0.32.0 — Per-spec role activation

## 1. Overview

The role corpus is repository-wide. A spec's frontmatter can append focus questions to a role but cannot switch one off. For a spec about multi-tenant authorization two lenses are the highest-value ones and the rest are noise, and each noise role still costs a full sub-agent round.

Field data says this is a noise problem, not a value problem. In one repository's round-1 overlap report over 92 merged findings, three project-authored roles produced 38 findings and shared exactly one cluster with the built-ins; two of that round's most expensive defects came from roles the default corpus does not contain. Those roles earn their tokens on a spec in their domain and earn nothing outside it. The mechanism is deny-only for the same reason: a role omitted from an allowlist by accident can cost a critical finding.

`domains` already deactivates a role per spec, reported as `domain-mismatch`. It is a taxonomy axis — a spec carries one domain — so expressing an arbitrary per-spec subset through it means inventing domains that describe nothing.

Declining one auditor saves, per phase round: one sub-agent invocation, its prompt (which since v0.31.2 carries the `## Project Context` block up to `claudeMDExcerptLineCap`, the `## Disposition` block, and up to `CodeFileCap` file lines), the files that role reads, and one result row per file to merge and read. It costs one optional frontmatter key, one `skipped_roles` reason, and one refusal. No new flag, command, or JSON payload field.

This spec assumes the v0.31.2 routing, in which `spec-coverage` is the only auditor id that changes routing.

## 2. `enabled` in the spec-frontmatter role overrides

### 2.1 The override value gains a second permitted key

`tp.review_roles` and `tp.audit_roles` keep their shape. The override object accepts a second key, `enabled`, a boolean:

```yaml
tp:
  audit_roles:
    data-migration: { enabled: false }
    multi-tenant-isolation:
      focus:
        - "Does every query filter by the active tenant?"
```

`enabled` and `focus` may appear together; `enabled: false` deactivates the role and its focus is never applied. `enabled: true` is accepted and is a no-op. Any other key remains a lint warning, and that warning's text changes with the permitted set: `is not a permitted override key (only focus)` becomes `(only focus and enabled)`.

On an older tp, `enabled: false` is rejected as an unpermitted key and ignored, so the role stays active and the user is told why.

### 2.2 The parsed override becomes a struct

`Frontmatter.ReviewRoles` and `Frontmatter.AuditRoles` change from `map[string][]string` to `map[string]RoleOverride`, carrying `Focus []string` and `Enabled *bool` (nil = unset = today's behavior).

Three sites move with the type: `parseRoleOverrides` (`internal/engine/frontmatter.go`) writes it; `ResolveOverrideFocus` (`internal/engine/overrides.go`) reads it and emits the "matches no active role" warning itself; `TranslateLegacyLens` returns `map[string][]string` into the same variable as `fm.ReviewRoles` and must return the new type or be adapted at the call site.

A non-boolean `enabled` is a lint warning and is ignored, leaving the role active. The existing warnings for a non-mapping value, a non-list `focus`, and a non-string focus element are unchanged.

### 2.3 Resolution drops the role and reports what it dropped

`ResolveOverrideFocus` returns a third value: the ids dropped for `enabled: false`. Its two callers (`internal/cli/review.go`, `internal/cli/audit.go`) pass that set to §2.4's `skipped_roles` assembly and §2.5's refusal.

The drop is applied **outside** `ResolveActiveCorpus` and after its domain filtering, so it never triggers that function's empty-panel fallback to the embedded corpus. A role already removed by `domains` is reported once, with `domain-mismatch`; `enabled: false` on an id matching no active role produces the existing "matches no active role" warning.

A spec that uses `review_roles` only to deactivate a role suppresses the legacy `tp: lens` shim, as one that adds focus does.

### 2.4 A deactivated role stays visible

Every role deactivated by `enabled: false` is named in `skipped_roles` with the new reason `disabled-by-spec`, alongside `no-checklist-items`, `no-spec-change`, `domain-mismatch`, and `no-baseline`.

`skipped_roles` remains omitted under `--compact`. Under `--compact` a driver cannot distinguish a deactivated role from any other absence; this is accepted, because the deactivation is declared in the spec the driver already holds.

### 2.5 A phase cannot be emptied

**On prompt emission**, when the `enabled: false` set would leave a phase with no active role, the command exits 2 naming the phase and the deactivated ids, with a hint to re-enable at least one role. The built-in `regression` role does not count as an active reviewer for this check.

`enabled: false` on `spec-coverage` is refused the same way, whatever else the phase holds: `routeChecklist` routes every spec-derived item to that id alone, so deactivating it drops the entire spec-derived checklist while every other auditor still emits.

1. The refusal fires only in each command's prompt-emission mode, including `tp review --perspective`. It must not fire under `tp review --record`, `--status` (with or without `--check`), `--merge`, `--resolve`, `--resolve-all`, `--verify`, `--report`, nor under `tp audit --record`, `--status` (with or without `--check`), `--merge`. `tp audit` registers no `--resolve`, `--resolve-all`, `--verify` or `--report`.
2. Corpus and override resolution move ahead of every write the emission path performs — ahead of `EnsureReviewState` (which creates `.tp-review/<spec>/` and `state.json` before `WriteSnapshotAtomic`) in `tp review`, and ahead of the snapshot in `tp audit`.

### 2.6 Precedence

Domain filtering (§2.3), the unknown-id warning (§2.3), then §2.5's refusal. Only §2.5 stops the run.

## 3. Corpus staleness needs no change

`ComputeRolesHash` hashes the phase's role files; a frontmatter override is covered by `spec_hash`. Toggling `enabled` changes the recorded round's `spec_hash` and leaves its `roles_hash` unchanged.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and the role-corpus rule in `CLAUDE.md` must each contain the substring `enabled: false deactivates a role for one spec`, and must state the `disabled-by-spec` reason, §2.5's two emission-only refusals (empty phase, and `spec-coverage`), and that `domains` is the durable axis while `enabled: false` is the one-spec exception.

`REFERENCE.md` must state that a `trim_candidate` has two levers — `enabled: false` removes the role from one spec; deleting its file removes it from every spec **unless it is the phase's last role file**, in which case `ResolveActiveCorpus` falls back to the embedded default corpus — and that tp does not guard against deactivating a role with open findings (Non-Goal 6).

## 5. Non-Goals

1. **A list-shaped allowlist.** Two YAML shapes for one key, with "listed" meaning "allowed" in one and "overridden" in the other. §1's data argues against allowlist semantics directly.
2. **Changing the `trim_candidate` output.** There is no rendered advice — only the `[trim candidate]` flag in `tp review --report`'s TTY block, which no agent sees, and audit has no TTY rendering. The guidance goes in `REFERENCE.md`.
3. **Re-enabling a filtered role.** `enabled: true` does not resurrect a role removed by `domains`.
4. **Deactivating the built-in `regression` role.** It is convergence machinery, accepts no overrides, and switching it off would let a spec converge without the round-over-round diff.
5. **A project-level default deactivation.** Deleting the role file already expresses it.
6. **Any guard on deactivating a role with open findings**, refusal or warning. `ReviewRoundClean` evaluates cleanliness per round over that round's rows, so a deactivated role's earlier findings do not keep a later round from being clean — no per-spec mechanism closes that without changing convergence semantics. A refusal is additionally unenforceable (`--no-state` and §2.5's emission-only scope) and needs per-role attribution that is advisory on recorded rows.
7. **Per-task role scoping.** A spec whose tasks run in two repositories needs the corpus resolved per task.

## 6. Tests

1. `enabled: false` removes the role from the emitted panel and names it in `skipped_roles` with `disabled-by-spec`.
2. `enabled: false` with `focus` on the same role deactivates it and applies no focus anywhere in the output.
3. `enabled: true` emits no warning and leaves the role active with its focus layered.
4. A non-boolean `enabled` warns and leaves the role active.
5. An unpermitted key still warns, with the text `(only focus and enabled)`.
6. `enabled: false` on an unknown id warns "matches no active role" and changes nothing; a role dropped by `domains` is reported once, with `domain-mismatch`.
7. Deactivating every role of a phase exits 2, emits nothing, and leaves no `.tp-review/<spec>/`, no `state.json`, and no snapshot — asserted for `tp review` and `tp audit` separately.
8. The same spec, under the other modes that take it as the positional, never produces the empty-phase error: `tp review --record` and `tp audit --record` exit 0; `--status --check` exits 0 or 1 by convergence. Asserting the absence of the error message rather than a non-2 code is what fails if the refusal leaks, since a malformed argument also exits 2. `--merge`, `--resolve`, `--resolve-all` and `--report` take an NDJSON positional and never parse the spec, so they are unaffected by construction and are not asserted.
9. `regression` does not satisfy §2.5's emptiness check: a review spec deactivating every corpus reviewer still exits 2.
10. `enabled: false` on `spec-coverage` exits 2 even when other auditors remain active.
11. A spec deactivating every user role exits 2 rather than falling back to the embedded corpus — pinning §2.3's placement.
12. A spec whose only `review_roles` entry is `enabled: false` suppresses the legacy `tp: lens` shim.
13. Toggling `enabled` changes `spec_hash` and leaves `roles_hash` unchanged.
14. `--compact` omits `skipped_roles`, including `disabled-by-spec` entries.
15. A guard test asserts the four documents contain `enabled: false deactivates a role for one spec`, and that `REFERENCE.md` states both trim-candidate levers.

### 6.1 Existing tests this change invalidates

Derived by running the search, not asserted: the tokens `ReviewRoles`, `AuditRoles`, `ResolveOverrideFocus`, `TranslateLegacyLens`, `parseRoleOverrides`, and `is not a permitted override key (only focus)` over `internal/**/*_test.go` return exactly four files.

1. `internal/engine/frontmatter_roleoverrides_test.go` — asserts the parsed `ReviewRoles`/`AuditRoles` shape and the `(only focus)` literal; moves with §2.2's type and §2.1's message.
2. `internal/engine/overrides_test.go` — asserts `ResolveOverrideFocus`'s two return values and builds override map literals; gains the third value and the new element type.
3. `internal/engine/lens_shim_test.go` — builds `ReviewRoles` literals and calls `TranslateLegacyLens`, whose return type changes.
4. `internal/cli/domain_lens_test.go` — exercises `ResolveOverrideFocus` through the CLI; moves with its new signature.
