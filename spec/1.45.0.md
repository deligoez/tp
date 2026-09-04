# tp v1.45.0 — The untracked task file

> **This file is decisions.** Its central decision was rejected in its first form by the operator the
> defect happened to. **That account is testimony and nothing here corroborates it** —
> `git log --oneline -- spec/1.45.0.md` returns a single commit, so no earlier draft exists to compare
> against, and the incident appears nowhere else in `spec/`, `skills/` or `CLAUDE.md`. §2.1 quotes the
> refutation because the rejected form is the one an implementer would otherwise write, and then gives
> the argument that does **not** rest on the testimony — a relation between two sentences, checkable
> by reading them. §5 is the ledger of what is reported and what is checked.

## 1. Overview

**The rule being mechanized:** *a converged cycle's task file is the record of its closure evidence,
and tp knows whether git can recover it.*

A field cycle lost one. **What follows is the operator's report about a repository this cycle cannot
reach; it is what motivated the release, and it is testimony rather than measurement (§5).** That
project names a task file `spec/upcoming-<name>.tasks.json` until release and renames it at the tag,
and its `.gitignore` carries `spec/upcoming-*.tasks.json` — so before the rename the file exists
**only on disk**. It was deleted mid-session on a wrong scope note, had never been tracked, and is not
in git history. Fourteen tasks' closure evidence and commit SHAs are unrecoverable.

tp emits **one advisory, once, at the release phase**, when the resolved task file is untracked —
naming what would be lost rather than what is true.

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

## 2. The advisory names the loss, not the state

```
task file spec/upcoming-x.tasks.json is untracked — 14 closed tasks and 27 commit SHAs exist only on disk
```

### 2.1 Why not "the task file is untracked"

The first draft specified this as a state advisory, one per invocation, the twin of the binary check.
The operator who lost the file refuted it. **The attribution is testimony** — the blockquote has no
corroborating artifact here (§5) — and what follows it does not depend on the attribution:

> I did not lose the file because I was confused about whether git had it. I executed my own wrong
> scope note saying to delete it. "Task file is untracked" does not contradict "this file should be
> deleted" — if anything it *supports* it. An untracked file reads as disposable.

**The two propositions are orthogonal**, so a state advisory can fire correctly, be read correctly,
and change nothing. That is a claim about two sentences and is checkable by reading them; it does not
rest on the recollection.

**A count contradicts the deletion where a state does not.** *Fourteen tasks and twenty-seven SHAs
exist only here* is incompatible with *this file is disposable*, and it is the same fact stated in the
units the loss is measured in.

## 3. Once, at the release phase

The advisory fires when the **computed phase** is `release` — whether that phase is reported by
`tp resume` or reached by `tp run`'s own oracle, which never invokes the resume command (`readCycle`
calls `AssembleResume` directly, `internal/engine/driver.go:256-261`). The field report places that
operator at the release step when the file went (testimony, §5).

**It does not fire "when the audit converges".** `DetectPhase` (`internal/engine/phase.go:28-41`)
tests `numDone < numTasks` and returns `implement` *before* it consults `auditConverged`, and with
zero tasks it never consults it at all. Convergence is therefore **necessary for `release` and not
sufficient**. Probed by `go run` over `engine.DetectPhase` in an rsync copy outside the repository:
`(2,1,false,true)` → `implement`, `(2,2,false,true)` → `release`, `(0,0,false,true)` → `review`.
Keyed on the convergence signal, the advisory would fire during `implement`, on every invocation —
which is exactly the habituation this section exists to prevent. Both routes key on the phase.

**Not per invocation.** The cadence is **this document's argument, not part of §2.1's quotation** —
the blockquote is entirely about state-versus-count and contains no sentence about cadence,
per-invocation warnings or habituation; "one per invocation" is this spec's own description of the
first draft. The argument stands on its own: that session ran tp dozens of times (testimony, §5), and
habituation to a per-invocation warning would have been complete long before the moment it mattered.
A per-invocation warning is right for a condition that is true all session and cheap to ignore
correctly, and it is the failure mode for a single irreversible moment.

**Every input exists at that moment.** `engine.PhaseRelease` (`internal/engine/phase.go:9`) is the
value compared at `internal/engine/driver.go:243`, and the resolved task file carries the closed-task
count and `commit_shas`. On this repository's own v0.36.0 task file:

```
python3 -c 'import json; ts=json.load(open("spec/0.36.0.tasks.json"))["tasks"]; print(
  len(ts),
  sum(1 for t in ts if t.get("status") == "done"),
  len([s for t in ts for s in (t.get("commit_shas") or [])]),
  sum(1 for t in ts if t.get("commit_sha")))'
# tasks, closed, commit_shas, non-null commit_sha → 24 24 47 24
```

So the sentence has real numbers to put in, not placeholders. Note the first two figures and the last
two: they are why that file cannot be §6 row 2's fixture.

**No new capability is proposed — but the predicate is new code.** tp already shells to git from
production: `internal/cli/config_extract.go:290` is
`exec.Command("git", "-C", dir, "status", "--porcelain")`, inside `gitWorkingTreeDirty`. The predicate
this release needs is not there — `git grep -rn "ls-files" -- internal/ cmd/` returns three hits, all
in `internal/cli/lock_pathspec_test.go`, so `git ls-files` runs nowhere in production. What is new is
a call, not a capability; what is newly *useful* is that the answer reaches the operator before the
file is gone rather than after. (`internal/cli/report.go`'s `untrackedCount` is **not** prior art for
this and must not be reused: it is a count of *done tasks with no measurable duration*, emitted as
`summary.untracked`, and nothing on that path touches git. The name collides; the subject does not.)

**Still an advisory.** Silenced by `--quiet`, never an error, never a refusal, no exit code changes.

## 4. Non-Goals

1. **No release-time rename command.** The same field report asked for one. It is a workflow verb that
   would bake one project's naming convention into the tool, and the reporter agreed on re-reading,
   ranking it *"clearly below the reframed advisory"*. If it is ever wanted, the shape that avoids the
   objection takes its destination from the caller — `tp mv <spec> <path>` doing a `git mv` plus a
   post-check that the file is now tracked, so tp supplies the guarantee and no convention.
2. **No `.gitignore` inspection, no prefix heuristic.** The predicate is `git ls-files` on the resolved
   path. *Why* a file is untracked does not change what is lost.
3. **No merge with the binary check.** They share a subject — a fact about the repository tp can see —
   and not a cadence: that release emits *"one advisory per invocation"* (its §1, and its Non-Goal 6,
   *"Not once per session — once per invocation"*), against this one's once-at-release. §3 is why.
   The refusal is one-sided: that release's own no-merge non-goal is about the plugin's version check
   and says nothing about this advisory, so nothing there is being echoed here.
4. **No refusal to converge, and no gate.** A cycle whose task file is untracked still converges,
   ships and reports. The advisory informs a decision; it does not take it.
5. **Nothing is written or moved.** tp reports; recovering the file is the operator's.

## 5. What is reported and what is checked

Three of this document's premises are the operator's retrospective self-report about a repository this
cycle cannot reach and a decision that cannot be rerun. Each is marked where it is used, and none is
corroborated by any artifact here:

1. **the loss itself** (§1) — the project, its naming convention, its `.gitignore` line, the deletion
   and the fourteen tasks;
2. **the attribution of §2.1's blockquote** to the operator who lost the file, and that it was a
   response to a first draft — `git log --oneline -- spec/1.45.0.md` returns one commit, so there is
   no draft-and-response pair in history;
3. **"that session ran tp dozens of times"** (§3), the premise under the habituation argument.

They are recorded as testimony, not measurement, and they are what motivated the release rather than
what it rests on.

**What does not depend on any of them.** The orthogonality argument in §2.1 is a claim about two
propositions, checkable by reading them. §3's trigger is a fact about `DetectPhase`. Non-Goal 2's
predicate is a fact about `git ls-files`. Every row in §6 is a fact about a mechanism a test can run.

The stronger assertion, that the reframed sentence *would* have prevented the deletion, is
unfalsifiable and is nowhere relied on. A test can show the advisory fires with the right numbers at
the right moment; no test can show it would have been obeyed.

## 6. Tests

Every row derives from a numbered decision and names a mutant that must fail it. The `from` cell names
the decision, not the fixture; where a row needs a particular fixture, the assertion names it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | the advisory names the closed-task count and the SHA total, both non-zero, matching the resolved task file | emit the state sentence — the refuted form, which passes any test asserting only that an advisory appeared |
| 2 | §2 *counts* | the counting is asserted **directly, on a synthesized fixture whose two pairs differ** — more tasks than closed tasks, and more `commit_shas` entries than non-null `commit_sha` fields | count all tasks rather than closed ones; and, separately, count `commit_sha` rather than `commit_shas` — both understate the loss, and both survive any fixture where the pairs coincide |
| 3 | §3 | it fires at `PhaseRelease` and **not** on the invocations before it — asserted by running a sequence, not one call | fire per invocation, which is the habituation failure §3 exists to avoid |
| 4 | §3 *route* | it fires on a `release`-phase cycle driven by `tp run`, which never invokes the resume command | key it on the `resume` command, so a `tp run` cycle never sees it; **and** key it on `auditConverged` instead of the computed phase, which fires on an `implement` cycle whose audit has converged |
| 5 | §1 *tracked* | a tracked task file produces nothing, whatever its `.gitignore` says | test `.gitignore` rather than `git ls-files`, which reports a tracked-then-ignored file as at risk — the mutant must be the *pattern-only* form (`git check-ignore --no-index`), because index-aware `check-ignore` agrees with `ls-files` on that file and would not fail the row |
| 6 | §4.4 | convergence, exit codes and `--check` are unchanged by the advisory's presence | let it set an exit code, turning an advisory into a gate |
| 7 | §3 *quiet* | `--quiet` silences it and the phase still reports release | route it to stdout, where it corrupts the JSON payload a driver parses |

**Rows 2 and 3 need different fixtures, and that is not an accident.** Row 2's fixture must hold an
open task, so that counting all tasks and counting closed ones give different answers; a cycle with an
open task is `implement`, not `release`, so the advisory does not fire on it at all. Row 2 therefore
asserts the counting function, not the emitted sentence, and row 3 asserts the cadence on a fully
closed cycle. The repository's own v0.36.0 task file is the counterexample that forces the split: the
command in §3 prints `24 24 47 24`, so counting all tasks and counting closed tasks both give 24 there
and the first mutant could not fail.

**Row 1 is the acceptance and row 3 is the one that would have been skipped.** A test that asserts
"an advisory was emitted" passes under the refuted design; only asserting the *numbers* and the
*cadence* separates the two forms, and the refutation is precisely that the two forms are not
interchangeable.
