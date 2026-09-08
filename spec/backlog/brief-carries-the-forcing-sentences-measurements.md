# brief-carries-the-forcing-sentences — measurements

Supplemental material for `brief-carries-the-forcing-sentences.md`; the spec stands without it.

Section numbers inside the blocks below are the draft's own: §1, §3 and §4 still exist in the spec
under those numbers; §2 and §2.1 are the deferred block carried here.

## The controlled measurement: arms and derivation

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

## The flip rows, deferred: no recorded round measures what returning them would have done

This was the draft's §2, the half of the release that returned a role's own non-PASS→PASS flips under
the second sentence, *do you still hold this — either answer is fine*. It is deferred, not dropped:
the design is complete, the corpus below says what it does and does not reach, and its own last
paragraph concedes that no recorded round measures the effect of returning the rows. It comes back
when a round supplies that measurement.

### 2. A prior PASS row with a note comes back

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

**`ChangedSince` extends to the new rows unchanged.** `filesChangedSince` (`internal/cli/audit.go:597`) already
tells a role whether its evidence file moved since the prior round, which is precisely the fact that
makes "do you still hold this" answerable rather than rhetorical.

### Tests that went with the flip rows

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | an item this role marked `FAIL` last round and `PASS` this round reaches its own next prompt | keep `if status == "PASS" { continue }` — the shipped behaviour, under which the round-13 withdrawal could not have been prompted |
| 2 | §2.1 | an item that was `PASS` in **both** rounds does **not** reach the prompt, even carrying a note | return every noted PASS row — measured at 97.0% of PASS rows, ~103 on v0.37.0's round 7, the bloat this design was refuted for |
| 3 | §2 *framing* | the returned flips are labelled as a judgement to re-affirm or withdraw, distinctly from the non-PASS rows' re-check labelling | render both under one heading, which turns "do you still hold this" into "re-check this finding" |
| 4 | §2 *scope* | a flip reaches **only** the role whose verdict moved | key the flip on `item_id` alone, so one role's change of mind is put to another role that never held it |

## Prohibitions in the emitted prompt

**Steering by prohibition makes the forbidden behaviour more available, not less.** A ban names the
thing, and naming it in context is most of what makes a model reach for it; the negation is a weak
modifier the strongly-activated concept overruns. The target behaviour stated plainly never speaks
the banned one at all.

**Measured on what tp emits today: 14% of the review prompt's sentences carry a prohibition** — eight
of fifty-seven, including *"Do NOT check implementation code or report 'not implemented' findings"*,
which says positively as *"judge the spec's text; conformance to code is the audit's question."*

**This release fixes only its own sentences and the prompt lines it touches.** A prohibition
survives where it is a hard guardrail with no positive phrasing — the isolation clause's *"do not edit
any file in the working tree"* is one — and even there it is paired with the target. Sweeping the rest
of the emission is a separate piece of work, because a rewrite of prompt text that nothing measures is
how a release ships prose churn.

## Why the review phase is out: the regression prompt

The draft's first Non-Goal, in full:

**The review phase gets none of this — but not for the reason this non-goal used to give.** It said
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

## The `evidence` field is write-only at the injection sites

`spec/1.1.0.md` demands `evidence` on a finding at record time and writes it. At `HEAD` the three
sites that put a prior finding back in front of a role — the resolved-findings block in
`internal/cli/review.go`, the verify prompt in `internal/cli/review_verify.go` and the regression
prompt in `internal/cli/review_regression.go` — each read `f.Resolved.Evidence`, the *resolution*
evidence, and `reviewFinding` (`internal/cli/review.go`) has no `evidence` field of its own. So the
record-time `evidence` is write-only there: nothing carries it forward into a later round's prompt.
Derive with `grep -rn '\.Evidence' internal/cli --include='*.go' | grep -v _test.go`. Reading it
back is the natural follow-up to this release and is a Non-Goal of it.
