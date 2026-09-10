# checklist-covers-what-changed — measurements

Supplemental material for `checklist-covers-what-changed.md`; the spec stands without it. **This file
is not a spec and `tp ground` never grades it.**

---

## The 77-byte probe and the draft corrections

**The cut is unreported, not unnoticed, and an earlier draft of the overview had that wrong.** It said
re-emitting the round produced zero bytes on stderr. It does not. The run exits 0 having written
exactly one notice — about a *different* cut, the `maxAutoDetectFiles` gate of the spec's §4:

```
$ tp audit spec/0.37.0.md > out.json 2> err.txt   # git clone at c0777dc6, v0.37.0's round-7 record
exit 0, err.txt = 77 bytes
160 files changed, auditing first 50 — name the rest with --affected-files
```

(The zero-byte reading is reproducible with `--quiet`, which is the likeliest origin of the error.)
The 36 files the 46 → 10 cut discarded are named nowhere and counted nowhere, and where the pool is
under fifty that gate stays quiet too, so nothing at all is printed — *Moved from the body at the
2026-09-11 pass* below measures that case at `10 of 25` with zero bytes of stderr. §4's subject is
therefore **the absence of a notice for this particular cut**, not an absence of output. **Probe it in
a `git clone`, never an `rsync -a --exclude .git` copy**: with no `.git`, `tp audit` exits 4 before
selection ever runs.

The selection model the release changes is reproduced below closely enough to regenerate round 7's
ten files exactly, in order, from the git history alone — verified by running it: at `c0777dc6` the
model's first ten equal the ten tp emitted, byte-for-byte and in order, over a 46-file universe.
**That is a claim about the model, not about the order in which this file was written.** An earlier
draft said *the defect was reproduced before it was described*, which leaves no artifact anyone can
check. The figures that do not come from that re-emission — the withdrawn proposal's corpus counts,
the `--numstat` shim run (below, under *Moved from the body at the 2026-09-11 pass*), the 77 bytes
above — each name their own command and their own ref instead.

`--quiet` erases stderr — measured at 77 bytes without the flag and 0 with it, same tree, same commit.

## What churn ranking is worth, measured

`internal/cli/unattended.go` is the file v0.37.0's four hardest audit rounds were about — §7 row 13's
carve-out lives in it, and the single commit that closed those rounds rewrote it by **71 added and 102
deleted** lines, a net of −31:

```
git rev-list <round-4 record>..<round-5 record> -- internal/cli/unattended.go   # -> ab2950f0
git -c diff.external= show --numstat --format= ab2950f0 -- internal/cli/unattended.go
```

**The universe is the release diff, and a constant holds it still.** With no `--base`,
`auditDiffRanges` (`internal/cli/audit_roles.go:454`) returns unstaged, staged, and
`<latest tag>...HEAD`, so every round of v0.37.0's audit ranked the files changed since `v0.36.0`.
That set grew **129 → 135 → 140 → 145 → 150 → 155 → 160** across the seven rounds, and every round's
prompt still read `10 of 46` — because `maxAutoDetectFiles` = 50 (§4) truncates the pool
alphabetically before selection sees it, and `filterAuditUniverse`'s drop rules take the 50 to 46. The
stability the prompt shows belongs to the 50-cap, not to the diff. It removed 79 to 110 of the
129–160 auditable files before `selectCodeFiles` saw anything.

Over that 46-file universe, with the priority partition applied and the cap at 10:

| | alphabetical (shipped) | by churn |
|---|---|---|
| rank of `internal/cli/unattended.go` | **30** in round 1, **31** in rounds 2–7, of 46 | **10** in round 1, **5** in rounds 2–7, of 46 |
| rounds where it reached the checklist | **0 of 7** | **7 of 7** |

Derivation, in a clean checkout at each round's record commit — the seven
`chore(audit): record spec/0.37.0.md audit round N` commits, found with
`git log --diff-filter=A -- spec/.tp-review/0.37.0/audit-round-N.ndjson`:

```
tp audit spec/0.37.0.md > round.json     # payload.files is the <=50 the 50-cap left
python3 - <<'PY'
import json, subprocess, os
BIN = {".png",".jpg",".jpeg",".gif",".svg",".ico",".woff",".woff2",".ttf",".eot",
       ".zip",".tar",".gz",".pdf",".exe",".dll",".so",".dylib",".o",".a"}
PRIO = ("lock", "validate", "auth", "secret", "perm")
g = lambda *a: subprocess.run(["git"] + list(a), capture_output=True, text=True).stdout
tag = g("describe", "--tags", "--abbrev=0").strip()
files = json.load(open("round.json"))["files"]
dele = set(g("diff", "--name-only", "--diff-filter=D", tag + "...HEAD").split())
fix = lambda p: p.endswith(".golden") or p.startswith("testdata/") or "/testdata/" in p
uni = sorted(p for p in files if p not in dele
             and os.path.splitext(p)[1].lower() not in BIN and not fix(p))
st = {}
for line in g("diff", "--numstat", tag + "...HEAD").splitlines():
    f = line.split("\t")
    if len(f) == 3 and f[0].isdigit(): st[f[2]] = int(f[0]) + int(f[1])
pr = lambda p: any(s in p.lower() for s in PRIO)
key = lambda p: (p not in st, -st.get(p, 0), p)       # absent last, churn desc, path
part = lambda k: ([p for p in uni if pr(p)] + [p for p in uni if not pr(p)] if k is None
                  else sorted([p for p in uni if pr(p)], key=k)
                     + sorted([p for p in uni if not pr(p)], key=k))
t = "internal/cli/unattended.go"
print(len(uni), part(None).index(t) + 1, part(key).index(t) + 1)
PY
```

**The model is validated against a real emission, not taken on trust.** `part(None)` is the shipped
order: its first ten equal the ten tp actually emitted at each of the seven commits, byte-for-byte and
in order, and the emitted `(diff: +272/-0)` on `internal/cli/audit_merge_severity_test.go` equals
`git diff --numstat v0.36.0...c0777dc6 -- internal/cli/audit_merge_severity_test.go`. Those ten are
the same in rounds 2 through 7 and differ from round 1 by one file — round 1 holds
`audit_stamping_test.go` where the rest hold `audit_record_test.go`.

The file was cut in **all seven rounds** and covered only because `go-safety` added it to its own
results file in rounds 2 through 7, writing *"Self-added, cut from the checklist a sixth round"* in
round 7's row. A role stepping outside its checklist is not a mechanism the loop can rely on: it
happened here because one role's operator-written brief had named the file, and nothing in tp caused
it.

**Where churn ranking still does not reach — and it is not this file.** Under §2's own rule
`unattended.go` is inside the cap in every one of the seven rounds, round 1 included, so this release
has no round in which its own motivating file stays cut. The limit is an order of magnitude up: the
ranking reorders 46 files, the audit's own diff held 129 to 160, and no ordering of ten covers either
number. That is why §1 names the divisible round as the real answer and this release as a stopgap.

**Inter-round diffs are much smaller, and are a different release (Non-Goal 6; §6.7 before the
2026-09-11 rewrite).** Under tp's own rules — `isAuditableType` plus `filterAuditUniverse`'s drops —
`git diff --name-only <record N-1>...<record N>` gives **13, 10, 9, 11, 9, 8** files for rounds 2
through 7. Far smaller than the release diff, and still not small enough: two of the six exceed
`CodeFileCap`, so re-basing does not make the cap stop biting.

**The cap is structural, not incidental — a second instance, from v1.0.1's audit.** That release
changed **240** Go files against `v1.0.0`, and one commit is **223** of them: `fa68051b`, whose whole
diff is 1,027 added lines and zero deleted, every added line reading `t.Parallel()`. The file that
sweep made racy — `internal/cli/notice_once_internal_test.go`, where two callers of a helper that
swaps `os.Stderr` were parallelised — ranks **145 of 240** alphabetically and **125 of 240** by
churn. At `CodeFileCap` = 10 neither ordering brings it inside the cap in any round, so no
auto-detected checklist could have surfaced it; what found it was the project gate, going red on one
run and green on the next. **§2's churn ranking is not the answer to a mechanical sweep**: its files
differ in churn only by how many `t.Parallel()` lines each received, which is a statement about how
many test functions a file holds and not about risk.

Derivation at `02ce05ef`:

```bash
git -c diff.external= diff --no-ext-diff --name-only v1.0.0...HEAD -- '*.go' | sort -u | wc -l
git -c diff.external= show --no-ext-diff --name-only --format= fa68051b -- '*.go' | sort -u | wc -l
git -c diff.external= show --no-ext-diff --format= -U0 fa68051b | grep '^+' | grep -v '^+++' \
  | sed 's/[[:space:]]//g' | sort | uniq -c
git -c diff.external= diff --no-ext-diff --name-only v1.0.0...HEAD -- '*.go' | sort -u \
  | grep -n notice_once_internal_test.go
git -c diff.external= diff --no-ext-diff --numstat v1.0.0...HEAD -- '*.go' \
  | awk '{print $1+$2, $3}' | sort -rn | grep -n notice_once_internal_test.go
```

**The fixture agreement that decides row 1.** A fixture whose alphabetical order happens to agree with
its churn order passes row 1 under both the fix and the defect — partial agreement is the normal case
rather than the pathological one, and on v0.37.0's round 7 the alphabetical ten and the churn ten
share **3** of 10 members.

## Withdrawn: a file carrying last round's open finding

**This section proposed a fourth decision and grounding refuted it. It is withdrawn rather than
repaired, and the measurements are kept because a mechanism deleted in silence teaches the next reader
nothing.**

It proposed: *a file named by an unresolved non-`PASS` row from the preceding round is added to the
selection of the role that filed it, before the cap.* **Four refutations were filed against it below.
Three hold. The fourth — the *structurally impossible* diagnosis — does not, and grounding round 2
falsified it; the paragraph that made it is corrected in place rather than deleted, because a
withdrawal record whose grounds are silently pruned is the same defect it was written to avoid.** So
the count is three of four, and an earlier draft of this paragraph said *each of its three supports
failed* — wrong on both the quantifier and the denominator.

**Its key is not in the emitted schema.** The proposal reads `location`. The audit row schema is
written by `renderAuditOutputSchema` (`internal/cli/audit_schema.go:13-30`) — not by
`audit_roles.go`, which only calls it at line 278, so a search of *that* file is evidence about
nothing and an earlier draft cited it. The schema declares seven required fields — `item_id, status,
evidence_file, evidence_lines, category, severity, notes` — plus an optional eighth, `class`
(*"kebab-case slug naming a mechanically checkable pattern; omit when not classifiable"*, line 26).
`location` is in neither list: a search for it over `internal/cli/audit_schema.go` returns **0**, and
so does one over `internal/cli/audit_roles.go`. Over every recorded audit round,
`location` names a path that exists on **44 of 523** non-`PASS` rows while `evidence_file` is present
on **476**; on v1.0.0's own rounds it is **1 of 124**. Both denominators grow with the corpus, so the
figures are pinned to `771b2ece` and derived — never read — with:

```
python3 -c "
import json,glob,os
tot=loc=ev=0
for f in glob.glob('spec/.tp-review/*/audit-round-*.ndjson'):
    for l in open(f):
        if not l.strip(): continue
        r=json.loads(l)
        if r.get('status')=='PASS': continue
        tot+=1
        v=r.get('location') or ''
        if v and os.path.exists(v): loc+=1
        if r.get('evidence_file'): ev+=1
print(tot, loc, ev)"
```

The counting rule matters and the proposal did not state one: *names a path that exists at HEAD*. A
looser rule — any non-empty `location` — counts rows whose value is a section like `§9`.

**Its diagnosis was *not* structurally impossible — this paragraph used to say it was, and grounding
round 2 falsified that.** The proposal's table claimed a file left one code-lens role's checklist
while staying on another's. The code fact the refutation rested on holds, and it is narrower than the
refutation made it: `internal/cli/audit_roles.go:329-332` gives every non-`spec-coverage` role
`sel.CodeFiles`, so **within one `tp audit` invocation the three code-lens roles receive one identical
list** — re-emitting `tp audit spec/1.0.0.md` in a `git clone` at `e245485c` and again at `cfd96bb8`,
the record commits bracketing v1.0.0's audit round 10, returns the same ten files for all three roles
at both trees. The step from there to *so that cannot happen* is the error: the loop does not run one
invocation, and the record shows the divergence the proposal described.

```
python3 -c "
import json,collections
for n in (9,10,11):
    per=collections.defaultdict(set)
    for l in open(f'spec/.tp-review/1.0.0/audit-round-{n}.ndjson'):
        if not l.strip(): continue
        r=json.loads(l); i=r.get('item_id') or ''; role=r.get('role') or ''
        if i.startswith('file-'):
            per[role].add(i[len('file-'+role+'-'):].split('-apply-the')[0])
    code={k:v for k,v in per.items() if k!='spec-coverage'}
    print(n, {k:len(v) for k,v in sorted(code.items())}, 'union', len(set().union(*code.values())))"
```

At `771b2ece` this prints ten `file-*` rows per code-lens role in each of rounds 9, 10 and 11, over a
**union of 10, then 17, then 16**. Round 9's three roles cover the same ten paths; rounds 10 and 11's
do not — round 10's `maintainability-conventions` set holds `internal/cli/clauses.go`,
`internal/cli/root.go`, `internal/engine/bookkeeping.go` and `internal/engine/reviewstate.go`, none of
them in `ax-contract`'s or `go-safety`'s. Round 11's `go-safety` row on `internal/cli/review.go` says
it in words: *"the file fell off round 10's checklist and is back on this one."*

**What produced the divergence this corpus does not settle, and the honest form of the paragraph says
so.** Neither recorded set is the list either bracketing tree emits, so a per-role emission against
differing trees and a role filing outside its own checklist both fit, and no round-10 row announces a
self-addition. Either way the phenomenon the proposal was built on is in the record, so it is not a
ground for withdrawing the proposal. The three remaining grounds are.

**Its premise was already false — but only read whole.** The proposal's sentence is
*"`SelectAuditFiles` picks per role from one universe; nothing consults what the previous round
found."* Quoting the second clause alone, as an earlier draft did, makes it a global claim that is
false: `loadAuditPriorRound` (`internal/cli/audit.go:542`) and `renderPriorRoundSection`
(`internal/cli/audit_roles.go:193`) already deliver each role its own non-`PASS` rows from the
immediately preceding recorded round, into the prompt. Read as the clause it is — a claim about the
*selection* path — it is true: `SelectAuditFiles` takes `AuditFileInputs` and nothing else
(`internal/engine/auditfiles.go:71`). Either reading refutes the proposal, and they refute it
differently: the input the proposal wanted is already plumbed, just not into selection, which is the
more useful fact for whoever takes this next.

**And the case it was built on does not exist.** The finding it tracked did not vanish in the middle
round: that round's `maintainability-conventions` **`PASS`** row restates it — restates, not repeats
verbatim, and the adverb was doing the work an earlier draft gave it. Round 9's `go-safety` wording is
*"readFilesContent 20 lines below"*; round 10's is *"drops an unreadable file at L1506 … while its
neighbour readFilesContent documents the opposite convention at L1534-1538"* — same subject, different
sentences, and the later one carries line numbers the earlier one does not. A reader checking this
claim by string comparison finds no match. A rule scoped to
non-`PASS` rows discards the channel that carried it — so the corpus the proposal measured on contains
the counterexample to the rule it proposed.

**What survives, for whoever takes it next.** A deferred finding *can* stop being asked about, and
`--status` cannot distinguish that from a repair. The mechanism has to key on `evidence_file`, and it
should start from the prior-round section that already exists rather than from file selection. That is
a different release from this one, whose subject is which files the checklist covers. **And the need
is broader than the proposal stated**: the corrected diagnosis paragraph above shows a file leaving
one code-lens role's recorded checklist while staying on another's in v1.0.0's rounds 10 and 11, so
whoever takes this inherits a second question the proposal never asked — why the roles of one round
diverge at all.

**§5's four test rows were withdrawn with it.** One is worth recording rather than deleting: its fourth
column named *a weakening of the assertion* whose stated consequence was that the row **passes**,
which is not "a mutant that must fail it" — the preamble's own standard, broken in the rows that
tested the proposal. Row 8 committed the same error one column over, in the surviving table: it named
a widening of the *test's own git shim* — fail every `git diff`, not only `--numstat` — which is a
fixture mutation and cannot test production at all. Its stated consequence was wrong as well: measured
at `c0777dc6` under exactly that shim, `tp audit spec/0.37.0.md` exits **4** with `no changed files
detected` and emits no prompts, so a test carrying row 8's assertion **fails** under it rather than
passing while measuring nothing. Keep the widened shim as a fixture hazard to avoid, not as a mutant.

## Routed here at the 2026-09-08 re-verification

One item from the candidates files lands on this spec's subject. It is recorded in this sidecar; the
spec body is not edited.

- **The review per-file cap has no guard on its marker.** `maxPerFile` in `internal/cli/review.go`
  truncates a file's excerpt and appends `\n[...truncated]` (line 1565 at `dd89c566`; a second,
  distinct marker `[...truncated by total cap]` follows at 1570), and no test asserts either marker
  appears. That is the same defect this spec closes on the audit side — a set silently cut and
  reported as whole — on the review side, where it is unmeasured. Source:
  `spec/0.35.0-candidates.md` item 5, whose other two thirds are settled: the lock-timeout boundary
  guard shipped as `internal/engine/lock_timeout_range_test.go`, and the two plan builders stay
  separate by `spec/backlog/an-unreadable-file-is-named.md` Non-Goals 1 and 2 (they were
  `refusals-that-name-nothing.md`'s Non-Goals 3 and 5 before that file's 2026-09-08 split).

## Decided at the 2026-09-08 decision pass

From `spec/undecided.md`, *The divisible round*.

**Decided: the split key is spec location — the section.** A round is divided into shards by section,
each shard one prompt carrying that section's checklist items; per-item convergence is unchanged.

**It is not folded into this spec.** This spec bounds a per-prompt checklist at ten items and names
the divisible round as its real answer, but taking both in one release would double a spec already
ranked second. The divisible round becomes a follow-on **tool** spec, *round-divides-by-section*, to
be written after this one ships; `spec/backlog/README.md` carries it under *Decided, awaiting a spec*
until it has a file.

The measurement the decision rests on — 6 of 97 `spec-coverage` items non-`PASS` at `13bfde30`, so a
split by *count* gives two shards that are each overwhelmingly `PASS` — is in `spec/undecided.md`
under that entry, with its counting rule.

## What v1.1.1 changed under this spec

`v1.1.1` shipped `scripts/audit-round-prep.py`, the brief-level form of this spec's derivation: a
previous round's `PASS` row whose `evidence_file` is untouched since the record commit is re-recorded
verbatim, and only the rest is re-measured (47 of 63 rows carried against `v1.1.0`'s round 4, at
`4f4bdc82`). Two consequences for the release that takes this spec:

- **A carried row is indistinguishable from a measured one in the recorded round.** The script writes
  the row byte-identical and `agents/tp-auditor.md` tells the role to re-record it unchanged, so the
  corpus cannot tell a `PASS` the role ran from one it copied. When tp derives the carry itself it
  stamps `carried_from: <round>` on the row, the field ground rounds already carry
  (`CarriedFrom` in `internal/engine/groundrow.go`, computed in `internal/engine/groundcarry.go`), and
  `--record` accepts it: audit rows are parsed leniently
  (no `DisallowUnknownFields` in `internal/cli/audit*.go`), measured at `808dd375`.
- **The derivation's inputs are the script's, not the spec's.** `changed` is the record commit's diff
  to `HEAD` plus the dirty tree; a row with no `evidence_file` is never carried; a round with no
  recorded predecessor carries nothing. Whatever this spec specifies must reproduce those three rules
  or say which one it changes, because the script's shell test pins them and the brief already
  promises them to roles.

**Where the pointers to this section stand (2026-09-11).** `CLAUDE.md`'s *Run the round cheaply*
points here and says no pending spec makes tp do the carry derivation yet, which is accurate: the
body, before and after the 2026-09-11 rewrite, holds no carry decision. `skills/tp/SKILL.md` line 424
still calls the script *"the brief form of `spec/backlog/checklist-covers-what-changed.md`, which makes
tp do the derivation"*, which is not. The script carries rows keyed by `item_id` into the next brief,
so it inherits `audit-records-what-was-graded`'s id fix; that spec's body says so.

## Moved from the body at the 2026-09-11 pass

The 2026-09-11 rewrite put the body under `skills/tp/SKILL.md` Step 0.5: decisions in the body,
measurements here. What moved, in substance:

**§2's `--numstat` shim run — the degenerate case.** Measured with a `git` shim on `PATH` exiting 128
whenever an argument is `--numstat`, at `c0777dc6`: `tp audit spec/0.37.0.md` exits 0, every emitted
entry renders `+0/-0`, the `--name-only` universe survives intact, stderr carries
`warning: git diff --numstat failed: exit status 128`, and the emitted ten are byte-identical to the
unshimmed alphabetical ten. So a probe failure does not corrupt the ranking — it removes it, falling
all the way back to the shipped order, and the only signal is that one stderr line. The body's
Non-Goal 8 (*no repair of the `--numstat` probe*) and test row 8 (pinning this case) were dropped at
the rewrite: a row pinning a degenerate case asserts a limitation rather than a decision, and the
decision it guarded — an unmeasured path sorts last — is row 3's.

**§3's measured run.** At v0.37.0's round-7 record commit, `tp audit spec/0.37.0.md --affected-files
<25 named .go paths>` exits 0 with **zero bytes of stderr**, every code-lens prompt reads
`## Affected Files (10 of 25)`, and the payload's `file_summary` reports `"truncated": false`. Fifteen
of the operator's own twenty-five never reach a role, and the one field that speaks to truncation says
none happened. The auto-detect notice fired on every one of v0.37.0's seven audit rounds, so an
operator who obeyed its advice would have hit the ten-file wall each time. The field-report
verification below re-measured the 25-file case at `18032abe` with the same result.

**§4's figures.** On v0.37.0's audit the auto-detect notice fired in all seven rounds while 46 → 10
dropped 36 files without a word each time. On the round-7 emission `prompts[].checklist_count` is `10`
and `len(prompts[].affected_files)` is `10` beside it, while the pre-cap `46` appears in no field. A
guard written from the literal sentence — assert that `10` and `46` are both absent from the payload
JSON — fails on an unmodified tree, which is why the body's row 5 asserts the presence of the pre-cap
total and never the absence of a digit.

**What else left the body.** §1's paragraph locating two deliverables in one selection function and
the third in its single caller is implementation, which Step 0.5 sends to the implementing task. §5,
the withdrawn proposal, became the one-line pointer at the end of §1; Non-Goal 6 restated it and was
dropped. Non-Goal 2 is narrowed: it fenced spec-coverage's whole selection, and now fences only its
ranking key and cap, because reading every closing sha lands in `audit-records-what-was-graded` §3 and
§4.3 here reports the excerpt cut.

## Why `audit-records-what-was-graded` ships first

Churn ranking alone would have made the field report's #27 worse. Under positional id suffixes, every
change of order inside one deep directory moves a suffix onto another file. Path order holds still
until a file is added; churn order moves whenever the relative churn of two files changes, which is
most rounds of an audit that is repairing code. So §2 without stable ids would re-point item ids in
rounds where the shipped order would not have. The ids are fixed first; the reproduction is in
`audit-records-what-was-graded-measurements.md`.

## Field report WB-3155, verified 2026-09-11

A field report (WB-3155), from a Laravel project on tp v1.1.1, numbered its friction points. Three land
here, one in part. Reproductions were run on 2026-09-11 with a binary built from `18032abe`
(`tp version v1.1.2-0.20260910212136-18032abe405f`), each in a fresh `git init` repository outside
this one; source citations are at `18032abe`.

### #28 — a role prompt is cut at ten files and only a heading says so: CONFIRMED

**The claim.** `## Affected Files (10 of 78)`: each role prompt carried ten `file_check` items and the
other 68 files were in no prompt, even when named with `--affected-files`. Only the in-prompt heading
said so; `tp audit`'s output carried no top-level warning. Someone who does not know to shard runs one
prompt, audits ten files, and records the round clean. It proposed that tp emit the shards itself, or
at least print *"78 files, auditing 10 per role prompt: 8 shards needed"*.

**Verified** by the verification unit at `18032abe`: 25 files named with `--affected-files` give each
code role `10 of 25`, zero bytes of stderr, and `file_summary.truncated: false`. The emission envelope
carries no `next_action` at all; the report's reading of the `next_action` it did see was not
re-measured, and §4.2 holds either way, since nothing stores the counts a truthful one would need. Two
sentences in tp read otherwise. The comment at `internal/cli/audit.go:709-710` — *"a named set is
audited whole, so its pre-cap count is its own length and it never reads as truncated"* — is true of
the auto-detect cap's bookkeeping, and the named set is then cut by the code-role cap all the same.
`skills/tp/REFERENCE.md` line 768 defines `truncated` as the auto-detect cap's flag, which is accurate
and leaves the code-role cut with no field.

**Already covered at emission** by §3 and §4.1. **Not covered before this pass:** a recorded audit
round stores round, findings, clean, recorded-at, file, spec hash, roles hash, id scheme and harness
note (`internal/cli/audit_record.go:239-249`), and nothing about how much each role graded — hence
§4.2. The proposal that tp shard the round itself is `round-divides-by-section`.

### #32 — `--base` and `--affected-from-tasks` choose the wrong files: PARTLY; the proposed cause REFUTED

**The claim.** `--base develop` brought five foreign files into the list that had arrived on the branch
by merging `develop`; the report proposed that `--base` give the branch's own changes against the
merge base, `A...B`. And `--affected-from-tasks` should also gather commits carrying the spec's prefix,
or at least print *"N files from task shas; M files changed on the branch but belong to no task"*.

**The two-dot premise is REFUTED.** tp already compares `<base>...HEAD` — `auditDiffRanges`
(`internal/cli/audit_roles.go:454-468`) returns `base + "...HEAD"` at `:461` — which is the three-dot,
merge-base form the report proposed. Reproduction: branch `main`; `develop` branched from it;
`feature` branched from it and commits `app/Mine.php`; `develop` commits `Foreign.php`; a second local
ref `develop-stale` is left at `develop`'s commit before `Foreign.php`; `feature` merges `develop`.

```
tp audit spec.md --base develop         -> files ["app/Mine.php"]                 stderr empty
tp audit spec.md --base develop-stale   -> files ["Foreign.php", "app/Mine.php"]  stderr empty
```

So the leak needs a base ref older than the branch that was merged; the report's local `develop`
likely lagged the one it merged. **What is true, and silent,** is that nothing says the universe holds
files that reached `HEAD` only by merging. §5 d1's derivation separates the two cases on this fixture:
`git log --first-parent --no-merges --name-only --format= develop-stale..HEAD` lists only
`app/Mine.php`, so `Foreign.php` is the one file whose only first-parent change is the merge; with the
current base the universe holds nothing else to flag.

**The `--affected-from-tasks` half.** A done task closed with a production commit (`app/calc.go`) and a
test commit (`tests/calc_test.go`), plus `app/stray.go` committed by no task:

```
tp audit spec.md --affected-from-tasks  -> files ["app/calc.go", "tests/calc_test.go"]   stderr 0 bytes
tp audit spec.md                        -> files ["app/calc.go", "app/stray.go", "tests/calc_test.go"]
```

The untasked file is left out without a word, hence §5 d2. The report's own cause — tasks closed with
one sha because a batch close refused a `commit` array — is its #25, taken by
`a-task-file-write-names-its-target`. The loss verification found instead, spec-coverage's task
mapping reading only the first of two recorded shas, is `audit-records-what-was-graded` §3. Gathering
commits by a subject-line prefix is not taken: the prefix is a project's convention, and tp has no
notion of one.

### #33 — the spec-coverage prompt is half a megabyte: PARTLY; the stated cause REFUTED

**The claim.** Round 3's single `spec-coverage` prompt was 519,296 characters and the file roles'
148–153 KB, *"most of it the spec body repeated in every shard"*. Spec-coverage cannot be sharded, since
its checklist derives from the spec, so one agent graded 350 items against half a megabyte. It
proposed sharding spec-coverage by section (`--sections 3-6`), or giving file roles a heading map in
place of the spec body.

**The stated cause is REFUTED: code-lens prompts carry no spec text.** Fixture: a spec padded past the
excerpt budget, two tasks, one code file, `tp audit spec.md --affected-files app/calc.go`. The
spec-coverage prompt is 14,105 bytes and contains a `## Spec Excerpt`; the `security` prompt is 4,232
bytes and `maintainability-conventions` 4,111, and neither carries the excerpt heading or any sentence
of the spec.

**CONFIRMED, and not in the report: the excerpt is cut, silently, and by bytes.** `SpecContentCap` is
10,000 (`internal/engine/fileio.go:17`) and the cut is a byte slice with `\n[...spec truncated]`
appended (`internal/cli/audit.go:528-529`). The fixture places a two-byte `ş` across byte 10,000; the
payload's excerpt ends `xxxxxx�\n[...spec truncated]` — the split byte became U+FFFD in the JSON.
Stderr is 0 bytes and no payload field mentions the cut.

**Evidence for `round-divides-by-section`.** The report's rewritten spec measured 122 KB by its own
count, so its conformance role read less than a tenth of it. Its prompt was nonetheless 519,296
characters, and with the excerpt capped at 10,000 bytes at least 509 KB of that is something other
than spec text — the checklist of 350 items and the prompt around it. What grows with the spec is the
number of items one prompt carries, and dividing the round by section — decided 2026-09-08, the
report's own first proposal — is what bounds it. The report's second proposal, a
heading map for the file roles, answers a cause that does not exist. §4.3 therefore reports the cut and
leaves the budget alone.
