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
