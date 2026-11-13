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
   and every edge is classed tree, cross or back. A node with no predecessor, or whose predecessors
   all sit in other tracks or on its own layer or below, has no primary parent: it is a root of the
   forest, and every edge into it is a cross or back edge. `/api/layout` gains `view=plan` (the whole
   graph) and serves columns and edge classes for every view.
2. **One graph card.** The Plan map and the Dependencies card become one card whose modes are Plan,
   Milestones, Gate closure and Neighbourhood, one renderer, one endpoint. The whole-graph mode
   reverses 1.13.0's "never all at once"; the forest with cross edges hidden is what makes it read.
   An edge on the marked critical path is always drawn, whatever its class, so a view with cross edges
   hidden never shows the path with holes.
3. **deck.gl draws it.** This reverses 1.8.0's "no library"; that rule's reason was a second layout
   derivation, and tp still derives every position. Conditions: vendored and pinned to one version
   with its licence and a checksum test, no network at runtime; loaded only when the card is shown; a
   character set that renders non-ASCII titles; a plain message without WebGL2, every list still
   working; the binary size measured and CLAUDE.md's stated bound corrected on purpose.
4. **Keyboard use moves to the lists, search and the text chain.** A canvas has no focusable nodes.
   Search selects and centres; Esc clears; the arrow keys pan; a link in a list or the drawer selects
   on the graph. The text views that outlive the old cards are the drawer, the lists above the card
   (ready, operator decisions, most waited-on, waiting epics) and the selected epic's chain as a
   collapsible text list under the card, as 1.13.0 kept it; together they carry keyboard use and
   give a keyboard user the dependency picture. This is a trade, recorded as one.

**Amended in 1.17.1.** The operator reviewed the shipped card and changed decision 2's default: cross
edges are drawn by default in every mode, light and under the tree edges and the critical path, and
K hides them; the address carries `cross=0` only when they are hidden. What keeps the plan readable
with them on is the drawing order (bands, cross edges, long tree edges, tree edges, the critical
path, nodes, labels) and fading the long edges, those spanning three layers or eight columns. A
selected epic's own chain draws its cross edges even with the toggle off
(`spec/1.17.1-measurements.md`).

## 3. Alternatives

- **A plain 2D canvas, the same layout, no library.** Rejected because the operator has a working,
  trusted page on this renderer, and porting it is cheaper and safer than writing a renderer. Not
  because the graph's size needs WebGL: a 2D canvas would draw it.
- **The 1.13.0 SVG map.** Rejected for its layout, every edge between milestone columns, not for its
  drawing.

## 4. Verification

The layout and the endpoint by Go tests; page functions by node tests; the drawing by the audit agent
with screenshots on a synthetic graph, since nothing in CI renders WebGL. The page records how many
nodes and edges it drew per mode on the card (`data-drawn-nodes`, `data-drawn-edges`); a node test
asserts the counts equal the layout's, and the screenshot audit reads them per mode, so a blank
canvas fails. The keyboard trade of decision 4 is checked there too: no canvas node takes focus, and
search, Esc, the arrows and the list links do what decision 4 says.

## 5. Non-goals

- Edge bundling.
- Writing from the page.
