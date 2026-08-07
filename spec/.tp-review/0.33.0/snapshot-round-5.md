# tp v0.33.0 — Honest convergence signals

## 1. Overview

tp's audit loop counts every non-PASS row against convergence, whether the row concerns the spec or the codebase at large. The general lenses in a real corpus always find something, so under that predicate there is no fixed point.

v0.32.0's own audit is the evidence. `spec-coverage` — the only auditor role that takes the spec-derived checklist, and therefore the only one measuring conformance — was 55/55 clean from round 2 through round 11, and no round in the entire audit produced a single FAIL. Rounds 3–11 were consumed by hint wording, advisory channels, git scoping and flag hygiene that `spec/0.32.0.md` never mentions. The divergence was real from round 3; it was noticed by hand at round 10, and only because a human asked why the loop was taking so long.

The failure is not that tp lacked a policy. It is that tp never said what was happening. A driver reading `tp audit --status` saw `converged: false` and a finding count, and nothing that distinguished "the implementation does not match the spec" from "the implementation matches the spec and three general lenses are reading the rest of the repository". Both states print the same shape.

This release ships the signal and nothing else. Two items:

1. **The divergence signal.** `tp audit --status` and `--record` report each auditor role's consecutive clean-round count, name `spec-coverage`'s separately, and say so explicitly when `spec-coverage` has been clean for the required number of rounds, with the spec unchanged since the last recorded round, while other roles still hold open findings.
2. **A registered check retires its own suggestion.** Registering a `checks` entry for a recurring class should stop tp from recommending that the check be written. It currently keeps recommending one that already exists.

**The convergence gate does not change.** No row scope, no `audit_converge_on`, no change to which rounds count as clean, no change to an exit code. §5 records why that separation is deliberate rather than incremental: a classification produced by a sub-agent, wired into the release gate, lets a genuine spec violation mislabelled `codebase` pass the gate — a worse failure mode than today's rule, which at least over-counts. Ship the signal, use it for a version, then decide whether the policy is needed.

Two constraints hold throughout. Every rule that could fail in two directions fails toward "keep auditing": a state tp cannot measure never produces the signal that tells a driver the decision is theirs, and every rule below is written to satisfy that rather than to follow the shortest implementation. And every command this release touches stays a function of the spec it is given plus the state recorded beside it — no new output depends on anything a repository does not commit.

## 2. The audit divergence signal

### 2.1 Reading a recorded round's rows

Four definitions govern everything in §2, and none may be restated differently anywhere else:

- **A row's role** is its `role` field, whitespace-trimmed, when that field is a JSON string that is non-empty after trimming. An absent key, a non-string value, an empty string and a whitespace-only string all mean the row **carries no role**. The trimmed value is the role id, compared byte-for-byte and never case-folded — corpus ids are lowercase kebab-case filenames, so a differing case is a different id and must surface as one rather than be silently merged.
- **A row is PASS** when its `status` field is a JSON string exactly equal to `PASS` — no trimming, no case folding. Everything else is non-PASS: an absent key, a non-string value, `pass`, ` PASS `, `FAIL`, `PARTIAL`. This is byte-for-byte the predicate `countAuditFindings` already applies when it decides a round's recorded `findings` count, and it must stay identical to it: if the two drifted, `other_roles_open` and the round's own `findings` would disagree on the same rows. The asymmetry with the `role` rule above is deliberate — `status` has an existing predicate to match and `role` does not.
- **A round contributes no rows** when its entry's `file` is empty, when the recorded NDJSON file cannot be read, or when any line in it fails to parse as JSON. The last case is the one that costs an extra parse and is worth it: a dropped malformed line could delete a role's only non-PASS row, which would make a role clean, lengthen a streak and make `divergence` *more* likely to fire — the one direction §1 forbids. A recorded round is only reachable through `--record`, which rejects malformed rows, so a file in this state was truncated or hand-edited.
- **The walked segment** is the trailing run of recorded rounds tp actually loads, defined in §2.2. Every value in §2 is computed from it alone, and no rule anywhere requires reading the whole recorded history.

A round in the walked segment that contributes no rows emits one advisory through `output.Notice`, naming which of the three causes applies:

| Cause | Advisory |
|---|---|
| entry's `file` is empty | `round <N> has no recorded rows file; skipping its rows` |
| file cannot be read | `round <N> file <name> is missing; skipping its rows` |
| a line fails to parse | `round <N> file <name> has unparseable rows; skipping its rows` |

The middle wording is the one the review record path already uses for the same condition. Such a round terminates the walk, so at most one advisory can fire per invocation. It applies on both outputs named in §2.5.

`output.Notice` is silenced by `--quiet`, and a round contributing no rows is not always the latest one — it terminates the walk, so it is the oldest round of the segment and the rounds above it can be perfectly readable. A `--quiet` driver therefore sees a truncated streak with no statement of why: `spec_coverage_clean_rounds: 1` reads the same whether `spec-coverage` regressed in the round below or that round's rows are gone. That is accepted, because both readings carry the same instruction. The skipped round *ends* the streak, so the reported number is never larger than the truth and `divergence` is never more likely to fire than it should be — the driver is told to keep auditing either way, and the cause only matters once it stops being told that.

### 2.2 Per-role clean streaks

A role is **clean in a round** when it has at least one row in that round and every one of its rows is PASS. A role with no rows in a round is not clean in it: the role was not measured, so cleanliness cannot be claimed for it.

A role's `consecutive_clean` counts trailing recorded audit rounds in which the role is clean, stopping at the first trailing round in which it is not. A round that contributes no rows therefore ends every role's streak, as does a round recorded before the role corpus existed, whose rows carry no role at all. Both are the conservative direction: they reset the signal toward "keep auditing" rather than toward "you may ship".

The walk starts at the latest recorded round and stops as soon as no further round can change a reported value — when every role present in the latest round has had its streak closed, when a round contributes no rows (which closes them all at once), or at the start of the recorded history. It is not a claim that the walk is short: in the state this release exists to detect, `spec-coverage`'s streak runs for most of the history and the walk runs with it. What the rule guarantees is that nothing is read whose contents provably cannot change an emitted value, and that the advisory of §2.1 cannot repeat.

`tp audit --status` and `tp audit --record` carry a `role_streaks` array. Its entries are the roles appearing in the **latest** recorded audit round — the panel the current decision rests on, not every role ever seen. Each entry is:

| Field | Meaning |
|---|---|
| `role` | the role id |
| `consecutive_clean` | trailing rounds in which the role is clean, as defined above |
| `open` | that role's non-PASS row count in the latest recorded round |

Order is `spec-coverage` first when it is present, then the remaining ids ascending.

`role_streaks` is `[]` whenever no role appears in the latest recorded audit round. Four states reach it: no recorded round exists at all; the latest round contributes no rows, by any of §2.1's three causes; the latest round recorded zero rows; the latest round's rows all carry no role. The array does not distinguish them, and it is not the field that discloses them — `spec_coverage_clean_rounds` reports `null` in all four (§2.3), which is the statement a driver needs: this round did not measure conformance. Convergence arithmetic is independent of the array and continues to read the stored per-round `clean` flag, so `converged: true` beside an empty `role_streaks` is a reachable and correct combination.

Every entry has rows in the latest round, so `open == 0` and `consecutive_clean >= 1` are the same condition, and for a role holding open rows the streak is always 0 and says nothing `open` does not. The streak earns its place on the roles that are quiet, where `open` gives one bit and the streak gives its length: a lens clean since round 2 and one that went clean this round after a repair both read `open: 0`, and only the streak separates them. That length is what tells a driver whether the panel has been stable long enough for the remaining findings to be backlog.

`tp audit --status` also emits `overlap_report`, and it cannot carry these numbers. `engine.OverlapReport` credits a role only for the non-PASS clusters it contributed to and documents that a role that found nothing never appears — so the roles `role_streaks` exists to describe, the quiet ones, are exactly the roles absent from it. The two arrays are near-disjoint in the state that matters rather than redundant: `overlap_report` counts clusters and answers "which role is redundant", `role_streaks` counts rows and is the only place a per-round history appears.

### 2.3 `spec-coverage` is reported by name

`spec_coverage_clean_rounds` is a top-level field on both outputs: the `consecutive_clean` of the `spec-coverage` role, or `null` when the **latest** recorded audit round holds no row attributed to that id — including when no round is recorded at all. The key is always emitted, `null` as an explicit JSON null, never an omitted key, and that holds under `--compact` too (§2.5).

`null` and `0` are different answers and must not be collapsed. `0` means the role was measured in the latest round and at least one of its rows is not PASS. `null` means the latest round did not measure conformance, and a driver must never read the resulting absence of `divergence` as evidence of anything. The reachable paths to `null` are all instances of that one rule:

1. The project's auditor corpus holds no `spec-coverage` role. `enabled: false` cannot produce this — v0.32.0 refuses it — but a populated `.tp/auditors/` without that file can.
2. The role is active but emits no prompt, because the spec yields no checklist items; it is named in `skipped_roles` with the reason `no-checklist-items`. On a spec with no tables and no numbered lists this holds every round.
3. The latest round contributes no rows, recorded zero rows, or holds only rows that carry no role — including a history recorded before the role corpus existed.

None is a defect and tp adds no refusal for any of them. They are why the field is tri-state: a driver that reads `null` learns that this loop is not measuring conformance, which is a different problem from the one this release solves.

Anchoring `null` on the latest round rather than on the whole history is deliberate. It is the question a driver is asking — *did the round I just recorded measure conformance?* — and it keeps the field readable without scanning rounds §2.2's walk would otherwise never touch.

The field is reported even when the `spec-coverage` entry is present in `role_streaks`. The point of this release is that the number a driver must read is not buried in an array it may not scan — and under `--compact` that array is not there at all (§2.5).

### 2.4 The divergence object

`divergence` is emitted when **all three** hold:

1. `spec_coverage_clean_rounds` is non-null and is at least the effective `audit_clean_rounds`, or at least 1 when that value is lower. The floor matters: `engine.ResolveWorkflow` layers a task-file `workflow` block without clamping and `tp validate` reports an out-of-range `audit_clean_rounds` as a warning, so an imported or hand-edited `0` reaches this comparison. Without the floor it would be satisfied by `spec_coverage_clean_rounds: 0` — the state in which `spec-coverage` itself is failing — and the signal would fire against its own meaning.
2. The latest recorded audit round holds at least one non-PASS row **not** attributed to `spec-coverage` — including any row carrying no role.
3. The spec is not stale: `engine.StateStale` over the recorded audit rounds and the current spec hash is false — the same value both outputs already report as `stale`.

Its fields, all five always present when the object is emitted:

| Field | Meaning |
|---|---|
| `other_roles_open` | the count of the non-PASS rows in condition 2 |
| `open_roles` | the ids of the roles holding those rows, each id once, ascending; `[]` when every such row carries no role |
| `unattributed_open` | how many of those rows carry no role; `0` when none do |
| `message` | the sentence below |
| `hint` | the decision the driver now owns, quoted in §2.6 |

No field of `divergence` is ever omitted, and none uses absence to mean zero. §2.3 forbids that collapse for `spec_coverage_clean_rounds`, and an object whose readers must branch on key presence to read one of five fields is not the self-sufficient object the rest of this section relies on.

Conditions 1 and 3 mirror the two terms of `engine.Converged` — the clean-round count and the staleness guard — with the count taken over `spec-coverage`'s rows instead of over the stored per-round `clean` flags. The two are not equivalent, and this spec does not claim they are. `engine.ConsecutiveClean` walks the stored per-round `clean` boolean, which no later event changes: a round recorded with zero rows stores `clean: true` and still ends every role's streak, and a round recorded clean whose file is later deleted does the same. In both, convergence arithmetic counts a clean round that the streaks do not, so a history can converge without ever firing `divergence`. A round recorded *with* findings and later stripped of its file is not an example — its stored flag is already false, so it stops both. The asymmetry runs one way only, which is the safe way: the signal is withheld, never invented.

Condition 3 is exactly `StateStale`'s test and nothing more: it compares the latest recorded round's hash against the spec on disk. A spec edited mid-streak, with further rounds recorded after the edit, therefore passes it — the streak spans the edit and the signal still fires. That is deliberate, because it is the same term `engine.Converged` uses: the signal is neither stricter nor looser than the gate it is reported beside, and a divergence report can never claim more than convergence would have. Condition 3 constrains `--status` only; on `--record` the round is stored with the spec hash computed in the same invocation, so `stale` is false there by construction. The condition is evaluated the same way on both and must not be wired to a different staleness source on either.

`open_roles` names *which* lenses hold the open findings, so the decision §2.6 hands to the driver can usually be made from `divergence` alone, including under `--compact` where `role_streaks` is absent. When every open row carries no role it is `[]` and `unattributed_open` equals `other_roles_open`; the object then names no lens, and `message` says so rather than leaving the empty array to be interpreted.

`other_roles_open` is kept even though condition 1 makes it arithmetically equal to the latest round's total non-PASS count, which both outputs already carry — as `findings` on `--record` and `audit_rounds[last].findings` on `--status`. That equality holds only while condition 1 holds, and a reader who has to know that to interpret the object does not have a self-sufficient object.

A row with no role is counted in `other_roles_open` because it cannot be shown to belong to another role — but neither can it be shown to belong to `spec-coverage`, so the count alone would let the message assert clean conformance while a possible spec violation sits unattributed in the same round. `unattributed_open` records it, and `message` carries it inline so the sentence is never read without it.

The message is one of these two, with `round`/`rounds` and `finding`/`findings` agreeing with their numbers. The round count is `spec_coverage_clean_rounds`, never the threshold it was compared against; the finding count is `other_roles_open`; the parenthetical count is `unattributed_open`:

```
spec-coverage clean 9 rounds; 24 findings open from other roles
spec-coverage clean 9 rounds; 24 findings open from other roles (including 3 with no role, which may be spec-coverage's)
```

The second form is used exactly when `unattributed_open` is greater than 0. The sentence restates numbers that sit in sibling fields, and that is deliberate: in v0.32.0 every number was already on screen for eight rounds and the conclusion was still not drawn.

When any condition fails, `divergence` is absent. Absence means only that the three conditions did not all hold.

### 2.5 Where the signal appears, and what `--compact` keeps

`spec_coverage_clean_rounds`, `role_streaks` and `divergence` appear on `tp audit <spec> --status`, with or without `--check`, and on `tp audit <spec> --record <file>`.

`--check` changes only the exit code, never the payload: the fields are computed and written before the exit-code branch, so `tp audit <spec> --status --check` carries the same three fields it carries without the flag. That is the invocation a gated driver actually runs, and a signal absent from it would be absent from its only audience.

On `--record` the signal is computed **after** the round is stored, so "the latest recorded round" is the round just recorded — the convention `harness_stale` already follows on that path. Carrying `role_streaks` there as well as on `--status` is what keeps the panel one call away from the driver that just recorded the round, which is the round-trip the two-call architecture exists to remove. `--record` also has a pre-recording refusal: an exhausted `audit_max_rounds` exits 4 with the escalation hint before any parse or state write, so no payload and no signal is emitted on that path. A driver in that state reads the signal from `--status`, which has no such refusal.

`--compact` treats the three fields as follows, and this table is the rule for them:

| Field | `--compact` | Why |
|---|---|---|
| `spec_coverage_clean_rounds` | kept, key always present, `null` value included | the number the release exists to surface |
| `divergence` | kept whole, every field | see below |
| `role_streaks` | dropped | per-role detail behind the decision, and the object carries its own attribution in `open_roles` |

`divergence` keeps `message` and `hint` under `--compact` even though both restate what sits elsewhere, which is the rule tp otherwise uses to drop a field. `--compact` is the mode an agent driver runs in, and §1's finding is precisely that the numbers were on screen for eight rounds while the conclusion was not drawn — stripping the two strings from the compact payload reproduces that failure in exactly the audience the release is for. The cost is bounded: `divergence` is emitted only when its three conditions hold, so the bytes are spent on the rounds where the conclusion matters and on no others.

`tp audit --merge` reads loose NDJSON files with no recorded-round history and emits none of these three fields. `tp review` emits none of them either (Non-Goal 3). §3's `mechanized_classes` is a `tp review` field and its `--compact` disposition is stated in §3.3.

### 2.6 The gate is untouched

`engine.Converged`, `engine.ConsecutiveClean`, the stored per-round `clean` flag, and the exit code of `tp audit --status --check` are unchanged by this release. A round holding only non-`spec-coverage` findings is still not clean and still does not count toward convergence. `next_action` keeps its three-state audit precedence and gains no divergence branch (Non-Goal 4), and `tp resume` is unchanged (Non-Goal 10).

The `hint` is this constant, emitted verbatim:

```
spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Decide explicitly whether they gate this release — audit convergence still counts every non-PASS row.
```

That sentence is the release's contract with its driver: tp reports the divergence and names who decides. It does not decide, and it offers no way to clear the gate on the driver's behalf (Non-Goal 8).

## 3. A registered check retires its mechanize candidate

### 3.1 One definition

An entry in the effective workflow's `checks` is **valid** when `engine.ValidateChecks` accepts it **on its own** — the single-entry call `runMechanicalChecks` already makes for each entry it runs. Validity is judged per entry, never over the slice, so one invalid entry never changes the treatment of another entry's class, in either direction. The validator's duplicate-class rule is cross-entry and is therefore structurally unreachable in this form; a class named by two entries is simply registered, and both consumers in §3.2 name it once.

A finding class is **mechanized** when a valid `checks` entry's `class` equals it exactly. That is the release's single meaning of the term, and §3.2's two uses both take it unchanged.

An entry the validator rejects does not mechanize its class: it neither retires the suggestion that asked for a check to be written, nor tells a reviewer to stop reporting the class. The grounding is what registration is evidence of, not whether any particular command runs the check — `tp review --record` executes no check at all, and a valid entry mechanizes there anyway. An entry tp will never run is not evidence that the class is mechanically checked.

Neither an invalid entry nor a duplicated class is unreachable, and this release adds no advisory for either, because two channels already carry them. `tp set --workflow checks=` and `tp set --workflow --project` validate the whole slice and reject both, but `tp validate` reports them as a **warning** — the message is `invalid check entries are skipped at execution time` — and exits 0, and `tp import` accepts a task file carrying them. So the state arrives through import or a hand edit, `tp validate` names it, and `runMechanicalChecks` already emits `skipping invalid check <i> (<class>): <err>` through `output.Notice` wherever it runs. That existing notice is unchanged by this release and is not deduplicated against anything added here.

### 3.2 Where it applies

**Candidate suppression.** A mechanized class is excluded from `mechanize_candidates`. The frequency threshold — a class in at least 2 distinct rounds, or at least 5 times in one round — is unchanged, and the observable rule is that suppressing one class never changes whether another class crosses it. Two modes:

1. `tp review <spec> --record <file>` — filters `mechanize_candidates`, the register-a-check `hint`, and the class list handed to `next_action`.
2. `tp review <spec> --status [--check]` — filters the class list handed to `next_action`. This mode emits no `mechanize_candidates` array of its own, and derives its class list from the recorded rounds by a separate call, so it is a genuinely distinct sink rather than a projection of mode 1's array.

Two consequences, of which the second is the one that matters:

1. When exclusion empties the candidate list, the accompanying register-a-check `hint` is not emitted, because that hint is already conditional on a non-empty list.
2. `next_action` does not name a mechanized class. When branches 1 and 2 of its precedence do not apply and every candidate is mechanized, the state falls through to the run-the-next-round branch, and the driver is no longer told to write a check that exists.

Whether the registered check **passes** is irrelevant: registration is the trigger. A failing check is already reported in `mechanical_checks` and already gates the exit code of `tp review --status --check`; suppressing the suggestion does not weaken that.

**The reviewer exclusion list.** Prompt emission appends `Mechanically checked classes — do NOT report findings of these classes:` followed by the mechanized classes. Two changes to its membership, and no others: an entry the validator rejects is dropped, and a class named by more than one entry appears once, keeping the first occurrence in registration order. The surviving order is otherwise unchanged — registration order, not sorted — and it is not the `mechanized_classes` order of §3.3, which is a different list with its own rule. A registered class that never reached candidate frequency stays on the list exactly as today, because this list is about what a reviewer should stop reporting, not about what tp is still suggesting. When the drop empties the list, the sentence is not appended at all.

That list is assembled at **two** sites and both change: the multi-role panel emission in `internal/cli/review.go`, and the standalone `tp review <spec> --perspective regression` path in `internal/cli/review_regression.go`. Guarding one leaves the other on the old membership rule, which is the failure this repository already recorded as "guard the value at the sink, not at the entry point".

`tp audit` surfaces no mechanize candidates and is unaffected. `tp review --report` is out of scope (Non-Goal 9).

### 3.3 What was suppressed stays visible

`mechanized_classes` accompanies `mechanize_candidates` on `tp review <spec> --record <file>`, carrying the candidate classes withheld because they are mechanized, each once, sorted ascending. It is emitted only when non-empty, and it is absent on `tp review --status`, which emits no candidate array for it to explain.

It survives `--compact` because `mechanize_candidates` does — `--compact` has never stripped that array, and stripping one half of a list and its withheld remainder would misreport the round rather than shorten it.

It lists the intersection, not every registered class: the registered set is what the driver wrote and can read back from `tp config` and `tp review --status`, so restating all of it beside the candidate list would spend tokens on configuration rather than on what changed. Without this field a class simply vanishes from the output on the round after it is registered, which reads as a bug.

## 4. Documentation

Four documents change, and each requirement is an exact substring so the guard test asserts the document rather than the implementer's choice of anchor.

`README.md` and `skills/tp/SKILL.md` must each contain both:

- `audit convergence still counts every non-PASS row`
- `a registered check retires its mechanize candidate`

`CLAUDE.md`'s audit-scope rule tells the driver to track `spec-coverage` by hand; it must now name the field that carries the number, by containing:

- `tp audit now reports spec_coverage_clean_rounds`

`skills/tp/REFERENCE.md` is the field-level reference and carries the shapes and the rules a reader cannot derive from them. It must contain these five sentences verbatim:

1. `A role with no rows in a round is not clean in it, so its streak ends.`
2. `spec_coverage_clean_rounds is null, not 0, when the latest recorded round holds no spec-coverage row.`
3. `role_streaks is omitted under --compact; spec_coverage_clean_rounds and divergence survive it, divergence with every field.`
4. `mechanized_classes names the candidate classes withheld because they are mechanized.`
5. `spec-coverage is the only role that measures spec conformance; the remaining findings are outside it. Decide explicitly whether they gate this release — audit convergence still counts every non-PASS row.`

Sentence 5 is the `divergence.hint` constant of §2.6. It is pinned in a committed document so the code constant, the documentation and the guard test cannot drift apart — not because a reader would otherwise lack it, since §2.5 keeps the hint under `--compact`. It contains the first required substring, which is why `REFERENCE.md` is not additionally asked for the two summary strings.

## 5. Non-Goals

1. **A `scope` field on audit rows.** Classifying a finding as `spec` or `codebase` is a judgement, and the only party positioned to make it is the sub-agent that wrote the row. Wired into the gate, one row mislabelled `codebase` lets a genuine spec violation ship — strictly worse than today's rule, which over-counts and therefore only wastes rounds. The signal in §2 needs no such field: `spec-coverage`'s streak is derived from routing tp already owns.
2. **`audit_converge_on`.** It has no meaning without item 1. Deferred to v0.34.0 with the evidence this release produces: if the divergence signal turns out to be enough for a driver to make the call, the policy is not needed.
3. **The same signal for `tp review`.** A reviewer commenting on code style rather than the spec has the same shape, but review already has a severity-aware predicate and no reviewer role monopolizes conformance the way `spec-coverage` does. There is no equivalent named streak to report.
4. **A divergence branch in `next_action`.** `next_action` names one step; making divergence one of them makes it advice rather than reporting, and §1's whole argument is that the decision is the driver's.
5. **Auto-resolving or parking a non-`spec-coverage` finding.** tp has no audit-side `--resolve`. Adding one is how a round of open findings quietly becomes a clean round.
6. **Per-role convergence thresholds.** One `audit_clean_rounds` still governs the whole phase.
7. **Retiring a mechanize suggestion because a check passes.** Registration is the trigger (§3.1). A pass-conditioned rule would make the suggestion reappear whenever the check went red, which is the moment the class is least in need of a second check.
8. **Recording the driver's acceptance of a divergence.** Anything tp stores that then clears the gate is `audit_converge_on` under another name, and it would arrive without the policy debate item 2 defers. The artifact today is a decision document the driver writes and commits beside the recorded rounds, which tp neither requires nor parses; `--harness-note` remains available for framing that belongs on a round. `tp audit --status --check` keeps exiting 1 while the sequence is not converged, and a CI wired to it stays red — which is the honest report of the recorded state.
9. **Suppression in `tp review --report`.** Every other mode takes the spec as a positional, and `engine.ResolveWorkflow` uses it as an anchor: a task file found through the ambient chain is kept only when its `spec` field resolves to that argument, and otherwise the spec-adjacent `<base>.tasks.json` wins. The spec therefore pins which workflow applies except where two committed task files declare the same spec. `--report` takes NDJSON positionals and no spec, so it has no anchor at all and its workflow would be decided by the ambient chain alone — whose highest-priority stored layer, `.tp/local.json`, is git-ignored and machine-local. The same invocation over the same files would suppress different classes on a developer machine and in CI, which §1's second constraint forbids. `--report` keeps listing every candidate class, registered or not.
10. **The divergence signal in `tp resume`.** `tp resume` is the phase oracle and its audit-phase payload names the next round and its unresolved count; carrying the signal there would put the same conclusion behind a third shape to keep in sync, and the driver reaching the audit phase runs `tp audit --status` next anyway.
11. **Removing the register-a-check `hint` in favour of `next_action`.** On `tp review --record` the two say the same thing and `next_action` says it better — it names the class and fills in the command. The duplication predates this release, both are filtered identically by §3.2, and removing a documented output field is a contract change that belongs in its own version.

## 6. Tests

1. Per-role streaks: three recorded rounds where `spec-coverage` is all-PASS in every round and a second role has a FAIL in the latest one. `role_streaks` reports `spec-coverage` with `consecutive_clean` 3 and `open` 0, and the other role with `consecutive_clean` 0 and `open` 1.
2. A role absent from a round ends its streak: a role clean in rounds 1 and 3 but with no rows at all in round 2 reports `consecutive_clean` 1, not 3. This is the discriminating case against an implementation that skips unmeasured rounds instead of ending on them.
3. `role_streaks` covers only the latest round's roles: a role present in round 1 and absent from round 2 does not appear in the array reported at round 2.
4. Ordering: with `spec-coverage` plus two other roles in the latest round, `spec-coverage` is first and the remaining two follow in ascending id order — asserted with fixture ids chosen so alphabetical order alone would not put `spec-coverage` first.
5. Row-role predicate: rows whose `role` is absent, `""`, `"   "`, or a number create no `role_streaks` entry, and no entry with an empty id ever appears. A row with `"  spec-coverage  "` is attributed to `spec-coverage`, and one with `"Spec-Coverage"` is not — the two halves pin trimming without case folding.
6. Row-PASS predicate: rows with `status` of `"pass"`, `" PASS "`, a number, or no `status` key are each non-PASS, so a role holding one is not clean and its `open` counts it. The `" PASS "` case is the discriminating one against an implementation that trims `status` by symmetry with `role`.
7. The three no-rows causes each end every streak and emit their own advisory wording exactly once: a deleted file, a round entry with an empty `file`, and a file containing one unparseable line. The third is the discriminating case — an implementation reusing a loader that silently drops malformed lines keeps the round's surviving rows and fails it.
8. The walk stops at a streak-closing round: over five recorded rounds where round 2's file is deleted and round 4 holds a non-PASS row for every role present in round 5, the walk closes every streak at round 4 and never reads round 2, asserted by the absence of round 2's advisory. An implementation walking the whole history emits it and fails.
9. `role_streaks` is `[]` and `spec_coverage_clean_rounds` is `null` in all four states of §2.2: no recorded round; a latest round contributing no rows, instantiated both as a deleted file and as an empty `file` entry; a latest round recorded with zero rows; a latest round whose every row carries no role. Only the contributes-no-rows state emits an advisory; in the other three the field is the sole disclosure.
10. `spec_coverage_clean_rounds` is `null` when the latest round holds no `spec-coverage` row even though an earlier round does, and `0` when the latest round holds one that is not PASS. The first half is the discriminating one against an implementation that scans the whole history for the role.
11. `divergence` fires when `spec-coverage` has been clean for the effective `audit_clean_rounds` and another role holds an open finding. Its `message` matches `spec-coverage clean 4 rounds; 2 findings open from other roles` verbatim over a fixture whose streak (4) exceeds the threshold (2) — pinning that the round slot carries the streak and not the threshold — and its `hint` equals the §2.6 constant verbatim. A second fixture with one required clean round and one open finding asserts the singular `spec-coverage clean 1 round; 1 finding open from other roles`.
12. `open_roles` names exactly the non-`spec-coverage` roles holding open rows, ascending, each id once over a fixture where one role holds three open rows; it is `[]`, not absent, when every open row carries no role, and `unattributed_open` then equals `other_roles_open`.
13. `divergence` is absent when `spec-coverage`'s streak is below the threshold, and absent when it meets the threshold with no other role holding an open finding.
14. The threshold is the effective `audit_clean_rounds`, not a literal 2: with `audit_clean_rounds` set to 3 and `spec-coverage` clean for exactly 2 rounds alongside another role's open finding, `divergence` is absent; a third clean round makes it fire.
15. The threshold floor holds: with a task-file `audit_clean_rounds` of 0 reaching the resolver and `spec_coverage_clean_rounds` at 0, `divergence` is absent; at 1 it fires. An implementation comparing against the unclamped value fires in the first half and reports `spec-coverage clean 0 rounds`.
16. A stale spec suppresses `divergence` on `--status` even when conditions 1 and 2 hold — the spec file is edited after the last round is recorded, `stale` reads true, and `divergence` is absent.
17. A non-PASS row carrying no `role` is counted in `other_roles_open`, disclosed in `unattributed_open`, and named inline in `message` with the `(including 1 with no role, which may be spec-coverage's)` suffix. With every non-`spec-coverage` open row attributed, `unattributed_open` is `0` — present, not absent — and the suffix does not appear.
18. `--record` computes the signal over the just-recorded round: recording a round in which `spec-coverage` completes its streak emits `divergence` in that same invocation's output, without a following `--status`.
19. `tp audit --status --check` carries all three fields, asserted on a non-converged fixture that emits `divergence` and exits 1. An implementation computing the signal only on the non-`--check` path fails.
20. `--compact` on both `--status` and `--record` omits `role_streaks`, keeps the `spec_coverage_clean_rounds` key including when its value is `null`, and keeps every field of `divergence` — `message` and `hint` byte-identical to their non-compact values.
21. The gate is unchanged: a fixture whose latest round holds only non-`spec-coverage` findings still reports `clean: false` and `converged: false`, `tp audit --status --check` still exits 1, and `next_action` is still the fix-and-re-audit directive. This is the guard test for §2.6 and must fail any implementation that lets the divergence signal reach convergence arithmetic.
22. `tp audit --merge` emits none of `role_streaks`, `spec_coverage_clean_rounds`, `divergence`.
23. A registered check suppresses its class: a class over the candidate threshold with a matching `checks` entry is absent from `mechanize_candidates` on `--record`, while an unregistered class over the same threshold is still listed.
24. Validity is per entry: a `checks` array holding one entry the validator rejects and one valid entry over a candidate class suppresses the valid entry's class and leaves the rejected entry's class listed. An implementation validating the whole slice suppresses neither and fails this test.
25. A class named by two entries is mechanized, and appears once in `mechanized_classes` and once in the reviewer exclusion list — pinning both the per-entry predicate and the de-duplication of §3.1.
26. Suppressing every candidate removes the register-a-check `hint` from `--record` output.
27. `tp review --status` honours the suppression through `next_action` on the same fixture, with neither `mechanize_candidates` nor `mechanized_classes` in the output. This is the second sink for the `next_action` path, and an implementation that filters only `--record`'s emitted array still names the mechanized class here.
28. A registered check whose command fails still suppresses its class, asserted with a check whose command exits non-zero.
29. `mechanized_classes` lists the suppressed classes sorted ascending, is absent when nothing was suppressed, does not list a registered class that never reached candidate frequency, and survives `--compact` on `--record`.
30. Prompt emission drops an invalid entry from the `Mechanically checked classes — do NOT report findings of these classes:` list while keeping a valid one, and keeps a valid entry whose class never reached candidate frequency — pinning that this list is the mechanized set and not the suppressed-candidate set. Over three valid entries registered out of alphabetical order, the list keeps registration order, distinguishing it from `mechanized_classes`.
31. When every registered entry is invalid, prompt emission appends no such sentence at all, rather than one ending in an empty list.
32. Both prompt-emission sites are covered: the multi-role panel and `tp review <spec> --perspective regression` each drop the invalid entry. The second is the discriminating half — an implementation guarding only the panel path passes the first and fails this.
33. `tp review --report` is unchanged: a class registered in a resolvable task file still appears in its `mechanize_candidates`, pinning Non-Goal 9 against an implementation that extends suppression there.
34. A guard test asserts `README.md` and `skills/tp/SKILL.md` each contain both required substrings of §4, that `CLAUDE.md` contains `tp audit now reports spec_coverage_clean_rounds`, and that `skills/tp/REFERENCE.md` contains the five verbatim sentences.

### 6.1 Existing tests this change invalidates

Derived by running the search, not asserted. The tokens `computeMechanizeCandidates`, `mechanizeCandidateClasses`, `mechanizeClassesFromRounds`, `mechanize_candidates` and `runAuditStatus` over `internal/**/*_test.go` name the files that pin the shapes this release changes.

1. `internal/cli/audit_record_test.go` — asserts the `tp audit --record` and `--status` payload shape, including that no `mechanical_checks` key is present. The new fields are additive, so its existing assertions hold; it gains the new ones rather than being rewritten.
2. `internal/cli/review_record_test.go` and `internal/cli/nextaction_test.go` — pin the candidate list and the mechanize branch of `next_action` against workflows with no registered checks. Both stay valid; the new behaviour is a new case in each, and either of them turning red means suppression is firing where no check is registered.
3. `internal/cli/review_test.go` and `internal/cli/review_regression_test.go` — contain the prompt-emission assertions for the `Mechanically checked classes` list at the two sites of §3.2. Any case there registering an entry the validator rejects, or the same class twice, is invalidated; cases registering only distinct valid entries are not.
4. `internal/cli/report_class_test.go` — **not invalidated.** It asserts `mechanize_candidates` from `tp review --report`, which Non-Goal 9 leaves unchanged. Listed so the search's hit on `mechanize_candidates` is accounted for rather than silently dropped.
