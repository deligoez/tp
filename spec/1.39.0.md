# tp v1.39.0 — The round knows which roles it expects

> **This file is decisions.** Every figure names the command that derives it. The two defects below
> were reproduced in a built fixture before they were described, and §1.1 is that transcript — a
> reading of the code would have justified either one, and only the run distinguishes them.

## 1. Overview

`tp audit` computes the panel a round expects — it emits one prompt per active role and a
`skipped_roles` entry, with a reason, for each role it declined to emit — and then **throws that set
away**. Nothing is recorded with the round, so when `--record` stamps `clean` and `--check` reads it,
neither can tell *a role that reported nothing* from *a role that reported nothing wrong*.

Two consequences, in opposite directions, both real:

- **Too lax.** A round that receives one of three emitted roles stamps `clean: true`. Two such rounds
  make `tp audit --status --check` exit **0** — this project's ship signal — on a spec whose
  conformance role never ran and whose only task is still open. Measured, §1.1.
- **Too strict.** v0.36.0 shipped with `--check` at exit 1 while `spec-coverage` was 74/74 PASS in
  each of rounds 10–13, because a non-conformance role's rows override a clean conformance role.
  Every non-PASS row in those four rounds belongs to `maintainability-conventions`, and the round
  v0.36.0 actually shipped on carries two of them at `status: FAIL`, `severity: error`, both
  unresolved — so this is **not** a case where nobody ever FAILed. Derivation:
  `python3 -c "import json;[print(n,[(r.get('role'),r.get('status'),r.get('severity')) for r in (json.loads(l) for l in open('spec/.tp-review/0.36.0/audit-round-%d.ndjson'%n) if l.strip()) if r.get('status')!='PASS']) for n in (10,11,12,13)]"`.
  `CLAUDE.md` records the divergence, but its v0.36.0 sentence says *zero open spec-scoped findings*
  while *no role held a FAIL* is its **v0.37.0** sentence. An earlier draft of this bullet merged the
  two, and the merged sentence is false; the source sentence in `CLAUDE.md` is not this file's to fix.

This release ships **one new fact — the panel a round expected — recorded where tp knows it, and two
uses of it**. It does not add a workflow field. Scoping convergence to the roles that decide it is the
second half and is §5.1.

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
rather than each round's `clean` byte.

## 2. The round records the panel it expected

> **This section changed shape after grounding: the panel is now written at emission, which is a new
> write on the emission path.** An earlier draft wrote it at record time and offered a fallback that
> derived it from `roles_hash`; both were refuted by running, below. The release's subject, its gate
> and the field it puts on the round entry are unchanged.

`ReviewRound` gains **`expected_roles`** — the role ids `tp audit` emitted a prompt for, plus each
`skipped_roles` id paired with its reason — and **`received_roles`**, the distinct non-empty roles
among the recorded rows.

The producer already exists on both sides. `runAudit` builds `prompts` and `skipped_roles`
(`internal/cli/audit.go`), and `auditRoundOpenByRole` (`internal/engine/rolestreaks.go`)
already tallies a round's rows by role id and already treats a role with no rows as *not measured*
rather than *clean*. **This release does not invent that distinction; it records the other half of it,
so the round-level stamp can make it too.**

**The corpus cannot supply the panel, and that is measured rather than argued.** `roles_hash` hashes
the role *files* (`ComputeRolesHash`, `internal/engine/rolehash.go`); the emitted panel is that set
minus frontmatter deactivations, domain mismatches, `no-checklist-items` skips and `--role`
narrowing. Two narrowings were built on a three-role fixture and **both leave `roles_hash`
byte-identical to the unnarrowed corpus's**:

```
spec frontmatter  tp: {audit_roles: {go-safety: {enabled: false}}}
  -> 2 prompts, skipped_roles [{"role":"go-safety","reason":"disabled-by-spec"}], roles_hash unchanged
tp audit <spec> --affected-files main.go --role ax-contract
  -> 1 prompt, skipped_roles [],                                                  roles_hash unchanged
```

`rolehash.go`'s own comment says why the first is invisible — *"a spec-frontmatter override is covered
by spec_hash, not here (no double-count)"*. **The second is invisible to everything**: under `--role`
the narrowed-away roles are named in no field of the emission and in no file on disk, so even the
reason-carrying `skipped_roles` hatch cannot see them. `--role` is v0.36.0's headline feature and this
project's per-role loop is built on it, so a rule deriving the panel from the corpus would judge every
`--role` round permanently un-clean, with nothing to appeal to.

**So the panel is written at emission**, where `runAudit` already holds both halves of it — not
re-derived at record, which is a separate invocation that never saw the `--role` flag, and not on
read.

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

**`ReviewRound` has gained a field three times already, each shipped under this defect; this release
is the first to *fix* it.** `roles_hash` landed in v0.25.0, `id_scheme` in v0.30.0, `harness_note` in
v0.31.0 — derivation: `git log --oneline -S'<the field declaration>' -- internal/engine/reviewstate.go`
for each, then `git tag --contains <sha>`. All three are optional keys on the round entry and all
three have been erasable ever since. Measured on `HEAD`:

```
inject expected_roles + spec_moved_mid_round onto round 1   -> present
tp audit <spec> --record <file>                             -> both GONE
inject a top-level reconciliations[]                        -> present
tp audit <spec> --record <file>                             -> GONE
```

`SaveReviewState` marshals a typed struct (`internal/engine/reviewstate.go`), and `ReviewState` has
exactly three fields, so **any key it does not know is dropped on the next write**. Reading is safe — nothing calls
`DisallowUnknownFields` — which is what makes the loss silent.

**The trigger is a stray `--record`, and only that.** `SaveReviewState` has exactly two production
callers, `internal/cli/audit_record.go` and `internal/cli/review_record.go`. Measured with the keys
injected: a plain emission, `tp audit --status` and `tp review` all leave them intact; only
`tp <phase> --record` erases them. `CLAUDE.md` warns that the PATH-installed tp lags the repository,
so the live hazard is a stray *recording* invocation from that binary — a narrower and more avoidable
event than "one stray invocation", which is what an earlier draft of this paragraph claimed.

**So `ReviewState` and `ReviewRound` round-trip unknown keys**: unmarshal them into a sibling map,
merge them back on save, and let a typed field win where both exist.

**This fix does not make a field durable against an *older* binary, and claiming it did would be this
section's worst sentence.** A binary predating the fix still marshals the typed struct and drops the
key. Measured: a tp built from tag `v0.37.0` was run over a `state.json` carrying an injected
`expected_roles`, and one `--record` dropped it — the same result as `HEAD`. What the fix buys is
durability against binaries **at or after** this release. That is less than the paragraph above might
suggest and is exactly what the two dependents need: the PATH hazard stays open for as long as any
pre-fix tp is installed anywhere.

**Those two dependents are the emit-time hash release and the reconcile release, and each states the
dependency in its own text.** The emit-time hash's `spec_moved_mid_round` counter and the reconcile
release's `reconciliations` rows are both keys a pre-fix tp erases; both specs carry that measurement
themselves, and the reconcile release additionally names the shipped test that asserts the loss
today. They are not the next two releases in roadmap order, and this release does not spare them the
restatement — it only has to ship first.

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
correct round held back by a role that does not decide the question — is §5.1's, and shipping this
half alone leaves it open.

**A round that receives a role it did not expect is clean-eligible and says so.** `received_roles`
carries the id, `role_streaks` already reports it, and the round is not failed for it. The reason is
that a surplus role is the only signal a panel is mis-scoped, and failing the round would delete it.

**No such round exists in this corpus, so this decision is offered without a measurement rather than
with the wrong one.** Over v0.37.0's seven audit rounds the distinct role set is the full four-role
panel in every one:
`python3 -c "import json,glob;[print(f, sorted({json.loads(l).get('role') for l in open(f) if l.strip()})) for f in sorted(glob.glob('spec/.tp-review/0.37.0/audit-round-*.ndjson'))]"`.
An earlier draft justified the rule with `internal/cli/unattended.go`'s coverage during that cycle,
which is a **different object**: `go-safety` was *in* the panel in all seven rounds, and what it added
was an off-checklist **item** — `file-go-safety-internal-cli-unattended-go-apply-the-go`, present in
the rounds the same glob reports when filtered on that `item_id`. A rule failing a round on a surplus
**role** would never have touched it, so that evidence supported nothing and is withdrawn.

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

## 5. Non-Goals

1. **Convergence is not scoped to a subset of roles here.** That is the *too strict* direction, it
   needs a fenced list field whose non-default value relaxes a gate, and it reads `expected_roles`,
   which does not exist until this release ships. It belongs to the release that takes what stops
   counting against convergence, alongside recorded dispositions.
2. **No new workflow field.** `audit_converge_on` is unchanged and still governs severity.
3. **`--status` does not spawn, wait, poll or time anything.** It reads files and returns. A progress
   report that blocks is a driver, and tp already has one.
4. **No retroactive effect.** A round with no recorded panel — recorded before this release, or
   recorded without an emission having opened it — keeps today's semantics and cannot be un-converged
   by upgrading.
5. **The review phase is unchanged.** `ReviewConsecutiveClean` has its own convergence policy and its
   own field; widening this to review doubles the surface for a defect measured only on the audit
   side. It also keeps `--no-state` out of scope: that flag is declared on `tp review` only
   (`internal/cli/review.go`), so no audit invocation can suppress the emission-time write §2 adds.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1.1 | §1.1's fixture rebuilt: two rounds carrying 1 of 3 roles leave `--check` at a **non-zero** exit | the shipped behaviour, which exits 0 — this test must be seen failing against `HEAD` before the fix lands |
| 2 | §2 | `expected_roles` records an emitted role and a skipped role **with its reason**, distinguishably | store a flat id list, which makes `no-checklist-items` indistinguishable from a role that never reported |
| 2b | §2 *emission* | the panel recorded for a round emitted with `--role` is exactly the roles emitted, and three successive `--role` emissions of one round union to the full panel | keep only the last emission's roles, so a round in which one role never reported still stamps clean |
| 2c | §2 *not the corpus* | with one role deactivated in the spec frontmatter, the panel records it declined-with-reason **while** `ComputeRolesHash` is byte-identical to the unnarrowed corpus's; the same fixture under `--role` records a one-role panel with `skipped_roles` empty | derive the panel from `roles_hash` equality — byte-identical in both cases, so it over-states the panel in the two narrowings this project actually produces |
| 3 | §2 *pinned* | deleting a role file after a round is recorded does not change that round's `expected_roles` | re-derive from the corpus on read, so editing `.tp/auditors/` rewrites history |
| 4 | §2 *legacy* | a round with no `expected_roles` key — recorded before this release, or recorded with no emission having opened it — stays clean and converged exactly as before | treat a missing key as an empty set, which un-converges every shipped release on upgrade |
| 4b | §2.2 | a state file carrying an unknown top-level key **and** an unknown key on a round entry keeps both across a `--record` | marshal the typed struct alone — the shipped behaviour, measured to drop both, and this must be watched failing against `HEAD` first |
| 4c | §2.2 *precedence* | where a typed field and a preserved unknown key share a name, the typed field wins and the stale copy does not resurface | merge the map last, letting a stale preserved value overwrite what this release just computed |
| 5 | §3 | a complete round's `clean` stamp is **byte-identical** before and after this release, across both `audit_converge_on` values | apply the panel rule to the severity computation, changing rounds this release must not touch |
| 6 | §3 *skipped* | a round missing only a role recorded as skipped-with-reason is clean | count a skipped role as missing, which makes a task file with no `source_sections` permanently unconvergeable |
| 7 | §3 *surplus* | a round carrying a role not in `expected_roles` is clean-eligible and the surplus id is reported | fail the round on a surplus role, which makes a mis-scoped panel unreportable. **This row runs against a built fixture, not a replay: no round in this corpus is surplus** (§3's derivation), so the mutant's consequence is argued, not measured |
| 8 | §4 | with two of three role files present, `in_flight` reports both row counts and the third as absent | report file presence only, which cannot distinguish a role that has written one row from one that has written forty |
| 9 | §4 *resolution* | a role file in neither the cwd nor the round dir is reported `unknown`, not `not-started`, and the found ones name which directory they came from | collapse the two, which reports a role that is running elsewhere as one that has not begun |
| 10 | §4 *run* | inside `TP_ROUND_DIR`, a `.part` file reads as running and its renamed form as finished | ignore `TP_ROUND_DIR`, which sends `--status` to the cwd during a run and finds nothing |

**Row 1 is the acceptance of this release and must be watched failing first.** It is the only row that
asserts the whole predicate end to end, and a test that has never been observed red proves nothing
about the code — it may be asserting a tautology and would pass identically either way.
