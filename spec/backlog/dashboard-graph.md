# tp — One graph card, laid out as a forest and drawn by a renderer

A backlog decision for 1.17.0. The operator found the 1.13.0 Plan map still unusable and pointed at a
viewer of theirs that reads well at thousands of nodes; they approved a prototype built from it. The
design was settled with a second opinion from the tp-smart session.

Class: **decision**.

## 1. Context

What makes the reference viewer readable is its layout, not its renderer: longest-path layers as
rows, a primary-parent forest packed into columns, tree edges drawn and cross edges hidden by
default, back edges marked. The 1.13.0 map draws every edge between milestone columns and reads as
noise; the scoped views of 1.13.0 sit in a second card.

## 2. Decision

1. **tp lays out a forest.** In Go, deterministic and golden-tested: milestones stay bands of
   consecutive layers (cyclic bands merged), tracks share rows, each node gets one primary parent (the
   deepest earlier predecessor in its track, ties by file order), the forest is packed into columns,
   and every edge is classed tree, cross or back. `/api/layout` gains `view=plan` (the whole graph)
   and serves columns and edge classes for every view.
2. **One graph card.** The Plan map and the Dependencies card become one card whose modes are Plan,
   Milestones, Gate closure and Neighbourhood, one renderer, one endpoint. The whole-graph mode
   reverses 1.13.0's "never all at once"; the forest with cross edges hidden is what makes it read.
3. **deck.gl draws it.** This reverses 1.8.0's "no library"; that rule's reason was a second layout
   derivation, and tp still derives every position. Conditions: vendored and pinned to one version
   with its licence and a checksum test, no network at runtime; loaded only when the card is shown; a
   character set that renders non-ASCII titles; a plain message without WebGL2, every list still
   working; the binary size measured and CLAUDE.md's stated bound corrected on purpose.
4. **Keyboard use moves to the lists and search.** A canvas has no focusable nodes; search selects
   and centres, and the drawer and lists carry keyboard use. This is a trade, recorded as one.

## 3. Verification

The layout and the endpoint by Go tests; page functions by node tests; the drawing by the audit agent
with screenshots on a synthetic graph, since nothing in CI renders WebGL.

## 4. Non-goals

- Edge bundling.
- Writing from the page.
