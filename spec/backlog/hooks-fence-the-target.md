# tp — Hooks fence the target

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `hooks-fence-the-target-measurements.md` beside it, and this file stands without
them. It writes down three decisions the 2026-09-08 decision pass took together — `spec/undecided.md`,
*The test-file fence and the write-deny fence's reach* — and a field report (WB-3155) that ran into
the first of them.

Class: **tool** — it changes what the plugin's hooks refuse and adds one workflow field; no round,
grade or convergence signal reads either.

## 1. The decision

**Context.** The plugin ships a `PreToolUse` fence that refuses hand-edits to tp's own state — the
contents of `.tp-review/`, task files, `.tp/config.json` and the git-ignored pointer `tp use` writes
beside it — and the reviewer and
auditor agent definitions register a second hook that lets a role unit write its own findings file
and escalation record and nothing else. Both hooks judge a call by every path-shaped argument in its
payload, whatever operation that path belongs to. For a single-target write tool that is the target.
For codedbpro's `batch`, which both matchers name because it can carry writes, it is also every path
that any *read* in the batch names. So a batch that only reads a `.tp-review/` snapshot is refused
with the hand-edit message, although the ground prompt tells a grader to read that snapshot for a
unit's surroundings. Inside a role unit, a batch that only reads the spec is refused because the spec
is not the unit's findings file. The same reads made as separate calls never reach either hook,
because neither matcher names a read tool. What the hook decides depends on whether the agent
batched its calls, not on what the calls do (sidecar, *The call shapes at HEAD* and *Field report
WB-3155*).

**Decision.**

1. **A tp hook judges a call by its write targets.** For a write-capable tool, the target is its path
   argument. **A batch is judged by its operations, each exactly as the same call made on its own
   would be:** an operation whose tool the matcher does not name passes, and an operation whose tool
   it names is judged by that operation's target. A batch whose operations only read therefore
   passes; a batch carrying one write into a fenced path is refused, and the refusal names that
   path. The write-deny fence and the role write allowlist both follow this rule.
2. **The test-file fence resolves its permission when the unit is spawned.** An implement unit may
   write a test file only when its task's acceptance names that file. tp computes that permission at
   spawn and passes it in the unit's environment. The enforcing hook reads that environment. It reads
   no tp state and runs no `tp` command.
3. **`test_globs` layers like `checks`.** The list of patterns that says what a test file is, is a
   workflow field. A list set at one layer replaces the list at the layer beneath it, and an explicit
   empty list counts as set.

**Consequences.** No tp hook refuses a read, whatever tool carries it. What a single-target write tool
is refused does not change, and `TestPreToolUseHookDeniesTheFencedPaths` pins it for every such tool.
That test's `batch` cases give the path as a top-level argument, a shape no batch sends, so they are
rewritten to carry it inside a write operation. Shell tools stay outside the matcher
(`TestPreToolUseHookDoesNotFireForShellWrites`). The hooks table in `skills/tp/REFERENCE.md` is
rewritten from `hooks/hooks.json` and states this rule; at `HEAD` it lists fewer tools than the
manifest registers, a correction `spec/undecided.md` deferred to this release.

**Alternatives.**
- *Keep reads out of the batch matcher.* This cannot be done. A matcher sees the tool name, and every
  batch is named `batch`, whatever its operations do.
- *Take `batch` out of both matchers.* That reopens the hole `dc80e766` closed: a batched write was
  never judged at all.
- *Resolve the test-file permission inside the hook, one `tp` call per write.* Rejected: every write
  would pay for a process start inside the 10-second bound `spec/0.35.0.md` §6.4 puts on every
  hook, and the hook would then depend on tp's state readers when it has never needed them.
- *Merge `test_globs` across layers.* Rejected: `checks` is the one list-typed override that exists,
  and it replaces. If the second one merged, two list fields would follow two rules.

## 2. The write fence judges the target

| the call | the hook |
|---|---|
| a single-target write tool naming a fenced path | refuses it, as today |
| a batch whose operations only read, whatever paths they name | passes it |
| a batch with one write operation into a fenced path, among any number of reads | refuses it and names the write's path |
| a batch whose writes target unfenced paths while its reads name fenced ones | passes it |
| a shell tool | is never called |

**The tool decides, never the key.** A read and a write can name their path under the same key: both
`faster_search` and `replace` take `path`. So the operation's tool is what separates them. The order
of keys inside an operation does not matter either.

**The role allowlist gets the same reading.** A role unit may read anything through a batch, and may
write only the two files it could write before. The files it may write do not change.

## 3. The test-file fence

**The rule** comes from the draft that registered the fence (sidecar, *Where the decisions come
from*): *a disagreement with a test is information, not an edit.* An implement unit may write a file
that matches `test_globs` only when its task's acceptance names that file. Otherwise the disagreement
goes on the closure evidence's `Out of scope:` line, which `tp done` already accepts, and the
refusal says so.

**Only a named file can be permitted, because the permission is fixed at spawn.** tp can compute the
permission only from what the task file holds at that moment. The draft also permitted a test that
*the behaviour change invalidates*, but tp cannot recognise that from the task text. So that case
reaches the fence through the file's name: a decomposer who knows a change breaks a test names the
test in the same task's acceptance. That is also where the change and the test belong, since the gate
runs at every close.

**One source, two consumers.** `tp next --brief` lists the test files this unit may write. It
computes that list with the same resolution the hook reads, so the brief and the hook cannot
disagree.

**Where the fence does not reach.**
- **No layer sets `test_globs`:** no file counts as a test file and the fence refuses nothing. tp
  ships no built-in list. tp does not know a project's language, and a guessed list refuses
  legitimate writes wherever the guess is wrong.
- **Outside `tp run`:** there is no spawn, so the brief's statement is the only fence. A hook that
  runs without the environment the driver sets refuses nothing, because it cannot tell which task it
  is serving.
- **Under `TP_UNATTENDED=1`:** no unit can write `test_globs`, at any layer and at any value — the
  rule `runner` and `notify_cmd` already follow
  (`TestUnattendedFence_RunnerAndNotifyCmdRefusedAtEveryLayer`).
  A fence the fenced unit could switch off would protect nothing. Because the permission is fixed at
  spawn, a write made during a unit changes nothing for that unit.

## 4. Non-Goals

1. **No anchor.** Whether a fenced path has to lie inside the project is not decided here (sidecar,
   *The anchor*). Judging the target makes that question answerable against a target instead of a
   payload. `spec/undecided.md` defers the answer, because a wrong anchor is a fence that silently
   stops fencing.
2. **No shell fencing.** The write-deny fence is there to stop hand-editing, not to sandbox, and tp's
   own commands write the fenced files through a shell.
3. **No new fenced class**, and no change to the message that names the tp command which owns each
   class.
4. **No built-in `test_globs` and no language detection.**
5. **The test-file fence judges which file is written, not what is written to it.**

## 5. Tests

Every row derives from a numbered decision and names a mutant that must fail it. The two hooks exist
at `HEAD`, so rows 1–9 quote the exit at `HEAD` and the exit under a mutant copy of the `HEAD` hook,
both from the sidecar's *Mutants built at HEAD*. A row marked *deferred* has a mutant that needs the
fix before it can be built, or a subject that does not exist yet. Its pair of values belongs to the
implementing task's acceptance.

| # | from | assertion | at `HEAD` | the mutant that must fail it |
|---|---|---|---|---|
| 1 | §1.1 | a batch of `read` and `faster_search` naming a `.tp-review/` snapshot exits 0 at the write-deny hook | 2 | judge every operation's path — `HEAD`, 2 |
| 2 | §1.1 *role* | with a role unit's environment set, a batch that reads the spec and creates the unit's own findings file exits 0 at the role allowlist | 2 | judge every operation's path — `HEAD`, 2 |
| 3 | §1.1 *write* | a batch that reads an unfenced file and edits a `.tp-review/` path exits 2 and names the edit's path | 2 | never judge a batch — 0 |
| 4 | §2 *tool* | a batch `faster_search` with `path` into `.tp-review/` exits 0; a batch `replace` with the same `path` exits 2 | 2 and 2 | drop `path` from the target keys — `replace` 0; judge by key — `HEAD`, `faster_search` 2 |
| 5 | §2 *list* | a batch `replace` whose `paths` list includes a task file exits 2 | 2 | read single-string targets only — 0 |
| 6 | §2 *order* | an operation whose `args` come before its `tool` is judged as in the other order: the write exits 2, the read exits 0 | 2 and 2 | pair each path with the nearest preceding tool name — deferred |
| 7 | §1 *single* | `Write` with `file_path` and codedbpro `create` with `file` into `.tp-review/` both exit 2 | 2 and 2 | judge batch operations only — 0 and 0 |
| 8 | §2 *role write* | in a role unit, a batch that edits the spec exits 2 | 2 | the role hook never judges a batch — 0 |
| 9 | §1 *doc* | the `PreToolUse` matcher cell in `skills/tp/REFERENCE.md` names the same tools as the matcher in `hooks/hooks.json` | fails | `HEAD`'s row |
| 10 | §3 | with `test_globs` `["*_test.go"]` and acceptance naming `a_test.go`, a write to `a_test.go` passes; a write to `b_test.go` exits 2 and the refusal names the `Out of scope:` line | deferred | permit every file `test_globs` matches |
| 11 | §1.2 | the verdicts of row 10 hold with no `tp` on the hook's `PATH` | deferred | resolve the permission by calling `tp` from the hook |
| 12 | §1.2 *spawn* | a `tp set` that changes `test_globs` after the unit is spawned leaves that unit's verdicts unchanged | deferred | re-read the configuration on every call |
| 13 | §3 *one source* | editing the task's acceptance to name `b_test.go` changes both what the brief lists and the hook's verdict | deferred | the brief derives its list on its own, from the task's `source_sections` |
| 14 | §3 *unattended* | `TP_UNATTENDED=1 tp set --workflow test_globs='[]'` exits 2 and writes nothing; `--project` likewise; both exit 0 without the variable | deferred | leave the field unfenced |
| 15 | §3 *opt-in* | when no layer sets `test_globs`, a write to `foo_test.go` passes | deferred | ship a built-in list |
| 16 | §1.3 | with the project at `["*_test.go"]`: a task layer of `[]` resolves to `[]`, and a task layer of `["spec/**"]` resolves to exactly `["spec/**"]` | deferred | merge the layers into a union |
