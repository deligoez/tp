# tp — A finding's exits agree

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-findings-exits-agree-measurements.md` beside it; this file stands without
them. Its audit-side sibling is `a-finding-can-leave-an-audit-round.md`, which decides what clears an
audit round; nothing here changes what any round grades.

Class: **tool** — it changes what tp counts, writes, refuses and says, and no convergence signal.

## 1. The decision

**Context.** A finding leaves a round either as a change to what it is about or as a `--resolve`
disposition, and every surface that reports the round should say which happened. Today the surfaces
disagree. `tp resume` counts rows rather than findings, so an audit round of nothing but `PASS` rows
reports its whole checklist as unresolved; and it closes only `wontfix`, while `tp review --status`
also closes `duplicate`, so one round gives two opposite answers (§2). No surface says whether the
dispositions written for a round reached the file that is graded, and the emitted instruction does not
say which file that is (§3, §8). The carry-forward list files accepted findings under a header that
calls them unresolved, and a `--role` filter drops the regression prompt without listing it (§4).
Writes of the loop's own state name no file, drop keys they do not know, and are not atomic (§6). A
refused `tp audit` invocation has already written a round snapshot (§7). And `--record` refuses
`--round` by its value rather than by its presence (§8). A field report (WB-3155) and this
repository's own loop measured each of these; the sidecar carries the reproductions.

**Decision.** One predicate defines a closed finding, and every counting surface reads it (§2).
Dispositions are counted where the round is reported (§3). The carry-forward says what it carries
(§4). A write of the loop's state names its file, keeps what it does not know, and lands whole (§6).
A refusal writes nothing (§7). The emitted instruction and `--help` name their commands, their
target and their positional (§8).

**Consequences.** Output keys join four payloads — three on `next_action.payload` (§2.1),
`dispositioned` on `tp review --status` (§3), `file` on `--record` and `tp set --workflow` (§6.1) —
each an agent-facing contract, so `skills/tp/REFERENCE.md` is owed an update. `--record` becomes
refusable on a flag it silently accepts today (§8). The fence on who may write a disposition ships
with `a-finding-can-leave-an-audit-round.md` §3, because the fence and the thing it fences are one
release.

**Alternatives.** Validating `--role` earlier instead of moving the snapshot write — built, and
rejected in §7 because it repairs one branch and loses the hint. Two payload counters instead of the
three that give an identity — built, and rejected in §2.1. Closing `fixed` on the counting surfaces —
rejected in §2, because it re-creates the disagreement this spec removes. Judging the content of
`resolved.evidence` — not attempted (Non-Goal 3).

## 2. One predicate for a closed finding

**A finding is a row whose `status` is absent or not exactly `PASS`**, tp's documented rule. Recorded
review rows carry no `status`, so each is a finding and that phase's count is unchanged (Non-Goal 6);
an audit checklist item graded `PASS` is not a finding and never counts as unresolved.

**A finding is closed when its `resolved.status` is `wontfix` or `duplicate` and its
`resolved.evidence` is non-empty.** This is the predicate the review side's convergence already
applies and the one `a-finding-can-leave-an-audit-round.md` §2 grades an audit round by. An
unrecognised `resolved.status` counts as open, fail-closed.

**`fixed` is not closed on any surface.** A repair is a claim about the text or code, and the round
that tests it is the next one — which is why `fixed` does not clear a round on either side. A count
that subtracted `fixed` would report nothing unresolved for a round that `tp review --status --check`
still reports not clean: the same one-round-two-answers defect this section exists to remove, moved
from `duplicate` to `fixed`. What a `fixed` disposition does get is visibility: §3 counts it as
dispositioned, so an operator who marks every finding `fixed` sees that the dispositions landed while
the round stays open until the next round tests the repairs.

**Every counting surface reads it**: `tp resume`'s `next_action` and its payload, and `tp review
--status`'s `clean` and `consecutive_clean`. The audit phase's `open`, `role_streaks` and `--check`
read the same surviving set; that half ships with `a-finding-can-leave-an-audit-round.md` §5.

### 2.1 The payload says what it counted

`unresolved_findings` gains three siblings: **`rows_recorded`**, **`findings_total`** and
**`findings_closed`**, all from the single pass §2 already makes. `findings_total` turns a
plausibility judgement into an identity — `unresolved_findings == findings_total - findings_closed`,
exactly, by construction. Two counters cannot do this; the sidecar's *Why three counters and not two*
gives the round that proves it. They are reported, not gated.

## 3. `--status` says whether the dispositions landed

**`tp review --status` reports `dispositioned: k/n` on every recorded round** — `n` the round's
findings, `k` those carrying any disposition, `fixed` included. It is computed from the recorded
round file on read, so it tells an operator whether dispositions reached the file that is graded:
today a disposition written into the merge output after `--record` exits 0 and never reaches the
recorded round, and nothing `--status` reports shows the difference. The audit phase's `--status`
carries the same key, from `a-finding-can-leave-an-audit-round.md` §5.

## 4. The carry-forward says what it carries

**The list of prior findings splits its header.** Rows with no disposition stay under a header that
says they are unresolved; rows accepted `wontfix` move under their own header — *accepted: do not
re-report unless the text it cites changed*. Today both sit under one header that calls them all
unresolved. **Suppressing accepted rows is intended and stays**: `spec/undecided.md` *A prior-round
section for `tp review`* closed that question no, and this section changes the header, not the set.
`fixed` rows keep their existing *resolved, do not regress* listing.

**A `--role` filter that drops the regression prompt lists it in `skipped_roles`.** Today a
narrowed emission drops that prompt and reports `skipped_roles: []`, so the caller cannot tell the
regression pass was not emitted.

## 5. The audit phase's counts — moved

Audit `open`, `role_streaks`, `--status --check` and `next_action` reading the surviving set ship
with `a-finding-can-leave-an-audit-round.md` §5; that spec's §2 defines the set.

## 6. A write of the loop's state

### 6.1 It names its file

| surface | today | after |
|---|---|---|
| `tp review\|audit <spec> --record <f>` | `{round, findings, clean, …}` | the same plus **`file`**, the recorded round file |
| `tp set --workflow <k>=<v>` | `{"updated": {…}}` | the same plus **`file`**, the task file whose block changed |

The `--project` branch of `tp set --workflow` already names the path it wrote. `--record`'s `file`
is what makes resolving into the recorded round usable without knowing the path shape by heart. Every
other task-file write naming its target is `a-task-file-write-names-its-target`.

### 6.2 It keeps the keys it does not know

**`--record` preserves `state.json` keys it does not understand**, at the top level and on a round
entry, and where a typed field and a preserved key share a name the typed field wins. Today one
`--record` drops every key the binary does not know, silently, and shipped round fields have been
erasable since they landed. The fix buys durability against binaries at or after this release and
nothing against an older one; `round-knows-its-panel-measurements.md` carries both measurements and
names the fields. Any release that adds a key to `state.json` — `round-knows-its-panel`'s panel
record among them — depends on this one shipping first.

### 6.3 It lands whole

**The recorded round file is written atomically at all three of its writers** — `tp review --record`,
`tp audit --record`, and the rewrite behind `--resolve`/`--resolve-all` on both phases — through a
temporary file in the same directory and a rename, the shape the snapshot beside it already uses. tp's
readers of that file take no lock and skip an unparseable line, so a partial read is reachable without
a crash and can grade a round on a truncated subset of its findings. **`tp audit --merge -o` writes
through the same temporary-and-rename path `tp review --merge -o` already uses**, so a symlinked `-o`
is replaced rather than followed and the output's mode no longer depends on a pre-existing file; a
hardlinked `-o` is detached, as on the review side, and that is accepted. **Ordering is unchanged**:
the round file lands before its index entry. The probes are in `loops-own-state-writes-measurements.md`
and this spec's sidecar.

## 7. A refused invocation writes no state

`tp audit <spec> --role <unknown>` refuses at exit 2 and has already written the round's snapshot,
so a mistyped flag leaves a snapshot on disk for a round nobody emitted. **The snapshot is written
after the last refusal, not while the spec is loaded.** "Validate earlier" is buildable and is not
what ships: it trades the refusal's diagnostic hint for a bare refusal and repairs the `--role`
branch alone, while moving the write keeps the hint and covers every refusal ahead of it. **The
snapshot still holds the spec as written, frontmatter included** — so the acceptance fixture must
carry frontmatter, or the row cannot tell a correct move from one that snapshots the blanked text
(§10 row 7).

## 8. `tp review`'s instruction and `--help` name their commands

**The emitted instruction names its commands and its target.** Today `tp review`'s says *"verify and
resolve them, then record"* — two bare verbs that are both flags, and a sequence that is right only
if the resolve comes first. It names
`tp review <findings> --resolve <idx> <wontfix|duplicate> <evidence>` and
`tp review <spec> --verify`, and says: **resolve before `--record`, or afterwards into the file
`--record` names.** The dispositions it offers are `wontfix|duplicate` because `--record` refuses a
file carrying `fixed` rows (`spec/0.31.0.md` §3.5). This release does not claim the wording is the
whole mechanism; the sidecar ranks the causes.

**`tp review --help` names each mode's positional.** Its mode list names `--record` and `--status`
beside the modes it lists today, with the positional each takes, the order `--resolve` takes its
arguments in, and that a recorded round's number is derived from state.

**`--record` refuses `--round` by its presence, not its value.** Today
`tp review <spec> --record <f> --round 1` passes and records the next round, while `--round 9` is
refused with a hint pointing to `--help`, which does not state the rule.

## 9. Non-Goals

1. **No new recorded field, no new flag, no config.** The contract changes are the output keys
   §1 *Consequences* lists, each documented in `skills/tp/REFERENCE.md`.
2. **No convergence change.** Nothing here changes which rows survive into a grade; §2 makes the
   counting surfaces read the set the review side already grades by.
3. **No judgement of `resolved.evidence`'s content.** It must be non-empty; tp reads no further.
4. **The carry-forward's suppression is unchanged** (§4), and `fixed` does not become closed (§2).
5. **No repair of past state.** Every count here is computed on read, so recorded rounds report
   correctly the moment this ships; a state directory an earlier refusal created, and a round file
   an earlier non-atomic write produced, are left alone.
6. **The review phase's `unresolved_findings` is unchanged**, because no recorded review row carries
   a `status` key. That is a property of the corpus, not a guarantee — row 4 measures it.
7. **No lock added, removed or re-keyed.** §6.3 changes how bytes land inside each critical section,
   not who may write.

## 10. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Rows 1, 2 and 6 were
watched red against `HEAD` and green under a built fix, and row 4 green in both; row 3 was reversed
with §2's `fixed` decision, and every other row is written, not yet watched. Rows whose subject does
not exist at `HEAD` defer their two counts to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | v0.37.0's audit round 7 **as it stands at `HEAD`** — every non-`PASS` row dispositioned `wontfix` with evidence — reports **0** unresolved, with the fixture's own properties asserted at test time | the shipped loop that closes only `wontfix` and counts `PASS` rows, which returns the round's `PASS` count |
| 2 | §2 *finding* | an audit round of N `PASS` rows and zero findings reports 0 for every N | count rows rather than findings, making the number a function of checklist size |
| 3 | §2 *fixed* | a finding resolved `fixed` counts as unresolved in `tp resume`, and `tp review --status` reports its round not clean | close on `fixed`, which reports 0 unresolved for a round `--status --check` still fails |
| 4 | §2 *review* | a review round's `unresolved_findings` is byte-identical before and after | apply the `PASS` filter to review rows, which carry no `status` and would all be dropped |
| 5 | §2 *duplicate* | a one-finding round resolved `duplicate` with evidence gives the same answer from `tp review --status` and from `tp resume` | close only `wontfix` — `HEAD`, where the two surfaces disagree |
| 6 | §7 | after a `--role` refusal in a repo with no state directory, none exists | the shipped order, which writes the snapshot first |
| 7 | §7 *bytes* | a `--role` invocation that emits a prompt still writes its snapshot, and **on a fixture whose spec carries frontmatter** the bytes equal the spec as written | snapshot the blanked spec; on a frontmatter-free fixture the two are byte-identical, so the fixture is the assertion |
| 8 | §7 *other refusals* | every argument tp rejects before emitting leaves no state — asserted over a hand-built list, because the refusal set is not enumerable in tp | fix the `--role` branch alone |
| 9 | §2.1 | the payload carries `rows_recorded`, `findings_total` and `findings_closed`, and the identity holds on every fixture in this table | emit `rows_recorded` and `findings_closed` alone, which leaves the shipped loop's miscount arithmetically consistent |
| 10 | §3 | on a one-finding recorded round, `dispositioned` reads `0/1`, then `1/1` after a `--resolve` into the recorded file, and stays `0/1` after a `--resolve` into the merge output | read the disposition count from the merge output, or report the finding count alone |
| 11 | §4 *header* | a carried `wontfix` row appears under the accepted header and under no header that calls it unresolved, and it is still listed | drop accepted rows from the list, which reverses a decision `spec/undecided.md` closed |
| 12 | §4 *regression* | `--role <x>` on a round that would emit the regression prompt lists it in `skipped_roles` | the shipped filter, which drops it and reports `skipped_roles: []` |
| 13 | §6.1 | `--record`'s `file` names the round file it wrote — not `state.json`, not the lock — and re-reading it returns the rows just recorded; `tp set --workflow`'s `file` names the task file whose block changed | assert the key's presence alone, which a constant string passes |
| 14 | §6.2 | a state file carrying an unknown top-level key and an unknown round-entry key keeps both across a `--record`, and a typed field sharing a name with a preserved key wins | marshal the typed struct alone — `HEAD`, which drops both; and merge the preserved map last, which lets a stale value resurface |
| 15 | §6.3 | with lock-free readers against repeated writes of a round file, no read observes a partial file and every whole read is byte-identical to what was written — at `--record` and at `--resolve` | the shipped plain write at either site; a fix covering only `--record` fails the `--resolve` arm |
| 16 | §6.3 *ordering, empty* | the round file lands before its index entry, and a round whose every role found nothing still writes an empty file and records `findings: 0, clean: true` | swap the order; or treat an empty write as a failure |
| 17 | §6.3 *merge `-o`* | `tp audit --merge -o` over a symlink leaves the link's target byte-identical, and over an existing `0644` file yields the mode `tp review --merge -o` yields | the shipped single write, which follows the link and keeps the old mode |
| 18 | §8 *instruction* | the emitted instruction names `--resolve` and `--verify` as commands and says to resolve before `--record` or into the file `--record` names | name `--resolve` alone, leaving `verify` bare |
| 19 | §8 *usage* | `--help` names every mode's positional, including `--record`'s and `--status`'s, and offers `wontfix\|duplicate` | offer `fixed`, which `--record` refuses |
| 20 | §8 *round* | `--record <f> --round 1` is refused at exit 2, the same as `--round 9`, and neither records a round | test the value against its default — `HEAD`, where `--round 1` records |
