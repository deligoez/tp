# tp — A finding's exits agree

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-findings-exits-agree-measurements.md` beside it; this file stands without
them. It is the non-convergence half of the hotfix spec formerly numbered `1.0.2`; the audit-side acceptance channel
is `a-finding-can-leave-an-audit-round.md`, and nothing here changes what any round grades.

Class: **tool** — it changes what tp counts, writes, refuses and says, and no convergence signal.
Budget it at the tool-class median in `CLAUDE.md`'s *What a cycle costs* table.

## 1. The decision

**Context.** A finding leaves a round either as a spec change or as a `--resolve` disposition, there
is no third way out, and `--status` says which happened. Two field reports and this repository's own
loop measured the places that sentence is false today. `unresolved_findings` counts rows rather than
findings, so a clean audit round reports its whole checklist as open (§2). A refused `tp audit`
invocation has already written a round snapshot by the time it refuses (§3). A `wontfix` with no
evidence is accepted at the write and ignored at the record, and an unattended agent can write one
(§4). Two predicates decide what *open* means and already disagree about `duplicate` (§5). `--record`
and `tp set --workflow` do not say which file they wrote (§6). And the emitted instruction names
`resolve` and `verify` as bare verbs where both are flags (§7).

**Decision.** One predicate defines a closed finding — a non-`PASS` row whose `resolved.status` is
`wontfix` or `duplicate` with non-empty evidence — and every surface reads it: the count, `open`,
`role_streaks`, `--check`, `next_action` and `tp resume`. A refusal writes nothing. The disposition
write is fenced. Every write names its file. The instruction names its commands.

**Consequences.** Three keys join `next_action.payload` (§2.1), an agent-facing contract, so
`skills/tp/REFERENCE.md` is owed an update. The snapshot write moves later in `runAudit` (§3). The
disposition fence this spec used to carry ships with the sibling spec instead, for the reason §4
gives.

**Alternatives.** Validating `--role` earlier instead of moving the snapshot write — buildable, and
rejected in §3 because it repairs one branch and loses the hint. Two payload counters instead of the
three that give an identity — built, and rejected in §2.1. Judging the content of
`resolved.evidence` — not attempted; tp requires a reason and does not read it (Non-Goal 3).

## 2. `unresolved_findings` counts open findings

`roundPayload` counts every row in the last round whose `resolved.status` is not `"wontfix"`. Every
audit checklist item is a row, so a clean round reports its whole checklist as unresolved.

**A row counts as unresolved when it is a finding and is not closed.** A finding is a row whose
`status` is absent or not exactly `PASS` — tp's own documented rule, so review rows all remain findings
and that phase's count is unchanged (Non-Goal 6). **Closed** means `resolved.status` is `wontfix`
**or** `duplicate`, with non-empty evidence (§4) — the same predicate `a-finding-can-leave-an-audit-round.md`
§2 grades by — **or** `fixed`; omitting `fixed` left the overwhelming majority of recorded closures
inert.

**An unrecognised `resolved.status` counts as open.** Fail-closed, matching `AuditRowsClean`'s
treatment of a severity it cannot grade.

The measurement this rests on — including the fact that it needs the round **as it stands at `HEAD`**,
because the dispositions were written after the tag — is in the sidecar under
*`unresolved_findings` on v0.37.0's round 7*.

### 2.1 The payload says what it counted

`unresolved_findings` gains three siblings in the same payload: **`rows_recorded`**,
**`findings_total`** and **`findings_closed`**. All four fall out of the single pass §2 already makes;
nothing is read twice.

**`findings_total` is what makes the count checkable, because it replaces a plausibility judgement
with an identity**: `unresolved_findings == findings_total - findings_closed`, exactly, by
construction. Two counters cannot do this — the sidecar's *Why three counters and not two* gives the
round that proves it, built rather than argued.

**They are reported, not gated.** They are here rather than in a minor because they are the same
field's trustworthiness: the fix makes the number right, these make it checkable.

**This release owes `skills/tp/REFERENCE.md` an update.** Its `next_action` row documents the payload
as `{round, unresolved_findings}`; that row must name the three new keys, because `next_action.payload`
is an agent-facing contract (Non-Goal 1).

## 3. A refused invocation writes no state

`tp audit <spec> --role <unknown>` refuses with exit 2 and **creates a state directory anyway**: the
refusal sits at `internal/cli/audit.go:379`, after the snapshot write `loadAuditSpec` makes at
`internal/cli/audit.go:518`. The transcript is in the sidecar under *The refused `--role`
invocation*.

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
row cannot fail (§9 row 6).

**Why it matters more than a stray file.** A round on disk because someone mistyped a flag is a round
about nothing, `--status` reports a directory's existence, and a typo is the likeliest cause.

## 4. The disposition write is fenced elsewhere

The two sinks this section carried — `tp audit --resolve` and `--resolve-all` refused under
`TP_UNATTENDED=1`, and `resolved.evidence` required non-empty both at the write and at the record —
ship with `a-finding-can-leave-an-audit-round.md` §3, together with their three test rows and the two
transcripts behind them. They belong there because that spec is what makes a disposition clear a
round: the fence and the thing it fences are one release, and shipping them apart would put the
escape hatch in a user's hands one release before the lock. Nothing in this spec depends on them
landing here.

## 5. One predicate, not two

`open`, `role_streaks`, `spec_coverage_clean_rounds` and `--check` must all derive from the surviving
set §2 defines and `a-finding-can-leave-an-audit-round.md` §2 grades. Two predicates are two things
to drift, and the drift is invisible because both produce plausible integers — measured on a live
tree, re-recording under a changed policy moved `clean` while leaving `role_streaks` at
`{0, open 1}`.

`next_action` follows the same set: when every non-`PASS` row in the latest round carries a
disposition, it says the round is disposed and the next step is to re-audit.

**The two predicates already disagree, at `HEAD`, about `duplicate`.** `roundPayload` in
`internal/engine/resumepayload.go` counts a row as unresolved unless `resolved.status` is exactly
`"wontfix"`; `reviewFindingResolvedAway` in `internal/engine/reviewclean.go` subtracts `wontfix`
**and** `duplicate`. One round, two surfaces, opposite answers — `tp review --status` and `tp resume`
on the same tree — and neither is reading the other's predicate. The transcript is in the sidecar
under *Two predicates disagree about `duplicate`*.

## 6. tp names every file it writes

| surface | today | after |
|---|---|---|
| `tp review\|audit <spec> --record <f>` | `{round, findings, clean, …}` | the same plus **`file`** |
| `tp set --workflow <k>=<v>` | `{"updated": {…}}` | the same plus **`file`** |

The `--project` branch of `tp set --workflow` already prints the path it wrote and warns when the value
is shadowed; the task branch prints neither, and a field report traced a `checks` value silently
landing in **another spec's** task file to that asymmetry.

`--record`'s `file` is what makes the sibling spec's `--resolve` precondition usable: without it a
reader must know the path shape by heart.

## 7. The emitted instruction names its commands

A field report describes a cycle that ran review rounds without ever calling `--resolve`. The emitted
`instruction` names `tp review --merge`, `--record` and `--status --check` as commands, and then says
*"verify and **resolve** them"* — **two bare verbs, and both are also flags**: `--verify` is listed
beside `--resolve`/`--resolve-all` under `Modes (mutually exclusive)`.

The line names both: `tp <phase> <findings> --resolve <idx> <wontfix|duplicate> <evidence>` and
`tp <phase> <spec> --verify`. And `--help` gains the usage line — today the flag's description carries
the selector shape but the positional order appears only after you trigger the error.

**The wording is one cause among four and this release does not claim it is the mechanism.** Two others
carry committed evidence: `skills/tp/SKILL.md` step 5 pointed at `merged.ndjson`, where a disposition
records nothing, until one commit before this spec; and `spec/0.31.0.md` §3.5 makes `--record` **reject**
a file carrying `fixed` rows, so a cycle disposing everything `fixed` records zero dispositions by
construction. The sidecar's *What the emitted instruction actually says* ranks them.

**That last fact fences the usage line above**: it offers `wontfix|duplicate` and not `fixed`, because
`fixed` at `--record` is an error and `fixed` post-record does not clear a round.

## 8. Non-Goals

1. **No new recorded field, no new flag, no config.** §2.1 is the one contract change: three keys on
   `next_action.payload`, agent-facing and documented in `skills/tp/REFERENCE.md`.
2. **No convergence change.** Nothing here changes which rows survive into a grade; §5 makes every
   surface read the surviving set the sibling spec defines, and §3 changes when a file is written.
3. **No judgement of `resolved.evidence`'s content.** §4 requires it to be non-empty at both sinks and
   reads no further.
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
   a guarantee.

## 9. Tests

Every row derives from a numbered decision and names a mutant that must fail it. **Watched red against
`HEAD` and green after the fix: rows 1, 2, 3 and 5, with row 4 green in both states.** Row 8 was
**not**: it was reworded after that run from two counters to three plus the identity, and
`findings_total` was never emitted or asserted in either colour — the run belongs to the row this one
replaces. Every other row is written, not yet watched.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | v0.37.0's round 7 **as it stands at `HEAD`** — every non-`PASS` row dispositioned — reports **0**, with the fixture's own properties (row count, non-`PASS` count, disposition count) asserted at test time | the shipped `!= "wontfix"` loop, which returns the round's `PASS` count |
| 2 | §2 *finding* | an audit round of N `PASS` rows and zero findings reports 0 for every N | count rows rather than findings, making the number a function of checklist size |
| 3 | §2 *fixed* | a non-`PASS` row with `resolved.status: "fixed"` does not count | keep `wontfix` as the only closing value |
| 4 | §2 *review* | a review round's count is byte-identical before and after | apply the `PASS` filter to review rows, which carry no `status` and would all be dropped |
| 5 | §3 | after a `--role` refusal in a repo with no state directory, none exists | the shipped order, which writes the snapshot first |
| 6 | §3 *scope* | a `--role` invocation that emits at least one prompt still writes its snapshot, and **on a fixture whose spec carries frontmatter** the bytes equal the spec before blanking | move the write after `BlankFrontmatter`. **The fixture is the assertion here**: that call returns its input byte-identically when there is no frontmatter, so on a frontmatter-free spec — the one §3's transcript used — the mutant is a no-op and the row certifies itself |
| 7 | §3 *other refusals* | the same holds for every argument tp rejects before emitting — asserted over a hand-built list, because the set is not enumerable in tp | fix the `--role` branch alone, leaving every sibling refusal writing state |
| 8 | §2.1 | the payload carries `rows_recorded`, `findings_total` and `findings_closed`, and `unresolved_findings == findings_total - findings_closed` holds on every fixture in this table | emit `rows_recorded` and `findings_closed` alone — measured, that pair leaves the shipped loop's answer arithmetically consistent with the round it miscounted |
| 9 | §5 | on a round with one disposed and one open finding, `open` is 1, `role_streaks[].open` is 1, `--check` exits 1, and `clean` is false — each named, not "agree" | give `open` its own predicate. Measured on a live tree: re-recording under a changed policy moved `clean` while leaving `role_streaks` at `{0, open 1}` |
| 10 | §5 *duplicate* | a one-finding round resolved `duplicate` with evidence gives the same answer from `tp review --status` and from `tp resume` | keep `roundPayload`'s `!= "wontfix"` test — `HEAD`, where the two surfaces disagree |
| 11 | §5 *next_action* | with every non-`PASS` row disposed, `next_action` names re-auditing rather than addressing findings | leave the string static |
| 12 | §6 | `--record`'s `file` names the round file it wrote — **not `state.json`, not the lock** — and re-reading that path returns the rows just recorded; `tp set --workflow`'s `file` names the task file whose `workflow` block changed | assert the key's presence alone, which a constant string passes |
| 13 | §7 *emission* | the emitted `instruction` names `--resolve` **and** `--verify` as commands | name `--resolve` alone, leaving `verify` bare and reproducing the ambiguity for the other flag |
| 14 | §7 *usage* | `--help`'s usage line carries the positional order, and the dispositions it offers are `wontfix\|duplicate` | offer `fixed` there — `spec/0.31.0.md` §3.5 makes `--record` reject a file carrying `fixed` rows, so the usage line would document an error |

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
