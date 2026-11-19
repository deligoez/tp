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

## Pass 2 (operator-approved)

The operator approved the proposed removals and next steps of pass 1, and added one rule. All of it
is built:

1. **The Graph toolbar's duplicate Fit and its ? shortcuts button are gone.** The canvas zoom group's
   Fit and the F key stay; the open legend lists the keys.
2. **One View menu** (`DESIGN.md`'s Dropdown menu) holds the owner selector, Colour by owner and
   Cross edges. It is a `popover="auto"` in the browser's top layer, so the full-screen pill's
   scrolling cannot clip it, and the browser gives it Esc and light dismiss; the button carries
   `aria-haspopup`, `aria-controls` and `aria-expanded`. A neutral badge counts the settings away
   from their default (`tpPure.viewChanged`), with its plural spelled out for screen readers. K still
   toggles cross edges.
3. **A Specs phase chip that counts no spec hides unless it is the one chosen**
   (`tpPure.phaseChipShown`); All always shows.
4. **At phone width the Spec tab shows the open spec's file name** (`tpPure.pathTail`), ellipsised
   inside its third of the row, the full path in its tooltip. Before, the path ran past the row.
5. **No tp command in plain view.** Agents run tp, so the page says what happens in plain words; a
   command still useful to a person is one click away, behind a **Copy command** button (the command
   in its tooltip and clipboard) or in an ⓘ note. Every place changed:

   | Place | Before | Now |
   |---|---|---|
   | Spec view, next step | "Next, as `tp resume` says" + the command in a code span | "Next" + the plain summary; Copy command |
   | Task detail, ready or in progress | `tp claim <id>` / `tp done <id> "<evidence>"` in a code span | "Next: claim this task" / "Next: close this task with its evidence"; Copy command |
   | Operator decisions (conditions, any-of groups, gates, threshold rewrites, grantable slices) | each command in a code span beside Copy | Copy command only |
   | Epic drawer, a condition | "... records it with `tp epic done {id} --evidence`" | "... marks it done with evidence"; Copy command |
   | Epics tab, no graph | `tp epic`, `tp epic set milestones=...`, `tp epic add`, `.tp/epics.json` | "An agent plans the epics ..."; the commands in a new ⓘ note (`info.nograph`) |
   | Plan review, nothing reviewed | "`tp epic review --milestone <m>` starts one" | "No slice of the plan has been reviewed yet."; the command moved into its ⓘ note (`info.slices`) |
   | Task view, epic waits | "so `tp next` starts none of this file's tasks yet" | "so none of this file's tasks can start yet" |
   | Specs table, next step column | a code span | plain text |
   | Source labels (status line) | "The epic graph (tp epic export)", "The task file list (tp export --list)" | "The epic graph", "The task file list" |
   | Spec reading and its error | "Reading {path} with tp export.", "tp export could not read {path}" | "Reading {path}.", "Could not read {path}" |
   | Fetch errors (slice history, epic history, a spec) | the tp command as the subject | "The slice's history", "The epic's history", "The spec {path}" |
   | History, slice without a key | "its next tp epic review emission records it" | "its next review round records it" |

   The ⓘ notes keep their `tp ...` source references: they are the one place a reader asks where a
   figure comes from. `TestPage_NoTpCommandShowsInPlainView` pins the visible keys in both languages;
   `TestPage_TheCommandsMovedOutOfViewAreOneClickAway` pins where each moved one went.

### Before you finish (`DESIGN.md`), pass 2

- One primary button: none added; View, Copy command and the chips are secondary.
- Brand colour only on interactive elements: the View badge is neutral (`--fill-strong`), not brand.
- Contrast: the badge uses `--text-inverse-strong` on `--fill-strong`; the contrast tests pass for
  every theme.
- Not colour alone: the badge carries a number and a spoken plural.
- Tokens only: every new rule uses the spacing, radius, shadow and type tokens.
- Grouping: the View menu's controls sit closer to each other than the menu sits to the bar.
- Text: sentence case; the plain next steps read as one short line.
- Targets: View and Copy command are `--ctl-md` keys; menu items keep the chip size.
- States: the View button and Copy command reuse `.key`'s hover, press and focus-visible.
- Buttons say what they do: "Copy" became "Copy command".
- Small screen and long content: shot at 390px in both languages and themes; the tab row fits (358 of 358px) and the Spec
  tab ellipsises a long file name.

## Counts

Measured by `measure.js` in the page at 1440×900, English, light, on the synthetic fixture. Panels
are top-level cards; folded counts those drawn folded; controls are visible buttons, inputs, selects,
links and summaries; words are the page's visible text; screens are the page height over the
viewport. More figures is counted as a panel after, the folded row it is.

| Epics tab | Before | Pass 1 | Pass 2 |
|---|---|---|---|
| Figure cards | 9 | 4 | 4 |
| Panels (folded) | 9 (0) | 10 (5) | 10 (5) |
| Controls | 72 | 60 | 56 |
| Words | 550 | 298 | 274 |
| Words on the first screen | 153 | 156 | 150 |
| Screens to the bottom | 4.2 | 2.8 | 2.8 |

| Other views, pass 1 → pass 2 | Controls | Words |
|---|---|---|
| Specs | 20 → 16 | 108 → 99 |
| Spec, read | 23 → 23 | 188 → 183 |
| Spec, tasks | 23 → 23 | 218 → 213 |
| Spec, history | 15 → 15 | 116 → 111 |

The View menu's three controls are not counted while the menu is closed. The stripped binary is
14,062,994 bytes after pass 2, the same as before it: the page grew by less than the segment padding.

## Review round 1

Two reviewers read pass 2: design conformance against `DESIGN.md`, and behaviour regression. Every
finding was fixed; none was declined. Each fix has its own commit and a test seen red first, except
the pure CSS ones, whose test pins the new rule.

| Finding | Fix | Test |
|---|---|---|
| High: the drawer's Copy command did nothing | One document-level `[data-copy]` delegate copies every Copy command button; the task list ignores a press on one | `TestPage_EveryCopyCommandButtonIsHandled` |
| Esc on the open View menu also left full screen or closed the drawer | `viewMenuOpen()` in `app.js`, exported on `tpApp`; all three Esc handlers ask it first | `TestPage_EscOnAnOpenViewMenuClosesOnlyTheMenu` |
| Choosing an owner left the View badge stale | The owner picker redraws the toolbar, as Colour by owner and Cross edges already did | `TestPage_EveryViewControlRefreshesTheBadge` |
| A fold closed before an alarm arrived stayed closed | A press stores the alarm it saw; an alarm that arrives later reopens the fold (`tpPure.foldOpen`) | `TestPure_AnAlarmThatArrivesAfterAPressReopensTheFold` |
| M1: slice labels showed tp flags in plain view | `tpPure.sliceLabel` names a slice in words (EN and TR); `tpPure.sliceArgs` keeps the flag form for the grant-round Copy command only | `TestPure_ASliceLabelIsPlainWordsAndOnlyItsCommandCarriesFlags` |
| M2: reading prose at 18/30 | `.blk` is `--type-body`; a code block 14/20 | `TestPage_ReadingTextAndTheGraphTooltipUseTheTypeScale` |
| M3: the graph tooltip used its own type, radius and padding | `--type-tiny`, `--r-2`, `--sp-2 --sp-3`; the id line in mono at `--text-inverse-weak` | same |
| M4: controls without hover or press | Hover and press on the chain and More summaries, figure cells (hover moved off press), wait groups, legend buttons, the legend toggle, the sub-tabs and the section Tasks button | `TestPage_EveryControlNamedInReviewHasHoverAndPress` |
| L1: mono beyond ids, paths and commands | Tab counts and chart axes use the body face. The "tp" wordmark stays mono, recorded as a deviation next to its rule | `TestPage_MonoIsForIdsAndEveryMeasureIsAToken` |
| L2: measures off the tokens | Segmented control `--sp-1` padding and gap, `--r-1` buttons at `--ctl-sm`; banner `--sp-12` inset; badge `--r-pill`; graph count `--sp-10`; matrix `--sp-1` spacing and `--ctl-sm` cells | same |
| L3: a list row had a hover but is not a control | `.row:hover` removed | same |
| L4: the tab count ellipsised at 390px | Below 600px a count is its number alone (the `.cl`/`.cs` pair the Spec tab already used) | `TestPage_ATabCountIsANumberOnAPhone` |
| L5: stale data faded and greyed | `.stale` keeps its colours; the banner alone says the data is stale | `TestPage_MonoIsForIdsAndEveryMeasureIsAToken` |
| The task panel closed with a typed "×" | It is a `.popx` button and draws the `--ic-x` mask | `TestPage_TheTaskPanelClosesWithTheXIcon` |

Reshot at 1440 and 390px in both languages and themes after the fixes. The segmented controls
grew 8px with the token padding and the 32px button.
