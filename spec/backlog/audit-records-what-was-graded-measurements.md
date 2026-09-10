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
