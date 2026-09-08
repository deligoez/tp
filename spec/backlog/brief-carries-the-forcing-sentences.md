# tp — Forced commitment in the brief

Class: loop

> **This file is decisions.** The sentences below are not proposals: each was written by hand into a
> role brief in this repository, and the round that received it did something the round before had
> not. What this release does is stop that depending on an operator remembering to type it.

## 1. Overview

`CLAUDE.md` records three sentences, six instances across two repositories, three of them this repo's
own v0.36.0 audit. In every case **the reading was already available to the role** — what the sentence
bought was the obligation to conclude something from it.

| the sentence | what the round did that the round before had not |
|---|---|
| the inputs behind this repair were chosen by whoever wrote it, which is the condition under which this cycle has been wrong *N* times — construct one that set does not reach | round 11 built the input the repair's own six missed and found both gate guards green; round 12 returned **two `error`-severity findings** |
| do you still hold this earlier judgement — either answer is fine | round 13 **withdrew its own** round-12 conclusion that two claims admitted no falsifying input, and built the reconstruction it had said did not exist |
| state what your prose knows that your row cannot carry, **and say what should be done about it** | **a peer cycle's** role narrowed its own note from four files to three call sites and retracted its earlier framing; without the second clause the same note sat unactioned for a round |

**The third instance belongs to a peer repository's cycle, not to this one.** `CLAUDE.md` says so —
*"a peer cycle's role"* — and that is why the count above reads *two repositories*. It is stated here in
prose as well as inside a table cell, because those two words were dropped from the cell once already
and §5 inherited the error.

**This release emits the first and the third.** The second is the framing for a set of rows — a
role's own non-`PASS`→`PASS` flips — that no recorded round measures the return of, and it is deferred
with them (§5).

### 1.1 The one controlled measurement, and it was not in the table

The three rows above are hand-run and uncontrolled. This repository also holds a **controlled**
measurement of the same intervention: v0.37.0's own review gave the three clauses to two of the four
reviewer roles, withheld them from the other two, and **swapped the arms** between rounds. The briefed
arm filed **15** and then **23** findings against a control of **35** in both rounds, and the effect
followed the arm rather than the role; the arm table and the command that recovers the assignment
from the raw per-role files are in the sidecar under "The controlled measurement: arms and
derivation".

**Two limits, stated rather than glossed.** The trial varied the **whole three-clause brief against no
brief**, so it cannot separate the count from the clause, or any clause from the others — which is why
§3 no longer claims the numberless form was measured. And what it measured is *what a role files*, not
how many rounds a cycle takes; the second has never been measured here and is not claimed anywhere in
this file.

An unattended run gets none of them. `tp run` spawns a role with the prompt `tp audit` emits, and
that prompt's framing block (`renderFraming`, `internal/cli/prompt_framing.go`) states the output
path, the reset discipline, the loop budget and the file-reading situation — and nothing that forces a
commitment.

**This release puts the two into the emitted prompt**, each supplied with a fact tp already holds.
It is text. There is no new field, no gate and no exit code.

## 2. The framing carries the provenance-and-count sentence

The framing block gains the first sentence, with the count derived from the recorded rounds:

> *N of the M rounds already recorded for this spec produced at least one non-PASS row. The inputs
> behind any repair you are reading were chosen by whoever wrote it — construct one that set does not
> reach.*

**The count is load-bearing and this is why it is emitted rather than described.** `CLAUDE.md` states
the instance plainly: *"chosen by the author"* reads as a caveat, *"the condition under which this
cycle has been wrong nine times"* reads as a standing defect, and only the second changed what the
role did. A sentence tp emits without a number is the version `CLAUDE.md` records as changing nothing.

**The mechanism has a name outside this repository, and it sharpens what the sentence has to do.** A
completion criterion carries **demand**: how much it requires. *"Every modified file accounted for"*
forces work that *"produce a change list"* does not, and the digging that demand provokes is latent in
the wording rather than written as its own step. The count is the demand; without it the sentence
states a caveat and asks for nothing — an instruction the role already believes it follows pays load
and changes no behaviour.

**tp emits the count it can derive, and names it exactly.** *Rounds that produced at least one
non-PASS row* is `len(rounds)` and each round's recorded `findings`, both already in the state index —
no new read, no new field. It is not the same quantity as *times this cycle has been wrong*, and the
sentence must not claim to be: an inflated count is the failure mode this whole release is about.

**Round 1 emits no count and no sentence.** With no recorded rounds there is no repair to have chosen
inputs for, and *"0 of 0 rounds"* is a sentence that teaches a role to discount the next one.

## 3. The framing asks what should be done

The framing block gains the third sentence, verbatim in force:

> *State what your prose knows that your row cannot carry — and say what should be done about it.*

**The second clause is the whole finding.** The measured instance is that the same note, without it,
sat unactioned for a round. A role told to explain produces an explanation; a role told to recommend
produces something a repair can be built from.

**It is emitted every round, including round 1**, because unlike §2 it rests on nothing the state
index has to supply.

## 4. The sentences are phrased positively, and the emission is audited for the opposite

**Both sentences state the target behaviour and name no banned one, and the release fixes only its own
sentences and the prompt lines it touches.** Steering by prohibition makes the forbidden behaviour
more available rather than less; a prohibition survives only where it is a hard guardrail with no
positive phrasing, and even there it is paired with the target. The share of the shipped review prompt
that steers by prohibition, and why the rest of the emission is not swept here, are in the sidecar
under "Prohibitions in the emitted prompt".

## 5. Non-Goals

1. **The review phase gets none of this.** `tp review` already appends a `regression` prompt reading
   the cycle's fixed findings, so review sees what a prior round settled; the audit framing is where
   these sentences go, and carrying them into the review emission is its own release. The measurement
   behind this non-goal's earlier wording is in the sidecar under "Why the review phase is out: the
   regression prompt".
2. **The flip rows are deferred, not dropped.** Returning a role's own non-`PASS`→`PASS` flips under
   *do you still hold this — either answer is fine* was this release's second half. No recorded round
   measures what returning them would have done, so the design, its corpus and its tests wait in the
   sidecar under "The flip rows, deferred: no recorded round measures what returning them would have
   done" until a round supplies that measurement.
3. **No read side for the `evidence` field.** `spec/1.1.0.md` demands `evidence` at record and
   writes it; at the three sites that put a prior finding back in front of a role the field is
   write-only, because `reviewFinding` has no `evidence` field. Reading it back into the prompt is this
   release's natural follow-up and not part of it; the sidecar's "The `evidence` field is write-only at
   the injection sites" names the sites.
4. **No gate, no counter, no convergence effect.** Nothing here changes `clean`, a streak, or an exit
   code. These are sentences in a prompt.
5. **No new workflow field, and no way to switch them off.** A brief whose forcing clauses are
   optional is the brief nobody types, which is the defect.
6. **The sentences are not made configurable or templated.** Their exact wording is what was tried; a
   template invites a weaker paraphrase, and §2 records that the numberless paraphrase changed nothing
   where it was tried.
7. **No claim that emitting them reproduces the measured effects.** Six instances is what exists, all
   hand-written, one of them in a peer repository's cycle. The release ships the sentences because
   they are free and the evidence points one way — not because six hand-run instances establish a
   rate.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | at round 4 with 3 prior rounds of which 2 recorded findings, the prompt says 2 of 3 | emit `len(rounds)` for both, which reports every cycle as having been wrong every round |
| 2 | §2 *round 1* | a round-1 prompt carries neither the count nor the sentence | emit "0 of 0", the phrasing that teaches a role to discount the clause |
| 3 | §3 | every emitted role prompt, round 1 included, carries the second clause — asserted over every role in the panel, not one | append it to one role's builder, which passes a single-role test and ships three roles without it |
| 4 | §4 | each emitted sentence equals the constant the release adds, and neither constant carries a prohibition — asserted as equality over the bounded constant, never as a search over the prompt | phrase either sentence as a ban, which names the behaviour it forbids |

**Row 3 is quantified over the panel deliberately.** A test that checks one role's prompt is satisfied
by a fix applied to one branch, and this repository has already shipped a guard whose claim about a
set was verified against the one member its author chose.
