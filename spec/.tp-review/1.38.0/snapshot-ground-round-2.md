# tp v1.38.0 — The checklist covers what changed

> **This file is decisions.** A measurement stays only where it justifies one, and every figure names
> the command that derives it — a number whose counting rule is unstated is a defect even when the
> number is right. This release was written by reading `internal/engine/auditfiles.go` and re-emitting
> v0.37.0's round 7, not from the prose of the release it replaces.

## 1. Overview

`tp audit` hands its three code-lens roles a file list that is **truncated alphabetically**, and tells
nobody but the roles. On v0.37.0's own audit each of `go-safety`, `ax-contract` and
`maintainability-conventions` received a prompt headed `## Affected Files (10 of 46)` — the same ten
files in rounds 2 through 7, differing from round 1 by one file — while re-emitting that round
produced **zero bytes on stderr** and exit 0.

The selection model this release changes is reproduced in §2.1 closely enough to regenerate round 7's
ten files exactly, in order, from the git history alone. That is the evidence the sections below rest
on: the defect was reproduced before it was described.

Three deliverables, all inside `engine.selectCodeFiles` and its one caller:

1. **Rank by churn, not by filename** (§2). The data is already on `AuditFileInputs`.
2. **A universe the operator named is not truncated** (§3). tp already records that it happened.
3. **Say so when it truncates** (§4). tp already computes the pre-cap size.

**This is a stopgap and says so.** Ranking the ten better does not audit the other thirty-six.
Covering an N-file surface with a bounded per-prompt count is the divisible round's subject, and that
release is not numbered because its split key is undecided. This one makes the ten the ten that
matter and stops the truncation being silent.

## 2. Rank the non-priority group by churn

`selectCodeFiles` (`internal/engine/auditfiles.go:173-194`) partitions the filtered universe into a
priority group — paths matching `lock`, `validate`, `auth`, `secret` or `perm` — and everything else,
takes them in that order, and stops at `CodeFileCap` = 10. Neither group is ranked: the universe was
sorted once, alphabetically, by `filterAuditUniverse`'s closing `sort.Strings` (`internal/engine/auditfiles.go:93`),
and nothing reorders it afterwards.

**Both groups are now ordered by churn, descending — `DiffStats[path][0] + DiffStats[path][1]` — ties
broken by path so the order stays total.** `AuditFileInputs.DiffStats` already carries `{added,
deleted}` per path (`internal/engine/auditfiles.go:56`) and `diffSummaryOf` already renders it into every emitted
entry, which is how the round-7 prompt above could print `(diff: +272/-0)`. **No new input is
collected, no new field is stored, and no new failure mode is introduced** — the release re-uses a
value that was already computed, already carried and already displayed, and only ever ignored.

**The priority group survives, and the two keys are orthogonal.** The five substrings encode *this is
dangerous when it changes*; churn encodes *how much it changed*. A file that is both should outrank a
file that is only one, which is exactly what a stable partition with a churn sort inside each group
gives. Collapsing to a single churn ranking would demote a small change to a locking path beneath an
unrelated churn spike, which is the trade the substrings exist to refuse.

**A path with no `DiffStats` entry sorts last, not first.** An absent entry means one of two things —
the file did not change, or `DiffUnmeasured` says no comparison covers it — and neither is evidence of
churn. Ranking an unmeasured file ahead of a measured one would make the release promote precisely the
files tp knows least about.

### 2.1 What this is worth, measured

`internal/cli/unattended.go` is the file v0.37.0's four hardest audit rounds were about — §7 row 13's
carve-out lives in it, and the single commit that closed those rounds rewrote it by **71 added and 102
deleted** lines, a net of −31:

```
git rev-list <round-4 record>..<round-5 record> -- internal/cli/unattended.go   # -> ab2950f0
git -c diff.external= show --numstat --format= ab2950f0 -- internal/cli/unattended.go
```

**The universe is the release diff, and a constant holds it still.** With no `--base`,
`auditDiffRanges` (`internal/cli/audit_roles.go:454-468`) returns unstaged, staged, and
`<latest tag>...HEAD`, so every round of v0.37.0's audit ranked the files changed since `v0.36.0`.
That set grew **129 → 135 → 140 → 145 → 150 → 155 → 160** across the seven rounds, and every round's
prompt still read `10 of 46` — because `maxAutoDetectFiles` = 50 (§4) truncates the pool
alphabetically before selection sees it, and `filterAuditUniverse`'s drop rules take the 50 to 46. The
stability the prompt shows belongs to the 50-cap, not to the diff.

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

## 3. A universe the operator named is not truncated

`AuditFileInputs.DiffUnmeasured` is set at `internal/cli/audit.go:335` when `--affected-files` or
`--affected-from-tasks` replaced the universe. **When it is true, `CodeFileCap` does not apply.**

An operator naming files is a statement that *these are the files*. Truncating that list to ten
alphabetically discards the one input tp holds that is better than its own heuristic — and today it
does exactly that, because the operator's list replaces the universe *upstream* of `SelectAuditFiles`
(`internal/cli/audit.go:323-326`) and is then capped like any other.

**Measured at v0.37.0's round-7 record commit.** `tp audit spec/0.37.0.md --affected-files <25 named
`.go` paths>` exits 0 with **zero bytes of stderr**, every code-lens prompt reads
`## Affected Files (10 of 25)`, and the payload's `file_summary` reports `"truncated": false`. Fifteen
of the operator's own twenty-five never reach a role, and the one field that speaks to truncation says
none happened. This is the clean silent case; v0.37.0's own audit is not it, being noisy about a
different cut (§4).

**This makes the notice at `internal/cli/audit.go:777` true for the first time.** That notice already tells the
operator to *"name the rest with `--affected-files`"*; following the advice currently cannot work past
the tenth file. The advice is live rather than hypothetical — the notice fired on every one of
v0.37.0's seven audit rounds, so an operator who obeyed it would have hit the ten-file wall each time.

**No cap replaces it.** A prompt built from two hundred named files may exceed what a role can read,
and this release does not know that bound — measuring it is the divisible round's job. What ships here
is the narrower claim: tp stops silently overriding an explicit instruction. If a named list is too
large, the operator is the party who can see it and split it.

## 4. Say so when it truncates

`SelectAuditFiles` already computes `CodeFilesTotal: len(universe)` (`internal/engine/auditfiles.go:79`), and
`buildRolePrompt` already renders `## Affected Files (10 of 46)` into the prompt
(`internal/cli/audit_roles.go:254-257`). **The role is told; the operator is not.**

The only truncation notice tp emits is `internal/cli/audit.go:768-777`, gated on `maxAutoDetectFiles`
= 50 (`internal/cli/audit.go:100`). It is about a different cut and cannot stand in for this one in
either direction. Where the pool exceeds fifty it fires and still says nothing about what follows it:
on v0.37.0's audit it fired in **all seven** rounds — 129 to 160 auditable files against the gate —
while 46 → 10 dropped 36 files without a word each time. Where the pool is under fifty it is silent
altogether, which is the case §3 measured at `10 of 25` with zero bytes of stderr.

**A notice fires whenever `len(sel.CodeFiles) < sel.CodeFilesTotal`**, naming both numbers and the
remedy §3 has just made real. It is `output.Notice`, so it writes to stderr and leaves the JSON
payload alone.

**The emission payload carries the same two numbers**, and §7 row 5 asserts that separately. `--quiet`
erases stderr — measured at 77 bytes without the flag and 0 with it, same tree, same commit — and a
driver reading the payload must still be able to tell that the audit was partial. Today it cannot:
`file_summary` carries `total_files`, `total_changed` and a `truncated` flag that reports the 50-gate
only, so on §3's 25-file run it reads `false` while fifteen named files were discarded, and neither
10 nor 46 appears anywhere outside the prompt text.

## 5. Withdrawn: a file carrying last round's open finding

**This section proposed a fourth decision and grounding round 1 refuted it three ways. It is withdrawn
rather than repaired, and the measurements are kept because a mechanism deleted in silence teaches the
next reader nothing.**

It proposed: *a file named by an unresolved non-`PASS` row from the preceding round is added to the
selection of the role that filed it, before the cap.* Each of its three supports failed.

**Its key is not in the emitted schema.** The proposal reads `location`. The audit row schema
`audit_roles.go` emits is `item_id, status, evidence_file, evidence_lines, category, severity, notes`
— `grep -c location internal/cli/audit_roles.go` returns **0**. Over every recorded audit round,
`location` names a path that exists on **44 of 523** non-`PASS` rows while `evidence_file` is present
on **476**; on v1.0.0's own rounds it is **1 of 124**. Derive both with:

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

**Its diagnosis was structurally impossible.** The proposal's table claimed a file left one code-lens
role's checklist while staying on another's. `audit_roles.go` hands every non-`spec-coverage` role
`sel.CodeFiles` — **one identical list** — so that cannot happen. What the recorded rounds differ on
is which role *filed* a row, not which role *held* the file, and the table's column label said the
wrong one.

**Its premise was already false.** *"Nothing consults what the previous round found"* — `loadAuditPriorRound`
and `renderPriorRoundSection` already deliver each role its own non-`PASS` rows from the immediately
preceding round. The input the proposal wanted is plumbed; the proposal did not know it.

**And the case it was built on does not exist.** The finding it tracked did not vanish in the middle
round: that round's `maintainability-conventions` **`PASS`** row restates it verbatim. A rule scoped to
non-`PASS` rows discards the channel that carried it — so the corpus the proposal measured on contains
the counterexample to the rule it proposed.

**What survives, for whoever takes it next.** A deferred finding *can* stop being asked about, and
`--status` cannot distinguish that from a repair. The mechanism has to key on `evidence_file`, and it
should start from the prior-round section that already exists rather than from file selection. That is
a different release from this one, whose subject is which files the checklist covers.

## 6. Non-Goals

1. **No new workflow field** — not the cap, not the ranking key, not the notice threshold. A workflow
   field is a fenced surface with four write sinks and its own resolution order; this release is a sort
   key, a conditional and a notice.
2. **No change to `spec-coverage`'s selection.** On the same round-7 emission its heading reads
   `max 20` where the other three read `10 of 46` — its own cap did not bite there, and
   `internal/cli/audit_roles.go:254` prints the counted form only when it does. It ranks by
   task-mapping count (`selectSpecCoverage`, `internal/engine/auditfiles.go:128-168`), which is the
   right key for a conformance lens; `AuditFileCap` = 20 is unchanged either way.
3. **No raise of `CodeFileCap`, and no change to `maxAutoDetectFiles`.** Raising the first trades a
   coverage hole for a prompt-size hole this release has not measured. The second is the larger hole,
   and it is fenced here so that it is not read as covered: it cut the pool to 50 in **every** round of
   v0.37.0's audit, removing 79 to 110 of the 129–160 auditable files before `selectCodeFiles` saw
   anything. What ships here reorders the 46 that survived.
4. **No change to `filterAuditUniverse`'s drop rules.** Binaries, fixtures and deleted files stay
   dropped, and the universe stays sorted before selection so the partition is deterministic.
5. **No retroactive effect.** Rounds already recorded keep the lists they were emitted with.
6. **No carry of a prior round's findings into file selection.** §5 proposed one and is withdrawn;
   its two non-goals went with it. The need is real and the mechanism is not this release's — it keys
   on `evidence_file` and belongs with the prior-round section that already carries those rows.
7. **No change to the diff base.** Ranking inter-round diffs rather than the release diff would shrink
   the universe — §2.1 measures by how much — but it is a separate decision with its own failure mode
   (a round's own repair commit becomes the whole audited surface), and two of the six measured
   inter-round sets exceed `CodeFileCap` anyway.
8. **No repair of the `--numstat` probe.** §2 names the failure and §7 row 8 pins its consequence.
   Routing a probe failure into the payload the way §4 routes truncation is a second mechanism on a
   second channel, and this release does not add one.

## 7. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | over a universe whose alphabetically-first files have the least churn, `selectCodeFiles` emits the highest-churn `CodeFileCap` paths, ties by path | keep the alphabetical order — the shipped behaviour, which returns a disjoint set on this fixture |
| 2 | §2 *priority* | a path matching one of `priorityPathSubstrings` outranks a higher-churn path matching none, and within the priority group churn still decides | drop the partition and sort the whole universe by churn, which demotes a `lock` file beneath an unrelated churn spike |
| 3 | §2 *absent* | a path with no `DiffStats` entry sorts **last** | treat a missing entry as zero and sort ascending, or as `MaxInt` — either puts unmeasured files at the head |
| 4 | §3 | with `DiffUnmeasured` set, a 25-file named universe reaches every code-lens role entire | apply `CodeFileCap` regardless — the shipped behaviour, which discards 15 of the operator's own files |
| 5 | §4 | truncation emits a notice naming both numbers, **and** the payload carries them — asserted separately, with `--quiet` and without | route the fact through `output.Notice` alone, which `--quiet` erases, leaving a driver unable to see the audit was partial |
| 6 | §4 | the notice fires on a universe below `maxAutoDetectFiles`, where the existing gate stays quiet — the 25-file `--affected-files` case §3 measured at `10 of 25`, zero bytes of stderr, `"truncated": false` | keep the existing gate, which is the defect |
| 7 | §6.2 | `spec-coverage`'s list is byte-identical before and after, on a fixture where the code list changes | apply the churn key to `selectSpecCoverage`, reordering a lens whose key is task coverage |
| 8 | §2 *absent*, §6.8 | under a `git` shim that exits 128 whenever an argument is `--numstat`, `DiffStats` is empty, the `--name-only` universe survives, and the emitted list is the alphabetical one at exit 0 — pinned as the documented degenerate case, not a silent regression of row 1 | have the shim fail every `git diff`, which empties the universe too and ends the audit before selection, so the test passes while measuring nothing |

§5's four rows are withdrawn with it. One is worth recording rather than deleting: its fourth column
named *a weakening of the assertion* whose stated consequence was that the row **passes**, which is
not "a mutant that must fail it" — the preamble's own standard, broken in the rows that tested the
proposal. Each of the eight rows above names a change to production code.

**The fixture's own properties are asserted, not assumed.** Two of them decide row 1, and this
repository has already lost a round to a guard whose fixture could not distinguish the behaviour it
was named for. (1) A fixture whose alphabetical order happens to agree with its churn order passes
row 1 under both the fix and the defect, so the test `require`s that the two orders differ *before*
asserting which one the code produced — partial agreement is the normal case rather than the
pathological one, and on v0.37.0's round 7 the alphabetical ten and the churn ten share **3** of 10
members. (2) The fixture must hold no path matching `priorityPathSubstrings`: one such path is
promoted under both orderings, so the two tens would not be disjoint and row 1's mutant column would
be false of the fixture it names.
