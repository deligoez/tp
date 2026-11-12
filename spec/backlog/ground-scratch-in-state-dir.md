# tp — The ground scratch file lives in the spec's state directory

A backlog decision, decided by the operator; it replaces the BUGS.md entry "Two specs with the same
base in different directories share the ground scratch file".

Class: **decision**.

## 1. The decision

**Context.** `tp ground` names its scratch file from the spec's base (`ground-<base>-r<N>.ndjson`),
so `x/a.md` and `y/a.md` grounded from one working directory write the same file. Their state
directories do not collide, since those are per spec directory.

**Decision.** The scratch file moves into a subdirectory of the spec's own state directory, apart
from the recorded `ground-round-N.ndjson`, as epic review keeps role output in an inbox: identity
comes from location, and an in-flight file cannot be mistaken for a recorded round. The ground
prompt names the new path; `--record`, the dirty-state classifier, `scripts/clean-emissions.sh` and
any plugin hook that grants a ground unit a write path follow it. There is no fallback read of the
old path, because the old path is the one that collides: an unrecorded round's scratch file found
there is named in a refusal that says where to move it.

Before building, check whether `TP_ROUND_DIR` under a run is derived from the task file's base in
the same way; if two same-base specs share a round directory, review and audit role parts collide
too, and the fix covers them.

**Checked before the build (2026-11-12, at 59975e14).**

- *`TP_ROUND_DIR` collides the same way.* `RoundDir` (`internal/engine/childenv.go`) is
  `<root>/.tp/rounds/<base>/<phase>-r<N>`, where `<base>` is `RunBase`, the task file's name with
  `.tasks.json` removed. `x/a.tasks.json` and `y/a.tasks.json` therefore share every round
  directory. The directory is keyed by the cycle and outlives a run on purpose, so the second
  cycle's oracle reads the first cycle's `role-<id>.ndjson` as its own finished roles and its
  record unit merges them. The fix covers it.
- *How it is covered.* The round directory stays under `.tp/rounds/`, which `.tp/.gitignore`
  ignores, and is keyed by the task file's location: its directory relative to the project root,
  then its base. A task file at the root keeps today's path. Moving it into the spec's state
  directory instead would turn role parts and `merged.ndjson`, which `review-resolve` keeps writing
  into, from ignored run artifacts into tracked bookkeeping that a commit under `hc` picks up. The
  rejected path-derived slug is a different case: there the name is one an agent is told to write
  and the operator retypes, while here tp derives the directory from the task file path it has
  already resolved, so `x/a.tasks.json`, `./x/a.tasks.json` and an absolute spelling name one
  directory.
- *Left out.* `run-<base>.json`, `locks/run-<base>.lock` and `last_failure-<base>.json` are keyed
  by the same base and collide the same way between two such task files; they are run state, not
  round artifacts, and stay as they are here.
- *The plugin.* `hooks/pre-tool-use-write-deny.sh` denies every write under `.tp-review/`, so the
  scratch file in its new place is granted there by exact shape, as the epic review inbox file is.
  That is plugin content (1.12.0).

**Consequences.** The path in the ground prompt changes. An in-flight scratch file now sits inside
the repository under the spec, so it shows in the working tree's status until `--record`, which
removes it. A plugin hook change is plugin content and a version bump.

## 2. Alternatives

- **A path-derived slug.** Rejected: the same spec reached as `x/a.md`, `./x/a.md` or an absolute
  path would name three files, and `--record` could not find the one written.
- **Stamp the owner in the file and refuse a mismatch.** Rejected: an agent writes the file, so the
  stamp is something tp would ask an agent to get right.
- **Read the old path once for compatibility.** Rejected: a file there may be the other spec's.

## 3. Non-goals

- Changing the ground payload's other fields.
