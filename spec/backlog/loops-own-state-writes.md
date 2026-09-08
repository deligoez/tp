# tp — The loop's own state writes

Class: tool

> **This file is decisions.** Two defects in how the loop writes and watches its own state, each
> re-run against `HEAD` while this file was written and again at its ground round rather than
> carried forward from any handover text. What the v0.36.0 corpus does and does not say about them,
> and why this file's `section-size` warnings were left standing, are in
> `loops-own-state-writes-measurements.md` under "What the v0.36.0 corpus says" and "The
> section-size warnings".

## 1. Overview

`spec/.tp-review/<base>/` is where the loop keeps what it knows: the round index, each round's
snapshot, each round's findings — and, since ground rounds, each ground round's snapshot and floor.
Two things are wrong with how it is written and watched, and they share a subject rather than a
mechanism:

1. **The round's findings file is not written atomically** (§2) — at any of its three writers, one of
   which is `--resolve` rather than `--record` — while the snapshot beside it is.
2. **The gate that watches the directory cannot see a directory-only change** (§3) — a blind spot the
   script documents in its own comment and does not close. Note the wording: files under a watched
   path are still caught, emptying a directory included.

Neither adds a command, a flag or a workflow field. `state.json` is already written atomically at
`HEAD` — `engine.SaveReviewState` in `internal/engine/reviewstate.go` goes through a unique temp file
and a rename — so the round findings file is the last plain write in that directory. A third defect
in the same area — a refused `--role` invocation still writes state — is owned by
`spec/backlog/a-findings-exits-agree.md` and is not in this file.

## 2. The round's findings file is written atomically

`internal/cli/review_record.go` and `internal/cli/audit_record.go` both write the round file with a
plain `os.WriteFile`; `git grep -n 'os.WriteFile(filepath.Join(engine.ReviewStateDir' -- internal/cli/`
returns those two lines and nothing else. `engine.WriteSnapshotAtomic`
(`git grep -n 'func WriteSnapshotAtomic'`), used for the snapshot in the same directory during the same
round, goes through `writeFileAtomic`: `os.CreateTemp` in the target directory, `Write`, `Close`,
`os.Rename`. The snapshot already proved the pattern in this exact directory; the round file is what
the readers below parse, and it is the one written without the guarantee.

**Three writers, not two, and the release covers all three.** The grep above matches on the literal
`engine.ReviewStateDir` argument, so it cannot see a writer taking its path from `args[0]` — do not
read it as an enumeration. The counting rule is *a non-test call that writes bytes to a path that is,
or can be, `<state-dir>/<review|audit>-round-N.ndjson`*; derive the candidates with
`git grep -n 'os.WriteFile' -- internal/cli/ | grep -v _test.go` and
`git grep -n 'writeNDJSON(' -- internal/ | grep -v _test.go`, following each back to its caller. The
third is `writeNDJSON` in `internal/cli/review_resolve.go`, the read-modify-write behind
`tp review --resolve`/`--resolve-all` and `tp audit --resolve`/`--resolve-all`, whose `filePath` is the
round file named on the command line.

**Why the third is covered rather than fenced out**, in order of weight. (a) The cleanliness predicate
is *designed* to read what `--resolve` writes: `ReviewRoundClean`'s doc comment says a later
`--resolve` re-evaluates the round without re-recording, and `reviewFindingResolvedAway` reads the
`resolved` dict live off the file — a designed read/write pair on one file. (b) The locks do not close
it and the writers do not share one: the record sites run under `engine.WithReviewStateLock`, which is
`WithFileLock` on *`state.json`*, while the resolve sites take `WithFileLock` on the *round file*
(`git grep -n 'WithFileLock\|WithReviewStateLock' -- internal/ | grep -v _test.go`), so they do not
exclude each other and the readers take neither. (c) It is the normal path rather than a misuse: the
rows carrying a `resolved` dict are counted in the sidecar under "Why an operator cannot catch it by
looking". Fencing it out would make this heading false — the round file would still be rewritten
non-atomically on the path the corpus uses most, leaving §2.1's chain unbroken through it — and the
repair is the same change as at the record sites, so it adds no abstraction and no new surface.

**Which readers — derived, not hand-listed.** `engine.LoadRoundRows` is the round file's reader; get
its call sites from
`git grep -n 'LoadRoundRows(' -- internal/ | grep -v _test.go | grep -v 'func LoadRoundRows'`, counting
rule *a non-test line containing `LoadRoundRows(` that is a call and not the declaration*. Read that
output rather than a count restated here. Two things in it matter: `internal/engine/reviewclean.go`
appears **twice** — `ReviewRoundClean`, and therefore review convergence, plus
`ReviewRoundNonBlockingOpen` — and `internal/cli/review_record.go` appears, because `--record` itself
walks every previously recorded round. **`tp review --merge` is not in the output and is not a
reader**: `internal/cli/review_merge.go` contains no `LoadRoundRows`, no `ReviewStateDir` and no
`os.ReadFile`, and `loadMergeFindings` opens the command's positional arguments — the per-role files,
never the recorded round file. `ComputeAuditRoleStreaks` reads the same files through
`internal/engine/auditrounds.go`'s own reader. **Audit convergence does not read them**: `Converged`
calls `ConsecutiveClean`, which walks the stored `Clean` booleans in `state.json`. So a truncated
*review* round file moves review convergence and a truncated *audit* round file does not — narrower
than "every convergence count", and sharper, because the count it does move is the one that decides
review convergence.

### 2.1 The measured difference, and the chain it feeds

**This is a live concurrency defect, not crash safety, and the difference is measured rather than
argued.** The probe: a payload of the corpus-median round-file size, written 4,000 times to one path by
a single writer while four lock-free readers read it, once with `os.WriteFile` and once with
`os.CreateTemp` + `Write` + `Close` + `os.Rename` — a transcription of `writeFileAtomic`. No fault
injection, no process death.

| writer | partial reads |
|---|---|
| `os.WriteFile` — the shipped shape | thousands, of the same order as the whole reads |
| temp file + `rename` | **zero, on every run** |

**No integers are quoted, because they are not reproducible and were not.** The counts are a function
of how many reads four goroutines complete during the writer's run, so they are load- and
machine-dependent: two independent implementations of this probe put the partial share of all reads at
47.5% on one machine and 52.6–52.9% across three runs on another. The zero is the claim the section
rests on, and it is categorical rather than statistical — `rename(2)` is atomic, so a reader observes
the old inode or the new one and never a half-written one. Anyone re-running this should expect the
ratio back, not the integers.

tp's own reads are lock-free by design — `engine.WithReviewStateLock` has exactly two non-test callers,
the two record paths, while `--status` and `tp resume` take no lock at all — so a partial read is
reachable without a crash.

**And the chain from a truncated file to a clean round is code, not inference.** `LoadRoundRows` does
`if err := json.Unmarshal(...); err != nil { continue }` — an unparseable line is **silently skipped**,
and its own doc comment says so. `ReviewRoundClean` then recomputes cleanliness live from whatever rows
survived ("Read live from the round's recorded findings"), and `ReviewConsecutiveClean` counts the
result.

**A truncated review round file can therefore make the round clean — up to a boundary, and the boundary
is measured.** Universally true is the mechanism: the truncation removes findings and the predicate
grades the round on whatever subset survived. Whether that subset reads *clean* depends on where the
prefix stops. Recipe, in a throwaway git repository outside this tree, against a round recorded with
three `severity: high` findings:

```
tp init demo.md && tp review demo.md --record f.ndjson    # f.ndjson: three severity:high rows
F=.tp-review/demo/review-round-1.ndjson; cp "$F" /tmp/full
for n in 0 100 300 600 $(wc -c < /tmp/full); do head -c "$n" /tmp/full > "$F"; tp review demo.md --status; done
```

`consecutive_clean` comes back **1** while the prefix stops inside the first row and **0** once one
complete blocking row survives — on that fixture, 1 at 0/100/300 bytes and 0 at 600 and intact. Every
size here is a function of how much the fixture pads its finding text, so re-derive rather than reuse
them. Two consequences the bare claim hides. The boundary is **not** the first row:
`review_converge_on` defaults to `blocking` and `ReviewRoundClean` grades on *blocking* survivors only,
so a prefix that keeps every advisory row and loses only the blocking ones is clean too — a second
fixture of two `medium` rows then one `high` reads clean with nine tenths of its bytes and two of its
three findings present. And the recorded count does **not** move: `state.json` still reports
`findings: 3` in every truncated arm while `clean` flips to `true`, because the count is stored at
record time and cleanliness is recomputed live. `findings: 3` beside `clean: true` is a
self-contradictory pair and the one signal the truncation does not erase — §2.2's indistinguishability
argument does not mention it and should not be read as denying that any signal survives.

### 2.2 Why an operator cannot catch it by looking

An empty round file is legitimate — a round in which every role found nothing writes one, and that is
what convergence looks like — and a truncated write lands in a state the corpus already holds honest
examples of, so nothing distinguishes the two by inspection; the size profile of the recorded round
files and the census of the zero-byte ones are in the sidecar under "Why an operator cannot catch it
by looking".

**Ordering is unchanged.** The round file is written before the index entry, and that stays: an
orphaned round file is rebuildable and an index entry pointing at nothing is not.

## 3. The gate sees an empty watched directory

`scripts/check-suite-state.sh` digests `spec/.tp-review` and `.tp/rounds` before and after the suite.
The whole of `state_digest`, quoted rather than excerpted — the `[ -d ]` guard and the outer pipe are
both load-bearing below:

```sh
state_digest() {
	local dir
	for dir in "${WATCHED[@]}"; do
		if [ -d "$dir" ]; then
			find "$dir" -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
		fi
	done | shasum -a 256 | cut -d' ' -f1
}
```

**`-type f` means an empty directory contributes nothing.** Creating one under a watched path and
removing one are both invisible to the wrapper — verified by running the real script in an
`rsync -a --exclude .git` copy outside the repository, with the mutation performed *by* the narrowed
suite so the wrapper brackets it: exit 0 in both directions on both arms, against a control that
writes a file and exits 1.

**Emptying a directory is *not* invisible, and an earlier draft of this section said it was.**
Removing the file changes the sorted file digest and the guard fires. The script's own comment already
says the correct, narrower thing — *"The blindness is to directory-only changes: every file written
under either path is still caught"* — under the heading `KNOWN BLIND SPOT, measured in audit round 3`,
closing with *"Routed to a later release rather than repaired here"*. Documented, and not closed. Take
the script's wording: an implementer working from the wider claim would write a test asserting the
wrapper misses a case it already catches.

**There is a live instance in this repository right now.**
`find spec/.tp-review .tp/rounds -type d -empty` returns `spec/.tp-review/1.0.0/rounds`. The fixture
does not have to be invented.

### 3.1 The decision: hash the sorted path listing as well as the file contents

`state_digest` emits, per watched directory, the `LC_ALL=C`-sorted listing of every path under it —
directories included — concatenated with the existing file-content hashes, and the whole is hashed as
today. Verify any candidate walk by digesting one fixture in four states and comparing all of them:
baseline, an empty directory added, that directory removed, a file's contents changed. Measured on the
proposed walk: the added state differs from baseline, the removed state returns to the **exact**
baseline value, and the content change differs. No other candidate below discriminates in more than
one direction.

**Adding `-type d` to the existing pipeline does NOT work, and the two ways of writing it fail
differently.** They have to be kept apart or the refutation is ambiguous:

| walk | empty dir added | that dir removed | file content changed |
|---|---|---|---|
| `-type f` (shipped) | no change | no change | detected |
| naive `find "$dir" -type f -o -type d -print0` | no change | no change | **no change** |
| parenthesised `find "$dir" \( -type f -o -type d \) -print0` | no change | no change | detected |
| sorted path listing **+** file content hashes | **changes** | **returns to baseline** | detected |

Why each rejected row fails the way it does — the naive form loudly, the parenthesised form by changing
nothing at all — is in the sidecar under "The two refuted walks". This release deletes the script's
`KNOWN BLIND SPOT` comment by closing it: a comment naming a blind spot is not a mitigation.

**The `.tp/rounds` arm gets the same walk.** The two arms are already one `WATCHED` array and one loop,
and the break-and-control run confirms they are equally blind today; making them differ would leave the
untested one to drift. In shipped tp its only creator is `engine.clearUnitArtifacts` on the `tp run`
path — `git grep -n 'MkdirAll' -- internal/ | grep -v _test.go`, the entry whose path is
`engine.RoundDir` — so in a fresh checkout it is absent rather than empty (`.tp/.gitignore` carries
`rounds/`), while here it is present and populated, so the fixture is available on both arms. One
exception, and the least harmless one available: that same grep returns
`internal/fakerunner/cmd/tp-fake-runner/main.go`'s `writeRoleFindings`, outside `cmd/tp` and off the
`tp run` path but driven by the suite — the very process this gate brackets — so row 6 must assume the
arm can be created *during* the bracketed run, not only before it.

## 4. Non-Goals

1. **The other v0.36.0 handovers are not here.** Two CI guards narrower than their own claims, and a
   load-sensitive gate test, are about what CI certifies rather than what the loop writes; they belong
   with `spec/backlog/gate-sequence.md`.
2. **No cleanup command, no `--prune`, no GC.** Deciding what an orphaned state directory means is a
   judgement tp cannot make from disk.
3. **No lock added, removed or re-keyed.** `engine.WithReviewStateLock` — a thin `WithFileLock` on the
   state path — already guards both record paths, and the `--resolve` paths take `WithFileLock` on the
   round file itself. Those are different keys, so a record and a resolve do not exclude each other
   today and will not after this release: §2 changes how bytes land inside each critical section, not
   who may write. Note what the locks do *not* do: readers take none at all, which is the whole reason
   the atomic write is needed.
4. **No change to what the gate hashes, only to what the walk can see.** The watched set stays
   `spec/.tp-review` and `.tp/rounds` — the `WATCHED` array in `scripts/check-suite-state.sh`.
5. **No retroactive repair of round files already written non-atomically.** They are complete or they
   are not, and rewriting them would fabricate their history.

## 5. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. **No row quotes a measurement of this repository**; where a row names a number it is a
*fixture parameter the test sets* — four readers, 4,000 writes — and every claim about the corpus names
its derivation instead, because the four corpus figures this file originally carried were exact at its
own commit and three were stale two days later.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | with four lock-free readers against 4,000 writes of a corpus-median payload, **zero** reads observe a partial file | plain `os.WriteFile`, the shipped shape, which the same probe measures at thousands of partial reads against the atomic arm's floor of zero, so the test discriminates without fault injection |
| 1b | §2 *no content change* | the bytes the reader sees are **byte-identical** to what the writer wrote, on every whole read — asserted with `bytes.Equal`, never with a length comparison | rename a temp file padded to the payload's exact length with the wrong bytes (the previous round's payload will do) |
| 1c | §2 *the third writer* | `tp review --resolve`/`--resolve-all` and `tp audit --resolve`/`--resolve-all` land their rewrite of the round file through the same atomic path as the record sites — asserted with row 1's probe shape, with the resolve command as the writer | the shipped `os.WriteFile` in `writeNDJSON`, which the same probe measures at partial reads while the record sites' arm is at zero, so a fix that covers only `--record` passes row 1 and fails this one |
| 2 | §2 *ordering* | the round file lands before its index entry, unchanged by this release | swap them, producing an index entry pointing at a file that does not exist |
| 3 | §2 *empty* | a round whose every role found nothing still writes its empty file and records `findings: 0, clean: true` | treat an empty write as a failure, which breaks the legitimate empty rounds the corpus already holds |
| 4 | §3 | the wrapper's digest **differs** when the suite creates an empty directory under a watched path | `-type f`, the shipped walk, which reports the two states identical — this is the assertion the script's own comment says is missing |
| 5 | §3 *the refuted shapes, kept apart* | neither `-type d` variant is enough, and the fixture must still fail under **each**, for different reasons: the naive `-type f -o -type d -print0` digests empty input in every state, and the parenthesised `\( -type f -o -type d \) -print0` is byte-identical to the shipped walk in every state | ship either variant — one looks like the fix and stops the digest measuring anything, the other looks like the fix and changes nothing at all. The first is **not** silent, and this cell must not say it is: `shasum` prints `Is a directory` once per directory, and under the script's own `set -euo pipefail` the `before=$(state_digest)` assignment fails and the step exits 1 before the suite runs. It fails closed and loudly, as the sidecar's "The two refuted walks" records |
| 5b | §3 *removal* | the digest differs when an empty directory is removed, and **returns to the exact baseline value** | the shipped file-content-only digest, which yields the same value in the added state and the removed state, so the removal direction is untested under it. The path listing is `LC_ALL=C sort`ed so the digest is a function of the *set* of paths rather than of the order `find` emits them — the same reason the shipped file-content pipeline already sorts, and this release's only new term that could otherwise be order-dependent. Without the sort the failure direction is a **false** difference over an unchanged tree, never a missed change, since reordering cannot hide a name that is present or absent. An earlier draft justified the sort as *an unsorted listing can change on addition without returning to baseline on removal*; probed on APFS that did not reproduce — readdir order there is a function of the name set alone, two directories built from the same names in opposite insertion orders walk identically, and add-then-remove returns to the exact baseline listing — so do not restate it |
| 6 | §3 *both arms* | `.tp/rounds` gets the same walk as `spec/.tp-review`, asserted on both | widen one arm, leaving the untested one to drift |

**Row 1b's original mutant could not fail its own test, in either reading, and both were built and
run.** It named "a temp file whose contents were never flushed". Read literally — `os.CreateTemp`,
`Write`, `Rename`, no `Close` — the file on disk is the full payload and `bytes.Equal` is true,
because `os.File.Write` is a `write(2)` syscall with no user-space buffer to leave unflushed; the
test stays green under it. Read as "never written at all", the file is zero bytes, which a length
comparison already catches, so the row would prove nothing about `bytes.Equal`. The mutant above is
the realizable one: full length, wrong bytes. Nothing in the current design reaches that state by
accident, which is exactly why it has to be injected rather than waited for.

**Row 3 is the one an implementer will get backwards.** An empty round file is not a failed write —
the corpus already holds legitimate ones, every one carrying `findings: 0, clean: true`; enumerate
them with the sidecar's zero-byte census command rather than trusting a count in prose. They are
**trailing clean rounds, not necessarily terminal ones**, and the same command says which are not, so
the fixture may be built from any of them. So the atomic write must produce an empty file
where the old one produced an empty file, and the test has to say so or the fix will "helpfully"
reject it. The hazard is sharper still: the empty file the shipped code produces and a
truncated-to-nothing file are the same bytes, so no assertion on the file alone can separate them —
which is the argument for fixing the write rather than detecting the symptom.

**Row 6 has a trap for whoever writes it.** `.tp/rounds` is git-ignored, so the script's own
`git status --short -- "${WATCHED[@]}"` diagnostic prints nothing for that arm even when the digest
does change. Assert on the exit code, never on the diagnostic.
