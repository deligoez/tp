# tp — Preparing a unit's worktree

A backlog decision candidate for 1.24.0. It comes from a field report: a night of parallel
implement driven by hand in the requesting project, a compiled-language repository whose build
cache is git-ignored. The operator settled, before this note was written, who may set the command:
the operator alone.

Class: **decision**. One project-level workflow field, and one step in starting an implement batch.

## 1. The decision

**Context.** With `run_max_parallel` above 1, `tp run` gives each unit of an implement batch a
fresh worktree of `HEAD` (REFERENCE.md, "Implement batches"). A fresh checkout carries no ignored
files: no dependencies, no `.env`, no build output. The unit's gate runs in that worktree, so in a
project whose gate compiles, every unit pays a cold build. Today the Limits paragraph says "a
project whose units or gate need them keeps `run_max_parallel` at 1", which turns the parallel mode
off for the projects whose builds are the slowest. The requesting project worked around it by hand:
before spawning each agent, it cloned its build cache into the worktree with copy-on-write.

**Decision.** An optional workflow field, `worktree_setup_cmd`: a shell command `tp run` runs in each
new batch worktree, after `git worktree add` and before any unit of the batch is spawned.

- **A command, not a list of paths.** The preparations that matter are a copy-on-write clone of a
  large cache, a symlink, or a dependency fetch, and a glob list cannot express those. A command
  covers all of them, and a plain copy is one of them.
- **Where it is read and who sets it.** It is read only from the project layer (`.tp/config.json`)
  of the main tree, as the batch starts; a value there that is not a string is a driver error. Like
  `quality_gate`, the task-level setter refuses it (exit 2). A task file's
  `workflow.worktree_setup_cmd` is refused too: `tp validate` reports it as an error and `tp import`
  refuses it ("read only from the project layer; move it to .tp/config.json"), and the driver never
  runs it. `tp config --resolved` reports it with source `project` or `default`. Setting or
  changing it is added to rule 9's operator decisions, and under `TP_UNATTENDED` it is fenced like
  `runner` and `notify_cmd`: refused at every layer and every value, exit 2, recorded as `other`.
  No new escalation name is added, since the driver would not branch on it.
  A unit cannot change the field through a file either: integration already refuses any sha that
  changes the project's `.tp/` (REFERENCE.md, Implement batches). A test pins this for this field,
  and a mutant that removes the refusal must turn it red.
- **Where and how it runs.**
  - Its working directory is the worktree's copy of the project directory: the worktree itself,
    or, when the project lives below the git top level, the same subdirectory inside it, so the
    working directory and `TP_MAIN_ROOT` name the same place in the two trees.
  - Its environment is the driver's, plus `TP_MAIN_ROOT` (the main tree's project root,
    absolute), `TP_UNIT_ID` (the task) and `TP_UNATTENDED=1`.
  - stdin is closed.
  - stdout and stderr go to `$TP_RUN_DIR/setup-<id>.log`. The unit has not been spawned yet, so
    it has no attempt log to write to.
  - Its timeout is `gate_timeout_seconds` (always resolved; 600 by default). On unix it runs in a
    process group of its own, and a timeout kills the whole group.
  - `SIGINT` or `SIGTERM` during a setup kills it, unwinds the batch and stops the run with
    `interrupted`, not `worktree-setup-failed`.
- **When.** Only where a worktree is made, which is at the batch start. The setups run one after
  another, in batch order, before any unit is spawned. A unit that runs alone runs in the main
  tree, which already holds the ignored files, so no setup runs for it. That covers a resumed wip
  task, a `solo` task and a batch of one. Crash recovery removes worktrees and makes none.
- **It leaves the tree clean.** After the command, the worktree's `HEAD` must name the same commit
  and branch as before it, and `git status --porcelain` in the worktree must print nothing,
  untracked files and submodules included, so only ignored files may be written. An untracked file
  would ride into the unit's commits, and it would break the clean-tree condition that v1.21.0's
  gate reuse rests on.
- **Its failure is its own.** A non-zero exit, a timeout, a moved `HEAD` or a dirty tree fails the
  batch start before any unit is spawned:
  - every task the batch claimed goes back to open;
  - every worktree and `tp-unit/` branch it added is removed;
  - no attempt is spent and `solo` is not set;
  - the run stops with a new stop reason, `worktree-setup-failed`, naming the task and the log
    path, and does not retry.

  The same unwinding applies to a failed `git worktree add`, with its own test, but that stop is
  `driver-error`: the driver could not make the worktree, which is not the setup failing. Before
  this release that failure left the batch claimed and its earlier worktrees in place. An unwind
  that leaves a worktree on disk or cannot release the tasks is a driver error as well; when it
  follows a failed setup, the result still names the setup and its log.
- **Reported, not counted as a unit.** The setup's wall time is written to its log and to a row of
  the run state's `setups`, since the run state has no gate time to sit beside. It is not a unit
  attempt, so `run_max_units` and the budget do not count it, but it is part of the run's elapsed
  time, which `run_max_wall_clock_seconds` and `totals.wall_clock_seconds` measure.
- **Unset changes almost nothing.** With no command a batch behaves as in v1.23.0, apart from a
  failed `git worktree add`, which now releases the batch. The Limits paragraph and
  `tp validate`'s `touches` warning name the field as the way past the cold worktree.
- **For a hand-driven parallel flow** this field does nothing; the remedy is `tp run` with
  `run_max_parallel` above 1.

## 2. Criterion, written before the build

The fixture is synthetic: its gate reads a large ignored directory that stands in for a build cache.
It ships if both of these hold:
- with the command cloning the cache, the gate in a batch worktree takes at most 1.5 times the main
  tree's warm gate, where without the command it pays the cold cost;
- the same tasks finish in less wall time at `run_max_parallel` 2 and 3 than at 1, with setup time
  included.

The fixture proves the mechanism, not the gain in a compiling project. The measurements file says
so, and the real figure belongs to the requesting project's own run. If the clone is no faster than
the cold path, the note is wrong and does not ship. Numbers go to `spec/1.24.0-measurements.md`.

## 3. Alternatives

- **A glob list of ignored paths to copy.** Refused: it cannot express a clone, a link or a fetch,
  and tp would have to choose a copy mechanism per filesystem.
- **Worktrees inside the repository, so a relative link reaches the cache.** Refused: v1.19.0 put
  worktrees outside the repository so that no tool scanning it sees them.
- **Setups run in parallel.** Not now. N commands would read one source at once, and a batch is at
  most eight units. Revisit if measurement shows serial setup dominating the wall time.
- **Keep worktrees between batches.** Refused for now: it reopens the crash-recovery and branch
  hygiene that v1.19.1 settled.

## 4. Non-goals

- tp does not learn any build tool's layout and ships no clone helper.
- No per-task setup.
- Review, audit and ground units never get a worktree, so no setup runs for them.
