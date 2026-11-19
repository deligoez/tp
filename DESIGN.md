---
name: Practical UI
version: 1.0
description: >
  Logic-driven UI design guidelines from the Practical UI book by Adham Dannaway, paired with the tokens of the Practical UI design system. Use it to design and build interfaces that are intuitive, accessible and beautiful.
theme:
  default: light
  modes: [light, dark]
fonts:
  heading: "Inter, system-ui, sans-serif"
  body: "Inter, system-ui, sans-serif"
colors:
  # In the Figma library, light and dark mode are applied with the Colours semantic collection's Light and Dark modes.
  light:
    text-strong: "rgba(0, 6, 38, 0.9)"        # headings, body copy, labels
    text-weak: "rgba(0, 9, 51, 0.65)"         # secondary text
    text-brand: "#4C64D9"                     # links, tertiary buttons
    text-disabled: "rgba(0, 17, 102, 0.1)"    # disabled only, never readable content
    text-error: "#C73A3A"
    text-warning: "#8F6C1A"
    text-success: "#067A57"
    text-information: "#1A74A8"
    text-inverse-strong: "#FFFFFF"            # on brand, strong and inverse fills
    text-inverse-weak: "rgba(255, 255, 255, 0.78)"
    text-inverse-disabled: "rgba(255, 255, 255, 0.12)"
    stroke-strong: "rgba(0, 13, 77, 0.45)"    # borders of interactive controls (3:1)
    stroke-weak: "rgba(0, 17, 102, 0.1)"      # dividers, decorative borders
    stroke-focus: "#4C64D9"                   # keyboard focus ring
    stroke-selected: "#4C64D9"
    stroke-brand-strong: "rgba(76, 100, 217, 0.8)"
    stroke-brand-weak: "rgba(76, 100, 217, 0.2)"
    stroke-error-strong: "rgba(199, 58, 58, 0.8)"
    stroke-error-weak: "rgba(199, 58, 58, 0.14)"
    stroke-warning-weak: "rgba(143, 108, 26, 0.2)"
    stroke-warning-strong: "rgba(143, 108, 26, 0.8)"
    stroke-success-weak: "rgba(6, 122, 87, 0.2)"
    stroke-success-strong: "rgba(6, 122, 87, 0.8)"
    stroke-information-weak: "rgba(26, 116, 168, 0.2)"
    stroke-information-strong: "rgba(26, 116, 168, 0.8)"
    stroke-disabled: "rgba(0, 17, 102, 0.1)"
    stroke-inverse-strong: "rgba(255, 255, 255, 0.6)"  # control borders on background-inverse
    stroke-inverse-weak: "rgba(255, 255, 255, 0.12)"
    stroke-inverse-disabled: "rgba(255, 255, 255, 0.12)"
    icon-neutral: "rgba(0, 13, 77, 0.45)"
    icon-brand: "rgba(76, 100, 217, 0.8)"
    icon-error: "rgba(199, 58, 58, 0.8)"
    icon-warning: "rgba(143, 108, 26, 0.8)"
    icon-success: "rgba(6, 122, 87, 0.8)"
    icon-information: "rgba(26, 116, 168, 0.8)"
    icon-disabled: "rgba(0, 17, 102, 0.1)"
    icon-inverse: "rgba(255, 255, 255, 0.6)"
    icon-inverse-strong: "#FFFFFF"
    icon-inverse-disabled: "rgba(255, 255, 255, 0.12)"
    fill-brand-strong: "#4C64D9"              # primary buttons
    fill-brand-weak: "rgba(89, 117, 255, 0.05)"
    fill-strong: "rgba(0, 6, 38, 0.9)"
    fill-weak: "rgba(0, 21, 128, 0.04)"       # tags, subtle containers
    fill-weaker: "rgba(0, 21, 128, 0.02)"
    fill-hover: "rgba(0, 21, 128, 0.04)"      # hover overlay
    fill-press: "rgba(0, 17, 102, 0.1)"       # press overlay
    fill-selected: "#4C64D9"
    fill-disabled: "rgba(0, 17, 102, 0.1)"
    fill-overlay: "rgba(0, 13, 77, 0.45)"     # dims the page behind modals
    fill-error-strong: "#C73A3A"
    fill-error-weak: "rgba(255, 74, 74, 0.05)"
    fill-warning-weak: "rgba(255, 192, 46, 0.05)"
    fill-warning-strong: "#8F6C1A"
    fill-success-weak: "rgba(10, 204, 146, 0.05)"
    fill-success-strong: "#067A57"
    fill-information-weak: "rgba(38, 176, 255, 0.05)"
    fill-information-strong: "#1A74A8"
    fill-yellow: "#FEC62E"                    # star rating only
    fill-inverse-strong: "#FFFFFF"            # on background-inverse
    fill-inverse-weak: "rgba(255, 255, 255, 0.06)"
    fill-inverse-hover: "rgba(255, 255, 255, 0.06)"
    fill-inverse-press: "rgba(255, 255, 255, 0.12)"
    fill-inverse-disabled: "rgba(255, 255, 255, 0.12)"
    fill-white: "#FFFFFF"                     # white in both modes, e.g. toggle knobs
    background-base: "#FFFFFF"
    background-raised: "#FFFFFF"
    background-overlay: "#FFFFFF"
    background-sunken: "#F5F6FA"
    background-alternate: "#F5F6FA"
    background-brand: "#4C64D9"
    background-inverse: "#12131A"
  dark:
    text-strong: "#FFFFFF"
    text-weak: "rgba(255, 255, 255, 0.78)"
    text-brand: "#A3B2FF"
    text-disabled: "rgba(255, 255, 255, 0.12)"
    text-error: "#FF9C9C"
    text-warning: "#E0BE70"
    text-success: "#77C7AF"
    text-information: "#7EC0E5"
    text-inverse-strong: "rgba(0, 6, 38, 0.9)"
    text-inverse-weak: "rgba(0, 9, 51, 0.65)"
    text-inverse-disabled: "rgba(0, 17, 102, 0.1)"
    stroke-strong: "rgba(255, 255, 255, 0.6)"
    stroke-weak: "rgba(255, 255, 255, 0.12)"
    stroke-focus: "#A3B2FF"
    stroke-selected: "#A3B2FF"
    stroke-brand-strong: "rgba(163, 178, 255, 0.8)"
    stroke-brand-weak: "rgba(163, 178, 255, 0.2)"
    stroke-error-strong: "rgba(255, 156, 156, 0.8)"
    stroke-error-weak: "rgba(255, 156, 156, 0.2)"
    stroke-warning-weak: "rgba(224, 190, 112, 0.2)"
    stroke-warning-strong: "rgba(224, 190, 112, 0.8)"
    stroke-success-weak: "rgba(119, 199, 175, 0.2)"
    stroke-success-strong: "rgba(119, 199, 175, 0.8)"
    stroke-information-weak: "rgba(126, 192, 229, 0.2)"
    stroke-information-strong: "rgba(126, 192, 229, 0.8)"
    stroke-disabled: "rgba(255, 255, 255, 0.12)"
    stroke-inverse-strong: "rgba(0, 13, 77, 0.45)"
    stroke-inverse-weak: "rgba(0, 17, 102, 0.1)"
    stroke-inverse-disabled: "rgba(0, 17, 102, 0.1)"
    icon-neutral: "rgba(255, 255, 255, 0.6)"
    icon-brand: "rgba(163, 178, 255, 0.8)"
    icon-error: "rgba(255, 156, 156, 0.8)"
    icon-warning: "rgba(224, 190, 112, 0.8)"
    icon-success: "rgba(119, 199, 175, 0.8)"
    icon-information: "rgba(126, 192, 229, 0.8)"
    icon-disabled: "rgba(255, 255, 255, 0.12)"
    icon-inverse: "rgba(0, 13, 77, 0.45)"
    icon-inverse-strong: "rgba(0, 6, 38, 0.9)"
    icon-inverse-disabled: "rgba(0, 17, 102, 0.1)"
    fill-brand-strong: "#A3B2FF"
    fill-brand-weak: "rgba(163, 178, 255, 0.06)"
    fill-strong: "#FFFFFF"
    fill-weak: "rgba(255, 255, 255, 0.06)"
    fill-weaker: "rgba(255, 255, 255, 0.03)"
    fill-hover: "rgba(255, 255, 255, 0.06)"
    fill-press: "rgba(255, 255, 255, 0.12)"
    fill-selected: "#A3B2FF"
    fill-disabled: "rgba(255, 255, 255, 0.12)"
    fill-overlay: "rgba(0, 13, 77, 0.45)"
    fill-error-strong: "#FF9C9C"
    fill-error-weak: "rgba(255, 156, 156, 0.08)"
    fill-warning-weak: "rgba(224, 190, 112, 0.08)"
    fill-warning-strong: "#E0BE70"
    fill-success-weak: "rgba(119, 199, 175, 0.08)"
    fill-success-strong: "#77C7AF"
    fill-information-weak: "rgba(126, 192, 229, 0.08)"
    fill-information-strong: "#7EC0E5"
    fill-yellow: "#FEC62E"
    fill-inverse-strong: "#12131A"
    fill-inverse-weak: "rgba(0, 21, 128, 0.04)"
    fill-inverse-hover: "rgba(0, 21, 128, 0.04)"
    fill-inverse-press: "rgba(0, 17, 102, 0.1)"
    fill-inverse-disabled: "rgba(0, 17, 102, 0.1)"
    fill-white: "#FFFFFF"                     # white in both modes, e.g. toggle knobs
    background-base: "#12131A"
    background-raised: "#1D1E26"
    background-overlay: "#292B33"
    background-sunken: "#000000"
    background-alternate: "#1D1E26"
    background-brand: "#A3B2FF"
    background-inverse: "#FFFFFF"
typography:
  # Sizes, line heights and letter spacing are in px.
  # All styles use Inter. "bold" is semi-bold (600) in Inter; "regular" is 400.
  # Mobile changes only size and line height, listed per group where different. Everything else stays the same.
  # In the Figma library, mobile sizes are applied with the Typography collection's Mobile mode.
  # Tablet uses the desktop sizes.
  display:   # hero headline, one per page
    regular: { weight: 400, size: 56, lineHeight: 64, letterSpacing: -1 }
    bold: { weight: 600, size: 56, lineHeight: 64, letterSpacing: -1 }
    mobile: { size: 40, lineHeight: 48 }
  heading-1:   # page title
    regular: { weight: 400, size: 40, lineHeight: 48, letterSpacing: -0.5 }
    bold: { weight: 600, size: 40, lineHeight: 48, letterSpacing: -0.5 }
    mobile: { size: 36, lineHeight: 44 }
  heading-2:   # section heading
    regular: { weight: 400, size: 32, lineHeight: 40, letterSpacing: 0 }
    bold: { weight: 600, size: 32, lineHeight: 40, letterSpacing: 0 }
    mobile: { size: 28, lineHeight: 36 }
  heading-3:   # sub-section heading
    regular: { weight: 400, size: 24, lineHeight: 32, letterSpacing: 0 }
    bold: { weight: 600, size: 24, lineHeight: 32, letterSpacing: 0 }
  heading-4:   # small heading
    regular: { weight: 400, size: 20, lineHeight: 28, letterSpacing: 0 }
    bold: { weight: 600, size: 20, lineHeight: 28, letterSpacing: 0 }
  body:   # long-form reading text
    regular: { weight: 400, size: 20, lineHeight: 32, letterSpacing: 0 }
    bold: { weight: 600, size: 20, lineHeight: 32, letterSpacing: 0 }
    regular-underlined: { weight: 400, size: 20, lineHeight: 32, letterSpacing: 0, decoration: underline }
    bold-underlined: { weight: 600, size: 20, lineHeight: 32, letterSpacing: 0, decoration: underline }
  small:   # UI text, labels, secondary text
    regular: { weight: 400, size: 16, lineHeight: 24, letterSpacing: 0 }
    bold: { weight: 600, size: 16, lineHeight: 24, letterSpacing: 0 }
    regular-underlined: { weight: 400, size: 16, lineHeight: 24, letterSpacing: 0, decoration: underline }
    bold-underlined: { weight: 600, size: 16, lineHeight: 24, letterSpacing: 0, decoration: underline }
  tiny:   # helper text, metadata
    regular: { weight: 400, size: 14, lineHeight: 20, letterSpacing: 0 }
    bold: { weight: 600, size: 14, lineHeight: 20, letterSpacing: 0 }
    regular-underlined: { weight: 400, size: 14, lineHeight: 20, letterSpacing: 0, decoration: underline }
    bold-underlined: { weight: 600, size: 14, lineHeight: 20, letterSpacing: 0, decoration: underline }
  uppercase:   # short overline labels only
    style: { weight: 600, size: 14, lineHeight: 20, letterSpacing: 2, textCase: uppercase }
spacing: [0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 56, 64, 80, 96, 128, 192, 256]
rounded: [0, 4, 8, 12, 16, 20, 24, 32]   # corner radius in px. Larger components get larger corner radii.
shadows:
  raised: "0 4px 8px -2px #0000000A, 0 2px 4px -2px #00000014"     # slightly elevated, e.g. cards, buttons
  overlay: "0 20px 24px -4px #00000014, 0 8px 8px -4px #0000000A"  # floating, e.g. menus, modals
  sunken: "inset 0 1px 4px #00000014"                              # recessed areas below the base surface level
layout:
  grid:
    desktop: { width: 1440, columns: 12, gutter: 32, margin: 120 }
    desktopWithSideNav: { width: 1440, columns: 12, gutter: 32, margin: 48, sideNav: 320 }
    tablet:  { width: 768, columns: 8,  gutter: 24, margin: 48 }
    mobile:  { width: 428, columns: 4,  gutter: 16, margin: 32 }
  minTarget: 48
  controlHeight: { small: 32, medium: 48, large: 56 }
  readingWidth: "40-80 characters per line"
icons:
  set: "Feather (outline, 24px grid). Use Lucide (lucide.dev) when Feather has no suitable icon."
---

```
╭───────────────────────────────────────────────────╮
│ ○ ○ ○                                  DESIGN.md  │
├───────────────────────────────────────────────────┤
│                                                   │
│                                   ■──────■        │
│        ╔═╗╦═╗╔═╗╔═╗╔╦╗╦╔═╗╔═╗╦    │ ╦ ╦╦ │        │
│        ╠═╝╠╦╝╠═╣║   ║ ║║  ╠═╣║    │ ║ ║║ │        │
│        ╩  ╩╚═╩ ╩╚═╝ ╩ ╩╚═╝╩ ╩╩═╝  │ ╚═╝╩ │        │
│                                   ■──────■        │
│                                                   │
│     Design guidelines your AI tools can follow    │
│                                                   │
│     ✓ Tokens     ✓ Guidelines     ✓ Components    │
│                                                   │
╰───────────────────────────────────────────────────╯
```

# Practical UI

Every design decision needs a logical reason. Aim for interfaces that are simple, familiar and accessible, with nothing that doesn't earn its place. These are guidelines, not strict rules. Only break one when you have a clear reason.

## Core principles

1. **Minimise usability risk.** If something might be unclear to someone with low vision, low computer literacy or reduced dexterity, change it. When unsure, choose the safer, more conventional option.
2. **Every detail needs a reason.** A detail earns its place if it adds meaning, helps people scan, or creates visual interest without getting in the way. Avoid decoration that could be mistaken for meaning, such as brand colour on text that isn't a link.
3. **Minimise interaction cost.** Reduce clicks, scrolling, typing, reading and remembering. Keep actions close to what they affect and make targets large.
4. **Minimise cognitive load.** Remove unnecessary information and choices, group related things, and use patterns people already know.
5. **Use common design patterns.** People expect your interface to work like others they use. Innovate only where it's the product's unique value.
6. **Be consistent.** Elements that look the same should work the same. Use the same word for the same thing everywhere.
7. **Meet WCAG 2.1 AA as a minimum.** Good accessibility is good usability for everyone.
8. **Less is more, but minimal isn't simple.** Never hide or remove information people need just to make a screen look cleaner.

## Using tokens

Tokens keep every screen consistent and make light and dark mode work automatically.

- Build every interface from the **semantic tokens** (`text-*`, `stroke-*`, `icon-*`, `fill-*`, `background-*`). Never hard-code colours or use the underlying primitive colours directly.
- Use only the spacing, radius, type and shadow values listed at the top of this file. Don't invent in-between values.
- Scale corner radius with component size: larger components get larger radii.
- Tokens come in **strong** and **weak** pairs. Strong carries meaning and draws the eye; weak supports it. Choose by importance: a heading is `text-strong`, its description is `text-weak`.
- The greys are semi-transparent, so they look right on any background. Don't replace them with solid greys.

## Colour

Colour should communicate meaning and keep content readable, not just decorate.

**Contrast**
- Text up to 18px: at least **4.5:1** against its background.
- Large text (above 18px bold, or above 24px regular): at least **3:1**.
- Borders and shapes people need to identify a control (input borders, checkbox outlines, button borders, toggle tracks), and icons that carry meaning: at least **3:1**. Use `stroke-strong` for borders and `icon-` tokens for icons.
- Purely decorative lines, such as dividers between page sections, may be lower contrast. Use `stroke-weak`.
- Avoid low-contrast grey text. Avoid maximum contrast too, such as pure black text on white, as the extreme contrast can cause eye strain when reading. Use `text-strong` for high-contrast text, and `text-weak` for secondary text, which is slightly lower contrast but still meets contrast minimums.
- For dark mode, aim well above the minimums. Dark interfaces are harder to see in bright light.

**Brand colour**
- There is **one brand colour**. Use it to mark things that are **interactive**: links, buttons, selected states and focus rings.
- Never apply the brand colour to non-interactive text, such as headings or body text, because people will think it's clickable. Icons in an icon container and highlight badges, such as "New", are the exceptions: they can use the Brand tone.
- Design in black and white first. Add colour only where it conveys meaning.
- You don't have to use the brand colour. A neutral, black-and-white interface, with Neutral-tone buttons and links, can look timeless and keeps the focus on content. Add colour only where it carries meaning.
  - For a neutral brand in the Figma library, set the frame's Brand mode to Neutral. Buttons, links, selected states, sliders and focus rings then all turn grey, so there's no need to override colours by hand.
- If the brand colour already has a meaning, such as red, green or amber, don't use it for interactive elements like buttons and links. People may mistake it for an error, success or warning. Use the Neutral tone for interactive elements instead.
- On marketing pages, background colours can separate and highlight sections. `background-alternate`, `background-inverse` and `background-brand` are all fine, and can each be used more than once.

**Meaning**
- Never rely on colour alone. Pair status colours with an icon or a word, underline text links, and give error fields a thicker border and a background as well as a colour.
- Use the 4 status colours only for status: error (red), warning (amber), success (green), information (teal). Status includes small cues that match the colour's meaning, such as green ticks for included features, red crosses for excluded ones, or trend badges: success for a good change, and warning or error for a bad one. Use the matching `icon-` token, such as `icon-success`.
- An alert is `fill-<status>-weak`, with a `stroke-<status>-weak` border, an `icon-<status>` icon, `text-strong` for the heading and `text-weak` for supporting text.

**Surfaces and depth**
- Surfaces sit on different levels of elevation. From lowest to highest: `background-sunken` for recessed areas below the page, `background-base` for the page itself, `background-raised` for cards, and `background-overlay` for menus, dropdowns and modals. Directly below a modal, a semi-transparent `fill-overlay` layer dims the rest of the page.
- Use shadows and background colour together to show elevation. Shadows work well in light mode, where the base, raised and overlay surfaces are all white. In dark mode, shadows are hard to see, so each higher level uses a progressively lighter background colour instead. Light comes from above, so the highest level is the lightest. Always apply both the background token and the shadow, so elevation works in either mode.
- Use the 3 shadows only:
  - `sunken` (an inner shadow): recessed areas below the base surface level, such as the tracks of toggles, sliders and progress bars.
  - `raised`: slightly elevated elements, such as cards, primary buttons and toggle knobs.
  - `overlay`: elements floating high above the page, such as menus, dropdowns, tooltips, modals and drawers.
- More important elements can sit higher (more elevated) to make them more prominent.
- A `stroke-strong` border can add emphasis to a card or container, such as a highlighted item in a group.
- For a lightly tinted panel or section, use `fill-weaker` or `fill-weak`. Keep `background-sunken` for areas that sit below the page.

**Text on photos**
- Text on images must still meet contrast minimums. Add a dark gradient or a semi-transparent overlay behind it, or place it on a solid background.

## Typography

These guidelines are about readability: making text easy to scan, read and understand.

- Use **a single sans serif typeface** (Inter). A second typeface may be used for headings only, to express brand personality.
- Use **2 weights only**: a heavier weight for headings and emphasis, and a lighter weight for everything else. More weights add visual noise and are harder to use consistently. With Inter, that is semi-bold (600) and regular (400), because Inter's bold is too heavy. With another typeface, pick whichever heavy weight looks balanced (bold or semi-bold). Never use thin or extra-bold weights for small text, as they can be difficult to read.
- Use **only** the styles in the type scale. Don't create new sizes. Use `bold` variants for headings by default; `regular` heading variants are for a lighter, softer look. Use `underlined` variants for links.
- Type styles can be used for their size, not just their role. A price or key number can use a heading style without being a heading in the page structure.
- Use `display` at most once per page. Use headings in order; don't skip levels for visual effect.
- **Choose body text sizes by level:**
  - `body` (20px): long reading text, such as articles and help pages, and supporting text under large headings (`display` and `heading-1`). In compact layouts, such as forms, modals and narrow columns, supporting text uses `small` to match the content around it.
  - `small` (16px): most interface text, such as labels, cards, tables, lists, chat messages and supporting text under smaller headings.
  - `tiny` (14px): sparingly, for metadata and helper text, such as timestamps and hints.

  Use `heading-4` only for headings, never for paragraphs. Never set long reading text smaller than 18px, to keep it readable.
- Use the line heights from the type scale. Smaller text gets proportionally more line height (about 1.5 for 14–16px text), and line height decreases as font size increases (headings are tighter).
- Supporting text under a heading uses `text-weak`.
- Keep reading text to **40–80 characters per line**. Longer lines make it hard to find where the next line starts, and shorter lines make the eye jump back too often. Don't let paragraphs span the full width of a wide screen.
- **Left align** text. A straight left edge gives the eye a consistent place to return to at the start of each line. Centre alignment is fine for headings and text blocks of 1–2 short sentences. Keep centred text centred on every screen size.
- Never justify text. Uneven letter and word spacing makes lines harder to follow, especially for people with dyslexia.
- Use the letter spacing from the type scale: -1px for `display`, -0.5px for `heading-1`, 2px for `uppercase` labels, 0 for everything else.
- Use uppercase only for short labels (`uppercase` style), never for headings, buttons or sentences. All-caps words share the same rectangular shape, so people have to read them letter by letter. Short uppercase labels above section headings, sometimes called eyebrows, help people scan long pages. Use a few words to name the section, such as "Pricing", and let the heading say the benefit.
- Align different-sized text on the same line to the baseline, not the vertical centre (for example "$79" and "/one-off"). It's easier to read and looks neater, because more elements line up.

## Layout and spacing

Spacing and alignment show how things relate and what matters most, so people can scan a screen quickly.

**Group related elements.** Show relationships with, from strongest to weakest:
1. a shared container (card, border, background),
2. proximity (related things close together),
3. similar appearance,
4. alignment along a common line.

**Spacing**
- Space elements by how closely related they are. **The gap inside a group is always smaller than the gap between groups.**
- Start with the smallest gaps for the innermost elements and increase as you move outwards.
- Padding inside an element scales with its size: small elements get small padding, large containers get more.
- Be generous with white space. When unsure, choose the larger option.

**Visual hierarchy**
- Show information in order of importance using size, colour, contrast, spacing, position and depth. If everything is emphasised, nothing is.
- Squint test: when blurred, the most important element and the purpose of the screen should still be obvious.

**Alignment and grid**
- Align the main layout to a **12-column grid** on desktop, 8 columns on tablet and 4 columns on mobile, using the gutters and margins listed at the top of this file. Gutters stay empty. With a side navigation on desktop, the grid sits in the content area next to it, with 48px margins. In Figma, apply the Desktop with Side Nav grid guide to the whole frame, as it already allows for the 320px navigation. On tablet and mobile, the navigation moves to the top, so use the standard Tablet and Mobile grid guides.
- Use the grid for the page margins and main containers, such as rows of cards. Elements inside them use the spacing tokens instead.
- The gutter is the minimum gap. Put more space between major side-by-side areas, such as an image and its details, or a main column and a sidebar, for example 64px. Keep their outer edges on the grid, even if the edges between them don't line up with columns.
- In Figma, add the matching Grid guide from the library (desktop, tablet or mobile) to the frame, and place the main layout on its columns.
- Keep text on a single strong left edge, and don't mix left, centre and right aligned text in the same group. Images and video can be wider than the text column, or the full page width, as long as the text keeps the left edge. Familiar patterns can be exceptions, such as your own messages on the right in a chat, prices and totals on the right of an order summary, or account and cart links on the right of a header. Text inside each element still stays left-aligned.
- Small supporting elements, such as badges, toggles, icon buttons and close buttons, can sit on the right edge of their container. A clean right edge works alongside the left one.
- Keep icons aligned with text so the left edge of the text stays straight.

**Marketing and editorial pages**
- Marketing, landing and editorial pages, such as articles, can be more expressive than app screens: larger type, more white space and wider images. The colour, contrast, accessibility and interaction guidelines stay the same.
- Keep reading text in a narrower column, about 8 of the 12 desktop columns. The column can sit in the centre of the page, but the text inside it stays left-aligned.
- Vary image widths to add rhythm: text width, wider than the text, or full width. Save full-width images for the most important ones. On mobile, images go full width.
- Use more space between story sections than in app screens, such as 96px on desktop and 64px on mobile, so long reads feel calm.

**Robustness**
- Design mobile first, then expand. Don't fill large screens just because there's room.
- Make components handle long text, large numbers and empty states. Let content reflow rather than truncating essential information. If you must truncate, truncate in the middle when the ends differ.
- Keep important content and actions visible. If something is off-screen (such as a carousel), show a hint of it so people know it's there.
- On tablet, use the extra width. Put images beside text, or 2 cards per row, where it fits, rather than stacking everything as on mobile.

## Copywriting

Clear, concise text lowers cognitive load, so people understand each screen at a glance.

- **Be concise.** Cut filler words and introductory phrases ("would you like to", "in order to", "please", "sorry", "oops"). Keep sentences under 20 words.
- **Sentence case everywhere**, including headings, buttons and labels. No title case, because capitals on every word interrupt scanning.
- Proper names, such as product names and the titles of songs, albums, books or playlists, keep their own capitalisation.
- **Plain language.** Write as if talking to a 6th grade student. No jargon, slang or unexplained abbreviations or acronyms.
- **Front-load** headings, links and list items: put the key information first.
- Use the inverted pyramid: the key point first, supporting detail next, background last (or on another screen).
- Break up content with **descriptive headings** and bullet points. Headings must make sense on their own.
- Use **numerals** ("3 items", not "three items"), as they're easier to scan and quicker to read than words. Use commas in thousands (1,000) and simplify large numbers (1.2 million).
- Avoid full stops in short interface text that isn't a full sentence, such as labels, buttons and headings, to keep the interface simple. Full sentences get a full stop, including text with more than one sentence. Be consistent across similar elements shown together, such as descriptions in a row of cards.
- Use vocabulary consistently. If the button says "Add to cart", the destination is "Cart", not "Bag".
- Don't use "my" or "your" on form labels ("Email", not "My email"). "My email" sounds like the interface's email, and "your" is unnecessary.
- **Links describe their destination.** Never "click here", "read more" or "learn more" on their own.
- **Error messages** say what happened and how to fix it. Never blame the person. For example: "Your payment wasn't successful. Check your card details and try again".

## Interaction states

States give people feedback, so they always know what's interactive and what's happening. These guidelines apply to every interactive element: buttons, links, form controls, tabs, cards and menu items.

- Every interactive element has distinct default, hover, press, focus and disabled states. Hover and press lay `fill-hover` / `fill-press` over the resting fill, so the base colour never changes.
- Text links and other text-only controls show hover by toggling the underline: underlined text loses it, plain text gains it.
- Not all links need an underline. Navigation menus, tabs and cards already look interactive, so they can drop the link styling.
- Disabled elements use the `-disabled` text, icon, stroke and fill tokens, and lose their shadow.
- Focus shows a solid **2px `stroke-focus` ring with a 2px gap** on every interactive element. Never remove focus styles.
- On inverse and brand surfaces, the focus ring uses `stroke-inverse-strong`.

## Buttons

Buttons should make the next step obvious.

**3 weights**
- **Primary:** solid `fill-brand-strong`, white text. Use **only one in view at a time**, for the single most important action, so people know what to do next and buttons don't compete for attention. On long pages, the primary action can repeat further down, once the first one has scrolled out of view. Use secondary or tertiary buttons for all other actions, including repeats of the primary action that would appear in the same view. If no action is most important, use no primary button.
- **Secondary:** no solid fill, a brand-coloured border and a `text-brand` label. Use it for alternatives (Back, Cancel) or several actions of equal importance.
- **Tertiary:** looks like an underlined text link in `text-brand`. Use it for the least important actions, and for destructive actions you want to de-emphasise. Tertiary buttons have padding on each side, so their label sits slightly in from the edge. When the action needs to line up with the left edge of the content, such as on its own below a paragraph, use a text link instead.
- Don't style secondary buttons with a low-contrast grey fill or outline, because they look disabled.

**Labels**
- Use a verb plus a noun that describes the outcome, so the label makes sense out of context, for example to screen reader users: "Save article", "Delete account", "Start free trial". Never "OK", "Submit" or "Yes" alone. Short, familiar labels such as "Log in", "Sign up" and "Cancel" are fine as they are.

**Layout**
- Order buttons by importance, **left to right**, with the primary button first and aligned to the left edge of the content. People scan down the left edge, and right-aligned buttons can be missed on large screens and by people using screen magnifiers.
- Exception: for a single field, such as a search field or an email sign-up, the primary button can sit attached to the right of the field. This saves space and shows the field and button belong together.
- In narrow, single-column forms, such as a login or sign-up form, buttons can be full width to line up with the fields.
- On mobile, stack buttons full-width, most important at the top.

**Size**
- Aim for targets of at least **48×48px**, with at least **8px** between adjacent targets. Make frequently used buttons larger.
- Small components such as checkboxes, radio buttons, toggles, tags, text links and small (32px) buttons can be smaller to save space. Keep them at least **24×24px**, and make the label part of the clickable area where possible, so the target is still easy to hit.
- Default control height is 48px (56px large, 32px small).

**Disabled buttons**
- **Avoid disabled buttons.** They don't explain why, are hard to see, and can't receive keyboard focus. Keep the button enabled and explain what's missing when it's pressed. If an action isn't available, remove it and say why, or show a lock icon with an explanation.

**Destructive actions**
- Add friction in proportion to the damage: make the action less prominent first; ask for confirmation for moderate actions; use a red confirmation and a required checkbox for severe ones (such as deleting an account).
- Don't make the initial destructive action red or prominent. Red goes on the confirmation step.
- Confirmation and undo are both optional, and depend on how much harm a mistake would cause. A minor action may need neither. Others may need only an undo, only a confirmation, or both. Add more protection as the potential harm increases.

**Icons with text**
- Match icon size and stroke weight to the text.
- Use 20px icons in default 48px controls, 16px icons in small 32px controls, and 24px icons in large 56px controls.
- Don't use icon-only buttons for important actions, unless the icon is universally familiar, such as close, menu, or play, pause, previous and next in a media player. Give every icon-only button an accessible name.
- Icons that support a heading or message help people understand it at a glance, such as a plan icon on a pricing card or a trash icon in a delete confirmation. Use an icon container for these, in the Neutral or Brand tone, or the matching status tone when the message is about a status, such as error in a delete confirmation, and keep them supporting the text, not replacing it.
- Use Feather icons. Feather is a small set, so use Lucide (lucide.dev) for anything it doesn't have. Lucide extends Feather with the same outline style. Never mix in icons from other sets.

## Forms

Forms should be quick to complete, with as few errors as possible.

**Layout**
- **Single column.** Don't put form fields side by side. One downward path is faster and makes fields less likely to be missed.
- Exception: short, related fields can sit side by side if they stay within the width of the single column, such as a card's expiry date and security code. This keeps the form shorter without the problems of a multi-column layout.
- **Labels go above fields**, 4px above, never to the left. Labels on the left increase cognitive load and slow people down.
- Group related fields under headings. Break long forms into steps of related questions, show progress, and order questions from easiest to hardest.
- Match field width to the expected input (a 4-digit postcode field is short).

**Fields**
- Use conventional field styles: a rectangle with a `stroke-strong` border (at least 3:1 contrast), label on top.
- **Never use placeholder text as a label.** It disappears when people start typing, can look like a pre-filled value, and is usually too low in contrast. Avoid placeholder text in general. Search boxes and chat message boxes are the exceptions, because their purpose is clear from where they sit. Their placeholder must still meet 4.5:1, and they still need a label for screen readers, even if it's hidden.
- Put hints **between the label and the field**, not below (keyboards and autofill menus cover space below the field).
- Field labels use regular weight `small` text in `text-strong`, not bold. Hints, "(optional)" and error messages use `tiny` text. Hints and the asterisk use `text-weak`.
- Mark required fields with an asterisk (*) **and** mark optional fields with "(optional)". State the convention at the top of the form.
- Exception: you can leave required fields unmarked when there are no optional fields and the form is short and familiar, such as a login, a newsletter sign-up or a checkout. People already understand these fields are needed. The same applies to forms that ask one question per screen and explain why each question is asked.
- Ask only for information you truly need. Replace optional fields with an opt-in checkbox that reveals the extra field.
- When people can choose several options from a set of checkboxes, but some people will choose none, ask a yes/no question with radio buttons first, and show the checkboxes only after "Yes". Don't add a "None of these" checkbox, because checkboxes are independent: ticking one shouldn't clear the others.

**Choosing controls**
- About 10 options or fewer: **radio buttons**, not a dropdown, unless saving space matters more than seeing every option, such as a sort control above a list or table.
- Long lists people know the answer to (country, city): **autocomplete**.
- Small numbers (quantity, guests): a **stepper** with horizontal +/− buttons at least 48px.
- A single on/off choice that takes effect on submit, or that reveals an optional field: **checkbox**. A setting that takes effect immediately, such as turning notifications on: **toggle switch**.
- In filters, use checkboxes and radio buttons, even when results update straight away, so all the filters look and work alike.
- Phrase checkboxes positively ("Send me product updates", not "Don't send me updates").

**Validation**
- Simplest approach: validate on submit. It's the easiest to design and build. If there are multiple errors, say how many and list them at the top of the form, each linking to its field. A single error only needs its message by the field.
- Invalid fields get a 2px `stroke-error-strong` border on `fill-error-weak`, with an error icon. The error message in `text-error` sits **between the label and the field**.
- The blue focus ring still appears on invalid fields, outside the red border.
- Never disable the submit button to prevent errors.

## Components

Build interfaces from these components. Use these names when describing a design, so it maps directly to the Practical UI Figma file. Don't invent a new component when one of these fits. Adapt components to the content by swapping text, images and optional parts. If no component fits, build from smaller components such as buttons, badges and cards, following the same patterns. When a library component's built-in styling, such as its colours, type or alignment, differs from these guidelines, keep it as it is and mention the difference, rather than overriding it. Choosing variants and rearranging layouts is fine.

### Actions

- **Button**: Primary, Secondary, Tertiary · Brand, Neutral, Inverse, Destructive · Large, Medium, Small. Use one primary button in view at a time, for the most important action.
- **Button icon**: an icon-only button, square or circle · Medium, Small. Only for universally familiar icons (close, menu, more, media controls), and give it an accessible name.
- **Button group**: 2–3 buttons in order of importance · Horizontal or Vertical. Primary first, left-aligned.
- **Text link**: for navigation and low-emphasis actions within text. Link text describes where it goes.
- **Dropdown menu**: a list of actions opened from a button, button icon or avatar. Use it for secondary actions, and try to keep important actions visible.

### Form controls

- **Text input**: single-line text. Label above, hint above the field.
- **Text area**: multi-line text, such as messages or descriptions.
- **Radio button group**: pick one option. Prefer radio buttons over a select for about 10 options or fewer, because every option stays visible. Stack them vertically.
- **Checkbox**: one or more options, or a single opt-in that takes effect after a form is submitted. Use positive phrasing in labels. Stack them vertically.
- **Toggle**: a single on/off setting that takes effect immediately.
- **Select**: pick one option from a list when saving space matters more than visibility.
- **Autocomplete**: pick from a long list where people know what they're looking for, such as country or product. Single or multiple selection.
- **Combobox**: similar to a select, but also filters the list of options as people type a search term. Single or multiple selection.
- **Stepper**: small numeric changes, such as quantity or number of guests. Use +/− buttons placed horizontally. Not for large numbers.
- **Segmented control**: switch between 2–5 related options or views, such as Day, Week and Month in a calendar.
- **Slider**: choose an approximate value in a range, where precision doesn't matter.
- **Date picker**: choose a date from a calendar, or type it in as text using slashes.
- **Search input**: lets people find content by entering a search term. The search can run as people type, when they press a search button, or when they press Enter on the keyboard.
- **File upload**: drag-and-drop zone with a file list showing upload progress.
- **Rating**: displays user feedback on a scale of 1 to 5, using stars or hearts.

### Feedback and status

- **Alert**: conveys an important message or status · Error, Warning, Success, Information, Neutral, Brand. Always pair the colour with an icon and a clear message.
- **Alert global**: a page-wide message across the top of the screen, such as a system outage.
- **Tooltip**: a short message on hover or press, such as explaining why a button is disabled. Never hide critical information in a tooltip.
- **Badge**: a non-interactive label for a status or a highlight · Error, Warning, Success, Information, Neutral, Brand. Badges for a status use the matching status tone, such as Success for "Verified". Badges that highlight something, such as "New" or "Most popular", can use the Brand tone. The rounded shape and light fill keep badges looking less prominent than buttons, so they don't look clickable.
- **Badge count**: shows the number of notifications, such as unread messages, on an icon or tab.
- **Badge dot**: online, busy, away or offline status, or a new notification.
- **Tag**: a small pill that categorises or labels content · selectable or dismissible. Selectable tags can act as compact checkboxes for short options.
- **Loading bar**: progress people can see but not change, such as an upload or a song playing in a mini player. Use a Slider when people can drag to change it.
- **Progress indicator**: shows which step people are on in a multi-step flow.
- **Empty state**: shown when there's nothing to display, such as a new account, a search with no results, or a page that no longer exists. Explain why and what to do next.

### Containers and overlays

- **Card**: displays content and actions on a single topic, such as a product, article, project or person · Vertical or Horizontal. Cards are often interactive: the whole card can be clickable, and it lifts on hover. In a row of cards, line up matching content, such as prices and buttons, by pinning it to the bottom of each card.
- **Modal**: presents critical information or a focused task that needs immediate attention, such as confirming an action · Default or Destructive. It floats above the page and blocks interaction with the content underneath. Use sparingly.
- **Drawer**: a panel that slides in from the right when people press a button or other control · Small or Large. Like a modal, it floats above the page and blocks interaction with the content underneath. Use it for secondary tasks that don't need a new page.
- **Accordion**: a list of headings that show or hide more information when pressed. It turns sections of content into a scannable list, such as FAQs. Don't hide content people need.
- **Divider**: separates groups of content · Weak or Strong. Prefer spacing first.

### Data display

- **Table**: presents information in a clear, organised structure that's easy to read and compare · Default or Striped. Right-align numbers.
- **Summary list**: displays pairs of data, such as account details, or the answers people gave in a form, such as a review step before submitting.
- **Avatar**: a person, shown as a photo, icon or initials · also stacked or labelled.
- **Icon container**: makes an icon larger and more prominent, and gives it a consistent circular shape, to support a heading or message.

### Navigation

- **Navigation header**: the main navigation across the top of the page.
- **Navigation side**: navigation in a sidebar, for apps with many sections.
- **Tabs**: used for navigation, or to show and hide content.
- **Breadcrumbs**: links to the pages above the current one, so people can go back. Don't include the current page, because people can already see where they are.
- **Pagination**: lets people move between pages.

### Page sections

- **Hero**: the opening section of a page, used to introduce the page or product.
- **Text block**: a heading with supporting text, left-aligned or centred.
- **Testimonial**: a quote from a real customer.
- **Footer**: links and information at the bottom of the page.

## Common AI mistakes to avoid

| Don't | Do |
|---|---|
| Use several brand or accent colours for decoration | Use one brand colour, only on interactive elements |
| Colour headings or body text with the brand colour | Keep headings `text-strong` and body text `text-strong` or `text-weak` |
| Use low-contrast grey body text for a "clean" look | Use `text-strong` for body and `text-weak` for secondary text, both above 4.5:1 |
| Put multiple primary buttons in view at once | One primary in view, the rest secondary or tertiary |
| Right-align or centre form buttons | Left-align buttons, primary first |
| Use placeholders instead of labels | A visible label above every field |
| Put form fields in two columns | One column, labels on top |
| Use title case on headings and buttons | Sentence case everywhere |
| Use "Learn more" or "Click here" links | Links that name their destination |
| Use 4+ font weights and arbitrary sizes | One light and one heavy weight (regular and semi-bold in Inter), sizes from the scale only |
| Use equal spacing everywhere | Small gaps inside groups, larger gaps between them |
| Centre long paragraphs | Left-align, 40–80 characters per line |
| Use dropdowns for 3–5 options | Radio buttons |
| Disable buttons until a form is valid | Enable them and show clear errors on submit |
| Rely on red alone to show an error | Red plus an icon, a thicker border and a message |
| Use icon-only buttons for unfamiliar key actions | An icon with a text label |
| Use glassmorphism, neumorphism or trendy effects | Simple, high-contrast styles that age well |

## Before you finish, check

- [ ] One primary button in view at a time, and it's the most important action
- [ ] Brand colour only on interactive elements
- [ ] All text meets 4.5:1 (3:1 for large text); control borders meet 3:1
- [ ] Nothing relies on colour alone
- [ ] Every value comes from the tokens
- [ ] Spacing inside groups is smaller than spacing between groups
- [ ] Text is left-aligned, sentence case, 40–80 characters per line, line heights from the type scale
- [ ] Targets are at least 48×48px (small controls at least 24×24px), with 8px between them
- [ ] Every interactive element has hover, press and visible focus states
- [ ] Forms: single column, labels above, hints above fields, required and optional both marked (except short, familiar forms)
- [ ] Buttons and links say exactly what they do or where they go
- [ ] Works on a small screen and with long content
