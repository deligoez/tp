# tp v1.52.0 — Repair locality

> **This file is decisions.** §1.1 was measured for this release from the recorded snapshots, not
> quoted. The first form of the number — *the share of a round's findings sitting in text the previous
> round wrote* — turned out to rank two cycles backwards, and §2 is the correction.

## 1. Overview

A repair round rewrites part of the spec. The next round reviews it and files findings. **How much of
what a round finds is in text the round before it just wrote?**

Nothing reports this, so a loop that has stopped converging and a loop that is converging normally
look identical from the counts. `CLAUDE.md` records the diagnosis being reached by hand, once, after
twelve flat rounds.

This release reports the number. **It gates nothing.**

**It needs the mid-round re-emission counter, not the hash pinning** — a grounding pass corrected this,
and the original claim is recorded because its reasoning was plausible and did not connect. The
measurement compares snapshot *N−1* against snapshot *N* and **never reads `spec_hash` at all**, so
pinning the hash to the snapshot changes nothing here. What does threaten it is that a re-emission
overwrites a round's snapshot in place — measured — after which snapshot *N* is not what round *N*'s
roles read. The counter that records such a re-emission is what marks a locality figure suspect, and
that is this release's real prerequisite.

### 1.1 Measured over 44 recorded rounds

Derivation: for each round *N*, diff `snapshot-round-(N-1).md` against `snapshot-round-N.md`, collect
the sections containing changed lines, and count round *N*'s findings whose `location` falls in one.

| cycle | rounds | findings in changed text | sections changed | concentration |
|---|---|---|---|---|
| v0.35.0 | 19 | 45.2% | 21.9% | 2.07× |
| v0.36.0 | 14 | **79.9%** | **22.2%** | **3.59×** |
| v0.37.0 | 11 | 92.5% | 71.4% | 1.29× |
| **all** | **44** | **70.2%** | **30.1%** | **2.33×** |

**The first two columns are medians across the cycle's rounds; the ratio divides those two medians
before rounding, and that rule is stated because leaving it out produced a wrong cell.** A grounding
pass re-derived this table and found v0.35.0's concentration printed as `2.0×`, which is only
reachable by dividing the *rounded* percentages (45/22 = 2.045); the medians themselves give 2.068,
which rounds to 2.1. One cell in four discriminated between the two rules, and §3 of this same file
warns against exactly this — a figure whose counting rule is unstated.

## 2. One number ranks the cycles backwards

**The share alone says v0.37.0 (92%) is the most repair-local cycle. By concentration it is the
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

## 3. What is reported

`--status` and `--record` carry, for a round with a predecessor: the share, the changed-section share,
the derived ratio, and the raw counts both are computed from.

**The raw counts travel with the percentages.** A repository has already been misled by a bare figure
whose counting rule was unstated, and by one whose denominator had moved while its numerator had not.
Two percentages and the four integers behind them cost nothing and cannot rot into a claim.

**Round 1 reports nothing.** There is no predecessor, and a first round trivially scores zero, which
reads as a healthy loop rather than as no data.

**A round whose predecessor has no snapshot reports nothing**, rather than falling back to the current
spec. Twenty recorded audit rounds have no snapshot; comparing against today's text would report a
locality that describes the operator's later edits.

**The section attribution is approximate and is labelled so.** A finding's `location` is matched
against the changed sections by prefix, so `§3.2` counts as inside `§3`. Sub-section granularity would
need every finding to carry one, and **16.6% of recorded rows — 2,756 of 16,635 — carry no `location`
at all**; those are excluded from the numerator **and** the denominator, and the excluded count is
reported.

**That figure was 9.9% in a previous draft and it was a different statistic wearing this one's
clothes.** `CLAUDE.md` records *"303 of 3,047 recorded rows (9.9%) carry no `role` or `class`"* — a
different field over a five-times-smaller row set — and the number was transplanted onto `location`
without being re-derived. A grounding pass caught it. The lesson is the one this repository already
carries about a key-name search reported as a claim about data: **a borrowed figure must be re-derived
against the question it is being made to answer.**

## 4. Non-Goals

1. **No gate, no threshold, no `next_action` branch.** A number nobody has acted on yet is not a rule.
   `CLAUDE.md`'s own precedent is a mechanism registered before its subject existed, which suppressed
   for eight rounds the class it was meant to measure.
2. **No single "locality score".** §2 is why: any one number here ranks the two known pathologies
   against each other rather than reporting both.
3. **No repair of the 35 rounds whose snapshot and hash disagree.** Those rounds are excluded from the
   measurement and counted as excluded.
4. **No attribution to *who* wrote the text.** Whether the operator, a repair unit or a role wrote a
   section is not recorded, and inferring it from commit authorship would be a different feature.
5. **No audit-phase figure in this release.** The 44 measured rounds are review rounds; 20 audit rounds
   have no snapshot and the phase's rounds change code rather than the spec, so the same arithmetic
   measures something else.

## 5. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1.1 | a fixture where one of four sections changed and 8 of 10 findings land in it reports share 80%, sections 25%, ratio 3.2 | report the share alone, which cannot distinguish this from a round that rewrote everything |
| 2 | §2 | two fixtures with **identical** shares and different changed-section counts produce different ratios | derive the ratio from the share, making the second input unreachable |
| 3 | §3 *counts* | the four raw integers are emitted alongside and are consistent with the percentages | emit percentages only, which is the shape this repository has twice been misled by |
| 4 | §3 *round 1* | round 1 emits no locality fields at all | emit 0, which reads as a perfectly healthy loop |
| 5 | §3 *no snapshot* | a round whose predecessor lacks a snapshot emits nothing and says why | fall back to the current spec, reporting the operator's later edits as the round's locality |
| 6 | §3 *no location* | rows with no `location` are excluded from both numerator and denominator, and the excluded count is reported | count them as outside, which silently improves every figure |
| 7 | §3 *prefix* | `§3.2` counts as inside a changed `§3`, and `§30` does **not** | match by string prefix without the separator, so `§3` swallows `§30` |
| 8 | §4.1 | convergence, `clean` and `--check`'s exit are identical with and without the fields | let it gate, which is the suppression precedent §4.1 names |

**Row 7 is the one a hand-written fixture will miss.** `§3` and `§30` differ by one character and a
prefix match is the obvious implementation; this repository has already lost a round to a fixture whose
incidental property carried the verdict.
