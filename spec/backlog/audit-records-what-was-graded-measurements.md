# audit-records-what-was-graded — measurements

Supplemental material for `audit-records-what-was-graded.md`; the spec stands without it. **This file
is not a spec and `tp ground` never grades it.** Every reproduction below was run on 2026-09-11 with
a binary built from `18032abe` (`tp version v1.1.2-0.20260910212136-18032abe405f`), each in a fresh
`git init` repository outside this one, so no probe wrote round state here. Source citations are at
`18032abe`.

---

## Field report WB-3155, verified 2026-09-11

A field report (WB-3155), from a Laravel project on tp v1.1.1, listed its friction points as numbered
items. The three this spec takes are below; the report's other items are verified in the sidecars of
the specs that take them. Refuted parts are said plainly.

### #27 — item ids re-point across rounds and collide across shards: CONFIRMED, and worse than reported

**The claim.** File-check ids are not stable: they are a cut prefix of the path plus a positional
suffix, so files in one directory share the prefix and the list's order decides which is `-2`. The
report saw it when an auditor noticed that a round-1 `PARTIAL` on
`CalculateRevenueBonusCampaignActionTest.php` carried an id that in round 2 named
`EventAndCommandDiscoveryTest.php`, so the round-2 `PASS` under that id looked like it closed the
finding by reading another file, and the convergence count took the round as clean. It proposed an id
derived from the whole path, or with a path hash appended, and no positional suffix. Verification
found the same mechanism also loses rows across shards, which the report did not claim.

**The derivation.** `fileCheckItems` (`internal/cli/audit_roles.go:98-121`) builds each id as
`file-<role>-` plus `slugifySubject(path + " " + text)`, and appends `-2`, `-3`, … when a slug repeats
within one emission (`:105-110`). `slugifySubject` (`:129-149`) lowercases, collapses non-alphanumeric
runs to `-`, and cuts the result to 40 characters (`:145-147`). The path comes first, so any path whose
slug is 40 characters or longer contributes nothing after its first 40: every file under
`tests/Services/Campaigns/RevenueBonusCampaign/` gets the slug
`tests-services-campaigns-revenuebonuscam`, and only the suffix tells them apart. The suffix is
positional, which is exactly what `spec/0.30.0.md` §10.3 set out to remove.

**Run A, across rounds (the spec's §6 rows 1–2).** A repository with a one-section spec and two files
in that directory, `CalculateRevenueBonusCampaignActionTest.php` and
`EventAndCommandDiscoveryTest.php`:

```
tp audit spec.md --affected-files <Calculate…>,<Event…>
  file-security-tests-services-campaigns-revenuebonuscam     -> Calculate…
  file-security-tests-services-campaigns-revenuebonuscam-2   -> Event…
tp audit spec.md --record r1.ndjson       # security FAIL (error) on the bare id, Calculate…
exit 0, round 1, findings 1, clean false
# commit AlphaListenerTest.php in the same directory, then:
tp audit spec.md --affected-files <Alpha…>,<Calculate…>,<Event…>
  file-security-tests-services-campaigns-revenuebonuscam     -> AlphaListenerTest.php
  file-security-tests-services-campaigns-revenuebonuscam-2   -> Calculate…
  file-security-tests-services-campaigns-revenuebonuscam-3   -> Event…
```

Both of round 1's ids name a different file in round 2. The round-2 security prompt's prior-round
section reads:

```
{"role":"security","item_id":"file-security-tests-services-campaigns-revenuebonuscam","status":"FAIL",
 "evidence_file":"tests/Services/Campaigns/RevenueBonusCampaign/CalculateRevenueBonusCampaignActionTest.php", …}
```

— the id the checklist now attaches to `AlphaListenerTest.php`. A role that grades Alpha `PASS` under
that id has answered the prior `FAIL` by id while never looking at the file it was about. The
verification unit's run on a fixture shaped like the report's went one step further: its round 2
re-pointed Calculate's id to the Event file, round 2 was all `PASS` and clean, round 3 converged, and
the Calculate file was never re-graded.

**Run B, across shards (rows 3–5).** Four files in the same directory, `AlphaTest`, `BravoTest`,
`CharlieTest`, `DeltaTest`, emitted as two shards of two with `--role security --affected-files`:

```
shard 1: …-revenuebonuscam -> AlphaTest.php     …-revenuebonuscam-2 -> BravoTest.php
shard 2: …-revenuebonuscam -> CharlieTest.php   …-revenuebonuscam-2 -> DeltaTest.php
```

Shard 1's rows all `PASS`; shard 2 records `FAIL`, `severity: error`, on Charlie.

```
tp audit --merge s1-security.ndjson s2-security.ndjson -o merged.ndjson
exit 0, stderr empty
{"by_status":{"PASS":2}, "duplicates_removed":2, "findings":0, "merged_count":2, …}
tp audit spec.md --record merged.ndjson
exit 0, round 1, findings 0, clean true, consecutive_clean 1
```

The `error` `FAIL` is gone, and Charlie and Delta have no recorded row at all. The drop is
`dedupAuditRows` (`internal/cli/audit_merge.go:212-227`), which keeps the first row per
`(role, item_id)`. The verification unit's run at the report's own scale — 25 files, three shards of
ten, ten and five — measured `merged_count` 10, `duplicates_removed` 15, `findings` 0, a clean round, and
a converged loop at `--status --check` exit 0, with 15 of the 25 files never recorded.
Sharding by hand is what the truncation notice at `internal/cli/audit.go:777` recommends:
*"%d files changed, auditing first %d — name the rest with --affected-files"*.

**`--record` does not look either.** The two shard files concatenated and recorded without `--merge`:
exit 0, stderr empty, findings 1, and the round file holds four rows, two pairs of which share an id
while naming different files and different verdicts. In that round `tp audit <round file> --resolve
security:file-security-tests-services-campaigns-revenuebonuscam wontfix "…"` resolved **index 0** —
Alpha's `PASS` — and left Charlie's `FAIL` untouched; `auditRowIndex`
(`internal/cli/audit_resolve.go:164-175`) returns the first match.

**What was promised.** `spec/0.30.0.md` §10.3: *"so the same subject keeps the same id across rounds
and a reordered checklist does not renumber unrelated items"*; §10.9 builds the cross-scheme merge
safety on it. `skills/tp/REFERENCE.md` line 766 repeats it: *"so the same file keeps the same id across
rounds"*.

**Why tp's own dogfooding never hit it.** Recomputing each recorded `file_check` row's id from its
own `evidence_file` over both round globs finds no row carrying a collision suffix; and over tp's
tracked `*.go` files no two share a slug, for any of the three code roles. The tree is not
collision-free, though, and an earlier note said it was: over every tracked file the audit would treat
as auditable, collisions exist — almost all among recorded round files under `.tp-review/`, which
share a long directory prefix, and the rest among `internal/engine/corpus/software/{auditors,
reviewers}/*.json`. None of them ever reached one checklist together. Derivation at `18032abe`:

```bash
python3 - <<'PY'
import subprocess, re, collections, os, json, glob
def slug(s):
    s = re.sub(r'[^a-z0-9]+', '-', s.lower()).strip('-')
    return s[:40].rstrip('-') if len(s) > 40 else s
BIN = {".png",".jpg",".jpeg",".gif",".svg",".ico",".woff",".woff2",".ttf",".eot",
       ".zip",".tar",".gz",".pdf",".exe",".dll",".so",".dylib",".o",".a"}
def auditable(p):
    b = os.path.basename(p)
    return not (os.path.splitext(p)[1].lower() in BIN or p.endswith(('.md', '.tasks.json'))
                or '.tp' in p.split('/') or (b.startswith('.') and '.' not in b[1:])
                or p.endswith('.golden') or p.startswith('testdata/') or '/testdata/' in p)
files = subprocess.run(['git', 'ls-files'], capture_output=True, text=True).stdout.split()
for label, sel in (('*.go', [f for f in files if f.endswith('.go')]),
                   ('auditable', [f for f in files if auditable(f)])):
    c = collections.Counter(slug(f + ' Apply the go-safety role rules to ' + f) for f in sel)
    print(label, len(sel), 'files sharing a slug:', sum(v for v in c.values() if v > 1))
suffixed = 0
for pat in ('spec/.tp-review/*/audit-round-*.ndjson', 'spec/backlog/.tp-review/*/audit-round-*.ndjson'):
    for f in glob.glob(pat):
        for l in open(f):
            if not l.strip(): continue
            r = json.loads(l); role = r.get('role') or ''; ev = r.get('evidence_file') or ''
            i = r.get('item_id') or ''
            if ev and i.startswith('file-' + role + '-'):
                s = slug(ev + ' Apply the %s role rules to %s' % (role, ev))
                if re.fullmatch(re.escape(s) + r'-\d+', i[len('file-' + role + '-'):]): suffixed += 1
print('recorded rows carrying a collision suffix:', suffixed)
PY
```

The auditable-file filter mirrors `isAuditableType` (`internal/cli/audit.go:880-897`) and
`filterAuditUniverse`'s fixture drops; it is an approximation of the shipped filter, not a call to it.

### #32b — a task's second closing commit is not read: CONFIRMED (a different defect from the one reported)

**The claim**, as reported: `--affected-from-tasks` dropped the test files from round 1's list,
because batch closes could not carry a `commit` array (the report's #25) and tasks were therefore
closed with their production commit only. That cause is real and belongs to
`a-task-file-write-names-its-target`, which takes #25; given both shas, `--affected-from-tasks` reads
both (below). The report's second half — `--base` pulling in files merged from the base branch — is
verified, and its proposed cause refuted, in `checklist-covers-what-changed-measurements.md`.
Verification found a different loss in the same area, which is what this spec takes.

**Reproduction (the spec's §6 rows 8–9).** A spec with one section and one task whose acceptance cites
it; `app/calc.go` committed, then `tests/calc_test.go` in a second commit; the task closed with
`tp done calc "…" --commit <prod sha> --commit <test sha>`. The task file then holds
`commit_sha: <prod>` and `commit_shas: [<prod>, <test>]`.

```
tp audit spec.md
exit 0, stderr empty
files:                         ["app/calc.go", "tests/calc_test.go"]
spec-coverage affected_files:  [{"path":"app/calc.go","tasks":["calc"],…}]
task-calc expected_evidence:   "files changed by task commit: app/calc.go"
security / maintainability:    both files
```

The test commit's file reaches every code-lens role and not the conformance role.
`GitTaskFileMapping` (`internal/engine/auditfiles.go:209-240`) reads `tasks[i].CommitSHA` only
(`:217-231`) — the mirror of `commit_shas[0]` — and spec-coverage's selection keeps only task-mapped
files. **The two task derivations disagree today**: on the same fixture, `tp audit spec.md
--affected-from-tasks` audits `["app/calc.go", "tests/calc_test.go"]`, so the universe derivation
already reads every sha while the mapping beside it reads one.

### #29 — a disposition that goes nowhere, and a `next_step` that fabricates a round: CONFIRMED

**The claim.** The report resolved a round's 75 rows into the scratch file it had passed to
`--record`; `--record` had copied that file to the round's own `audit-round-2.ndjson`, every one of the
75 `--resolve` calls reported success, and none reached state. It had made the same mistake on the
review side earlier (its #16) and concluded the fault is the interface: a path is accepted in place of
a round, and the success output does not say which it wrote. It proposed, strongest first: (a)
`--resolve` takes a round id rather than a path; (b) a path that is not a recorded round is refused;
(c) the success output names its target — *"not a recorded round, state unchanged"*; (d) `--record`
says which file to resolve against. This spec takes (c); (d) is the `file` key
`a-findings-exits-agree` gives `--record`; (a) and (b) are design questions that stay with
`a-finding-can-leave-an-audit-round`.

**The instruction.** `skills/tp/SKILL.md` line 385 (Workflow D step 3) records with
`tp audit <spec> --record results.ndjson`; line 386 (step 4) then says *"record how each was closed:
`tp audit results.ndjson --resolve …`"* — the merge output, after the round was recorded from it.

**Reproduction (rows 10–14).** One file pair, one role, a merge file of one `FAIL` and one `PASS`,
recorded as round 1 (`findings 1, clean false`). With `TP_ROUND` unset:

```
tp audit merged.ndjson --resolve 0 wontfix "accepted: out of scope"
exit 0
{"evidence":"accepted: out of scope","file":"<…>/merged.ndjson","index":0,"selector":"0","status":"wontfix"}
tp audit spec.md --status        -> still round 1, findings 1, not clean
```

The payload says nothing about where the round lives. A single `--resolve` offers no `next_step` here
because `allFindingsResolved` (`internal/cli/review_resolve.go:248-255`) requires a disposition on
every row, `PASS` rows included. `--resolve-all` always offers one
(`internal/cli/audit_resolve.go:150-156`):

```
tp audit merged.ndjson --resolve-all wontfix "accepted: out of scope"
stderr: resolved 2 audit rows as wontfix (0 already resolved, skipped)
{"next_step":"tp audit <spec> --record <…>/merged.ndjson","resolved_count":2,…}
# merged.ndjson now: FAIL -> wontfix, PASS -> wontfix
tp audit spec.md --record merged.ndjson          # following next_step verbatim
exit 0, round 2, findings 1, clean false
tp audit .tp-review/spec/audit-round-1.ndjson --resolve-all wontfix "accepted" --force
{"next_step":"tp audit <spec> --record .tp-review/spec/audit-round-1.ndjson", …}
tp audit spec.md --record .tp-review/spec/audit-round-1.ndjson
exit 0, round 3, findings 1, clean false
ls .tp-review/spec/   ->  audit-round-1.ndjson audit-round-2.ndjson audit-round-3.ndjson
                          snapshot-audit-round-1.md state.json
```

Rounds 2 and 3 are round 1's rows re-recorded, and neither has a snapshot of its own: no emission
stands behind them, yet each is a recorded round like any other. The
`next_step` text is `auditResolveNextStep` (`internal/cli/audit_resolve.go:243-248`); its doc comment
names the driver's `$TP_ROUND_DIR/merged.ndjson` (`:27-31`), and the recorder is *"idempotent on
TP_ROUND, additive by hand"* (`internal/cli/audit_record.go:231-254`). Measured: with `TP_ROUND=3`,
recording a copy of round 3 left the round count at 3. So the step is right inside a run and
fabricates a round outside one.

**Side finding, decided in the spec's §4 d4.** `--resolve-all` writes a disposition onto `PASS` rows
(the loop at `internal/cli/audit_resolve.go:132-139` skips only rows already carrying one), as the
`resolved 2 audit rows` line above shows for a file holding one finding.

**How this relates to the acceptance spec.** `a-finding-can-leave-an-audit-round` makes a disposition
in the recorded round file change the round's verdict, and decides what a disposition into any other
file may do to state. At `HEAD` that mistake is silent; this spec's §4 d3 is the sentence that tells
the operator, whichever way that design lands.

## Why this ships before the churn ranking

`checklist-covers-what-changed` §2 orders the code roles' list by churn. Churn changes every round, so
the order of files inside one deep directory changes with it, and every change of order under the
shipped derivation moves a positional suffix onto another file. Churn ordering alone would therefore
make Run A's re-point happen in rounds where alphabetical order would have held still. Stable ids are
the precondition for reordering at all, which is why this spec is first in the backlog and the churn
ranking follows it.

## A candidate derivation, not a decision

The verification unit proposed `file-<role>-<slug40>-<first 8 hex of sha256(path)>`, keeping today's
readable prefix and adding a digest of the whole path. It satisfies decision 1; whether the digest is
eight characters or more, and which hash, belong to the implementing task. A residual digest collision
is not silent under decision 2: the two rows it would produce share an id and name different files,
which is the conflict `--merge` and `--record` refuse.

## Built before review

On 2026-09-11 the spec as written at `bd4a18f1` was built in a `git clone --no-hardlinks` of that
commit (17 files, +597/−120), per `spec/backlog/README.md`'s *How to pick one up*. Every §6 row's
fixture was run against a binary built from `bd4a18f1` and one built from the clone, and every named
mutant was applied in the clone, built, run and reverted. All the rows that existed then reproduced
their quoted `HEAD` values, all changed under the build, and every mutant reddened its row — with the
two exceptions below. The clone, the diff and the fixture harness were scratch and are not kept; the
findings are.

**What building changed in the spec.**

- *§6's preamble* said the value under a mutant is the `HEAD` value. Not for row 3: with decision 2
  built, restoring the positional suffix makes `--merge` exit 1 on the colliding pair, where `HEAD`
  exits 0 and records a clean round. The row still reddens; the clause was dropped.
- *§2's "Why both"* said decision 2 ends tp's silent choice between two verdicts. It did not while the
  conflict key was `evidence_file` and `status`: two rows with the same id, file and `FAIL`, one
  `severity: error` and one `warning`, under `audit_converge_on=blocking`, merged at exit 0 with
  `duplicates_removed 1, by_severity {warning: 1}` and recorded `findings 1, clean: true` — at `HEAD`
  and under the build alike. Severity joined the key; row 4b is that input.
- *§2's consequence* said an older-scheme round gets "the disclaimer marker-less rounds already get".
  That disclaimer calls the ids *"positional (file-<role>-<n>)"*, false for a `"slug"` round, so it
  cannot be reused verbatim; the spec now says only that such a round is not comparable.
- *Decision 1* said "whole path". A digest over the path as given gave `app/a.go`, `./app/a.go` and
  `app//a.go` three ids where `HEAD`'s slug gave one, because `--affected-files` does not normalise. The
  spec now says the path is taken cleaned; row 1b.
- *Decision 1* said rounds "recorded under it" carry the new `id_scheme`. The recorder stamps its own
  scheme, so a round emitted by the old binary and recorded by the new one was stamped with the new
  value and the next prompt showed an id from no checklist without the disclaimer. tp's own rule —
  rebuild the dev binary after every implementing commit — makes that sequence routine here. The spec
  now says the scheme is the emission's; row 7b.
- *Decision 1* had no uniqueness clause. A birthday search over one deep directory found two paths
  whose 8-hex SHA-256 digests coincide (`…/F42057Test.php`, `…/F48508Test.php`): emission exited 0 and
  handed out one id twice, and `--merge`/`--record` then refused a pair of which neither row is wrong,
  with no way through but dropping a verdict. The spec now says no emission hands out one id for two
  files; row 1c.
- *§3* said "the mapped set can only grow", true of the set and false of what spec-coverage receives:
  a task whose first commit touched only a doc and whose second touched `app/x.go` maps nothing at
  `HEAD`, so spec-coverage fell back to all three named files; under the build it received `app/x.go`
  alone. The sentence went; the consequence now says so.
- *§3 row 9* held only on its fixture. A done task carrying `commit_sha` and no `commit_shas` mapped
  under the build while `--affected-from-tasks` exited 4 (*"no done task carries commit_shas"*), so
  the two derivations disagreed in the other direction. The decision now binds both; row 9b.
- *§4 decision 2* said "when `TP_ROUND` is set". With `TP_ROUND=7` and one recorded round the step was
  still offered, and following it appended round 2. The decision now requires `TP_ROUND` to be the
  round the file's rows match; row 10b.
- *§4 decision 3* said "the recorded round whose rows the file's rows match". Built as equality, a
  resolve into one role's findings file — whose rows are all in round 1 — read "matches no recorded
  round". The decision now says containment; row 12d. Built from the working directory alone, the
  search also gave a false "no match" when resolving a file in another repository from a parent
  directory; searching from the file's own project root as well fixed it.
- *§6 row 11*'s "following it leaves the round count unchanged" could not be reddened by its mutant:
  the count holds because the recorder is idempotent on `TP_ROUND`, which this spec does not touch.
  The clause went.

**Existing tests the build changed**, each a consequence of a decision rather than a sign against it:
`TestIDScheme_RecordedOnAuditOnly` and `TestIDScheme_LegacyRoundStaysMarkerless` (expect `"slug"`),
`TestIsLegacyRound_DetectionByMarker` (a `"slug"` round is comparable), `TestFileCheckItems_CollisionSuffix`
(asserts `-2`), `TestAuditResolveAll_DisposesEveryUndisposedRow` (a `PASS` row gets a disposition),
`TestAuditPriorRound_LegacyRoundDisclaimer` (asserts `"positional"`), and ten assertions of
`scripts/audit-round-prep-test.sh`, whose fixture has no `state.json` and so reads as scheme `""`.
The shell test is in no gate. Two assertions a probe added to it stayed green under their mutant
because the case ran on a tree where nothing would be carried anyway — the implementing task must
run the older-scheme case on a tree where something would be.

**Surfaces the build found beyond the spec's own list.** `skills/tp/SKILL.md` Workflow D's rule 3
(*"a `file_check` id is a path prefix cut short plus a positional suffix"*), its step 3 (*"dedups by
`role`+`item_id`"*) and the `--resolve-all` inventory line (*"every undisposed audit row"*);
`runAuditResolve`'s doc comment and `auditResolveNextStep`, which name `$TP_ROUND_DIR/merged.ndjson`;
and `internal/cli/docs_contract_test.go`, whose guard `Contains "file-<role-id>-<slug>"` stays green
whether or not `REFERENCE.md` is rewritten, since the new form contains the old one. No test at
`bd4a18f1` asserts a resolve payload's `next_step`.

**Corpus check for decision 2.** Over both round globs, 115 recorded audit rounds and 13,420 rows
hold 13,420 distinct `(role, item_id)` keys at `bd4a18f1`: decision 2 would have refused none of tp's
own rounds.

**Left to the implementing tasks** (a sentence that would change with the implementation is not
spec): the digest and its length, the `id_scheme` string — which `scripts/audit-round-prep.py` must
read from the same constant the binary uses — the refusal's wording, and the resolve payload's key
names.

## Ground round 1

`spec/backlog/.tp-review/audit-records-what-was-graded/ground-round-1.ndjson` holds the round. Its
six `PARTIAL` rows were each repaired by narrowing or deleting the sentence; one of them measured
something worth keeping.

**A stale `TP_ROUND` does worse than append.** Row 10b once said that following the step under a
`TP_ROUND` the file does not match *appends a round*. That holds only when `TP_ROUND` is past the last
recorded round. When it names an **earlier** recorded round, following the `next_step` rewrites that
round's file: with two rounds recorded and `TP_ROUND=1`, round 1's file was replaced by round 2's
rows. The row now asserts only what decision 2 changes — that the payload names no `--record` — since
both outcomes are what the mutant restores.

## Ground round 2

Two of the six repaired sentences were still `PARTIAL`, and both repairs were subtractions.

- **The budget sentence went.** It named the median `CLAUDE.md` gives a release that is not about the
  loop. The grader measured that `spec/1.1.0.md` was re-classed a loop release because it changed what
  `--record` accepts and what the emitted prompt says — and decisions 1 and 2 here do both — while
  `spec/backlog/README.md` lists this spec as tool. Which class holds is a budgeting call the spec does
  not need to make; the class line stays as the README's.
- **The documentation list went.** Its four named passages exist, but read as the list of text the
  decisions make false it was short by the `REFERENCE.md` passages that say a refused `tp audit
  --merge` still writes `-o` before exiting 1 (lines 1109–1116, and 186 with 199–202 on the record
  step that runs against that leftover). Decision 2 adds an exit-1 path that writes nothing. The spec
  now says only that documentation describing replaced behaviour is rewritten; the passages found so
  far, for the documentation task: `REFERENCE.md` 766, 186, 199–202, 1109–1116; `SKILL.md` 385, 423–426,
  675, 678 (line numbers at `30965923`).

## Review round 1

`spec/backlog/.tp-review/audit-records-what-was-graded/review-round-1.ndjson` holds the round: four
roles, 48 findings, every one dispositioned in that file. Three clusters moved the design; the rest
narrowed wording or added the row a clause lacked.

**The driver disposes rows in the merge output, so §4 had to say where it does not apply.** Under
`tp run` the round directory's `merged.ndjson` is what the record unit writes and what `review-resolve`
and `audit-fix` units dispose rows in (`engine.MergedFindingsPath`'s doc comment, `internal/engine/unitkind.go`),
and the unit's durable-write predicate reads it. Three roles found independently that decision 1's
step, read as universal, would send a unit away from the file its own predicate reads, and that
decision 3's statement would fire on every in-run resolve. Decision 1 now names itself the interactive
loop's step, and decision 3's statement is replaced by decision 2's step whenever that step is offered.

**Scheme stamps were the wrong thing to trust; `evidence_file` is the right one.** Round 1 found
decision 1's *"the emission's scheme"* had nowhere to live — an audit emission writes only its
snapshot, and `--record` stamps `id_scheme` — and that throwing away a whole older-scheme round also
threw away spec-coverage's rows, whose ids do not change. The repair dropped the new `id_scheme` value
and the emission-scheme clause altogether: a prior `file_check` row now answers the item decision 1
gives its `role` and `evidence_file`, whatever id it was recorded under. That makes rounds recorded
before the release usable at once and needs no marker. `scripts/audit-round-prep.py` already chooses
carried rows by `evidence_file`; only its re-measure list changes. **Rows 1c and 7b, and the
"no emission hands out one id twice" clause, were withdrawn**: with the id a function of the path
alone, row 1c's colliding pair could not be given two ids without breaking decision 1, and row 7b
tested the emission stamp that no longer exists. A residual collision is left to decision 2.

**`evidence_file` is the subject only of a `file_check` item.** For a spec-derived item it is the
grader's citation, and every hand-sharded emission re-emits spec-coverage's items under the same ids,
so decision 2 as written would have refused every hand-sharded round run with the full panel — the
case Non-Goal 3 says the release makes merge honestly. The comparison is now scoped to `file_check`
items, gained the disposition's status, and `--merge` keeps writing `-o` on the conflict exit, as on
its existing exit-1 path, so the driver's `;`-chained audit record unit reaches `--record`'s refusal
instead of a missing file.

## Review round 2

`spec/backlog/.tp-review/audit-records-what-was-graded/review-round-2.ndjson` holds the round: five
prompts including regression, 45 findings. Two of round 1's repairs did not survive, and the
replacements are simpler than what they replace. **§6 was renumbered in this round**, so a row number
cited in an earlier section of this file names the table as it stood then.

**Matching a prior row by `evidence_file` dropped the rows it was meant to save.** The audit output
schema tells a grader to leave `evidence_file` null on a `FAIL` (`internal/cli/audit_schema.go`), and
four roles measured the corpus independently: in `slug`-stamped rounds most `file_check` `FAIL` rows
carry none, and a few dozen non-`PASS` rows cite a file other than the item's — usually a document the
code contradicts. Round 1's by-file rule would have listed every schema-conforming `FAIL` as answering
nothing and filed the citing rows under the wrong item. The id is the right key once decision 1 makes
it stable; the only rounds whose ids are wrong are those recorded before the release, and decision 1
now requires that its ids never equal an earlier one, so those rows answer nothing instead of
answering the wrong file. The cost is one re-measure of `file_check` items after the upgrade.

**Refusing a conflicting pair deadlocked the driver and every full-panel shard.** The driver's audit
record unit runs `[ -f $TP_ROUND_DIR/merged.ndjson ] || tp audit --merge …; tp audit <spec> --record
…`: once a refused merge had written `-o`, every retry skipped the merge and refused again, and the
oracle never re-spawns role units, so the hint's *re-grade* could not happen. And spec-coverage sees a
different file list in each hand shard, so its verdicts on one item legitimately differ; re-grading
returns the same answer. Keeping the worse verdict and reporting the group answers both: no finding is
dropped, nothing waits on a re-grade, and the choice is deterministic and visible. `--record` refuses
duplicates outright because its round file is a byte copy of its input — measured, two identical
`FAIL` rows record as two findings at `HEAD`.

**Containment was right for naming a round and wrong for re-recording one.** Measured at `HEAD`:
`TP_ROUND=1 tp audit spec.md --record empty.ndjson` turned round 1 from `findings 1, clean false` into
a zero-byte, clean round. Decision 2 in §4 now offers the re-record step only for a file whose rows
are exactly the round's; decision 3 keeps containment for `matching_rounds`, which only informs.

**The statement is for the interactive loop.** Review's resolve payload offers `--verify`, never a
re-record, so under a run every review-resolve unit would have been told its write reached nothing
while its own durable-write predicate reads that file. The statement now appears only with `TP_ROUND`
unset, and on stderr. The search also covers the working directory's repository again, since this
project's own merge outputs live in a scratch directory outside any repository — a half of the
pre-review build the round-1 text had dropped.
