# tp — The round knows which roles it expects

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `round-knows-its-panel-measurements.md` beside it; this file stands without them.
The two defects below were reproduced in a built fixture before they were described, and §1.1 is that
transcript — a reading of the code would have justified either one, and only the run distinguishes
them. §4a absorbs the role-scoped convergence half of what was the rows spec
(`02b-what-a-rounds-rows-say.md`).

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

This release ships **one new fact — the panel a round expected — recorded where tp knows it, and two
uses of it** (§3, §4), plus the field that scopes convergence to the roles that decide it (§4a),
which depends on the panel and is the *too strict* half.

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
the blind ones.

**One detail of this transcript matters for §2 and is easy to miss: only round 1 was opened by an
emission.** Recording the same file twice produced round 2 with no emission in between, and the state
directory afterwards holds `snapshot-audit-round-1.md` and no round-2 snapshot. Under §2 that second
round therefore has no recorded panel and stays *unknown*, so §3 fires on round 1 alone — which is
enough to break the streak and take `--check` off zero. That is why §6 row 1 asserts the **exit code**
rather than each round's `clean` byte, and why its fixture emits before every round it wants §3 to
see.

## 2. The round records the panel it expected

`ReviewRound` gains **`expected_roles`** — the role ids `tp audit` emitted a prompt for, plus each
`skipped_roles` id paired with its reason — and **`received_roles`**, the distinct non-empty roles
among the recorded rows.

The producer already exists on both sides. `runAudit` builds `prompts` and `skipped_roles`
(`internal/cli/audit.go`), and `auditRoundOpenByRole` (`internal/engine/rolestreaks.go`)
already tallies a round's rows by role id and already treats a role with no rows as *not measured*
rather than *clean*. **This release does not invent that distinction; it records the other half of it,
so the round-level stamp can make it too.**

**The panel is written at emission, because `--role` leaves no trace at record.** The corpus cannot
supply it — `roles_hash` is byte-identical under both narrowings this project produces, and under
`--role` the narrowed-away roles are named in no field and no file — so it is not re-derived at
record, which is a separate invocation that never saw the flag, and not on read. The two probes are in
the sidecar under *Why the panel cannot come from the corpus*.

### 2.1 The panel record

**The panel file is a sibling of the snapshot tp already writes.** An emission writes the round's
spec snapshot into the state directory (`snapshot-<phase>-round-N.md`) and records nothing about the
panel; the panel record goes beside it, one per round, written atomically for the same reason the
snapshot is — `--status` reads it lock-free. `--record` copies it onto the round entry as
`expected_roles`, which is what `--check` and a re-read of history consume; §4's `in_flight` reads the
file itself, because the round it describes is not recorded yet.

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
one, and upgrading tp never retroactively unconverges a shipped release. This is the same
conservative direction `RolesStale` (`internal/engine/reviewstate.go`) takes on a pre-v0.25.0 round
with no stored `roles_hash`.

### 2.2 `state.json` preserves keys it does not understand

Optional keys on the round entry are erasable today, and three shipped fields have been erasable since
they landed — the sidecar's *Three fields shipped erasable* names them. Measured on `HEAD`:

```
inject expected_roles + spec_moved_mid_round onto round 1   -> present
tp audit <spec> --record <file>                             -> both GONE
inject a top-level reconciliations[]                        -> present
tp audit <spec> --record <file>                             -> GONE
```

`SaveReviewState` marshals a typed struct (`internal/engine/reviewstate.go`), and `ReviewState` has
exactly three fields, so **any key it does not know is dropped on the next write**. Reading is safe — nothing calls
`DisallowUnknownFields` — which is what makes the loss silent.

**So `ReviewState` and `ReviewRound` round-trip unknown keys**: unmarshal them into a sibling map,
merge them back on save, and let a typed field win where both exist.

The trigger is a stray `--record` and only that, and the fix buys durability against binaries at or
after this release and nothing against an older one — both measured, in the sidecar under *The trigger
is a stray `--record`, and what the fix does not buy*. The two releases that depend on this fix state
the dependency in their own text; this one only has to ship first.

## 3. A round missing an expected role is not clean

`AuditRowsClean` gains the panel: **a round is clean only if every expected, non-skipped role appears
in `received_roles` and the round's rows satisfy the severity policy.** The severity half is unchanged
and `audit_converge_on` still governs it, and closing this must not change what any complete round
already stamped.

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

`clean` is stamped at record time and never re-graded, which this release preserves. The new input is
not produced at that moment — §2 writes it at emission, one step earlier — but it is *available* there,
and reading a file the emission already wrote is what keeps the stamp a single-shot decision.

**What this deliberately does not catch.** An operator who emits one role and records that one role
has a complete round by construction — panel and received are both that role — and this rule will call
it clean. The gap it closes is between what tp *emitted* and what came *back*, which is §1.1 exactly.
Whether a one-role panel should gate a release is `spec_coverage_clean_rounds`'s question and
`CLAUDE.md`'s shipping rule, not this one's.

**This makes `--check` stricter, and that is the whole point of this half.** The direction matters:
the failure it closes is a ship signal firing on an audit that did not happen, and the conservative
resolution of "did this role report?" is the one that keeps auditing. The opposite direction — a
correct round held back by a role that does not decide the question — is §4a's, and shipping this
half alone leaves it open.

**A round that receives a role it did not expect is clean-eligible and says so.** `received_roles`
carries the id, `role_streaks` already reports it, and the round is not failed for it. The reason is
that a surplus role is the only signal a panel is mis-scoped, and failing the round would delete it.

## 4. `--status` reports the round in flight

`--status` already **names** the round being worked: `in_flight_round` is a top-level key of the
payload, backed by `engine.InFlightRound`, which returns the next round number when that round's
snapshot exists (measured: `4` after an un-recorded emission on the §1.1 fixture). What it does not
carry is any per-role progress, so the interactive fallback — where the orchestrator, not `tp run`,
spawns the agents — cannot ask how far the round has got. Under `tp run` the driver knows; outside
one, nobody does.

**`--status` gains an `in_flight` object hung off that existing round number** — not a second,
differently-named round pointer beside it: the panel §2's emission recorded for the round, and per
role whether its output file is present and how many rows it holds. The panel comes from the
emission's own record for exactly §2's reason — the corpus in force is not the panel — and here there
is no recorded round to fall back on. When no emission has opened the round, `in_flight_round` is
null and the object is absent.

tp already names the file. `roleOutputPath` (`internal/cli/prompt_framing.go`) returns
`<phase>-r<round>-<role>.ndjson`, the prompt's `output_path` and the name written into the prompt
body, and the clause tp appends to every role prompt (`incrementalClause`,
`internal/cli/clauses.go`) already instructs the role to *"write each row to the output file as you
decide it, not once at the end"*. **A partially written file is therefore the expected state of a running
role, not a corruption**, and row count is the progress signal.

**Two resolutions, because there are two worlds, and the release states both rather than assuming
one.** Inside a run `TP_ROUND_DIR` is set and the path is
`$TP_ROUND_DIR/role-<role>.ndjson.part`, renamed by the driver on exit 0 — so `.part` present means
running, final name present means finished, and the two are distinguishable. Outside a run the prompt
names a bare filename relative to wherever the operator is standing, which tp does not control:
`--status` looks in the current directory and in the round directory, reports which of the two it
found each file in, and reports a role as `unknown` rather than `not-started` when it found neither.

**`unknown` is not `not-started`, and conflating them would be the defect this release is about.**
tp cannot see a file written into a directory it was not told about; reporting that as "this role has
not begun" is the same class of error as stamping a round clean because a role's rows never arrived.

## 4a. `audit_converge_roles` — the roles whose rows gate

**Depends on §2's `expected_roles`.** This is the *too strict* direction §1 names, moved here from the
rows spec; a search for `expected_roles` or `ExpectedRoles` across `internal/` returns zero matches
at `HEAD`, so nothing in this section can be built before §2.

`audit_converge_roles` — a workflow field listing the role ids whose rows gate convergence.

**Default: unset, and unset means every row gates, exactly as today.** The default is stated over
*rows* rather than over roles because that is what was measured: at `HEAD` a `FAIL` row with an
unrecognised role gates, and a `FAIL` row with **no `role` key** gates while appearing in no per-role
signal (sidecar, *Two rows that gate while appearing in no signal*). A default phrased as *"every
expected role"* would therefore ship two silent relaxations before anyone narrowed anything, the
second the reverse of the fail-closed rule `auditclean.go` states for the sibling field — *a row tp
cannot grade is a row tp must not stop counting*. So: **when the list is set, a row gates only if its
`role` is in the list.** A row with no `role` key gates under every setting, including a set one; it
is not a role and cannot be listed out.

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
3. **`--status` does not spawn, wait, poll or time anything.** It reads files and returns. A progress
   report that blocks is a driver, and tp already has one.
4. **No retroactive effect.** A round with no recorded panel — recorded before this release, or
   recorded without an emission having opened it — keeps today's semantics and cannot be un-converged
   by upgrading.
5. **The review phase is unchanged.** `ReviewConsecutiveClean` has its own convergence policy and its
   own field; widening this to review doubles the surface for a defect measured only on the audit
   side. It also keeps `--no-state` out of scope: that flag is declared on `tp review` only
   (`internal/cli/review.go`), so no audit invocation can suppress the emission-time write §2 adds.
6. **No count of `PASS` rows carrying a note.** The rows spec proposed one and it was dropped because
   it measures a constant: every role-carrying `PASS` row in the recorded corpus carries a note, so the
   counter would equal the role's `PASS` count in every round tp has recorded. The derivation is in the
   sidecar under *PASS rows carrying a note (dropped counter)*; the underlying gap — a `PASS` whose
   note names a defect — is prose a reader has to open the row to see, and `CLAUDE.md` carries the
   reading rule.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1.1 | §1.1's fixture rebuilt with an **emission before each** of two rounds carrying 1 of 3 roles, **then a third `--record` with no emission**: `--check` exits **non-zero** after the third, and `consecutive_clean` is at most 1. The exit code is asserted, not each round's `clean` byte, because the third round has no recorded panel and stamps as it does today. **This row is the acceptance and must be watched failing against `HEAD` first** — a test that has never been observed red may be asserting a tautology and would pass identically either way | the shipped behaviour, which exits 0 after the second round; and a fix that treats the two missing-role rounds as unknown rather than not-clean, which ground round 2 measured passing the two-round form of this fixture — the panel-less third record is what separates the two |
| 2 | §2 | `expected_roles` records an emitted role and a skipped role **with its reason**, distinguishably | store a flat id list, which makes `no-checklist-items` indistinguishable from a role that never reported |
| 2b | §2 *emission* | the panel recorded for a round emitted with `--role` is exactly the roles emitted, and three successive `--role` emissions of one round union to the full panel | keep only the last emission's roles, so a round in which one role never reported still stamps clean |
| 2c | §2 *not the corpus* | with one role deactivated in the spec frontmatter, the panel records it declined-with-reason **while** `ComputeRolesHash` is byte-identical to the unnarrowed corpus's; the same fixture under `--role` records a one-role panel with `skipped_roles` empty | derive the panel from `roles_hash` equality — byte-identical in both cases, so it over-states the panel in the two narrowings this project actually produces |
| 3 | §2 *pinned* | deleting a role file after a round is recorded does not change that round's `expected_roles` | re-derive from the corpus on read, so editing `.tp/auditors/` rewrites history |
| 4 | §2 *legacy* | a round with no `expected_roles` key — recorded before this release, or recorded with no emission having opened it — stays clean and converged exactly as before | treat a missing key as an empty set, which un-converges every shipped release on upgrade |
| 4b | §2.2 | a state file carrying an unknown top-level key **and** an unknown key on a round entry keeps both across a `--record` | marshal the typed struct alone — the shipped behaviour, measured to drop both, and this must be watched failing against `HEAD` first |
| 4c | §2.2 *precedence* | where a typed field and a preserved unknown key share a name, the typed field wins and the stale copy does not resurface | merge the map last, letting a stale preserved value overwrite what this release just computed |
| 5 | §3 | a complete round's `clean` stamp is **byte-identical** before and after this release, across both `audit_converge_on` values | apply the panel rule to the severity computation, changing rounds this release must not touch |
| 6 | §3 *skipped* | a round missing only a role recorded as skipped-with-reason is clean | count a skipped role as missing, which makes a task file with no `source_sections` permanently unconvergeable |
| 7 | §3 *surplus* | a round carrying a role not in `expected_roles` is clean-eligible and the surplus id is reported | fail the round on a surplus role, which makes a mis-scoped panel unreportable. **This row runs against a built fixture, not a replay: no round in this corpus is surplus**, so the mutant's consequence is argued, not measured |
| 8 | §4 | with two of three role files present, `in_flight` reports both row counts and the third as absent | report file presence only, which cannot distinguish a role that has written one row from one that has written forty |
| 9 | §4 *resolution* | a role file in neither the cwd nor the round dir is reported `unknown`, not `not-started`, and the found ones name which directory they came from | collapse the two, which reports a role that is running elsewhere as one that has not begun |
| 10 | §4 *run* | inside `TP_ROUND_DIR`, a `.part` file reads as running and its renamed form as finished | ignore `TP_ROUND_DIR`, which sends `--status` to the cwd during a run and finds nothing |
| 11 | §4a | with `audit_converge_roles: ["spec-coverage"]`, a round where only a non-listed role holds non-PASS rows converges — **replayed on v0.36.0 r12–13 and v0.37.0 r6–7, both of which exit 1 today under `all`** (sidecar, *What the two shipped releases actually measure*) | ignore the list, reproducing the two releases that shipped at exit 1 |
| 11b | §4a *default* | with `audit_converge_roles` unset, a row with an unrecognised role **and** a row with no `role` key both still gate | default to `expected_roles`, which relaxes both cases before anyone narrows anything |
| 12 | §4a *fence* | the task layer, `tp import` and `tp config --extract` refuse only a write that changes what resolves; the `--project` rule is open and its row cannot be written until §4a's decision is settled in review | apply a value rule at `tp import`, which deadlocks it by refusing a block it merely carries forward |
| 13 | §4a *unknown* | a listed role absent from `expected_roles` is an error | treat it as a no-op, converging on an empty gating set |
