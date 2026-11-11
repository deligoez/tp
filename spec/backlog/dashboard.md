# tp dashboard — one view of the plan, the specs and every loop

> Decision candidate, rewritten against the v1.6.0 export as shipped and updated for v1.7.0. The
> operator answered §6's questions 1, 6 and 12; §4 records a web research pass on graph rendering
> and dashboard design, and the operator's reviews of prototypes 1 and 2, which add a Spec tab: one
> spec at a time, read beside its tasks. v1.7.0 moved the export to `version: 2`, which this
> candidate reads: a `depends_on` entry may be `{any_of, why}`, and `epics[]` add
> `blocked_by_any_of`, `held_by_any_of`, `blocked`, `held` and `moot` (spec/1.7.0.md §7).

## Decision (proposed)

`tp dashboard [--port N]` serves one static page, embedded in the binary, on `127.0.0.1`. Its data
is `tp epic export`'s JSON and a read model of one spec (its text, tasks, loops and `tp resume` view,
with the list of task files to pick from, §4), both produced in-process by the code the commands
use, re-derived when tp's files change and pushed to the page. Two tabs: Epics draws the plan (map,
chain, lists, matrix, plan review); Spec shows one spec at a time, its text read beside its tasks.
Every detail opens in one drawer. It writes nothing and exposes no write endpoint. Stdlib only
(`net/http`, `embed`, Server-Sent Events); no JS framework or CDN.

**Measured on v1.6.0 (built from `main`, synthetic graphs made with tp's own writers):**

| graph | export bytes | wall time | split by key (pretty JSON) |
|---|---|---|---|
| 7 epics, 2 tracks, `opens_after`, 1 condition, 1 bound file (3 tasks), 1 claim, 1 slice | 11.6 KB | ~20 ms | graph 1.7 K, milestones 5.5 K, epics 2.8 K |
| 530 epics, 6×6 milestones, ~3 edges each, no bound files | 736 KB | 30 ms | graph 237 K, milestones 234 K, epics 198 K |
| same, 100 bound files | 795 KB | 40 ms | — |

So ~1.5 KB per epic; the field's "~1 MB at 530" is the same order. `milestones` repeats per-epic
facts (`ready`, `blocked`, `gate.members`) that `epics[]` already holds: about a third of the bytes.

## 1. What it shows, field by field

| Dashboard element | v1.6.0 export source |
|---|---|
| Lanes, milestone boxes, opening order | `graph.milestones`, `milestones[].opens_after`, `.open`, `.waiting_on` |
| Epic node: label, colour, condition shape | `epics[].id/title/milestone/status/condition` |
| Edge with its why, tags | `graph.epics[].depends_on[{id,why}]`, `.tags` |
| Blocked / held badges | `epics[].blocked_by`, `.held_by`; condition `actionable`, `not_actionable` |
| Gate marker (green/red, threshold, met) | `milestones[].gate` |
| Critical path highlight | top-level `critical_path.by_count`; per milestone `critical_path.by_estimate` |
| "Start next" ranking | `epics[].downstream`, `milestones[].ready`, `.claimed`, `.operator` |
| KPI: share done, counts | `milestones[].share_done`, `.counts` (work epics; `conditions_done` apart) |
| KPI: throughput | `milestones[].throughput` (ISO weeks) |
| KPI: estimate vs actual | `milestones[].rollup[{estimate_minutes, actual_minutes, tracked, estimate_total}]` |
| Bound spec, phase, loop state | `epics[].files[{path, phase, tasks, review, audit, ground}]` |
| Epic review slices | `slices[{dir, key, round, state}]` (`in_flight` seen live after an emission) |

**Gaps** (each with where it belongs):

1. **Per-task detail** (id, title, status, deps, estimate, closure evidence, commit shas). Only
   `tasks: {open, wip, done}` counts today. → *export*, opt-in (`--tasks`) so the default stays small.
2. **Per-round loop history** (findings per round, severity, resolved/open, convergence signals).
   Only `{round, state}`. → *export*, opt-in; it is what `tp review/audit --status` already derive.
3. **Run/unit events** (`tp run` in flight: current unit, attempt, spend, stop reason). Not in the
   export; `.tp/run-<base>.json` holds it and `tp run --status` derives `run_state`. → *separate
   stream* (it changes per unit, not per plan edit); v1 reads `tp run --status`'s payload per file.
4. **Specs bound to no epic** (tp's own repo has no graph). Export exits 3 without `.tp/epics.json`.
   → *a project-wide export* (`tp export`?) listing every task file with phase and loops.
5. **A change token**: no `revision`/`generated_at` in the payload. → *export*, additive (no bump).
6. **Naming trap seen in the run:** `rollup.estimate_minutes` is 0 while `estimate_total` is 75 for
   the same epic (closed-task estimate vs all tasks). The page must label them; worth a doc line.

## 2. Real time

| Option | Cost on ~530 epics | Misses |
|---|---|---|
| A. Page polls `/export` every N s | 40 ms CPU + ~0.8 MB per tick per tab | nothing; wastes work when idle |
| B. Server re-exports on a tick, hashes bytes, pushes over SSE only on change | 40 ms per tick (2 % CPU at 2 s), one push per real change | nothing: it reads tp's derivation, not a path list |
| C. Watch files (`.tp/`, `spec/.tp-review/`, `*.tasks.json`, `.tp/run-*.json`), re-export on event | ~0 idle; needs `fsnotify` (stdlib has none) or a stat walk | any path tp starts writing that the list forgot |
| D. Event stream from `tp run` | needs a writer-side protocol | everything not under `tp run` (manual commands) |

**Recommend B** for v1: no dependency, no path list to keep in sync with tp's layout, correct by
construction, and the cost is measured, not guessed. Add a stat fingerprint (C without fsnotify)
only if the field graph's export proves slower than ~100 ms. D waits for gap 3.

## 3. Delivery and security

Web first (zoom/pan over hundreds of nodes; a TUI cannot draw the graph). One `index.html` + JS/CSS
via `embed`, `GET /` , `GET /export` (ETag = byte hash), `GET /events` (SSE). Bind `127.0.0.1`
only; reject requests whose `Host` is not `127.0.0.1:<port>`/`localhost:<port>` (DNS rebinding);
every non-GET is 405; no CORS headers; no external fetches from the page. Print the URL on start.
An export failure (exit 3, a malformed task file) is shown as a banner with tp's error and hint,
keeping the last good map greyed.

## 4. Map layout

**tp computes the layout; the page only draws it.** The graph is already layered — tracks are
lanes, milestones are columns, dependency depth orders epics inside a milestone — so no general
graph-layout library is needed, and computing it in Go keeps it deterministic, testable and in line
with the non-goal "no second derivation in JS". The export gains an additive per-epic `layout`
(`lane`, `column`, `order`), `order` from one barycenter pass per column to cut edge crossings (the
Sugiyama step that matters). Rendering is SVG; edges move to a canvas only if a field graph proves
SVG slow. Rejected: deck.gl (a WebGL engine for millions of points whose graph layers are in a
reboot and ship a force layout, wrong for a layered DAG; WebGL pays off past roughly ten thousand
nodes, the field graph has ~530) and ELK.js/dagre (layout engines for graphs without layers; tp's
layers are given, and either would be the page's one dependency).

- **Swimlane per track**, milestones as boxes along it. x-position = opening depth (longest
  `opens_after` chain to the milestone), so cross-track `opens_after` reads left to right and
  `[]` siblings share a column. Closed milestones dimmed, with `waiting_on` on hover.
- **Epics as nodes** inside their milestone box, placed by `layout`; status by colour AND shape or
  icon (done ✓, ready ●, blocked ⊘, held ⏸, moot ◌), never colour alone; conditions a distinct
  shape, actionable ones outlined as "operator".
- **Edges** drawn on hover/selection by default (530×3 always-on edges is a hairball); the selected
  epic's upstream and downstream highlighted and the rest dimmed; `why` on edge hover; an any-of
  group drawn as one forked edge labelled "one of".
- **Gate** as a bar at the right edge of each milestone box: green/red, member count, threshold.
- **Critical path** always drawn as a thick line through `critical_path.by_count`.
- **Filters:** open milestones only, one track, one tag, status, "ready now"; each active filter a
  removable chip, the filter state in the URL.
- **Detail panel:** epic (title, status, deps with whys, downstream, held_by) → bound files (path,
  phase, tasks bar) → loops (ground/review/audit round + state) → tasks (gap 1) → slices naming it.
- **Live change:** an epic whose fields changed in the last push glows briefly.

### What the field artifact taught

The field project's own map (about 500 epics, ~2,200 edges, one self-contained page with the graph
inline) had four views; answers came from the list and the dependency tree, and the map was the
showpiece. Its lessons, in shape: depth bands read well, and an epic with no dependencies belongs
next to its first dependent, not with the other roots (else the first band fills with unrelated
roots); everything drawn at once at 500+ nodes is a hairball, and bands that cross milestones lose
milestone identity; edges off by default, the selection lighting its upstream and downstream; the
question asked most was "what does this wait on, what waits on it, and why", the second "which
epics block the most work". It never had gates, conditions, progress state, review findings, a diff
between two graph revisions, or a one-milestone-plus-neighbours filter. So: tp computes the ranks,
the map stays an overview, and the chain view below is a primary view, not a panel.

### Views besides the map

Overview first, then zoom and filter, then details on demand: the map is not the first thing read.

- **Chain** (primary): the selected epic — by default the most waited-on — with its upstream and
  downstream as collapsible outlines, each edge with its why, a name re-centring the chain; beside
  it, the most waited-on epics.

- **KPI strip** (top): share done, ready count, critical-path length, blocked/held counts, operator
  queue (conditions, gates awaiting `--met`), open review loops, throughput sparkline, estimate vs
  actual.
- **"Now" lists**, before the map: ready in `tp epic ready`'s order; operator decisions
  (actionable conditions, `choices` of any-of groups); held and blocked epics with what each waits
  on. These answer "what moves next", the question a dependency map answers worst.
- **Milestone matrix:** milestone × milestone heat map of edge counts. Readability studies favour a
  matrix over node-link for most tasks past ~20 nodes and node-link only for following a path, so
  the matrix answers "which milestone loads which" and the map stays for paths.
- **Review loops:** one row per slice, as `tp epic review --status --all` prints it (state, round,
  open counts, grantable).

### Prototypes 2 and 3: the plan, and one spec at a time

The operator read prototype 1 and asked for the following, and read prototype 2 and narrowed the
work side to one spec. Prototype 3 has all of it, on synthetic data, every panel whose data no
command gathers yet tagged so:

- **A spec and its tasks beside the epics, one spec at a time.** Two tabs: *Epics* (the plan) and
  *Spec*. The Spec tab shows one spec, chosen from a picker (grouped by phase, filterable, escalations
  marked) or reached from an epic's bound specs or an operator row. Prototype 2 put every spec on one
  board and every task on one map; the operator found that unneeded: the tab is about the spec in hand.
- **Reading the spec is the tab's centre.** The spec's text, in its numbered sections, beside its
  task graph and task list. Every section and paragraph carries a stable anchor (`§3 ¶2`), because
  feedback on the spec comes next and attaches to one. A task names the sections it comes from
  (`source_sections`): hovering a task lights them, and a section's task count lights its tasks.
- **Every label names its unit** ("ready epics", "ready tasks"): "Ready now" read as either.
- **An ⓘ on every panel and figure:** what it lists, in a sentence or two, and where its data comes from.
- **Details in a drawer, never a scroll jump.** An epic opens its chain and bound specs; a task its
  source sections, acceptance, dependencies and command. A click moves the page only when that is
  what it asked for: "show on the map", or a jump to a section.
- **The map leaves the page's scroll alone.** A plain wheel scrolls the page and shows a hint; ⌘ or
  Ctrl + wheel, or a trackpad pinch, zooms; a full-screen button gives the map the window, where the
  wheel zooms and a two-finger scroll pans. Zoom eases toward its target rather than jumping.
- **Larger type**, a 15.5 px base.
- **The milestone matrix stays**, coloured by how far each pair's dependencies are done (green all,
  amber some, red none) and its diagonal by the milestone's own share done.
- **Review loops are tp's real loop.** The Spec tab shows the spec's own: the phase `tp resume`
  reports, the ground, review and audit rounds as dots, the next step and any escalation. The epic
  review slices stay on the Epics tab as "Plan review", and a spec's escalation joins its operator list.

### What the Spec tab needs from tp

Needs first, development last. The link needs nothing new: a task file binds to one epic through its
top-level `epic` (v1.5.0), and `epics[].files` already carries each bound file's phase, task counts
and `{round, state}` per loop. What is missing is one read of a single spec, and a list to pick from:

| Need | Held by tp today | Gap |
|---|---|---|
| Every task file, bound or not: path, title, phase, task counts, escalation | the binding scan over `*.tasks.json` (only bound files reach the export); the spec's first heading | unbound files (gap 4), the title |
| The spec's text in numbered sections, each part with a stable anchor | the spec file; sections for `source_sections` and `spec_excerpt`; `tp ground --units`, whose floor units carry an identity (`text_sha`, ordinal) that a later round matches | rendering it, and choosing the anchor feedback will use |
| Each task: id, title, status, `depends_on`, estimate, acceptance, `source_sections`, `started_at`, `closed_at`, commit | the task file | gap 1 |
| Each task's state: ready, waits on a task, waits on its epic | `tp ready`/`tp next`, the epic block | derived |
| Each loop's rounds: findings or FAIL count, clean, cap, clean rounds needed, streak | review/audit `state.json` entries (`findings`, `clean`, `recorded_at`), `--status` | gap 2, counts only |
| Next step, blockers and escalations | `tp resume` (`next_action`, `blockers`); `tp run` escalation records | none: the file's own `tp resume` |

Not held, so not shown: who holds a `wip` task (tp records no claimant).

So the Spec tab is one new read model, a task file with its spec text, tasks, loops and `tp resume`
view, plus the list to pick from, built in Go from code tp already has, with no change to the task
file or the graph schema. Whether it is an export key or a command of its own is the decision
note's question. **Feedback on the spec is later and its own decision** (who writes it, where it is
stored, how a round reads it); v1 only leaves the anchors for it.

### History: what tp already holds

The operator asked whether the dashboard can show how a spec and an epic got where they are, loop by
loop, and whether tp should keep that history. Prototype 9 shows it on synthetic data: a spec's
History view (its rounds on a time axis, and what changed in the spec between two review rounds), an
epic's history in its drawer, a lifecycle-over-time chart, and a "since you last looked" strip. What
each moment reads:

| Moment | Held by tp today | Gap |
|---|---|---|
| An epic planned | the git history of `.tp/epics.json` | none, where the graph is committed |
| Claimed, and by whom | `claimed: {at, by}` in the graph; binding leaves it, only `tp epic claim --release` removes it | none: a bound epic without the record was bound unclaimed, which binding allows (`engine.CheckEpicBinding` refuses only a condition and a done epic) |
| A spec bound to it | the task file's `created_at` when `tp init --epic` bound it | the time `tp set --epic` binds an existing file: `bound_at` |
| Each ground, review and audit round | `state.json` entries (`recorded_at`, `findings`, `clean`, `spec_hash`), the round files, and each round's spec snapshot (`snapshot-round-N.md`, `snapshot-<phase>-round-N.md`) | none: what changed between two review rounds is the diff of their snapshots, beside the earlier round's findings and resolutions |
| Tasks written | none | the time `tp import` wrote them: `imported_at` |
| Each task started and closed, on what evidence | `started_at`, `closed_at`, `closed_reason`, `commit_sha(s)` | none; a reopened task keeps only its current state |
| Released, and the epic done | an epic's `done_at`, from the audit round that brought its last bound file to release; `done: {at, evidence}` for an unbound epic | none |
| Derived figures over time (critical path, blocked count) | none | not tp's to hold |

The state directories are committed in tp's own repository; a project that does not commit them
keeps this history on one machine, so the history views read what is on disk and say nothing about
what is not.

**Proposed: no event log.** tp derives status rather than storing it (an epic's status is never
written), and a log beside the files would be a second copy of the truth that drifts the first time
someone edits a file by hand, and an append-only file that every branch and parallel agent writes.
Instead, three steps:

1. Show what is held. The History view, the epic's history and the claim (recorded or skipped) need
   no new data, only the read model reading it.
2. Add the two missing times where the fact lives, the way `claimed.at`, `done.at` and `met.at`
   already are: `bound_at` and `imported_at` on the task file. That is its own decision candidate,
   `lifecycle-timestamps.md`.
3. Leave trends to the dashboard. "Since you last looked" lives in the browser; a week-by-week chart
   of derived figures would come from snapshots the dashboard keeps for itself, never from tp, and
   losing them breaks nothing.

## 5. Scope

**v1:** `tp dashboard`, option B, two tabs. Epics: map + filters + chain + KPI strip + "Now" lists +
matrix + plan review, from today's export plus `revision`/`generated_at` (gap 5) and `layout`. Spec:
picker, loop, reader, task graph and task list, from the spec read model (§4).
**Later:** feedback on the spec, per-finding detail (severity, resolution), `tp run` units live (3),
a TUI summary, a `--md`/static snapshot for sharing, and the history views §4's "History" lists
beyond the operator's v1 choice (§6, question 4).

## Non-goals

No writes, buttons or claims from the page. No second derivation in JS (it renders the export's
fields; it computes nothing a tp command reports). No remote access, auth or multi-user. The page
reads no file: round files, snapshots, run logs and state directories reach it only through the read
model, which reads them in Go the way `tp review --status` does. No JS dependency or CDN.

## 6. Questions, answered

The operator decided 1, 6 and 12; the rest take the recommended answer unless the decision note's
review changes them.

1. Web page now, TUI later? **Web only in v1** (operator).
2. How does the page learn of change? **B: in-process re-export on a 2 s tick, SSE on hash change.**
3. Per-task detail in v1? **Yes, since the operator's review of prototype 1: the Spec tab shows each
   of the spec's tasks with its dependencies** (operator; the needs are §4's "What the Spec tab needs").
4. Loop history in v1? **Round counts and clean per round, yes (the loop dots); severity and
   resolution per finding stay later.** The operator then asked how a spec and an epic got where
   they are (§4, "History"). **Proposed: the Spec tab's History view in v1, built only from what tp
   holds today (rounds, their snapshots, task times, claim and done records); the lifecycle-over-time
   chart after `bound_at` lands; the operator confirms.**
5. `tp run` units live in v1? **Only `tp run --status` per bound file, read in-process; no new stream.**
6. What does a repo with no graph show (tp's own)? Was **exit 3 with a hint** (operator). Reopened by
   the Spec tab: such a repo has specs and tasks to show. **Proposed: open on the Spec tab, the
   Epics tab saying there is no graph; the operator confirms.**
7. Add `revision`/`generated_at` to the export? **Yes, additive within `version: 2`.**
8. Trim `milestones`' duplication for the page? **No; 0.8 MB on localhost is fine; don't fork the contract.**
9. Port: fixed default or random? **Fixed default (e.g. 7377) with `--port`; exit 1 naming it if taken.**
10. Edges always drawn or on demand? **On demand + critical path; a toggle for all.**
11. Under `TP_UNATTENDED`? **Refuse (exit 2): a long-running server is not a unit's job.**
12. Does the field project's own drawing retire at v1? **Yes: tp takes the map over entirely; the
    field artifact retires once `tp dashboard` ships, after its layout lessons are checked against
    §4** (operator).
13. Track: decision or full? **Decision track: build the page in a clone first, 2 roles, 2 rounds;
    a clickable prototype on synthetic data the field graph's size comes before the note.**
