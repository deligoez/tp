# BUGS — reproduced defects, fixed test-first

One line per defect: what the failing test asserts, and how to reproduce it. Each entry was
reproduced with a freshly built binary in a scratch project; the long transcripts are in the archived
backlog sidecars (`spec/backlog/ARCHIVE.md` gives the sha to read them at). Fix track: write the test
from the repro, watch it fail, fix, gate, review the diff, commit, **delete the line**. A defect whose
fix needs a design choice leaves this file for a decision note.

## Needs an operator decision

- **A task whose work is a `tp set --project` change is dispatched to a worktree unit, then its commit
  is refused.** Field report (2026-11-25, worktree pool, `run_max_parallel=4`): the unit had to change
  `.tp/config.json` with `tp set --workflow --project quality_gate=...`; `tp done` refused the commit
  ("changes .tp/config.json, which only tp writes"), and the coordinator closed it on the main tree.
  Choice: mark such tasks coordinator-only at dispatch, or accept a commit whose only `.tp/config.json`
  change is the one `tp set` produced.
- **`tp land` leaves the task file modified, so under `hc` every land costs a separate close commit.**
  Field report (2026-11-25): about ten extra commits in a 13-task run. Choice: say so in land's
  output, or let land commit the task file under the configured `commit_strategy`.

## Fix track

Field report of 2026-11-25 (v1.30.0 worktree pool): reproduce each before fixing.

- **The brief's close recipe omits `--file`.** In a repo with more than one task file, `tp done <id>
  --commit <sha> --claim-token <t>` run inside a worktree exits 3 (task file not found); it works with
  `--file <path>`. Test: the recipe a worktree brief prints names the task file.
- **`tp brief` carries no `touches`.** `.task` holds acceptance, id, source_sections, spec_excerpt and
  title, while the task file holds narrowed `touches` globs the pool uses. Test: a task with `touches`
  briefs them.
- **The worktree brief does not say to use absolute paths.** A unit's MCP file tool resolved a
  relative path against the main checkout and edited the wrong tree. Test: the worktree brief's runtime
  notes name the worktree's absolute path.
- **The 300-character description limit is enforced only by validate/import.** The skill's
  decomposition rules never state it; a first import failed on 8 descriptions. Test: SKILL.md's
  decomposition rules name the limit.

Found in this repo (2026-11-25):

- **Resolving a carried finding `fixed` after the round that read the repair leaves it
  `fixed-unverified`.** Review round 4 of 1.31.0 read the repaired spec and found no high finding, but
  rounds 1-3 were resolved after round 4 was recorded, so `--status --check` still exits 1 with 16
  carried rows. Repro: record round N+1 over a repaired spec, then `--resolve-all fixed` round N; the
  rows stay `fixed-unverified`. Either the next_action tells the agent to resolve before the next round,
  or a later clean round that read the repaired text verifies them.
- **A conflicting integration crashes `tp run` with exit 4.** Run 01M87V6B84WEHERP4ETZPXHBH2:
  `found-apply-attach`'s cherry-pick stopped on a `brief.go` conflict with `found-brief`, landed
  earlier; the driver's `git reset --keep <pre>` then exited 128 on the unmerged index, leaving main
  with 2 of 12 commits and the cherry-pick in progress. An earlier conflict (`found-add-cmd`) reported
  the same 128 and reopened the task. Test: a unit whose commits conflict with main is aborted
  (`cherry-pick --abort`) before the reset, main is back at its pre-integration commit, and the task
  reopens; the message says conflict, not exit 128.
- **Task-audit record units leave their round files uncommitted on main.** Under `tp run`,
  `spec/.tp-review/<v>/task-audit-N.ndjson` and `state.json` stay untracked/modified after each record
  unit, so every later close warns "3 uncommitted change(s) not on the keep-list". Test: after a
  task-audit record unit, `git status` on main shows no round file.

Field report of 2026-11-25 (a requesting project, Agent-tool pool, v1.30.0):

- **`tp land` exits 0 when a task is blocked.** A land that hit `.git/index.lock: File exists` listed
  the task under `blocked` and exited 0; the coordinator committed the task file as if it had landed.
  Test: a land with a non-empty `blocked` exits non-zero, and an index.lock is retried.
## Decided elsewhere, not a fix

- **An auto-detected file list beyond 10 is cut per code role, and an inline spec excerpt stops at
  10 KB.** A named `--affected-files` list is not cut since v1.15.0, and every code role's round says
  `graded N of M` in `--record`, `--status` and `next_action`. The spec cut drops prose only: each
  spec-coverage checklist item carries its own `text`, and the marker names the path to read the rest.
  Measured on tp's shipped specs (`spec/1.20.0-measurements.md`): 21 of 22 audited specs exceed the
  cut, 65% of spec-coverage rows grade a section past it, and those rows hold 15 FAILs and 78
  PARTIALs, so the rest was read. The code roles' cut was measured from history in
  `spec/1.21.0-measurements.md`: half of their findings sit beyond rank 10 and are real, and yield does
  not follow churn. The constructed full-list round ran in v1.21.1 and showed one long prompt skims;
  the depth now comes from v1.22.0's task audit of every closed task's own files, and the cut stays
  (`spec/undecided.md`, *The divisible round*, closed).
