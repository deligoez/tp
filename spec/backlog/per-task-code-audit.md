# tp — A task's code is audited once, soon after it closes

A backlog decision drafted on 2026-11-14 and revised on 2026-11-15 with the tp-smart session, after
the operator chose the narrow form (go-safety only) and the Decision track: built in a clone first, with
a field run before the tag. The measurements are in `spec/1.21.0-measurements.md`,
`spec/1.21.1-measurements.md` and, for this note's own runs, `spec/1.22.0-measurements.md`.

## 1. Context

- The release audit's code roles read the ten files with the most churn. In tp's history half of their
  findings, and 21 of 43 FAILs, sit on files beyond that cut, and a listed file's yield does not follow
  its rank.
- A trial gave a role the whole 121-file pool in one prompt. It graded every file and found a fraction
  of what ten-file prompts found on the same files; 8 of 10 of the control's extra findings were real.
  Depth needs small prompts.
- An agent's cost is mostly fixed. One three-file run cost about 81k tokens against 110–148k for the
  ten-file runs, so a file adds a few thousand and the agent itself most of the rest. That rests on one
  run, and the count is cumulative over the agent's turns, so part of it is the harness's, not tp's.
- A task's auditable files are few: in tp's 558 done tasks the median is 2 and the 90th percentile 6.

## 2. Decision

1. A new workflow field, `task_audit_roles`, lists the code roles that audit each task; default empty,
   tp's own project config lists `go-safety`. Only code roles may be listed, never `spec-coverage`.
2. `tp done` stamps the task with the roles the field lists at that moment. Tasks closed before the
   field is set carry no stamp and owe nothing.
3. **Batches.** The done tasks that owe a role and sit in no recorded batch are packed greedily, in close
   order (task id breaks a tie), into batches of at most ten auditable files — the task's commit files
   under the release audit's filters. A task with more than ten files is its own batch, cut into sorted
   slices of ten. The packing is computed from the task file alone and is prefix-stable: a later close
   never changes an earlier batch. A batch is due when it is full (exactly ten files, or the next task
   would overflow it), or when no implement unit is ready or running.
4. **The unit.** tp emits one audit unit per (batch, role), an audit emission of kind `task` through the
   release audit's path: the same file-check prompt and item ids, the inbox, `--record-inbox`, graded
   rows. It reads main as it stands and records the HEAD it read. The batch's members are frozen at
   emission and stored with it. The units have their own sequence, `task-audit-<n>`; they count toward
   neither `audit_max_rounds` nor the clean rounds, and an epoch cuts them like any round. The prompt
   adds one line: follow each call out of these files one hop into its callee to learn the callee's
   error and exit contract; a row still anchors at a line in the batch's own files. The line stays only
   if its trial run (§4) shows it earns its place.
5. **Findings do not block the close, and no fix unit runs mid-phase.** A task-audit row joins the
   release audit's carried set and is repaired by the existing audit fix path.
6. **Re-asking.** At the start of the audit phase, the files holding an unresolved task-audit row are
   graded again by a `task` emission over those files, packed the same way, so a row on a file outside
   the release cut is not left standing. A later PASS clears it; without a fix it keeps holding.
7. **Presence.** The audit does not converge while a task's stamped roles, intersected with the roles
   the field lists now, lack a recorded batch. A role removed from the field stops being owed; a role
   added later owes nothing on tasks already closed. `tp reopen` takes a task out of its batch; its old
   rows stay carried, and the re-close stamps and packs it again. A run that stops mid-phase leaves
   tasks unbatched; presence holds and the next run forms them.
8. The field is project config. Under `tp run` an agent does not change it, and accepting a task-audit
   finding without a change is `accept-finding`, as for any audit finding.

## 3. Cost

Batches of ten put the fixed cost at one agent per ten files. For a release like 1.8.0, which closed 57
tasks over about 170 auditable files, that is some 17 units: about 2M tokens for go-safety, against
about 4.5M for one unit per task and about 130k for today's capped role. The field run measures the real
figure before the tag.

## 4. Alternatives and cuts

- **Blocking the close, or a fix unit as soon as a row lands.** Early repair was never measured; the
  trial bought coverage and depth. It brings priority scheduling, discards of running parallel units and
  a fix-unit round. Revisit if the field run shows later tasks building on a found defect.
- **Reading at the task's own integrated commit.** The rows are about the code that ships.
- **Every N closes instead of ten files.** Ten files is the only prompt size whose depth was measured.
- **A step in the implementer's brief.** It is not independent; the depth came from fresh agents.
- **Chunking or raising the cap of the release pool.** The same tokens, every finding at the end; a long
  list is skimmed.

## 5. Non-goals and limits

- Other roles per task in tp's own config; a diff-only prompt; changing the release audit's cut.
- Auditing interactions between tasks. The callee hop of §2.4 is evidence for a row on a batch file,
  never a graded item of its own.
- The trial's historical FAIL was cross-file: a call in one task's file into another file's function
  that exits the process. Neither the whole-pool runs nor the three-file run found it. If the callee line
  does not find it either, depth on a task's own files does not reach a callee's contract, and that is a
  written limit of the role.

## 6. Field run

With real agents on a real task file, before the tag, its criteria written first:

- findings per batch, and a labelled sample of real against not real;
- tokens per unit against the agent's tool turns, and per file;
- files read outside the batch (the callee hop);
- the share of batches emitted partial, and the delay from a close to its batch's record;
- the carried task rows at the release audit, and whether each was asked again.
