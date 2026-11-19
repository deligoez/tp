# Dashboard simplify: answer three questions first, fold the rest

Decision track. Follows `dashboard-redesign.md`, which rebuilt the look on `DESIGN.md` and left the
structure as it was. This note decides the structure.

## Problem

The Epics tab opens on nine figure cards and nine panels, and reaching its bottom takes several
screen heights (counts in the table below, from `measure.js` run in the page on the synthetic
fixture at 1440×900). Every panel is drawn open and at the same weight, so the page fails
`DESIGN.md`'s squint test ("when blurred, the most important element and the purpose of the screen
should still be obvious") and its cognitive-load principle ("remove unnecessary information and
choices, group related things").

## Who asks what

Agents read tp through the CLI, never the page, so every panel's reader is the operator, either at a
glance (opening the page to see whether anything needs them) or digging (following up on one thing).

| Panel | Question it answers | Reader | How often |
|---|---|---|---|
| Share done, Ready, Blocked, Operator decisions | Is the plan moving, does anything need me | glance | every visit |
| Critical path, Held, Review loops, Lint, Epics per week | How healthy is the plan | digging, or an alarm | when non-zero |
| Ready epics | What can start now | glance | every visit |
| Operator decisions | What waits on me | glance | every visit |
| Most waited-on epics | What unblocks the most | glance | every visit |
| Graph | How the plan hangs together | digging | often |
| Waiting epics | Why each blocked epic waits | digging | sometimes |
| Lifecycle | How many epics sit in each stage, which milestones are open | digging | sometimes |
| Lifecycle over time | How the stages moved over weeks | digging | rarely |
| Milestone matrix | How done each milestone's dependencies are | digging | rarely |
| Plan review | Which slices were reviewed, how the rounds went | glance before work starts, digging after | phase-dependent |

## Decision

1. **The default screen answers three questions**: what moves now (Ready epics), what needs me
   (Operator decisions), and is anything stuck (Most waited-on epics, and the Blocked figure). The
   Now row follows the four headline figures, before everything else.
2. **Four headline figures, the rest one step away.** Share done, Ready, Blocked and Operator
   decisions stay in the strip. Critical path, Held, Review loops, Lint and Epics per week move into a
   "More figures" fold. The fold opens itself while Held, Lint errors or stale loops are non-zero, so
   an alarm is never folded away.
3. **Digging panels are folds** (`DESIGN.md`'s Accordion: "turns sections of content into a scannable
   list ... Don't hide content people need"). Waiting epics, Lifecycle, Lifecycle over time, Milestone
   matrix and Plan review keep their heading, their ⓘ and a one-line summary when folded; one press
   opens them. Each fold remembers its state per viewer in the browser, the way the theme and
   language already do; nothing about it reaches the server.
4. **Plan review keeps its §5 placement rule.** While no epic has a bound spec it is the planning
   stage's work and opens unfolded under the figures; once one does it closes the tab, folded.
5. **The Graph stays open.** It is the tab's map and already loads only when scrolled into view.

## Alternatives

- **A separate "Overview" tab** holding only the Now row. Rejected: a fourth tab adds a choice, and
  the Epics tab would still open on everything.
- **Drop the digging panels.** Rejected: each answers a question the operator does ask, and
  `DESIGN.md` says "never hide or remove information people need just to make a screen look cleaner".
  Removals are listed below for the operator; none is made here.
- **Hide zero figures outright.** Rejected: a zero is an answer ("nothing is held"); folding keeps it
  one press away instead of gone.

## Non-goals

No server, API or payload change; no panel removed; the Specs and Spec tabs keep their structure (one
panel each, already one screen at the fixture's size); EN and TR stay in step.

## Proposed removals (operator decision, not made)

- The Graph toolbar's own **Fit** button, which repeats the **Fit** in the canvas's zoom group.

## Counts

Measured by `measure.js` in the page at 1440×900, English, light, on the synthetic fixture. Panels
are top-level cards; controls are visible buttons, inputs, selects, links and summaries; words are the
page's visible text; screens are the page height over the viewport.

| Epics tab | Before | After |
|---|---|---|
| Figure cards | 9 | |
| Panels | 9 | |
| Controls | 72 | |
| Words | 550 | |
| Words on the first screen | 153 | |
| Screens to the bottom | 4.2 | |
