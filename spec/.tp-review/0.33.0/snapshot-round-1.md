# tp v0.33.0 — Honest convergence signals

## 1. Overview

tp's audit loop counts every non-PASS row against convergence, whether the row concerns the spec or the codebase at large. The general lenses in a real corpus always find something, so under that predicate there is no fixed point.

v0.32.0's own audit is the evidence. `spec-coverage` — the only auditor role that takes the spec-derived checklist, and therefore the only one measuring conformance — was 55/55 clean from round 2 through round 11, and no round in the entire audit produced a single FAIL. Rounds 3–11 were consumed by hint wording, advisory channels, git scoping and flag hygiene that `spec/0.32.0.md` never mentions. The divergence was real from round 3; it was noticed by hand at round 10, and only because a human asked why the loop was taking so long.

The failure is not that tp lacked a policy. It is that tp never said what was happening. A driver reading `tp audit --status` saw `converged: false` and a finding count, and nothing that distinguished "the implementation does not match the spec" from "the implementation matches the spec and three general lenses are reading the rest of the repository". Both states print the same shape.

This release ships the signal and nothing else. Two items:

1. **The divergence signal.** `tp audit --status` and `--record` report each auditor role's consecutive clean-round count, name `spec-coverage`'s separately, and say so explicitly when `spec-coverage` has been clean for the required number of rounds while other roles still hold open findings.
2. **A registered check retires its own suggestion.** Registering a `checks` entry for a recurring class should stop tp from recommending that the check be written. It currently keeps recommending one that already exists.

**The convergence gate does not change.** No row scope, no `audit_converge_on`, no change to which rounds count as clean, no change to an exit code. §5 records why that separation is deliberate rather than incremental: a classification produced by a sub-agent, wired into the release gate, lets a genuine spec violation mislabelled `codebase` pass the gate — a worse failure mode than today's rule, which at least over-counts. Ship the signal, use it for a version, then decide whether the policy is needed.

## 2. The audit divergence signal

### 2.1 Per-role clean streaks

A recorded audit round's rows carry a `role`. For one role id, that round's **role rows** are the rows whose `role` field is a non-empty string equal to that id.

A role is **clean in a round** when it has at least one row in that round and every one of its rows has `status` exactly `"PASS"`. A role with no rows in a round is not clean in it: the role was not measured, so cleanliness cannot be claimed for it.

A role's `consecutive_clean` counts trailing recorded audit rounds in which the role is clean, stopping at the first trailing round in which it is not. A round whose recorded NDJSON file is missing or unreadable contributes no rows, and therefore ends every role's streak.

`tp audit --status` and `tp audit --record` carry a `role_streaks` array. Its entries are the roles appearing in the **latest** recorded audit round — the panel the current decision rests on, not every role ever seen. Each entry is:

| Field | Meaning |
|---|---|
| `role` | the role id |
| `consecutive_clean` | trailing rounds in which the role is clean, as defined above |
| `open` | that role's non-PASS row count in the latest recorded round |

Order is `spec-coverage` first when it is present, then the remaining ids ascending. With no recorded audit round, `role_streaks` is `[]`.

### 2.2 `spec-coverage` is reported by name

`spec_coverage_clean_rounds` is a top-level field on both outputs: the `consecutive_clean` of the `spec-coverage` role, or `null` when **no** recorded audit round holds a row attributed to that id.

`null` and `0` are different answers and must not be collapsed. `0` means the role was measured and its streak is broken; `null` means nothing in the recorded history measured spec conformance, which is the state of a project whose auditor corpus has no `spec-coverage` role at all. A corpus can reach that state by holding auditor role files that do not include one — `enabled: false` cannot, because v0.32.0 refuses it.

The field is reported even when it equals `role_streaks[0].consecutive_clean`. The whole point of this release is that the number a driver must read is not buried in an array it may not scan.

### 2.3 The divergence object

`divergence` is emitted when **both** hold:

1. `spec_coverage_clean_rounds` is non-null and is at least the effective `audit_clean_rounds`.
2. The latest recorded audit round holds at least one non-PASS row **not** attributed to `spec-coverage` — including any row carrying no role at all.

Its fields:

| Field | Meaning |
|---|---|
| `other_roles_open` | the count of the non-PASS rows in condition 2 |
| `unattributed_open` | how many of those rows carry no role; present only when greater than 0 |
| `message` | `spec-coverage clean <N> rounds; <M> findings open from other roles` |
| `hint` | the decision the driver now owns, quoted in §2.5 |

The threshold is the effective `audit_clean_rounds`, not a literal 2: the signal fires exactly when `spec-coverage` alone would have satisfied convergence.

A row with no role is counted in `other_roles_open` because it cannot be shown to be `spec-coverage`'s, and the count must not under-report. `unattributed_open` discloses that this happened, so a driver never reads a confident per-role picture assembled from rows that carry no attribution. `tp` already warns once per file about roleless rows at record time; this is the same condition surfaced where the decision is made.

When either condition fails, `divergence` is absent. Absence is the signal that there is no divergence.

### 2.4 Where the signal appears

Both fields of §2.2 and §2.3 appear on `tp audit <spec> --status` (with or without `--check`) and on `tp audit <spec> --record <file>`.

On `--record` the signal is computed **after** the round is stored, so "the latest recorded round" is the round just recorded — the convention `harness_stale` already follows on that path.

Under `--compact`: `spec_coverage_clean_rounds` and `divergence` survive, because they are the decision the driver makes; `role_streaks` is omitted as explanatory, alongside `harness_stale` and `overlap_report`.

`tp audit --merge` reads loose NDJSON files with no recorded-round history and emits none of these fields. `tp review` emits none of them either (Non-Goal 3).

### 2.5 The gate is untouched

`engine.Converged`, `engine.ConsecutiveClean`, `ReviewRound.Clean`, and the exit code of `tp audit --status --check` are unchanged by this release. A round holding only non-`spec-coverage` findings is still not clean and still does not count toward convergence. `next_action` keeps its three-state audit precedence and gains no divergence branch (Non-Goal 4).

The `hint` says so, verbatim:

```
spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Decide explicitly whether they gate this release — audit convergence still counts every non-PASS row.
```

That sentence is the release's contract with its driver: tp reports the divergence and names who decides. It does not decide.

## 3. A registered check retires its mechanize candidate

### 3.1 The suppression rule

A finding class is **mechanized** when the effective workflow's `checks` holds an entry whose `class` equals it exactly.

A mechanized class is excluded from `mechanize_candidates`. Exclusion is applied to the computed candidate list, so the frequency threshold — a class in at least 2 distinct rounds, or at least 5 times in one round — is unchanged; only what is emitted shrinks.

Three consequences follow, and the third is the one that matters:

1. `mechanize_candidates` no longer lists the class.
2. When exclusion empties the list, the accompanying register-a-check `hint` is not emitted, because that hint is already conditional on a non-empty list.
3. `next_action` branch 3 does not fire on a mechanized class. With every candidate mechanized, the state falls through to branch 4 and the driver is told to run the next round instead of to write a check that exists.

Whether the registered check **passes** is irrelevant to suppression: registration is the trigger. A failing check is already reported in `mechanical_checks` and already gates the exit code of `tp review --status --check`; suppressing the suggestion does not weaken that.

An entry that `engine.ValidateChecks` rejects does **not** mechanize its class. Such an entry is skipped rather than executed, so it can never retire the suggestion that asked for a check.

### 3.2 Where suppression applies

1. `tp review <spec> --record <file>` — filters `mechanize_candidates`, the register-a-check `hint`, and the class list handed to `next_action`.
2. `tp review <spec> --status [--check]` — filters the class list handed to `next_action`. This mode emits no `mechanize_candidates` array of its own.
3. `tp review --report <files…>` — filters `mechanize_candidates` and the TTY block's hint.

`--report` takes NDJSON positionals and no spec, so it cannot use the spec-anchored workflow resolution the other two use. It resolves the workflow through the standard task-file discovery chain — `--file` > `TP_FILE` > the `.tp/local.json` active pointer > auto-detect — layered over the project config as usual. When no task file resolves, no checks are registered and nothing is suppressed, which is exactly today's behaviour.

`tp audit` surfaces no mechanize candidates and is unaffected.

### 3.3 What was suppressed stays visible

`mechanized_classes` accompanies `mechanize_candidates` — so on `tp review --record` and `tp review --report` — carrying the candidate classes withheld because a check is registered, sorted ascending. It is emitted only when non-empty.

It lists the intersection, not every registered check class: a registered check for a class that never reached candidate frequency is already visible in `mechanical_checks`, and repeating it here would cost tokens to say nothing. Without this field a class simply vanishes from the output on the round after it is registered, which reads as a bug.

## 4. Documentation

`README.md`, `skills/tp/SKILL.md`, `skills/tp/REFERENCE.md` and `CLAUDE.md` must each contain both substrings:

- `audit convergence still counts every non-PASS row`
- `a registered check retires its mechanize candidate`

`REFERENCE.md` must additionally document the JSON shape of `role_streaks`, `spec_coverage_clean_rounds`, `divergence` and `mechanized_classes`, and must state three rules a reader cannot derive from the shapes:

1. A role with no rows in a round is not clean in it, so its streak ends.
2. `spec_coverage_clean_rounds` is `null`, not `0`, when no recorded round measured that role.
3. `role_streaks` is omitted under `--compact` while `spec_coverage_clean_rounds` and `divergence` survive it.

`CLAUDE.md`'s existing audit-scope rule must name `spec_coverage_clean_rounds` as the field that now carries the number that rule tells the driver to track by hand.

## 5. Non-Goals

1. **A `scope` field on audit rows.** Classifying a finding as `spec` or `codebase` is a judgement, and the only party positioned to make it is the sub-agent that wrote the row. Wired into the gate, one row mislabelled `codebase` lets a genuine spec violation ship — strictly worse than today's rule, which over-counts and therefore only wastes rounds. The signal in §2 needs no such field: `spec-coverage`'s streak is derived from routing tp already owns.
2. **`audit_converge_on`.** It has no meaning without item 1. Deferred to v0.34.0 with the evidence this release produces: if the divergence signal turns out to be enough for a driver to make the call, the policy is not needed.
3. **The same signal for `tp review`.** A reviewer commenting on code style rather than the spec has the same shape, but review already has a severity-aware predicate and no reviewer role monopolizes conformance the way `spec-coverage` does. There is no equivalent named streak to report.
4. **A divergence branch in `next_action`.** `next_action` names one step; making divergence one of them makes it advice rather than reporting, and §1's whole argument is that the decision is the driver's.
5. **Auto-resolving or parking a non-`spec-coverage` finding.** tp has no audit-side `--resolve`. Adding one is how a round of open findings quietly becomes a clean round.
6. **Per-role convergence thresholds.** One `audit_clean_rounds` still governs the whole phase.
7. **Retiring a mechanize suggestion because a check passes.** Registration is the trigger (§3.1). A pass-conditioned rule would make the suggestion reappear whenever the check went red, which is the moment the class is least in need of a second check.

## 6. Tests

1. Per-role streaks: three recorded rounds where `spec-coverage` is all-PASS in every round and a second role has a FAIL in the latest one. `role_streaks` reports `spec-coverage` with `consecutive_clean` 3 and `open` 0, and the other role with `consecutive_clean` 0 and `open` 1.
2. A role absent from a round ends its streak: a role clean in rounds 1 and 3 but with no rows at all in round 2 reports `consecutive_clean` 1, not 3. This is the discriminating case against an implementation that skips unmeasured rounds instead of ending on them.
3. `role_streaks` covers only the latest round's roles: a role present in round 1 and absent from round 2 does not appear in the array recorded at round 2.
4. Ordering: with `spec-coverage` plus two other roles in the latest round, `spec-coverage` is first and the remaining two follow in ascending id order — asserted with fixture ids chosen so alphabetical order alone would not put `spec-coverage` first.
5. `spec_coverage_clean_rounds` is `null` when no recorded round holds a `spec-coverage` row, and `0` when a round holds one that is not PASS. Both halves in one test; an implementation defaulting the absent case to `0` fails the first.
6. `divergence` fires when `spec-coverage` has been clean for the effective `audit_clean_rounds` and another role holds an open finding, and its `message` matches `spec-coverage clean 2 rounds; 1 findings open from other roles` verbatim.
7. `divergence` is absent when `spec-coverage`'s streak is below the threshold, and absent when it meets the threshold with no other role holding an open finding. Two fixtures, one test.
8. The threshold is the effective `audit_clean_rounds`, not a literal 2: with `audit_clean_rounds` set to 3 and `spec-coverage` clean for exactly 2 rounds alongside another role's open finding, `divergence` is absent; a third clean round makes it fire.
9. A non-PASS row carrying no `role` is counted in `other_roles_open` and disclosed in `unattributed_open`. With every non-`spec-coverage` open row attributed, `unattributed_open` is absent.
10. `--record` computes the signal over the just-recorded round: recording a round in which `spec-coverage` completes its streak emits `divergence` in that same invocation's output, without a following `--status`.
11. `--compact` on both `--status` and `--record` omits `role_streaks` and retains `spec_coverage_clean_rounds` and `divergence`.
12. The gate is unchanged: a fixture whose latest round holds only non-`spec-coverage` findings still reports `clean: false` and `converged: false`, `tp audit --status --check` still exits 1, and `next_action` is still the fix-and-re-audit directive. This is the guard test for §2.5 and must fail any implementation that lets the divergence signal reach convergence arithmetic.
13. `tp audit --merge` emits none of `role_streaks`, `spec_coverage_clean_rounds`, `divergence`.
14. A registered check suppresses its class: a class over the candidate threshold with a matching `checks` entry is absent from `mechanize_candidates` on `--record`, while an unregistered class over the same threshold is still listed.
15. Suppressing every candidate removes the register-a-check `hint` and makes `next_action` the run-the-next-round directive rather than the register-a-check one. The `next_action` half is the discriminating assertion: an implementation that filters only the emitted array still fires branch 3.
16. `tp review --status` honours the suppression through `next_action` on the same fixture, with no `mechanize_candidates` array of its own in the output.
17. `tp review --report` suppresses a class registered in a task file resolved through the discovery chain, and suppresses nothing when no task file resolves.
18. A registered check whose command fails still suppresses its class, asserted with a check whose command exits non-zero.
19. An invalid `checks` entry does not suppress: an entry `engine.ValidateChecks` rejects leaves its class in `mechanize_candidates`.
20. `mechanized_classes` lists the suppressed classes sorted ascending, is absent when nothing was suppressed, and does not list a registered class that never reached candidate frequency.
21. A guard test asserts the four documents of §4 each contain both required substrings, and that `REFERENCE.md` states the three rules listed there.

### 6.1 Existing tests this change invalidates

Derived by running the search, not asserted. The tokens `computeMechanizeCandidates`, `mechanizeCandidateClasses`, `mechanizeClassesFromRounds`, `mechanize_candidates` and `runAuditStatus` over `internal/**/*_test.go` name the files that pin the shapes this release changes.

1. `internal/cli/audit_record_test.go` — asserts the `tp audit --record` and `--status` payload shape, including that no `mechanical_checks` key is present. The new fields are additive, so its existing assertions hold; it gains the new ones rather than being rewritten.
2. `internal/cli/report_class_test.go` — asserts `mechanize_candidates` from `tp review --report` over fixtures with no task file in scope. Unchanged in behaviour, since nothing resolves and nothing is suppressed, but it is the file that pins the "no task file resolves" half of §3.2.
3. `internal/cli/review_record_test.go` and `internal/cli/nextaction_test.go` — pin the candidate list and branch 3 of `next_action` against workflows with no registered checks. Both stay valid; the new behaviour is a new case in each, and any of them turning red means suppression is firing where no check is registered.
