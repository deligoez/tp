# tp — An audit round records what was graded

**Closed 2026-09-11: landed via the fix track, not this spec's loop**, at `45d7858d` (stable
`file_check` ids), `666ab808` (`--merge` keeps disagreeing verdicts), `719f932c` (a key disposes every
finding under it), `ca2159ad` (every closing sha is read), `e650b1ac` (a resolve says when it changed
no round), `1d50e220` (the prep script) and `fcb57b4e` (an empty array is no findings). The loop was
stopped after two ground and four review rounds; the body below is the text those rounds read.

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `audit-records-what-was-graded-measurements.md` beside it, and this file stands
without them. It comes from a field report (WB-3155) verified claim by claim on 2026-09-11; each
verification, with its reproduction and source citations, is in the sidecar under *Field report
WB-3155, verified 2026-09-11*.

Class: **tool** — it changes which id a checklist item carries, which files the conformance role is
given and what the disposition path says, and no convergence rule.

## 1. Overview

Three defects with one shape: **the recorded round holds something other than what was graded, and
nothing says so.** Each runs at exit 0 with no warning and a payload that reads normally.

1. **An item id can name a different file in a different round or shard** (§2).
2. **A task's second closing commit is not read** by the one role that measures conformance (§3).
3. **A disposition written where the skill says to write it does not reach the round**, and the
   step tp offers next fabricates one (§4).

It ships before `checklist-covers-what-changed`, which reorders the code roles' file list by churn:
a list whose order moves every round moves a positional suffix with it, so that release alone would
make §2's defect fire more often (sidecar, *Why this ships before the churn ranking*).

It was built in a clone before its first grading round. What that changed in this text, and which
existing tests change with which decision, is in the sidecar under *Built before review*; each such
test moves with the task whose change breaks it, since the gate runs at every close.

## 2. A file check has one id, in every round and every shard

**Context.** A `file_check` item's id is built from a slug of its path cut to a fixed length, with a
positional suffix added when two files in one emission share the slug — which every file in one
sufficiently deep directory does. Two failures follow, both reproduced (sidecar, field report #27):

- **Across rounds**, a new file that sorts earlier in the same directory shifts every suffix after
  it, so an id that named one file now names another. The prior-round section then hands a role its
  previous `FAIL` under an id the current checklist attaches to a different file.
- **Across shards** — the workaround tp's own truncation notice recommends, *name the rest with
  `--affected-files`* — each emission restarts the suffix counter, so two shards hand out the same ids
  for different files. `tp audit --merge` keeps the first row per `(role, item_id)`: an
  `error`-severity `FAIL` in the second shard disappears, the merge reports no findings, and the round
  records clean over files no recorded row graded.

`spec/0.30.0.md` §10.3 promised that the same subject keeps the same id across rounds, and
`skills/tp/REFERENCE.md` repeats the promise. Neither holds for files that share a deep directory.

### 2.1 The id, and what a prior row answers

**Decision 1.** A `file_check` item's id is a function of the role and the file's path, and of
nothing else — not its position in the list, not which other files share the list, not the shard it
was emitted in, not where the checkout lives or which directory tp runs from. The path is taken
relative to the project root — the directory tp resolves `.tp/` from — and cleaned, so a leading
`./`, a doubled separator, an absolute spelling or a spelling relative to a subdirectory does not make
a second id; a file outside that root is taken by its absolute path. The emitted file list names each
file once, by that path. The id keeps a readable prefix derived from the path, carries no positional
suffix, and ends in a fixed-length digest of the path of at least 64 bits.

**Decision 1b.** A prior round's row answers the current item whose id it carries, as today. A
`file_check` id that does not end in such a digest is of the earlier derivation: a prior row carrying
one answers no current item, and the prior-round section lists it apart from the rows that do, under a
line saying its id is of the earlier derivation.

**Why by id, and why the digest is in the spec.** The recorded id is the one fact the defect
corrupts, and only in rounds recorded before this release. `evidence_file` cannot stand in for it:
the audit output schema tells a grader to leave it null on a `FAIL`, and on other rows it can cite a
file other than the one the item names (sidecar, *Review round 2*). The digest's shape is what tells
an earlier id apart row by row, whichever binary recorded the round; its width is what keeps two paths
from sharing an id, which a 32-bit digest measurably did (sidecar, *Built before review*).

**Consequences.** Recorded rounds keep their ids; tp does not rewrite them. `scripts/audit-round-prep.py`
exits as it does when there is nothing to carry whenever the previous round holds a `file_check` row
of the earlier derivation, so the first round after the upgrade is graded in full rather than briefed
from rows no item answers; a Go test in the suite runs the script's shell test.

### 2.2 Two verdicts on one item

**Decision 2.** A row's **verdict** is its `status`, its `severity` and its disposition's status.
`tp audit --merge` collapses rows sharing `(role, item_id)` only when their verdicts agree, keeping
the first; rows whose verdicts differ are all kept, and each such group is reported under a
`conflicts` key naming every row's input, line and verdict. `tp audit <spec> --record` records every
row, as today, so a round holding a group is clean only when each of its rows is. A `--resolve`
selector `role:item_id` that names more than one row refuses with exit 2 and names their indexes.

**Why.** Decision 1 removes the known cause. Decision 2 ends the silent choice at every other one — a
role that misnames an id or writes two rows for one item, a hand-concatenated results file, one file
named in two overlapping shards. tp does not rank verdicts: every one that differs reaches the round,
and the round's own clean check, under whichever `audit_converge_on` resolves, decides what they add
up to. Nothing is refused, so the driver's record chain records as before.

**Consequences.** A hand-sharded round in which spec-coverage runs in every shard records each shard's
verdict on its items, and each shard sees a different file list, so the verdicts can differ for no
fault in the code; the skill tells an operator who shards by hand to run spec-coverage in one shard.
Text in `skills/tp/SKILL.md` and `skills/tp/REFERENCE.md` that describes the replaced behaviour is
rewritten with it.

## 3. Every closing commit counts

**Context.** A task closed with more than one commit — a production commit and a test commit, the
shape `tp done <id> "…" --commit <a> --commit <b>` records — contributes only its first commit's files
to spec-coverage's file list and to its checklist item's *files changed by task commit* evidence. The
other commits' files reach no conformance role, and nothing says so (sidecar, field report #32b).

**Decision.** Every audit derivation that reads a task's commits reads the same shas: every
`commit_shas` entry, or `commit_sha` when that is all a task carries. The task-acceptance item's
evidence names every file so mapped.

**Consequences.** More files map, and spec-coverage ranks its capped list by how many tasks map each
file, so which files survive the cap can change — a test file several tasks touched can displace a
production file. The cap and the ranking key are unchanged here; ranking is
`checklist-covers-what-changed`'s. A task whose first commit touched no audited file now maps the files
its other commits touched, so spec-coverage receives those instead of the fallback list it receives
when nothing maps (sidecar, *Built before review*).

## 4. A disposition that does not reach the round says so

**Context.** `skills/tp/SKILL.md` Workflow D records the round from `results.ndjson` in step 3 and
then, in step 4, tells the agent to resolve findings into `results.ndjson` — the merge output, which
the recorded round no longer reads. The disposition lands in a file the round does not read, and the
round does not change. Once every row in a file carries a disposition, the resolve payload offers a
`next_step` of re-recording that file. Under `tp run` — where the merge output in the round
directory is exactly where a unit disposes rows — that command rewrites the in-flight round in place,
the case it was written for; outside a run it appends a new round built from the previous round's rows
with no emission behind it. `--resolve-all` also writes a disposition onto `PASS` rows. All of this
is reproduced in the sidecar under field report #29.

**Decisions.** Rows are compared as JSON values, dispositions aside, never as bytes; a file with no
rows matches no round.

1. Workflow D step 4 names the **recorded round file** by its path shape — the round file under the
   spec's `.tp-review/` directory that step 3 recorded — as where a disposition written after step 3
   goes, and says that a disposition written into the merge output before step 3 is recorded with it.
   It is the interactive loop's step; a unit under `tp run` follows its own brief. The step needs no
   key a later release adds; when `--record` gains a `file` key (`a-findings-exits-agree`), it may
   cite that.
2. The re-record `next_step` is offered only inside a driver round — `TP_ROUND` set, and `TP_FILE`
   naming the task file whose spec it belongs to — when the file's rows are exactly the rows of that
   spec's round `TP_ROUND`, and every non-`PASS` row in the file carries a disposition. A file holding
   part of a round is never offered it, since recording it would replace the round with that part.
   Otherwise no resolve payload names `--record`.
3. Outside a driver round, a `--resolve` or `--resolve-all` into a file that is **not a recorded
   round** still writes. Its payload carries `recorded_round: false` and `matching_rounds`: each
   recorded round file, as an absolute path, whose rows include every row of the file — empty when
   none does. It says on stderr, with the command's other human-readable line, that the file is not a
   recorded round and that no round state changed. The search reads the recorded rounds, of the phase
   the command resolves, of every spec whose round state lives under the project root of the working
   directory or of the file, never through the active-file pointer, which can name another spec; a
   round state it cannot read is named on stderr, never skipped in silence. A recorded round is a file
   its spec's round state lists as a round's file, compared after resolving symlinks; a resolve into
   one carries `recorded_round: true`. Inside a driver round no search runs and the payload carries
   neither `matching_rounds` nor the statement, since the unit's brief decides where its dispositions
   go. **`tp review --resolve` and `--resolve-all` do the same**, because the same write into the
   same kind of file is lost the same way on the review side; there, `tp review --merge` rewrites the
   rows it merges, so a role's own findings file matches no round. Whether such a write should be
   refused, and what a disposition in a recorded round does to its verdict, stay with
   `a-finding-can-leave-an-audit-round`.
4. **`--resolve-all` skips `PASS` rows**, with or without `--force`, and counts them under
   `skipped_count`. A disposition answers a finding, and a `PASS` row is not one.

**Consequences.** `a-finding-can-leave-an-audit-round` makes a disposition in the recorded round file
change the round's verdict; this spec makes sure the agent writes it there and is told when it did
not. What a write into a non-recorded file may do to state is that spec's decision; this spec's §6
rows assert only what the payload says about it. The skill's step carries no guard test, for the
reason `CLAUDE.md` gives about `Contains` guards over prose; the audit reads it.

## 5. Non-Goals

1. **No ranking change.** How the code roles' files are ranked and cut is
   `checklist-covers-what-changed`.
2. **No acceptance or convergence change.** A disposition changes no recorded verdict here; that,
   and any refusal of a disposition into a non-recorded file, is `a-finding-can-leave-an-audit-round`.
3. **No section sharding.** Dividing a round is `round-divides-by-section`. This release makes a
   hand-sharded round merge honestly; it does not make sharding a tp feature.
4. **Spec-derived item ids are unchanged.** Only `file_check` ids change derivation.

## 6. Tests

Every row derives from the decision or consequence its `from` column names, and names a mutant that
must fail it. A row runs outside a driver round unless it sets `TP_ROUND` and `TP_FILE`. Rows whose
fixture runs at `HEAD` quote the value measured there; the value under the fix is the row's
assertion, and seeing it is the implementing task's acceptance. Rows marked *deferred* have a subject
that does not exist at `HEAD`, so both counts belong to that acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 d1 *rounds* | two files in one directory whose path slugs share their cut prefix, emitted in round 1; round 2 adds a third file that sorts before both. Each file's id is identical in both emissions. At `HEAD`: 2 of round 1's 2 ids name a different file in round 2 | the shipped derivation |
| 1b | §2.1 d1 *spelling* | one file named five ways — plain, with a leading `./`, with a doubled separator, by its absolute path, and relative to a subdirectory run from there — each alone with `--affected-files`, gets one id. At `HEAD`: the absolute and subdirectory spellings each get an id of their own | derive the id from the path as given |
| 1c | §2.1 d1 *checkout* | the same file in two clones whose absolute paths differ within their first ten characters, named by absolute path in each, gets one id. At `HEAD`: two ids | derive the id from the absolute path |
| 1d | §2.1 d1 *width* | the two paths of the sidecar whose 32-bit SHA-256 prefixes coincide, emitted together, get two ids. At `HEAD`: two ids | a 32-bit digest |
| 1e | §2.1 d1 *list* | one file named three ways in one `--affected-files` list yields one checklist item. At `HEAD`: three | normalise the id but not the list |
| 2 | §2.1 d1 *prior round* | same fixture as row 1, round 1 recording a `FAIL` on the first file: in round 2's prompt, the prior-round row and the checklist item carrying the same id name the same file. At `HEAD` they name different files | the shipped derivation |
| 3 | §2.1 d1 *shards* | four files in one deep directory, emitted as two shards of two with `--affected-files`; shard 2 carries an `error` `FAIL`. The merged output holds four distinct ids and the `FAIL`. At `HEAD`: `merged_count` 2, `duplicates_removed` 2, `findings` 0, exit 0, and the recorded round is clean | the shipped derivation |
| 4 | §2.1 d1b *old round* | a prior round recorded at `HEAD` holding a `FAIL` `file_check` row whose id carried no collision suffix: the prior-round section lists the row under the earlier-derivation line. At `HEAD`: listed with the other rows | keep the earlier id for a file whose id carried no suffix |
| 4b | §2.1 *prep* | `scripts/audit-round-prep.py` over a previous round recorded at `HEAD` exits as it does with nothing to carry. At `HEAD`: exit 0, carrying its untouched `PASS` rows | carry whatever the ids' derivation |
| 4c | §2.1 *prep, current rounds* | the same over a previous round recorded under decision 1 carries its untouched `PASS` rows. *Deferred* | carry nothing from any round |
| 5 | §2.2 d2 *status* | `--merge` over two inputs sharing one `(role, item_id)`, one `PASS` and one `FAIL`: the output holds both, `conflicts` names both input lines, exit 0. At `HEAD`: the first (`PASS`) kept, nothing reported | keep the first, the shipped dedup |
| 5b | §2.2 d2 *severity* | two `FAIL` inputs sharing one `(role, item_id)`, the first `severity: warning` and the second with no severity: both kept and reported, and `--record` of the output under `audit_converge_on=blocking` records the round not clean. At `HEAD`: the `warning` row kept, round clean | leave `severity` out of the verdict |
| 5c | §2.2 d2 *disposition* | two inputs with the same `status` and `severity`, the first carrying a `wontfix` disposition and the second none: both kept and reported. At `HEAD`: the first kept | leave the disposition out of the verdict |
| 5d | §2.2 d2 *three rows* | three inputs sharing one `(role, item_id)`, two agreeing and one not: the output holds one of the pair and the third, and `conflicts` names all three lines. *Deferred* | report pairs only |
| 5e | §2.2 d2 *agreeing* | two inputs holding the same verdict and differing in `note` and `evidence_file`: the first kept, nothing reported. At `HEAD`: the first kept | compare whole rows |
| 5f | §2.2 d2 *spec-coverage shards* | two shards whose spec-coverage rows on one spec-derived item are `PASS` in the first and `PARTIAL` in the second: both kept and reported, and the recorded round is not clean. At `HEAD`: the `PASS` kept, round clean | keep the first |
| 6 | §2.2 d2 *selector* | `--resolve security:<id>` on a round file holding two rows under that id exits 2 and names both indexes. At `HEAD`: exit 0, the first row disposed | resolve the first match, the shipped selector |
| 8 | §3 | a task closed with a production and a test commit: spec-coverage's `affected_files` and its task item's evidence both hold both files. At `HEAD`: 1 of 2, the production file only | read the first sha only |
| 9 | §3 *agreement* | same fixture with `--affected-from-tasks`: the audited universe and spec-coverage's list are the same set. At `HEAD`: 2 and 1 | read the first sha only in the task mapping |
| 9b | §3 *`commit_sha` only* | a done task carrying `commit_sha` and no `commit_shas`: `--affected-from-tasks` audits that commit's files. At `HEAD`: exit 4, no done task carries `commit_shas` | read `commit_shas` only in the universe derivation |
| 9c | §3 *exit-4 reason* | a done task carrying only a `commit_sha` that does not resolve: `--affected-from-tasks` exits 4 with the reason it gives an unresolvable `commit_shas` entry. At `HEAD`: the reason says no done task carries `commit_shas` | read `commit_shas` only when choosing the reason |
| 10 | §4 d2 *outside a run* | `--resolve-all` into a copy of a recorded round: the payload names no `--record`. At `HEAD` it does, and following it takes the recorded round count 1 → 2 with no emission | offer the step unconditionally, the shipped behaviour |
| 10b | §4 d2 *stale round* | the same inside a driver round whose `TP_ROUND` the file's rows are not: no `--record`. At `HEAD` it does | offer the step whenever `TP_ROUND` is set |
| 10c | §4 d2 *part of a round* | `--resolve-all` into one role's rows of round 1 inside driver round 1: no `--record`. At `HEAD` it is offered, and recording an empty file under `TP_ROUND=1` turns round 1 into a clean round of zero rows | offer the step when the round contains the file's rows |
| 11 | §4 d2 *inside a run* | `--resolve-all` into a copy of round 1 inside driver round 1: the step is offered. At `HEAD`: offered | drop the step altogether |
| 11b | §4 d2 *`PASS` rows* | `--resolve 0` on a copy of round 1 holding one `FAIL` and one undisposed `PASS`, inside driver round 1: the step is offered. At `HEAD`: not offered | require a disposition on `PASS` rows too, the shipped condition |
| 12 | §4 d3 | after `--record`, `--resolve 0 wontfix "<evidence>"` into the merge output exits 0, writes the disposition, carries `recorded_round: false` and `matching_rounds` naming the recorded round file, and says on stderr that no round state changed. At `HEAD` it carries none of them | omit them, the shipped payload |
| 12b | §4 d3 *pointer* | two specs each with a recorded audit round, the `tp use` pointer naming the other spec: the resolve into the first spec's merge output names the first spec's round file. *Deferred* | find the round through the active-file pointer |
| 12c | §4 d3 *review* | after `tp review <spec> --record merged.ndjson`, `tp review merged.ndjson --resolve 0 wontfix "<evidence>"` carries `recorded_round: false` and names the recorded review round file. At `HEAD` it carries neither | leave the review resolve payload as shipped |
| 12d | §4 d3 *review all* | the same with `tp review merged.ndjson --resolve-all wontfix "<evidence>"`. At `HEAD` it carries neither | leave the review resolve-all payload as shipped |
| 12e | §4 d3 *one role's file* | a resolve into one audit role's findings file, whose rows are a subset of the recorded round's, names that round. *Deferred* | match only a file whose rows equal a round's |
| 12f | §4 d3 *several* | a resolve into a file whose rows are in two recorded rounds names both. *Deferred* | name the first match |
| 12g | §4 d3 *in a run* | the resolve of row 12 inside a driver round carries neither `matching_rounds` nor the stderr statement. *Deferred* | search and state whenever the file is not a recorded round |
| 12h | §4 d3 *outside the project* | the merge output kept in a directory outside any project, resolved from the project root: `matching_rounds` names the round. *Deferred* | search only the project holding the file |
| 12i | §4 d3 *unreadable state* | another spec's round state made unreadable: the resolve exits 0, writes the disposition, and names that state on stderr. *Deferred* | skip an unreadable round state |
| 12j | §4 d3 *rewritten keys* | a hand-written merge output, resolved once so its rows are rewritten with sorted keys, still names the recorded round on the second resolve. *Deferred* | compare rows as bytes |
| 13 | §4 d3 *recorded* | the resolve into the recorded round file carries `recorded_round: true` and says nothing on stderr about round state. *Deferred* | make the statement unconditional |
| 13b | §4 d3 *symlink* | the recorded round file named through a symlinked directory carries `recorded_round: true`. *Deferred* | compare paths as text |
| 14 | §4 d4 | `--resolve-all` over one `FAIL` and one `PASS` row: `resolved_count` 1, `skipped_count` 1, the `PASS` row carries no `resolved` block. At `HEAD`: `resolved_count` 2 | disposition every row, the shipped loop |
| 14b | §4 d4 *force* | `--resolve-all --force` over a `PASS` row already carrying a disposition leaves its `resolved` block unchanged. At `HEAD`: overwritten | let `--force` reach `PASS` rows |
