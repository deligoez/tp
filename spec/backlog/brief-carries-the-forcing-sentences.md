# tp — Forced commitment in the brief

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `brief-carries-the-forcing-sentences-measurements.md` beside it, and this file
stands without them.

Class: **loop** — it changes what the audit roles are told, so the rounds that review it are shaped by
the text it changes.

**Sequencing.** §2 writes into the audit role prompt's *Prior Round* block, which
`a-finding-can-leave-an-audit-round` also changes — it makes each prior-round row carry its
disposition and evidence. This spec ships after that one, so it writes into the block that release
leaves rather than into today's.

## 1. Overview

The sentences below are not proposals: each was written by hand into a role brief, and the round that
received it did something the round before had not. `CLAUDE.md` records them. In every case **the
reading was already available to the role** — what the sentence bought was the obligation to conclude
something from it. This release stops that depending on an operator remembering to type it.

| the sentence | what the round did that the round before had not |
|---|---|
| the inputs behind this repair were chosen by whoever wrote it, which is the condition under which this cycle has been wrong *N* times — construct one that set does not reach | round 11 built the input the repair's own six missed and found both gate guards green; round 12 returned **two `error`-severity findings** |
| do you still hold this earlier judgement — either answer is fine | round 13 **withdrew its own** round-12 conclusion that two claims admitted no falsifying input, and built the reconstruction it had said did not exist |
| state what your prose knows that your row cannot carry, **and say what should be done about it** | **a peer cycle's** role narrowed its own note from four files to three call sites and retracted its earlier framing; without the second clause the same note sat unactioned for a round |

**This release emits the first and the third.** The second is the framing for a set of rows — a
role's own non-`PASS`→`PASS` flips — that no recorded round measures the return of, and it is deferred
with them (§5).

### 1.1 The one controlled measurement

The three rows above are hand-run and uncontrolled. v0.37.0's own review also ran a **controlled**
trial of the same intervention: it gave the three clauses to two of the four reviewer roles, withheld
them from the other two, and swapped the arms between rounds. The briefed arm filed markedly fewer
findings than the control in both rounds, and the effect followed the arm rather than the role; the
arm table and the command that recovers the assignment are in the sidecar under *The controlled
measurement: arms and derivation*.

**Two limits.** The trial varied the whole three-clause brief against no brief, so it cannot separate
the count from the clause, or one clause from another. And what it measured is what a role files, not
how many rounds a cycle takes; the second has never been measured here and is not claimed.

An unattended run gets none of these sentences: `tp run` spawns a role with the prompt `tp audit`
emits, which states the output path, the reset discipline, the loop budget and the file-reading
situation, and nothing that forces a commitment. **This release puts two of them into the emitted
prompt**, each supplied with a fact tp already holds. It is text: no new field, no gate, no exit code.

## 2. The Prior Round block carries the provenance-and-count sentence

The *Prior Round* block — present in a role's prompt when that role has prior non-`PASS` rows to
re-check — opens with the first sentence, its count derived from the recorded rounds:

> *N of the M rounds already recorded for this spec produced at least one non-PASS row. The inputs
> behind any repair you are reading were chosen by whoever wrote it — construct one that set does not
> reach.*

It goes there because that block is where a role reads what the last round found and what was done
about it — once `a-finding-can-leave-an-audit-round` ships, each row's disposition and its evidence —
which is exactly the repair the sentence asks the role to test.

**The count is load-bearing, which is why it is emitted rather than described.** `CLAUDE.md` states
the instance plainly: *"chosen by the author"* reads as a caveat, *"the condition under which this
cycle has been wrong nine times"* reads as a standing defect, and only the second changed what the
role did. A sentence tp emits without a number is the version recorded as changing nothing.

**The count stays although it is nearly constant.** In the recorded corpus almost every audit round
produced a non-`PASS` row, so *N* equals *M* in nearly every prompt this sentence would reach (sidecar,
*How constant N of M is*). That does not make it redundant: what changed behaviour was a number
standing in the sentence, and *M of M* reads as *every round so far found something*, which is the
standing defect in its strongest form. It stays *N of M* rather than *M* because *N* is the quantity tp
can state without inflating — a round with no non-`PASS` row exists — and it is not the same quantity
as *times this cycle has been wrong*, which the sentence must not claim to be.

**Round 1 emits no count and no sentence.** With no recorded rounds there is no repair to have chosen
inputs for, and *"0 of 0 rounds"* is a sentence that teaches a role to discount the next one. The
block's own absence on round 1 already delivers this.

## 3. The framing asks what should be done

The framing block every role prompt carries gains the third sentence, verbatim in force:

> *State what your prose knows that your row cannot carry — and say what should be done about it.*

**The second clause is the whole finding.** The measured instance is that the same note, without it,
sat unactioned for a round. A role told to explain produces an explanation; a role told to recommend
produces something a repair can be built from.

**It is emitted every round, including round 1**, because unlike §2 it rests on nothing the recorded
rounds have to supply.

## 4. The sentences are phrased positively, and the lines they touch are too

**Both sentences state the target behaviour and name no banned one, and the release fixes only its own
sentences and the prompt lines it touches** — which includes the *Prior Round* block's own opening
instruction, today a prohibition. Steering by prohibition makes the forbidden behaviour more available
rather than less; a prohibition survives only where it is a hard guardrail with no positive phrasing,
and even there it is paired with the target. The share of the shipped prompt that steers by
prohibition, and why the rest is not swept here, are in the sidecar under *Prohibitions in the emitted
prompt*.

## 5. Non-Goals

1. **The review phase gets none of this.** `tp review` already appends a `regression` prompt reading
   the cycle's fixed findings, so review sees what a prior round settled; the audit prompt is where
   these sentences go, and carrying them into the review emission is its own release. The sidecar's
   *Why the review phase is out: the regression prompt* carries the measurement.
2. **The flip rows are deferred, not dropped.** Returning a role's own non-`PASS`→`PASS` flips under
   *do you still hold this — either answer is fine* was this release's second half. No recorded round
   measures what returning them would have done, so the design, its corpus and its tests wait in the
   sidecar under *The flip rows, deferred* until a round supplies that measurement.
3. **No read side for the `evidence` field.** `spec/1.1.0.md` demands `evidence` at record and writes
   it; at the three sites that put a prior finding back in front of a review role the field is
   write-only (sidecar, *The `evidence` field is write-only at the injection sites*). Reading it back
   there is a follow-up and not part of this release; the audit side's prior-round evidence is
   `a-finding-can-leave-an-audit-round`'s.
4. **No gate, no counter, no convergence effect.** Nothing here changes `clean`, a streak, or an exit
   code. These are sentences in a prompt.
5. **No new workflow field, and no way to switch them off.** A brief whose forcing clauses are
   optional is the brief nobody types, which is the defect.
6. **The sentences are not made configurable or templated.** Their exact wording is what was tried; a
   template invites a weaker paraphrase, and §2 records that the numberless paraphrase changed nothing
   where it was tried.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. None of the sentences exists at `HEAD`, so each row's pair of counts is the implementing
task's acceptance, per Step 0.5.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | at round 4 with 3 prior rounds of which 2 recorded non-`PASS` rows, the *Prior Round* block says 2 of 3. The fixture must hold a clean round: on the corpus's usual *N* = *M* the mutant and the fix print the same number | emit the round count for both, which reports every cycle as having been wrong every round |
| 2 | §2 *round 1* | a round-1 prompt carries neither the count nor the sentence | emit "0 of 0", the phrasing that teaches a role to discount the clause |
| 3 | §3 | every emitted role prompt, round 1 included, carries the second clause — asserted over every role in the panel, not one | append it to one role's builder, which passes a single-role test and ships three roles without it |
| 4 | §4 | each emitted sentence equals the constant the release adds, and neither constant carries a prohibition — asserted as equality over the bounded constant, never as a search over the prompt | phrase either sentence as a ban, which names the behaviour it forbids |

**Row 3 is quantified over the panel deliberately.** A test that checks one role's prompt is satisfied
by a fix applied to one branch, and this repository has already shipped a guard whose claim about a
set was verified against the one member its author chose.
