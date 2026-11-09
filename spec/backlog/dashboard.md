# tp dashboard — one view of the plan, the specs and every loop

> Decision candidate, rewritten against the v1.6.0 export as shipped. Nothing here is decided until
> the operator answers §6.

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

- **Swimlane per track**, milestones as boxes along it. x-position = opening depth (longest
  `opens_after` chain to the milestone), so cross-track `opens_after` reads left to right and
  `[]` siblings share a column. Closed milestones dimmed, with `waiting_on` on hover.
- **Epics as nodes** inside their milestone box, ordered by `downstream.longest_chain`; colour by
  status (planned, claimed, specced, in_progress, done); conditions a distinct shape, actionable
  ones outlined as "operator".
- **Edges** drawn on hover/selection by default (530×3 always-on edges is noise); the selected
  epic's upstream and downstream highlighted; `why` on edge hover.
- **Gate** as a bar at the right edge of each milestone box: green/red, member count, threshold.
- **Critical path** toggle: thick line through `critical_path.by_count`.
- **Filters:** open milestones only, one track, one tag, status, "ready now".
- **Detail panel:** epic (title, status, deps with whys, downstream, held_by) → bound files (path,
  phase, tasks bar) → loops (ground/review/audit round + state) → tasks (gap 1) → slices naming it.
- **KPI strip:** share done (graph and per milestone), throughput sparkline, estimate vs actual,
  critical-path length, ready count, operator queue (conditions, gates awaiting `--met`).

## 5. Scope

**v1:** `tp dashboard`, option B, map + filters + detail panel down to file/loop level + KPI strip,
all from today's export; additive export keys only if a gap blocks the page (5 is cheap).
**Later:** per-task and per-round detail (gaps 1–2), `tp run` units live (3), unbound-spec
projects (4), a TUI summary, a `--md`/static snapshot for sharing.

## Non-goals

No writes, buttons or claims from the page. No second derivation in JS (it renders the export's
fields; it computes nothing a tp command reports). No remote access, auth or multi-user. No reading
of round files, run logs or state directories. No JS dependency or CDN.

## 6. Open questions for the morning (recommended answer in bold)

1. Web page now, TUI later? **Web only in v1.**
2. How does the page learn of change? **B: in-process re-export on a 2 s tick, SSE on hash change.**
3. Per-task detail in v1? **No in v1's spec, but add `--tasks` to the export first if the detail
   panel without tasks reads as empty to the field.**
4. Loop history in v1? **No; `{round, state}` suffices; history is gap 2, later.**
5. `tp run` units live in v1? **Only `tp run --status` per bound file, read in-process; no new stream.**
6. What does a repo with no graph show (tp's own)? **Exit 3 in v1 with a hint; project-wide export later.**
7. Add `revision`/`generated_at` to the export? **Yes, additive, `version` stays 1.**
8. Trim `milestones`' duplication for the page? **No; 0.8 MB on localhost is fine; don't fork the contract.**
9. Port: fixed default or random? **Fixed default (e.g. 7377) with `--port`; exit 1 naming it if taken.**
10. Edges always drawn or on demand? **On demand + critical path; a toggle for all.**
11. Under `TP_UNATTENDED`? **Refuse (exit 2): a long-running server is not a unit's job.**
12. Does the field project's own drawing retire at v1? **Yes, once its layout lessons are checked
    against §4 before the spec is written.**
13. Track: decision or full? **Decision track: build the page in a clone first, 2 roles, 2 rounds.**
