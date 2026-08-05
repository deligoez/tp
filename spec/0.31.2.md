# tp v0.31.2 — De-magic the auditor routing

## 1. Overview

Field feedback from a Laravel/PHP repository reported that the `security` auditor never receives checklist items and is silently skipped with `no-checklist-items`, even on a spec whose sections are explicitly about authorization and input validation. The report was reproduced against both the built-in corpus and an ejected `--domain software` corpus.

The reporter's diagnosis — that audit checklist items derive only from the spec's structured elements, so a code-property lens can never earn an item — is not what the code does. `generateRoleAuditPrompts` dispatches on the role's **id string**, and its `default` branch already gives every other role one `file_check` item per affected file, which is exactly the code-property behavior the report asks for. Only two ids behave differently, and one of them is harmful:

- `spec-coverage` receives the spec-derived and finding items. This is intended and stays.
- `security` receives `AuditFileSelection.Security`, a list filtered by the hardcoded substrings `lock`, `validate`, `auth`, `secret`, `perm` matched against the file path or the first 200 lines of the file at HEAD. A PHP middleware matches none of them, so the list is empty, so the role is dropped.
- `maintainability-conventions` receives `AuditFileSelection.Maintainability` — which is byte-for-byte what the `default` branch already computes. This arm is dead weight.

A third id dispatch lives outside that switch: `buildRolePrompt` attaches the `## Project Context` excerpt of `CLAUDE.md` only when the role id is `maintainability-conventions`, so a conventions auditor authored under any other id judges conventions without ever seeing the project's conventions.

Two properties were confirmed empirically. Renaming the role id from `security` to any other value, with the role file otherwise unchanged, makes the same role emit a prompt against the same file. Renaming the *file* from `Dummy.php` to `AuthDummy.php`, with the role id left at `security`, also makes it emit. So the behavior depends on a role id string and an English keyword list, neither of which is part of the role contract tp documents.

This release removes the id-based dispatch and neutralizes the language-specific default auditor prompts. It is a patch: no new commands, no new flags, no schema change.

## 2. Auditor routing stops dispatching on role id

### 2.1 Only spec-coverage keeps a dedicated checklist source

`generateRoleAuditPrompts` reduces its switch to two arms. The `spec-coverage` id keeps its spec-derived checklist and its own file selection. Every other role — built-in, ejected, or user-authored — takes the shared arm: one `file_check` item per selected code file, with the item id, text, and expected evidence produced by `fileCheckItems` from that role's own id, exactly as custom roles already receive today. The `security` and `maintainability-conventions` arms are deleted; neither id carries any behavior the shared arm does not.

The observable consequence is the fix, stated precisely: a role named `security` and an identically-authored role under any other id receive the same number of items over the same files, in the same order, differing only in the role id that `fileCheckItems` embeds in each item's `item_id` and `text`. Their prompts are likewise identical modulo the role id and the output path.

The caller chain collapses with the switch. `routeChecklist` no longer produces per-role buckets: it takes the spec-derived and finding entries and returns the single `spec-coverage` checklist, dropping its `sel` argument and its `sec`/`maint` return values. `generateRoleAuditPrompts` drops the `secItems` and `maintItems` parameters, and `runAudit` stops destructuring three values. Every non-`spec-coverage` role's items are built inside the shared arm by calling `fileCheckItems` with the role's own id.

### 2.2 A file outside a role's lens is a PASS

The shared arm hands every role one item per code file with no relevance filter, so a `security` role now receives an item for a file with no security surface. The audit status enum is `PASS`/`PARTIAL`/`FAIL`, and `tp audit --record` counts every non-`PASS` row against convergence, so an auditor answering "cannot evaluate, nothing here is in my domain" with `PARTIAL` would manufacture a false blocker on every round.

The expected disposition is therefore stated in the item itself. `fileCheckItems` extends the `expected_evidence` it generates so that each item instructs: inspect the file, and record `PASS` citing the inspected file when nothing in this role's domain appears in it. `PARTIAL` and `FAIL` remain reserved for a defect the role actually found. The wording is part of the emitted contract, not left to sub-agent judgment.

### 2.3 Every non-spec-coverage role receives the project conventions

`buildRolePrompt` stops gating the `## Project Context` block on the `maintainability-conventions` id. The block is attached to every role taking the shared arm whenever the `CLAUDE.md` excerpt is non-empty, and `spec-coverage` continues not to receive it — it judges spec coverage against the `## Spec Excerpt` block, which no other role gets. The resulting rule is stateable in one sentence: `spec-coverage` gets the spec, every other role gets the code files and the project's conventions.

The excerpt is capped at 50 lines by `claudeMDExcerptFor`, so the added cost is bounded and paid once per role prompt. It buys the correctness the gate was destroying: a conventions or house-style auditor authored under any id other than `maintainability-conventions` previously reported findings against conventions it had never been shown, and each such finding costs a resolve round-trip and can hold the two-consecutive-clean audit gate shut.

### 2.4 The security file heuristic and its inputs are removed

The keyword-filtered selection exists only to serve the deleted arm, so it goes with it:

1. `selectSecurity`, the `securitySubstrings` list, `securityHeadLineCap`, and the `headLines` helper are removed from `internal/engine/auditfiles.go`.
2. `AuditFileSelection.Security` is removed; `SelectAuditFiles` returns the spec-coverage list and the shared code-file list only.
3. `AuditFileInputs.HeadReader` is read only by `selectSecurity` and assigned only in `runAudit`, and `GitHeadReader` exists only to produce it, so the field, its assignment, and the function are all removed. `AuditFileInputs` keeps `Universe`, `DiffStats`, `Deleted`, and `TaskFiles`.

This narrows detection coverage in one case, and the release does not pretend otherwise. The deleted security list was capped at 20 files while the shared list caps at 10, so on an audit whose universe exceeds 10 files a keyword-matching file sorting alphabetically at position 11 through 20 previously reached the security lens and no longer does. The interim levers are `--affected-files` to scope the universe deliberately and, when it lands, the role-declared file selection of §5.1; the cap is not raised here because doing so would double the file list of every emitted role at once.

### 2.5 The shared selection is named for what it is

`AuditFileSelection.Maintainability` now serves every non-`spec-coverage` role, so a field named after one role is the same misleading-name trap that produced this defect. The field is renamed to `CodeFiles`, `selectMaintainability` to `selectCodeFiles`, and `MaintainabilityFileCap` to `CodeFileCap`. The cap value stays 10, the selection rule (first N universe files after the drop rules, alphabetical) is unchanged, and no emitted JSON field changes — this is an internal rename.

### 2.6 The affected-files header states the real cap

`buildRolePrompt` writes the fixed header `## Affected Files (max 20)`. After §2.4 the only role still bounded by 20 is `spec-coverage`; every other role is bounded by `CodeFileCap`, so the literal would overstate the cap in most prompts, and an auditor reading "max 20" above a 10-item list reasonably concludes nothing was truncated. The header is rendered from the cap actually applied to that role's list, and when the pre-cap universe held more files than the list shows, it also states how many of how many are shown, so a truncated file set is never presented as a complete one.

### 2.7 The skip outcomes that survive

`no-checklist-items` remains reachable for two distinct reasons, and both are reported the same way — the role is named in `skipped_roles` with that reason, and the audit exit code is unchanged:

1. `spec-coverage` is skipped when the spec has no table rows, no numbered list items, no task acceptance entries, and no findings, so no spec-derived item exists.
2. Every non-`spec-coverage` role is skipped when the shared code-file list is empty — the universe was empty, or `filterAuditUniverse` dropped every file as binary, as a `testdata/**` or `*.golden` fixture, or as deleted. A diff touching only images or fixtures therefore still skips every code-file auditor.

What can no longer happen is a role being skipped while other roles receive items, because a keyword list did not match a filename.

### 2.8 Existing tests this change invalidates

Three committed tests encode the behavior being removed and must change with it, and one must be widened because its current scope would let the defect return:

1. `TestGenerateAuditPrompts_EmptyRoleOmitted` (`internal/cli/audit_routing_test.go`) asserts `security` is absent from the prompts when the only affected file is `plain.go`. Its assertion inverts: `security` is emitted with one `file_check` item for `plain.go`. This becomes the regression guard of §6 item 1.
2. `TestRouteChecklist_Disjoint` (`internal/cli/audit_routing_test.go`) asserts `security` receives exactly the one auth-matching file. It is rewritten against `routeChecklist`'s new single return value, asserting the spec-coverage checklist alone.
3. `TestFilterFiles_Security` and the security half of `TestFilterFiles_Cap20` (`internal/engine/auditfiles_test.go`) test only the deleted selection and are deleted with it. The remaining assertions in that file are renamed mechanically to `CodeFiles` and `CodeFileCap`.
4. `TestGenerateAuditPrompts_CLAUDEmdOnlyForMaintainability` (`internal/cli/audit_named_test.go`) pins the id gate §2.3 removes. It is rewritten to assert that every non-`spec-coverage` role's prompt contains the conventions marker and that `spec-coverage`'s does not.

## 3. The default auditor prompts become language-neutral

### 3.1 The problem the defaults create

The embedded `software` auditor prompts are written in Go, and more narrowly in tp's own Go: `errors wrapped with %w`, `files written with 0o600`, `lowercase packages, camelCase symbols`, `_ = err` as a finding, `Files written by the tool`. In a PHP, TypeScript, or Ruby project these match nothing or produce noise, and — before §2 — a user could not tell, because the auditor was silently skipped. The three reviewer defaults (`implementer`, `tester`, `architect`) and the `spec-coverage` auditor are already language-neutral and are not touched.

### 3.2 security.json states invariants, not Go idioms

The `security` role keeps its id — ejected corpora and `tp.audit_roles` frontmatter overrides key on it — and its focus is rewritten so each line names an invariant that holds in any language:

1. Every acquired lock, transaction, or handle is released on all paths, including error and early-return paths.
2. No swallowed failure: an ignored error return, an empty catch, or an error discarded instead of propagated with context is a finding.
3. Values from user input, request parameters, or the environment are validated before they reach a query, a file path, or a command.
4. Queries, paths, and commands are built with the platform's parameterizing or joining API, never by string concatenation from input.
5. Authorization is checked at the point of the write, not only at the entry point, and never from a value the caller supplied.
6. Files and directories the tool creates use restrictive permissions, and secrets never reach logs, error messages, or committed files.

### 3.3 maintainability-conventions.json states properties, not Go syntax

The `maintainability-conventions` focus is rewritten the same way, and its `instructions` field drops the word `idiomatic` in favour of naming the project's own conventions as the standard:

1. Errors are propagated with context naming the failing operation, not rethrown bare and not flattened into a lossy string.
2. Publicly reachable symbols carry a doc comment explaining intent rather than restating the signature.
3. Functions stay short enough to read in one pass (roughly 80 lines); a longer one needs a stated reason.
4. Naming follows the conventions already present in the surrounding code rather than introducing a competing style.
5. No leftover TODO or FIXME without a ticket reference, no commented-out code, and no debug output left behind.

### 3.4 Ejection says the defaults are a starting point

`tp init --eject-roles` emits one advisory line stating that the written roles are starting points whose focus a project should rewrite for its own stack and conventions. The line goes to the human/info channel alongside the existing success message, is suppressed by `--quiet`, and adds no key to the eject JSON payload, whose shape stays exactly `{ejected, domain}`. The wording names no language, so it is emitted for every domain including `prose`. The written files stay byte-identical to the embedded corpus, which v0.25.0 §5.4 requires and the existing byte-identity test enforces.

### 3.5 The corpus hash deliberately stays `builtin`

`ComputeRolesHash` returns the `builtin` sentinel for a phase with no user role files, so rewriting the embedded prompts does not flip `roles_stale` and an in-flight audit sequence is not marked as having run under different instructions. This matches the v0.25.0 §5.2 precedent, under which content shipping with a tp release is excluded from the corpus hash because a tp upgrade is already a visible event. It is recorded here as a decision rather than left to read as an oversight; a project that wants its role content hashed can eject the corpus, at which point the files are hashed like any other user corpus.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and the role-corpus rule in `CLAUDE.md` each state part of the audit routing contract and must all tell one story: `spec-coverage` is the single reserved id with a dedicated checklist source, and every other id — built-in, ejected, or user-authored — is routed identically, receiving one item per affected code file plus the project conventions block. No doc may state that `security` or `maintainability-conventions` receives a special file list or a special prompt section. `REFERENCE.md` additionally documents the deterministic item ids, where `file-sec-<n>`/`file-maint-<n>` is already wrong today and becomes `file-<role-id>-<slug>`.

## 5. Non-Goals

1. **Role-declared file selection.** Letting a role narrow its own file set (a `file_focus` glob or keyword list in the role JSON) is the principled replacement for the deleted heuristic, but it is a schema addition and belongs in a minor release. This patch removes the hardcoded heuristic without replacing it; every non-`spec-coverage` role sees the same code files.
2. **A per-language default corpus.** Shipping the Go specifics as a `go` domain was considered and rejected for this patch. The reporter's suggested form — marking the current defaults `domains: ["go"]` so a non-Go project gets an empty auditor set — does not work against current semantics: `ResolveActiveCorpus` applies domain filtering only to the user corpus, the embedded corpus is the unfiltered fallback, and a filter that empties the user panel falls back to the same embedded panel with a warning. Making that produce an empty panel is a fallback-semantics change with its own risks, so §3 fixes the prompts instead of adding a domain.
3. **Migrating an already-ejected corpus.** A project that ran `tp init --eject-roles` owns its role files, and tp never rewrites project-owned data on upgrade — including the reporter's own configuration, which therefore receives §2 but not §3. Re-ejecting over local edits requires `--force` and is the user's deliberate act. §4's documentation states that the shipped defaults changed, which is the notice; detecting that an ejected file is byte-identical to a superseded default is a possible future convenience, not part of this patch.
4. **Renaming the `security` role id.** The id is a public key: it names ejected files and `tp.audit_roles` override keys. It stays.
5. **Per-spec role activation.** Disabling a role for one spec is v0.32.0. Until then the levers for declining a role's cost are the existing ones: eject the corpus and omit or rewrite the role file, or narrow a role's work with a `tp.audit_roles` frontmatter focus override.
6. **Cross-repository task execution.** A per-task working directory and quality gate are a separate release.
7. **Mitigating item-id churn across the upgrade.** A `security` role's `file_check` item ids derive from its selected file paths, so an audit sequence crossing this upgrade replays prior-round rows whose item ids no longer exist in the new round. The auditor re-checks the item either way, and the `legacy` marker covers only rounds recorded before stable ids, so this one-release transition is left unmarked rather than given a new marker.

## 6. Cost

The release makes a previously-silent role emit, and that is not free. In a repository where the keyword heuristic matched nothing — the case this release exists to fix — an audit round gains one role prompt and therefore one sub-agent invocation, up to `CodeFileCap` named code files that role reads itself, and one result row per file that the agent must produce, merge, and read every round even when all of them pass. That cost repeats for each required clean round. It is the price of the lens running at all, which is what the release buys. In a repository where the heuristic already matched, cost does not rise: the shared list caps at 10 where the security list capped at 20, and the per-role file inliner already inlined at most one role's files per round.

Two smaller deltas ride along. §2.3 adds up to 50 lines of project conventions to each non-`spec-coverage` prompt, replacing the resolve round-trips that a convention-blind auditor generated. §2.6 adds a short count to one header line and removes a false claim.

## 7. Tests

1. Given one affected file whose path and content match no security keyword, every auditor role in the active corpus emits a prompt and `skipped_roles` is empty — the reported defect, as a regression guard, and the inverted assertion of `TestGenerateAuditPrompts_EmptyRoleOmitted`.
2. A role file authored identically except for its id (`security` versus any other id) produces prompts that are equal after replacing the role id and the output path — not merely equal in item count — so a surviving id dispatch anywhere in prompt construction fails the test.
3. Renaming the affected file so its path contains a former security keyword changes nothing about which roles emit or how many items they receive.
4. A file whose path does not match a security keyword but whose first lines contain `validate` or `auth` receives exactly the same treatment as any other code file, covering the deleted content half of the heuristic.
5. `spec-coverage` still receives the table-row, list-item, task-acceptance, and finding items, and receives no `file_check` items; `routeChecklist` returns that one checklist.
6. Every non-`spec-coverage` role receives the same affected-files list for the same audit invocation, and `CodeFileCap` is asserted as the literal 10 while `spec-coverage` still uses `AuditFileCap` as the literal 20, so a silent cap change fails a test.
7. Every `file_check` item's `expected_evidence` instructs a `PASS` citing the inspected file when nothing in the role's domain appears in it.
8. Every non-`spec-coverage` role's prompt contains the `## Project Context` block when a `CLAUDE.md` excerpt exists, and `spec-coverage`'s prompt does not — the rewritten `TestGenerateAuditPrompts_CLAUDEmdOnlyForMaintainability`.
9. A role's affected-files header states the cap actually applied to its list, and states how many of how many files are shown when the pre-cap universe was larger.
10. An audit whose affected files are all dropped by the universe filter (a `testdata/` or `.golden` path) names every non-`spec-coverage` role in `skipped_roles` with reason `no-checklist-items` and leaves the exit code unchanged.
11. `spec-coverage` is still skipped with `no-checklist-items` on a spec with no structured elements, no tasks, and no findings — the existing skip test stays green unmodified.
12. A guard test walks `internal/engine/corpus/*/auditors/*.json` and asserts that no `focus` line and no `instructions` string contains, case-insensitively, any of `%w`, `0o600`, `0o644`, `camelCase`, `_ = err`, `goroutine`, `lowercase packages`, or `files written by the tool`. Reviewer roles are deliberately out of scope, since §3.1 establishes that they are already neutral.
13. A guard test asserts that `README.md`, `skills/tp/SKILL.md`, and `skills/tp/REFERENCE.md` each state the two-arm routing rule and that none of them still describes a special file list or item-id scheme for `security` or `maintainability-conventions`.
14. `tp init --eject-roles` emits the advisory substring on the info channel for both `--domain software` and `--domain prose`, `--quiet` suppresses it, the stdout JSON keys stay exactly `{ejected, domain}`, and the existing byte-identity test still passes.
