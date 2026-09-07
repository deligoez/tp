# A finding can leave a round

> **This file is decisions. Its measurements are in `spec/backlog/00-a-finding-can-leave-a-round-measurements.md`, which `tp ground`
> does not grade.** That split is itself a result: grading round 2 of this spec measured 149 floor
> units over 376 lines, and counting the graded rows by `kind` put the highest finding rate on design
> claims and a near-worthless one on re-derived figures — correct findings about sentences that, had
> they never been written, would have offered nothing to find. So the rule this file now follows is
> *a number does not live in a spec; a reference does.*
>
> The theme is one sentence: **a finding leaves a round either as a spec change or as a `--resolve`
> disposition, there is no third way out, and `--status` says which happened.** Two field reports and
> this repository's own loop measured the three places that sentence is false.
>
> **It was `spec/0.37.1.md`.** `.claude-plugin/plugin.json` is read twice — Claude Code resolves plugin
> updates from it, and `hooks/session-start.sh` reads the same field as the **minimum tp version**. It
> says `1.0.1`, so tagging `v0.37.1` would tell everyone who installed that binary through the plugin
> that their tp is too old. After `v1.0.0` there is no 0.x release.
>
> §2 and §3 were built and measured before this file was written; §10 says which rows were watched in
> which colour and which were only written. Ground round 2 refuted a universal claim in §4.1 twice, by
> two independently constructed designs — that record is in the measurements file, because it is what
> makes the choice in §4.1 checkable.

## 1. Scope

A hotfix does not move other releases. The pending work carries no version numbers to move — commit
`3aa992fd` put it under `spec/backlog/`, named by priority — so the earlier version of this section,
which argued against another renumbering, is void. What survives is the bar it set.

**The bar: a defect qualifies only if its fix is measured, carries no semantic change beyond the
repair, and needs no new recorded field.** Two candidates were held back by it and are in the backlog:
the audit checklist's alphabetical ordering (`01-checklist-covers-what-changed.md`, because churn
ranking is a behaviour change) and `--check` on a partial round (`02a-round-knows-its-panel.md`,
because it needs a recorded panel). §4.1 records the one place the bar itself had to move.

Precedent, derivation and the corrected count are in `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §1.

## 2. `unresolved_findings` counts open findings

`roundPayload` counts every row in the last round whose `resolved.status` is not `"wontfix"`. Every
audit checklist item is a row, so a clean round reports its whole checklist as unresolved.

**A row counts as unresolved when it is a finding and is not closed.** A finding is a row whose
`status` is absent or not exactly `PASS` — tp's own documented rule, so review rows all remain findings
and that phase's count is unchanged (Non-Goal 5). **Closed** means `resolved.status` is `wontfix`
**or** `fixed`; omitting `fixed` left the overwhelming majority of recorded closures inert.

**An unrecognised `resolved.status` counts as open.** Fail-closed, matching `AuditRowsClean`'s
treatment of a severity it cannot grade.

The measurement this rests on — including the fact that it needs the round **as it stands at `HEAD`**,
because the dispositions were written after the tag — is `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §2.1–§2.3.

### 2.1 The payload says what it counted

`unresolved_findings` gains three siblings in the same payload: **`rows_recorded`**,
**`findings_total`** and **`findings_closed`**. All four fall out of the single pass §2 already makes;
nothing is read twice.

**`findings_total` is what makes the count checkable, because it replaces a plausibility judgement
with an identity**: `unresolved_findings == findings_total - findings_closed`, exactly, by
construction. Two counters cannot do this — `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §2.4 gives the round that
proves it, built rather than argued.

**They are reported, not gated.** They are here rather than in a minor because they are the same
field's trustworthiness: the fix makes the number right, these make it checkable.

**This release owes `skills/tp/REFERENCE.md` an update.** Its `next_action` row documents the payload
as `{round, unresolved_findings}`; that row must name the three new keys, because `next_action.payload`
is an agent-facing contract (Non-Goal 2).

## 3. A refused invocation writes no state

`tp audit <spec> --role <unknown>` refuses with exit 2 and **creates a state directory anyway**. The
transcript is in `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §3.

**The snapshot write moves**, from `loadAuditSpec` — which writes it while merely *loading* the spec —
to just past the role filter, the last point at which a refusal can still occur before the payload is
assembled. `loadAuditSpec` returns the bytes rather than writing them.

**"Validate earlier" is buildable and is not what ships.** An earlier guard resting on
`engine.RoleIsRecognised` compiles and works; it trades the refusal's diagnostic hint for a bare
refusal and repairs the `--role` branch alone, while moving the write keeps the hint and covers every
refusal ahead of it.

**The bytes must be the pre-blanking spec.** `engine.BlankFrontmatter` runs immediately after the
current write site, so they are copied before that call rather than re-read at the new one. **The
fixture must carry frontmatter** — without it that call is a byte-identical no-op and the acceptance
row cannot fail (§10 row 6).

**Why it matters more than a stray file.** A round on disk because someone mistyped a flag is a round
about nothing, `--status` reports a directory's existence, and a typo is the likeliest cause.
## 4. An accepted audit finding can clear the round

**On the audit side there is no way to accept a finding.** A `FAIL` resolved `wontfix` with evidence,
written into the round's own **recorded** file, changes nothing: `clean` is stamped at record and
`auditRoundOpenByRole` reads `AuditRowIsPass` alone, never `resolved`. The same disposition on the
review side clears the round, because `engine.ReviewRoundClean` recomputes live. Both transcripts are
in `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §4.

So the only exits are repairing the finding, destroying the record, or **re-recording the same rows
under a different `audit_converge_on`** — a third exit that reaches only non-`error` severities, is
human-only under `TP_UNATTENDED`, and costs a round of budget. A cycle that has decided to ship over a
known, justified finding cannot say so. That is the audit half of the operator's complaint that cycles
drag on; §7 is the review half.

**The surviving set is computed live; the policy is not.** `AuditRoundClean` reads the round's recorded
rows, drops every row closed `wontfix` or `duplicate` **with evidence**, and grades what remains under
the policy that was in force when the round was recorded. `fixed` does not close a round — a repair is
a claim about the code and the next round is what tests it.

**This reverses a stated refusal, and says so.** `spec/0.37.0.md` §2 does not merely stamp `clean`; it
names `--resolve` in the alternative it rejects, and it writes: *"Surviving is deliberately not the
word: it would imply a disposition that removes a row from the count, which §5 declines to create."*
So this is not a gap being filled. What justifies reversing it is that §2's **argument** — 67 non-`PASS`
rows across v0.29.0–v0.32.0 carrying severities outside the enum, which a live re-grade would make
blocking on install — is about a *policy switch*, not about a disposition. A disposition removes a row
the operator wrote evidence for; it cannot retroactively re-grade a round nobody touched.

### 4.1 Which design, and what it costs

**Two designs satisfy this without adding a recorded field**, and an earlier draft of this section
claimed none existed. Both were built and run: one by a peer session asked to try, one by the grading
unit whose brief told it to construct a counter-example. `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §4 has both, with
their transcripts.

- **A — re-stamp at `--resolve`.** After writing the disposition into the recorded round file,
  `--resolve` re-stamps that round's entry: every non-`PASS` row disposed ⇒ `Clean=true`, otherwise the
  stamp stands. `spec/0.37.0.md` §2 already anticipates an operator write changing a stamp.
- **B — recover the policy from the stamp.** `all` implies `blocking`, so over the recorded rows the
  stored `clean` bool identifies the policy in force wherever the two disagree, and the remaining case
  grades under `all`.

**This release takes A.** It writes at the moment the operator acts, so no reader recomputes anything
and no round is ever re-graded by being read; B recovers correctly but leaves the recovery in a reader,
which is the thing §2 fenced.

**Both are conservative in exactly one case, and A's is the one that matters here:** under `blocking`,
a partial disposition does not clear the round. Every non-`PASS` row must carry one, so a round whose
`error` finding is accepted and whose `warning` finding is left open still reports not clean, where
`blocking` alone would have called it clean. The exit is `--resolve-all` with a shared
justification, which is the channel `skills/tp/SKILL.md` documents. That is a cost this release accepts
rather than a defect: the sentence it must deliver is *the acceptance channel works*, not *`blocking`'s
accepted-open semantics extend to `--resolve`*.

**`--resolve` must derive the base from the path** when its positional is a recorded round file
(`spec/.tp-review/<base>/audit-round-N.ndjson`), and must not re-stamp when handed any other file. This
turns "resolve into the file `--record` wrote" from advice into a precondition.

**No new recorded field.** §1's bar holds.

## 5. One predicate, not two

`open`, `role_streaks`, `spec_coverage_clean_rounds` and `--check` must all derive from the function §4
introduces. Two predicates are two things to drift, and the drift is invisible because both produce
plausible integers — measured on a live tree, the third exit in §4 moved `clean` while leaving
`role_streaks` at `{0, open 1}`.

`next_action` follows the same set: when every non-`PASS` row in the latest round carries a
disposition, it says the round is disposed and the next step is to re-audit.

## 6. tp names every file it writes

| surface | today | after |
|---|---|---|
| `tp review\|audit <spec> --record <f>` | `{round, findings, clean, …}` | the same plus **`file`** |
| `tp set --workflow <k>=<v>` | `{"updated": {…}}` | the same plus **`file`** |

The `--project` branch of `tp set --workflow` already prints the path it wrote and warns when the value
is shadowed; the task branch prints neither, and a field report traced a `checks` value silently
landing in **another spec's** task file to that asymmetry.

`--record`'s `file` is what makes §4.1's precondition usable: without it a reader must know the path
shape by heart.

## 7. The emitted instruction names its commands

A field report describes a cycle that ran review rounds without ever calling `--resolve`. The emitted
`instruction` names `tp review --merge`, `--record` and `--status --check` as commands, and then says
*"verify and **resolve** them"* — **two bare verbs, and both are also flags**: `--verify` is listed
beside `--resolve`/`--resolve-all` under `Modes (mutually exclusive)`.

The line names both: `tp <phase> <findings> --resolve <idx> <wontfix|duplicate> [evidence]` and
`tp <phase> <spec> --verify`. And `--help` gains the usage line — today the flag's description carries
the selector shape but the positional order appears only after you trigger the error.

**The wording is one cause among four and this release does not claim it is the mechanism.** Two others
carry committed evidence: `skills/tp/SKILL.md` step 5 pointed at `merged.ndjson`, where a disposition
records nothing, until one commit before this spec; and `spec/0.31.0.md` §3.5 makes `--record` **reject**
a file carrying `fixed` rows, so a cycle disposing everything `fixed` records zero dispositions by
construction. `spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §5 ranks them.

**That last fact fences the usage line above**: it offers `wontfix|duplicate` and not `fixed`, because
`fixed` at `--record` is an error and `fixed` post-record does not clear a round.

## 8. Also shipped in this release

Committed already, listed so the release notes and the tree agree; the measurements are in
`spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §6.

- Both write fences read the path argument `mcp__codedbpro__replace` actually sends (`path`, `paths`).
- `mcp__codedbpro__batch` reaches both fences: it carries a nested write while the tool name on the
  call is `batch`, and neither matcher named it.
- The plugin manifest is bumped so both reach installed users, and the session-start test's
  near-boundary version is derived from the minimum instead of pinned.

This is not a changelog: other commits shipped since `v1.0.0` and are not listed.
## 9. Non-Goals

1. **No third pre-measured defect.** §1 names the two that were held back and why. §§4–7 are a
   different class: they close what field reports measured. §6 and §7 cite a report; §4 and §5 close
   what this repository measured on itself, which is a weaker provenance and is stated as such.
2. **No new recorded field, no new flag, no config.** §4.1 takes the design that needs none. §2.1 is
   the one contract change: three keys on `next_action.payload`, agent-facing and documented in
   `skills/tp/REFERENCE.md`.
3. **One convergence change, scoped to one signal.** §4 changes which rows survive into the audit
   grade, and nothing else: not the panel, not the `spec_hash`, not the review side. §2 changes what a
   driver is told and §3 changes when a file is written; neither touches convergence. **Budget this as
   loop-class rather than tool-class.** The often-quoted ~23-against-~11 medians are `CLAUDE.md`'s
   loop-vs-everything-else split, not this predicate's: under *"changes a convergence signal"* the
   class is two cycles, median 22, against 12.5 for the other fourteen. Two points is not a median
   worth quoting, so the rule is the class and not the number. The stopping rule is stated up front: if
   the finding count does not fall after a cut, the problem is the loop and not the document.
4. **Not the review side's discoverability beyond the emitted string.** §7 fixes what tp *says*; the
   two causes with committed evidence behind them are named there and not repaired here.
5. **No repair of past state.** `unresolved_findings` is computed on read, so every recorded round
   reports correctly the moment this ships. A state directory an earlier refusal created is
   indistinguishable from a legitimately emitted round that was never recorded, and is left alone.
6. **The review phase's count is unchanged, and the reason is the corpus rather than the code.** §2's
   finding clause is vacuous over every review row that exists — no review row carries a `status` key.
   It is **not** vacuous by construction: `internal/cli/review_record.go` validates no key set, so a
   review row carrying `status: "PASS"` would be accepted today and would stop counting under §2's
   clause. This release does not make the vacuity structural, and row 4 measures the corpus rather than
   a guarantee. The count and its command are in `spec/backlog/00-a-finding-can-leave-a-round-measurements.md`.
7. **Not a floor-budget signal.** Grading this spec measured that its own forensics dominated the
   floor. The remedy is `spec/backlog/04a-ground-command-friction.md`, which owns `tp ground`'s
   surfaces; the writing rule that follows from it belongs in `skills/tp/SKILL.md`. Neither is here.

## 10. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Ground round 2
falsified that standard on three rows by building their mutants and watching them survive; those rows
are repaired below and the section says which rows have been **watched** and which have only been
**written**.

**Watched red against `HEAD` and green after the fix: rows 1, 2, 3 and 5, with row 4 green in both
states.** Row 8 was **not**: it was reworded after that run from two counters to three plus the
identity, and `findings_total` was never emitted or asserted in either colour — the run belongs to the
row this one replaces. Rows 6 and 9–17 are written, not yet watched. Suite timings are in
`spec/backlog/00-a-finding-can-leave-a-round-measurements.md` §7 rather than here, because the quoted ones predated a test
parallelisation commit.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | v0.37.0's round 7 **as it stands at `HEAD`** — 106 rows, 3 non-`PASS`, all 3 dispositioned — reports **0**, with the fixture's own properties asserted at test time | the shipped `!= "wontfix"` loop, which returns 103 |
| 2 | §2 *finding* | an audit round of N `PASS` rows and zero findings reports 0 for every N | count rows rather than findings, making the number a function of checklist size |
| 3 | §2 *fixed* | a non-`PASS` row with `resolved.status: "fixed"` does not count | keep `wontfix` as the only closing value |
| 4 | §2 *review* | a review round's count is byte-identical before and after | apply the `PASS` filter to review rows, which carry no `status` and would all be dropped |
| 5 | §3 | after a `--role` refusal in a repo with no state directory, none exists | the shipped order, which writes the snapshot first |
| 6 | §3 *scope* | a `--role` invocation that emits at least one prompt still writes its snapshot, and **on a fixture whose spec carries frontmatter** the bytes equal the spec before blanking | move the write after `BlankFrontmatter`. **The fixture is the assertion here**: that call returns its input byte-identically when there is no frontmatter, so on a frontmatter-free spec — the one §3's transcript used — the mutant is a no-op and the row certifies itself |
| 7 | §3 *other refusals* | the same holds for every argument tp rejects before emitting — asserted over a hand-built list, because the set is not enumerable in tp | fix the `--role` branch alone, leaving every sibling refusal writing state |
| 8 | §2.1 | the payload carries `rows_recorded`, `findings_total` and `findings_closed`, and `unresolved_findings == findings_total - findings_closed` holds on every fixture in this table | emit `rows_recorded` and `findings_closed` alone — measured, that pair leaves the shipped loop's 103 arithmetically consistent with a 106-row round |
| 9 | §4 | a one-`FAIL` audit round, resolved `wontfix` with evidence in its **recorded** file, takes `consecutive_clean` 0 → 1 and `role_streaks[].open` 1 → 0 | the shipped stamp, which returns `clean: false` and `open: 1` |
| 10 | §4 *fixed* | the same row resolved `fixed` does **not** clear the round | close on any disposition, which would let a repair certify itself instead of leaving the next round to test it |
| 11 | §4 *evidence* | `wontfix` with an empty or absent evidence string does not clear the round | drop the evidence requirement, making acceptance free |
| 12 | §4 *policy* | a round recorded under `all` still grades under `all` after the project is set to `blocking`, and the reverse — **asserted with a `warning`-severity finding** | read the policy live. **The severity is the assertion**: `AuditRowsClean` grades `error` and unrecognised severities as blocking under both policies, so on an `error` fixture the live-read mutant returns the same answer and survives |
| 13 | §5 | on a round with one disposed and one open finding, `open` is 1, `role_streaks[].open` is 1, `--check` exits 1, and `clean` is false — each named, not "agree" | give `open` its own predicate. Measured on a live tree: re-recording under a changed policy moved `clean` while leaving `role_streaks` at `{0, open 1}` |
| 14 | §5 *next_action* | with every non-`PASS` row disposed, `next_action` names re-auditing rather than addressing findings | leave the string static |
| 15 | §6 | `--record`'s `file` names the round file it wrote — **not `state.json`, not the lock** — and re-reading that path returns the rows just recorded; `tp set --workflow`'s `file` names the task file whose `workflow` block changed | assert the key's presence alone, which a constant string passes |
| 16 | §7 *emission* | the emitted `instruction` names `--resolve` **and** `--verify` as commands | name `--resolve` alone, leaving `verify` bare and reproducing the ambiguity for the other flag |
| 17 | §7 *usage* | `--help`'s usage line carries the positional order, and the dispositions it offers are `wontfix\|duplicate` | offer `fixed` there — `spec/0.31.0.md` §3.5 makes `--record` reject a file carrying `fixed` rows, so the usage line would document an error |

**Row 7 is quantified over the refusal set deliberately, and it guards against regression rather than
sweeping extant siblings.** §3's transcript is one refusal, chosen because it is the one that was
reported; a fix scoped to it passes rows 5 and 6 and leaves the class open. Measured at `HEAD` the
class has exactly one open member — an unreadable `--findings` path (exit 3), a nonexistent
`--affected-files` path (exit 3) and an unsafe `--base` (exit 2) all abort before `loadAuditSpec` and
already write nothing. And "the refusal set" is not enumerable anywhere in tp: the exit sites are
spread across `audit.go`, `rolefilter.go` and `role_panel.go`, so the test hand-builds its list, which
is the shape `CLAUDE.md` calls a quantifier you did not count. Both facts belong in the row, because
together they say what it is worth — a later release adding a refusal ahead of the write cannot
silently reopen the class.
