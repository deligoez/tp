# hooks-fence-the-target — measurements

Supplemental material for `hooks-fence-the-target.md`; the spec stands without it. **This file is not
a spec and `tp ground` never grades it.** Every exit code below was observed at `18032abe`. The hooks
and agent definitions there are byte-identical to `v1.1.1`: `git diff --stat v1.1.1 HEAD -- hooks
agents` prints nothing.

---

## The call shapes at HEAD

**How it was run.** Each case is one JSON payload in the shape Claude Code sends a `PreToolUse` hook
(`hook_event_name`, `tool_name`, `tool_input`, `cwd`), fed on stdin to the hook script from the
repository root with `PATH=/usr/bin:/bin` and no other environment. The exception is the role-hook
cases, which also set `TP_ROUND_DIR=spec/.tp-review/1.1.0/round-2` and `TP_UNIT_ID=architect`.
`internal/cli/hooks_pre_tool_use_test.go` drives the hook the same way (`runPreToolUseHookRaw`). The
snapshot path used is `spec/.tp-review/1.1.0/snapshot-round-1.md`. One case, which runs as written:

```bash
bash -c 'printf %s "{\"tool_name\":\"mcp__codedbpro__batch\",\"tool_input\":{\"ops\":[{\"tool\":\"read\",\"args\":{\"file\":\"spec/.tp-review/1.1.0/snapshot-round-1.md\"}}]}}" | env -i PATH=/usr/bin:/bin hooks/pre-tool-use-write-deny.sh; echo "exit $?"'
```

It prints the four-line scope-fence message (`tp scope fence: spec/.tp-review/1.1.0/snapshot-round-1.md
is tp's own state and must not be hand-edited (v0.35.0 §6.2).` …) and `exit 2`.

| case | hook | tool / operations | exit |
|---|---|---|---|
| A1 | write-deny | batch: `read` of the snapshot, `faster_search` with `path` into `spec/.tp-review/1.1.0` | **2** |
| A2 | write-deny | the same batch shape on `spec/1.1.0.md` and `spec/backlog` | 0 |
| A3 | write-deny | batch: one `read` of `.tp/config.json` | **2** |
| A4 | write-deny | batch: five reads of ordinary files and one `read` of `spec/0.25.0.tasks.json` | **2** |
| B1 | write-deny | batch: `read` of `spec/1.1.0.md`, `edit` of the snapshot | 2 |
| B2 | write-deny | batch: `create` of `.tp/config.json` | 2 |
| B3 | write-deny | batch: `read` of the snapshot, `edit` of `internal/cli/run.go` | **2** |
| B4 | write-deny | batch: `replace` with `path` `spec/.tp-review/1.1.0` | 2 |
| B5 | write-deny | batch: `faster_search` with the same `path` | **2** |
| B6 | write-deny | batch: `replace` with `paths` `["README.md","spec/1.1.0.tasks.json"]` | 2 |
| C1 | write-deny | `Write`, `file_path` = the snapshot | 2 |
| C2 | write-deny | `mcp__codedbpro__create`, `file` = the snapshot | 2 |
| C3 | write-deny | `Write`, `file_path` = `internal/cli/run.go` | 0 |
| C4 | write-deny | `Write`, `file_path` = `/private/tmp/throwaway-copy/spec/.tp-review/1.0.0/state.json` | 2 |
| R1 | role allowlist | batch: `read` of `spec/1.1.0.md` | **2** |
| R2 | role allowlist | batch: `create` of the unit's own `role-architect.ndjson.part` | 0 |
| R3 | role allowlist | batch: `read` of `spec/1.1.0.md`, `create` of the unit's own findings file | **2** |
| R4 | role allowlist | batch: `edit` of `spec/1.1.0.md` | 2 |

Bold marks a call the spec's decision passes, although `HEAD` refuses it. **R1 and R3 are not in the
field report**, and they are the sharper half. They show that inside a role unit under `tp run`, any
batch that reads anything other than the unit's own two files is refused — including the spec the
role was spawned to review — with the message *"is not this unit's to write"*. The role allowlist
extracts paths exactly as the write-deny fence does (`hooks/pre-tool-use-role-write-allow.sh:160-173`
against `hooks/pre-tool-use-write-deny.sh:84-98`).

**Why.** Both scripts collect every `"file_path"`, `"notebook_path"`, `"file"` and `"path"` string
value and every string in a `"paths"` array, anywhere in the payload
(`hooks/pre-tool-use-write-deny.sh:84-98`, `hooks/pre-tool-use-role-write-allow.sh:160-173`). Neither
looks at the operation a value belongs to. The write-deny script then tests each value against the
four fenced classes (`denied()`, `:63-71`).

## Which tools the matchers name

`hooks/hooks.json`, `agents/tp-reviewer.md` and `agents/tp-auditor.md` carry the same matcher string
of nine alternatives: the four native editors, codedbpro's `create`, `edit`, `patch`, `replace`, and
`batch`. Matching each tool name against that string as a full-string regex gives: `mcp__codedbpro__read`, `faster_search`,
`meta_search`, `memo`, `lint`, `diff`, `Read` and `Bash` do not match; `batch` and `edit` do. So a
read made as its own call never reaches either hook, and only `batch` carries a read into one. `batch`
has been in the matchers since `dc80e766`, which added it because a batched write was never judged
at all (`internal/cli/hooks_pre_tool_use_test.go:37-45` records why).

`skills/tp/REFERENCE.md:461` gives the matcher as the four native editors alone. `spec/undecided.md`,
*Inferring a spec's class*, recorded that mismatch and deferred correcting it to this release.

## Mutants built at HEAD

Each mutant is a copy of the `HEAD` script with one edit, run on the same payloads (compact JSON, so
the inserted `case` sees `"tool_name":"mcp__codedbpro__batch"` byte for byte):

| mutant | the edit |
|---|---|
| skip-batch | after `payload=$(cat)`, `exit 0` when the payload's tool is `batch` |
| batch-only | after `payload=$(cat)`, `exit 0` unless the payload's tool is `batch` |
| no-path-key | the key alternation loses `path` |
| no-paths-array | the `paths` array pass is disabled |
| role-skip-batch | skip-batch, applied to the role allowlist |

| case | `HEAD` | skip-batch | batch-only | no-path-key | no-paths-array |
|---|---|---|---|---|---|
| B1 read + edit into `.tp-review/` | 2 | **0** | 2 | 2 | 2 |
| B4 batch `replace` with `path` | 2 | 0 | 2 | **0** | 2 |
| B6 batch `replace` with `paths` | 2 | 0 | 2 | 2 | **0** |
| C1 `Write` into `.tp-review/` | 2 | 2 | **0** | 2 | 2 |
| C2 `create` into `.tp-review/` | 2 | 2 | **0** | 2 | 2 |
| K1 `edit` op with `args` before `tool` | 2 | 0 | 2 | 2 | 2 |
| K2 `read` op with `args` before `tool` | 2 | 0 | 2 | 2 | 2 |

| case | `HEAD` | role-skip-batch |
|---|---|---|
| R3 read spec + create own findings | 2 | 0 |
| R4 edit spec | 2 | **0** |

Bold marks each of the spec's rows 3, 4, 5, 7 and 8 going red under its named mutant. K1 and K2 only
show that `HEAD` ignores key order, which it does trivially because it looks at every operation. The
mutant row 6 names — pair a path with the nearest preceding tool name — can only be built on top of
the fix. It is recorded because key order in a JSON object carries no meaning, and nothing in the
batch tool's schema requires `tool` to come before `args`.

**A constraint on the implementation, not a decision.** The `HEAD` scripts parse no JSON on purpose:
they have no dependency and only match a key with a bare quote on each side, so a file's contents
(where quotes arrive escaped) can never be taken for an argument
(`hooks/pre-tool-use-write-deny.sh:14-20`). Telling which operation a path belongs to is the first
thing either hook will need that pattern matching alone may not give it. Row 6 exists so that
whatever the implementing task builds is tested on both key orders.

## Field report WB-3155, verified 2026-09-11

**#6 — "the hook cuts read-only access too."** The claim: the plugin's
`pre-tool-use-write-deny.sh` refuses a `mcp__codedbpro__batch` that only *searches* a `.tp-review/`
path, so the fence looks at the path rather than at write intent. Two subagents and the reporter hit
it independently. The file it blocked is the snapshot the ground prompt says to read. And
`zigread`/`zigrep` pass, so the fence is "also inconsistent". Proposal: fence write operations and
let read tools through.

- **Verdict: CONFIRMED** for the refusal. **INTENDED** for the inconsistency.
- **Reproduction:** cases A1 (exit 2) and A2 (exit 0) above — the same batch shape, `read` plus
  `faster_search`, differing only in whether a path lies under `.tp-review/`. A3 and A4 show that the
  other fenced classes behave the same, and that one fenced read in a batch of six refuses the whole
  batch. Observed stderr is the four-line scope-fence message quoted above.
- **Source:** the extraction is at `hooks/pre-tool-use-write-deny.sh:84-98`, the fenced classes at
  `:63-71`, and the matcher that routes `batch` to the script at `hooks/hooks.json:18`. The ground
  prompt's instruction is `internal/cli/ground.go:1088-1090`: *"The snapshot named at the top of
  this prompt is the spec as this round found it: read it for a unit's surroundings — its section,
  what precedes it, what the sentence is about — never to measure a unit."*
- **What was not confirmed:** the inconsistency. `zigread` and `zigrep` are shell commands, and shell
  tools are outside the matcher by design — `spec/0.35.0.md` §6.2: *"the denial exists to stop
  hand-editing, not to sandbox, and tp's own commands rewrite `*.tasks.json` on every close"*. That
  is pinned by `TestPreToolUseHookDoesNotFireForShellWrites`. That a shell read passes where a
  batched read is refused is the defect in the batched read, not a second defect in the shell path.
- **Beyond the report:** the role allowlist has the same defect in a harder form (R1, R3).

## Where the decisions come from

**Taken by the 2026-09-08 decision pass** (`spec/undecided.md`, *The test-file fence and the
write-deny fence's reach*): the write fence matches on the write target; the test-file permission is
precomputed into the child environment at spawn; `test_globs` follows `pickChecks` in
`internal/engine/configresolve.go`, which returns the first present layer. `checks` is the only
list-typed field in `model.WorkflowOverride` (`internal/model/projectconfig.go:58`), and
`TestResolveWorkflowLayers_ChecksReplaceSemantics` pins its replace rule.

**The fence's rule and its brief** come from the draft that first registered the test-file fence,
reachable as `git show 11c6d7ba:spec/0.47.0.md`, §2. It gives the rule quoted in the spec's §3, and
*"`tp brief` states the fence with the unit's own permission resolved … The fence text and the hook
must agree by construction — one source, two consumers."* That draft left the resolution site as
*"Design pass needed"*; the 2026-09-08 pass answered it.

**Four things were decided while writing this spec, not by that pass, and a reviewer should read them
as proposals:**

1. *The role allowlist follows the same rule.* The recorded decision names the write-deny hook. The
   allowlist shares the extraction and the defect (R1, R3), and a spec named for hooks that fixed one
   of the two would leave a role unit unable to batch a read of its own spec.
2. *No built-in `test_globs`.* The draft proposed defaulting to *"the common conventions of the spec's
   domain"*, but tp has no notion of a project's language, so any default would be a guess.
3. *`test_globs` is unwritable under `TP_UNATTENDED=1`.* Otherwise the fenced unit could run
   `tp set --workflow test_globs='[]'` through a shell — outside every matcher — and switch the fence
   off for every later unit. Fixing the permission at spawn protects only the unit already running.
4. *A hook without the driver's environment refuses nothing.* The sibling allowlist does the
   opposite: it refuses everything when `TP_ROUND_DIR` and `TP_UNIT_ID` are absent
   (`hooks/pre-tool-use-role-write-allow.sh:35-47`). It can, because an allowlist that cannot build
   itself has nothing to allow. A deny-list that does not know which task it serves has nothing it
   can deny without refusing every test file.

**The permission can only be the named file.** This is a consequence of precomputing, not a fifth
decision. The draft also permitted a test that *"the behaviour change … invalidates"*, which tp cannot
evaluate from a task file at spawn. The v0.31.2 rule the draft cites — a change and the test it
invalidates belong to the same task — is what makes naming the file cheap.

## The anchor

Case C4: a `Write` naming `/private/tmp/throwaway-copy/spec/.tp-review/1.0.0/state.json` — a path in
no repository — is refused at exit 2, because the patterns are unanchored (`*/.tp-review/?*`).
`spec/undecided.md` records the same observation and defers the anchor (`CLAUDE_PROJECT_DIR`, a
git-root walk, or nothing): *"a wrong anchor is the other direction: a fence that silently stops
fencing"*. The spec leaves it there.
