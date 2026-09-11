# tp — An audit round records what was graded

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

**Decision 1.** A `file_check` item's id is a function of the role and the file's path **relative to
the repository root**, and of nothing else — not its position in the list, not which other files
share the list, not the shard it was emitted in, not where the checkout lives. The path is taken
cleaned and relative, so a leading `./`, a doubled separator or an absolute spelling of a file inside
the repository does not make a second id. The id keeps a readable prefix derived from the path and
carries no positional suffix.

**Decision 1b.** A prior round's `file_check` row answers the current item that decision 1 gives its
`role` and `evidence_file`, whatever id it was recorded under; a row without an `evidence_file`
answers none. The prior-round section lists each such row under that item's id. Spec-derived rows are
matched by id, and a round without an `id_scheme` marker is not comparable, both as today.

**Why by file.** The id a row was recorded under is the one fact the defect corrupts, and
`evidence_file` is the file the grader read. Matching on it makes every round recorded before this
release usable at once, with no marker to trust: a round's `id_scheme` is stamped by the binary that
records it, which need not be the one whose checklist the rows answer.

**Consequences.**

- Recorded rounds keep their ids; tp does not rewrite them, and the `id_scheme` value is unchanged.
- `scripts/audit-round-prep.py` already chooses carried rows by `evidence_file`; it now also lists a
  `file_check` row to re-measure by its `evidence_file`, not by its id alone, and its shell test runs
  in the project gate.
- A residual collision — two paths that decision 1 maps to one id — is not silent: decision 2 refuses
  the pair wherever it meets.

### 2.2 Two verdicts on one item

**Decision 2.** Two rows with the same `(role, item_id)` **conflict** when they carry different
verdicts — `status`, `severity`, or a disposition's status — or, for a `file_check` item, name
different files in `evidence_file`, compared as decision 1 takes a path. For a spec-derived item
`evidence_file` is the grader's citation rather than the item's subject, and every shard carries
spec-coverage's items, so it is not compared.

- `tp audit --merge` does not deduplicate a conflicting pair. It names every pair by input and line,
  exits 1 and, like its existing exit-1 path, still writes `-o`, holding both rows of each pair.
- `tp audit <spec> --record` refuses a file holding a conflicting pair: exit 1, every pair named by
  line at once, no round written. Its hint says a checklist item takes one row and that a pair is
  settled by re-grading the item, not by deleting either row.
- Rows with the same `(role, item_id)` that do not conflict are one row: `--merge` keeps the first,
  and `--record` records the first.

**Why.** Decision 1 removes the known cause. Decision 2 makes every other cause loud — a role that
misnames an id or writes two rows for one item, a hand-concatenated results file, one file named in two
overlapping shards, a residual collision — and ends a choice tp was never entitled to make silently:
which of two disagreeing verdicts on one item is the round's. Severity is part of a verdict because
under `audit_converge_on: blocking` it decides whether the round is clean; a disposition is, because
`a-finding-can-leave-an-audit-round` makes it decide the round. `--merge` keeps writing `-o` on this
exit so that a chain which records after the merge whatever its exit code — the driver's audit record
unit is one — ends in `--record`'s refusal naming the pair, not in a missing file.

**Consequences.** A `role:item_id` selector passed to `--resolve` can no longer dispose one file's row
while another file's verdict under the same id stays open. Text in `skills/tp/SKILL.md` and
`skills/tp/REFERENCE.md` that describes the replaced behaviour is rewritten with it.

## 3. Every closing commit counts

**Context.** A task closed with more than one commit — a production commit and a test commit, the
shape `tp done <id> "…" --commit <a> --commit <b>` records — contributes only its first commit's files
to spec-coverage's file list and to its checklist item's *files changed by task commit* evidence. The
other commits' files reach no conformance role, and nothing says so (sidecar, field report #32b).

**Decision.** Every audit derivation that reads a task's commits reads the same shas: every
`commit_shas` entry, or `commit_sha` when that is all a task carries. The task-acceptance item's
evidence names every file so mapped.

**Consequences.** Spec-coverage's cap and its ranking key are unchanged here;
`checklist-covers-what-changed`'s Non-Goals fence that key. A task whose first commit touched no
audited file now maps the files its other commits touched, so spec-coverage receives those instead
of the fallback list it receives when nothing maps (sidecar, *Built before review*).

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

**Decisions.**

1. Workflow D step 4 names the **recorded round file** by its path shape — the round file under the
   spec's `.tp-review/` directory that step 3 recorded — as where a disposition written after step 3
   goes, and says that a disposition written into the merge output before step 3 is recorded with it.
   It is the interactive loop's step: a unit under `tp run` follows its own brief, which disposes rows
   in the round directory's merge output and re-records it through decision 2. The step needs no key a
   later release adds; when `--record` gains a `file` key (`a-findings-exits-agree`), it may cite that.
2. The re-record `next_step` is offered only when a driver round is set (`TP_ROUND`), it names one of
   the recorded rounds the file's rows match (decision 3), and every non-`PASS` row in the file
   carries a disposition. Otherwise no resolve payload names `--record`.
3. A `--resolve` or `--resolve-all` into a file that is **not a recorded round** still writes. Its
   payload carries `recorded_round: false` and `matching_rounds`, the recorded round files whose rows
   include every row of the file, dispositions aside — empty when none does. When the payload offers
   decision 2's step, that step is what carries the disposition into the round; otherwise the payload
   says in words that the file is not a recorded round and that no round state changed. The search
   reads the recorded rounds of every spec whose round state lives in the repository holding the file,
   never through the active-file pointer, which can name another spec, and names in the payload any
   round state it cannot read rather than skipping it. A recorded round is a file its spec's round
   state lists as a round's file, compared after resolving symlinks; a resolve into one carries
   `recorded_round: true`. **`tp review --resolve` and `--resolve-all` do the same**, because the same
   write into the same kind of file is lost the same way on the review side. Whether such a write
   should be refused, and what a disposition in a recorded round does to its verdict, stay with
   `a-finding-can-leave-an-audit-round`.
4. **`--resolve-all` skips `PASS` rows**, with or without `--force`. A disposition answers a finding,
   and a `PASS` row is not one.

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

Every row derives from a numbered decision and names a mutant that must fail it. Rows whose fixture
runs at `HEAD` quote the value measured there; the value under the fix is the row's assertion, and
seeing it is the implementing task's acceptance. Rows marked *deferred* have a subject that does not
exist at `HEAD`, so both counts belong to that acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 d1 *rounds* | two files in one directory whose path slugs share their cut prefix, emitted in round 1; round 2 adds a third file that sorts before both. Each file's id is identical in both emissions. At `HEAD`: 2 of round 1's 2 ids name a different file in round 2 | restore the positional suffix |
| 1b | §2.1 d1 *spelling* | one file named four ways — plain, with a leading `./`, with a doubled separator, by its absolute path — each alone with `--affected-files`, gets one id. At `HEAD`: the absolute spelling gets an id of its own | derive the id from the path as given |
| 2 | §2.1 d1 *prior round* | same fixture as row 1, round 1 recording a `FAIL` on the first file: in round 2's prompt, the prior-round row and the checklist item carrying the same id name the same file. At `HEAD` they name different files | restore the positional suffix |
| 3 | §2.1 d1 *shards* | four files in one deep directory, emitted as two shards of two with `--affected-files`; shard 2 carries an `error` `FAIL`. The merged output holds four distinct ids and the `FAIL`. At `HEAD`: `merged_count` 2, `duplicates_removed` 2, `findings` 0, exit 0, and the recorded round is clean | restart the suffix per emission |
| 4 | §2.1 d1b *old round* | a prior round recorded at `HEAD` whose `file_check` row's id names, under decision 1, no current item: the prior-round section lists the row under the id decision 1 gives its `evidence_file`. At `HEAD`: listed under its recorded id | match prior rows by recorded id |
| 4b | §2.1 d1b *no file* | a prior `file_check` row without `evidence_file` is listed as answering no item. *Deferred* | fall back to the recorded id |
| 4c | §2.1 d1b *marker-less* | a prior round without an `id_scheme` marker is listed as not comparable as a whole. At `HEAD`: the same | match a marker-less round's rows by file too |
| 4d | §2.1 *prep* | `scripts/audit-round-prep.py` over a previous round holding a `FAIL` `file_check` row lists it to re-measure with its `evidence_file`. At `HEAD`: listed by id only | list re-measured rows by id, the shipped listing |
| 5 | §2.2 d2 *file* | `--merge` over two inputs sharing one `file_check` `(role, item_id)`, `status` and `severity`, naming different `evidence_file`s: exit 1, both input lines named, and `-o` holds both rows. At `HEAD`: exit 0, first row kept | compare `status` and `severity` only |
| 5b | §2.2 d2 *severity* | two inputs sharing one `(role, item_id)`, `evidence_file` and `status: FAIL`, one `severity: warning` and one `error`: `--merge` exits 1 naming both. At `HEAD`: exit 0, `duplicates_removed` 1, the `warning` row kept | leave `severity` out of the comparison |
| 5c | §2.2 d2 *disposition* | two inputs agreeing on every compared field but one carrying a `wontfix` disposition: `--merge` exits 1 naming both. At `HEAD`: exit 0, first row kept | leave the disposition out of the comparison |
| 5d | §2.2 d2 *citation* | two inputs sharing one spec-derived `(role, item_id)`, `status` and `severity`, citing different `evidence_file`s: `--merge` keeps one and exits 0. At `HEAD`: the same | compare `evidence_file` on every item |
| 6 | §2.2 d2 *record* | `--record` of one file holding the pair of row 5 exits 1, names both lines, and writes no round file. At `HEAD`: exit 0, both rows recorded under one id | record without the check |
| 6b | §2.2 d2 *record agreeing* | `--record` of one file holding the same `FAIL` row twice records one finding. At `HEAD`: `findings` 2 | record both rows |
| 7 | §2.2 d2 *agreeing* | `--merge` over two inputs holding the same row identically keeps one and exits 0. At `HEAD`: `merged_count` 1, `duplicates_removed` 1 | refuse on `(role, item_id)` alone, which would refuse any results file merged twice |
| 7b | §2.2 d2 *other fields* | two inputs agreeing on every compared field and differing in `note`: `--merge` keeps the first and exits 0. At `HEAD`: the same | compare whole rows |
| 8 | §3 | a task closed with a production and a test commit: spec-coverage's `affected_files` and its task item's evidence both hold both files. At `HEAD`: 1 of 2, the production file only | read the first sha only |
| 9 | §3 *agreement* | same fixture with `--affected-from-tasks`: the audited universe and spec-coverage's list are the same set. At `HEAD`: 2 and 1 | read the first sha only in the task mapping |
| 9b | §3 *`commit_sha` only* | a done task carrying `commit_sha` and no `commit_shas`: `--affected-from-tasks` audits that commit's files. At `HEAD`: exit 4, no done task carries `commit_shas` | read `commit_shas` only in the universe derivation |
| 9c | §3 *exit-4 reason* | a done task carrying only a `commit_sha` that does not resolve: `--affected-from-tasks` exits 4 with the reason it gives an unresolvable `commit_shas` entry. At `HEAD`: the reason says no done task carries `commit_shas` | read `commit_shas` only when choosing the reason |
| 10 | §4 d2 *outside a run* | `--resolve-all` into a file with `TP_ROUND` unset: the payload names no `--record`. At `HEAD` it does, and following it takes the recorded round count 1 → 2 with no emission | offer the step unconditionally, the shipped behaviour |
| 10b | §4 d2 *stale round* | the same, with `TP_ROUND` naming a round that is not one the file's rows match: the payload names no `--record`. At `HEAD` it does | offer the step whenever `TP_ROUND` is set |
| 11 | §4 d2 *inside a run* | `--resolve-all` with `TP_ROUND` naming the recorded round the file's rows match: the step is offered. At `HEAD`: offered | drop the step altogether |
| 11b | §4 d2 *`PASS` rows* | `--resolve 0` on a file of one `FAIL` and one undisposed `PASS`, `TP_ROUND` naming its round: the step is offered. At `HEAD`: not offered | require a disposition on `PASS` rows too, the shipped condition |
| 12 | §4 d3 | after `--record`, `--resolve 0 wontfix "<evidence>"` into the merge output exits 0, writes the disposition, and carries `recorded_round: false`, `matching_rounds` naming the recorded round file, and the no-state-changed statement. At `HEAD` the payload carries none of them | omit them, the shipped payload |
| 12b | §4 d3 *pointer* | two specs each with a recorded audit round, the `tp use` pointer naming the other spec: the resolve into the first spec's merge output names the first spec's round file. *Deferred* | find the round through the active-file pointer |
| 12c | §4 d3 *review* | after `tp review <spec> --record merged.ndjson`, `tp review merged.ndjson --resolve 0 wontfix "<evidence>"` carries `recorded_round: false` and names the recorded review round file. At `HEAD` the payload carries neither | leave the review resolve payload as shipped |
| 12d | §4 d3 *review all* | the same with `tp review merged.ndjson --resolve-all wontfix "<evidence>"`. At `HEAD` the payload carries neither | leave the review resolve-all payload as shipped |
| 12e | §4 d3 *one role's file* | a resolve into one role's findings file, whose rows are a subset of the recorded round's, names that recorded round. *Deferred* | match only a file whose rows equal a round's |
| 12f | §4 d3 *several* | a resolve into a file whose rows are in two recorded rounds names both. *Deferred* | name the first match |
| 12g | §4 d3 *in a run* | with `TP_ROUND` naming the round the file matches, the payload offers the step and does not say that no round state changed. *Deferred* | say it whenever the file is not a recorded round |
| 13 | §4 d3 *recorded* | the same resolve into the recorded round file carries `recorded_round: true` and no not-a-recorded-round statement. *Deferred* | make the statement unconditional |
| 13b | §4 d3 *symlink* | the recorded round file named through a symlinked directory carries `recorded_round: true`. *Deferred* | compare paths as text |
| 14 | §4 d4 | `--resolve-all` over one `FAIL` and one `PASS` row: `resolved_count` 1, the `PASS` row carries no `resolved` block. At `HEAD`: `resolved_count` 2 | disposition every row, the shipped loop |
| 14b | §4 d4 *force* | `--resolve-all --force` over a `PASS` row already carrying a disposition leaves that disposition byte-identical. At `HEAD`: overwritten | let `--force` reach `PASS` rows |
