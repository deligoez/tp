# tp — Repair locality

Class: tool

> **This file is decisions.** §1.1 is measured for this release from the recorded snapshots, not
> quoted, and the script in §1.1 *is* the derivation — running it is the only way the table is
> checked. The first form of the number — *the share of a round's findings sitting in text the
> previous round wrote* — reverses two of the three pairwise orderings between cycles and moves
> v0.37.0 from first to last, and §2 is the correction.

## 1. Overview

A repair round rewrites part of the spec. The next round reviews it and files findings. **How much of
what a round finds is in text the round before it just wrote?**

Nothing reports this, so a loop that has stopped converging and a loop that is converging normally
look identical from the counts. `CLAUDE.md` records the diagnosis being reached by hand **twice**,
for two different cycles and at two different values: v0.37.0's *"24% of the file was forensics each
repair round had written for the next round to review"*, reached after twelve flat rounds, and
v0.35.0's *"43% of that cycle's findings sat in text the previous round had just written"*. Two hand
measurements of one statistic, neither reproducible from the record, neither stating its rule.

This release reports the number. **It gates nothing.**

The arithmetic in §1.1 reads two snapshots and a findings file and never opens `spec_hash`; the
emit-time hash release (`spec/backlog/round-records-the-text-it-read.md`) only empties §4 item 3's
exclusion set for rounds recorded after it and repairs none of the rounds §1.1 drops, so it is not a
prerequisite of this one.

**The counter is read for the pair, not for the round.** A re-emission overwrites a round's snapshot
in place, so the earlier bytes exist nowhere — `spec/backlog/reconcile.md` §2.1 names the field that
records it, `spec_moved_mid_round`. Round *N*'s locality is computed from snapshots *N−1* and *N*, so a
re-emission of **either** falsifies it, and a re-emission of *N−1* leaves round *N*'s own counter
absent. Whatever consumes the counter reads it for both members of the pair. The same asymmetry
decides §4 item 3: a pair is excluded when *either* snapshot mismatches, and what that costs the table
against excluding on the round's own hash alone is in `repair-locality-measurements.md` under "Two
earlier readings of the table".

### 1.1 Measured over 40 recorded rounds

**The derivation is the script, and the script is what to run.** Every choice the numbers turn on is
in it: a section is any heading of level ≥ 2 whose text opens with a section id, at *any* depth
(`#### 3.1.1` is a section); a section is changed when the line diff attributes a changed line to it
**in the new snapshot**; the changed-section denominator is the new snapshot's section count; a
finding is inside when its parsed id equals a changed id or descends from one; the cycle figures are
**medians over that cycle's rounds** and the `all` row is the median over all 40 pooled rounds, not a
median of medians; the ratio divides the two unrounded medians and is rounded once, at the end.

```
python3 - <<'PY'
import json, os, re, difflib, hashlib, statistics as st
HEAD = re.compile(r'^#{2,}\s+(\d+[a-z]?(?:\.\d+[a-z]?)*)[.\s]')   # any heading level >= 2
LOC  = re.compile(r'^§(\d+[a-z]?(?:\.\d+[a-z]?)*)')               # leading section id of a location
def owners(lines):                       # line index -> id of the section that owns it
    hs = [(m.group(1), i) for i, l in enumerate(lines) if (m := HEAD.match(l))]
    starts = [i for _, i in hs]; own = [None] * len(lines)
    for k, (s, i) in enumerate(hs):
        for j in range(i, starts[k + 1] if k + 1 < len(starts) else len(lines)): own[j] = s
    return [s for s, _ in hs], own
def sha(p): return "sha256:" + hashlib.sha256(open(p, 'rb').read()).hexdigest()
def cycle(c):
    b = f"spec/.tp-review/{c}"
    by = {r["round"]: r for r in json.load(open(b + "/state.json"))["review_rounds"]}
    shares, secs, rounds, unparsed = [], [], [], 0
    for n in sorted(by):
        pa, pb = f"{b}/snapshot-round-{n-1}.md", f"{b}/snapshot-round-{n}.md"
        fp = f"{b}/review-round-{n}.ndjson"
        if n - 1 not in by or not all(map(os.path.exists, (pa, pb, fp))): continue
        rows = [json.loads(l) for l in open(fp) if l.strip()]
        if not rows: continue                                                          # empty file
        if sha(pa) != by[n-1]["spec_hash"] or sha(pb) != by[n]["spec_hash"]: continue   # §4 item 3
        old, new = open(pa).read().splitlines(), open(pb).read().splitlines()
        ids, own = owners(new); changed = set()
        for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, old, new, autojunk=False).get_opcodes():
            if tag != 'equal':
                for j in range(j1, j2):
                    if own[j]: changed.add(own[j])
        inside = located = 0
        for r in rows:
            m = LOC.match(str(r.get("location", "")).strip())
            if not m: unparsed += 1; continue
            located += 1; x = m.group(1)
            if any(x == s or x.startswith(s + ".") for s in changed): inside += 1
        if located and ids:
            rounds.append(n); shares.append(100 * inside / located)
            secs.append(100 * len(changed) / len(set(ids)))
    return rounds, shares, secs, unparsed
allS, allC, tot, unp = [], [], 0, 0
for c in ("0.35.0", "0.36.0", "0.37.0"):
    R, S, C, u = cycle(c); allS += S; allC += C; tot += len(R); unp += u
    s, k = st.median(S), st.median(C)
    print(f"| v{c} | {len(R)} | {s:.1f}% | {k:.1f}% | {s/k:.2f}x |")
s, k = st.median(allS), st.median(allC)
print(f"| all | {tot} | {s:.1f}% | {k:.1f}% | {s/k:.2f}x |   unparseable locations: {unp}")
PY
```

| cycle | rounds | findings in changed text | sections changed | concentration |
|---|---|---|---|---|
| v0.35.0 | 15 | 40.7% | 18.4% | 2.21× |
| v0.36.0 | 14 | **79.9%** | **22.2%** | **3.59×** |
| v0.37.0 | 11 | 92.6% | 71.4% | 1.30× |
| **all** | **40** | **72.1%** | **34.0%** | **2.12×** |

Of the 1,453 finding rows these 40 rounds filed, **7 yield no section id** and leave both sides of
the share — the script prints that count as `unparseable locations`, and §3.1 is the decision.

**Doc task.** `spec/1.0.0.md` §8 quotes the earlier 44-round form of this table, crediting *"the
repair-locality spec §1.1"*; it is updated when this release is implemented, and the sidecar's "Two
earlier readings of the table" says what differs and why a grounding round could not reproduce the
cells.

### 1.2 A grounding round measured this at 100%

A third ground round on another backlog spec found every one of its non-`PASS` rows in text the repair
between rounds had written; the instance, and the budget rule it argues for, are in the sidecar under
"A grounding round measured this at 100%". It is why §3 reports the share rather than gating on it: the
number is a prompt to look at what the last repair wrote, not a verdict on the document.

### 1.3 A third hand measurement, this one reproducible and with its rule stated

`spec/1.1.0.md`'s review cycle gives the statistic a third value, and unlike the two in §1 it can be
re-derived from committed artifacts: both snapshots and both round files are in the repository.

**The rule, stated before the count.** Diff `snapshot-round-1.md` against `snapshot-round-2.md`;
every added or replaced line is attributed to its enclosing `## N.` heading; a round-2 finding counts
as *in repair-written text* when the leading `§N` of its `location` is one of those headings. That is
**section granularity, and it is biased upward** — §5 holds half the findings and the repair rewrote
only part of it, so a finding against an untouched part of a touched section still counts.

At that granularity: the repair changed 24 lines across **§1, §3 and §5**, and **26 of 28** round-2
findings (92.9%) land in those three sections; only §2 and §4 carry one each. No row's `location`
failed to parse.

**Tightened one level, where the document allows it.** §5 is a numbered table, so a finding can be
attributed to a row rather than to the section. The repair rewrote or added rows **3, 5, 8 and 10**.
Of §5's fourteen findings, **ten name a rewritten row, four name no row at all, and none names only
a row the repair left alone.** Treating the four as unattributable in both directions gives a band of
**78.6%–92.9%** rather than a point.

So the three measurements read 24%, 43% and 78.6–92.9% — and the third is the only one whose rule and
bias direction are written down, which is §1's complaint about the first two. Two things follow that
the number alone does not carry. The qualitative fact is sharper than the percentage: **every §5
finding that names a row names one the repair had just rewritten**, so the round did not merely
concentrate on new text, it found nothing to say about the old. And a cycle at this value is not
thereby failing — the same cycle's repairs also closed defects that were verified closed by
construction; what the value says is that the round's *ask* had been almost entirely replaced by the
round before it, which is the condition §1 says nothing reports.

## 2. One number ranks the cycles backwards

**The share alone says v0.37.0 (93%) is the most repair-local cycle. By concentration it is the
least (1.3×).** Its rounds rewrote a median of 71% of the spec's sections, so almost anything a
reviewer found was necessarily in changed text. v0.36.0 put 80% of its findings into 22% of the file —
a genuinely local loop — and scores lower on the share.

**So the release reports both, and derives the ratio.** A single percentage is not reportable here: it
is a share of findings divided by nothing, and the denominator moves.

**They are two different pathologies and an operator needs to tell them apart:**

| | high concentration | low concentration |
|---|---|---|
| **high share** | the loop is chasing its own repairs in a small area | the repairs are rewriting most of the spec each round |
| **low share** | healthy — findings are spread over text the loop did not just write | — |

v0.36.0 is the top-left, v0.37.0 the top-right, and `CLAUDE.md` records the second being diagnosed by
hand as *"24% of the file was forensics each repair round had written for the next round to review."*
That figure is a share of the *file*, not of findings nor of sections — it corroborates the
placement and is neither of the two numbers this release reports.

**This ordering is the release's reason for existing, and it is the part that does not depend on the
cells.** It holds under the table above, under the 44-round form, and under an independent
reimplementation by a grounding round that reproduced none of the printed percentages: in all three,
v0.37.0 is first by share and last by concentration, and v0.36.0 is first by concentration.

## 3. What is reported

`--status` and `--record` carry, for a round whose predecessor exists **and has a snapshot**: the
share, the changed-section share, the derived ratio, the four integers the two percentages are
computed from — `findings_in_changed`, `findings_located`, `sections_changed`, `sections_total` —
and `location_parse_failures`.

**The raw counts travel with the percentages.** A repository has already been misled by a bare figure
whose counting rule was unstated, and by one whose denominator had moved while its numerator had not.
Two percentages and the five integers behind them cost nothing and cannot rot into a claim.
`location_parse_failures` is also how *approximate* reaches the output rather than only this
document: a consumer that sees it non-zero knows the share was computed over fewer rows than the
round filed.

**Round 1 reports nothing.** There is no predecessor, and a first round trivially scores zero, which
reads as a healthy loop rather than as no data.

**A round whose predecessor has no snapshot reports nothing**, rather than falling back to the
current spec, which would report a locality describing the operator's later edits. This guard has no
instance among the recorded review rounds — the phase that lacks snapshots is the audit phase, which
§4 item 5 keeps out; the census is in the sidecar under "The snapshot census". It exists for the case
`spec/backlog/round-records-the-text-it-read.md` test row 3 names — a round recorded with no
preceding emission.

### 3.1 Locations are parsed

**Locations are parsed, not matched, and a substantial share of review rows needs it.** Every recorded
review-round row carries a `location`, but many are not a bare section id: letter-suffixed sections
(`§8a`, `§2.0a`, `§8a.1`), multi-section values (`§12.1 / §12.4`, `§8.6 vs §9.1`), values with
trailing annotation (`§10.2 (line 120)`), bare heading numbers with no `§` (`3.1`, `4.2.3`), and free
prose. Under a naive prefix match every one of those scores as **outside** the changed text, biasing
the share **downward** — the direction that makes a repair-chasing loop look healthy, which is the one
failure §1 says nothing reports. So the value is parsed, by two patterns in order: take the leading
`§N[a](.N[a])*`; failing that, take a leading bare `N[a](.N[a])*` that is followed by whitespace,
punctuation or the end of the value — the survey found most of the values the first pattern rejects
are of that shape; for a multi-section value take the first id. What neither pattern parses is
excluded from the numerator **and** the denominator and counted in `location_parse_failures`. The
census and its derivation are in the sidecar under "Location shapes in the review corpus".

A row without a `location` cannot be recorded since v1.1.0 — `--record` refuses it
(`requiredFindingFields` in `internal/cli/review_record.go`) — so the missing-`location` case is a
guard for rounds recorded before that release, not a limit; the phase split behind it is in the
sidecar under "No review row lacks a location".

**The section attribution is approximate and is labelled so.** A parsed id is matched against the
changed sections by prefix **downward only**: `§3.2` counts as inside a changed `§3`, and a changed
`§3.2` does not make a finding at `§3` inside. Sub-section granularity in the other direction would
need every finding to carry one.

## 4. Non-Goals

1. **No gate, no threshold, no `next_action` branch.** A number nobody has acted on yet is not a rule.
   `CLAUDE.md`'s own precedent is narrower than this caution — a `workflow.checks` entry registered
   before its subject existed, which for eight rounds suppressed the finding class it was meant to
   measure — but it is the recorded case of a mechanism reading a signal too early.
2. **No single "locality score".** §2 is why: any one number here ranks the two known pathologies
   against each other rather than reporting both.
3. **No repair of the review rounds whose snapshot and recorded hash disagree.** They are excluded
   and counted as excluded, **per pair**: a round is dropped when *either* snapshot it is measured
   from mismatches, which is what §1's last paragraph is about. The census is in the sidecar under
   "The exclusion census".
4. **No attribution to *who* wrote the text.** Whether the operator, a repair unit or a role wrote a
   section is not recorded, and the commit-authorship fallback carries **zero** signal here rather
   than merely being a different feature: subagents commit as the operator, so `git log` over `spec/`
   returns one author (sidecar, "Authorship carries no signal").
5. **No audit-phase figure in this release.** Not because the snapshots are missing — most audit
   rounds have one. The measurement would be structurally uninformative there: in most consecutive
   audit snapshot pairs the two files are byte-identical, so the changed-section set is empty and the
   share is 0 by construction rather than by the loop being healthy; and a large share of audit rows
   carry no `location`, against none of the review rows, so the denominator would be missing much of
   what the phase filed. The three derivations are in the sidecar under "Why the audit phase is out".

The open question of whether `tp review` should carry a prior-round section of the kind `tp audit`
has is not decided here; `spec/undecided.md` holds it as "A prior-round section for `tp review`".

## 5. Tests

Every row derives from a numbered decision, names the input it runs against, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1.1 | over two snapshots and a findings file written into `t.TempDir()`, where one of four sections changed and 8 of 10 located findings fall in it: share 80%, sections 25%, ratio 3.2 | report the share alone, which cannot distinguish this from a round that rewrote everything |
| 2 | §2 | two such fixtures with **identical** shares and different changed-section counts produce different ratios | derive the ratio from the share, making the second input unreachable |
| 3 | §3 *counts* | the `--record` envelope carries `findings_in_changed`, `findings_located`, `sections_changed`, `sections_total` and `location_parse_failures`, and each percentage equals its quotient of the first four | emit percentages only, which is the shape this repository has twice been misled by |
| 4 | §3 *round 1* | round 1 of a fresh cycle emits no locality key at all | emit 0, which reads as a perfectly healthy loop |
| 5 | §3 *no snapshot* | with `snapshot-round-(N-1).md` deleted from the fixture, round *N* emits nothing and names the missing file | fall back to the current spec, reporting the operator's later edits as the round's locality |
| 6 | §3.1 *parse* | a fixture round whose `location` values are `§8a.1`, `§12.1 / §12.4`, `§10.2 (line 120)`, `4.2.3` and a line of free prose: the first four parse to `8a.1`, `12.1`, `10.2` and `4.2.3`, the prose is counted once in `location_parse_failures` and leaves both numerator and denominator | require a bare `^§[0-9]+(\.[0-9]+)*$`, which discards the first four as *outside* the changed text and biases every share downward |
| 7 | §3.1 *prefix* | a fixture whose two section ids are `§1` and `§10`, asserting first that they collide — `"§10".startswith("§1")` and `§10` is not a descendant of `§1` — then that a finding at `§10` is **not** inside a changed `§1` while one at `§1.1` is | match by string prefix without the separator, so `§1` swallows `§10` |
| 8 | §4 item 1 | replayed over a recorded cycle, `consecutive_clean`, `clean` and `--check`'s exit code are byte-identical with and without the fields | let it gate, which is the suppression precedent §4 item 1 names |

**Row 7 is the one a hand-written fixture will miss, and the pair has to be one the corpus
produces.** An earlier draft used `§3` against `§30`, invoking this repository's rule that a
fixture's incidental property must be asserted rather than chosen — and then chose a pair with the
property it was warning about: `§30` appears in no recorded row, while `§1` against `§10`–`§19` is
one of the separator-less prefix pairs the review corpus actually contains (sidecar, "Separator-less
prefix pairs"). Hence the assertion on the collision itself, before the behaviour.
