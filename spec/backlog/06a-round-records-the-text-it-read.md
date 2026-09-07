# tp v1.40.0 — The round carries the text it read

> **This file is decisions.** Every figure names the command that derives it, and cites code by what
> the search finds rather than by a line number. That is not style: this spec's first grounding round
> found one of its two line citations pointing 37 lines above its subject after three unrelated
> commits, and its own re-derived corpus table stale by the very mechanism its header warned about.

## 1. Overview

A round's snapshot is written **at emission**, in both phases. `engine.WriteSnapshotAtomic` is called
from `internal/cli/review.go` under the comment *"snapshot the spec at round start (prompt
emission)"*, and from `internal/cli/audit.go` with `engine.PhaseAudit`:

```
grep -rn 'WriteSnapshotAtomic' internal/cli/ --include='*.go' | grep -v _test.go   # exactly two
```

A round's `spec_hash` is computed **at record**, by `engine.SpecHash(specPath)`, which re-reads the
file from disk. There are two such record-time call sites, one per phase — see §2 for the list and
for the third site that must be left alone.

Between those two moments the spec is edited — that is what a review round is *for*. So the round ends
up carrying two artifacts that describe different texts: a snapshot of what the roles were given, and
a hash of whatever the file said when the operator got round to recording.

**`spec_hash` becomes the hash of the round's own snapshot**, which makes the two agree by
construction rather than by the operator's timing — wherever a snapshot exists. Where none does, §2's
fallback applies and there is no second artifact to disagree with.

### 1.1 The corpus, re-derived

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

## 2. `spec_hash` is the snapshot's hash

At record time, `spec_hash` is `sha256` of the round's snapshot file rather than of the spec path.

**This is not a new artifact, a new field or a new write.** The snapshot is already written, already
atomic, already named per round and phase, and already read back by the regression path. The release
changes **two arguments at two record-time hash calls, in two files**:

```
grep -rn 'SpecHash(specPath)' internal/cli/review_record.go internal/cli/audit_record.go
  review_record.go:91    record, review phase   -> change
  audit_record.go:102    record, audit phase    -> change
  audit_record.go:375    --status, audit phase  -> leave: this is the CURRENT-spec side of
                                                   Converged/StateStale and must keep hashing the path
```

Two, not one, and the distinction is load-bearing rather than pedantic: §6 row 2 exists *because* a
fix applied to one phase passes rows 1 and 3, which is a hazard only if there are two sites. The
invariant this buys — *a round's `spec_hash` is the hash of the text stored beside it* — then holds
for every round recorded afterwards **that has a snapshot**, with no operator discipline; the
fallback below exempts the rest, vacuously, since there is then no snapshot to disagree with.

**A round whose snapshot is missing keeps today's behaviour and is not failed.** `spec_hash` for such
a round falls back to hashing the spec path, exactly as now.

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

## 3. A re-emission that changes the text is recorded

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

## 4. Why not the other two shapes

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

## 5. Non-Goals

1. **No repair of the 35 existing mismatches.** They are history; a round already recorded keeps its
   stored hash. Rewriting them would fabricate a claim about what those rounds read.
2. **No gate, warning or `next_action` on `spec_moved_mid_round`.** §3 produces the number and stops.
3. **No *edit* to `StateStale`.** It stays `rounds[len(rounds)-1].SpecHash != currentHash`, and every
   caller already passes a freshly computed `SpecHash(specPath)` as `currentHash`
   (`grep -rn 'StateStale' internal/ --include='*.go' | grep -v _test.go`), so no call site changes.
   **Its answers do change, and that is the correction rather than a side effect**: a round emitted,
   then edited, then recorded reports `stale: false` today — measured in a sandbox, because under
   shipped behaviour both sides hash the edited file — and will report `stale: true` afterwards. The
   Non-Goal is "no edit", not "no consequence"; a reader who takes it as the second will be surprised
   by the first round they record after upgrading.
4. **The 20 snapshot-less audit rounds are not backfilled.** No snapshot exists to hash, and
   synthesising one from today's spec would assert exactly the falsehood this release removes.
5. **No new workflow field.**

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | emit, edit the spec, record — the recorded `spec_hash` equals sha256 of the round's snapshot and **not** of the edited file | hash the spec path, which is the shipped behaviour and produces the 35 |
| 2 | §2 | over every round in a fixture repo, `spec_hash` equals its snapshot's sha256 — asserted as an invariant over the set, not on one round | hash the path on the audit phase only, which a single-round review test cannot see |
| 3 | §2 *fallback* | a round whose snapshot is absent records the spec path's hash and does not error | error on a missing snapshot, which breaks a **new** `--record` that had no preceding emission. Measured live: delete `snapshot-round-N.md`, run `tp review <spec> --record <file>` — exit 0 today, spec path's hash stored. It does **not** break reading old rounds; §2 says why |
| 4 | §3 | emit, edit, emit, record — `spec_moved_mid_round` is 1; a third differing emission makes it 2 | store a boolean, which reports six repairs in one round identically to one — the distinction the field exists to make, and one no recorded round can supply today (§3) |
| 5 | §3 *identical* | a re-emission whose bytes are unchanged does **not** increment the count | compare timestamps or always increment, so re-running `tp review` to re-read a prompt registers as a spec move |
| 6 | §3 *absent* | a round with no re-emission omits the key rather than recording 0 | emit 0 always, which makes every pre-v1.40.0 round indistinguishable from one measured at zero |
| 7 | §5.1 | a round recorded before this release keeps its stored `spec_hash` byte for byte after upgrade | recompute historical hashes on read, rewriting what past rounds are understood to have covered |

**Row 2 is the one that must be stated over the set.** Rows 1 and 3 are single cases and a fix applied
to one phase passes both — and the audit path is not merely a second phase, it is a second *file*
(§2), so a reviewer reading only `review_record.go` sees a complete-looking change. Only an assertion
quantified over every recorded round in the fixture fails when the audit path is left behind, and this
repository has lost rounds to exactly that shape: a claim about a set checked against one member the
author chose.

**That quantifier is vacuous on the audit side unless the fixture holds an audit round** — an
all-review fixture satisfies "every round" while proving nothing about `audit_record.go`. The
fixture's audit-round count is therefore *asserted* rather than chosen, the same rule that turned a
`zz` directory name into a `require.Less` on the sort order.
