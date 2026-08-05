# tp v0.31.2 — De-magic the auditor routing

## 1. Overview

Field feedback from a Laravel/PHP repository reported that the `security` auditor never receives checklist items and is silently skipped with `no-checklist-items`, even on a spec whose sections are explicitly about authorization and input validation. The report was reproduced against both the built-in corpus and an ejected `--domain software` corpus.

The reporter's diagnosis — that audit checklist items derive only from the spec's structured elements, so a code-property lens can never earn an item — is not what the code does. `generateRoleAuditPrompts` dispatches on the role's **id string**, and its `default` branch already gives every other role one `file_check` item per affected file, which is exactly the code-property behavior the report asks for. Only two ids behave differently, and one of them is harmful:

- `spec-coverage` receives the spec-derived and finding items. This is intended and stays.
- `security` receives `AuditFileSelection.Security`, a list filtered by the hardcoded substrings `lock`, `validate`, `auth`, `secret`, `perm` matched against the file path or the first 200 lines of the file at HEAD. A PHP middleware matches none of them, so the list is empty, so the role is dropped.
- `maintainability-conventions` receives `AuditFileSelection.Maintainability` — which is byte-for-byte what the `default` branch already computes. This arm is dead weight.

A third id dispatch lives outside that switch: `buildRolePrompt` attaches the `## Project Context` excerpt of `CLAUDE.md` only when the role id is `maintainability-conventions`, so a conventions auditor authored under any other id judges conventions without ever seeing the project's conventions.

Two properties were confirmed empirically. Renaming the role id from `security` to any other value, with the role file otherwise unchanged, makes the same role emit a prompt against the same file. Renaming the *file* from `Dummy.php` to `AuthDummy.php`, with the role id left at `security`, also makes it emit. So the behavior depends on a role id string and an English keyword list, neither of which is part of the role contract tp documents.

This release removes the id-based dispatch and neutralizes the language-specific default auditor prompts. The keyword list is not part of what it removes: §2.4 keeps it, demoted from a per-id **filter** that could empty a role to a **ranking** signal applied identically to every role, and documents it as such. The condemned magic was the id dispatch and the list's power to drop a role, not the list itself. It is a patch: no new commands, no new flags, and no change to the audit result schema an auditor returns.

## 2. Auditor routing stops dispatching on role id

### 2.1 Only spec-coverage keeps a dedicated checklist source

`generateRoleAuditPrompts` reduces its switch to two arms. The `spec-coverage` id keeps its spec-derived checklist and its own file selection. Every other role — built-in, ejected, or user-authored — takes the shared arm: one `file_check` item per selected code file, with the item id, text, and expected evidence produced by `fileCheckItems` from that role's own id, exactly as custom roles already receive today. The `security` and `maintainability-conventions` arms are deleted, and the now-unread `roleSecurity` and `roleMaintainability` constants are deleted with them; `roleSpecCoverage` remains as the one reserved id.

The caller chain collapses with the switch. `routeChecklist` no longer produces per-role buckets: it takes the spec-derived and finding entries and returns the single `spec-coverage` checklist, dropping its `sel` argument and its `sec`/`maint` return values. `generateRoleAuditPrompts` drops the `secItems` and `maintItems` parameters, and `runAudit` stops destructuring three values. Every non-`spec-coverage` role's items are built inside the shared arm by calling `fileCheckItems` with the role's own id.

`spec-coverage` therefore stays a reserved id. The claim this release makes is narrower and is stated exactly: **no id other than `spec-coverage` changes any part of prompt construction.** §7 item 2 pins it with the fixture §2.9 defines, because the property is not observable by comparing two roles inside one audit run — see §2.9 for why.

### 2.2 A file outside a role's lens is a PASS

The shared arm hands every role one item per code file with no relevance filter, so a `security` role now receives an item for a file with no security surface. The audit status enum is `PASS`/`PARTIAL`/`FAIL`, and `tp audit --record` counts every non-`PASS` row against convergence, so an auditor answering "cannot evaluate, nothing here is in my domain" with `PARTIAL` would manufacture a false blocker on every round.

The expected disposition is therefore part of the emitted prompt rather than left to sub-agent judgment. It is rendered **once per prompt**, as a fixed paragraph immediately after the `## Checklist` block in every shared-arm prompt, not repeated into each item's `expected_evidence` — which stays the existing short `inspect file: <path>` so a ten-file checklist does not carry ten copies of the same sentence. The paragraph reads exactly:

```
## Disposition
A file containing nothing in this role's domain is a PASS, not a PARTIAL. Record it as
PASS with evidence_file set to that path and evidence_lines set to the full range you
read (for example "1-120"), meaning: the whole file was inspected and nothing in this
role's domain appears in it. Reserve PARTIAL and FAIL for a defect you actually found.
```

The block is scoped to shared-arm prompts only. `spec-coverage`'s prompt must **not** carry it: its checklist holds table rows, list items, task acceptance and findings, over which "a file containing nothing in this role's domain" is meaningless, and a `PASS` there already cites the spec element's implementing code. An implementation that writes the paragraph unconditionally in `buildRolePrompt` is wrong, and §7 item 8 asserts its absence there.

Naming the full inspected range keeps the existing output schema intact — `renderAuditOutputSchema` requires both `evidence_file` and `evidence_lines` for a `PASS`, and "I read all of it and found nothing" is exactly what a whole-file range asserts. No schema field becomes optional and no enum value is added.

### 2.3 Every non-spec-coverage role receives the project conventions

`buildRolePrompt` stops gating the `## Project Context` block on the `maintainability-conventions` id. The block is attached to every role taking the shared arm whenever the `CLAUDE.md` excerpt is non-empty, and `spec-coverage` continues not to receive it — it judges spec coverage against the `## Spec Excerpt` block, which no other role gets. The resulting rule is stateable in one sentence: `spec-coverage` gets the spec, every other role gets the code files and the project's conventions.

`claudeMDExcerptFor` yields the `## Conventions` section of `CLAUDE.md` when that heading exists and otherwise falls back to the file's first 50 lines, so in a project without that heading the block carries the file's opening 50 lines instead of a conventions section. That fallback is pre-existing and unchanged; §2 only widens who receives it. Either way the block is capped at `claudeMDExcerptLineCap` (50 lines), so the added cost per prompt is bounded.

The change buys the correctness the gate was destroying: a conventions or house-style auditor authored under any id other than `maintainability-conventions` previously reported findings against conventions it had never been shown, and each such finding costs a resolve round-trip and can hold the two-consecutive-clean audit gate shut.

### 2.4 The security keyword list becomes a ranking signal, not a filter

The keyword list is not deleted. Deleting it was the first draft of this release and it narrowed detection materially: `selectSecurity` caps at 20 **matches** while scanning the entire filtered universe, so today a keyword-matching file at any universe position reaches the security lens, and a plain first-10-files list would drop every matching file that does not sort into the first ten. The defect was never the keyword list — it was using the list as a **filter** that could empty a role, and using it for one hardcoded id.

`selectCodeFiles` therefore orders the filtered universe by putting path-keyword-matching files first, then the rest, each group alphabetical, and caps the result at `CodeFileCap`. The list can never empty a role, because a universe with no match still yields its first `CodeFileCap` files. Every role gets the same ordering, so no id is involved. The consequences:

1. `selectSecurity`, `AuditFileSelection.Security`, and the per-role security list are removed; `SelectAuditFiles` returns the spec-coverage list and the shared code-file list.
2. `securitySubstrings` survives as the ranking input and is renamed `priorityPathSubstrings` to state its new job, keeping the same five values `lock`, `validate`, `auth`, `secret`, `perm`. Its predicate `containsSecuritySubstring` is renamed `matchesPriorityPath` so the list and the function that reads it stop naming a security filter that no longer exists.
3. The content half of the heuristic is dropped: ranking matches against the path only. `AuditFileInputs.HeadReader` is read only by `selectSecurity` and assigned only in `runAudit`, and `GitHeadReader` exists only to produce it, so the field, its assignment, and the function are all removed. So are `securityHeadLineCap` and the `headLines` helper, whose only reader is `selectSecurity`'s content branch — `.golangci.yml` enables the `unused` linter, so an unreferenced unexported constant or function fails `golangci-lint run`, which is the quality gate. `AuditFileInputs` keeps `Universe`, `DiffStats`, `Deleted`, and `TaskFiles`. A `git show HEAD:<path>` subprocess per candidate file is not worth paying for a ranking signal, and it is unavailable for a file absent at HEAD — which is every newly added file, the most common audit subject.

The residual narrowing is stated plainly: a keyword-matching file beyond the first `CodeFileCap` of the ranked order no longer reaches any lens, where the old security list held up to `AuditFileCap` matches. The lever is `--affected-files` to scope the universe deliberately, and the role-declared file selection of §5 item 1 when it lands. The cap is not raised here because doing so would double the file list of every emitted role at once.

### 2.5 The shared selection is named for what it is

`AuditFileSelection.Maintainability` now serves every non-`spec-coverage` role, so a field named after one role is the same misleading-name trap that produced this defect. The field is renamed to `CodeFiles`, `selectMaintainability` to `selectCodeFiles`, and `MaintainabilityFileCap` to `CodeFileCap`. The cap value stays 10 and `AuditFileCap` stays 20 for `spec-coverage`.

`AuditFileSelection` additionally gains two integer fields, `SpecCoverageTotal` and `CodeFilesTotal`, carrying each list's pre-cap size so §2.6 can render it. `CodeFilesTotal` is the size of the filtered universe — after the binary, fixture, and deleted drop rules, since a dropped file was never a candidate. `SpecCoverageTotal` is the size of the pool that branch actually capped: the task-mapped set in the normal branch, and the filtered universe in the no-task fallback branch.

### 2.6 The affected-files header states the real cap

`buildRolePrompt` writes the fixed header `## Affected Files (max 20)`. After §2.4 the only role still bounded by 20 is `spec-coverage`; every other role is bounded by `CodeFileCap`, so the literal would overstate the cap in most prompts, and an auditor reading "max 20" above a ten-item list reasonably concludes nothing was truncated. `buildRolePrompt` takes the applied cap and the pre-cap total from §2.5 and renders exactly one of two forms:

- `## Affected Files (max 10)` when the pre-cap total is less than or equal to the list length — nothing was dropped.
- `## Affected Files (10 of 34)` when it is greater — the list length and the pre-cap total. The cap is not repeated here: this form renders only once the cap has bitten, at which point the list length *is* the cap, so a trailing `max 10` would restate the leading `10` and feed no decision the auditor makes.

The numbers are the role's own cap and total, so a `spec-coverage` prompt renders `max 20`. Rendering the count is what keeps the cap from being a silent truncation.

### 2.7 The skip outcomes that survive

`no-checklist-items` remains reachable for two distinct reasons, and both are reported the same way — the role is named in `skipped_roles` with that reason, and `tp audit` still exits 0:

1. `spec-coverage` is skipped when the spec has no table rows, no numbered list items, no task acceptance entries, and no findings, so no spec-derived item exists.
2. Every non-`spec-coverage` role is skipped when the shared code-file list is empty — the universe was empty, or `filterAuditUniverse` dropped every file as binary, as a `testdata/**` or `*.golden` fixture, or as deleted. A diff touching only images or fixtures therefore still skips every code-file auditor.

What can no longer happen is a role being skipped while other roles receive items, because a keyword list did not match a filename.

### 2.8 Existing tests this change invalidates

The inventory below is exhaustive against a search for every committed reference to this literal set, which `scripts/check-test-inventory.py` reads from the fenced block so the spec and the check can never disagree about what was searched:

```search-literals
sel.Security
selectSecurity
HeadReader
securityHeadLineCap
headLines
containsSecuritySubstring
sel.Maintainability
selectMaintainability
MaintainabilityFileCap
roleSecurity
roleMaintainability
## Project Context
## Affected Files (max 20)
```

The bare role ids `security` and `maintainability-conventions` are deliberately absent from that set. They appear in 37 committed tests as ordinary fixture role names, and after this release a fixture role name carries no behavior, so referencing one is not evidence of impact; searching them would drown the inventory in 27 unaffected tests. Each was checked by hand: `TestInitEjectRoles_Output` asserts only the eject JSON keys, which §3.4 leaves untouched, and `TestAuditSchema_PromptFieldsAndOutputSchema` asserts block presence rather than block order, so §2.2's inserted `## Disposition` leaves it green. The list itself is the contract — it carries no count, because a count is one more thing that can drift out of step with it:

1. `TestGenerateAuditPrompts_EmptyRoleOmitted` (`internal/cli/audit_routing_test.go`) asserts `security` is absent from the prompts when the only affected file is `plain.go`. Its assertion inverts: `security` is emitted with one `file_check` item for `plain.go`. This becomes the regression guard of §7 item 1.
2. `TestRouteChecklist_Disjoint` (`internal/cli/audit_routing_test.go`) asserts `security` receives exactly the one auth-matching file. It is a black-box test in package `cli_test` that shells the binary and parses the emitted JSON, so it cannot assert on `routeChecklist`'s return value; it is rewritten at the same CLI level to assert that spec-derived items appear only in the `spec-coverage` prompt and that every other role's items are `file_check` items over the shared list.
3. `TestFilterFiles_Security` and the security half of `TestFilterFiles_Cap20` (`internal/engine/auditfiles_test.go`) test the deleted per-role list and are deleted. `TestFilterFiles_Security`'s fixture is reused for a new ranking test (§7 item 4).
4. `TestFilterFiles_DropsBinaryFixturesDeleted` (`internal/engine/auditfiles_test.go`) changes twice: its `assert.Empty(t, sel.Security)` line references a removed field and is deleted, and its `require.Len(t, sel.Maintainability, 1)` / `assert.Equal(t, "keep.go", sel.Maintainability[0].Path)` lines are renamed to `CodeFiles` like every other surviving reference. Nothing else in it moves.
5. `TestFilterFiles_Maintainability` and `TestFilterFiles_DropFirstBackfill` (`internal/engine/auditfiles_test.go`) reference `sel.Maintainability` and `MaintainabilityFileCap` and are renamed mechanically to `CodeFiles` and `CodeFileCap`. Their fixtures contain no priority keyword, so the §2.4 ranking leaves their alphabetical expectations valid.
6. `TestAuditPrompts_BodyOrderAndEmbedding` (`internal/cli/audit_prompts_test.go`) breaks once, on `NotContains(sec, "## Project Context")` — precisely the id gate §2.3 removes. That negative assertion for `security` is deleted and the one for `spec-coverage` stands. Its header lookup at the literal `## Affected Files (max 20)` does **not** break: the fixture supplies one affected file to a `spec-coverage` prompt, so the pre-cap total equals the list length and §2.6 renders that exact string. The lookup is nonetheless changed to a prefix match on `## Affected Files (` as hardening, so a future fixture that triggers the truncated form does not silently fail an ordering assertion.
7. `TestAudit_FileFilterCap` (`internal/cli/integration_test.go`) supplies 50 path-matching files and bounds the `security` list with `assert.LessOrEqual(..., 20)`. That assertion still passes once the bound falls to `CodeFileCap`, so the test does not break — it stops testing anything. It is listed here because a silently-loosened assertion is worse than a failing one: the bound is tightened to an exact `10`, making it the only committed end-to-end exercise of the cap.
8. `TestGenerateAuditPrompts_CLAUDEmdOnlyForMaintainability` (`internal/cli/audit_named_test.go`) pins the id gate §2.3 removes. It is rewritten — and renamed to drop `OnlyForMaintainability` — to assert that every non-`spec-coverage` role's prompt contains the conventions marker and that `spec-coverage`'s does not.

### 2.9 Why prompt equality is measured across invocations

`generateRoleAuditPrompts` carries a per-role file inliner: a single `inlinerDone` latch gives the first emitted role whose file set fits the reading budget the whole file contents plus a `filesComplete` framing, and every later role named paths only. Two identically-authored roles present in the same corpus therefore differ by an inlined-content block, because of emission position rather than id. Separately, `fileCheckItems` builds each item id as `file-<role-id>-<slug>` where the slug derives from text containing the role id and is truncated to 40 characters, so two ids of different length truncate at different points and no substitution maps one prompt onto the other.

§2 changes neither mechanism, so the equality claim of §2.1 is pinned against a fixture that holds both constant. The fixture is stated in full because each clause is load-bearing:

1. **Two separate audit invocations**, one per role id, rather than two roles in one corpus — otherwise only the first-emitted role is the inliner.
2. **The corpus for each invocation holds exactly one role, the one under test.** `spec-coverage` is not in it. Leaving `spec-coverage` present would not do: emission order puts it first, so it would take the `inlinerDone` latch and neither role under test would be the inliner.
3. **The two ids have equal length and differ only in their final character** (for example `security` and `securitz`). Equal length alone is not enough: the slug is `slugifySubject(path + " " + "Apply the <roleID> role rules to <path>")` truncated at 40 characters, and when the cut lands inside the id occurrence only a prefix of the id survives, which a whole-token substitution cannot map. Ids differing only in the last character make every truncated prefix identical, so the comparison holds for any path length.

Under that fixture the two prompts must be byte-identical after substituting the role id token and rewriting the output path (`audit-r<N>-<role-id>.ndjson`), which the equal-length ids keep the same length. Any surviving id branch anywhere in prompt construction fails the comparison.

### 2.10 The inventory is checked mechanically

`test-inventory-drift` was raised in three consecutive review rounds of this spec, and `tp review --record` named it a mechanize candidate. A fourth prose correction would have produced a fourth drift, so the comparison is mechanized instead: `scripts/check-test-inventory.py <spec> "<section heading>"` reads the `search-literals` block of §2.8, finds every committed Go test function referencing any of those literals, and exits 1 naming any test the inventory does not mention. It is registered as a `test-inventory-drift` mechanical check on this spec's workflow, so every later round runs it.

Writing it paid for itself twice before it ever guarded a round. It proved the original literal set was over-broad — the bare role ids `security` and `maintainability-conventions` matched 37 tests, 27 of them unaffected — which is why §2.8 now searches symbols and prompt literals only. And its own first version mis-parsed the spec, ending the section early because the literal set contains lines beginning with `##`; the fix is to track fenced blocks when scanning for a section boundary. Both are recorded here because a check that is trusted without being tested is worth less than no check.

The check is a review-loop tool, not a shipped feature: it verifies a claim this spec makes about the repository, and any future spec that enumerates affected tests can reuse it by adding a `search-literals` block.

## 3. The default auditor prompts become language-neutral

### 3.1 The problem the defaults create

The embedded `software` auditor prompts are written in Go, and more narrowly in tp's own Go: `errors wrapped with %w`, `files written with 0o600`, `lowercase packages, camelCase symbols`, `_ = err` as a finding, `Files written by the tool`, and `idiomatic` as the maintainability standard. In a PHP, TypeScript, or Ruby project these match nothing or produce noise, and — before §2 — a user could not tell, because the auditor was silently skipped. The three reviewer defaults (`implementer`, `tester`, `architect`) and the `spec-coverage` auditor are already language-neutral and are not touched.

### 3.2 security.json states invariants, not Go idioms

The `security` role keeps its id — ejected corpora and `tp.audit_roles` frontmatter overrides key on it — and its focus is rewritten so each line names an invariant that holds in any language:

1. Every acquired lock, transaction, or handle is released on all paths, including error and early-return paths.
2. No swallowed failure: an ignored error return, an empty catch, or an error discarded instead of propagated with context is a finding.
3. Values from user input, request parameters, or the environment are validated before they reach a query, a file path, or a command.
4. Queries, paths, and commands are built with the platform's parameterizing or joining API, never by string concatenation from input.
5. Authorization is checked at the point of the write, not only at the entry point, and never from a value the caller supplied.
6. Files and directories the tool creates use restrictive permissions, and secrets never reach logs, error messages, or committed files.

Its `instructions` field is already language-neutral and is unchanged.

### 3.3 maintainability-conventions.json states properties, not Go syntax

The `maintainability-conventions` focus is rewritten the same way:

1. Errors are propagated with context naming the failing operation, not rethrown bare and not flattened into a lossy string.
2. Publicly reachable symbols carry a doc comment explaining intent rather than restating the signature.
3. Functions stay short enough to read in one pass (roughly 80 lines); a longer one needs a stated reason.
4. Naming follows the conventions already present in the surrounding code rather than introducing a competing style.
5. No leftover TODO or FIXME without a ticket reference, no commented-out code, and no debug output left behind.

Its `instructions` field currently ends "only whether the code is idiomatic and maintainable", which names an absent universal standard. That clause becomes "only whether the code is maintainable and consistent with the conventions already present in this project", so the standard is the project's own conventions — which §2.3 now actually puts in front of the role.

### 3.4 Ejection says the defaults are a starting point

`tp init --eject-roles` emits this exact line:

```
note: these roles are starting points; rewrite their focus for your project's stack and conventions.
```

`output.Info` and `output.Success` both return early when `jsonMode` is set, and `jsonMode` is enabled whenever stdout is not a terminal, so either would make the line invisible in every piped and every agent-driven run — the exact audience that adopts ejected roles unread. The advisory therefore goes through a new `output.Notice(msg)` primitive: it writes to stderr, it is suppressed by `--quiet`, and it is **not** suppressed by JSON mode. `Notice` is documented as the channel for a one-time advisory about project-owned data a command just wrote; eject is its only caller in this release. Nothing is added to the eject JSON payload, whose keys stay exactly `{ejected, domain}`, and the written files stay byte-identical to the embedded corpus as v0.25.0 §5.4 requires. The wording names no language, so the same line is emitted for every domain including `prose`.

### 3.5 The corpus hash deliberately stays `builtin`

`ComputeRolesHash` returns the `builtin` sentinel for a phase with no user role files, so rewriting the embedded prompts does not flip `roles_stale` and an in-flight audit sequence is not marked as having run under different instructions. This matches the v0.25.0 §5.2 precedent, under which content shipping with a tp release is excluded from the corpus hash because a tp upgrade is already a visible event. It is recorded here as a decision rather than left to read as an oversight; a project that wants its role content hashed can eject the corpus, at which point the files are hashed like any other user corpus.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and the role-corpus rule in `CLAUDE.md` each state part of the audit routing contract and must all tell one story. Every one of the four must state the routing rule in a form containing the substring `spec-coverage is the only auditor id that changes routing`, and must not contain the substrings `file-sec-`, `file-maint-`, or any claim that `security` or `maintainability-conventions` receives its own file list or its own prompt section. The wording deliberately says *changes routing* rather than *is reserved*: `regression` is already a reserved id in both phases — the shared role parser rejects it as a user role file id (`REFERENCE.md` §Role Corpus) — so a claim that `spec-coverage` is the only reserved id would contradict an invariant this release does not touch. `REFERENCE.md` additionally documents the deterministic item ids, where `file-sec-<n>`/`file-maint-<n>` is already wrong today and becomes `file-<role-id>-<slug>`.

All four must also record that the embedded default auditor prompts changed in this release and that an already-ejected corpus keeps its old copies — the notice §5 item 3 relies on. That requirement is pinned the same way as the first: each of the four must contain the substring `ejected role files are not rewritten on upgrade`.

Two of them additionally document the audit prompt's body order verbatim — `skills/tp/REFERENCE.md` and `skills/tp/SKILL.md` both render it as `Role → Role Rules → Spec Excerpt → Project Context → JSON-array Checklist → Affected Files → Output Schema`. §2.2 inserts a `## Disposition` block into that order for shared-arm prompts and §2.3 changes who receives `## Project Context`, so both lines are wrong after this release and must be rewritten to show the shared-arm order (`Role → Role Rules → Project Context → JSON-array Checklist → Disposition → Affected Files → Output Schema`) alongside the `spec-coverage` order (`Role → Role Rules → Spec Excerpt → JSON-array Checklist → Affected Files → Output Schema`). The same two documents state `prompts[].role` as an enum of the three built-in ids, which has been wrong since user-defined roles shipped in v0.25.0; they must state instead that the value is any active role id from the corpus.

## 5. Non-Goals

1. **Role-declared file selection.** Letting a role narrow or prioritize its own file set (a `file_focus` glob or keyword list in the role JSON) is the principled generalization of §2.4's shared ranking, but it is a schema addition and belongs in a minor release. This patch keeps one fixed ranking for every role.
2. **A per-language default corpus.** Shipping the Go specifics as a `go` domain was considered and rejected for this patch. The reporter's suggested form — marking the current defaults `domains: ["go"]` so a non-Go project gets an empty auditor set — does not work against current semantics: `ResolveActiveCorpus` applies domain filtering only to the user corpus, the embedded corpus is the unfiltered fallback, and a filter that empties the user panel falls back to the same embedded panel with a warning. Making that produce an empty panel is a fallback-semantics change with its own risks, so §3 fixes the prompts instead of adding a domain.
3. **Migrating an already-ejected corpus.** A project that ran `tp init --eject-roles` owns its role files, and tp never rewrites project-owned data on upgrade — including the reporter's own configuration, which therefore receives §2 but not §3. Re-ejecting over local edits requires `--force` and is the user's deliberate act. The §4 documentation notice is how a user learns the shipped defaults moved; detecting that an ejected file is byte-identical to a superseded default is a possible future convenience, not part of this patch.
4. **Renaming the `security` role id.** The id is a public key: it names ejected files and `tp.audit_roles` override keys. It stays.
5. **Per-spec role activation.** Disabling a role for one spec is v0.32.0. Until then the levers for declining a role's cost are the existing ones: eject the corpus and omit or rewrite the role file, or narrow a role's work with a `tp.audit_roles` frontmatter focus override.
6. **Cross-repository task execution.** A per-task working directory and quality gate are a separate release.
7. **Mitigating item-id churn across the upgrade.** A `security` role's `file_check` item ids derive from its selected file paths, so an audit sequence crossing this upgrade replays prior-round rows whose item ids no longer exist in the new round. The auditor re-checks the item either way, and the `legacy` marker covers only rounds recorded before stable ids, so this one-release transition is left unmarked rather than given a new marker.

## 6. Cost

The release makes a previously-silent role emit, and that is not free. In a repository where the keyword list matched nothing — the case this release exists to fix — an audit round gains one role prompt and therefore one sub-agent invocation, up to `CodeFileCap` named code files that role reads itself, and one result row per file that the agent must produce, merge, and read every round even when all of them pass. That cost repeats for each required clean round. It is the price of the lens running at all, which is what the release buys.

For a repository whose keyword list already matched, the deltas are **per role**, not uniform, and the earlier draft's "per shared-arm prompt" framing generalized one role's numbers across all of them — the same over-reach in the opposite direction as the "roughly unchanged" claim it replaced. Stated correctly:

| Role | §2.3 conventions | §2.2 Disposition | §2.6 header | Affected-file lines | Net |
|---|---|---|---|---|---|
| `security` | +up to 50 | +~5 | +a few chars | −10 (cap 20 → 10) | +~45 |
| `maintainability-conventions` | 0 (already had it) | +~5 | +a few chars | 0 (already capped at 10) | +~5 |
| any custom role | +up to 50 | +~5 | +a few chars | 0 (already capped at 10) | +~55 |
| `spec-coverage` | 0 (never gets it) | 0 (excluded) | +a few chars | 0 (cap stays 20) | ~0 |

Only `security` pays the −10, because §1 establishes that every other role already ran on the `default` branch's list at `MaintainabilityFileCap`. Only `maintainability-conventions` skips the conventions block's cost, because it alone already received it. What the growth buys is a role that can no longer report against conventions it was never shown (§2.3) and can no longer manufacture a PARTIAL on an out-of-lens file (§2.2), each of which cost a resolve round-trip measured in whole rounds rather than lines. The honest summary: this release trades a narrower file list and a larger fixed prompt preamble for a wider role panel, and claims to be free nowhere.

## 7. Tests

1. Given a spec with structured elements (so `spec-coverage` earns a checklist) and one affected code file whose path matches no keyword, every auditor role in the active corpus emits a prompt and `skipped_roles` is empty — the reported defect, as a regression guard, and the inverted assertion of `TestGenerateAuditPrompts_EmptyRoleOmitted`.
2. The §2.9 fixture: two audit invocations, each against a single-role corpus (no `spec-coverage`), with ids of equal length differing only in their final character, produce byte-identical prompts after substituting the role id token and the output path. It fails if any id branch survives anywhere in prompt construction.
3. Renaming the affected file so its path contains a keyword changes which roles emit not at all, and changes how many items each receives not at all.
4. `selectCodeFiles` ranks path-keyword-matching files ahead of the rest, each group alphabetical: given a universe where a matching file sorts last, it appears first in the returned list, and given a universe with no match the list is the first `CodeFileCap` files alphabetically — the list is never empty for a non-empty universe.
5. A file whose path does not match a keyword but whose *content* contains `validate` or `auth` is ranked as an ordinary file. §2.4 item 3 removes `HeadReader`, so after the change no engine-level channel supplies file content at all and this assertion cannot sit beside item 4's `selectCodeFiles` test: it is written at the CLI level, where the file exists on disk, asserting that such a file lands in alphabetical position rather than promoted. The absence of a content channel is itself what is pinned.
6. `spec-coverage` still receives the table-row, list-item, task-acceptance, and finding items and no `file_check` items; `routeChecklist` returns that one checklist.
7. Every non-`spec-coverage` role receives the same affected-files list for the same audit invocation, and `CodeFileCap` is asserted as the literal 10 while `AuditFileCap` is asserted as the literal 20, so a silent cap change fails a test.
8. The `## Disposition` paragraph appears exactly once in every shared-arm prompt, does not appear in the `spec-coverage` prompt at all, and its index falls between the index of `## Checklist` and the index of `## Affected Files (` — pinning §2.2's placement, its scope, and its once-per-prompt rendering. No `expected_evidence` value contains the word `PASS`, pinning that the rule is not repeated per item.
9. Every non-`spec-coverage` role's prompt contains the `## Project Context` block when a `CLAUDE.md` excerpt exists, and `spec-coverage`'s prompt does not.
10. A role whose pre-cap total equals its list length renders `## Affected Files (max 10)`; a role whose pre-cap total is larger renders `## Affected Files (10 of 34)` with its own list length and total; a `spec-coverage` prompt with nothing truncated renders `## Affected Files (max 20)`.
11. An audit whose affected files are all dropped by the universe filter (a `testdata/` or `.golden` path) names every non-`spec-coverage` role in `skipped_roles` with reason `no-checklist-items` and exits 0.
12. `spec-coverage` is still skipped with `no-checklist-items` on a spec with no structured elements, no tasks, and no findings — the existing skip test stays green unmodified.
13. A guard test walks `internal/engine/corpus/*/auditors/*.json` and asserts that no `focus` line and no `instructions` string contains, case-insensitively, any of `%w`, `0o600`, `0o644`, `camelCase`, `_ = err`, `goroutine`, `lowercase packages`, `files written by the tool`, or `idiomatic`. Reviewer roles are deliberately out of scope, since §3.1 establishes that they are already neutral.
14. A guard test asserts that `README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md`, and `CLAUDE.md` each contain the substrings `spec-coverage is the only auditor id that changes routing` and `ejected role files are not rewritten on upgrade`, and that none contains `file-sec-` or `file-maint-`. It further asserts that `skills/tp/REFERENCE.md` and `skills/tp/SKILL.md` each contain the two §4 prompt-body orders verbatim and no longer state `prompts[].role` as an enum of the three built-in ids, so every §4 requirement is guarded rather than only the first two.
15. `tp init --eject-roles` emits the §3.4 line verbatim on stderr for `--domain software` and `--domain prose`, emits it with stdout piped (the JSON-mode case `output.Info` would swallow), does not emit it under `--quiet`, and leaves the stdout JSON keys exactly `{ejected, domain}`; the existing byte-identity test still passes.
16. `priorityPathSubstrings` is asserted to equal exactly `["lock", "validate", "auth", "secret", "perm"]`, mirroring the cap-literal guard of item 7: the rename from a filter to a ranking signal is the moment a value is most likely to be quietly dropped or added, and items 4 and 5 exercise only one keyword each.
17. `SpecCoverageTotal` is pinned on both of its branches: on a spec whose task file maps fewer files than the diff touches and whose task-mapped set exceeds `AuditFileCap`, the `spec-coverage` header renders `## Affected Files (20 of <task-mapped size>)`; with no task file, the total equals the filtered-universe size. Without both cases an implementation that always uses the filtered universe passes.
