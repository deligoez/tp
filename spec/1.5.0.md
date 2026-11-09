# tp v1.5.0 — the plan tracks its own progress

> Full track. The second release of `spec/backlog/epics.md` (§9). The interview record and the field
> evidence behind these decisions are in `spec/1.5.0-measurements.md`.

## 1. Decision

v1.3.0 and v1.4 made the plan a reviewed graph of epics. Nothing yet connects that graph to the work:
a task file does not know its epic, an epic has no status, a gate moves nothing, and "what can start
now" is still read by hand from milestone order, gates and dependencies. This release connects them,
and brings the conditions a plan waits on into the graph instead of beside it.

1. **Binding.** A task file names at most one epic in a new top-level field `epic`; an epic may have
   several bound files. Bound files are found by walking the repository's `*.tasks.json`.
2. **Derived status.** An epic's status is computed when it is read, never stored: from its bound
   files' `tp resume` phases, or, for an epic with no bound file, from a done record.
3. **Gates open milestones.** A milestone is open when every earlier milestone of its track has a green
   gate; only an open milestone's epics are ready. A gate may carry an exit threshold, which the
   operator records as met, with evidence, once the gate's members are done.
4. **Conditions open work.** An epic marked `condition: true` is a precondition the operator decides,
   such as an external event that must happen before its dependents start. It holds no task file and
   is done only by an evidenced record.
5. **The graph gates the work.** `tp next`, `tp ready` and `tp claim` on a bound task file consult the
   epic's dependencies: while one is not done, no task of the file is ready.
6. **Epic commands.** `tp epic ready`, `claim`, `done`, `gate` and `status`, and a JSON `tp epic
   export` with every derived field: the contract a view of the plan reads.

tp measures nothing and runs no command to decide any of it: whether code is right is the bound
spec's checks and audit, and whether a threshold is met is the operator's evidenced record.

## 2. Binding

`tp init <spec> --epic <id>` and `tp set --epic <id>` write `epic`; `tp set --epic ""` removes it. A
binding is refused when the graph does not hold the id, when the epic is a condition, or when the epic
carries a done record. One spec covering a family of epics is written as one spec per epic referring
to a shared design note, so that blocking stays exact: a later epic's dependencies never hold back an
earlier one's tasks.

An epic's bound files are every tracked or untracked `*.tasks.json` under the repository root, ignore
rules respected, whose `epic` names it.

## 3. Status

| status | holds when |
|---|---|
| `planned` | no bound file, no done record, no claim |
| `claimed` | no bound file, no done record, a claim |
| `specced` | a bound file, and no task of any bound file is `wip` or `done` |
| `in_progress` | a task of a bound file is `wip` or `done`, and not every bound file is at `release` |
| `done` | every bound file is at `release`, or, with no bound file, a done record |

`release` is the phase `tp resume` already reports, so the epic layer and the task layer cannot
disagree about one file; a bound spec edited after its release falls back to `audit`, and its epic
is in progress again until the audit converges. There is no `implemented` status and no per-epic
check list. A claim on an epic with a bound file is ignored. `done_at` is the done record's time,
else the time of the audit round that brought the last bound file to `release` most recently: it
moves when an epic reopens and is done again.

## 4. Milestones and gates

A milestone's gate has two lists. Its `members` hold it; when the gate names no `members`, or `gates`
names no entry for the milestone, its members are the milestone's epics that are neither conditions nor
listed `deferrable`. Its `deferrable` epics never hold it, so work that waits on a trigger that may
never come is listed there. A gate may carry `threshold`, the exit condition in the operator's words.
Every milestone therefore has a gate.

- **Green** means every member is `done` and, when the gate has a threshold, a met record exists.
- **`tp epic gate <m> --met --evidence "<text or URL>"`** writes `met: {at, evidence, threshold}`,
  copying the threshold it was judged against. It is refused while a member is not done, since a
  threshold is measured after the work, and while the threshold is empty, since it is written before
  the measurement. A threshold edited after its met record shows as changed in `tp epic gate` and
  `tp epic status`; the record stands.
- **Open.** A milestone is open when every earlier milestone of its track has a green gate; the first
  milestone of a track is always open, and tracks do not wait on each other.
- A threshold is met once: whether a later measurement falls back is the business dashboard's
  question, not the plan's. The gate's members are read live, so a member that falls back turns the
  gate red again while its met record stands.

## 5. Conditions

A condition epic carries `condition: true` and holds what it waits for in its `acceptance`; a work epic
that must not start before it depends on it, like any edge.

- No task file binds to a condition epic, and it cannot be claimed; the other way round, `condition:
  true` is refused on an epic that has a bound file or a claim, so a condition never has either.
- **`tp epic done <id> --evidence "<text or URL>"`** writes `done: {at, evidence}` on an epic with no
  bound file, condition or not; it is refused on an epic with a bound file, whose status is derived,
  and on a condition whose acceptance is empty. `tp epic done <id> --undo` removes the record.

## 6. The graph gates the work

A dependency in this release is always a direct one, for a bound file, for `tp epic ready`, for a
condition and for the blocked list: an epic's own `depends_on` edges, and a done record on a
dependency is the operator's statement that it is done, whatever that dependency's own dependencies
are. A bound file's epic is blocked while one of its dependencies is not `done`. Work already in
flight is not held back: the block stops starting a task, not finishing one. While the epic is
blocked, `tp next`, `tp next --peek`, `tp ready` and `tp claim` offer no task to start and carry
`blocked_by_epics` naming the dependencies, `tp claim` exits 1, `tp done` refuses a task that is not
already `wip`, and the next-task counts `tp done` reports leave the blocked tasks out. A `wip` task
stays resumable and closable: `tp next` returns it, and `tp resume` offers its implement unit as it
does today. With no `wip` task, `tp resume` reports the block as a blocker and offers no implement
unit, so `tp run` never drives a unit that cannot move. Only dependencies gate a bound file:
binding an epic is the decision to start it, so a milestone not yet open does not hold its tasks back.
A bound file whose epic the graph does not hold, or with no graph file, behaves as unbound and says
so. An unbound file behaves as it does at `v1.4.2`. A task's `depends_on` keeps naming tasks of its
own file.

## 7. Epic commands

- **`tp epic ready [--all]`** lists the work epics that can start: in an open milestone, not done,
  every dependency `done`, no bound file, no claim, not a condition. Listed apart, as waiting on the
  operator, are the conditions that are not done, whose dependencies are done and whose milestone is
  open, and the gates whose members are done and whose threshold is not yet met. `--all` lifts the
  open-milestone rule. An epic whose milestone is in no track is never ready; that is a lint error.
- **`tp epic claim <id> [--by <name>]`** writes `claimed: {at, by}` under the file's lock; it exits 1
  when a claim exists, and on a condition, a done epic or an epic with a bound file. A claim reserves:
  it is not refused for a blocked epic or a closed milestone. `--release` removes it. `tp epic ready`
  and `tp epic status` name each claim's holder, the answer to "who has it" when several agents work.
- **`tp epic gate <m>`** reports each member `green` or `red`, the deferrable epics apart, and the
  threshold and its met record; it exits 0 only when the gate is green.
- **`tp epic status [--milestone <m>]`** reports, per milestone: whether it is open; counts by status
  and the share done; the gate's state; the ready list and the blocked list with each blocking epic;
  what waits on the operator; `critical_path`; `throughput`, epics whose current `done_at` falls in
  each ISO week, so an epic that reopens and is done again counts in its later week; and the
  estimate-versus-actual roll-up `tp report` computes per task, summed per epic. Under `--compact`,
  the estimate chain, `throughput` and the roll-up are left out.
- **`critical_path`** is the longest chain, by count, of not-done epics joined by dependency edges
  and ending at a not-done member of the milestone's gate; a done epic cuts a chain, and a chain may
  cross milestones and tracks. The heaviest such chain by summed estimate is reported only when every
  epic on every such chain, the gate members included, has an estimate: an epic's `estimate_minutes`
  when set, else the sum of its bound tasks' estimates when every one of them has one, else none.
- **`tp epic export`** prints the graph as JSON when given no format flag, `--md` keeping the tables.
  The JSON carries a schema `version` and every derived field, and for each bound file its path,
  phase, task counts by status, and each loop's round and state; it lists every epic review slice
  with its key and state. It is the contract a view of the plan reads, so that no reader of the plan
  parses tp's state files or derives a status a second time. Each epic review emission records its
  slice's key in the slice's state directory; a slice recorded before v1.5.0 is listed with `key:
  null` until its next emission.

Under `TP_UNATTENDED`, every write of a done record, a met record or a binding exits 2, the removals
included: `tp epic done` and its `--undo`, `tp epic gate --met`, and `tp init --epic` and
`tp set --epic`, an empty value too. Marking work done or undone without its files, deciding a
threshold and choosing which epic a file starts are the operator's. A unit escalates them as the
decisions `epic-done`, `gate-met` and `epic-binding`. `tp epic claim` runs, since an agent reserves
its own work.

## 8. Schema

An epic gains `condition` (written only when true), `estimate_minutes` (written only when set),
`claimed` and `done`; a gate gains `threshold` and `met`. `claimed`, `done` and `met` are written by
their commands and by `tp epic import`, the migration; a writer that replaces a whole object (`set
gates=`, a `--bulk` row) carries them over and refuses a payload that names them. An epic review
block shows `condition` and a gate's `threshold`, so editing either re-asks its unit. A task file
gains `epic`. A file without them reads as before.

## 9. Non-goals

1. **No command runs at read time** and no per-epic check list.
2. **No sustained metric tracking**; a gate or a condition is met once.
3. **No binding of one task file to several epics**, and no per-task epic.
4. **No planned dates.** A plan-versus-actual by date waits until a plan carries dates.
5. **No change to `tp resume`'s phases** or to how a single spec's loop converges.

## 10. Tests

The counts of each row are confirmed by the task that builds it, once its subject exists.

| # | Behaviour | Fixture | Mutant that turns it red |
|---|---|---|---|
| 1 | `tp set --epic b` binds a task file; an absent id, a condition and a done epic are refused and nothing is written | the chain a→b→c→d, one task file | a binding written before the id is checked |
| 2 | b is `planned`, then `claimed`, then `specced` once a file binds it, `in_progress` once a task is `wip`, and `done` once the file is at `release` | the chain with c and d done, one file bound to b | a status that reads `done` from the tasks alone, before the audit |
| 3 | while c is not done, `tp next` on b's file reports no task and `blocked_by_epics: [c]`, `tp resume` reports a blocker and no implement unit, and `tp done` on an open task exits 1; with one task `wip`, `tp next` returns it and `tp resume` offers its unit; once c is done, the next task is offered | the chain with d done, b's file with two open tasks | a `tp resume` that hides the `wip` task's unit, or one that ignores the graph |
| 4 | `tp epic done c --evidence <e>` records c done; the same on b while b has a bound file exits 1; `--undo` removes c's record | the chain, b bound | a done refused on the wrong epic |
| 5 | a condition epic with an empty acceptance is refused `done`; with its acceptance written it is accepted | a root condition epic | an acceptance check on the evidence instead |
| 6 | an epic of milestone B with no dependencies is not in `tp epic ready` while A's gate is red, is listed with `--all`, and is listed once A's gate is green | `milestones: [[A, B]]`, x not done in A, y in B with no dependencies | a ready list that ignores milestone order |
| 7 | with a gate named only on B, B opens once A's epics are done and C opens only once B's gate is green | `milestones: [[A, B, C]]`, an epic in each, a gate on B | a milestone `gates` does not name counted green |
| 8 | `tp epic gate A --met` is refused while a member is not done and while the threshold is empty, and accepted once both hold; the gate is then green | the chain with a gate on A carrying a threshold and no `members` | a met accepted before the members are done |
| 9 | a condition epic of A is not a member of A's default gate, and a work epic listed `deferrable` does not hold it | the chain plus a condition and a deferrable epic in A | a default membership that counts conditions |
| 10 | `tp epic ready` lists a condition with done dependencies apart from the work epics | the chain plus a root condition epic | a condition listed among the work epics |
| 11 | a second `tp epic claim b` exits 1; `--release` then lets it through | the chain | a claim that overwrites an existing one |
| 12 | `tp epic status` reports the not-done epics before a done one as the critical path by count, the done epic cutting the chain | the chain with b done and A's gate on a | a path that counts or crosses done epics |
| 13 | `tp epic export` prints JSON with a `version`, b's bound file with its phase, and one recorded epic review slice with its key | the chain, b bound, one slice of `--epic b` recorded | a slice listed without its key |
| 14 | under `TP_UNATTENDED`, `tp epic done`, `tp epic done --undo`, `tp epic gate --met` and `tp set --epic ""` exit 2 and `tp epic claim` exits 0 | the chain, b's file bound | a fence on claim, or none on a removal |
