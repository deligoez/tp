# Working map — the effort in flight

**This file is an index, not a store.** A finding lives in exactly one place: the spec that takes it.
The map gists it and points; it never restates it. A second home is how a claim drifts, and this
repository has already paid for that four times.

Not a release. It carries no version number so it cannot be mistaken for one.

## Destination

Every pending spec is **grounded** — its claims measured rather than asserted — the process findings
this effort surfaced are routed into the specs that own them, and **v0.37.1 ships**. Implementing any
spec is out of scope: nothing here starts a tp cycle.

## Work items

| # | work | state |
|---|---|---|
| W1 | **Ground the pending specs.** Protocol below; log below | 3 fully ground; the citation and numeric classes ground across all 18 |
| W2 | Route the skills-examination findings into the specs that own them | **done** — 0.48.0, 0.41.0, 0.50.0, candidates |
| W3 | **v0.37.1 ships.** Spec written and ground, both fixes built and green in a scratch copy | ready — waiting only on the operator |
| W4 | `forward-spec-ref` lint rule — survived prototyping (18 findings pre-repair, 0 today, 0 false positives) | needs a decision: it wants the shipped boundary, which `tp lint` has no git access for |
| W5 | CLAUDE.md carries **326 of 632 lines (53%)** of planning reference in an always-loaded document | separate effort; sized, not started |
| W6 | tp's emitted prompts steer by prohibition — **14%** of the review prompt, **12%** of CLAUDE.md | routed into 0.41.0; the CLAUDE.md half is W5 |

## The ground protocol

Stable before it is used, because it runs 14 more times and every improvement pays off 14×.

**1. Enumerate the claims.** A claim is a statement about how the world *is*, or about what a proposed
mechanism *does when run*. Decisions and predictions are not claims. **Counting by intuition
undercounts**: a keyword heuristic said 11 for a spec that carried 17.

**2. Order them.** A claim whose verdict depends on another unsettled claim belongs to a later pass.
`0.52.0`'s *"concentration 2.0×"* could not be judged before *"how is the ratio derived"* was settled,
and the second was never stated.

**3. Match the evidence to the claim's kind.** Reading is evidence about text; only running is
evidence about behaviour. Both were tried on behavioural claims this cycle: `0.49.0`'s three held
under reading, and `0.43.0`'s `-type d` mechanism read perfectly and **did not work at all**.

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
| 0.52.0 | 17 | 14 | 2 | 1 | 0 | 0 | corpus query |
| 0.53.0 | 10 | 9 | 0 | 1 | 0 | 0 | **simulation probe** |
| 0.37.1 | 17 | 14 | 0 | 3 | 0 | 0 | reachability read + tag history |
| *all 18 files* | — | — | — | **49** | — | — | the registered citation check, run over the corpus |

**Three runs, and the pattern has not moved: every refutation was a cheap sentence.** A borrowed
number, an unstated rounding rule, a plausible-sounding reason, and four quantifiers — *four patch
releases* (ten), *five releases* (seventeen), *fifteen audit histories* (sixteen), *the driver
branches on it* (nothing reads it). Every claim that cost something to make held, because making it
**was** the verification. **Start with the cheap sentences.**

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

## Routed from the skills examination

Source: `mattpocock/skills`, read in full (37 skills, 164 files). Each row is routed or parked; the
substance lives in the target, never here.

| finding | source skill | target | state |
|---|---|---|---|
| Build the loop before any hypothesis; the loop is the work, everything else is mechanical | `diagnosing-bugs` | 0.48.0 | routed |
| 3–5 ranked falsifiable hypotheses before testing any | `diagnosing-bugs` | 0.48.0 + protocol §4 | **routed** — 0.48.0 §2 step 3 |
| Completion criterion as a checklist the unit can check itself against | `diagnosing-bugs` | 0.48.0 | routed |
| Minimality test: every remaining element is load-bearing, remove one and it goes green | `diagnosing-bugs` | 0.48.0 | **routed** — 0.48.0 §3 |
| "If no correct seam exists, that itself is the finding" | `diagnosing-bugs` | 0.48.0 | **routed** — 0.48.0 §3 |
| Demand drives legwork — *"every modified model accounted for"* forces work that *"produce a list"* does not | `writing-for-agents` | 0.41.0 | **routed** — 0.41.0 §3 |
| Negation is a failure mode; prompt the positive | `writing-for-agents` | 0.41.0 (emission) + W5 (CLAUDE.md) | **routed** — 0.41.0 §4a; CLAUDE.md half parked as W5 |
| Leading words recruit pretraining; an invented word pays in definition tokens | `writing-for-agents` | naming, `ground` | settled — validates the name |
| Two axes reported separately, never merged or reranked, because one masks the other | `code-review` | 0.50.0 | **routed** — 0.50.0 §4.1 |
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
- **Claim enumeration.** The weakest step: intuition undercounted 11 against 17. Whether this is a
  parsing problem, a definition problem, or irreducibly a reading problem is not yet clear.
- **What `UNVERIFIABLE` costs.** Zero instances in 27 grounded claims, so the verdict is designed and
  untested. A spec resting on a field report would produce one; none has been grounded yet.
