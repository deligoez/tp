# tp — An audit round records what was graded

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `audit-records-what-was-graded-measurements.md` beside it, and this file stands
without them. It comes from a field report (WB-3155) verified claim by claim on 2026-09-11; each
verification, with its reproduction and source citations, is in the sidecar under *Field report
WB-3155, verified 2026-09-11*.

Class: **tool** — it changes which id a checklist item carries, which files the conformance role is
given and what the disposition path says, and no convergence rule. Budget it at the tool-class
median in `CLAUDE.md`'s *What a cycle costs*.

## 1. Overview

Three defects with one shape: **the recorded round holds something other than what was graded, and
nothing says so.** Each runs at exit 0 with nothing on stderr and a payload that reads normally.

1. **An item id can name a different file in a different round or shard** (§2).
2. **A task's second closing commit is not read** by the one role that measures conformance (§3).
3. **A disposition written where the skill says to write it does not reach the round**, and the
   step tp offers next fabricates one (§4).

It ships before `checklist-covers-what-changed`, which reorders the code roles' file list by churn:
a list whose order moves every round moves a positional suffix with it, so that release alone would
make §2's defect fire more often (sidecar, *Why this ships before the churn ranking*).

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

**Decision 1.** A `file_check` item's id is a function of the role and the file's **whole path**, and
of nothing else — not its position in the list, not which other files share the list, not the shard
it was emitted in. It keeps a readable prefix derived from the path and carries no positional
suffix. Audit rounds recorded under it carry a new `id_scheme` value.

**Decision 2.** `tp audit --merge` and `tp audit <spec> --record` **refuse** two rows with the same
`(role, item_id)` whose `evidence_file` or `status` differ, instead of keeping the first. The refusal
has the shape `--record` already gives a row outside the category enum: exit 1, every conflicting
pair named by input and line at once, nothing written. Rows that agree on both are not refused;
`--merge` still keeps one of them.

**Why both.** Decision 1 removes the known cause. Decision 2 makes every other cause loud — a role
that misnames an id, a hand-concatenated results file, a residual collision in the derivation — and
it ends a choice tp was never entitled to make silently: which of two disagreeing verdicts on one
item is the round's.

**Consequences.**

- The prior-round section treats a round recorded under an older `id_scheme` as not comparable, with
  the disclaimer marker-less rounds already get. The two schemes' ids are not built to match, so a
  string match across them would be an accident.
- Rounds already recorded keep their ids; tp does not rewrite them.
- A `role:item_id` selector passed to `--resolve` can no longer dispose one file's row while another
  file's verdict under the same id stays open.
- `skills/tp/REFERENCE.md`'s sentence on item ids is rewritten to the new derivation.
- `scripts/audit-round-prep.py` carries rows between rounds and lists the rest by id, so it inherits
  the fix: a carried row answers the item it was recorded against. Across the scheme change a carried
  row would bring an old-scheme id into a new-scheme round, so the script carries nothing from a round
  whose `id_scheme` differs from the current one.

## 3. Every closing commit counts

**Context.** A task closed with more than one commit — a production commit and a test commit, the
shape `tp done <id> "…" --commit <a> --commit <b>` records — contributes only its first commit's files
to spec-coverage's file list and to its checklist item's *files changed by task commit* evidence. The
other commits' files reach no conformance role, and nothing says so (sidecar, field report #32b).

**Decision.** Spec-coverage's task-to-file mapping reads every sha the task records: every
`commit_shas` entry, or `commit_sha` when that is all a task carries. The task-acceptance item's
evidence names every file so mapped.

**Consequences.** The mapped set can only grow. Which mapped files survive spec-coverage's own cap,
and in what order, is unchanged here; `checklist-covers-what-changed`'s Non-Goals fence that ranking
key.

## 4. A disposition that does not reach the round says so

**Context.** `skills/tp/SKILL.md` Workflow D records the round from `results.ndjson` in step 3 and
then, in step 4, tells the agent to resolve findings into `results.ndjson` — the merge output, which
the recorded round no longer reads. The disposition lands in a scratch file and the round does not
change. Once every row in a file carries a disposition, the resolve payload offers a `next_step` of
re-recording that file. Under `tp run` that command rewrites the in-flight round in place, which is
the case it was written for; outside a run it appends a new round built from the previous round's
rows with no emission behind it. `--resolve-all` also writes a disposition onto `PASS` rows. All of
this is reproduced in the sidecar under field report #29.

**Decisions.**

1. Workflow D step 4 names the **recorded round file** by its path shape — the round file under the
   spec's `.tp-review/` directory that step 3 recorded — as where a disposition written after step 3
   goes, and says that a disposition written into the merge output before step 3 is recorded with it.
   It needs no key a later release adds; when `--record` gains a `file` key
   (`a-findings-exits-agree`), the step may cite the key instead.
2. The re-record `next_step` is offered only when a driver round is set (`TP_ROUND`). Outside one, no
   resolve payload names `--record`.
3. A `--resolve` or `--resolve-all` into a file that is **not a recorded audit round** still writes,
   and its payload says so in words: the file is not a recorded round, no round state changed, and the
   recorded round whose rows the file's rows match — found by content across the project's recorded
   rounds, never through the active-file pointer, which can name another spec — or, when none matches,
   that none does. A recorded round is a file its spec's round state lists as a round's file. **`tp
   review --resolve` says the same**, because the same write into the same kind of file is lost the
   same way on the review side. Whether such a write should be refused, and what a disposition in a
   recorded round does to its verdict, stay with `a-finding-can-leave-an-audit-round`.
4. **`--resolve-all` skips `PASS` rows.** A disposition answers a finding, and a `PASS` row is not
   one. The payload counts skipped `PASS` rows apart from rows already carrying a disposition, and the
   condition for offering decision 2's `next_step` reads non-`PASS` rows only.

**Consequences.** `a-finding-can-leave-an-audit-round` makes a disposition in the recorded round file
change the round's verdict; this spec makes sure the agent writes it there and is told when it did
not. What a write into a non-recorded file may do to state is that spec's decision; this spec's §6
rows assert only what the payload says about it. The skill's step carries no guard test, for the
reason `CLAUDE.md` gives about `Contains` guards over prose; the audit reads it.

## 5. Non-Goals

1. **No ranking change.** Which files a role receives and in what order is
   `checklist-covers-what-changed`.
2. **No acceptance or convergence change.** A disposition changes no recorded verdict here; that,
   and any refusal of a disposition into a non-recorded file, is `a-finding-can-leave-an-audit-round`.
3. **No section sharding.** Dividing a round is `round-divides-by-section`. This release makes a
   hand-sharded round merge honestly; it does not make sharding a tp feature.
4. **Spec-derived item ids are unchanged.** Only `file_check` ids change scheme.

## 6. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Rows whose fixture
runs at `HEAD` quote the value measured there, which is also the value under the mutant, since the
mutant restores the shipped behaviour; the value under the fix is the row's assertion, and seeing it
is the implementing task's acceptance. Rows marked *deferred* have a subject that does not exist at
`HEAD`, so both counts belong to that acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 d1 *rounds* | two files in one directory whose path slugs share their cut prefix, emitted in round 1; round 2 adds a third file that sorts before both. Each file's id is identical in both emissions. At `HEAD`: 2 of round 1's 2 ids name a different file in round 2 | restore the positional suffix |
| 2 | §2 d1 *prior round* | same fixture, round 1 recording a `FAIL` on the first file: in round 2's prompt, the prior-round row and the checklist item carrying the same id name the same file. At `HEAD` they name different files | restore the positional suffix |
| 3 | §2 d1 *shards* | four files in one deep directory, emitted as two shards of two with `--affected-files`; shard 2 carries an `error` `FAIL`. The merged output holds four distinct ids and the `FAIL`. At `HEAD`: `merged_count` 2, `duplicates_removed` 2, `findings` 0, exit 0, and the recorded round is clean | restart the suffix per emission |
| 4 | §2 d2 *merge* | `--merge` over two hand-written inputs sharing one `(role, item_id)` with different `evidence_file` and `status` exits 1, names both input lines, and writes no output file. At `HEAD`: exit 0, first row kept | keep the first row, the shipped dedup |
| 5 | §2 d2 *record* | `--record` of one file holding that conflict exits 1, names both lines, and writes no round file. At `HEAD`: exit 0, both rows recorded under one id | record without the check |
| 6 | §2 d2 *agreeing* | `--merge` over two inputs holding the same row identically keeps one and exits 0. At `HEAD`: `merged_count` 1, `duplicates_removed` 1 | refuse on `(role, item_id)` alone, which would refuse any results file merged twice |
| 7 | §2 *scheme* | a prior round recorded under the older `id_scheme` value is rendered with the not-comparable disclaimer. *Deferred* | treat only a marker-less round as not comparable, the shipped predicate |
| 8 | §3 | a task closed with a production and a test commit: spec-coverage's `affected_files` and its task item's evidence both hold both files. At `HEAD`: 1 of 2, the production file only | read the first sha only |
| 9 | §3 *agreement* | same fixture with `--affected-from-tasks`: the audited universe and spec-coverage's list are the same set. At `HEAD`: 2 and 1 | read the first sha only in the task mapping |
| 10 | §4 d2 *outside a run* | `--resolve-all` into a file with `TP_ROUND` unset: the payload names no `--record`. At `HEAD` it does, and following it takes the recorded round count 1 → 2 with no emission | offer the step unconditionally, the shipped behaviour |
| 11 | §4 d2 *inside a run* | with `TP_ROUND` naming the recorded round, the step is offered, and following it leaves the round count unchanged. At `HEAD`: offered, count unchanged | drop the step altogether, which breaks the driver |
| 12 | §4 d3 | after `--record`, `--resolve 0 wontfix "<evidence>"` into the merge output exits 0, writes the disposition, and its payload says the file is not a recorded round, that state is unchanged, and names the recorded round file. At `HEAD` the payload carries none of the three | omit the statement, the shipped payload |
| 12b | §4 d3 *pointer* | two specs each with a recorded audit round, the `tp use` pointer naming the other spec: the resolve into the first spec's merge output names the first spec's round file. *Deferred* | find the round through the active-file pointer, which names the other spec's round |
| 12c | §4 d3 *review* | after `tp review <spec> --record merged.ndjson`, `tp review merged.ndjson --resolve 0 wontfix "<evidence>"` says the file is not a recorded round and names the recorded review round file. At `HEAD` the payload carries none of it | leave the review resolve payload as shipped |
| 13 | §4 d3 *recorded* | the same resolve into the recorded round file carries no not-a-recorded-round statement. *Deferred* | make the statement unconditional, which says nothing |
| 14 | §4 d4 | `--resolve-all` over one `FAIL` and one `PASS` row: `resolved_count` 1, the `PASS` row carries no `resolved` block. At `HEAD`: `resolved_count` 2 | disposition every row, the shipped loop |
