# The panel as a small dashboard

Status: proposed, 2026-11-25. Decision track. Requested by the operator after the 1.32.1 run panel field
check.

## Decision

The plugin's pane grows from a run monitor into a small, read-only dashboard of the work in progress.
It answers three questions, in this order:

1. **What is happening now?** Every unit `tp run` holds, and every task an agent holds outside a run:
   claimed and `wip` tasks, who holds each, for how long, in which tree, and land requests waiting.
2. **What needs me?** Blockers, escalations, operator decisions and round caps, in plain words.
3. **How far along is it?** Tasks done, ready and blocked; each open epic's progress.

It has five views behind hotkeys: **Now** (the default, all three answers in one screen), **Tasks**,
**Epics**, **Run** (today's panel) and a task detail opened from any list (title, acceptance, touches,
holder). In the desktop app's `widget` seat it draws a glance form: phase, the count in progress and
the first line of "needs you".

**One payload.** The pane reads one new read-only command, `tp now --json`, instead of polling
`tp resume`, `tp run --status`, `tp list`, `tp epic status` and the claim state separately every few
seconds. The payload carries exactly what the pane draws and nothing else, so it is equally a cheap
"where are we" for an agent (P3): one call where an agent today makes three.

**Language.** Every word the pane draws comes from a two-language table, English and Turkish. The
language is chosen by the first of: a project setting in `.tp/config.json`, Claude Code's own language
setting where a module can read it, the system locale, English. tp's own terms are translated
(`implement phase` reads `uygulama aşaması`); what the operator wrote, such as task titles and epic
names, is shown as written. `tp now --json` stays English and machine-shaped; translation is the
pane's job.

## Rules it keeps

- **Read-only.** A button switches views or opens a detail; nothing starts, stops, lands, resolves or
  claims. Agents keep running tp.
- **`DESIGN.md` and rule 14.** Built from the design system's tokens as far as a terminal carries
  them (bold, dim, one accent, the theme's status colours), and no tp command or flag in plain view.
- **Cheap when idle.** A live run or held task is read every 5 seconds, an idle project once a minute,
  as today.

## Alternatives

- **Poll the existing commands from the pane.** No tp change, but five processes every 5 seconds,
  five payload shapes to keep in step, and nothing an agent gains. Rejected.
- **Point at the web dashboard (`tp dashboard`).** It already draws the task graph, but it is a
  browser page beside the session, not the session itself, and it shows structure, not what is running.
  The two stay: the web dashboard for the graph, the pane for now.
- **Actions in the pane** (land, stop a run). Declined by the operator: a second place that acts on
  tp is a second place to get wrong.

## Non-goals

- Editing tasks, specs or epics from the pane.
- Live logs or transcripts of a unit.
- A language beyond English and Turkish in this release.
- The latest finished unit in `tp run --status` (open in `spec/1.32.0-measurements.md` §4), unless
  `tp now` carries it anyway.

## Settled at build time, not here

The exact field list of `tp now --json`, the hotkeys, the row layout per view and width, how the
pane learns Claude Code's language setting, and the setting's name in `.tp/config.json`.
