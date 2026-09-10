# tp — The round knows which roles it expects

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `round-knows-its-panel-measurements.md` beside it; this file stands without them.
The two defects below were reproduced in a built fixture before they were described, and §1.1 is that
transcript — a reading of the code would have justified either one, and only the run distinguishes
them. §4a absorbs the role-scoped convergence half of the former rows spec, whose accepted-finding half
is `a-finding-can-leave-an-audit-round.md`.

Class: **loop** — it changes what `--check` reads and adds a write on the emission path, so it is
reviewed by the mechanism it changes. Budget it at the loop-class median in `CLAUDE.md`'s *What a
cycle costs* table.

## 1. Overview

`tp audit` computes the panel a round expects — it emits one prompt per active role and a
`skipped_roles` entry, with a reason, for each role it declined to emit — and then **throws that set
away**. Nothing is recorded with the round, so when `--record` stamps `clean` and `--check` reads it,
neither can tell *a role that reported nothing* from *a role that reported nothing wrong*.

Two consequences, in opposite directions, both real:

- **Too lax.** A round that receives one of three emitted roles stamps `clean: true`. Two such rounds
  make `tp audit --status --check` exit **0** — this project's ship signal — on a spec whose
  conformance role never ran and whose only task is still open. Measured, §1.1.
- **Too strict.** v0.36.0 shipped with `--check` at exit 1 while `spec-coverage` was clean in each of
  rounds 10–13, because a non-conformance role's rows override a clean conformance role — derivation
  in the sidecar under *The too-strict direction, derived*.

This release ships **one new fact — the panel a round expected — recorded where tp knows it**, the
use that closes the *too lax* half (§3), and the field that scopes convergence to the roles that decide
it (§4a), which depends on the panel and is the *too strict* half.

### 1.1 The transcript

A three-role corpus (`spec-coverage`, `go-safety`, `ax-contract`), one spec, one open task, one Go
file. `tp audit` emits three prompts. A round is then recorded carrying **only `ax-contract`'s rows**,
all PASS, and the same file is recorded a second time:

```
round: 1   clean: true   consecutive_clean: 1   converged: false
           spec_coverage_clean_rounds: null
           role_streaks: [{"role":"ax-contract","consecutive_clean":1,"open":0}]
round: 2   clean: true   consecutive_clean: 2   converged: true
           spec_coverage_clean_rounds: null
           role_streaks: [{"role":"ax-contract","consecutive_clean":2,"open":0}]

$ tp audit spec/demo.md --status --check   ->  exit 0
```

**Nothing was implemented and two thirds of the panel never ran.** `spec_coverage_clean_rounds: null`
and a one-entry `role_streaks` both report the absence correctly — the honest signals are already
there. `clean`, `consecutive_clean` and `converged` are the three that are blind, and `--check` reads
the blind ones. Why §6 row 1 asserts the exit code rather than each round's `clean` byte is in the
sidecar under *Only round 1 was opened by an emission*.

## 2. The round records the panel it expected

The recorded round gains **`expected_roles`** — the role ids `tp audit` emitted a prompt for, plus each
`skipped_roles` id paired with its reason — and **`received_roles`**, the distinct non-empty roles
among the recorded rows. §1.1 shows the per-role signals already reporting a role with no rows as
absent; recording the panel lets the round-level stamp make the same distinction.

**The panel is written at emission, because `--role` leaves no trace at record.** The corpus cannot
supply it — `roles_hash` is byte-identical under both narrowings this project produces, and under
`--role` the narrowed-away roles are named in no field and no file — so it is not re-derived at
record, which is a separate invocation that never saw the flag, and not on read. The two probes are in
the sidecar under *Why the panel cannot come from the corpus*.

### 2.1 The panel record

**A round is opened by more than one emission, so the panel is a union.** Three successive `--role`
emissions against one fixture each reported `output_path audit-r1-<role>.ndjson` and left
`in_flight_round: 1` — one round, emitted three times. The panel for round N is therefore the union
of the roles emitted for round N, with the declines accumulating the same way; one unnarrowed
emission contributes the whole corpus at once.

**A skipped role is expected-and-excused, not missing.** `spec-coverage` skipped with reason
`no-checklist-items` is a fact about the task file, not a role that failed to report, so it is
recorded with its reason and §3 does not count it against the round. The reason is what makes the two
separable, which is why the record is not a bare id list.

**A round with no recorded panel is "unknown", not "empty".** That covers every round recorded before
this release and any round recorded without an emission having opened it: §3's rule cannot fire on
one, and upgrading tp never retroactively unconverges a shipped release.

### 2.2 Unknown `state.json` keys — moved

That `state.json` round-trips keys it does not understand is a precondition of this release, not part
of it: the panel is an optional key a pre-fix `--record` would erase. It moved on 2026-09-11 to
`a-findings-exits-agree`, which ships first; its measurements stay in this spec's sidecar under *Three
fields shipped erasable* and *The trigger is a stray `--record`, and what the fix does not buy*.

## 3. A round missing an expected role is not clean

**A round is clean only if every expected, non-skipped role appears in `received_roles` and the
round's rows satisfy the severity policy.** The severity half is unchanged and `audit_converge_on`
still governs it, and closing this must not change what any complete round already stamped.

**The hole is orthogonal to severity, and that is measured rather than argued.** A single-row round
carrying one `PASS` from one of three expected roles stamps `clean = true` under **both** policy
values:

```
audit_converge_on=all       -> clean=true
audit_converge_on=blocking  -> clean=true
```

So no setting of the severity axis closes it. `CLAUDE.md` records v0.37.0's review reaching the same
conclusion about the *too strict* direction from the other side — severity parity does not close a
*role*-scoping gap — and it is why the panel is a separate input rather than a stricter reading of the
rows tp already has.

`clean` stays stamped once, at record time, and is never re-graded; the panel it reads was written by
the emission one step earlier.

**What this deliberately does not catch.** An operator who emits one role and records that one role
has a complete round by construction — panel and received are both that role — and this rule will call
it clean. The gap it closes is between what tp *emitted* and what came *back*, which is §1.1 exactly.
Whether a one-role panel should gate a release is `spec_coverage_clean_rounds`'s question and
`CLAUDE.md`'s shipping rule, not this one's.

**This makes `--check` stricter, and that is the whole point of this half.** The failure it closes is a
ship signal firing on an audit that did not happen, and the conservative resolution of "did this role
report?" is the one that keeps auditing. The opposite direction — a correct round held back by a role
that does not decide the question — is §4a's, and shipping this half alone leaves it open.

## 4. In-flight progress — cut

Cut on 2026-09-11: per-role progress on `--status` for a round not yet recorded has no measured cost
behind it, and its three test rows went with it. The design and its two file-resolution worlds stay in
the sidecar under *§4 in-flight progress (cut)*.

## 4a. `audit_converge_roles` — the roles whose rows gate

**Precondition: before this section costs a cycle, check whether the shipped
`audit_converge_on: blocking` already suffices.** It closes one of the two shipped releases' replays
and, replayed on `v1.0.0`'s audit, converges well before that audit's last round (sidecar, *Would
`blocking` have sufficed?*). If it suffices for the cycles this project runs, §4a is not built and §2–§3
ship alone.

**Depends on §2's `expected_roles`.** This is the *too strict* direction §1 names, moved here from the
rows spec; nothing in this section can be built before §2.

`audit_converge_roles` — a workflow field listing the role ids whose rows gate convergence.

**Default: unset, and unset means every row gates, exactly as today.** The default is stated over
*rows* rather than over roles because that is what was measured: at `HEAD` a `FAIL` row with an
unrecognised role gates, and a `FAIL` row with **no `role` key** gates while appearing in no per-role
signal (sidecar, *Two rows that gate while appearing in no signal*). A default phrased as *"every
expected role"* would therefore ship two silent relaxations before anyone narrowed anything, the
second the reverse of the fail-closed rule the sibling field keeps — *a row tp cannot grade is a row tp
must not stop counting*. So: **when the list is set, a row gates only if its `role` is in the list.**
A row with no `role` key gates under every setting, including a set one; it is not a role and cannot
be listed out.

**Role is the missing axis, on the evidence of two replays and no more.** Two releases shipped with
`--check` at exit 1 while `spec-coverage` held no non-PASS row, and in every masking round the rows
belong to a non-conformance role. Severity parity closes one of the two and not the other — the
replay is in the sidecar under *What the two shipped releases actually measure* — so the general claim
that severity does not close this is not made here; what both releases share is the role axis.

**Fenced under `TP_UNATTENDED=1` like `audit_converge_on`**, because a set list relaxes a gate. At the
task layer, `tp import` and `tp config --extract`, the fence refuses a write that **changes what
resolves**, as the shipped field's does — a value rule there would deadlock `tp import`, which carries
a resolved block forward (sidecar, *The shipped fence at four sinks*). **The `--project` sink's rule
is open and this file does not settle it**: there is no relaxing literal for a list, whether a list
narrows is decidable only against a per-base `expected_roles`, and the two candidates — refuse every
non-empty list at `--project`, or refuse only a list that is not a superset of every base's panel —
are named for the review round; the fenced release's own history is that an under-determined
`--project` rule cost it four audit rounds.

**A role listed but not expected is an error, not a silent no-op.** `audit_converge_roles: ["typo"]`
would otherwise converge on an empty gating set — the failure mode of every allow-list that is not
checked against its universe, and nothing catches it today.

**A narrowed panel does not silence anyone.** Non-gating roles still emit, still record, still appear
in `role_streaks` and `--merge`. They stop deciding whether the loop continues. tp's audit-side merge
does not collapse the role axis in the first place — it dedups on `(role, item_id)` — and nothing in
the merge surface is changed here (sidecar, *The separation is the point*).

## 5. Non-Goals

1. **No workflow field beyond §4a's `audit_converge_roles`.** `audit_converge_on` is unchanged and
   still governs severity; §2 adds a recorded fact, not a knob.
2. **No new disposition, and no change to what a disposition does.** A finding accepted with evidence
   is `a-finding-can-leave-an-audit-round.md`'s subject; §4a scopes rows by role, not by
   `resolved`.
3. **No retroactive effect.** A round with no recorded panel — recorded before this release, or
   recorded without an emission having opened it — keeps today's semantics and cannot be un-converged
   by upgrading.
4. **The review phase is unchanged.** `ReviewConsecutiveClean` has its own convergence policy and its
   own field; widening this to review doubles the surface for a defect measured only on the audit
   side. It also keeps `--no-state` out of scope: that flag is declared on `tp review` only, so no
   audit invocation can suppress the emission-time write §2 adds.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. Rows 4b and 4c moved with §2.2 to `a-findings-exits-agree`; rows 8–10 were cut with §4.
The remaining numbers are unchanged.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1.1 | §1.1's fixture rebuilt with an **emission before each** of two rounds carrying 1 of 3 roles, **then a third `--record` with no emission**: `--check` exits **non-zero** after the third, and `consecutive_clean` is at most 1. The exit code is asserted, not each round's `clean` byte, because the third round has no recorded panel and stamps as it does today. **This row is the acceptance and must be watched failing against `HEAD` first** — a test that has never been observed red may be asserting a tautology and would pass identically either way | the shipped behaviour, which exits 0 after the second round; and a fix that treats the two missing-role rounds as unknown rather than not-clean, which ground round 2 measured passing the two-round form of this fixture — the panel-less third record is what separates the two |
| 2 | §2 | `expected_roles` records an emitted role and a skipped role **with its reason**, distinguishably | store a flat id list, which makes `no-checklist-items` indistinguishable from a role that never reported |
| 2b | §2 *emission* | the panel recorded for a round emitted with `--role` is exactly the roles emitted, and three successive `--role` emissions of one round union to the full panel | keep only the last emission's roles, so a round in which one role never reported still stamps clean |
| 2c | §2 *not the corpus* | with one role deactivated in the spec frontmatter, the panel records it declined-with-reason **while** `roles_hash` is byte-identical to the unnarrowed corpus's; the same fixture under `--role` records a one-role panel with `skipped_roles` empty | derive the panel from `roles_hash` equality — byte-identical in both cases, so it over-states the panel in the two narrowings this project actually produces |
| 3 | §2 *pinned* | deleting a role file after a round is recorded does not change that round's `expected_roles` | re-derive from the corpus on read, so editing `.tp/auditors/` rewrites history |
| 4 | §2 *legacy* | a round with no `expected_roles` key — recorded before this release, or recorded with no emission having opened it — stays clean and converged exactly as before | treat a missing key as an empty set, which un-converges every shipped release on upgrade |
| 5 | §3 | a complete round's `clean` stamp is **byte-identical** before and after this release, across both `audit_converge_on` values | apply the panel rule to the severity computation, changing rounds this release must not touch |
| 6 | §3 *skipped* | a round missing only a role recorded as skipped-with-reason is clean | count a skipped role as missing, which makes a task file with no `source_sections` permanently unconvergeable |
| 7 | §3 *one-sided* | a round carrying a role not in `expected_roles`, and every expected role besides, is clean — the rule asks that the expected roles arrived, not that nothing else did. Built fixture: no recorded round carries a surplus role | fail the round on a surplus role, which makes a mis-scoped panel unreportable |
| 11 | §4a | with `audit_converge_roles: ["spec-coverage"]`, a round where only a non-listed role holds non-PASS rows converges — **replayed on v0.36.0 r12–13 and v0.37.0 r6–7, both of which exit 1 today under `all`** (sidecar, *What the two shipped releases actually measure*) | ignore the list, reproducing the two releases that shipped at exit 1 |
| 11b | §4a *default* | with `audit_converge_roles` unset, a row with an unrecognised role **and** a row with no `role` key both still gate | default to `expected_roles`, which relaxes both cases before anyone narrows anything |
| 12 | §4a *fence* | the task layer, `tp import` and `tp config --extract` refuse only a write that changes what resolves; the `--project` rule is open and its row cannot be written until §4a's decision is settled in review | apply a value rule at `tp import`, which deadlocks it by refusing a block it merely carries forward |
| 13 | §4a *unknown* | a listed role absent from `expected_roles` is an error | treat it as a no-op, converging on an empty gating set |
