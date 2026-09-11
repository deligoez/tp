# a-task-file-write-names-its-target — measurements

Supplemental material for `a-task-file-write-names-its-target.md`; the spec stands without it. **This
file is not a spec and `tp ground` never grades it.** Every transcript below was run on 2026-09-11
with a binary built from `18032abe` (`tp version v1.1.2-0.20260910212136-18032abe405f`), each in a
fresh `git init` scratch directory outside the repository. `hc` is on that machine's `PATH`, so
`commit_strategy` resolves to `hc` and every close below carries `--commit`. Source citations are
`file:line` at `18032abe`.

## Field report WB-3155, verified 2026-09-11

A field report (WB-3155) from an agent running tp v1.1.1 on a Laravel/PHP backend, written in
Turkish. The items below are the ones this spec takes, plus one it declines (#26). Each was
reproduced against `HEAD` before it was written here; where the report's explanation of its own
observation was wrong, the verdict says so and the spec takes only what survived.

### #22 — "auto-selection silently targets another spec's task file" — PARTLY: mechanism refuted, danger confirmed

**Claim.** With several `*.tasks.json` in the repository, `tp list`, `tp remove` and `tp add` without
`--file` pick one and do not say which. Twenty-five `tp remove` calls during a replan all went to
another spec's file; only the fact that those tasks were `done` (and `tp remove` refuses non-`open`
tasks) prevented silent destruction of another plan.

**Verdict.** The mechanism the report names is **refuted**: several candidates where auto-detection
looks make every write refuse at exit 3, not pick one. The danger is **confirmed** through two other
paths, both silent at exit 0.

**Reproduction 1 — a pointer left from an earlier spec.** Two one-heading specs, `alpha.md` and
`gamma.md`, in one directory:

```
tp init alpha.md                      → {"created":"alpha.tasks.json"}
tp add '{"id":"a1",…}'                → {"added":["a1"]}
tp use alpha.tasks.json               → {"active_file":"alpha.tasks.json"}
tp init gamma.md                      → {"created":"gamma.tasks.json"}   stderr: 0 lines, exit 0
tp add '{"id":"g1",…}'                → {"added":["g1"]}                 exit 0
tp remove a1                          → {"removed":"a1"}                 stderr: 0 lines, exit 0
```

Afterwards `alpha.tasks.json` holds `[g1]` and `gamma.tasks.json` holds `[]`: the task meant for gamma
went to alpha, and alpha's open task was deleted. `.tp/local.json` still reads
`{"active": "alpha.tasks.json"}`. Continuing the same fixture, `tp import` of a gamma task document
wrote gamma (`{"imported":1,"path":"gamma.tasks.json"}`), and the next `tp next` claimed `g1` out of
alpha and returned `"spec_excerpt": "## 1. One\nAlpha one."` — the other spec's text, the only visible
trace. The orchestrator reproduced the `remove` half independently on its own fixture.

**Reproduction 2 — several candidates, no pointer.** Same two specs, both initialised, no `tp use`:

```
tp remove x1 → {"error":"multiple task files: alpha.tasks.json, gamma.tasks.json. Set TP_FILE=<path> or use tp --file <path> <command>","code":3,"hint":"run 'tp use <file>' to set the task file, or 'tp init <spec>' to create one"}
tp add '{…}' → {"error":"no task file found. Use --spec to create one, or run tp init first","code":3,"hint":"run 'tp use <file>' …"}
```

The `tp add` message is false in that directory: `internal/cli/add.go:171-177` replaces discovery's
candidate list with its own text whenever `--spec` is absent.

**Reproduction 3 — a current-directory file shadowing a subdirectory's.** `alpha.tasks.json` in the
current directory, `sub/beta.tasks.json` one level down: `tp add '{"id":"b1",…}'` returned
`{"added":["b1"]}` at exit 0 with no stderr, and `b1` landed in `alpha.tasks.json`. Auto-detection
scans subdirectories only when the current directory holds no candidate.

**Source.** `internal/engine/discover.go:15-62` — the pointer is consulted at `:30-37`, before
auto-detection at `:39-61`, and the multiple-candidate refusal is `:59-60`.
`internal/engine/configresolve.go:18-42` joins the pointer's stored value to the project root, so a
pointer-resolved path is absolute while an auto-detected one is relative — which is why the spec
fixes `file`'s form rather than echoing whichever path discovery produced.

**Which path the reporter's session took is not recoverable from the report.** Both silent paths are
reproduced above. The report says it pinned with `tp use` *afterwards*, which fits a pointer left by
an earlier spec's session as well as it fits shadowing; the spec covers both.

**Payload keys that already name a file** (observed in the transcripts above): `tp init` →
`created`, `tp import` → `path`, `tp use` → `active_file`. Every other write in the transcripts —
`add`, `remove`, `next`, `set`, `set --bulk`, `done --batch` — names none.

### #24 — "no exit from `wip`" — CONFIRMED

A task claimed with `tp claim t1`:

```
tp remove t1          → {"error":"cannot remove: task t1 is wip (must be open)","code":4,"hint":"Use `tp reopen` first to reset to open, then retry."}
tp remove t1 --force  → the same, exit 4
tp reopen t1          → {"error":"cannot reopen: task t1 is wip (must be done)","code":4,"hint":"run 'tp status' and 'tp list' to inspect task states"}
tp set t1 status=open        → exit 2, "field \"status\" is managed — use `tp claim`, `tp close`, or `tp reopen`"
tp set t1 started_at=null    → exit 2, "field \"started_at\" is managed — set automatically by `tp claim` / `tp done` / `tp next`"
```

`tp remove`'s hint is false for a `wip` task: following it reaches `tp reopen`'s refusal. Source:
`internal/model/task.go:152-165` (the transition table has no `wip`→`open` edge),
`internal/cli/remove.go:51-52`, `internal/cli/reopen.go:48`. The report's workaround — rewriting the
`wip` task's fields in place with `tp set` because the content was the same work — was honest only
because it was; had the plan changed, closing the task was the remaining exit.

### #14 — "`tp next` does not separate asking from claiming" — INTENDED; one residue CONFIRMED

Plain `tp next` claiming is intended and documented: `skills/tp/SKILL.md:603` (*Resume WIP or claim
next ready*), `:605` (`--peek`, *Preview without claiming*), `:606` (`--brief` claims and returns the
brief in one call), and `internal/cli/next.go:30`. The report's own suggestion was `--peek`, which
exists. That is why the body keeps plain `tp next` as it is and does not argue the point.

**Residue.** `--peek` skips a `wip` task (`internal/cli/next.go:65-73`), so it previews a different
task than plain `tp next` returns. Fixture `t1` `wip`, `t2` ready: `tp next --peek` → `t2`;
`tp next` → `t1`.

### #25 — "`tp done --batch` refuses in `commit` what the CLI accepts" — PARTLY

**Array in `commit` — confirmed.** An entry `{"id":"t1","reason":"…","commit":["<A>","<B>"]}`:

```
{"error":"invalid NDJSON line: json: cannot unmarshal array into Go struct field batchEntry.commit of type string","code":3,"hint":"run 'tp use <file>' to set the task file, or 'tp init <spec>' to create one"}
```

A file whose first line is blank and whose third line carries the array returns the identical
message: no line number (`internal/cli/done.go:973-976`). The hint is the code-3 default
(`internal/output/output.go:54-65`); the call site at `internal/cli/done.go:612-614` passes none.

**`commit_shas` already works — the report did not know.** `{"id":"t1","reason":"…","commit_shas":["<A>","<B>"]}`
closed at exit 0 and recorded `commit_shas: [A, B]`, `commit_sha: A` (`internal/cli/done.go:592-601`,
`:795`, `:926-956`). `skills/tp/SKILL.md:368` names it; `skills/tp/REFERENCE.md:53-66`, the NDJSON
field list, does not.

**Both keys at once — a silent drop the report did not hit.** `{"commit":"<A>","commit_shas":["<B>"]}`
closed at exit 0 (`closed 1, failed 0`) and recorded `[B]`; `A` is gone. `resolveCommitSHAs` prefers
the list and reads the single value only when the list is empty (`internal/cli/done.go:930-934`).

**The same wrong hint elsewhere.** `tp import` over a task file that already holds tasks refuses with
*task file already exists: … (use --force to overwrite)* and the code-3 default hint telling the agent
to `tp init` one (`internal/cli/importcmd.go:176`).

**Connection to the report's #32.** The report traces a file-selection miss to tasks closing with one
sha because of this item. With `commit_shas` available that half is avoidable; the separate defect —
task-to-file mapping reading only the first sha — is `audit-records-what-was-graded`'s.

### #5 — "`tp done` splits acceptance at `;`" — CONFIRMED

Acceptance `- Model exists; migration runs` / `- Endpoint returns 200. Body is JSON` / `- Tests pass`:

```
tp add …        → {"added":["b1"]}                                  (no count)
tp validate     → "task b1: acceptance has 5 criteria (max 3); hint: split into ~2 tasks by concern …"
tp done b1 --commit <A> -- $'- model and migration\n- endpoint\n- tests'
                → exit 1, "closure verification failed: acceptance has 5 criteria but reason has 3 evidence line(s)"
                  hint: (1) Model exists (2) migration runs (3) Endpoint returns 200 (4) Body is JSON (5) Tests pass
```

A JSON-array acceptance `["Model exists; migration runs", "Tests pass"]` is stored as
`- Model exists; migration runs\n- Tests pass` and counts 3. Source:
`internal/engine/closure.go:10-32` splits on `\n- ` and then on `. ` and `; ` inside every part;
`skills/tp/REFERENCE.md:29-40` says *all delimiters are equivalent* and not that they compound. The
report's own figure (five bullets opening into nine criteria) was not re-derived — its task file is
not available here.

### #11 — "the atomicity bar changed silently between versions" — REFUTED; residue taken

The five thresholds are the same literals at `v0.31.0` and `HEAD`:

```
for r in v0.31.0 HEAD; do git show $r:internal/engine/validate.go | grep -n 'EstimateMinutes > 15\|len(words) > 8\|len(t.SourceSections) > 2\|len(t.Description) > 300\|len(criteria) > 3'; done
```

prints the same five lines, `201`–`218`, for both. `internal/engine/closure.go`'s diff over the same
range is a `strings.Split` → `strings.SplitSeq` rewrite with no behaviour change. What produced the
report's jump in findings is not established here.

**Residue.** The task `b1` above, closed, still yields its atomicity finding under `tp validate`, and
`tp validate --strict` reports `valid: false`, one error, exit 1 — a split hint for finished work.

### #12 — "the `section-anchor` format changed" — REFUTED; residue taken

The canonical-format message entered in `089e8e9a`, first contained in `v0.22.0`
(`git log -S'Expected canonical format' -- internal/engine/validate.go`), and
`internal/engine/resolve_section.go` has changed since `v0.31.0` only by two `go fix` modernisations
(`git log v0.31.0..HEAD -- internal/engine/resolve_section.go`). The 2026-09-11 verification
attributed the report's warnings to its spec having been rewritten.

**Residue.** Spec headings `### 1.1 Task Model`, task anchor `### 1.1`: the finding says *not found*
and prints the canonical format with no suggestion, though the number matches exactly one heading.
`tp set --bulk` with one line `{"id":"t1","field":"source_sections","value":["### 1.1 Task Model"]}`
cleared it in one call — the report's *no bulk repair* is also refuted, but nothing points at it.

### #13 — "only a full `tp import --force` repairs the coverage map" — REFUTED; residue taken

A spec gains a heading after decomposition:

```
tp validate → error "total_sections is 3 but spec has 4 headings"   (no hint)
tp set t1 'source_sections=["## 1. Models"]'                        (its current value)
tp validate → no coverage finding
```

Any anchor write recomputes the map, a same-value write included. The residue is that the finding
names no repair, which is what sent the report to `--force`.

### #26 — "a dependency chain locks a unit waiting on work in another repository" — not taken

A changelog task depends on tasks whose work lives in another repository and is waiting on a design
decision; the report asks for a task scope marking a unit as external. `spec/undecided.md`'s
*Cross-repo specs* was closed by the operator on 2026-09-08 and reopens *"when a field cycle asks for
it with a task file that names a second root"*. The operator reconfirmed on 2026-09-11 that
cross-repository work stays out of scope; the workaround is to drop the edge or split the unit.

## The write set

The commands that write a task file they resolved through discovery, at `18032abe`:

```
bash -c 'comm -12 <(git grep -l DiscoverTaskFile -- "internal/cli/*.go" | grep -v _test | sort) <(git grep -l WriteTaskFile -- "internal/cli/*.go" | grep -v _test | sort)'
```

prints `add.go`, `claim.go`, `close.go`, `commit.go`, `done.go`, `next.go`, `remove.go`, `reopen.go`,
`set.go` — covering `tp add` (single, `--bulk`, `--stdin`), `claim`, `close`, `commit`, `done` (single
and `--batch`), `next` when it claims, `remove`, `reopen`, and `set` (task field, `--bulk`, and
`--workflow` on the task layer, whose key is `a-findings-exits-agree` §6's). The derivation is
file-grained, so the implementing task re-derives it at its own `HEAD` and adds `tp unclaim`.

Outside the set, and why: `tp init` and `tp import` write the file their argument names and already
name it; `tp config --extract` thins task files found by a project scan
(`internal/cli/config_extract.go:300`), not by discovery; `tp use` and `tp set --local`/`--project`
write configuration; the `--resolve` modes and `tp ground` lock round files and the spec.

## Alternatives

1. **Refuse the write when the pointer resolves and another task file is in reach** — the report's
   first option. The pointer exists for a directory holding several task files; refusing there makes
   `tp use` a no-op in the only case it serves. The refusal for the ambiguous case already exists
   (reproduction 2).
2. **Notice on every write, or whenever no pointer is set** — the report's second and third options.
   The first fires on every single-spec project; the second is inverted, since reproduction 1's loss
   happens *because* a pointer is set.
3. **`tp init`/`tp import` move the pointer.** `.tp/local.json` is per-checkout, git-ignored state; a
   second agent in the same checkout may be working the spec it names, and moving it redirects that
   agent silently — the defect this spec exists to remove.
4. **Notice on reads too.** A read loses nothing, and the next write names its file before any loss;
   a notice per read is token cost on the commands an agent runs most.
5. **`tp remove --force` covers `wip`** — the report's alternative for #24. It deletes the task, and
   a released claim usually still belongs to the plan.
6. **`tp release`, the report's own name.** Rejected by the orchestrator of the 2026-09-11 pass: `release`
   is also the phase `tp resume` reports once a cycle converges, so an agent reading `phase: release`
   could take the command for the next step, and the word reads as cutting a release. `tp unclaim`
   pairs with `tp claim`, the command whose effect it undoes; the bare-usage rule (§3) stays as well.
7. **Stop plain `tp next` from claiming** (#14). It is documented, and `tp next --brief`, the unit's
   first call in the subagent workflow, relies on claiming and briefing in one call.
8. **A batch entry with both `commit` and `commit_shas` records their union.** The order of the union
   is a guess; a refusal costs the author one edit and guesses nothing.

## Carried from `two-advisories`

§6 of the spec is `two-advisories.md` §3 as it stood at `fb381786`, with its derivations moved here.
That spec's ground rounds for this half stay as history under
`spec/backlog/.tp-review/08b-untracked-task-file/` and `spec/backlog/.tp-review/two-advisories/`, and
`two-advisories-measurements.md` keeps its own *What is testimony* section.

### What is testimony

The advisory was motivated by an operator's report about a repository this cycle cannot reach: a
project that names task files `spec/upcoming-<name>.tasks.json` until release, ignores that pattern in
`.gitignore`, and deleted one mid-session on a wrong scope note — fourteen tasks' closure evidence and
commit shas, never tracked, unrecoverable. Three premises are that operator's retrospective account
and nothing here corroborates them: **the loss itself**; **the attribution** of the refutation of the
first draft (a per-invocation *task file is untracked* warning) — *"an untracked file reads as
disposable"*; and **"that session ran tp dozens of times"**, under the habituation argument. They
motivated the advisory; nothing rests on them. The orthogonality argument is a claim about two
sentences, checkable by reading them; the trigger is a fact about the phase computation; the
predicate is a fact about the git index. That the reframed notice *would* have prevented the deletion
is unfalsifiable and relied on nowhere.

The naming convention is that project's, not tp's: tp ships no `upcoming-` prefix and no such ignore
line, which is why the predicate reads the index and no prefix.

### The phase is the trigger, not convergence

`internal/engine/phase.go:28-42` tests `numDone < numTasks` and returns `implement` before it consults
audit convergence, and with zero tasks never consults it, so convergence is necessary for `release`
and not sufficient. Read off its branches (and probed with `go run` when `two-advisories` was
grounded): `(tasks 2, done 1, audit converged)` → `implement`; `(2, 2, converged)` → `release`;
`(0, 0, converged)` → `review`. `tp run` reaches the phase without invoking `tp resume`:
`internal/engine/driver.go:261` calls `AssembleResume` directly, and `:243` compares against
`PhaseRelease`. Hence spec rows 23 and 24.

### The predicate is new code, the capability is not

```
git grep -c 'ls-files' -- internal/ cmd/
```

prints `internal/cli/lock_pathspec_test.go:3` and nothing else, so `git ls-files` runs nowhere in
production; the search is known to work because it finds the test file. tp already shells to git from
production — `internal/cli/config_extract.go:286-290`, `gitWorkingTreeDirty`, runs
`git -C <dir> status --porcelain`. `internal/cli/report.go:88`'s `untrackedCount` is **not** prior art:
it counts done tasks without a measurable duration, and nothing on its path touches git.

### Which commands can see a task file

Re-run at `18032abe` in a directory holding a spec and no task file: `tp lint spec.md` exits 0,
`tp ground spec.md --units` exits 0 and writes nothing, `tp status` exits 3. The release phase cannot
be computed without a task file having been read, so the notice's trigger never meets the commands
that run without one.

### Rows 22 and 23 need different fixtures

Row 22's fixture must hold an open task so that counting all tasks and counting closed ones differ —
and a cycle with an open task is `implement`, where the notice never fires. So row 22 asserts the
counting directly and row 23 asserts the cadence on a fully closed cycle. This repository's own
`v0.36.0` task file is the counterexample that forces the split:

```
python3 -c 'import json; ts=json.load(open("spec/0.36.0.tasks.json"))["tasks"]; print(len(ts), sum(1 for t in ts if t.get("status") == "done"), len([s for t in ts for s in (t.get("commit_shas") or [])]), sum(1 for t in ts if t.get("commit_sha")))'
```

prints `24 24 47 24` at `18032abe` — tasks, closed, `commit_shas` entries, non-null `commit_sha`.
Counting all tasks and counting closed ones coincide there, so the first mutant could not fail on it;
the sha pair differs, so the second could.
