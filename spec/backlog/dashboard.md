# tp dashboard — one view of the plan, the specs and every loop

> Decision candidate. Comes after `spec/1.5.0.md`, whose export is its data contract.

## Decision

The operator wants one view of everything tp holds: the epic graph with its dependency trees and
details, the plan's progress as KPIs (counts and share done per milestone, gates, critical path,
throughput, estimate versus actual), every spec and its phase, every loop (ground, review, audit, epic
review) round by round, the implementations (tasks, closures, commits), and the runs in flight. It
updates while tp works.

1. **It is a tp subcommand, `tp dashboard`.** Its page ships inside the tp binary and the same
   process serves it on localhost; there is no separate backend and nothing to install beside tp.
   This supersedes non-goal 1 of `spec/backlog/epics.md` ("No rendering"). The reason is the live
   view: a separate tool could only watch files after tp wrote them and re-derive tp's logic from
   outside, while tp itself derives every status with the code that writes it and can see work in
   flight, such as a `tp run` unit, before it reaches a file.
2. **It reads only tp's exports, never tp's state files.** Round files, state directories and run
   logs are tp's internal layout and change between releases; a view that read them would break on
   upgrade and, worse, compute phases and statuses a second time. Every figure the dashboard shows is
   one tp already derives, read through the same code its JSON exports use. v1.5.0's
   `tp epic export --json` is versioned for this and carries each epic's bound files with their phase,
   task counts and loop states.
3. **It draws the map.** The dependency graph is drawn by tp: milestones and tracks as lanes,
   epics and their edges with their whys, gates and conditions, each epic coloured by its derived
   status. It replaces the renderers projects draw for themselves; the requesting project's own map
   is the first one it retires, and the layout lessons its request recorded become this spec's input.
4. **It is live.** The process watches the files tp writes and pushes a fresh export to the page when
   they change, so a review round or a closed task shows without a reload.
5. **It writes nothing.** Every decision stays with the commands and their fences.

## Open questions for its spec

- Web first, a TUI later, or both: a dependency graph of hundreds of epics needs zoom and pan,
  which argues for the web page first.
- The map's layout: the requesting project's lessons from drawing its own (a layout defect read as a
  graph defect, among them), gathered from it before the spec is written.
- A project-wide export for specs bound to no epic, such as this repository's own, and its shape.
- Whether runs in flight need an event stream from `tp run`, or the files it already writes suffice.
- Binding to localhost only, and what the page shows when an export fails.
