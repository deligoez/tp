# tp v1.41.0 — Forced commitment in the brief

> **This file is decisions.** The three sentences below are not proposals: each was written by hand
> into a role brief in this repository, and the round that received it did something the round before
> had not. What this release does is stop that depending on an operator remembering to type it.

## 1. Overview

`CLAUDE.md` records three sentences, six instances across two repositories, three of them this repo's
own v0.36.0 audit. In every case **the reading was already available to the role** — what the sentence
bought was the obligation to conclude something from it.

| the sentence | what the round did that the round before had not |
|---|---|
| the inputs behind this repair were chosen by whoever wrote it, which is the condition under which this cycle has been wrong *N* times — construct one that set does not reach | round 11 built the input the repair's own six missed and found both gate guards green; round 12 returned **two `error`-severity findings** |
| do you still hold this earlier judgement — either answer is fine | round 13 **withdrew its own** round-12 conclusion that two claims admitted no falsifying input, and built the reconstruction it had said did not exist |
| state what your prose knows that your row cannot carry, **and say what should be done about it** | a role narrowed its own note from four files to three call sites and retracted its earlier framing; without the second clause the same note sat unactioned for a round |

An unattended run gets none of them. `tp run` spawns a role with the prompt `tp audit` emits, and
that prompt's framing block (`internal/cli/prompt_framing.go:54-91`) states the output path, the reset
discipline, the loop budget and the file-reading situation — and nothing that forces a commitment.

**This release puts the three into the emitted prompt**, each supplied with a fact tp already holds.
It is text and one filter change. There is no new field, no gate and no exit code.

## 2. A prior PASS row with a note comes back

`loadAuditPriorRound` reads the previous round's rows and drops every PASS row at
`internal/cli/audit.go:571` — a bare `if status == "PASS" { continue }`. `renderPriorRoundSection`
then shows a role its own non-PASS rows under *"context to re-check, not a verdict to repeat"*
(`internal/cli/audit_roles.go:193-210`).

**A row this role has just changed its mind about is returned to it as well**, in the same section,
under the second sentence: *do you still hold this — either answer is fine.* The set is this role's
items that were **non-`PASS` last round and are `PASS` in this one**.

### 2.1 Why not "every prior PASS row that carries a note"

That was this release's first design, and it is **refuted by its own corpus.** It rested on the claim
that most PASS rows carry an empty note. Measured over every recorded audit row:

| | |
|---|---|
| audit rows | 23,441 |
| `PASS` rows | 22,592 |
| `PASS` rows carrying a note | **21,924 — 97.0%** |

The claim is not merely wrong, it is inverted, and the notes say why — the audit prompt asks each role
to record the range it inspected, so a PASS note is the ordinary evidentiary record:

> `git via exec.Command arg-list (no shell); flock via WithFileLock; malformed files handled with …`

Returning those would carry ~97% of the checklist into the next prompt — on v0.37.0's round 7, about
103 rows — which is exactly the bloat that design claimed to avoid.

**The status flip is the mechanical half of the same signal, and it is small.** Measured over 80
consecutive round pairs: **209 flips in total, median 2 per round**, largest 16 of 102 rows (15.7%).

**It reaches the measured instance directly.** A role withdrawing a judgement is a role whose verdict
on an item moved, and the flip *is* that event — recorded, with no prose to interpret.

**What it does not reach is stated rather than glossed.** A judgement living in a `PASS` note that was
never a finding — the 55/55-PASS case where a role's prose named a real defect three rounds running —
stays invisible here, because tp cannot separate it from the 21,924 notes that are simply evidence.
That half is reported, not returned, by the release that corrects a round's ledger.

**The existing rows keep their existing framing.** A non-PASS row is *context to re-check*; a
PASS-with-note row is *a judgement to re-affirm or withdraw*. They are different asks and the section
labels them differently — collapsing them into one instruction is how "re-check this" becomes "repeat
this", which the current wording already guards against.

**`ChangedSince` extends to the new rows unchanged.** `filesChangedSince` (`internal/cli/audit.go:565`) already
tells a role whether its evidence file moved since the prior round, which is precisely the fact that
makes "do you still hold this" answerable rather than rhetorical.

## 3. The framing carries the provenance-and-count sentence

The framing block gains the first sentence, with the count derived from the recorded rounds:

> *N of the M rounds already recorded for this spec produced at least one non-PASS row. The inputs
> behind any repair you are reading were chosen by whoever wrote it — construct one that set does not
> reach.*

**The count is load-bearing and this is why it is emitted rather than described.** `CLAUDE.md` states
the measurement plainly: *"chosen by the author"* reads as a caveat, *"the condition under which this
cycle has been wrong nine times"* reads as a standing defect, and only the second changed what the
role did. A sentence tp emits without a number is the version that was measured not to work.

**The mechanism has a name outside this repository, and it sharpens what the sentence has to do.** A
completion criterion carries **demand**: how much it requires. *"Every modified file accounted for"*
forces work that *"produce a change list"* does not, and the digging that demand provokes is latent in
the wording rather than written as its own step. The count is the demand; without it the sentence
states a caveat and asks for nothing, which is why it measured as a no-op — an instruction the role
already believes it follows pays load and changes no behaviour.

**tp emits the count it can derive, and names it exactly.** *Rounds that produced at least one
non-PASS row* is `len(rounds)` and each round's recorded `findings`, both already in the state index —
no new read, no new field. It is not the same quantity as *times this cycle has been wrong*, and the
sentence must not claim to be: an inflated count is the failure mode this whole release is about.

**Round 1 emits no count and no sentence.** With no recorded rounds there is no repair to have chosen
inputs for, and *"0 of 0 rounds"* is a sentence that teaches a role to discount the next one.

## 4. The framing asks what should be done

The framing block gains the third sentence, verbatim in force:

> *State what your prose knows that your row cannot carry — and say what should be done about it.*

**The second clause is the whole finding.** The measured instance is that the same note, without it,
sat unactioned for a round. A role told to explain produces an explanation; a role told to recommend
produces something a repair can be built from.

**It is emitted every round, including round 1**, because unlike §3 it rests on nothing the state
index has to supply.

## 4a. The three sentences are phrased positively, and the emission is audited for the opposite

**Steering by prohibition makes the forbidden behaviour more available, not less.** A ban names the
thing, and naming it in context is most of what makes a model reach for it; the negation is a weak
modifier the strongly-activated concept overruns. The target behaviour stated plainly never speaks
the banned one at all.

**Measured on what tp emits today: 14% of the review prompt's sentences carry a prohibition** — eight
of fifty-seven, including *"Do NOT check implementation code or report 'not implemented' findings"*,
which says positively as *"judge the spec's text; conformance to code is the audit's question."*

**This release fixes only its own three sentences and the prompt lines it touches.** A prohibition
survives where it is a hard guardrail with no positive phrasing — the isolation clause's *"do not edit
any file in the working tree"* is one — and even there it is paired with the target. Sweeping the rest
of the emission is a separate piece of work, because a rewrite of prompt text that nothing measures is
how a release ships prose churn.

## 5. Non-Goals

1. **The review phase gets none of this.** `tp review` has no prior-round section at all — the grep
   returns nothing — so §2 has nothing to extend there and §3's count would sit beside a role that
   cannot see what it said last round. Giving review a prior-round section is its own release, and
   guessing its shape from the audit side is how a mechanism ships before its subject exists.
2. **No gate, no counter, no convergence effect.** Nothing here changes `clean`, a streak, or an exit
   code. These are sentences in a prompt.
3. **No new workflow field, and no way to switch them off.** A brief whose forcing clauses are
   optional is the brief nobody types, which is the defect.
4. **The three sentences are not made configurable or templated.** Their exact wording is what was
   measured; a template invites a weaker paraphrase, and §3 records that the weaker paraphrase was
   measured not to work.
5. **No claim that emitting them reproduces the measured effects.** Six instances is what exists, all
   hand-written, all in this repository's own cycles. The release ships the sentences because they are
   free and the evidence points one way — not because six hand-run instances establish a rate.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | an item this role marked `FAIL` last round and `PASS` this round reaches its own next prompt | keep `if status == "PASS" { continue }` — the shipped behaviour, under which the round-13 withdrawal could not have been prompted |
| 2 | §2.1 | an item that was `PASS` in **both** rounds does **not** reach the prompt, even carrying a note | return every noted PASS row — measured at 97.0% of PASS rows, ~103 on v0.37.0's round 7, the bloat this design was refuted for |
| 3 | §2 *framing* | the returned flips are labelled as a judgement to re-affirm or withdraw, distinctly from the non-PASS rows' re-check labelling | render both under one heading, which turns "do you still hold this" into "re-check this finding" |
| 4 | §2 *scope* | a flip reaches **only** the role whose verdict moved | key the flip on `item_id` alone, so one role's change of mind is put to another role that never held it |
| 5 | §3 | at round 4 with 3 prior rounds of which 2 recorded findings, the prompt says 2 of 3 | emit `len(rounds)` for both, which reports every cycle as having been wrong every round |
| 6 | §3 *round 1* | a round-1 prompt carries neither the count nor the sentence | emit "0 of 0", the phrasing that teaches a role to discount the clause |
| 7 | §4 | every emitted role prompt, round 1 included, carries the second clause — asserted over every role in the panel, not one | append it to one role's builder, which passes a single-role test and ships three roles without it |

**Row 7 is quantified over the panel deliberately.** A test that checks one role's prompt is satisfied
by a fix applied to one branch, and this repository has already shipped a guard whose claim about a
set was verified against the one member its author chose.
