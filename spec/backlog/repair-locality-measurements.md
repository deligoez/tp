# repair-locality — measurements

Supplemental material for `repair-locality.md`; the spec stands without it.

## Two earlier readings of the table

**Two earlier readings of this table are downstream and are corrected here.** Deleting the
`sha(pa) != …` line — running *without* §4 item 3's exclusion — gives 19 / 14 / 11 = **44** rounds and
45.2% / 79.9% / 92.6%, 21.9% / 22.2% / 71.4%, `all` 70.2% / 30.1% / 2.33×. Those are the figures the
first draft of this file printed, and **`spec/1.0.0.md` §8 quotes them** — *"a median of 70.2% across
44 rounds"*, *"45.2% → 79.9% → 92.5%"*, crediting *"the repair-locality spec §1.1"*. Two things are
wrong there and neither changes anything that rests on the table: the 44-round set includes four
rounds §4 item 3 excludes, and `92.5%` is `92.5925…` truncated where every other cell in the row was
rounded. **That spec must be updated when this one is implemented**; it is the only outward citation
of these cells.

Excluding a pair when either snapshot mismatches drops 4 of v0.35.0's rounds; excluding only on the
round's own hash drops 2.

**A grounding round could not reproduce any of the eight percentage cells across 288 configurations,
and the reason is worth keeping.** Its sweep varied heading level over `{2}`, `{3}`, `{2,3}` and
`{1,2,3}` — this repository's specs carry `####` sections (`8a.3`, `4.2.3.1`, `3.1.1`), so the
configuration that reproduces the table was outside the space swept. Under the script above, the
44-round form reproduces seven of the eight cells exactly, and the eighth to 0.1. **The cells were
never wrong; the derivation was never stated**, which is the same defect §3.1 names one paragraph on
and the reason the script is printed here rather than described.

## A grounding round measured this at 100%

**The review corpus above measures repair locality after the fact. `tp ground` produced a case where
it was the whole of a round.** On 2026-09-04 the grounding programme ran the first *third* round in
its corpus, deliberately as a measurement: eleven specs had two rounds, and their asked-only non-`PASS`
rate fell from a mean of 32.9% to 19.6%, ten of eleven falling — so the question was whether a third
round continues the trend. `spec/backlog/next-action-and-check-tell-the-truth.md` was chosen because
its round 2 was the cleanest recorded, 2 non-`PASS` of 30 asked.

**Round 3 returned 4 non-`PASS`, above both of its own earlier rounds.** Counting rule: rows with no
`carried_from` over that spec's `ground-round-{1,2,3}.ndjson` under `spec/backlog/.tp-review/` gives
**25.0% → 6.7% → 30.8%** (8/32, 2/30, 4/13). The round's own report said 36.4%, counting only the 11
floor units it owed and excluding the 2 rows it filed against cut units, both `PASS`. **Both readings
are defensible and they differ by 5.6 points, so the rule is the figure**; this section uses the
corpus-wide one so the three rounds are comparable with every other spec.

**And every one of the four sat in text the repair wrote between rounds.** Round 3 carried 47 units
forward and nothing older resurfaced. **An earlier draft of this paragraph called those 47 "round 2's
carried units" — round 2 carried 20; 47 is round 3's carry**, and the two are different quantities
one sentence apart. That error is the same class this section is about, committed while writing it. So the rate did not measure slow convergence — it measured
that a repair pass is new ungraded prose, and that grading it is what a later round mostly does.

**The instance is worth keeping because of what the repaired text was.** Round 2 had faulted two
universals a preamble asserted over its own seven-row table. The repair withdrew both — correctly —
and added a paragraph explaining *why line-pinning a derivation is unsafe*. That paragraph then
carried a wrong section pointer, a line count reachable only from an uncommitted draft, and a `[1-7]`
character class that asserts a denominator of seven rather than deriving one: appending an eighth row
leaves it printing `1` and `7` where the truth is `2` and `8`. **A passage written to close a class
re-committed it one level up**, and a round caught it rather than a reader.

**What that argues for is a budget rule rather than a round count.** A round is worth running when the
previous round's repair touched prose, and its budget is the repaired units — not the floor, and not a
fixed N. On the same evidence, a repair that had made only the two withdrawals would plausibly have
produced a clean third round; the 260 words of self-forensics are where all four findings live. That is
this repository's standing rule — *a repair that introduces a new abstraction belongs to the next
version* — arriving from the grounding side, and it is why §3 reports the share rather than gating on
it: the number is a prompt to look at what the last repair wrote, not a verdict on the document.

## The snapshot census

0 of 172 recorded review rounds lack a snapshot, while 20 of 108 audit rounds do. Derivation:

```
python3 -c 'import json,glob,os,collections
c=collections.Counter()
for s in glob.glob("spec/.tp-review/*/state.json"):
    d=json.load(open(s)); b=os.path.dirname(s)
    for ph,k,f in (("review","review_rounds","snapshot-round-%d.md"),("audit","audit_rounds","snapshot-audit-round-%d.md")):
        for r in d.get(k) or []: c[ph, os.path.exists(os.path.join(b,f%r["round"]))]+=1
print(sorted(c.items()))'
```

## Location shapes in the review corpus

Every one of the 3,883 recorded review-round rows under `spec/.tp-review/` carried a `location` when
this was written, but **485 of them (12.49%) are not a bare section id**. The leading-`§` pattern
parses 3,761 of 3,883; the remaining **122 (3.14%)** yield no id. Derivation:

```
python3 -c 'import json,glob,re
v=[str(json.loads(l).get("location","")).strip() for f in glob.glob("spec/.tp-review/*/review-round-*.ndjson") for l in open(f) if l.strip()]
strict=re.compile(r"^§[0-9]+(\.[0-9]+)*$"); lead=re.compile(r"^§\d+[a-z]?(\.\d+[a-z]?)*")
print(len(v), "not-bare:", sum(not strict.match(x) for x in v), "unparseable:", sum(not lead.match(x) for x in v))'
```

Re-run over both globs (`spec/.tp-review/` and `spec/backlog/.tp-review/`) at the cleanup that moved
the forensics here: 4,059 rows, 122 unparseable under the `§` pattern alone, and **113 of those 122
are a bare heading number** (`3.1`, `4.2.3`) — the shape §3.1's second pattern now parses. What is
left is free prose and file paths (`internal/cli/plugin_test.go:92`, `CLAUDE.md:206`), which no
section pattern should claim:

```
python3 -c 'import json,glob,re
v=[str(json.loads(l).get("location","")).strip() for f in glob.glob("spec/.tp-review/*/review-round-*.ndjson")+glob.glob("spec/backlog/.tp-review/*/review-round-*.ndjson") for l in open(f) if l.strip()]
lead=re.compile(r"^§\d+[a-z]?(\.\d+[a-z]?)*"); bare=re.compile(r"^\d+[a-z]?(\.\d+[a-z]?)*(?![\w-])")
un=[x for x in v if not lead.match(x)]
print(len(v),"unparseable with § only:",len(un),"of which bare-number:",sum(1 for x in un if bare.match(x)))'
```

## No review row lacks a location

**No review row lacks a `location` at all, so the missing-`location` case is a guard and not a
limit.** Split by phase over `spec/.tp-review/*/*.ndjson`: review-round files 3,883 rows with 0
missing; audit-round files 12,970 rows with 3,048 missing; ground-round files carry no `location` key
at all. **The entire shortfall is in phases this release does not read.** An earlier draft printed
the corpus-wide rate — 16.6% when written, 18.9% now — as a limitation of *this* measurement; its
actual rate on the measured population is 0%. The guard stays, because nothing enforces that a future
review row carries the field. The lesson is the one this repository already carries about a key-name
search reported as a claim about data — **a borrowed figure must be re-derived against the question
it is being made to answer** — with the half that draft still missed: a re-derived figure must also
be measured over the population that answers it. Derivation:

```
python3 -c 'import json,glob,os,re,collections
c=collections.Counter()
for f in glob.glob("spec/.tp-review/*/*.ndjson"):
    b=os.path.basename(f)
    k=("review-round" if re.match(r"review-round-\d+\.ndjson$",b) else "review-role" if re.match(r"review-r\d+-",b)
       else "audit-round" if re.match(r"audit-round-\d+\.ndjson$",b) else "audit-role" if re.match(r"audit-r\d+-",b) else "ground")
    for l in open(f):
        if l.strip():
            v=json.loads(l).get("location"); c[k, bool(isinstance(v,str) and v.strip())]+=1
print(sorted(c.items()))'
```

Since v1.1.0, `--record` refuses a row without a non-empty `location` (`requiredFindingFields` in
`internal/cli/review_record.go`), so the guard now covers only rounds recorded before that release.

## The exclusion census

35 of 172 review rounds had a snapshot whose hash disagreed with the recorded `spec_hash` when this
was written; that takes v0.35.0 from 19 rounds to 15 under the per-pair rule. Re-derive with

```
python3 -c 'import json,glob,os,hashlib;print(sum(1 for s in glob.glob("spec/.tp-review/*/state.json") for r in (json.load(open(s)).get("review_rounds") or []) if os.path.exists(p:=os.path.join(os.path.dirname(s),"snapshot-round-%d.md"%r["round"])) and "sha256:"+hashlib.sha256(open(p,"rb").read()).hexdigest()!=r["spec_hash"]))'
```

## Authorship carries no signal

`git log --format=%an -- spec/ | sort -u` returns one name over 1,177 commits, because subagents
commit as the operator.

## Why the audit phase is out

88 of 108 audit rounds have a snapshot. Of the 76 consecutive audit snapshot pairs, **49 are
byte-identical**, so in nearly two thirds of audit round transitions the changed-section set is empty
and the share is 0 by construction; and 3,048 of 12,970 audit rows (23.5%) carry no `location`,
against 0 of 3,883 review rows. Use the snapshot-census command above for the first figure and the
phase-split command for the third; the pair comparison:

```
python3 -c 'import json,glob,os
P=[(os.path.dirname(s),r["round"]) for s in glob.glob("spec/.tp-review/*/state.json") for r in json.load(open(s)).get("audit_rounds") or []]
h=lambda b,n: os.path.join(b,"snapshot-audit-round-%d.md"%n)
Q=[(b,n) for b,n in P if os.path.exists(h(b,n)) and os.path.exists(h(b,n-1))]
print("pairs:",len(Q),"differing:",sum(open(h(b,n),"rb").read()!=open(h(b,n-1),"rb").read() for b,n in Q))'
```

## Separator-less prefix pairs

**`§30` appears in zero recorded rows**, no spec here has thirty sections, while `§1` against
`§10`–`§19` is one of **28 separator-less prefix pairs among the review corpus's 100 distinct section
ids** (77 across all recorded phases). Derivation:

```
python3 -c 'import json,glob,re,itertools
s=re.compile(r"^§[0-9]+(\.[0-9]+)*$")
ids=sorted({x for f in glob.glob("spec/.tp-review/*/review-round-*.ndjson") for l in open(f) if l.strip()
            for x in [str(json.loads(l).get("location","")).strip()] if s.match(x)})
print(len(ids), sum(1 for a,b in itertools.permutations(ids,2) if b.startswith(a) and not b[len(a):].startswith(".")),
      sum(1 for x in ids if x.startswith("§30")))'
```
