# The trim pass — a note, deferred behind `spec/1.0.1.md`

**Not a release, and not yet a design.** This holds what has been measured about a proposed pass that
removes what a spec has accumulated, so nothing here is re-derived when it is taken up. It is deferred
deliberately: `spec/1.0.1.md` is written and ready to ground, and a design question should not block a
release that is.

Its subject is one sentence: **after a spec converges, should a pass remove the text the loop wrote
about itself, before decomposition consumes the result?**

## 1. Why it is not obviously a good idea, stated first

Three ways it could be wrong, because a note that only argues for something is an advertisement.

**The best sentences in these specs are the ones a naive trim would delete.** *"An earlier draft said
X, and one command refuted it"* is the highest-value shape this repository produces, and it looks
exactly like bloat. `CLAUDE.md`'s own rule: an entry silently edited to match its own correction
teaches nobody why the first reading was wrong.

**The author is the worst judge of what is redundant.** Measured twice in one day, from two
directions: a field reporter's own repairs failed grading in rounds 2 *and* 3, and five repairs in
this repository refuted their own round's finding on re-derivation. "Which of my sentences are
unnecessary" is the same judgement, made by the same person.

**It is an unmeasured intervention in a project whose rule is to prove by running.** "Trimming makes
a spec more processable" is a hypothesis, and until §4's prediction is written down before a pass
runs, it stays one.

## 2. What is measured — the three facts that shape it

**The carry cost is local, and this is the fact that decides feasibility.** `§8`'s carry joins on
`(text_sha, ordinal)`, and `ordinal` is an occurrence index per identical text, **not** a position
index — every row in this repository's corpus carries `ordinal: 1`:

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["ordinal"] for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip());print(c)'
```

So deleting a unit does not shift any other unit's key. Measured in an `rsync` copy on
`spec/1.49.0.md` at round 3: before the edit, `floor 58 / carried 47 / asked 11`; after deleting one
line from the middle of the document, `floor 58 / carried 46 / asked 12`. **One line deleted cost
exactly one re-ask.** A trim does not un-converge the document, which is the objection that would
have killed it.

**There is one recorded precedent in this repository and it paid off.** `CLAUDE.md` records v0.37.0
cutting **24 %** of its own file — forensics each repair round had written for the next round to
review — after measuring roughly 50 findings a round against its shipped twin's 5, on normative
sections **8 % smaller**. The payoff was not measured on round counts, which had gone flat
(65, 56, 51, 47, 48, 60, 42, 40, 45, 53, 54, 69). It was measured on **decomposition**: 19 tasks with
8 blocked became **20 with 0**.

**The growth is structural, not stylistic.** A field report on a 226-unit spec measured its floor
growing 191 → 225 → 224 → 226 across five converging rounds while its `NOT-A-CLAIM` count stayed flat
at 87 → 83 → 84 → 84 → 84. **All the growth was claims** — which is to say repair forensics, since
that is what repair rounds add. The target is therefore not wordiness. It is text the loop wrote
about its own operation.

## 3. What the shape has to be, given those three

**Timing: after grounding converges, before decomposition.** Trimming *before* grounding deletes
claims before they are graded, which costs the corpus the defect instances that are its whole value,
and does it on unmeasured judgement. Trimming after leaves per-unit data — verdict, kind, tier,
anchor — as a measured basis, and decomposition is both the consumer and where the one recorded
payoff was measured.

**The rule cannot be "remove forensics".** It has to separate two things that look identical:

| keep | cut |
|---|---|
| forensics recording a **refuted claim** — a fence against re-proposing it | forensics recording **how the round got there** — the loop's notebook |

**The pass is run by a unit that did not write the text**, on candidates derived from the record
rather than chosen by reading, with the operator deciding. That follows from §1's second objection.

## 4. The prediction, to be written before any pass runs

v0.37.0's rule, in the form that fired as written: **if the asked count of the next round and the
blocked-task count at decomposition do not fall after the cut, the problem is the loop and not the
document.** Pre-register it; a pass run without it measures nothing.

## 5. What is undecided, named so it is not mistaken for design

- **The candidate rule.** `NOT-A-CLAIM` is the only measured signal available today — 22.1 % of rows
  corpus-wide, but strongly section-dependent: pooled by section position over 19 specs it runs
  **2.4 % at §0 monotonically to 37.2 % at §7**, with §8 (the Tests table, one row per assertion)
  dropping back to 14.0 %. So a `NOT-A-CLAIM` unit is anomalous in §2 and expected in §6, and any
  threshold has to be relative to its section. Whether that is a good enough candidate rule is
  untested.
- **The better signal does not exist yet.** The candidate that would actually work is *"text a repair
  wrote, graded `PASS` in the round after"* — forensics that has served its purpose. It is not
  derivable: **nothing records which finding a repair answers.** A field reporter established that
  this is structural rather than a gap in one corpus — content-matching a repair to its finding only
  works when the author copied the mechanism verbatim rather than reasoning from it.
- **Whether tp does anything at all here.** The pass could be entirely a process, or tp could report
  removal candidates from the recorded rounds and let the operator decide — which is the shape this
  project prefers (P4, and *reporting is not scheduling*). No decision.
- **Whether the destination matters more than the deletion.** The forensics exists because the loop
  has nowhere else to put it. A durable home for it would make the trim unnecessary, and that is
  already an undecided candidate in `spec/undecided.md` under a different name.

## 6. Where this came from

Raised by the operator after 28 recorded ground rounds across 19 specs, on the observation that every
repair in the programme grew its spec — `spec/1.43.0.md` +130/−49 and over `section-size`,
`spec/1.46.0.md` 176 → 290 lines, `spec/1.50.0.md` 337 → 440, `spec/1.0.1.md` 471 insertions in one
pass. The growth is real and monotone; whether it is a cost is what §4 would measure.
