# tp — Two advisories

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `two-advisories-measurements.md` beside it; this file stands without them.

Class: **tool** — two stderr lines and the predicates behind them; nothing in the review or audit
loop changes. Budget it at the tool-class median in `CLAUDE.md`'s *What a cycle costs* table.

## 1. Overview

Two rules from `CLAUDE.md`, each mechanized as one advisory. *Dogfood the in-progress binary, never
the lagging release* (§2): tp emits one advisory per invocation when the binary running inside a tp
repository is not the one the working tree would build. *A converged cycle's task file is the record
of its closure evidence, and tp knows whether git can recover it* (§3): tp emits one advisory, once,
at the release phase, when the resolved task file is untracked — naming what would be lost rather
than what is true. Both are advisories in the same sense: stderr, silenced by `--quiet`, never an
error, never an exit-code change, nothing written or moved.

They were specified as two releases and kept apart by cadence alone — per invocation against once at
release — and a release's fixed round overhead makes cadence the wrong seam: two review-and-audit
cycles for two predicates that share a channel, a silencing flag and a non-goal list is the cost of a
seam that buys nothing, so they ship as one release with two sections and two cadences.

## 2. The binary advisory

**The rule being mechanized:** *dogfood the in-progress binary, never the lagging release.*
`CLAUDE.md` has carried it since **v0.24.0** — `git log -S 'Dogfood the in-progress binary' -- CLAUDE.md`
gives one commit, `3e1a3f0a`, whose first containing tag is v0.24.0 and which `git show v0.23.0:CLAUDE.md`
does not have. It is violated by anyone who forgets to rebuild, and the violation is silent — a stale
binary produces confident output about behaviour it does not have.

### 2.1 The version comparison is the wrong predicate

The original specification compared the running binary's version against *the newest spec version
under development*. Prototyped on this repository, that predicate is wrong in both directions: a
`HEAD` build reports the last tag with the patch incremented plus a pseudo-version suffix, so the
advisory fires on exactly the binary the rule asks for; and two builds many commits apart carry the
same comparable number once the suffix is stripped, though their `vcs.revision` values differ. The
discriminator is the commit, and it is already embedded: `debug.ReadBuildInfo()` — which
`internal/cli/root.go:56` already calls, inside an `if version == "dev"` guard the advisory cannot
share — carries `vcs.revision` and `vcs.modified`, and both survive the `-ldflags="-s -w"` production
build. The transcripts are in the sidecar under "The version comparison is the wrong predicate".

### 2.2 The predicate

Inside a git repository that contains tp's own module, four states, decided in order:

| state | condition | advisory |
|---|---|---|
| **release** | no `vcs.revision`, and `Main.Version` is a real version | names that version and the rebuild command — this is a `go install`ed binary, not a build of this tree |
| **unattributable** | no `vcs.revision`, and `Main.Version` is `(devel)` | **nothing** |
| **stale** | `vcs.revision` present and ≠ `HEAD` | names both revisions and the rebuild command |
| **current** | `vcs.revision` == `HEAD` | **nothing** |

**A missing `vcs.revision` does not by itself mean a release**, which is why the second row exists.
Probed three ways in a copy of the tree outside the repository, reading each with
`go version -m <binary>`:

| build | `vcs.revision` | `Main.Version` | `tp --version` |
|---|---|---|---|
| `go install github.com/deligoez/tp/cmd/tp@v1.0.0` | absent | `v1.0.0` | `v1.0.0` |
| `go build -buildvcs=false ./cmd/tp`, inside the checkout | absent | `(devel)` | `dev` |
| `go build ./cmd/tp` in a copy with `.git` removed | absent | `(devel)` | `dev` |

The last two **are** builds of this tree, so the release advisory would assert the opposite of the
truth about them, and there is no installed version for it to name — `root.go:56` rejects `(devel)`,
which is why they report `dev`. `Main.Version` is the second signal that separates the three, and it
sits in the same `BuildInfo` as the revision. tp cannot tell whether such a build is current or
stale, so the honest advisory for that state is none.

**Both revisions are truncated by tp for display.** `vcs.revision` is the full 40-character sha
(`build vcs.revision=65d0b09f280084b9b6b4bb4588ef217c12701419` on a probed binary), not a short one.

**`vcs.modified` is not a trigger.** A dirty working tree is the normal state of development; the
binary is still the one this tree builds, up to uncommitted edits. A build from a dirty tree records
`vcs.modified=true` beside a `vcs.revision` equal to `HEAD`, so the flag is separable from staleness
and triggering on it would fire on every invocation from a tree with uncommitted edits.

**Commit distance is not computed, and the reason is that it changes no advice** — the rebuild
command is the same one commit behind or eleven. The cost argument is weaker than it looks and should
not be leaned on: the predicate above already needs `HEAD` on every invocation, and tp has no
in-process git (`grep -rn 'exec.Command("git"' --include='*.go' internal/ cmd/ | grep -v _test.go |
grep -v testdata | wc -l` returns 16, every one a subprocess), so a distance costs a *second* call,
not the first.

**A repository that is not tp sees nothing.** The gate compares the module path from `BuildInfo`
against the module path in the repository's own `go.mod`. Only the first side is already in hand: tp
reads no `go.mod` anywhere today (`grep -rn 'go\.mod' --include='*.go' internal/ cmd/ | grep -v
_test.go` returns nothing), so the repository side is a new input rather than a reuse. It is cheap
and exact, and it is still not a heuristic.

### 2.3 Not the plugin's version check, and the two must not be merged

`hooks/session-start.sh` compares `tp --version` against `.claude-plugin/plugin.json`'s version, runs
in `SessionStart`, and **fails** (exit 2, via `fail()`). This one compares the binary's *commit*
against the tree's `HEAD`, runs on any invocation inside the repository, and **advises**.

**They disagree on the case that matters, and that is why both exist.** A freshly installed release
developing the next version's spec passes the hook and trips this advisory: a `go install`ed binary
at the manifest's version is not below it, so the hook's `version_below` is silent — while
`go version -m` on that binary prints the module line and `-buildmode=exe` and no `vcs` setting at
all, which is §2.2's release state. §2.2 compares two revision strings for equality, so nothing here
is ordered and this release introduces no version comparison.

## 3. The untracked-task-file advisory

**The rule being mechanized:** *a converged cycle's task file is the record of its closure evidence,
and tp knows whether git can recover it.*

A field cycle lost one. **What follows is the operator's report about a repository this cycle cannot
reach; it is what motivated the advisory, and it is testimony rather than measurement — the sidecar's
"What is testimony" is the ledger.** That project names a task file `spec/upcoming-<name>.tasks.json`
until release and renames it at the tag, and its `.gitignore` carries `spec/upcoming-*.tasks.json` —
so before the rename the file exists **only on disk**. It was deleted mid-session on a wrong scope
note, had never been tracked, and is not in git history. Fourteen tasks' closure evidence and commit
SHAs are unrecoverable.

**The naming convention is not tp's and is not adopted here.** tp ships no `upcoming-` prefix and no
such `.gitignore` line; a check keyed on the prefix would help only projects that share it. What
generalises is the loss: every command that reads task state resolves the active task file's path, and
whether git tracks that path is one `git ls-files` away.

**Not *every* invocation.** `root.go`'s `PersistentPreRun` does no discovery, and `tp lint spec.md`
and `tp ground spec.md --units` both exit 0 in a directory holding no task file (`tp status` in the
same directory exits 3). The commands that do resolve one are
`git grep -ln DiscoverTaskFile -- 'internal/cli/*.go' | grep -v _test`. This costs the release
nothing: `DetectPhase` computes `release` from a task count, so the phase this advisory keys on
cannot be reached without a task file having been read.

### 3.1 The advisory names the loss, not the state

```
task file spec/upcoming-x.tasks.json is untracked — 14 closed tasks and 27 commit SHAs exist only on disk
```

The first draft specified this as a state advisory, one per invocation, the twin of the binary
advisory. The operator who lost the file refuted it. **The attribution is testimony** — the
blockquote has no corroborating artifact here — and what follows it does not depend on the
attribution:

> I did not lose the file because I was confused about whether git had it. I executed my own wrong
> scope note saying to delete it. "Task file is untracked" does not contradict "this file should be
> deleted" — if anything it *supports* it. An untracked file reads as disposable.

**The two propositions are orthogonal**, so a state advisory can fire correctly, be read correctly,
and change nothing. That is a claim about two sentences and is checkable by reading them; it does not
rest on the recollection.

**A count contradicts the deletion where a state does not.** *Fourteen tasks and twenty-seven SHAs
exist only here* is incompatible with *this file is disposable*, and it is the same fact stated in the
units the loss is measured in.

### 3.2 Once, at the release phase

The advisory fires when the **computed phase** is `release` — whether that phase is reported by
`tp resume` or reached by `tp run`'s own oracle, which never invokes the resume command (`readCycle`
calls `AssembleResume` directly, `internal/engine/driver.go:261`). The field report places that
operator at the release step when the file went (testimony).

**It does not fire "when the audit converges".** `DetectPhase` (`internal/engine/phase.go:28-41`)
tests `numDone < numTasks` and returns `implement` *before* it consults `auditConverged`, and with
zero tasks it never consults it at all. Convergence is therefore **necessary for `release` and not
sufficient**. Probed by `go run` over `engine.DetectPhase` in an rsync copy outside the repository:
`(2,1,false,true)` → `implement`, `(2,2,false,true)` → `release`, `(0,0,false,true)` → `review`.
Keyed on the convergence signal, the advisory would fire during `implement`, on every invocation —
which is exactly the habituation this section exists to prevent. Both routes key on the phase.

**Not per invocation.** The cadence is **this document's argument, not part of §3.1's quotation** —
the blockquote is entirely about state-versus-count and contains no sentence about cadence,
per-invocation warnings or habituation; "one per invocation" is this spec's own description of the
first draft. The argument stands on its own: that session ran tp dozens of times (testimony), and
habituation to a per-invocation warning would have been complete long before the moment it mattered.
A per-invocation warning is right for a condition that is true all session and cheap to ignore
correctly — which is what makes it right for §2 — and it is the failure mode for a single
irreversible moment.

**Every input exists at that moment.** `engine.PhaseRelease` (`internal/engine/phase.go:9`) is the
value compared at `internal/engine/driver.go:243`, and the resolved task file carries the closed-task
count and `commit_shas`, so the sentence has real numbers to put in, not placeholders. §5's note on
rows 9 and 10 derives them from this repository's own v0.36.0 task file and says why that file cannot
be a counting fixture.

**No new capability is proposed — but the predicate is new code.** tp already shells to git from
production: `internal/cli/config_extract.go:290` is
`exec.Command("git", "-C", dir, "status", "--porcelain")`, inside `gitWorkingTreeDirty`. The predicate
this advisory needs is not there — `git grep -rn "ls-files" -- internal/ cmd/` returns three hits, all
in `internal/cli/lock_pathspec_test.go`, so `git ls-files` runs nowhere in production. What is new is
a call, not a capability; what is newly *useful* is that the answer reaches the operator before the
file is gone rather than after. (`internal/cli/report.go`'s `untrackedCount` is **not** prior art for
this and must not be reused: it is a count of *done tasks with no measurable duration*, emitted as
`summary.untracked`, and nothing on that path touches git. The name collides; the subject does not.)

**Still an advisory.** Silenced by `--quiet`, never an error, never a refusal, no exit code changes.

## 4. Non-Goals

1. **Never an error.** No exit code changes, in any command, under any condition, from either
   advisory.
2. **No rebuild, nothing written or moved.** tp names the rebuild command and names the loss; running
   the one and recovering the file are the operator's.
3. **No merge with the plugin's version check.** §2.3 states why; merging makes one of the two wrong
   for its own case.
4. **No version comparison anywhere in this release.** §2.2 compares two revision strings for
   equality.
5. **No binary advisory outside tp's own repository.** The generalisation to any project developing a
   tool with tp is real and is not taken here: it needs a way to know which binary a project is
   dogfooding, which tp does not have and this release does not invent.
6. **The binary advisory is not once per session — once per invocation.** tp holds no session identity
   (`grep -rniE 'sessionID|session_id|TP_SESSION' --include='*.go' internal/ cmd/ | grep -v _test.go`
   returns nothing), and inventing one to deduplicate an advisory is a larger mechanism than the
   advisory.
7. **No release-time rename command.** The same field report asked for one. It is a workflow verb that
   would bake one project's naming convention into the tool, and the reporter agreed on re-reading,
   ranking it *"clearly below the reframed advisory"*. If it is ever wanted, the shape that avoids the
   objection takes its destination from the caller — `tp mv <spec> <path>` doing a `git mv` plus a
   post-check that the file is now tracked, so tp supplies the guarantee and no convention.
8. **No `.gitignore` inspection, no prefix heuristic.** The predicate is `git ls-files` on the resolved
   path. *Why* a file is untracked does not change what is lost.
9. **No refusal to converge, and no gate.** A cycle whose task file is untracked still converges,
   ships and reports. The advisory informs a decision; it does not take it.
10. **Not one cadence.** The two advisories share a channel and a silencing flag and not a trigger:
    §2 fires per invocation and §3 once at release, and §3.2 is why the second cannot borrow the
    first's.

## 5. Tests

Every row derives from a numbered decision and names a mutant that must fail it. The `from` cell names
the decision, not the fixture; where a row needs a particular fixture, the assertion names it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 | a binary whose `vcs.revision` equals `HEAD` emits **nothing**, while a spec numbered above the pseudo-version that binary reports is under development | the version predicate, which fires here — this is the false positive, and it must be watched failing first |
| 2 | §2.1 | two binaries whose reported versions reduce to the **same** comparable number but carry different `vcs.revision` values are distinguished: one silent, one advising | a predicate comparing the numeric prefix — what the repository's shipped comparator produces, reducing both of the sidecar's binaries to `0.37.1`. The revision is present in both strings; the prefix comparison discards it |
| 3 | §2.2 *release* | `BuildInfo` with no `vcs.revision` **and** a real `Main.Version` advises, naming that version; the two builds reaching the same absence with `Main.Version` `(devel)` emit nothing. All three fixtures are in the test, not the first alone | dropping the `Main.Version` condition, which makes the advisory assert "not a build of this tree" about two builds of this tree — and, separately, treating a missing revision as a match, which silences the case §2.3 says the hook cannot catch |
| 4 | §2.2 *dirty* | `vcs.modified` true with a matching revision emits nothing | trigger on dirty — a build from a working tree records `vcs.modified=true` alongside a `vcs.revision` equal to `HEAD`, so the trigger fires on every invocation from a tree with uncommitted edits |
| 5 | §2.2 *order* | the release state is decided before the stale state, so a missing revision never reads as a mismatch against `HEAD` | compare first and check presence second, producing a stale advisory whose embedded-revision half is empty |
| 6 | §4.1 | the binary advisory changes no exit code — asserted across a command set, not one command | assert on `tp audit` alone, which leaves every sibling free to regress |
| 7 | §4.6 | two invocations in the same shell each emit the binary advisory | deduplicate via a file or environment marker, which is the session identity §4.6 declines to invent |
| 8 | §3.1 | the advisory names the closed-task count and the SHA total, both non-zero, matching the resolved task file | emit the state sentence — the refuted form, which passes any test asserting only that an advisory appeared |
| 9 | §3.1 *counts* | the counting is asserted **directly, on a synthesized fixture whose two pairs differ** — more tasks than closed tasks, and more `commit_shas` entries than non-null `commit_sha` fields | count all tasks rather than closed ones; and, separately, count `commit_sha` rather than `commit_shas` — both understate the loss, and both survive any fixture where the pairs coincide |
| 10 | §3.2 | it fires at `PhaseRelease` and **not** on the invocations before it — asserted by running a sequence, not one call | fire per invocation, which is the habituation failure §3.2 exists to avoid |
| 11 | §3.2 *route* | it fires on a `release`-phase cycle driven by `tp run`, which never invokes the resume command | key it on the `resume` command, so a `tp run` cycle never sees it; **and** key it on `auditConverged` instead of the computed phase, which fires on an `implement` cycle whose audit has converged |
| 12 | §3 *tracked* | a tracked task file produces nothing, whatever its `.gitignore` says | test `.gitignore` rather than `git ls-files`, which reports a tracked-then-ignored file as at risk — the mutant must be the *pattern-only* form (`git check-ignore --no-index`), because index-aware `check-ignore` agrees with `ls-files` on that file and would not fail the row |
| 13 | §4.9 | convergence, exit codes and `--check` are unchanged by the task-file advisory's presence | let it set an exit code, turning an advisory into a gate |
| 14 | §3.2 *quiet* | `--quiet` silences it and the phase still reports release | route it to stdout, where it corrupts the JSON payload a driver parses |

### 5.1 Notes on rows 1, 2, 8, 9 and 10

**Row 1 is the binary advisory's acceptance, and its fixture is the sidecar's first transcript** — a
binary built from `HEAD` inside this repository while a spec numbered above the pseudo-version that
build reports is under development. That is the exact configuration the original predicate flags and
the rule asks for. The qualifier is load-bearing: because Go increments the patch, a spec numbered at
or below the last release is *below* what a `HEAD` build reports and would not trip the refuted
predicate at all.

**Row 2's fixture is buildable and its cheaper form is not.** The two binaries do not report the same
version string — a VCS-stamped build embeds the revision, so two such builds of different commits
cannot. They report the same *comparable number*, which is what a version predicate sees. **The
restriction to VCS-stamped builds is not decoration.** Ground round 2 built the counterexample:
`git checkout <rev> && go build -buildvcs=false` at both revisions, in a clone, and **both print
`tp version dev`** — identical strings from different commits. Stated without the qualifier the
sentence contradicts §2.2's own rows 2 and 3, which are about exactly those builds.

**Rows 9 and 10 need different fixtures, and that is not an accident.** Row 9's fixture must hold an
open task, so that counting all tasks and counting closed ones give different answers; a cycle with an
open task is `implement`, not `release`, so the advisory does not fire on it at all. Row 9 therefore
asserts the counting function, not the emitted sentence, and row 10 asserts the cadence on a fully
closed cycle. The repository's own v0.36.0 task file is the counterexample that forces the split:

```
python3 -c 'import json; ts=json.load(open("spec/0.36.0.tasks.json"))["tasks"]; print(
  len(ts),
  sum(1 for t in ts if t.get("status") == "done"),
  len([s for t in ts for s in (t.get("commit_shas") or [])]),
  sum(1 for t in ts if t.get("commit_sha")))'
# tasks, closed, commit_shas, non-null commit_sha → 24 24 47 24
```

Counting all tasks and counting closed tasks both give 24 there, and counting `commit_sha` and
`commit_shas` differ, so the first mutant could not fail and the second could.

**Row 8 is the task-file advisory's acceptance and row 10 is the one that would have been skipped.**
A test that asserts "an advisory was emitted" passes under the refuted design; only asserting the
*numbers* and the *cadence* separates the two forms, and the refutation is precisely that the two
forms are not interchangeable.
