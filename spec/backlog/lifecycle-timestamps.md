# tp — A task file records when it was bound and when its tasks were written

A backlog decision candidate, named by slug; its priority and release number are assigned later. It
comes out of `dashboard.md` §4 ("History"): the dashboard can lay a spec's and an epic's path out over
time from what tp already records, except two moments tp never writes down.

Class: **schema** — two optional keys on the task file; no command changes what it decides.

## 1. The decision

**Context.** The moments of the lifecycle are recorded where they happen: a claim as `claimed.at` in
the graph, a done record as `done.at`, a met gate as `met.at`, each round as `recorded_at`, each task
as `started_at` and `closed_at`. Two are not:

- **When a spec was bound to its epic.** `tp init --epic` binds at creation, so the task file's
  `created_at` is that time. `tp set --epic` binds an existing file and leaves no time; `updated_at`
  moves on every later write. An epic's move from claimed or planned to specced has no date then.
- **When the tasks were written.** `tp import` fills the task file and records no time, so the end of
  decompose, and how long decomposing took, are not on disk.

**Decision (proposed).** The task file gains two optional top-level keys, written by the commands that
make the fact true, in the form `claimed.at` already uses (RFC 3339, UTC):

- `bound_at`: written by `tp init --epic` and `tp set --epic <id>`; rewritten when the file is bound
  to another epic; removed by `tp set --epic ""` together with `epic`.
- `imported_at`: written by `tp import`, rewritten by `tp import --force`.

A file written before this release has neither key and reads exactly as it does today: a reader that
needs the time falls back to what it can derive (`created_at` for a file `tp init --epic` made) or
says it does not know. `tp epic export` carries both per bound file, so the dashboard reads them from
the contract rather than from the file.

## 2. Alternatives

- **An event log** (`.tp/events.ndjson`, one line per write). Exact transitions, a replay for free.
  Rejected: tp derives status rather than storing it, and a log is a second copy of the truth that
  drifts the first time a file is edited by hand; an append-only file every branch and parallel agent
  writes is a merge conflict by design.
- **Git history of the task file.** Needs no change. Rejected as the only source: not every project
  commits its task files, an uncommitted bind is invisible, and a commit's time is when someone
  committed, not when tp bound.
- **The dashboard's own snapshots.** Fine for trends, which `dashboard.md` leaves to it; wrong for a
  fact that should outlive the dashboard and exist before it first ran.

## 3. Non-goals

No history of reversals: a task reopened, a binding removed or a claim released keeps only its current
state. No time on any other key. No change to how status, phase or `tp resume` are derived.

## 4. Questions

1. Does unbinding keep `bound_at` as a record that the file was once bound? **Proposed: no; the key
   follows `epic`, as `claimed` follows the claim.**
2. Does any later import into the same file move `imported_at`? **Proposed: yes; it is the last time
   tp wrote the file's tasks.**
3. Track: decision. Two roles, at most two rounds, built in a clone first.
