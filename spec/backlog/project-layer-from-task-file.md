# tp — The project layer belongs to the task file

A backlog decision, decided by the operator; it replaces the BUGS.md entry "The project config layer
resolves from the process working directory, not from the task file".

Class: **decision**. A documented precedence change.

## 1. The decision

**Context.** `ProjectWorkflowOverride` discovers `.tp/` from the working directory, so with `--file`
(or `TP_FILE`) naming a task file in another project tree, that plan is driven by the current
project's `quality_gate`, caps, `commit_strategy` and checks. A test fixture in a temp directory
discovers tp's own config for the same reason.

**Decision.**

- The project layer is resolved from the task file's directory, always.
- A command whose work happens in the working tree — by writing to it or by sending agents to read
  it: the quality gate, registered checks, a commit, `tp review` and `tp audit` emissions, `tp run`
  — is refused when the task file's project root is not the working directory's, with a hint naming
  the directory to run it from. What reads only the task file and its state stays allowed from
  outside. The two sides are not written from memory: a test enumerates every command and asserts
  its side, so a new command has to be placed.
- The **project root** is the directory holding the `.tp/` discovery stops at; with no `.tp/`, the
  git top level; with neither, the task file's own directory. Two task files outside any repository
  and any `.tp/` never compare equal by reaching the filesystem root.
- A project config that exists but cannot be read is an error on the paths that use it, not "no
  project config".

**Verify before building.** List every place `.tp/` is discovered — the project config, the role
corpus, routed classes, the epic graph and any other — and the start directory each uses. The
decision covers every one a task-file command reaches; any it leaves is named here with the reason,
since fixing one site of several leaves the entry's title untrue.

**Checked before the build (2026-11-13, at 59975e14).** Every place tp finds `.tp/`
(`DiscoverTPDir`, `ProjectConfigDir`, `ProjectRoot`, `ProjectWorkflowOverride`) and the directory
it starts from:

| Site | Starts from | Reached by a task-file command | Here |
|---|---|---|---|
| project config, `ProjectWorkflowOverride` (`configresolve.go`) | working directory | `ResolveWorkflow` (review, audit, their record and status, resume, the run oracle), `EffectiveWorkflowForTaskFile` (the gate in done, close and import, run), brief, next, plan, config, the commit strategy, the unattended fences | follows the task file |
| project config, `ResolveEffectiveWorkflow` (`tp config`) | working directory | `tp config --file` | follows the task file |
| `.tp/local.json` `notify_cmd` (`LocalNotifyCmd`) | working directory | run | follows the task file |
| working-tree classification, keep-list and bookkeeping in `AssembleResume` | working directory | resume, `tp run` | the task file's project root when it is not the working directory's |
| round directory root (`BuildNextUnits`, `RoundDir`) | working directory, or the run's root | resume, run | as the row above |
| epic graph for a bound task file (`EpicBlockFor`) | the task file's directory | next, resume, export | already follows |
| epic binding check (`tp set --epic`) | working directory | set | follows the task file |
| lock files (`LockFilePath`), run lock | the locked file's directory | every writer | already follow |
| role corpus and its hash (`LoadRoleCorpus`, `rolehash.go`) | the spec's directory | review, audit, resume, run | left: the corpus belongs to the spec, which `tp review <spec>` takes with no task file at all; the task file's spec field resolves from the task file, so for a spec inside its task file's tree the two agree, and review and audit emission is refused across trees anyway |
| `.tp/local.json` `active` (`ResolveLocalActive`) | working directory | the discovery chain itself | left: it is how a task file is found, so it cannot start from one |
| `.tp/local.json` flag defaults (`LocalFlagDefaults`) | working directory | every command | left: applied while flags are parsed, before any task file is resolved, and they are per-operator presentation (`--compact`, `--quiet`), not project policy |
| routed classes (`.tp/routed-classes.json`) | repository root | none: only `scripts/audit-round-prep.py` reads it | not a tp discovery |
| dashboard, epic graph commands, `use`, `keep`, `validate --project`, `set --workflow --project`, `init`'s `.tp/` | working directory | no task file | unchanged (non-goal) |

`ProjectWorkflowOverride` swallows a load error: a `config.json` that exists but cannot be read or
parsed returns an empty override, so the built-in defaults run as if there were no project config.
`ResolveEffectiveWorkflow` (`tp config`) is the one path that already refuses it. No site needs a
further design choice: the three left are named with their reasons.

`ProjectRoot` (`projectconfig.go`) already computes the root the decision defines: the directory
holding the `.tp/` discovery stops at, else the git top level, else the start directory itself.

**Consequences.** Resolution precedence in REFERENCE changes. Driving another tree's plan with
`--file` keeps working for reports and stops for work in the tree, which until now ran against the
wrong project and exited 0.

## 2. Alternatives

- **Resolve from the task file only.** Rejected: the other project's gate would run, and its spec
  would be reviewed, against this tree.
- **Split on reading versus writing commands.** Rejected: review and audit write nothing yet send
  agents to read the working tree.

## 3. Non-goals

- Changing discovery for commands that take no task file.
