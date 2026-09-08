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
under fifty that gate stays quiet too, so nothing at all is printed — §3 measures that case at
`10 of 25` with zero bytes of stderr. §4's subject is therefore **the absence of a notice for this
particular cut**, not an absence of output. **Probe it in a `git clone`, never an
`rsync -a --exclude .git` copy**: with no `.git`, `tp audit` exits 4 before selection ever runs.

The selection model the release changes is reproduced below closely enough to regenerate round 7's
ten files exactly, in order, from the git history alone — verified by running it: at `c0777dc6` the
model's first ten equal the ten tp emitted, byte-for-byte and in order, over a 46-file universe.
**That is a claim about the model, not about the order in which this file was written.** An earlier
draft said *the defect was reproduced before it was described*, which leaves no artifact anyone can
check. The figures that do not come from that re-emission — the withdrawn proposal's corpus counts,
§2's `--numstat` shim run, the 77 bytes above — each name their own command and their own ref instead.

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

**Inter-round diffs are much smaller, and are a different release (§6.7).** Under tp's own rules —
`isAuditableType` plus `filterAuditUniverse`'s drops — `git diff --name-only <record N-1>...<record N>`
gives **13, 10, 9, 11, 9, 8** files for rounds 2 through 7. Far smaller than the release diff, and
still not small enough: two of the six exceed `CodeFileCap`, so re-basing does not make the cap stop
biting.

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
  separate by `spec/backlog/refusals-that-name-nothing.md` Non-Goals 3 and 5.

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
