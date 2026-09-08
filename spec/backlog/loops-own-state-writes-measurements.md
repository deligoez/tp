# loops-own-state-writes — measurements

Supplemental material for `loops-own-state-writes.md`; the spec stands without it.

## What the v0.36.0 corpus says

**Only §3 is a v0.36.0 handover.** Its blind spot is that cycle's `task-suite-state-assert` item;
§2 was found by measurement while this file was being written, and **no row in the v0.36.0 corpus
concerns the round findings file's write**. Derive it:
`python3 -c 'import glob,json,re;[print(f.split("/")[-1], json.loads(l).get("class") or json.loads(l).get("item_id")) for f in glob.glob("spec/.tp-review/0.36.0/*.ndjson") for l in open(f) if l.strip() and re.search(r"atomic|WriteFile", l, re.I)]'`
Read the rows it returns, not the count, and note the counting rule: *rows the command prints*, not
distinct findings. **Three** review rows match, not two. Two are round 1's
`atomicity-mechanism-unspecified` and `partial-write-policy-unstated`, both about the `--out-dir`
prompt set; the third is round 10's `cross-package-suite-races-shared-state`, an `implementer`
finding about concurrently-running test packages writing `snapshot-round-N.md` through
`engine.WriteSnapshotAtomic`. On the audit side the matching rows are round 7's
`snapshot-tmp-race-under-parallel-role-units` — `WriteSnapshotAtomic`'s fixed temp name, recorded
fixed inside that same cycle — together with round 8's rows verifying that same fix, which is one
subject across several rows. So every matching row is about the **snapshot** write or the `--out-dir`
prompt set, and **none is about the round findings file's write**. An earlier draft of this paragraph
said "two" review rows and "the only audit atomicity row"; both were counted off the command printed
above, and both are refuted by its output.

## The section-size warnings

**`tp lint` reports three `section-size` warnings on this file, and they are deliberate.** §2, §2.1
and §3.1 sit over the 50-line advisory threshold; the growth is round 2's additions, and the sections
were compressed once already (§2 from 103 lines to its current size). Getting under the threshold
means deleting a decision or a derivation, or splitting §2 — and splitting renumbers §2.1 and §2.2,
which are the anchors **both recorded ground rounds filed their units against**. This repository has
paid for renumbering three times, each leaving stale cross-references behind, so the warnings are
cheaper than the rename. `errors` stays 0 and `tp lint` exits 0; re-derive with
`tp lint spec/backlog/loops-own-state-writes.md`.

(Written before the spec's measurement sections moved into this file; the warning count now differs
from three, and the reason the anchors were not renumbered stands.)

## Why an operator cannot catch it by looking

**The severity is in the indistinguishability rather than the odds.** Derive the size profile of the
recorded round files with

```
python3 -c 'import glob,os,re,statistics; fs=[f for f in glob.glob("spec/.tp-review/*/*.ndjson") if re.search(r"/(review|audit)-round-\d+\.ndjson$",f)]; sz=[os.path.getsize(f) for f in fs]; print("files",len(fs),"median",statistics.median(sz),"max",max(sz),"over4k",sum(1 for s in sz if s>4096),"zero",sum(1 for s in sz if s==0))'
```

The median is tens of kilobytes and roughly nine in ten exceed a single 4 KB page, so a partial write
is a real shape and not a theoretical one. **No exact measured integer is stated here** — the hedged
ratio and the page size are deliberate, and the earlier wording "no figure is stated here" stood beside
three of them. This section once quoted four exact figures, each correct at its own commit; compare
`git show e73788ab:spec/1.43.0.md` against the command above at `HEAD` and one of the four still stands,
the rest having moved when v1.0.0's round files entered the corpus. Run the command.

The rows carrying a post-hoc `resolved` dict — the `--resolve` path §2 covers as the third writer —
are counted over the same file set by adding
`sum(1 for f in fs for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict))`.

Its last field counts the round files that are **zero bytes**. Reading each one's entry out of its
`state.json`, and asking at the same time whether it is the last round of its phase:

```
python3 -c 'import glob,os,json,re
last = {}
for f in glob.glob("spec/.tp-review/*/*.ndjson"):
    m = re.search(r"/(review|audit)-round-(\d+)\.ndjson$", f)
    if m: last[(os.path.dirname(f), m.group(1))] = max(last.get((os.path.dirname(f), m.group(1)), 0), int(m.group(2)))
for f in sorted(glob.glob("spec/.tp-review/*/*.ndjson")):
    m = re.search(r"/(review|audit)-round-(\d+)\.ndjson$", f)
    if not m or os.path.getsize(f): continue
    d, ph, n = os.path.dirname(f), m.group(1), int(m.group(2))
    st = json.load(open(os.path.join(d, "state.json")))
    ent = [(r.get("findings"), r.get("clean")) for r in st[ph + "_rounds"] if r.get("round") == n]
    print(f, ent, "last-of-phase" if n == last[(d, ph)] else "NOT-last (last is %d)" % last[(d, ph)])'
```

Every zero-byte round comes back `(0, True)`. Those are **legitimate** — a round in which every role
found nothing writes an empty file, and that is what convergence looks like. But run the command
before repeating the second half of the old claim: they are **trailing clean rounds, not necessarily
terminal ones**, and a substantial minority come back `NOT-last`, which is exactly what a
two-clean-round convergence rule produces. An earlier draft of this file called every one of them the
last round of its cycle; the command refutes it. Nothing in the argument needs "terminal", only
"legitimately empty".

That is why this matters: **a truncated write lands in a state the corpus already holds honest examples
of.** Fewer rows read as a cleaner round, an empty file reads as a converged one, and nothing
distinguishes the two. A defect whose signature is identical to success is not one an operator can
catch by looking.

## The two refuted walks

The naive form is the sharper refutation, and it is worse than merely insufficient: `-print0` binds to
the second branch of the `-o` only, so `find` prints directories alone, `shasum` errors on every one
of them, and the digest is `sha256` of the empty string in **every** fixture state — including the one
where a file's contents changed. As a digest it stops measuring anything at all. **The errors are not
swallowed**: nothing in the script discards output — `git grep -c 'dev/null' -- scripts/check-suite-state.sh`
returns no match — and `before=$(state_digest)` captures stdout only, so `shasum: <dir>: Is a directory`
reaches the operator's terminal once per directory. (Claim only what that grep tests. The script *does*
redirect — `git grep -c '>&2' -- scripts/check-suite-state.sh` counts the FAIL diagnostic block, every
line of it deliberately on stderr.) Under the script's own `set -euo pipefail` it does not get that far
— `xargs` exits non-zero, `pipefail` propagates it, and the `before=$(...)` assignment fails, measured
by running the assignment under the same shell options.

The parenthesised form is the quieter refutation: directories go to stderr, only file hashes reach
stdout, and the digest is **byte-identical to the shipped walk** in every fixture state. It is not that
it fails to notice the added directory; it changes nothing whatsoever. That is a sharper claim than
"no change in either direction", and it is the one to assert.

**A comment naming a blind spot is not a mitigation.** `CLAUDE.md` already carries the general form —
a gate step that certifies itself in text is the failure this project has measured ten ways — and a
script that documents its own gap is the same shape one level down.

## Routed here at the 2026-09-08 re-verification

Two items from the candidates files land on this spec's subject, and v1.1.0's audit round 2 added a
third. They are recorded in this sidecar; the spec body is not edited.

- **Two spellings of one path take two locks.** `LockFilePath` in `internal/engine/lock.go` resolves
  its target with `filepath.Abs` alone (line 140 at `dd89c566`) and never `filepath.EvalSymlinks`, so
  a symlinked task-file path and its physical path produce two different lock files and two writers
  proceed at once. No production file calls `EvalSymlinks`: all **fifteen** call sites under
  `internal/` are in `_test.go` files, which resolve their temp directory before exercising the lock
  — `internal/engine/runlock_test.go`, `internal/cli/lock_timeout_test.go`,
  `internal/cli/init_lock_test.go` and `import_lock_test.go` among them. A workaround that every test
  performs and no caller does is the shape this spec exists to close. Source:
  `spec/0.35.0-candidates.md` item 9.
- **`runResult` is declared twice.** `internal/cli/init_lock_test.go` and
  `internal/cli/import_lock_test.go` each declare a function-local `type runResult struct { stderr
  string; code int }` — byte-identical bodies, legal because the scope is the test function, and two
  copies that can drift apart. Source:
  `spec/0.35.0-candidates.md` item 11.
- **`tp audit --merge -o` follows a symlink out of the round directory; the review twin no longer
  does.** `internal/cli/audit_merge.go:119` writes the merged rows with a single `os.WriteFile`, which
  follows a symlink at the `-o` path. `tp review --merge` goes through `writeMergeOutput`
  (`internal/cli/merge_inputs.go:87`), which creates a temporary beside `-o` and renames over it, so
  the link is replaced rather than followed — a side effect of the shape adopted for §5 row 10's
  declined write, not a symlink guard anyone specified. Measured at `c75e5c3d` on the same fixture
  built twice — a round directory holding `o.ndjson -> ../outside/target.txt` and one valid input:
  `tp audit --merge a.ndjson -o o.ndjson` exits 0 with `target.txt` overwritten by the merged row and
  `o.ndjson` still a symlink; `tp review --merge a.ndjson -o o.ndjson` exits 0 with `target.txt`
  unchanged and `o.ndjson` now a regular file. Pre-existing on both sides at v1.0.1 and out of v1.1.0's
  scope, since §4 fences that release out of the audit phase. Source: v1.1.0 audit round 2.
