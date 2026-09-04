# The trim pass, and the scope pass it turned into — a note

**Not a release, and not yet a design.** This holds what has been measured about removing what a spec
carries, so nothing is re-derived when it is taken up. Deferred deliberately behind `spec/1.0.1.md`,
which is written and ready to ground.

**The note started with the wrong subject, and the correction is the most useful thing in it.** The
first version proposed a pass that removes, after a spec converges, the text the loop wrote about
itself — repair forensics. A field report from a session that had driven five ground rounds on a
33 KB spec **refuted its central fact by classifying its own diffs**, and pointed at a different pass
that belongs at the other end of the cycle. Both are kept below: the refuted reading is recorded
rather than deleted, because an entry silently edited to match its own correction teaches nobody why
the first reading was wrong.

## 1. What was refuted, and by what

The first version's third fact read: *a field report measured its floor growing 191 → 225 → 224 → 226
across five converging rounds while its `NOT-A-CLAIM` count stayed flat at 87 → 83 → 84 → 84 → 84, so
all the growth was claims, which is to say repair forensics.*

**The flat non-claim count is right and the inference from it is wrong.** Asked for the diffs, the
reporter counted units in round 5's floor whose `(text_sha, ordinal)` is absent from round 1's:
**74 new-or-changed against 39 that disappeared**, netting the +35 the first version reasoned from.
Hand-classified in a single pass by its author, who marks it a judgement rather than a measurement:

| class | the rule used | count |
|---|---|---|
| **new specification** | delete it and an implementer builds something different, or cannot build | ~38 |
| **fence / forensics** | delete it and they still build the right thing, but a later reader re-proposes the refuted option | ~21 |
| **correction in place** | replaced a false sentence in the same slot — net growth 0, its predecessor is among the 39 | ~15 |

**The decisive block: 23 of the 74 are one section's test-driver instructions**, rewritten because
round 1's version named drivers that do not work — a helper aimed inside a parallel region, a two-arg
form that raises a `TypeError`, a guard assertion given a fully-qualified class name. There was
nothing to preserve; the section was replaced with instructions that run.

So **claims ≠ forensics**, and "all growth was claims, therefore all growth was forensics" skips a
step. Most of that spec's growth was specification round 1 was missing or had wrong. **Forensics was
real and was about 21 of 74.**

## 2. Why a forensics trim is hard, and the two objections that survived

**The sentences worth most look exactly like bloat, and they share a grammar.** The four the reporter
would fight hardest for are all **pure negation** — a sentence whose entire content is that a
plausible thing is false:

- *"a bound of the form 'seven leaves × 7 days = 49 days' holds only on a strictly forward path with
  no revisits; a machine bouncing between two leaves survived 120 simulated days despite the 7-day
  timer"*
- *"the reason is **not** data loading: the action's body is empty"*
- *"60 days was chosen **not** because it covers the child's lifetime"*
- *"one test limitation is accepted up front: the fake does not reproduce the real failure"*

A length- or redundancy-based trimmer eats these first. The first is the only place in 33 KB that
records why the obvious arithmetic is wrong; without it the next reader re-derives 49 days in a
minute.

**The author is the worst judge of which of their own sentences are redundant** — five repairs in this
repository refuted their own round's finding on re-derivation, and the reporter's own repairs failed
grading in rounds 2 *and* 3. **And a direction that was not anticipated: the author is also the worst
judge of how to pose the question.** The reporter's first attempt at asking their user what to cut
used section numbers and class names and was unanswerable; the version that got an answer in one line
per item asked four plain questions per section — *what does it do / what exists today / what breaks
if we remove it / why is it a candidate* — in the domain's language. Same content, different register.
**The person who owns the scope owns the product, not the state machine.**

## 3. The pass that was actually wanted, which is a different pass

The reporter wanted a trim and would not have wanted this one. **Their spec's problem was not
forensics; it was an entire subsystem that should never have been in it**, deleted by their user in
one sentence once the question was finally posed. Measured on the converged spec:

| | |
|---|---|
| floor units in the sections now being deleted | **48 of 226 = 21 %** |
| ground units *asked* in those sections, all five rounds | **61 of 280 = 22 %** |
| review round-2 findings in those sections | **26 of 53 = 49 %** |
| of which critical | **5 of 5** |
| of which high | 13 of 24 |

**A post-convergence forensics trim would not have touched a line of it.** Those sections were
internally coherent, factually grounded and thoroughly reviewed. They were simply not wanted.

**And the ordering argument is this project's own, one level up.** Ground runs before review because
review is told the premises hold. The same shape: **ground and review both presuppose the spec's
scope.** Ground asks *is this sentence true*; review asks *is this design sound*; neither can ask
*should this section exist*. So a scope pass belongs **before ground**, for exactly the reason ground
belongs before review — grounding a section that should not exist is wasted, and reviewing it is
worse than wasted, because it returns criticals that pull toward fixing what should be deleted. All
five of that cycle's criticals did precisely that.

## 4. Why the existing interview does not cover it

tp has a Step 0 interview and the reporter ran two rounds of it. The failure is structural:
**the interview happens before the spec exists, so it cannot interview sections that writing
invents.** Their doomed subsystem existed because an interview question was answered *yes*, and the
consequence of that answer was then invented as a whole subsystem that nobody was ever asked about.
`SKILL.md` already says to pause and return to the interview when new ambiguities arise during
writing — that half is discipline, and it was missed — but **nothing in the tooling surfaces that a
section has appeared which no interview answer entails.**

## 5. Three things tp could do, cheapest first — with what is verified

**1. Findings per section in `tp review --status` — a reporting nicety, and NOT a scope signal.** The
data exists and is complete: `location` is present on **3,883 of 3,883** recorded review rows (100 %),
counting rule = every row of every `spec/.tp-review/*/review-round-*.ndjson`, so this is an aggregation
over a field already written. Keep it if it is free. **Do not let it prompt the scope question** — it
was demoted here after a false positive and a false negative arrived from opposite corpora.

**The false positive is this repository's.** Bucketing v0.37.0's review rounds by the leading section
number in `location`: §2 drew **143** findings against §1's 37 — a 3.9× spread — and §2 was that
release's **core clause**, not a scope error.

**The false negative is the field report's, and it is the sharper half.** Their §4.3 is the section
their user cut *first and most decisively* — one sentence, *"there is already a 30-day counter, we
should not touch any of this at all"* — and it produced **2 findings from 2 roles**, at the bottom of
the table beside sections nobody questioned. Seventeen floor units, every one `PASS` or `NOT-A-CLAIM`
by round 5. Nothing was wrong with it; it was not wanted.

**So density is uninformative in both directions — it is not even reliably necessary**, which is
strictly stronger than *necessary but not sufficient* and is why the earlier draft of this paragraph
was too generous. The obvious refinement fails too: cross-role agreement on their corpus scored doomed
sections `[5, 5, 4, 2]` against surviving `[4, 4, 3, 2, 2, 1, 1, 1, 1, 1, 1, 1]`, and of the six
sections drawing ≥ 3 roles, three were cut and three were kept — a coin flip.

**And the blind spot is structural rather than one unlucky section.** Their §4.3's signature is a
section grounding finds nothing in at all, and such sections are ordinary here: over sections with at
least six graded rows, **13 of 106 (12 %) are 100 % `PASS`/`NOT-A-CLAIM` across every round** —

```
python3 -c 'import json,glob,collections,re;s=collections.defaultdict(list);[s[(f.split("/")[2],re.match(r"[^0-9]*(\d+(?:\.\d+)?)",json.loads(l).get("anchor") or "").group(1))].append(json.loads(l)["verdict"]) for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip() and re.match(r"[^0-9]*(\d+)",json.loads(l).get("anchor") or "")];t=[k for k,v in s.items() if len(v)>=6];print(len([k for k in t if all(x in ("PASS","NOT-A-CLAIM") for x in s[k])]),"of",len(t))'
```

— so roughly one section in eight produces no ground signal whatever about whether it belongs. **The
section with the clearest scope error in that cycle was invisible to every automated signal the cycle
produced**: five ground rounds graded it clean and two review rounds barely noticed it. The only thing
that surfaced it was asking a person whether they wanted it.

**2. A goal-entailment question — better than density, and it also fails.** One prompt asking, per
section, *does §1's stated goal entail this section?* Scored by the reporter against their own four cut
sections: it catches §4.6, §4.7 and §5.4 — **24 findings and all five criticals** — and **misses §4.3**,
the section their user cut first and most decisively. Their §1's second sentence names three derivative
problems that map exactly onto §4.3, §4.4 and §4.5, so **§4.3 is explicitly entailed** and the test
returns a confident *derivable, keep* on the one section that motivated the whole idea.

**The failure mode is not the one this note predicted, and the prediction is recorded because it was
wrong.** An earlier draft expected a *vague* §1 to make every section derivable and the question to
return nothing. Their §1 is sharp and it failed anyway, for a different reason: **§1 was written by the
same author, in the same sitting, as the sections.** Scope creep propagates into the goal statement
ahead of the sections it licenses, so testing a document against its own §1 tests one author's single
act against itself. **A sharp §1 does not help if it is sharply wrong about what belongs — that is
worse than vagueness, because it produces a confident *derivable*.**

**The condition is checkable, and it holds on this entire corpus.** Counting rule: for each pending
spec, the commits that added the file —

```
for f in spec/*.md; do git log --format='%h' --diff-filter=A -- "$f" | wc -l; done
```

**All 22 pending specs were introduced in a single commit**, §1 and the body together, 17 of them in
one sweep on 2026-09-02. So entailment would degrade to self-confirmation on every spec here, and the
degradation is not detectable from inside the document.

**What survives is a narrower rule**: the entailment test is only as independent as §1's *provenance*.
It works where §1 comes from somewhere the section-writing did not — an issue description, a ticket, a
recorded interview answer — and self-confirms where §1 was drafted alongside the body. tp knows which
case it is in more often than not: Step 0's interview happens before §1 exists.

**3. A scope pass proper, before ground.** Its output is neither findings nor dispositions but a
**decision menu** — one row per section: *what it does / what exists today / what breaks if removed /
why it is a candidate* — in domain language, with one column only the human fills. The reporter's user
answered the whole menu in three lines, and it is the only one of the three that found **both** §4.3
and §4.6.

**All three internal signals fail for one reason, and that is the argument.** Density is uninformative
in both directions; cross-role agreement is a coin flip; entailment self-confirms when the goal is
home-grown. **Every one of them asks the document about itself.** Three independent failures, each
measured on a corpus that had already converged, is a stronger case for a human-answered pass than any
assertion about human judgement.

## 6. The property that separates it from every other pass

**Ground is agent-answerable** — probe the code. **Review is agent-answerable** — argue the design.
**Scope is not agent-answerable at all, only agent-preparable.** That makes it a different kind of
pass from the other three, and it is the argument that it should probably never gate anything.

## 7. What is measured about the trim itself, if it is ever taken

**The carry cost is local**, which is the objection that would have ended it and does not. `§8`'s carry
joins on `(text_sha, ordinal)`, and `ordinal` is an occurrence index per identical text rather than a
position index — every row in this repository's corpus carries `ordinal: 1`:

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["ordinal"] for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip());print(c)'
```

Deleting a unit therefore shifts no other unit's key. Measured in an `rsync` copy on `spec/1.49.0.md`
at round 3: `floor 58 / carried 47 / asked 11` before, and after deleting one line from the middle,
`floor 58 / carried 46 / asked 12`. **One line, one re-ask.** Independently corroborated by the
reporter: across five rounds with substantial rewriting their ask never exceeded what the edits
touched — 71, 13, 4, 1. Deletion behaves like any other edit.

**The one recorded precedent, and its payoff metric.** `CLAUDE.md` records v0.37.0 cutting **24 %** of
its own file after measuring roughly 50 findings a round against its shipped twin's 5, on normative
sections **8 % smaller**. The payoff was not round counts, which had gone flat (65, 56, 51, 47, 48,
60, 42, 40, 45, 53, 54, 69) — it was **decomposition**: 19 tasks with 8 blocked became **20 with 0**.

**The prediction to pre-register**, in the form that fired as written there: *if the asked count of the
next round and the blocked-task count at decomposition do not fall after the cut, the problem is the
loop and not the document.* A pass run without it measures nothing.

## 8. Open, and named so it is not mistaken for design

- **Whether the scope pass or the trim pass is the one worth building.** The evidence says scope: 21 %
  of one spec's units, 22 % of its asked units, 49 % of its review findings and 5 of 5 criticals sat
  in sections that should not have existed, and no forensics trim reaches them. The trim's own
  evidence is one precedent in one repository.
- **The candidate rule for a trim, if it is built.** `NOT-A-CLAIM` is the only measured signal
  available — 22.1 % of rows corpus-wide, strongly section-dependent: pooled by section position over
  19 specs it runs **2.4 % at §0 monotonically to 37.2 % at §7**, with §8 (a Tests table, one row per
  assertion) dropping back to 14.0 %. Any threshold must be relative to its section, and whether that
  is good enough is untested.
- **The better trim signal does not exist.** *Text a repair wrote, graded `PASS` in the round after* is
  what would work, and **nothing records which finding a repair answers.** A second field report
  established that this is structural rather than a corpus gap: content-matching a repair to its
  finding only works when the author copied the mechanism verbatim rather than reasoning from it.
- **Whether a destination beats a deletion.** The forensics exists because the loop has nowhere else to
  put it; a durable home would make the trim unnecessary, and that is already an undecided candidate in
  `spec/undecided.md` under a different name.
- **Decomposition evidence is absent on the field side.** The reporter's spec has not reached
  `tp import`, so the 19→20 / 8→0 precedent is confirmed by nothing outside this repository.
