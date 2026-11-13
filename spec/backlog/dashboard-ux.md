# tp — The dashboard at the size of a real plan

A backlog decision for 1.13.0. The operator asked for it and delegated the night's design decisions
to the agent, which took a second opinion from the tp-smart session; the cycle records which calls
were made that way.

Class: **decision**.

## 1. Context

`tp dashboard` was built against small graphs. At a plan's real size it fails in a few measurable
ways (the audit and the field report are in `spec/1.13.0-measurements.md`, as conclusions):

- **Waiting epics** lists every blocked epic flat, one row each. In a planning phase almost every
  epic is blocked, so the list is most of the page and pushes the Plan map, the chain, the matrix and
  Plan review out of reach.
- **The Plan map** stacks each milestone's epics in one column and fits the whole graph to the strip,
  so labels shrink below reading size and most of the canvas stays empty. Search finds an epic but
  does not take the view to it, selecting from a list does not move the map, it cannot be panned on a
  touch screen, its nodes take no keyboard focus, and the drawer covers it.
- **Full screen** was asked for in the prototype round and never reached the 1.8.0 spec; nothing
  removed it.
- **No picture of dependencies is readable**: the chain is a flat list that runs to thousands of
  pixels, and no layout makes hundreds of epics legible at once.

## 2. Decision

**Layout belongs to tp, drawing to the page.** No JavaScript library is added: 1.8.0 rejected one for
the same reason. tp computes a layered layout in Go. A node's layer is its dependency depth;
milestones are bands of consecutive layers, so a milestone spans as many layers as its own chains
need — one column per milestone would stack its epics and run edges inside the column, the current
map's failure. Within a band, layers come from the longest path among that band's epics; each layer
is ordered by a few barycenter sweeps, ties broken by file order so the output is stable.

**One read-only dashboard endpoint, not `tp epic export`.** Positions are presentation and change
with the drawing; the export is a versioned contract. The endpoint takes a closed `view` and the
milestone or epic it is about, and returns nodes with **layer and order as small integers**, not
pixels, and edges listed explicitly with their marks (on the critical path, an any-of group), so the
page computes nothing about the graph. Every parameter is matched against ids the graph holds by
lookup and never builds a path; `view` is a closed set; `depth` is clamped; a milestone or epic id
the graph does not hold is a 404, and anything malformed is a 400.
Those refusals are tested first, and a golden test on a small graph pins the layout.

**The graph is shown in scoped views, never all at once.**

1. **Milestones**: milestones as nodes, their opens-after edges between them, each with its epic
   count, its done count and the count of edges into it from other milestones. The overview.
2. **A gate's closure**: pick a milestone; what its gate pulls in, banded by milestone, the critical
   path marked, any-of alternatives drawn as one dashed fork. It draws `tp epic scope`'s own data.
3. **An epic's neighbourhood**: what it depends on to the left and what depends on it to the right, to
   a chosen depth (default 2). Clicking a node re-centres on it. This replaces the chain list, which
   stays as a collapsible text view.

Edges that skip layers are drawn as plain curves first, and selecting a node highlights its edges and
dims the rest. Routing them through dummy nodes is added only if a screenshot of a closure near the
measured 90th-percentile size does not read without them; the measurements record which.

**A number shown beside a drawing either counts what is drawn or says what else it counts.** Added
in 1.14.1, after the Milestones view printed "edges in" under each milestone while the lines drawn
were its opens-after edges and the count was of dependency edges between epics: the count now reads
"epic edges in" and the view's help says those edges are counted, not drawn. The same holds for a
total beside a total of something else (the Epics tab counts every epic, the share and the
Lifecycle count work epics; one sentence names the condition epics between them).

**The map becomes usable.** A full-screen control (the Fullscreen API, the plain wheel zooming while
in full screen, Esc leaving it); pan and pinch on touch; search and any list selection centre and
select the epic; the drawer narrows the map instead of covering it (changed by the operator in
1.17.1: the drawer is an overlay over a dimmed page, which keeps its width; a click on the backdrop
or Esc closes it, and a phone keeps its bottom sheet); nodes take focus with a roving
tabindex and Enter selects; the drawer takes focus when it opens and returns it on close; the first
view fits the width at a readable scale.

**Waiting epics answers "what is blocked, and by what".** It groups the blocked epics of open
milestones by what they wait on directly; an epic waiting on three blockers appears in three groups.
Each group is headed by its blocker, the blocker's state (startable, claimed, waiting) and the number
of epics waiting on it directly, and groups are ordered by that number. A few groups show, the rest
behind "show all"; blocked epics of milestones not yet open collapse into one counted row. The
section total counts distinct epics, so group sizes do not sum to it; the total agrees with the KPI or
the difference is named. Ids in it link to the drawer. The section moves below the Plan map.

**Smaller fixes, as their own commit series, dropped if review is busy.** The "open review loops" KPI
splits slices that need a round from slices only stale after a graph edit; list titles wrap instead
of cutting to a few letters; a gate's threshold and purpose are readable in full; a mark on epics with
a description and a per-milestone count of described epics; named set slices appear in Plan review;
Plan review stacks into cards on a phone; the header's update time reads relative; a favicon.

**Build order** follows the complaints: full screen and map behaviour, then Waiting epics, then the
endpoint and the three views, then the smaller fixes.

**Verification.** The layout and the endpoint by Go tests; pure page functions by the existing page
tests; behaviour by re-running the audit on the same synthetic graph against a checklist with one line
per complaint, with before-and-after screenshots in the measurements.

## 3. Alternatives

- **Vendor dagre or elk.** Rejected: hundreds of kilobytes to a megabyte, a second layout derivation
  beside tp's, and the first dependency of the page.
- **Put positions in `tp epic export`.** Rejected: presentation in a versioned contract.
- **Draw the whole graph as a layered DAG.** Rejected as the default: at plan size no ordering makes
  it readable; the scoped views are the readable sizes.
- **Render descriptions as markdown.** Deferred at first (1.11.0 chose escaped preformatted text);
  shipped in 1.17.1 through the page's own small renderer, which escapes before it draws
  (`spec/1.17.1-measurements.md`, After the hand test).

## 4. Non-goals

- Any write from the dashboard.
- Smaller SSE payloads (deltas, compression).
- A theme switch.
