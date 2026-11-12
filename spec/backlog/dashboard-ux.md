# tp — The dashboard at the size of a real plan

A backlog decision for 1.13.0. The operator asked for it and delegated the design decisions for the
night to the agent and the tp-smart session; the cycle records which calls were made that way.

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
the same reason, and the layers already exist. tp computes a layered layout in Go — layers, then a
few barycenter sweeps to order each layer, with dummy nodes for edges that skip layers — deterministic
and tested; the API stays read-only and returns positions; the page draws SVG and handles pan, zoom
and full screen.

**The graph is shown in scoped views, never all at once.**

1. **Milestones**: milestones as nodes, their opens-after edges between them, each with its epic
   count, its done count and the count of edges into it from other milestones. This is the overview.
2. **A gate's closure**: pick a milestone; what its gate pulls in, columns by milestone, the critical
   path marked, any-of alternatives drawn as one dashed fork. It draws `tp epic scope`'s own data.
3. **An epic's neighbourhood**: what it depends on to the left and what depends on it to the right, to
   a chosen depth (default 2). Clicking a node re-centres on it. This replaces the chain list, which
   stays as a collapsible text view.

**The map becomes usable.** A full-screen control (the Fullscreen API, with the plain wheel zooming
while in full screen and Esc leaving it); pan and pinch on touch; search and any list selection
centre and select the epic; the drawer narrows the map instead of covering it; nodes take focus with
a roving tabindex and Enter selects; the drawer takes focus when it opens and returns it on close;
the first view fits the width at a readable scale.

**Waiting epics answers "what is blocked, and by what".** It groups the blocked epics of open
milestones by what they wait on, each group headed by the blocker with its state (startable, claimed,
waiting) and the number of epics it would unblock, ordered by that number; a few groups show, the
rest behind "show all"; blocked epics of milestones not yet open collapse into one counted row. Ids
in it link to the drawer, and its total agrees with the KPI or the difference is named. The section
moves below the Plan map.

**Smaller fixes in the same release.** The "open review loops" KPI splits slices that need a round
from slices only stale after a graph edit; list titles wrap instead of cutting to a few letters; a
gate's threshold and purpose are readable in full; a mark on epics with a description and a
per-milestone count of described epics; named set slices appear with milestone slices in Plan
review; Plan review stacks into cards on a phone; the header's update time reads relative; a
favicon.

## 3. Alternatives

- **Vendor dagre or elk.** Rejected: hundreds of kilobytes to a megabyte for a layering the graph
  already has, a second layout derivation beside tp's, and the first dependency of the page.
- **Draw the whole graph as a layered DAG.** Rejected as the default: at plan size it is many layers
  wide and tall and no ordering makes it readable; the scoped views are the readable sizes.
- **Render descriptions as markdown.** Deferred: 1.11.0 chose escaped preformatted text, and safe
  rendering needs a sanitiser the page does not have.

## 4. Non-goals

- Any write from the dashboard.
- Smaller SSE payloads (deltas, compression).
- A theme switch.
