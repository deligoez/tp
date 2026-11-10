# tp dashboard — one view of the plan, the specs and every loop

> Decision candidate, rewritten against the v1.6.0 export as shipped and updated for v1.7.0. The
> operator answered §6's questions 1, 6 and 12; §4 records a web research pass on graph rendering
> and dashboard design. v1.7.0 moved the export to `version: 2`, which this candidate reads: a
> `depends_on` entry may be `{any_of, why}`, and `epics[]` add `blocked_by_any_of`,
> `held_by_any_of`, `blocked`, `held` and `moot` (spec/1.7.0.md §7).

## Decision (proposed)

`tp dashboard [--port N]` serves one static page, embedded in the binary, on `127.0.0.1`. Its only
data is `tp epic export`'s JSON, produced in-process by the same code the command uses, re-derived
when tp's files change and pushed to the page. It draws the map (tracks, milestones, epics, edges,
gates, conditions, critical path) and a detail panel (epic → bound file → phase → loops → tasks),
and a KPI strip. It writes nothing and exposes no write endpoint. Stdlib only (`net/http`, `embed`,
Server-Sent Events); no JS framework or CDN.

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

## 5. Scope

**v1:** `tp dashboard`, option B, map + filters + detail panel down to file/loop level + KPI strip,
all from today's export; additive export keys only if a gap blocks the page (5 is cheap).
**Later:** per-task and per-round detail (gaps 1–2), `tp run` units live (3), unbound-spec
projects (4), a TUI summary, a `--md`/static snapshot for sharing.

## Non-goals

No writes, buttons or claims from the page. No second derivation in JS (it renders the export's
fields; it computes nothing a tp command reports). No remote access, auth or multi-user. No reading
of round files, run logs or state directories. No JS dependency or CDN.

## 6. Questions, answered

The operator decided 1, 6 and 12; the rest take the recommended answer unless the decision note's
review changes them.

1. Web page now, TUI later? **Web only in v1** (operator).
2. How does the page learn of change? **B: in-process re-export on a 2 s tick, SSE on hash change.**
3. Per-task detail in v1? **No in v1's spec, but add `--tasks` to the export first if the detail
   panel without tasks reads as empty to the field.**
4. Loop history in v1? **No; `{round, state}` suffices; history is gap 2, later.**
5. `tp run` units live in v1? **Only `tp run --status` per bound file, read in-process; no new stream.**
6. What does a repo with no graph show (tp's own)? **Exit 3 in v1 with a hint; project-wide export
   later** (operator).
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
