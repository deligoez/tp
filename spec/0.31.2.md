# tp v0.31.2 — De-magic the auditor routing

## 1. Overview

Field feedback from a Laravel/PHP repository reported that the `security` auditor never receives checklist items and is silently skipped with `no-checklist-items`, even on a spec whose sections are explicitly about authorization and input validation. The report was reproduced against both the built-in corpus and an ejected `--domain software` corpus.

The reporter's diagnosis — that audit checklist items derive only from the spec's structured elements, so a code-property lens can never earn an item — is not what the code does. `generateRoleAuditPrompts` dispatches on the role's **id string**, and its `default` branch already gives every other role one `file_check` item per affected file, which is exactly the code-property behavior the report asks for. Only two ids behave differently, and one of them is harmful:

- `spec-coverage` receives the spec-derived and finding items. This is intended and stays.
- `security` receives `AuditFileSelection.Security`, a list filtered by the hardcoded substrings `lock`, `validate`, `auth`, `secret`, `perm` matched against the file path or the first 200 lines of the file at HEAD. A PHP middleware matches none of them, so the list is empty, so the role is dropped.
- `maintainability-conventions` receives `AuditFileSelection.Maintainability` — which is byte-for-byte what the `default` branch already computes. This arm is dead weight.

Two properties were confirmed empirically. Renaming the role id from `security` to any other value, with the role file otherwise unchanged, makes the same role emit a prompt against the same file. Renaming the *file* from `Dummy.php` to `AuthDummy.php`, with the role id left at `security`, also makes it emit. So the behavior depends on a role id string and an English keyword list, neither of which is part of the role contract tp documents.

This release removes the id-based dispatch and neutralizes the language-specific default auditor prompts. It is a patch: no new commands, no new flags, no schema change.

## 2. Auditor routing stops dispatching on role id

### 2.1 Only spec-coverage keeps a dedicated checklist source

`generateRoleAuditPrompts` reduces its switch to two arms. The `spec-coverage` id keeps its spec-derived checklist and its own file selection. Every other role — built-in, ejected, or user-authored — takes the shared arm: one `file_check` item per selected code file, with the item id, text, and expected evidence produced by `fileCheckItems` from that role's own id, exactly as custom roles already receive today. The `security` and `maintainability-conventions` arms are deleted; neither id carries any behavior the shared arm does not.

The observable consequence is the fix: a role named `security` and an identically-authored role named anything else now produce identical checklists for identical inputs.

### 2.2 The security file heuristic and its inputs are removed

The keyword-filtered selection exists only to serve the deleted arm, so it goes with it:

1. `selectSecurity`, the `securitySubstrings` list, `securityHeadLineCap`, and the `headLines` helper are removed from `internal/engine/auditfiles.go`.
2. `AuditFileSelection.Security` is removed; `SelectAuditFiles` returns the spec-coverage list and the shared code-file list only.
3. `AuditFileInputs.HeadReader` and `GitHeadReader` are removed when no other caller reads them; a caller that still needs file content at HEAD keeps them, and the spec-coverage selection is unaffected either way.

Removing the heuristic does not widen the token budget: the shared list is capped at 10 files while the security list was capped at 20, and the per-role file inliner already inlines at most one role's files per audit round.

### 2.3 The shared selection is named for what it is

`AuditFileSelection.Maintainability` now serves every non-`spec-coverage` role, so a field named after one role is the same misleading-name trap that produced this defect. The field is renamed to `CodeFiles`, `selectMaintainability` to `selectCodeFiles`, and `MaintainabilityFileCap` to `CodeFileCap`. The cap value, the selection rule (first N universe files after the drop rules, alphabetical), and every emitted JSON field are unchanged — this is an internal rename with no output difference.

### 2.4 An empty spec-coverage checklist still skips the role

`no-checklist-items` remains a real outcome for `spec-coverage`: a spec with no table rows, no numbered list items, no task acceptance, and no findings produces no spec-derived items, and the role is skipped and named in `skipped_roles` as before. What can no longer happen is a role being skipped because a keyword list did not match a filename.

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

The `maintainability-conventions` focus is rewritten the same way:

1. Errors are propagated with context naming the failing operation, not rethrown bare and not flattened into a lossy string.
2. Publicly reachable symbols carry a doc comment explaining intent rather than restating the signature.
3. Functions stay short enough to read in one pass (roughly 80 lines); a longer one needs a stated reason.
4. Naming follows the conventions already present in the surrounding code rather than introducing a competing style.
5. No leftover TODO or FIXME without a ticket reference, no commented-out code, and no debug output left behind.

### 3.4 Ejection says the defaults are a starting point

`tp init --eject-roles` adds one advisory line to its output stating that the written roles are language-neutral starting points and that a project should rewrite their focus for its own stack and conventions. The line is informational output only; the written files stay byte-identical to the embedded corpus, which §5.4 of the v0.25.0 spec requires and the existing byte-identity test enforces.

## 4. Documentation

`README.md` and `skills/tp/SKILL.md` describe role routing wherever they state that auditors receive checklists. They must say that `spec-coverage` receives the spec-derived checklist and every other auditor receives one item per affected code file, with no id treated specially, so a reader can predict what a newly authored role will get. `CLAUDE.md`'s role-corpus rule notes the same, and drops any claim that a role id selects a behavior.

## 5. Non-Goals

1. **Role-declared file selection.** Letting a role narrow its own file set (a `file_focus` glob or keyword list in the role JSON) is the principled replacement for the deleted heuristic, but it is a schema addition and belongs in a minor release. This patch removes the hardcoded heuristic without replacing it; every non-`spec-coverage` role sees the same code files.
2. **A per-language default corpus.** Shipping the Go specifics as a `go` domain was considered and rejected for this patch. The reporter's suggested form — marking the current defaults `domains: ["go"]` so a non-Go project gets an empty auditor set — does not work against current semantics: `ResolveActiveCorpus` applies domain filtering only to the user corpus, the embedded corpus is the unfiltered fallback, and a filter that empties the user panel falls back to the same embedded panel with a warning. Making that produce an empty panel is a fallback-semantics change with its own risks, so §3 fixes the prompts instead of adding a domain.
3. **Renaming the `security` role id.** The id is a public key: it names ejected files and `tp.audit_roles` override keys. It stays.
4. **Per-spec role activation.** Disabling a role for one spec is v0.32.0.
5. **Cross-repository task execution.** A per-task working directory and quality gate are a separate release.

## 6. Tests

1. Given one affected file whose path and content match no security keyword, every auditor role in the active corpus emits a prompt and `skipped_roles` is empty — the reported defect, as a regression guard.
2. A role file authored identically except for its id (`security` versus any other id) produces the same checklist item count and the same affected-files list; the assertion fails if id-based dispatch returns.
3. Renaming the affected file so its path contains a former security keyword changes nothing about which roles emit or how many items they receive.
4. `spec-coverage` still receives the table-row, list-item, task-acceptance, and finding items, and receives no `file_check` items.
5. `spec-coverage` is still skipped with `no-checklist-items` on a spec with no structured elements, no tasks, and no findings — the existing skip test stays green unmodified.
6. Every non-`spec-coverage` role receives the same affected-files list, capped at the code-file cap, for the same audit invocation.
7. A guard test asserts that no embedded default auditor focus line contains a language-specific token from a small deny-list (`%w`, `0o600`, `camelCase`, `_ = err`, `packages`), so a Go idiom reintroduced into a default prompt fails a test rather than shipping.
8. The existing eject byte-identity test still passes: the advisory line of §3.4 is output, not file content.
