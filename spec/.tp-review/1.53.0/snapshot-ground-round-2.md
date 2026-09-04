# tp v1.53.0 — The spec-hash reset

> **This file is decisions.** It opens with the one decision that must be taken before implementation
> starts, because five review rounds in an earlier cycle established that a review round cannot take
> it. Everything after §2 is settled.

## 1. Overview

`consecutive_clean` is **a claim about a spec**. When the spec changes, rounds recorded against the
old text no longer support the claim.

**The review phase already acts on this** — a `fixed` resolution forces a further round and
`tp resume` raises `spec-stale`. **The audit phase checks the trailing edge only.** `engine.Converged`
already carries a hash gate — its second conjunct is `!StateStale(rounds, currentHash)`, which compares
the *last* recorded round's hash to the spec on disk. So movement after the last round unconverges;
movement *between* rounds is invisible, and the streak survives any amount of it.

Measured in a fixture built outside the repo — `tp init` on a two-section spec plus one task, record an
all-PASS audit round, append a section, record a second all-PASS round: `consecutive_clean: 2`,
`converged: true`, `tp audit <spec> --status --check` exit 0. Appending once more, with no further round
recorded, gives `consecutive_clean: 2`, `stale: true`, `converged: false`.

**It needs the emit-time hash.** A reset keyed on a hash that was itself re-read at record time
resets on the wrong event; the release that pins a round's hash to its own snapshot is what makes the
comparison mean what it says.

## 2. The decision, which must be taken first

Either:

**(a) Store a per-round vintage byte**, so the reset applies only from this release forward. Cost: a
schema addition.

**(b) Accept a retroactive reset** and re-measure every claim resting on recorded history. Cost,
measured on v0.35.0's audit — **9 rounds carrying 7 distinct `spec_hash` values** (r1=r2, r7=r8):

| | before | after a retroactive reset |
|---|---|---|
| `blocking` converges at | round 3 | round 8 |
| `all` converges at | round 9 | **never** |

So a shipped, converged cycle reports `converged: false` on install, and *"six rounds of a nine-round
phase saved"* becomes one — 9−3 against 9−8. **Whoever takes (b) owns re-deriving those figures**, with:

```
python3 -c '
import json
d="spec/.tp-review/0.35.0/"; R=json.load(open(d+"state.json"))["audit_rounds"]
def clean(r,pol):
    for x in (json.loads(l) for l in open(d+r["file"]) if l.strip()):
        if x.get("status")=="PASS": continue
        if pol!="blocking" or x.get("severity") not in ("warning","info"): return 0
    return 1
for pol in ("blocking","all"):
    c=[clean(r,pol) for r in R]
    def first(reset):
        n=0
        for i in range(len(R)):
            n = 0 if not c[i] else (1 if n and reset and R[i]["spec_hash"]!=R[i-1]["spec_hash"] else n+1)
            if n>=2: return i+1
        return "never"
    print(pol,"before:",first(0)," after:",first(1))'
```

**That command recomputes `clean` from the round files rather than reading the stored flag, and it has
to.** Audit `clean` is stamped at record time and never recomputed live — `engine.AuditRowsClean`'s own
doc comment says so, contrasting itself with its review twin `ReviewRoundClean`, which re-reads on
every call. **So this table cannot be produced by running shipped tp against the stored history.** The
same command is the implementer's fixture generator, and the policy must be pinned rather than
inherited: v0.35.0's stored flags are the `all` grading, and under `blocking` the same history
converges at round 8 and would report `converged: true`.

### 2.1 Three of those nine rounds carry a hash their roles did not read

Rounds 5, 6 and 7 carry a recorded `spec_hash` that is not the sha256 of their own snapshot, and those
three are the whole of tp's audit-phase snapshot/hash divergence (Non-Goal 3). It is not incidental
here: round 7 is half of the r7=r8 pair that puts `blocking`'s post-reset convergence at round 8. The
figures above are derived from the recorded hashes, which is the only thing a reset can key on — not
from the text those rounds' roles read. Per phase, over every recorded round carrying a snapshot:

```
python3 -c '
import json,glob,hashlib,os
for k,p in (("review","snapshot-round-%d.md"),("audit","snapshot-audit-round-%d.md")):
    m=t=0
    for f in glob.glob("spec/.tp-review/*/state.json"):
        d=os.path.dirname(f)
        for r in json.load(open(f))[k+"_rounds"]:
            s=os.path.join(d,p%r["round"])
            if not os.path.exists(s): continue
            t+=1; m+= "sha256:"+hashlib.sha256(open(s,"rb").read()).hexdigest()!=r["spec_hash"]
    print(k,m,"of",t)'
```

### 2.2 The legacy marker is not a vintage byte

**There is no third option, and the existing marker does not supply one.** `engine.IsLegacyRound` is
`r.IDScheme == ""`, and the slug has been stamped on every audit round since v0.30.0 — so it separates
v0.30.0 from v0.29.0, not this release from its predecessors. **Adding that check changes the trailing
streak in none of the recorded audit histories.** The number of histories is deliberately not quoted —
it grows with every cycle. Derive it and the delta together:

```
python3 -c '
import json,glob
def streak(rs,legacy):
    n=0
    for i in range(len(rs)-1,-1,-1):
        if not rs[i]["clean"]: break
        if n and not (legacy and "" in (rs[i].get("id_scheme",""),rs[i+1].get("id_scheme",""))) \
             and rs[i]["spec_hash"]!=rs[i+1]["spec_hash"]: n+=1; break
        n+=1
    return n
h=[r for r in (json.load(open(f))["audit_rounds"] for f in glob.glob("spec/.tp-review/*/state.json")) if r]
print(sum(streak(r,0)!=streak(r,1) for r in h),"of",len(h))'
```

## 3. The mechanism, once the decision is made

`consecutive_clean` is **derived, not stored** — `engine.ConsecutiveClean` walks `AuditRounds`, and the
audit path plus `tp review --status` share it. So the shape is to **add** rather than change:
`ConsecutiveCleanSince` and `ConvergedSince`, called from the audit path only.

**`Converged` must move too, not just `ConsecutiveClean`.** `engine.Converged` folds
`ConsecutiveClean` internally, so a reset that leaves `converged` answering the pre-reset value is not
a reset.

**`ConvergedSince` must say how it composes with the conjunct already there.** `Converged` is
`ConsecutiveClean(rounds) >= requiredCleanRounds && !StateStale(rounds, currentHash)` — the trailing-edge
gate of §1. The two are not the same test and neither subsumes the other: `StateStale` compares the last
recorded round against the spec on disk, this release compares each round against its predecessor. The
decision this release takes is that `ConvergedSince` **keeps `StateStale` unchanged and narrows the
first conjunct only**, so a converged verdict means "enough trailing clean rounds over one unchanged
text, and that text is still on disk". Replacing `StateStale` rather than keeping it would let a spec
move after the last round pass, which is behaviour tp has today and must not lose.

**Seven non-test call sites, and which ones move is the whole risk.** Derive the list, and cite no line
numbers here. An earlier draft of this section carried two pairs of them for `audit_record.go`, and
**neither pair was ever a `Converged` call site** — over the last 80 commits touching that file the two
calls appear at no such pair, and the pair offered as the *correction* is the `engine.ConsecutiveClean`
call six lines below each `Converged` call. That is a grep for the wrong one of the two functions this
section exists to distinguish, inside the sentence telling the reader to re-derive. Run:

```
grep -rn 'engine\.Converged(\|= Converged(' internal/ --include='*.go' | grep -v _test.go
```

It returns seven, in this breakdown: `internal/cli/audit_record.go` twice, `internal/cli/budget.go`,
`internal/cli/run_status.go`, `internal/engine/resume.go` twice, and `internal/cli/review_status.go`.
The one further raw match is in `internal/cli/audit_signal_test.go` and is correctly excluded.

**`internal/engine/resume.go`'s audit line is the one that gets missed.** It feeds `DetectPhase`'s `release`
branch, so leaving it on the shared `Converged` makes `tp resume` report `phase: release` on a streak
the reset just invalidated. **`review_status.go` must not move.**

## 4. The boundary against `--reconcile`

`--reconcile` records **why** a spec moved, preserving the operator's reason. This records only
**that** it moved, and only for the streak.

**They compose and neither implies the other.** A reset with no explanation tells an operator to redo
work without saying why; an explanation with no reset lets a stale claim stand. Two readers: the loop,
and the person.

## 5. Non-Goals

1. **No change to the review phase, and the surface is `tp review --status`.** `review_status.go`'s
   `Converged` and `ConsecutiveClean` calls stay on the shared functions; review already acts on spec
   movement by its own route. Naming the surface is load-bearing, because the review phase reports
   `consecutive_clean` through **two different functions**: `review_status.go` calls the severity-blind
   shared `engine.ConsecutiveClean`, while `review_record.go`, `review.go` and `import_enforce.go` call
   the severity-aware `engine.ReviewConsecutiveClean` (and `ReviewConverged`) over the same rounds — so
   `tp review --status` and `tp review --record` can already disagree. **This release freezes that
   inconsistency rather than fixing it**, and does so knowingly: it is a severity question, not a hash
   question, and it belongs to whichever release takes review-side convergence next.
2. **No reset of `spec_coverage_clean_rounds` or `role_streaks` in this release.** They are derived
   from the same rounds and the same argument applies, but each is a separate claim with its own
   consumers, and widening the blast radius of a reset is how a correct change becomes an incident.
3. **No repair of the rounds whose snapshot and recorded hash disagree.** The figure is per phase, and
   the audit half is the one that matters here: review 35 of 172 rounds carrying a snapshot, **audit 3
   of 88** — and all three audit ones are in v0.35.0, the single history §2's cost table rests on. §2
   names those rounds and carries the derivation. Whatever §2 decides, their hashes are not what their
   roles read, and this release does not pretend otherwise.
4. **No new workflow field.** The reset is not configurable; a spec that moved, moved.
5. **No `--reconcile` interaction.** A reconciliation explains a movement; it does not exempt it.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | two clean audit rounds either side of a spec change leave `consecutive_clean` at 1, not 2 | the shipped behaviour, which reports 2 |
| 2 | §3 *converged* | `converged` is false in the same fixture, not merely `consecutive_clean` reduced | move `ConsecutiveClean` alone, leaving `Converged` answering the pre-reset value |
| 3 | §3 *resume* | `tp resume` reports the audit phase, **not** `phase: release`, in that fixture | **move the `internal/cli` sites and leave `internal/engine/resume.go`'s audit line on the shared function.** That file carries two `Converged` calls, review and audit, and only the audit one reaches `DetectPhase`, whose `numTasks > 0 && numDone == numTasks` branch returns `PhaseRelease` on `auditConverged` alone — so the mutant is reachable and the release branch is the only thing that discriminates it |
| 4 | §5.1 | **`tp review --status`** — the surface §5.1 names, and the one computed by the shared `ConsecutiveClean`/`Converged` — reports byte-identical `consecutive_clean` and `converged` before and after. The `--record` path is out of this row's scope because it runs the severity-aware `ReviewConsecutiveClean`, which this release does not touch | move `review_status.go` too, resetting a phase that already handles movement its own way |
| 5 | §2 | whichever option §2 takes, its stated cost is asserted: under (a) a pre-release round is unaffected; under (b) v0.35.0's recorded history reports `converged: false` | leave the vintage rule untested, so the release ships with (a)'s schema and (b)'s behaviour |
| 6 | §3 *no change* | with `audit_clean_rounds` **pinned at the default 2**, an unchanged spec across five rounds leaves the streak at 5 | reset on every round. **The pin is the row, not decoration:** the field's legal range is 1–10 (enforced in `internal/engine/projectconfig.go` and warned in `internal/engine/validate.go`), and at 1 this mutant survives its own test — measured, one clean round at `audit_clean_rounds=1` gives `consecutive_clean: 1`, `converged: true`, `tp audit --status --check` exit 0, so a per-round reset leaves convergence reachable |

**Row 3 is first on this list because no draft test caught its mutant — and that sentence is an
author's note, not a checkable claim.** The drafts were written in context and never committed — every
committed revision of this file already carries row 3 and a paragraph saying this, so none of them is
one of the drafts:

```
git log --all --format=%h --follow -- spec/1.53.0.md | tee /dev/stderr | wc -l   # revisions
git log --all --format=%h --follow -- spec/1.53.0.md |
  while read c; do git show $c:spec/1.53.0.md 2>/dev/null || git show $c:spec/0.53.0.md; done |
  grep -c 'no draft test caught it'                                             # revisions carrying it
```

And no implementation of this release exists, so no draft assertion list was ever *run* against the
mutant. Ground round 1 graded the sentence **UNVERIFIABLE** and it stays in with that status stated,
rather than being softened until it stops claiming anything — softening would delete the reason row 3
is ordered first without supplying another. A reader who does not take the author's word for it has row
3's own mutant column, which is mechanically checkable and is what the ordering ultimately rests on.
