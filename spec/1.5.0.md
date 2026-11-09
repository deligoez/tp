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
rules respected, whose `epic` names it. A binding clears the epic's claim.

## 3. Status

| status | holds when |
|---|---|
| `planned` | no bound file, no done record, no claim |
| `claimed` | no bound file, no done record, a claim |
| `specced` | a bound file, and no task of any bound file is `wip` or `done` |
| `in_progress` | a task of a bound file is `wip` or `done`, and not every bound file is at `release` |
| `done` | every bound file is at `release`, or, with no bound file, a done record |

`release` is the phase `tp resume` already reports, so the epic layer and the task layer cannot
disagree about one file. There is no `implemented` status and no per-epic check list. `done_at` is the
done record's time, else the latest `closed_at` across the bound files.

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
- A gate is met once. Whether a later measurement falls back is the business dashboard's question,
  not the plan's.

## 5. Conditions

A condition epic carries `condition: true` and holds what it waits for in its `acceptance`; a work epic
that must not start before it depends on it, like any edge.

- No task file binds to a condition epic, and it cannot be claimed.
- **`tp epic done <id> --evidence "<text or URL>"`** writes `done: {at, evidence}` on an epic with no
  bound file, condition or not; it is refused on an epic with a bound file, whose status is derived,
  and on a condition whose acceptance is empty. `tp epic done <id> --undo` removes the record.

## 6. The graph gates the work

`tp next`, `tp next --peek`, `tp ready` and `tp claim` on a bound task file read the graph first:
while any dependency of the file's epic is not `done`, no task is ready, the response carries
`blocked_by_epics` naming them, and `tp claim` exits 1. An unbound file behaves as it does at
`v1.4.2`. A task's `depends_on` keeps naming tasks of its own file.

## 7. Epic commands

- **`tp epic ready [--all]`** lists the work epics that can start: in an open milestone, every
  dependency `done`, no bound file, no claim, not a condition. The conditions whose own dependencies
  are done are listed apart, as waiting on the operator, and so are the gates whose members are done
  and whose threshold is not yet met. `--all` lifts the open-milestone rule.
- **`tp epic claim <id> [--by <name>]`** writes `claimed: {at, by}` under the file's lock and exits 1
  when a claim exists; `--release` removes it.
- **`tp epic gate <m>`** reports each member `green` or `red`, the deferrable epics apart, and the
  threshold and its met record; it exits 0 only when the gate is green.
- **`tp epic status [--milestone <m>]`** reports, per milestone: whether it is open; counts by status
  and the share done; the gate's state; the ready list and the blocked list with each blocking epic;
  what waits on the operator; `critical_path`, the longest chain of not-done epics to the milestone's
  gate, by count, and by summed estimate when every epic on it has one; `throughput`, epics reaching
  `done` per ISO week of `done_at`; and the estimate-versus-actual roll-up `tp report` computes per
  task, summed per epic. An epic's optional `estimate_minutes` counts when it is set; otherwise its
  estimate is the sum of its bound tasks'.
- **`tp epic export`** prints the graph as JSON when given no format flag, `--md` keeping the tables.
  The JSON carries a schema `version` and every derived field, and for each bound file its path,
  phase, task counts by status, and each loop's round and state; it lists every epic review slice
  with its key and state. It is the contract a view of the plan reads, so that no reader of the plan
  parses tp's state files or derives a status a second time; each epic review state directory records
  its slice's key for it.

Under `TP_UNATTENDED`, `tp epic done`, `tp epic gate --met` and `tp set --epic` exit 2: marking work
done without its files, deciding a threshold and moving a file between epics are the operator's.
`tp epic claim` runs, since an agent claims its own work.

## 8. Schema

An epic gains `condition` (written only when true), `estimate_minutes` (written only when set),
`claimed` and `done`; a gate gains `threshold` and `met`. `claimed`, `done` and `met` are written only
by their commands. A task file gains `epic`. A file without them reads as before.

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
| 2 | b is `planned`, then `claimed`, then `specced` once a file binds it, `in_progress` once a task is `wip`, and `done` once the file is at `release` | the chain, one file bound to b | a status that reads `done` from the tasks alone, before the audit |
| 3 | `tp next` on b's file reports no task and `blocked_by_epics: [c]` while c is not done, and the task once c is done | the chain, b's file with one open task | a `tp next` that ignores the graph |
| 4 | `tp epic done c --evidence <e>` records c done; the same on b while b has a bound file exits 1; `--undo` removes c's record | the chain, b bound | a done refused on the wrong epic |
| 5 | a condition epic with an empty acceptance is refused `done`; with its acceptance written it is accepted | a root condition epic | an acceptance check on the evidence instead |
| 6 | an epic of milestone B with done dependencies is not in `tp epic ready` while A's gate is red, is listed with `--all`, and is listed once A's gate is green | `milestones: [[A, B]]`, a in A, b in B | a ready list that ignores milestone order |
| 7 | with a gate named only on B, B opens once A's epics are done and C opens only once B's gate is green | `milestones: [[A, B, C]]`, an epic in each, a gate on B | a milestone `gates` does not name counted green |
| 8 | `tp epic gate A --met` is refused while a member is not done and while the threshold is empty, and accepted once both hold; the gate is then green | the chain with a gate on A carrying a threshold and no `members` | a met accepted before the members are done |
| 9 | a condition epic of A is not a member of A's default gate, and a work epic listed `deferrable` does not hold it | the chain plus a condition and a deferrable epic in A | a default membership that counts conditions |
| 10 | `tp epic ready` lists a condition with done dependencies apart from the work epics | the chain plus a root condition epic | a condition listed among the work epics |
| 11 | a second `tp epic claim b` exits 1; `--release` then lets it through | the chain | a claim that overwrites an existing one |
| 12 | `tp epic status` reports the chain's not-done epics as the critical path by count | the chain, a done | a path that counts done epics |
| 13 | `tp epic export` prints JSON with a `version`, b's bound file with its phase, and one recorded epic review slice with its key | the chain, b bound, one slice of `--epic b` recorded | a slice listed without its key |
| 14 | under `TP_UNATTENDED`, `tp epic done` and `tp epic gate --met` exit 2 and `tp epic claim` exits 0 | the chain | a fence on claim |
