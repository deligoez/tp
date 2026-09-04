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
| state what your prose knows that your row cannot carry, **and say what should be done about it** | **a peer cycle's** role narrowed its own note from four files to three call sites and retracted its earlier framing; without the second clause the same note sat unactioned for a round |

**The third instance belongs to a peer repository's cycle, not to this one.** `CLAUDE.md:120` says so —
*"a peer cycle's role"* — and that is why the count above reads *two repositories*. It is stated here in
prose as well as inside a table cell, because those two words were dropped from the cell once already
and §5 inherited the error.

### 1.1 The one controlled measurement, and it was not in the table

The three rows above are hand-run and uncontrolled. This repository also holds a **controlled**
measurement of the same intervention: v0.37.0's own review gave the three clauses to two of the four
reviewer roles, withheld them from the other two, and **swapped the arms** between rounds.

| round | briefed arm | findings | control arm | findings |
|---|---|---|---|---|
| 11 | `architect` + `ax-economist` | **15** | `implementer` + `tester` | **35** |
| 12 | `implementer` + `tester` | **23** | `architect` + `ax-economist` | **35** |

`regression` (9, then 12) sits in neither arm. The effect follows the **arm, not the role**: each pair
filed far less briefed than the same pair filed unbriefed one round away, and the control filed 35
both times.

**The arm assignment is recorded nowhere.** `CLAUDE.md` and `spec/candidates.md` both give the three
totals and no arms. It is recoverable only from the **raw per-role files**, because `--merge` dedups on
`(location, class)` and the merged round totals are 54 and 69, which reproduce nothing:

```
python3 -c 'import json,glob,re,collections
c=collections.defaultdict(dict)
for f in glob.glob("spec/.tp-review/0.37.0/review-r[0-9]*-*.ndjson"):
    m=re.search(r"review-r(\d+)-(.+)\.ndjson",f); c[int(m.group(1))][m.group(2)]=sum(1 for l in open(f) if l.strip())
print([(n,dict(sorted(c[n].items()))) for n in (11,12)])'
```

**Two limits, stated rather than glossed.** The trial varied the **whole three-clause brief against no
brief**, so it cannot separate the count from the clause, or any clause from the others — which is why
§3 no longer claims the numberless form was measured. And what it measured is *what a role files*, not
how many rounds a cycle takes; the second has never been measured here and is not claimed anywhere in
this file.

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

**A row this role has just resolved is returned to it as well**, in the same section, under the second
sentence: *do you still hold this — either answer is fine.* The set is this role's items that were
**non-`PASS` last round and are `PASS` in this one** — the rows the shipped filter drops at the exact
moment they become a resolution the role might have got wrong. §2.1 states what that set does and does
not reach.

### 2.1 Why not "every prior PASS row that carries a note"

That was this release's first design, and it is **refuted by its own corpus.** It rested on the claim
that most PASS rows carry an empty note. Measured over the recorded audit **round** files —
`spec/.tp-review/*/audit-round-*.ndjson`, which is every recorded audit row exactly once, because the
per-role `audit-r<N>-<role>.ndjson` files are the pre-merge inputs to those same rounds and adding them
counts their rows twice — at commit `0f7f164f`:

| | |
|---|---|
| audit rows | 12,970 |
| `PASS` rows | 12,447 |
| `PASS` rows carrying a non-empty `notes` | **12,113 — 97.3%** |

```
python3 -c 'import json,glob
rs=[json.loads(l) for f in glob.glob("spec/.tp-review/*/audit-round-*.ndjson") for l in open(f) if l.strip()]
p=[r for r in rs if r.get("status")=="PASS"]
print(len(rs),len(p),sum(1 for r in p if (r.get("notes") or "").strip()))'
```

**The glob is the measurement.** An earlier draft of this table quoted 23,441 / 22,592 / 21,924, which
is this glob summed with its own superset `audit-*.ndjson` — two overlapping globs added together, so
every merged-round row was counted twice. Only the percentage survived, because a ratio is invariant
under a uniform double count, which is exactly why the figure looked right.

The claim is not merely wrong, it is inverted, and the notes say why — the audit prompt asks each role
to record the range it inspected, so a PASS note is the ordinary evidentiary record:

> `git via exec.Command arg-list (no shell); flock via WithFileLock; malformed files handled with …`

Returning those would carry almost the whole checklist into the next prompt — on v0.37.0's round 7,
103 of its 106 rows are `PASS` and all 103 carry a non-empty `notes`
(`spec/.tp-review/0.37.0/audit-round-7.ndjson`) — which is exactly the bloat that design claimed to
avoid. That figure is derived from round 7 itself, not from the corpus table above.

**The status flip is the mechanical half of the same signal, and it is small.** A flip is keyed on
`(role, item_id)` — the key `loadAuditPriorRound` already buckets by — and counted between consecutive
`audit-round-*.ndjson` files within one spec directory. At `0f7f164f` that is **91 pairs across 17 spec
directories: 256 non-`PASS`→`PASS` flips, median 2 per pair**, largest 16 of 102 rows (15.7%, v0.36.0
r1→r2). The key is stated because it changes the answer: `item_id` alone merges roles that never held
each other's verdicts.

```
python3 -c 'import json,glob,re,collections,statistics
d=collections.defaultdict(list)
for f in glob.glob("spec/.tp-review/*/audit-round-*.ndjson"):
    d[f.split("/")[2]].append((int(re.search(r"-(\d+)\.ndjson",f).group(1)),f))
R=lambda f:[json.loads(l) for l in open(f) if l.strip()]
n=[]
for k in d:
    fs=sorted(d[k])
    for (_,a),(_,b) in zip(fs,fs[1:]):
        A={(r.get("role"),r.get("item_id")):r.get("status") for r in R(a)}
        B={(r.get("role"),r.get("item_id")):r.get("status") for r in R(b)}
        n.append(sum(1 for k2,v in B.items() if k2 in A and A[k2]!="PASS" and v=="PASS"))
print(len(n),sum(n),statistics.median(n),max(n))'
```

**It does not reach §1's flagship instance, and that instance is evidence for the shipped filter rather
than for this addition.** Measured on v0.36.0, same key: r12→r13 has **zero** non-`PASS`→`PASS` flips.
Round 13's withdrawal is two `maintainability-conventions` items moving `PARTIAL`(`error`) →
`FAIL`(`error`); the only non-`PASS`→`PASS` flip in that cycle's late rounds is r10→r11, a different
round. And both of those rows were non-`PASS` in round 12, so `internal/cli/audit.go:571` **returned**
them to that role in round 13 — the shipped filter is what put the judgement back in front of the role
that then withdrew it.

```
python3 -c 'import json
L=lambda n:[json.loads(l) for l in open(f"spec/.tp-review/0.36.0/audit-round-{n}.ndjson") if l.strip()]
A={(r["role"],r["item_id"]):r["status"] for r in L(12)}
B={(r["role"],r["item_id"]):r["status"] for r in L(13)}
print([(k[1],A.get(k),v) for k,v in B.items() if A.get(k)!=v])'
```

**So the two halves of §2 are motivated separately, and only one of them has a measured instance.** What
§1's instance establishes is that the *second sentence* was what the shipped return set lacked — the
rows were already in front of the role, the obligation to conclude was not — and that is §3 and §4's
business, which is the larger half of this release. What the flip set adds is the case the shipped
filter drops **entirely**: an item this role marked non-`PASS` and has since marked `PASS` disappears
from its own prompt. No recorded round measures what returning it would have done, and this release
does not claim one does.

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

1. **The review phase gets none of this — but not for the reason this non-goal used to give.** It said
   *"`tp review` has no prior-round section at all — the grep returns nothing"*. Ground round 1
   measured otherwise, and the measurement reproduces: `internal/cli/review.go` auto-appends a
   `regression` prompt as a further entry to a **default** review round whenever the round is at least
   the second and the cycle has either a non-empty diff or one recorded fixed finding, and
   `buildRegressionPrompt` (`internal/cli/review_regression.go`) writes a `## Previously fixed
   findings` heading carrying each finding's resolution evidence. Derive it with
   `git grep -n 'buildRegressionPrompt' -- 'internal/cli/*.go'`, which returns the call site, the
   definition and one other caller. The grep returning nothing is true of a *differently named*
   heading and is a different fact.

   So review does see what a prior round settled. What it does not have is the audit side's
   **per-role** section — the regression prompt is one extra role reading the cycle's fixed findings,
   not each role reading its own rows — and §2's mechanism is defined over a role's own rows. That is
   the real reason this release stops at the audit side, and extending it is its own release rather
   than a guess made from here.
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
