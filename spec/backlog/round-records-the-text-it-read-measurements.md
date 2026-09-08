# round-records-the-text-it-read — measurements

Supplemental material for `round-records-the-text-it-read.md`; the spec stands without it.

Two drafts were merged into that spec: *the round carries the text it read* (the hash) and *the
spec-hash reset* (the streak). Blocks below are verbatim from whichever draft carried them, and section
numbers inside a block are that draft's own.

## The corpus, re-derived

Over every recorded round in `spec/.tp-review/`, measured at `d7aad3b7`:

| phase | rounds | no snapshot on disk | snapshot sha256 ≠ recorded `spec_hash` |
|---|---|---|---|
| review | 172 | 0 | **35** |
| audit | 108 | 20 | 3 |

```
for each spec/.tp-review/<base>/state.json, for each recorded round:
  sha256(snapshot-round-N.md)  vs  round.spec_hash        # review
  sha256(snapshot-audit-round-N.md)  vs  round.spec_hash  # audit
```

**Re-run the block; do not quote the row.** The three numerators — 35, 20, 3 — have not moved once
since this release was first drafted. Both denominators have: 168 → 172 and 97 → 108, which is
`spec/.tp-review/1.0.0`'s own 4 review and 11 audit rounds exactly. Re-deriving a figure does not
immunise it, because the thing that moves is the denominator; only the numerator plus the command
survives, which is why the percentages this table used to carry are gone.

**The asymmetry is the argument, and it is a numerator argument.** 35 against 3, over denominators of
the same order. The phase whose purpose is to change the spec certifies a text it did not read an
order of magnitude more often than the phase that changes code. This is not a rare race; it is the
normal shape of a review round.

**Two cautions, both earned.** The 35 is unchanged from when it was last counted against 156 review
rounds, and the sentence that recorded that fact carried an arithmetic error inherited from the
roadmap: *"v0.36.0 and v0.37.0 added twelve rounds"*. Per-base counts give 141 review rounds before
v0.36.0, so 156 = 141 + 15 already contains **all** of v0.36.0; the twelve are v0.37.0's alone, and
v1.0.0 has since added 4 more. The correct statement today is **27 review rounds added since the 156,
carrying zero mismatches** — the percentage fell without the defect improving. And 19 of the 35 are
one cycle, v0.31.0's, so the mean is nobody's experience: eleven of seventeen bases carry none, and
one carries nineteen.

```
python3 -c "import json,glob,os,collections;print(collections.Counter({os.path.basename(os.path.dirname(f)): len(json.load(open(f)).get('review_rounds') or []) for f in glob.glob('spec/.tp-review/*/state.json')}))"
```

The audit phase's **20 rounds with no snapshot at all** predate snapshotting on that phase
(`git log --reverse -S WriteSnapshotAtomic -- internal/cli/audit.go`). They are not mismatches and are
not fixed here; §2 states what they resolve to.

The per-phase count of rounds carrying a snapshot whose sha256 differs from the recorded hash — the
reset draft's Non-Goal 3 quoted it as review 35 of 172 and audit 3 of 88, all three audit ones in
v0.35.0 — derives with:

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

## Why the fallback is about the live path, and the overwrite measured

**The reason is the live path, not backward compatibility** — and the backward-compatibility reason an
earlier draft gave was measured false. A recorded round's `spec_hash` is never recomputed: it is read
straight off the struct field, and no reader touches a snapshot. Measured — delete the snapshots of
two *recorded* rounds in a sandbox, then `tp review <spec> --status` and `tp resume <spec>`: both exit
0 and return the stored hashes byte for byte. So no fallback, erroring or otherwise, can make the
snapshot-less audit rounds of §1.1 unreadable; they are read out of `state.json` and were never going
to be hashed again. What an erroring fallback *would* break is a **new** `--record` for a round with
no snapshot on disk — a record with no preceding emission — and that case is live today: delete
`snapshot-round-N.md` and `tp review <spec> --record <file>` still exits 0 and stores the spec path's
hash.

**What this does not claim.** It makes `spec_hash` equal *the last text emitted for that round*, not
*the text each role actually read*. Those differ when a round is emitted, roles run, the spec is
edited, and the round is emitted **again** before recording.

**The overwrite is measured, not assumed.** Reproducible on demand rather than quoted, because the
digests of any one run are transient:

```
tp review <spec>                  -> snapshot-round-1.md   sha A
edit <spec>
tp review <spec>                  -> snapshot-round-1.md   sha B != A
ls .tp-review/<base>/             -> one snapshot-round-1.md; A's bytes exist nowhere
```

Run here against a sandbox spec: `dfbe94d7… -> 02e4be9c…`, one file. Structurally,
`engine.WriteSnapshotAtomic` renames onto the single fixed name `snapshotFilename(phase, round)`
returns — one path per `(phase, round)`, no history — so **the first emission's text is
unrecoverable**. That is why §2 cannot promise more than it does, and why §3 counts the event rather
than trying to reconstruct it. It is the only residue.

## `spec_moved_mid_round`, handed to reconcile

This was the hash draft's §3. Its consumer is `spec/backlog/reconcile.md`, which is why it left the
spec; the design is kept here for that release.

### 3. A re-emission that changes the text is recorded

When `tp review` or `tp audit` writes a snapshot for a round that already has one, and the bytes
differ, the round records that it happened: **`spec_moved_mid_round`**, a count of such re-emissions,
absent on rounds where it never occurred.

**The signal is produced here and consumed elsewhere.** This release does not warn, block, reset a
streak or recommend anything on it — deciding what a mid-round spec move *means* is the reconcile
release's subject, and a gate built on a signal nobody has yet seen data for is the mistake this
project has already paid for eight rounds of suppression to learn.

**Counted, not flagged, and the corpus is not the argument.** A boolean cannot distinguish one repair
from six; the count costs the same as the flag; that is the whole case. It is tempting to reach for
§1.1's 35 as the witness and it would be the wrong population — **a mismatch and a re-emission are
different events, and a two-arm probe on the current binary shows they can be disjoint in both
directions**:

```
arm A   emit -> edit spec -> record                 snapshot 02e4be9c, recorded hash 83d448f0
        MISMATCH, zero re-emissions
arm B   emit -> edit spec -> emit -> record         snapshot 02e4be9c, recorded hash 02e4be9c
        re-emission, NO mismatch
```

So §1.1's 35 mismatches are, as far as the record can say, rounds on which `spec_moved_mid_round`
would be **absent** — a boolean and a count would report every one of them identically. Nothing in
the corpus records how many times any round was emitted, which is exactly why §3 produces the signal
before anything gates on it.

**The count is durable only once the ship-signal release ships**, which makes that release this one's
prerequisite. Measured on `HEAD`: `spec_moved_mid_round` injected onto a round entry is **gone after
the next `--record`**, exit 0, silently — `engine.ReviewRound` is a typed struct with no catch-all and
`SaveReviewState` does `json.MarshalIndent(st, …)`, while reading ignores unknown keys. Its §2.1,
"`state.json` preserves keys it does not understand", is what changes that. **The roadmap records this
release as ready and names no dependency**, while sibling rows spell theirs out; the dependency is
stated here because a claim only two specs make is a claim the ordering does not carry.

**Compared by bytes, not by hash of a re-read — and this is a trade, not a free move.** The
comparison is between the snapshot already on disk and the bytes about to replace it. Only the right
operand is in hand: neither emission site reads the existing snapshot today
(`engine.WriteSnapshotAtomic` writes and renames without reading; the only reader of a round snapshot
anywhere is `review_autodiff.go`, and it reads round *N-1*), so §3 adds one `os.ReadFile` of the
snapshot at each emission. What the release removes is a second read of the **spec** at record time;
what §3 adds is a first read of the **snapshot** at emission. It is still the right trade — the
snapshot's bytes are what the answer is *about*, and hashing a re-read spec would reintroduce exactly
the timing dependence §2 exists to remove — but it costs a read, and an earlier draft claimed it cost
nothing.

### Tests that went with `spec_moved_mid_round`

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 4 | §3 | emit, edit, emit, record — `spec_moved_mid_round` is 1; a third differing emission makes it 2 | store a boolean, which reports six repairs in one round identically to one — the distinction the field exists to make, and one no recorded round can supply today (§3) |
| 5 | §3 *identical* | a re-emission whose bytes are unchanged does **not** increment the count | compare timestamps or always increment, so re-running `tp review` to re-read a prompt registers as a spec move |
| 6 | §3 *absent* | a round with no re-emission omits the key rather than recording 0 | emit 0 always, which makes every pre-release round indistinguishable from one measured at zero |

## Why not the other two shapes

Recorded so neither is re-proposed:

**Pin the hash at emission into the state.** This means writing state on a read-only-looking command.
`tp review` and `tp audit` already write a snapshot, so the objection is not purity — it is that
emission state has no natural owner when a round is emitted several times, and choosing one is the
same undecided question §2 leaves as residue. Hashing the snapshot inherits the answer instead of
inventing one.

**Make the emission the recorded unit.** The state model is rounds all the way down. Ten engine
functions take `rounds []ReviewRound` — among them `ConsecutiveClean`, `Converged`, `StateStale`,
`RolesStale` and `ComputeAuditRoleStreaks` — and nine CLI files load the round index:

```
grep -n 'rounds \[\]ReviewRound' internal/engine/*.go
grep -rln 'LoadReviewState' internal/cli/ --include='*.go' | grep -v _test.go
```

Re-basing that on emissions is a rewrite of the state model to fix a hash argument.

An earlier draft named `--merge` and `--report` in that list. Both are file-driven —
`runReviewMerge(args []string, outputPath string)` and `runReviewReport(args []string)` take the
NDJSON files named on the command line — and neither reads the round index at all, so naming them
overstated the blast radius of an alternative this section is rejecting. That is the direction an
argument for a decision must never err in.

## The retroactive alternative, costed on v0.35.0

The reset draft's §2 put two options side by side; the spec takes (a). Verbatim:

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

### Three of those nine rounds carry a hash their roles did not read

Rounds 5, 6 and 7 carry a recorded `spec_hash` that is not the sha256 of their own snapshot, and those
three are the whole of tp's audit-phase snapshot/hash divergence. It is not incidental
here: round 7 is half of the r7=r8 pair that puts `blocking`'s post-reset convergence at round 8. The
figures above are derived from the recorded hashes, which is the only thing a reset can key on — not
from the text those rounds' roles read. The per-phase derivation is the last block of "The corpus,
re-derived" above.

## The legacy marker is not a vintage byte

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

## The convergence call sites

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
Re-run at `HEAD` while merging the two drafts: the same seven, same breakdown.

**`internal/engine/resume.go`'s audit line is the one that gets missed.** It feeds `DetectPhase`'s `release`
branch, so leaving it on the shared `Converged` makes `tp resume` report `phase: release` on a streak
the reset just invalidated. **`review_status.go` must not move.**
