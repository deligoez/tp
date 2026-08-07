# tp v0.33.0 — Honest convergence signals

## 1. Overview

tp's audit loop counts every non-PASS row against convergence, whether the row concerns the spec or the codebase at large. The general lenses in a real corpus always find something, so under that predicate there is no fixed point.

v0.32.0's own audit is the evidence. `spec-coverage` — the only auditor role that takes the spec-derived checklist, and therefore the only one measuring conformance — was 55/55 clean from round 2 through round 11, and no round in the entire audit produced a single FAIL. Rounds 3–11 were consumed by hint wording, advisory channels, git scoping and flag hygiene that `spec/0.32.0.md` never mentions. The divergence was real from round 3; it was noticed by hand at round 10, and only because a human asked why the loop was taking so long.

The failure is not that tp lacked a policy. It is that tp never said what was happening. A driver reading `tp audit --status` saw `converged: false` and a finding count, and nothing that distinguished "the implementation does not match the spec" from "the implementation matches the spec and three general lenses are reading the rest of the repository". Both states print the same shape.

This release ships the signal and nothing else. Two items:

1. **The divergence signal.** `tp audit --status` and `--record` report each auditor role's consecutive clean-round count, name `spec-coverage`'s separately, and say so explicitly when `spec-coverage` has been clean for the required number of rounds while other roles still hold open findings.
2. **A registered check retires its own suggestion.** Registering a `checks` entry for a recurring class should stop tp from recommending that the check be written. It currently keeps recommending one that already exists.

**The convergence gate does not change.** No row scope, no `audit_converge_on`, no change to which rounds count as clean, no change to an exit code. §5 records why that separation is deliberate rather than incremental: a classification produced by a sub-agent, wired into the release gate, lets a genuine spec violation mislabelled `codebase` pass the gate — a worse failure mode than today's rule, which at least over-counts. Ship the signal, use it for a version, then decide whether the policy is needed.

Every rule below that could fail in two directions fails toward "keep auditing". A state tp cannot measure never produces the signal that tells a driver the decision is theirs.

## 2. The audit divergence signal

### 2.1 Reading a recorded round's rows

A recorded audit round's rows are loaded with the same loader the review side already uses. Two definitions govern everything in §2, and neither may be restated differently anywhere else:

- **A row's role** is its `role` field when that field is a non-empty JSON string. An absent key, an empty string, and a non-string value all mean the row **carries no role**. No `role_streaks` entry is ever created for an empty role id.
- **A round contributes no rows** when `engine.LoadRoundRows` reports the round's recorded NDJSON file as not found — the file is absent or cannot be read. A file that reads but holds malformed lines does **not** take this path: that loader drops the malformed lines and returns the rest, and the surviving valid rows decide the round. This is the only meaning of "unreadable" in this spec.

A round that contributes no rows is disclosed rather than silently absorbed. Each such round emits one advisory through `output.Notice`, in the wording the review record path already uses for the same condition: `round <N> file <name> is missing; skipping its rows`. This applies on both outputs of §2.4.

### 2.2 Per-role clean streaks

A role is **clean in a round** when it has at least one row in that round and every one of its rows has `status` exactly `"PASS"`. A role with no rows in a round is not clean in it: the role was not measured, so cleanliness cannot be claimed for it.

A role's `consecutive_clean` counts trailing recorded audit rounds in which the role is clean, stopping at the first trailing round in which it is not. A round that contributes no rows therefore ends every role's streak, as does a round recorded before the role corpus existed, whose rows carry no role at all. Both are the conservative direction: they reset the signal toward "keep auditing" rather than toward "you may ship".

`tp audit --status` and `tp audit --record` carry a `role_streaks` array. Its entries are the roles appearing in the **latest** recorded audit round — the panel the current decision rests on, not every role ever seen. Each entry is:

| Field | Meaning |
|---|---|
| `role` | the role id |
| `consecutive_clean` | trailing rounds in which the role is clean, as defined above |
| `open` | that role's non-PASS row count in the latest recorded round |

Order is `spec-coverage` first when it is present, then the remaining ids ascending.

`role_streaks` is `[]` whenever no role appears in the latest recorded audit round. Three states reach it and they are not distinguished, because the array answers one question and the answer is the same in all three: no recorded round exists at all; the latest round contributes no rows (§2.1 has already emitted its advisory); the latest round recorded zero rows. Convergence arithmetic is independent of this array and continues to read the stored per-round `clean` flag, so `converged: true` beside an empty `role_streaks` is a reachable and correct combination.

A non-`spec-coverage` role's streak is reported because it separates two states the finding count alone conflates: a lens finding *new* things every round, and a stable set of findings nobody has fixed. The first is a regression the driver must chase; the second is backlog.

### 2.3 `spec-coverage` is reported by name

`spec_coverage_clean_rounds` is a top-level field on both outputs: the `consecutive_clean` of the `spec-coverage` role, or `null` when **no** recorded audit round holds a row attributed to that id.

`null` and `0` are different answers and must not be collapsed. `0` means the role was measured and its streak is broken. `null` means nothing in the recorded history measured spec conformance, and a driver must never read the resulting absence of `divergence` as evidence of anything. Two paths reach `null`, and the second is the likelier:

1. The project's auditor corpus holds no `spec-coverage` role. `enabled: false` cannot produce this — v0.32.0 refuses it — but a populated `.tp/auditors/` without that file can.
2. The role is active but emits no prompt, because the spec yields no checklist items; it is named in `skipped_roles` with the reason `no-checklist-items`. On a spec with no tables and no numbered lists this holds every round, `spec_coverage_clean_rounds` stays `null`, and `divergence` never fires.

Neither path is a defect and tp adds no refusal for either. They are why the field is tri-state: a driver that reads `null` learns that this loop is not measuring conformance, which is a different problem from the one this release solves.

The field is reported even when the `spec-coverage` entry is present in `role_streaks`. The point of this release is that the number a driver must read is not buried in an array it may not scan — and under `--compact` that array is not there at all (§2.5).

### 2.4 The divergence object

`divergence` is emitted when **all three** hold:

1. `spec_coverage_clean_rounds` is non-null and is at least the effective `audit_clean_rounds`.
2. The latest recorded audit round holds at least one non-PASS row **not** attributed to `spec-coverage` — including any row carrying no role.
3. The spec is not stale: its hash matches the one recorded on the latest round.

Its fields:

| Field | Meaning |
|---|---|
| `other_roles_open` | the count of the non-PASS rows in condition 2 |
| `open_roles` | the ids of the roles holding those rows, ascending; a row carrying no role contributes no id |
| `unattributed_open` | how many of those rows carry no role; present only when greater than 0 |
| `message` | the sentence below |
| `hint` | the decision the driver now owns, quoted in §2.6 |

Condition 1's threshold is the effective `audit_clean_rounds`, and condition 3 is the second half of `engine.Converged`. Together they make the claim behind the signal literally true: `divergence` fires exactly when `spec-coverage` alone would have satisfied convergence. Without condition 3 a spec edited after the last recorded round would produce a divergence report whose honest answer is "re-audit, the spec moved".

`open_roles` is what makes the object self-sufficient: it names *which* lenses hold the open findings, so the decision §2.6 hands to the driver can be made from `divergence` alone, including under `--compact` where `role_streaks` is absent.

A row with no role is counted in `other_roles_open` because it cannot be shown to belong to another role — but neither can it be shown to belong to `spec-coverage`, so the count alone would let the message assert clean conformance while a possible spec violation sits unattributed in the same round. `unattributed_open` records it, and `message` carries it inline so the sentence is never read without it.

The message is one of these two, with `round`/`rounds` and `finding`/`findings` agreeing with their numbers:

```
spec-coverage clean 9 rounds; 24 findings open from other roles
spec-coverage clean 9 rounds; 24 findings open from other roles (including 3 with no role, which may be spec-coverage's)
```

The second form is used exactly when `unattributed_open` is present. The sentence restates numbers that sit in sibling fields, and that is deliberate: in v0.32.0 every number was already on screen for eight rounds and the conclusion was still not drawn.

When any condition fails, `divergence` is absent. Absence means only that the three conditions did not all hold.

### 2.5 Where the signal appears

`spec_coverage_clean_rounds`, `role_streaks` and `divergence` appear on `tp audit <spec> --status` (with or without `--check`) and on `tp audit <spec> --record <file>`.

On `--record` the signal is computed **after** the round is stored, so "the latest recorded round" is the round just recorded — the convention `harness_stale` already follows on that path.

Under `--compact`: `spec_coverage_clean_rounds` and `divergence` survive, because they are the decision the driver makes; `role_streaks` is omitted as explanatory. `harness_stale` is the compact-omitted companion on both outputs; `overlap_report` is also omitted, but only `--status` emits it at any verbosity.

`tp audit --merge` reads loose NDJSON files with no recorded-round history and emits none of these fields. `tp review` emits none of them either (Non-Goal 3).

### 2.6 The gate is untouched

`engine.Converged`, `engine.ConsecutiveClean`, the stored per-round `clean` flag, and the exit code of `tp audit --status --check` are unchanged by this release. A round holding only non-`spec-coverage` findings is still not clean and still does not count toward convergence. `next_action` keeps its three-state audit precedence and gains no divergence branch (Non-Goal 4).

The `hint` says so, verbatim:

```
spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Decide explicitly whether they gate this release — audit convergence still counts every non-PASS row.
```

That sentence is the release's contract with its driver: tp reports the divergence and names who decides. It does not decide, and it offers no way to clear the gate on the driver's behalf (Non-Goal 8).

## 3. A registered check retires its mechanize candidate

### 3.1 The suppression rule

A finding class is **mechanized** when the effective workflow's `checks` holds a **valid** entry whose `class` equals it exactly.

An entry is valid when `engine.ValidateChecks` accepts it **on its own** — the single-entry call `runMechanicalChecks` already makes for each entry it runs. Validity is judged per entry, never over the slice, and three consequences follow from that choice:

1. One invalid entry never suppresses another entry's class, and never un-suppresses it either.
2. The validator's duplicate-class rule is cross-entry and is structurally unreachable in the single-entry form, so a class registered twice is mechanized rather than rejected.
3. An entry that is skipped rather than executed can never retire the suggestion that asked for a check — which is the whole reason validity is consulted at all.

A mechanized class is excluded from `mechanize_candidates`. The frequency threshold — a class in at least 2 distinct rounds, or at least 5 times in one round — is unchanged, and the observable rule is that suppressing one class never changes whether another class crosses it.

Two further consequences, of which the second is the one that matters:

1. When exclusion empties the candidate list, the accompanying register-a-check `hint` is not emitted, because that hint is already conditional on a non-empty list.
2. `next_action` does not name a mechanized class. When branches 1 and 2 of its precedence do not apply and every candidate is mechanized, the state falls through to the run-the-next-round branch, and the driver is no longer told to write a check that exists.

Whether the registered check **passes** is irrelevant to suppression: registration is the trigger. A failing check is already reported in `mechanical_checks` and already gates the exit code of `tp review --status --check`; suppressing the suggestion does not weaken that.

### 3.2 Where suppression applies

The definition in §3.1 is the release's single meaning of "mechanized" and governs every place tp derives a class list from `checks`:

1. `tp review <spec> --record <file>` — filters `mechanize_candidates`, the register-a-check `hint`, and the class list handed to `next_action`.
2. `tp review <spec> --status [--check]` — filters the class list handed to `next_action`. This mode emits no `mechanize_candidates` array of its own.
3. `tp review --report <files…>` — filters `mechanize_candidates` and the TTY block described in §3.3.
4. `tp review <spec>` prompt emission — the reviewer-facing "do NOT report findings of these classes" list drops entries §3.1 calls invalid. Telling a reviewer to stop reporting a class whose check never runs is the same defect in the other direction, and leaving this path on a second predicate would give one release-defined term two behaviours.

`tp audit` surfaces no mechanize candidates and is unaffected.

Items 1, 2 and 4 take a spec positional and resolve the workflow as they do today. Item 3 does not: `tp review --report` takes NDJSON positionals and no spec, so it cannot use spec-anchored resolution. Its workflow resolves through the standard task-file discovery chain — `--file` > `TP_FILE` > the `.tp/local.json` active pointer > auto-detect — layered over `.tp/config.json` in the usual precedence. Two rules make that unambiguous:

- The discovered task file is used **unconditionally**. Its `spec` field is not consulted, and the spec-adjacent `<spec-base>.tasks.json` fallback does not apply, because there is no spec to be adjacent to. Reusing the spec-anchored helper unchanged would compare against an empty spec path, match nothing, and ship a suppression path that never fires.
- `checks` registered in `.tp/config.json` are in force even when **no** task file resolves. That layer is not conditional on a task file, so a project-config check suppresses its class from `--report` in an empty directory. Nothing is suppressed only when no valid check is registered in either layer.

Because item 3 makes an otherwise position-only command depend on ambient discovery, it discloses what resolved: `checks_source` carries the path of the task file whose workflow was used, `.tp/config.json` when only the project layer registered checks, or `null` when no valid check is registered at all. Without it, "no candidate suppressed" and "no workflow resolved" are the same output, and the second is recoverable only by guessing at `--file`.

### 3.3 What was suppressed stays visible

`mechanized_classes` accompanies `mechanize_candidates` — so on `tp review --record` and `tp review --report` — carrying the candidate classes withheld because a valid check is registered, sorted ascending. It is emitted only when non-empty, and it survives `--compact` alongside `mechanize_candidates`, which that flag has never stripped.

In `tp review --report`'s TTY rendering it appears as a `Mechanized Classes:` block under the same non-empty condition, immediately after the mechanize-candidates block. §3.3's argument applies to a human reading the TTY output exactly as it does to an agent reading JSON, and the release must not fix the defect on one path and reintroduce it on the other.

It lists the intersection, not every registered class: the registered set is what the driver wrote and can read back from `tp config` and `tp review --status`, so restating all of it beside the candidate list would spend tokens on configuration rather than on what changed. Without this field a class simply vanishes from the output on the round after it is registered, which reads as a bug.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md` and `skills/tp/REFERENCE.md` must each contain both substrings:

- `audit convergence still counts every non-PASS row`
- `a registered check retires its mechanize candidate`

`CLAUDE.md`'s existing audit-scope rule must name `spec_coverage_clean_rounds` as the field carrying the number that rule currently tells the driver to track by hand. It is not required to repeat the two substrings above: `CLAUDE.md` and `SKILL.md` are both loaded into an agent's context in this repository, and a guard test that pins the same sentence in both makes the duplication permanent.

`REFERENCE.md` must additionally document the JSON shape of `role_streaks`, `spec_coverage_clean_rounds`, `divergence`, `mechanized_classes` and `checks_source`, and must contain these three sentences verbatim, so the guard test asserts the document rather than the implementer's choice of anchor:

1. `A role with no rows in a round is not clean in it, so its streak ends.`
2. `spec_coverage_clean_rounds is null, not 0, when no recorded round measured that role.`
3. `role_streaks is omitted under --compact; spec_coverage_clean_rounds and divergence survive it.`

## 5. Non-Goals

1. **A `scope` field on audit rows.** Classifying a finding as `spec` or `codebase` is a judgement, and the only party positioned to make it is the sub-agent that wrote the row. Wired into the gate, one row mislabelled `codebase` lets a genuine spec violation ship — strictly worse than today's rule, which over-counts and therefore only wastes rounds. The signal in §2 needs no such field: `spec-coverage`'s streak is derived from routing tp already owns.
2. **`audit_converge_on`.** It has no meaning without item 1. Deferred to v0.34.0 with the evidence this release produces: if the divergence signal turns out to be enough for a driver to make the call, the policy is not needed.
3. **The same signal for `tp review`.** A reviewer commenting on code style rather than the spec has the same shape, but review already has a severity-aware predicate and no reviewer role monopolizes conformance the way `spec-coverage` does. There is no equivalent named streak to report.
4. **A divergence branch in `next_action`.** `next_action` names one step; making divergence one of them makes it advice rather than reporting, and §1's whole argument is that the decision is the driver's.
5. **Auto-resolving or parking a non-`spec-coverage` finding.** tp has no audit-side `--resolve`. Adding one is how a round of open findings quietly becomes a clean round.
6. **Per-role convergence thresholds.** One `audit_clean_rounds` still governs the whole phase.
7. **Retiring a mechanize suggestion because a check passes.** Registration is the trigger (§3.1). A pass-conditioned rule would make the suggestion reappear whenever the check went red, which is the moment the class is least in need of a second check.
8. **Recording the driver's acceptance of a divergence.** Anything tp stores that then clears the gate is `audit_converge_on` under another name, and it would arrive without the policy debate item 2 defers. The artifact today is a decision document the driver writes and commits beside the recorded rounds, which tp neither requires nor parses; `--harness-note` remains available for framing that belongs on a round. `tp audit --status --check` keeps exiting 1 while findings are open, and a CI wired to it stays red — which is the honest report of the recorded state.

## 6. Tests

1. Per-role streaks: three recorded rounds where `spec-coverage` is all-PASS in every round and a second role has a FAIL in the latest one. `role_streaks` reports `spec-coverage` with `consecutive_clean` 3 and `open` 0, and the other role with `consecutive_clean` 0 and `open` 1.
2. A role absent from a round ends its streak: a role clean in rounds 1 and 3 but with no rows at all in round 2 reports `consecutive_clean` 1, not 3. This is the discriminating case against an implementation that skips unmeasured rounds instead of ending on them.
3. `role_streaks` covers only the latest round's roles: a role present in round 1 and absent from round 2 does not appear in the array reported at round 2.
4. Ordering: with `spec-coverage` plus two other roles in the latest round, `spec-coverage` is first and the remaining two follow in ascending id order — asserted with fixture ids chosen so alphabetical order alone would not put `spec-coverage` first.
5. Row-role predicate: rows whose `role` is absent, `""`, or a number create no `role_streaks` entry, and no entry with an empty id ever appears.
6. A round whose recorded file is deleted contributes no rows, ends every streak, and emits the `round <N> file <name> is missing; skipping its rows` advisory once. A round file left in place but containing one malformed line keeps its valid rows, emits no such advisory, and its roles stay clean — the discriminating half against an implementation that treats any parse failure as unreadable.
7. `role_streaks` is `[]` with no recorded round, and `[]` when the latest round's file is missing while `converged` still reads from the stored per-round flags. The second half pins that the array and convergence arithmetic are independent.
8. `spec_coverage_clean_rounds` is `null` when no recorded round holds a `spec-coverage` row, and `0` when a round holds one that is not PASS. Both halves in one test; an implementation defaulting the absent case to `0` fails the first.
9. `divergence` fires when `spec-coverage` has been clean for the effective `audit_clean_rounds` and another role holds an open finding, and its `message` matches `spec-coverage clean 2 rounds; 1 finding open from other roles` verbatim. A second fixture with one clean round required and one open finding asserts the singular `spec-coverage clean 1 round; 1 finding open from other roles`, pinning both pluralizations.
10. `open_roles` names exactly the non-`spec-coverage` roles holding open rows, ascending, and omits an id for a row carrying no role.
11. `divergence` is absent when `spec-coverage`'s streak is below the threshold, and absent when it meets the threshold with no other role holding an open finding.
12. The threshold is the effective `audit_clean_rounds`, not a literal 2: with `audit_clean_rounds` set to 3 and `spec-coverage` clean for exactly 2 rounds alongside another role's open finding, `divergence` is absent; a third clean round makes it fire.
13. A stale spec suppresses `divergence` even when conditions 1 and 2 hold — the spec file is edited after the last round is recorded, `stale` reads true, and `divergence` is absent.
14. A non-PASS row carrying no `role` is counted in `other_roles_open`, disclosed in `unattributed_open`, and named inline in `message` with the `(including 1 with no role, which may be spec-coverage's)` suffix. With every non-`spec-coverage` open row attributed, `unattributed_open` is absent and the suffix does not appear.
15. `--record` computes the signal over the just-recorded round: recording a round in which `spec-coverage` completes its streak emits `divergence` in that same invocation's output, without a following `--status`.
16. `--compact` on both `--status` and `--record` omits `role_streaks` and retains `spec_coverage_clean_rounds` and `divergence`, including `divergence.open_roles`.
17. The gate is unchanged: a fixture whose latest round holds only non-`spec-coverage` findings still reports `clean: false` and `converged: false`, `tp audit --status --check` still exits 1, and `next_action` is still the fix-and-re-audit directive. This is the guard test for §2.6 and must fail any implementation that lets the divergence signal reach convergence arithmetic.
18. `tp audit --merge` emits none of `role_streaks`, `spec_coverage_clean_rounds`, `divergence`.
19. A registered check suppresses its class: a class over the candidate threshold with a matching `checks` entry is absent from `mechanize_candidates` on `--record`, while an unregistered class over the same threshold is still listed.
20. Validity is per entry: a `checks` array holding one entry the validator rejects and one valid entry over a candidate class suppresses the valid entry's class and leaves the rejected entry's class listed. An implementation validating the whole slice suppresses neither and fails this test.
21. A class registered twice is mechanized rather than rejected, pinning §3.1 consequence 2 against an implementation that validates the slice and trips the duplicate-class rule.
22. Suppressing every candidate removes the register-a-check `hint` from `--record` output.
23. `tp review --status` honours the suppression through `next_action` on the same fixture, with no `mechanize_candidates` array of its own in the output. This is the discriminating mode for the `next_action` path: `--status` derives its class list from the recorded rounds by a separate call, so an implementation that filters only `--record`'s emitted array still names the mechanized class here.
24. `tp review --report` suppresses a class registered in a task file resolved through the discovery chain, and suppresses a class registered only in `.tp/config.json` with no task file resolving. The second half is the discriminating one against an implementation that gates suppression on a resolved task file.
25. `checks_source` on `--report` carries the resolved task file's path, `.tp/config.json` when only that layer registered a check, and `null` when neither does.
26. A registered check whose command fails still suppresses its class, asserted with a check whose command exits non-zero.
27. `mechanized_classes` lists the suppressed classes sorted ascending, is absent when nothing was suppressed, does not list a registered class that never reached candidate frequency, and survives `--compact` on `--record`.
28. `tp review --report`'s TTY rendering shows a `Mechanized Classes:` block when the field is non-empty and omits it otherwise.
29. Prompt emission drops an invalid entry from the "do NOT report findings of these classes" list while keeping a valid one — §3.2 item 4, and the test that keeps one release-defined term from meaning two things.
30. A guard test asserts `README.md`, `skills/tp/SKILL.md` and `skills/tp/REFERENCE.md` each contain both required substrings of §4, that `CLAUDE.md` contains `spec_coverage_clean_rounds`, and that `REFERENCE.md` contains the three verbatim sentences.

### 6.1 Existing tests this change invalidates

Derived by running the search, not asserted. The tokens `computeMechanizeCandidates`, `mechanizeCandidateClasses`, `mechanizeClassesFromRounds`, `mechanize_candidates` and `runAuditStatus` over `internal/**/*_test.go` name the files that pin the shapes this release changes.

1. `internal/cli/audit_record_test.go` — asserts the `tp audit --record` and `--status` payload shape, including that no `mechanical_checks` key is present. The new fields are additive, so its existing assertions hold; it gains the new ones rather than being rewritten.
2. `internal/cli/report_class_test.go` — asserts `mechanize_candidates` from `tp review --report` over fixtures with no task file in scope. It is the file that pins the "nothing registered in either layer" half of §3.2, and it is invalidated only if any of its fixtures sits under a `.tp/config.json` carrying `checks`.
3. `internal/cli/review_record_test.go` and `internal/cli/nextaction_test.go` — pin the candidate list and the mechanize branch of `next_action` against workflows with no registered checks. Both stay valid; the new behaviour is a new case in each, and either of them turning red means suppression is firing where no check is registered.
4. `internal/cli/review_test.go` — contains the prompt-emission assertions for the "do NOT report findings of these classes" list. §3.2 item 4 changes that list's membership rule, so any case there registering an entry the validator rejects is invalidated.
