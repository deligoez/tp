# tp — Floor anchors need fixtures

Class: tool

## 1. Overview

`engine.FloorAnchorOf` bills six kinds of unit to an anchor no reader would predict, and every
grouping still totals the floor size, so no consumer of anchors can detect one of them by checking its
own arithmetic. Split out of `ground-command-friction.md` on 2026-09-08, where it was the section
*"`FloorAnchorOf` bills six kinds of unit to an anchor no reader would predict"*, with the non-goal
that fences it. It carries no test row from that file — the six fixtures in §2 are the acceptance —
and it is the one part of the split that reaches no field report and no grounding round of its own.
The measurements and the state each fixture was built at are in
`floor-anchors-need-fixtures-measurements.md`.

## 2. `FloorAnchorOf` bills six kinds of unit to an anchor no reader would predict

Found while a lint field grouped by anchor was being specified and the specification kept describing
behaviour the function does not have (measurements file, "`FloorAnchorOf`"). The field was dropped;
the six defects are the function's and belong here. Each is a fixture built and run against
`engine.FloorAnchorOf`.

| # | fixture | anchor produced | what a reader expects |
|---|---|---|---|
| 1 | a unit under `## 2. Second` with no blank line above the heading | `§1` | `§2` |
| 2 | a unit under a top-level `## Unnumbered parent` that follows a `### 1.1 Child` branch | `§1.1` | the preceding *top-level* numbered section |
| 3 | a document whose numbered headings are all `# 1.`-style H1s | every unit `§0` | `§1` and after |
| 4 | a `## 9.` heading inside a ` ``` ` block that is itself inside a `~~~` block | `§9` | no anchor — it is a code sample |
| 5 | a `## 9.` heading in an indented code block | `§9` | no anchor |
| 6 | `## 2026-09-07 release notes` and `## 0.19.0 — Agent Friction Reduction` | `§2026`, `§0.19.0` | no anchor — neither is a section number |

Two of these are one bug with two faces. `floorSectionHeadingRe` matches `#{2,6}`, which is why a
numbered H1 opens nothing (#3), and `floorFenceRe` toggles on ` ``` ` **or** `~~~` without recording
which delimiter opened the block, so a ` ``` ` line closes a `~~~` fence (#4). Both have a code
comment conceding the gap; neither has a test.

**What makes this worth a release rather than a note.** The sum is not affected — every uncut unit
gets exactly one anchor, so any grouping still totals the floor size, and that invariance is exactly
why the defect survived a grading round that asserted the sum. A consumer of anchors therefore
cannot detect any of the six by checking its own arithmetic; only a fixture whose expected anchor is
written down can. Any release that ships an anchor-keyed output must ship these fixtures with it.

## 3. Non-Goals

1. **This is not a request to redefine the family's notion of a code block.** Stated because it was
   the first proposal. `~~~` and indented blocks are out of scope for every rule in
   `internal/engine/vague.go` by decision. Defects 4 and 5 are in scope only because
   `floorAnchorsByLine` carries a comment claiming *"a heading inside a fence is code"* — the defect
   is the gap between the comment and the code, not the family's scope.
2. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code.

## 4. Tests

**This decision carries no test row from the original file**, and that is deliberate rather than an
omission: §2's six fixtures *are* the acceptance, each one an input, an anchor the function produces
today and the anchor a reader expects, and the implementing task owes a case per row. The mutant for
every one of them is the shipped function, which is why the six are written and watched red before
anything is changed.
