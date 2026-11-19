# Dashboard redesign on DESIGN.md (Decision track)

`tp dashboard` gets a full redesign against `DESIGN.md` (Practical UI; `CLAUDE.md` rule 14). The
operator asked on 2026-11-19 for the whole dashboard to be redesigned, from scratch if need be.

## Operator decisions

1. The brand colour is DESIGN.md's own blue, taken verbatim: `text-brand`, `fill-brand-strong` and
   `stroke-focus` with their light and dark values.
2. The font stack is `Inter, system-ui, sans-serif`. No font is embedded, so the binary does not grow
   and the look follows what the machine has installed.
3. `DESIGN.md` is a verbatim copy at the repository root.

## Decision

Rebuild the page's markup, styles and view code from DESIGN.md's tokens and components. Keep the
server, its payloads and `pure.js`'s logic. Today's page is three views, a drawer and a deck.gl graph,
built from 81 custom properties and 72 hex literals of its own; every value moves onto DESIGN.md's
semantic tokens, type scale, spacing, radii and shadows. Each view is rebuilt as one unit:

1. tokens and the shell;
2. Epics;
3. Specs;
4. Spec and the drawer;
5. the graph's colours.

Every unit runs the gate. The tests that pin the old markup are rewritten against the new markup, and
each is seen red first. The behaviour tests (node runs of `pure.js`, contrast, i18n parity, the
server) carry over; the contrast tests are repointed to the new token names.

## Collisions with DESIGN.md, and what this note recommends

Each recommendation that departs from DESIGN.md is written beside the code that needs it, with its
reason, as rule 14 asks. Line numbers refer to `DESIGN.md`.

1. **Eight task-state colours against four status tones** (L284: the 4 status colours are for status
   only; L274: the brand blue marks what is interactive).
   - **Today:** `--ready` is blue, which now reads as "clickable", and `--crit` is a fifth orange.
   - **Recommendation:** map each state onto a status tone or a neutral.

     | state | colour |
     |---|---|
     | done | success |
     | blocked | error |
     | held, the operator's call | warning |
     | active and in progress | information |
     | ready | neutral strong, a solid ink mark |
     | specced, claimed and waiting | neutral weak |
     | moot | the disabled tint |

     The critical path becomes a heavier `stroke-strong` line instead of an orange hue.
   - **Why:** it keeps the four tones for status, keeps the brand for interaction, and the page
     already gives every state its own mark shape, so colour is never the only carrier (L283).

2. **Eight categorical owner colours against one brand colour** (L274, L276: design in black and
   white first).
   - **Recommendation:** outside the graph canvas, owners appear as neutral badges carrying their
     handle (L504). The "Colour by owner" switch keeps a categorical palette, but only on the canvas
     and its legend, as a recorded data-display deviation. That palette leaves out the hues of the
     four status tones and the brand blue, so an owner never reads as an error or a link.
   - **Why:** colour by owner is a shipped feature (`spec/backlog/ARCHIVE.md`, epic-owner), and
     DESIGN.md covers interface chrome, not chart series.

3. **Density: today's 13px text against the 14px floor and the 48/24px targets** (L313: `tiny`
   14px for metadata; L311–312: `small` 16px for most interface text; L415–417).
   - **Recommendation:**
     - `small` (16px) for the tables, lists and drawer, and `tiny` (14px) only for metadata.
     - Toolbar controls use the small 32px control height, and chips, tags and segmented controls
       keep at least 24px (L416). Primary page controls take 48px.
     - Labels inside the canvas stay at least 14px.
   - **Why:** the floor is an accessibility rule, not a style choice. The density goes back by
     spacing, not by shrinking the text.

4. **IBM Plex Mono for ids, paths and commands against a single typeface** (L305).
   - **Recommendation:** a system monospace stack (`ui-monospace, SFMono-Regular, Menlo, monospace`)
     only for code: `<code>`, `<kbd>`, epic and task ids, paths and commands. This is recorded as a
     deviation.
   - **Why:** an id is code that people copy and compare character by character. A monospace face
     for code is a role, not a second interface face. IBM Plex goes, because the operator chose
     Inter.

5. **The graph's colours have to follow the tokens, through a live theme switch.**
   - **Recommendation:** keep today's path. `graph.js` reads the tokens through `getComputedStyle`
     and `pure.graphPalette` builds the colours; `tp-theme` triggers a re-read and a redraw. Repoint
     `GRAPH_TOKENS` at the semantic names.
   - Then fix the one trap: `parseColour` drops alpha, and DESIGN.md's greys are semi-transparent
     (L259). Composite each rgba token over `background-base` before it is used, and cover that in a
     node test, written first and seen red.

6. **Icons** (L432: Feather, with Lucide where Feather has none; never another set; L430: every
   icon-only button has an accessible name).
   - **Recommendation:**
     - Inline SVG paths from Feather (MIT) and Lucide (ISC), copied into the page, with their
       licences recorded beside deck.gl's in `vendor/PROVENANCE.md`.
     - They replace the "i", "×", "+" and "−" glyph buttons.
     - Nothing is fetched, so the page still loads nothing remote.

7. **One primary button in view** (L400).
   - The dashboard is read-only, so it has no primary action. It uses no primary button (L400:
     "If no action is most important, use no primary button").
   - Graph modes, language and theme use segmented controls (L492), and the views use tabs.

## Alternatives

- **Reskin only:** swap the token values and keep the markup. This is the cheapest option, and every
  pin survives. Rejected: today's markup groups, controls and density break the spacing, target and
  control rules no matter which colours fill them, and the operator asked for a redesign.
- **A framework or a build step** (React, Tailwind CLI). Rejected: the page is embedded, served by the
  Go standard library alone, loads nothing remote and has no build step. The tests read its plain
  sources.

## Non-goals

- No change to the server, the endpoints, the SSE events or the payloads.
- No new features. A feature is not removed unless this note lists it; it lists none.
- No embedded font. deck.gl stays at its vendored version.
- EN/TR parity is kept: keys may be renamed, but both dictionaries move together.
