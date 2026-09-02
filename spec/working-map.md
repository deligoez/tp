# Working map — the effort in flight

**This file is an index, not a store.** A finding lives in exactly one place: the spec that takes it.
The map gists it and points; it never restates it. A second home is how a claim drifts, and this
repository has already paid for that four times.

Not a release. It carries no version number so it cannot be mistaken for one.

## Destination

**Reached: `spec/1.0.0.md` — `tp ground` — is written**, from three pilot runs over 44 claims. The
protocol below was its input; the spec is now the single source of truth for it, and this section of
the map is history rather than instruction.

What remains of the effort: the process findings are routed into the specs that own them (**done**),
and **v0.37.1 ships** (ready, waiting on the operator). Implementing any spec is out of scope:
nothing here starts a tp cycle.

## Work items

| # | work | state |
|---|---|---|
| W1 | **Ground the pending specs.** Rules in `spec/1.0.0.md` §2–§6; log below | 4 fully ground, `1.0.0` by its own rules; the citation and numeric classes ground across all 18 |
| W2 | Route the skills-examination findings into the specs that own them | **done** — 1.48.0, 1.41.0, 1.50.0, candidates |
| W3 | **v0.37.1 ships.** Spec written and ground, both fixes built and green in a scratch copy | ready — waiting only on the operator |
| W4 | `forward-spec-ref` lint rule — survived prototyping (18 findings pre-repair, 0 today, 0 false positives) | needs a decision: it wants the shipped boundary, which `tp lint` has no git access for |
| W5 | CLAUDE.md carries **326 of 632 lines (53%)** of planning reference in an always-loaded document | separate effort; sized, not started |
| W6 | tp's emitted prompts steer by prohibition — **14%** of the review prompt, **12%** of CLAUDE.md | routed into 1.41.0; the CLAUDE.md half is W5 |

## The ground protocol — superseded

**`spec/1.0.0.md` is now the single source of truth for every rule below.** What follows is kept as
the pilot's working text, because the spec cites it as its input; it is not a second home for the
rules and must not be edited to disagree with the spec.

**1. Enumerate the claims.** A claim is a statement about how the world *is*, or about what a proposed
mechanism *does when run*. Decisions and predictions are not claims. **Counting by intuition
undercounts**: a keyword heuristic said 11 for a spec that carried 17.

**2. Order them.** A claim whose verdict depends on another unsettled claim belongs to a later pass.
`1.52.0`'s *"concentration 2.0×"* could not be judged before *"how is the ratio derived"* was settled,
and the second was never stated.

**3. Match the evidence to the claim's kind.** Reading is evidence about text; only running is
evidence about behaviour. Both were tried on behavioural claims this cycle: `1.49.0`'s three held
under reading, and `1.43.0`'s `-type d` mechanism read perfectly and **did not work at all**.

| the claim is about | the evidence |
|---|---|
| what a document says | read it |
| how code is structured | read it |
| what the corpus contains | run a query |
| existing behaviour | run the command |
| whether a proposed mechanism works | build a probe and run it |
| whether a defect is real | write the test, watch it red, fix, watch it green |
| whether a guard measures anything | break its subject, run the suite, then run the control |

**4. Before concluding, name three to five falsifiable hypotheses and rank them.** Single-hypothesis
reasoning anchors on the first plausible idea. This cycle anchored three times and was wrong each
time — a checker read as fail-open when the pipeline was eating the exit code; a digest fix that
never ran; a false-positive class characterised from the wrong reading. Each states its prediction:
*if X is the cause, then changing Y removes it.* A hypothesis with no prediction is a vibe.

**5. Facts are yours to find, never the operator's to supply.** Where a claim needs a fact from the
environment, go and get it. Only two things reach the operator: a fact **only the author holds**
(a field report, an intent), and a **decision**.

**6. When you must ask, show and do not block.** Put the ranked list in front of the operator and
carry on with the claims that do not depend on the answer. A question that halts everything is an
escalation, and escalation is for a decision, not for a fact you have not finished chasing.

**7. Repair carries its derivation.** A correction that arrives with the command that produced it is
verified at birth. One that does not is a new ungrounded claim, and corrections **can** be wrong —
two were, this cycle.

**8. Phrase the repair positively.** Steering by prohibition drags the forbidden thing into context.
State the target behaviour instead; keep a prohibition only where it is a hard guardrail with no
positive phrasing, and pair it with the target even then.

## Ground log

| spec | claims | PASS | PARTIAL | FAIL | UNVERIFIABLE | QUESTION | deepest evidence used |
|---|---|---|---|---|---|---|---|
| 1.52.0 | 17 | 14 | 2 | 1 | 0 | 0 | corpus query |
| 1.53.0 | 10 | 9 | 0 | 1 | 0 | 0 | **simulation probe** |
| 0.37.1 | 17 | 14 | 0 | 3 | 0 | 0 | reachability read + tag history |
| **1.0.0** | **32** | **20** | **4** | **8** | 0 | 0 | **built-and-run probe, with a control** |
| *all 18 files* | — | — | — | **49** | — | — | the registered citation check, run over the corpus |

**Four runs, and the pattern has not moved: every refutation was a cheap sentence.** A borrowed
number, an unstated rounding rule, a plausible-sounding reason, and four quantifiers — *four patch
releases* (ten), *five releases* (seventeen), *fifteen audit histories* (sixteen), *the driver
branches on it* (nothing reads it). Every claim that cost something to make held, because making it
**was** the verification. **Start with the cheap sentences.**

### The fourth run: the protocol turned on its own spec

**The fourth run put the protocol on the document that specifies it, and the refutation rate went up,
not down: 8 of 32 against 7 of 44.** Enumeration undercounted again — a first reading found ~21 claims
where a second found 32 — which is the map's own weakest step firing on the run that was watching for
it. Four results are worth carrying out of it:

- **A spec is likeliest to be wrong about its own evidence.** Three of the eight refutations were the
  document mis-describing how it knew something: the three pilot specs *"had already been reviewed"*
  (`review_rounds` is empty for all three, and no `.tp-review/` directory exists for any pending spec
  or its `0.38.0`–`0.54.0` predecessor); a read reported as a run, because the run was what the
  argument needed; and a rule claimed to have *"fired or been refuted"* in the pilot while the
  document's own §3 says that verdict has zero pilot instances.
- **A figure with no counting rule is not merely unproven, it is unreproducible — and it can point the
  wrong way.** *"15 of 46 sentences"* survived sixteen readings of the counting rule across two file
  versions without coming close (nearest 21 of 55). Re-derived with the rule stated, the heuristic
  catches **75%** of the ground spec's own prose units, not a third: the claim was wrong in direction,
  not just in magnitude, and the section's argument had to be rebuilt on the measurement.
- **A precedent citation is a claim, and this one was inverted.** The ground spec appealed to the audit
  phase's isolation clause as already requiring a scratch copy. The clause forbids the edit outright
  (`internal/cli/clauses.go:19`), and `spec/0.36.0.md` §2.4 — headed *"What is deliberately not
  provisioned"* — rejected that exact recipe for three reasons. **Grounding a citation means reading
  what it cites**: two of the three do not bind a one-prompt phase, and the third rests on a reference
  to a section that turned out to be *Agents*. The operator then took the decision the paragraph had
  been assuming — the unit makes its own copy, tp provisions nothing — and §4.1 now states it.
- **Absence claims want the whole surface.** *"Nothing checks the spec against the world"* is false as
  written: `scripts/check-spec-code-citations.py` is registered in `workflow.checks`, and
  `scripts/check-measured-claims.py` re-runs a claim's derivation and fails when it goes stale. Both
  are narrow, both are precedent, and the second's four recorded escapes are now constraints on the
  spec that overlooked it.

**Two defects found in neighbours while grounding this one, routed and not restated here.** The
PASS-note release quotes **21,924 of 22,592** `PASS` rows carrying a note; the committed corpus holds
**11,505 of 11,839** under the field `notes` — the **share** (97.0% vs 97.2%) is right and the counts
are not reproducible from the tree. And `spec/0.36.0.md` §2.4 cites `spec/0.35.0.md` §6.3 for its
no-second-write rule; that section is *Agents* — a fifth renumbering casualty, found by reading the
citation rather than by checking that the file exists.

### The first three runs

**The three-hypothesis step earned its place on first use.** 0.37.1 claimed
`unresolved_findings` was driver-facing. Three hypotheses — the driver branches on it / it reports
only / nothing reads it — and the third held: the field is written at two sites and **read nowhere in
tp**. The severity argument survived, but its subject changed from the driver to a reader, and the one
documented misled reader is this repository's own orchestrator.

**A whole claim class was ground at once, and the check this effort registered found it.** 49 path
citations across the pending set named a file that does not resolve: a bare filename plus a line
number, where `internal/cli/audit.go:355` was meant. Bare names without a line number are an accepted shorthand and were left
alone; the checker's rule is *slash or line number*, which a hand replication of it got wrong before
the real thing was run.

**A numeric sweep over every bolded figure in the pending set found nothing further.** Two suspects
resolved on inspection: `156 rounds` is correctly framed as history beside the current 168, and
`3,046 rows` is scoped to a 22-file subset rather than the 16,635-row corpus.

## The v1.0.0 review loop — decision rule, pre-registered before round 3

Written **after** round 2 recorded and **before** its repair, because this repository's own lesson is
that a loop ended on a rule written afterwards has been ended on the result.

**Counts so far: 45, then 61.** The rise is expected and is not by itself a signal — the round-1
repair rewrote most of the document, and `ax-economist` measured that **71 of round 2's 84 floor
units (84.5%) are new or changed against round 1's 63**. A round that reviews mostly new text draws
findings on mostly new text.

**So the signal is two clusters, not the total.** Round 2's 61 split into a tier cluster and a test
cluster, and `architect` supplied the diagnosis the repair is built on: the five highs are not five
defects but one — *the spec decided which tier is enough and never decided what the record does with
the tier actually reached* — with the instruction to **repair §7.2 first and re-derive §4.1, §4.2 and
§6 from it, not in parallel.**

| cluster | round 2 | round 3 prediction | what a miss means |
|---|---|---|---|
| §7.2 + §4.1 + §4.2 + §6 + §3 | **24** | **< 8** | the tier concept is wrong rather than underspecified; cut `tier` to `read`/`run` and delete the kind table rather than refine it a third time |
| §11 | **9** | **< 4** | the table's format is the defect; drop the mutant column for a per-row named input |

**And a stop rule.** If round 3's total is not below **45** while both clusters are clean, the loop is
reviewing this document's prose rather than its decisions, and the answer is to cut the document
rather than repair it again — the same rule that ended `spec/0.37.0.md`'s review at twelve flat
rounds.

**Two structural repairs go in with the rest, because both roles said the row could not carry them.**
`tester`: the input a row's mutant column claims is discriminating must appear in that row's
*assertion* column — three of round 2's §11 findings are one defect with that one cause, and the
format invites it. `regression`: after a repair that changes a vocabulary, sweep the document for
every sentence reasoning *from* the old vocabulary; seven of its nine silent changes sit in untouched
sentences adjacent to rewritten ones, which is the signature of a section-by-section rewrite with no
downstream sweep.

### Round 3's outcome — the pre-registered rule missed, and what was done instead

**Both predictions missed, and the record says so before it says anything else.**

| cluster | round 2 | round 3 | predicted | |
|---|---|---|---|---|
| §7.2 + §4.1 + §4.2 + §6 + §3 | 24 | **12** | < 8 | missed |
| §11 | 9 | **9** | < 4 | missed |
| total | 61 | **54** | (stop rule: < 45 with both clean) | clusters not clean, so the stop rule did not fire |

**The rule's consequences were not applied, and that is an override rather than an oversight.** It
said a missed tier cluster means the tier concept is wrong and should be cut to `read`/`run`. The
round's own evidence says otherwise: `architect` verified the tier machinery against shipped code and
found the gap had **moved** — to §3's `QUESTION` definition, which defines the verdict over a
disjunction while §7.2 had legalised only one disjunct — and `tester` found rows 6, 7 and 7b all
discriminate. Cutting a mechanism three roles report working, on a threshold guessed a round earlier,
would be worse than the defect.

**Why the rule missed is the lesson, and it is not "the threshold was wrong".** It was pre-registered
on **location counts**, and a location is not a lens. §11's count held at 9 while the role that owns
§11 went from 16 rows to 9, withdrew **12 of 14** judgements, and found **one** unfailable mutant
against five and then seven. The section's count stayed flat because *other* roles moved into it. A
count per `location` conflates *this section is defective* with *roles are looking here*.

### The decision: stop repairing in prose, build the derivation and run it

Four of five roles said it independently and the fifth's measurements support it. The evidence that
settled it: **`scripts/floor-prototype.py`, one run, found four defects three review rounds had not.**

| what the run settled | prose could not |
|---|---|
| §2.1 step 3 strips list markers from the first line only → **777 sub-four-character floor units and 143 colliding `text_sha`** across 53 specs, 0 and 3 after the repair | eight of them sat in this spec's own §10 through two full rounds of five-role review |
| §2.1 step 4's terminator is unstated → two conforming readings agree on **5.0%** of hashes | same segmentation, so no reading of the prose exposes it |
| §11 row 4's byte bound is undecidable without an encoding → JSON over on **52 of 53**, the labelled shape under on **53 of 53** | three serialisations differ by 70% and the spec named none |
| the floor grew **74 → 98 → 143** across three snapshots | every figure the spec quoted about itself was stale within one round |

**The companion-file split went in with it.** `spec/1.0.0-corrections.md` holds the withdrawal
derivations; the spec cites a row in one clause. `ax-economist` measured why: §2.2 obliges a
disposition per floor unit **per round**, so a paragraph of forensics is priced once per round for the
life of the cycle, and 16% of round 2's new floor units narrated deleted drafts. The pattern is the
repository's own — `0.34.0-guard-tests.md`, `0.34.0-release-counts.md`, `0.35.0-candidates.md`.

### Pre-registered for round 4 — on better instruments this time

Location counts are abandoned. Two measurements from outside the loop's own inputs:

| instrument | round 1 | 2 | 3 | round 4 rule |
|---|---|---|---|---|
| `implementer`'s blocked tasks | 13 | 11 | 10 | **below 6, or the spec stops converging toward buildability** — decompose what is buildable and let task acceptance carry the rest |
| findings on §2.1 + §2.2, now prototype-backed | — | 6 | 13 | **below 5, or running did not settle them either** and the floor is cut from the release |

**And a standing note so a round does not spend a finding on it:** `long-spec` and `section-size` fire
on this document and on three of five shipped specs. They are advisories, not defects.

### Round 4's outcome — the rule fired, and this time it is followed

| instrument | 1 | 2 | 3 | 4 | rule | |
|---|---|---|---|---|---|---|
| `implementer`'s blocked tasks | 13 | 11 | 10 | **8** | below 6 | **missed** |
| findings on §2.1 + §2.2 | — | 6 | 13 | **9** | below 5 | **missed** |
| total findings | 45 | 61 | 54 | **46** | — | |

**Everything that measures the document says it is converging.** Findings falling since round 2;
blocked tasks falling; `regression`'s defect rate **15.5% → 14.8% → 10.3%**; blocker locality
**8 of 10 → 4 of 8** in freshly written text; zero duplicates in the merge, so five roles are finding
genuinely distinct things; `tester` withdrew 12 of 14 judgements in round 3 and `architect` discharged
three in round 4 by reading the artifact rather than the spec's claim about it.

**And one measurement says those numbers overstate it.** `tester` checked its own nine round-3
findings against the post-repair text: **five were textually unchanged** while all nine carried
`resolved.status: fixed` with one blanket evidence string. Re-derived over the whole round with the
guard `tester` prescribed — a finding quoting a baseline anchor should not still find that anchor —
**15 of 21 (71%)** survive their own repair. That is an upper bound rather than a count, since a
finding can quote a correct sentence and argue about its consequence. It is still the decisive fact of
the cycle: **a `--resolve-all` with one evidence string is not a record of repair**, and every later
round was told not to re-report what it named.

**So the pre-registered consequence is taken as written: decompose what is buildable and let task
acceptance carry the rest.** Round 3's consequence was overridden with reasons; overriding a second
time would mean there was never a rule. Three things make it the right call independently of the rule:
four roles have said for two rounds that the remaining defects need *building* rather than
specifying; the cycle's most valuable findings came from running (`scripts/floor-prototype.py` found
in one run what three review rounds missed, and `implementer` then broke the prototype by running it
harder); and tp's own workflow puts code-versus-spec conformance in the audit phase, which is what
half of round 4's findings actually want.

**Two process repairs go in first, because decomposing broken text produces broken tasks.**
Round 4's findings are applied as a **shrinking** repair — the first of the cycle: `question_shape` is
deleted (`ax-economist` proved it a total function of `(kind, tier)`, carrying zero bits), §11's
corpus-quantified rows are restated over constructed inputs (`implementer`: a test over a directory
that changes every release is a standing tax, and two of three instances were already red), and §3's
asymmetry paragraph loses the ordering claim it derived from the section that denies ordering. And
resolution stops being blanket: each finding is dispositioned on its own anchor.

## Routed from the skills examination

Source: `mattpocock/skills`, read in full (37 skills, 164 files). Each row is routed or parked; the
substance lives in the target, never here.

| finding | source skill | target | state |
|---|---|---|---|
| Build the loop before any hypothesis; the loop is the work, everything else is mechanical | `diagnosing-bugs` | 1.48.0 | routed |
| 3–5 ranked falsifiable hypotheses before testing any | `diagnosing-bugs` | 1.48.0 + protocol §4 | **routed** — 1.48.0 §2 step 3 |
| Completion criterion as a checklist the unit can check itself against | `diagnosing-bugs` | 1.48.0 | routed |
| Minimality test: every remaining element is load-bearing, remove one and it goes green | `diagnosing-bugs` | 1.48.0 | **routed** — 1.48.0 §3 |
| "If no correct seam exists, that itself is the finding" | `diagnosing-bugs` | 1.48.0 | **routed** — 1.48.0 §3 |
| Demand drives legwork — *"every modified model accounted for"* forces work that *"produce a list"* does not | `writing-for-agents` | 1.41.0 | **routed** — 1.41.0 §3 |
| Negation is a failure mode; prompt the positive | `writing-for-agents` | 1.41.0 (emission) + W5 (CLAUDE.md) | **routed** — 1.41.0 §4a; CLAUDE.md half parked as W5 |
| Leading words recruit pretraining; an invented word pays in definition tokens | `writing-for-agents` | naming, `ground` | settled — validates the name |
| Two axes reported separately, never merged or reranked, because one masks the other | `code-review` | 1.50.0 | **routed** — 1.50.0 §4.1 |
| Facts are the agent's job; decisions are the user's | `grilling` | protocol §5 | done |
| Frontier: a question depending on an open question belongs to a later round | `grilling` | protocol §2 | done |
| Show the ranked list; do not block on it | `diagnosing-bugs` | protocol §6 | done |
| Fog of war — *can you state the question precisely now*, not answer it | `wayfinder` | `candidates.md` | **routed** — `candidates.md` |
| Context load vs cognitive load; the second is the price of human agency | `writing-for-agents` | W5 | parked |
| No-op test: an instruction the model already obeys pays load to say nothing; settle it by running | `writing-for-agents` | W5 | parked |

**Three independent convergences**, recorded because agreement reached separately is the strongest
evidence either party has: *a decision lives in one place, refer by name not id* (this cycle derived
it after 47 stale citations); *plugin.json's version gates whether users see an update* (their ADR
0002, tp's release rule, identical); and *demand drives legwork* (their prose, tp's measured 15/23
against a control of 35).

**Not taken, with reasons.** Issue-tracker centricity — tp is file-based. `to-spec`'s user-story
template — tp's specs are decision documents a review loop attacks, and the measurement-plus-mutant
form is stronger for that. `implement` — 15 lines against tp's brief, gate, commit-strategy and
escalation machinery.

## Fog

In scope, not yet sharp enough to state as a question.

- **Where the ground record lives.** Two runs suggest a precondition to review rather than a phase of
  its own, but two points do not settle it, and the shape decides whether a record is needed at all.
- **Claim enumeration.** The weakest step, and the only one that has now failed on every run:
  intuition undercounted 11 against 17, then 21 against 32 on the ground spec itself. The mechanical
  floor is not the answer either — measured over that spec it flags 75% of the prose units, so it
  substitutes a coverage obligation for a shortlist. Whether this is a parsing problem, a definition
  problem, or irreducibly a reading problem is not yet clear.
- **What `UNVERIFIABLE` costs.** Zero instances in 76 grounded claims, so the verdict is designed and
  untested. A spec resting on a field report would produce one; none has been grounded yet.
