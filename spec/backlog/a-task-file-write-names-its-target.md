# tp — A task-file write names its target

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-task-file-write-names-its-target-measurements.md` beside it, and this file
stands without them. It takes the untracked-task-file advisory from `two-advisories.md`, now a
forwarding stub, and the task-file items of a field report (WB-3155) verified on 2026-09-11; the
sidecar says which parts of that report were refuted.

Class: **tool** — it changes what the task-file commands print, accept and refuse; nothing in the
review, audit or ground loop changes. Budget it at the tool-class median in `CLAUDE.md`'s *What a
cycle costs*.

## 1. The decision

**Context.** A command that writes a task file does not say which one. Where auto-detection finds
several candidates tp already refuses at exit 3; where the choice is unambiguous to tp and not to the
agent, it writes silently. The case that costs work is the active-file pointer an earlier `tp use` left
behind (git-ignored, under `.tp/`): after `tp init` of a second spec, `tp add` and `tp remove` keep writing the first spec's
file at exit 0, and neither the payload nor stderr names a file. A `tp remove` there deletes an open
task from a plan the agent was not working on. Row 1 is that fixture; the transcript is in the
sidecar under *#22*. The same session exposes four smaller gaps with the same subject — what the
agent believes it wrote against what tp wrote: a claimed task cannot be given back, `tp next --peek`
previews a different task than `tp next` returns, a batch close refuses the multi-commit form the CLI
accepts, and a bullet list of acceptance criteria counts more criteria than it has bullets.

**Decision.**

1. Every command that writes a task file it found by discovery names that file in its payload, and
   says on stderr when the file was chosen by the pointer or by a current-directory file shadowing
   another, while a second task file sits nearby (§2).
2. `tp unclaim` gives a claim back, and `tp next --peek` previews what `tp next` would return (§3).
3. A batch close accepts what `--commit` accepts, a bullet is one criterion, and tp reports the
   criterion count it parsed when the task is written (§4).
4. `tp validate`'s atomicity, anchor and coverage findings name a repair that runs (§5).
5. At the release phase, an untracked task file is reported with what would be lost (§6).

**Consequences.** Every task-file write gains one payload key; stderr gains a line only where more
than one task file is in reach. Discovery order does not change, so no command targets a different
file than it does today — the release makes the choice visible, not different. Acceptance lists whose
bullets contain `. ` or `; ` count fewer criteria afterwards, so closing such a task needs fewer
evidence lines; closed tasks are unaffected, since §5 stops validating them for atomicity.

**Alternatives.** Refusing the write whenever the pointer resolves and another task file exists was
the field report's first option. It is rejected because the pointer exists for exactly that
directory: refusing there makes `tp use` useless in the one case it was built for, and the refusal
the report asked for is what auto-detection already does. Having `tp init` or `tp import` move the
pointer is rejected because the pointer is per-checkout state another agent in the same checkout
may rely on; tp warns and names the `tp use` command instead. Both, and the smaller alternatives
below, are argued in the sidecar under *Alternatives*.

## 2. A write names its target

**The payload carries `file`.** Every command that writes a task file it resolved through discovery
puts the path it wrote under a top-level `file` key, relative to the directory the command ran in, so
the value works as `--file`'s argument from there. The set of such commands is derived in the sidecar
under *The write set*, and the implementing task re-derives it. `--record` and `tp set --workflow`
carry the same key under `a-findings-exits-agree` §6, which this spec does not re-specify.

**A notice says how the file was chosen.** When the file came from the `tp use` pointer, or
from a current-directory file that stopped auto-detection before a subdirectory's, and another
`*.tasks.json` exists in the written file's directory or where auto-detection looks from the current
directory, the write emits one notice on stderr naming the file written, the rule that chose it, and
the other candidates. A file named by `--file` or `TP_FILE`, or the only task file in reach, gets no
notice: the notice is for the case the agent may not know about, and one that fires on every write
teaches the agent to skip it. It is a notice in the sense `an-unreadable-file-is-named` settles —
stderr only, silenced by `--quiet`, no exit code; the payload key is what survives `--quiet`.

**Reads get no notice.** A read cannot lose work, and the next write names its file before anything
is lost.

**`tp init` and `tp import` warn when the pointer names another file.** When either writes a task
file and the pointer names a different task file that exists, it emits one notice naming both and the
`tp use` command that would switch. It does not move the pointer.

**`tp add` in an ambiguous directory names the candidates.** Without `--spec`, where auto-detection
finds several task files, `tp add` refuses at exit 3 with the candidate list auto-detection's own
refusal already prints, instead of saying no task file was found.

## 3. A claim can be given back, and a preview agrees with the claim

**`tp unclaim <id> [id…]` returns a `wip` task to `open`** and clears what the claim wrote —
`started_at` and `duration_source` — so the next claim times the work afresh. Several ids follow
`tp claim`'s batch contract. A task that is not `wip` is refused at exit 4: a `done` task's hint
names `tp reopen`, which also clears the closure fields. Today `tp remove` and `tp reopen` refuse a
`wip` task and `tp set` refuses to write its status or `started_at` (sidecar, *#24*), so the exit the
field report found was to close the task with evidence for work nobody did.

**The command is not named `release`**, the field report's name, because `release` is the phase `tp
resume` reports after a converged audit and an agent could read the phase as the next command. It
requires an id, so a bare `tp unclaim` is a usage error at exit 2 that changes nothing.

**A hint that names a verb names the right one.** `tp remove` on a `wip` task names `tp unclaim`,
and on a `done` task `tp reopen`; `tp reopen` on a `wip` task names `tp unclaim`. `tp remove --force`
stays open-only: it deletes, and giving a claim back is not a deletion.

**`tp next --peek` previews what `tp next` would return.** Plain `tp next` resumes a `wip` task
before it claims a ready one; `--peek` today skips the `wip` task and shows the next ready one
(row 12). After this release `--peek` returns the task plain `tp next` would return, `wip` included,
and writes nothing. Plain `tp next` still claims.

## 4. What a batch close and an acceptance list accept

### 4.1 A batch `commit` is what `--commit` is

A `tp done --batch` entry's `commit` takes a string or an array, recorded as the same shas passed to
repeated `--commit` flags would be. An entry carrying both `commit` and `commit_shas` is refused as
an entry failure naming both keys, because one of them is silently dropped today (row 14). A line
that fails to parse is reported with its line number in the file and a hint naming the entry's
fields, not the task-file hint an exit-3 error without its own hint falls back to; `tp import`'s
exit-3 refusals whose cause is not the task file's location name their cause the same way.
`skills/tp/REFERENCE.md`'s NDJSON field list names `commit_shas`.

### 4.2 A bullet is one criterion

When acceptance is a bullet list, each bullet is one criterion, whatever `. ` or `; ` it contains; a
JSON-array acceptance is a bullet list. Prose acceptance keeps splitting on `. ` and `; `. The one
count drives atomicity, closure and the report: `tp add` and `tp import` return the parsed criterion
count of every task they wrote, so the number an author will be held to at closure is visible when
the task is written rather than when it is closed. `skills/tp/REFERENCE.md`'s delimiter table states
the bullet rule and that the delimiters do not compound.

## 5. `tp validate` names a repair that runs

One task, three findings. The field report's causes for all three were refuted (sidecar, *#11*,
*#12*, *#13*); what survives is that each finding points at nothing the agent can run.

- **Atomicity skips `done` tasks.** A closed task cannot be split, so a finding on it — and a
  `--strict` failure from it — asks for work that has no subject. `open` and `wip` tasks keep it.
- **An anchor whose section number matches one heading suggests that heading.** When an unresolved
  `source_sections` entry's section number equals the number of exactly one heading, the finding
  suggests that heading verbatim and names `tp set --bulk` as the one-call repair across tasks. No
  match, or several, adds nothing.
- **The coverage-count error names its repair.** The finding that the stored section count disagrees
  with the spec carries a hint that, run as printed, recomputes the coverage map and changes no
  task's anchor.

## 6. An untracked task file is reported at the release phase

This is `two-advisories.md` §3's decision, carried unchanged; its testimony and derivations moved
into this spec's sidecar under *Carried from `two-advisories`*.

**It names the loss, not the state.** When the computed phase is `release` and git does not track
the resolved task file, tp emits one notice naming the file, its closed-task count and its commit-sha
total: `task file <path> is untracked — <N> closed tasks and <M> commit SHAs exist only on disk`.
*Untracked* alone does not contradict *this file can be deleted* — an untracked file reads as
disposable — while a count of closure evidence that exists nowhere else does.

**It fires at the phase, not at convergence and not per invocation.** The phase is `release` whether
`tp resume` reports it or `tp run`'s oracle reaches it. A converged audit is necessary for that phase
and not sufficient, so keying on convergence would fire during implementation; and a warning that
fires on every call has been learned as noise by the one irreversible moment it exists for.

**It is only a notice.** Silenced by `--quiet`, never on stdout, no exit code, nothing written or
moved, no refusal to converge. The predicate is whether git tracks the resolved path. Why it is
untracked — an ignore rule, a naming convention — does not change what is lost, so neither is read.

## 7. Non-Goals

1. **No change to discovery order.** `--file`, then `TP_FILE`, then the pointer, then
   auto-detection; §2 makes the choice visible and changes no target.
2. **No refusal on the pointer path** (§1, *Alternatives*).
3. **No cross-repository task scope.** The field report's request to mark a task as belonging to
   another repository is `spec/undecided.md`'s *Cross-repo specs*, closed; the field instance is in
   the sidecar and reopens nothing here.
4. **`--record`'s and `tp set --workflow`'s `file` key belong to `a-findings-exits-agree` §6.** §2's
   notice still applies to a `tp set --workflow` whose task file came from the pointer.
5. **`tp config --extract` gains no file list.** It thins every task file in the project by scan
   rather than by discovery, so there is no choice for a notice to reveal.
6. **Plain `tp next` keeps claiming**, and `tp remove --force` keeps refusing `wip` (§3).
7. **No release-time rename command.** Carried from `two-advisories` Non-Goal 7: a verb that would
   bake one project's task-file naming convention into the tool.

## 8. Tests

Every row derives from a numbered decision and names a mutant that must fail it. **Where a row's
subject exists at `HEAD`, the row states what `HEAD` does**, from the sidecar's transcripts; where it
does not — the `file` key, the notices, `tp unclaim`, the count report — there is nothing to observe
yet. The value under each mutant is confirmable only once the code lands, so Step 0.5 defers it to
the implementing task's acceptance. Row 1 is the acceptance and must be watched failing first.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *pointer* | fixture: `alpha.md` and `gamma.md` in one directory; `tp init alpha.md`, add open task `a1`, `tp use alpha.tasks.json`, `tp init gamma.md`, then `tp remove a1`. The payload's `file` is `alpha.tasks.json` and stderr carries exactly one notice naming `alpha.tasks.json` and the pointer. At `HEAD`: payload `{"removed":"a1"}`, no `file` key, 0 stderr lines, exit 0 | emit the notice only when auto-detection chose the file, never the pointer |
| 2 | §2 *payload* | every command in the write set, run once with `--file gamma.tasks.json` while the pointer names `alpha.tasks.json`, returns `file` equal to `gamma.tasks.json`, and re-reading that path shows the write | fill `file` from the pointer rather than from the path written — a constant with respect to the flag |
| 3 | §2 *form* | run from a subdirectory with the pointer at the project root, `file` is relative, and passing it back as `--file` from the same directory reaches the same task file | emit the pointer's stored value, which is relative to the project root and misses from a subdirectory |
| 4 | §2 *quiet case* | with `--file`, with `TP_FILE`, and in a project holding one task file, a write emits no notice | notice on every write, which fires on the single-spec project |
| 5 | §2 *reach* | a current-directory task file shadowing one a level down, and a pointer to a file two levels down beside a sibling task file, each produce the notice | count other candidates in the current directory only, which misses both |
| 6 | §2 *init/import* | `tp init gamma.md` and `tp import` of a gamma task document, with the pointer on `alpha.tasks.json`, each emit one notice naming both files and `tp use`; with no pointer, or the pointer on the file just written, nothing; the pointer is unchanged afterwards | warn whenever a pointer exists, which fires when it already names the new file |
| 7 | §2 *add* | `tp add` without `--spec` where auto-detection finds two task files exits 3 and names both | `HEAD`'s message, which says no task file was found |
| 8 | §3 *unclaim* | `tp unclaim t1` on a `wip` task leaves it `open` with `started_at` and `duration_source` absent; a following `tp next` claims it with a new `started_at` | set the status and keep `started_at`, so the next close times the abandoned claim |
| 9 | §3 *refusals* | `tp unclaim` on an `open` task and on a `done` task exits 4 and writes nothing, the second hint naming `tp reopen`; bare `tp unclaim` exits 2 | accept any non-`open` status, which turns a `done` task `open` with its closure fields intact |
| 10 | §3 *hints* | `tp remove` on `wip` names `tp unclaim` and on `done` names `tp reopen`; `tp reopen` on `wip` names `tp unclaim` | `HEAD`'s hint, which names `tp reopen` for every non-`open` status |
| 11 | §3 *batch* | `tp unclaim a b c` with `b` not `wip` unclaims `a` and `c` and reports `b` as a failure with its hint | abort on the first failure |
| 12 | §3 *peek* | fixture: `t1` `wip`, `t2` ready. `tp next --peek` returns `t1` and leaves the task file byte-identical; on a fixture with no `wip` task it returns the ready task and claims nothing. At `HEAD` `--peek` returns `t2` where `tp next` returns `t1` | `HEAD`'s skip of `wip` under `--peek`; and, separately, a `--peek` that delegates to plain `tp next`, which claims on the second fixture |
| 13 | §4.1 *array* | a batch entry with `commit: [A, B]` records `commit_shas` `[A, B]` in that order, as `--commit A --commit B` does and as `commit_shas: [A, B]` does. At `HEAD` the array form exits 3 with a JSON type error | keep only the array's first element |
| 14 | §4.1 *both* | an entry with `commit: A` and `commit_shas: [B]` fails as an entry naming both keys and records nothing for that task. At `HEAD` it closes with `commit_shas` `[B]` and `A` is dropped | `HEAD`'s precedence of the list over the single value |
| 15 | §4.1 *line* | a batch file whose first line is blank and whose third line fails to parse is reported as line 3, with a hint naming the entry's fields; `tp import` over a populated target gets a hint that does not say `tp init`. At `HEAD` the batch error has neither a line number nor a field hint | number non-blank lines only, which reports line 2; and the code-3 default hint |
| 16 | §4.2 *bullets* | acceptance `- A; B` / `- C. D` / `- E` counts 3 criteria in `tp validate`, in `tp done`'s closure check and in `tp add`'s count; prose `A. B; C` still counts 3. At `HEAD` the bullet form counts 5 in both validate and closure | keep splitting inside bullets; and, separately, stop splitting prose, which counts the prose form as 1 |
| 17 | §4.2 *count* | `tp add --bulk` and `tp import` return a per-task criterion count equal to what `tp validate` counts for the same task | count bullets rather than parsed criteria, which reports 1 for a prose acceptance of two sentences |
| 18 | §5 *anchor* | headings `### 1.1 Task Model` and `### 1.10 Audit`: the entry `### 1.1` suggests `### 1.1 Task Model` only, with `tp set --bulk` named; `### 9.9` suggests nothing new. At `HEAD` `### 1.1` against `### 1.1 Task Model` alone suggests nothing | match by string prefix, which also matches `### 1.10 Audit` |
| 19 | §5 *atomicity* | a `done` task with five criteria yields no atomicity finding under `tp validate` or `--strict`; the same task `wip` and `open` yields one. At `HEAD` the `done` task yields one, and `--strict` exits 1 on it | skip every non-`open` task, which silences the `wip` case |
| 20 | §5 *coverage* | after the spec gains headings, the coverage-count error's hint, run verbatim, clears that finding and leaves every task's `source_sections` byte-identical. At `HEAD` the error carries no hint | a hint naming a command that does not recompute the map, such as `tp validate` |
| 21 | §6 | the notice names the closed-task count and the sha total, both non-zero and matching the resolved task file | the state sentence, which passes any test asserting only that a notice appeared |
| 22 | §6 *counts* | the counting is asserted directly on a synthesized fixture whose pairs differ — more tasks than closed tasks, more `commit_shas` entries than non-null `commit_sha` fields | count all tasks rather than closed ones; and, separately, count `commit_sha` rather than `commit_shas` — both survive any fixture where the pairs coincide |
| 23 | §6 *cadence* | it fires at the release phase and not on the invocations before it, asserted over a sequence of calls | fire per invocation |
| 24 | §6 *route* | it fires on a release-phase cycle driven by `tp run`, which never invokes the resume command | key it on the `resume` command; and, separately, key it on audit convergence instead of the computed phase, which fires on an implementing cycle whose audit has converged |
| 25 | §6 *tracked* | a tracked task file produces nothing, whatever the ignore rules say | test the ignore rules instead of the index — the pattern-only form, since an index-aware ignore check agrees with the index on a tracked-then-ignored file and would not fail the row |
| 26 | §6 *not a gate* | convergence, exit codes and `--check` are unchanged by the notice's presence, and `--quiet` silences it while the phase still reports `release` | let it set an exit code; and, separately, write it to stdout, where it corrupts the payload a driver parses |
