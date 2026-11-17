# tp — An epic's owner

A backlog decision candidate for 1.23.0, asked for by the requesting project, whose epic graph is
split across several people. The operator settled its open points before the note was written: the
field name, one field for work and conditions alike, and every dashboard view below.

Class: **decision**. One optional epic field, one declared set, read by the ready and status
reports and the dashboard.

## 1. The decision

**Context.** Nothing in `.tp/epics.json` says whose an epic is. `tp epic claim --by` answers who is
on it now and lasts as long as the work; whose job it is, and who decides a condition, lives in
description prose. The two diverge: an agent claims for an hour an epic a person owns.

**Decision.** An optional list of handles, `epics[].owner`, and a declared set they come from.

- **The set.** A top-level `owners` list, each entry a `handle` and an optional display `name`,
  written like `milestones`: `tp epic set owners='[{"handle":"ana","name":"Ana"}]'` replaces it
  whole. Nothing else about a person is stored (no email, no account): tp reads nothing else.
- **The field.** `tp epic set <id> owner='["ana","ben"]'`, an apply `set` row, `tp epic add` and
  `tp epic import` write it; `[]` unsets it and an unset owner is not written. The list keeps the
  order it was written in, never sorted: the first owner is the one the map paints. On a work epic
  it reads as whose job it is; on a condition, as who decides it. There is no second field.
- **Handles.** Compared byte for byte, and otherwise opaque. `owner-handle` (error) flags what would
  bring a typo split back inside the set: a blank handle, one with leading or trailing space, two
  declared handles equal ignoring case, and one epic naming a handle twice. `unknown-owner` (error):
  an epic names a handle the set does not declare.
- **Writes.** A write may not add a lint error (exit 1), so dropping a handle an epic still names,
  or naming one before it is declared, is refused by the write itself, as dropping a milestone in
  use is. `tp epic apply` lints once after its last row, so a rename is one apply file: re-point the
  epics, then replace the set. Done by hand it is three writes: declare, move, drop.
- **Import.** A graph file carries its own `owners`, so an import carries the set with the epics.
  A file whose epics name handles it does not declare is written and its `unknown-owner` findings
  reported, as import treats every lint error today.
- **Claim is untouched.** `claimed.by` stays free text, never matched against handles.
- **Not scope.** An owner is not in an epic's review block or the scope fingerprint, so setting or
  moving one re-asks no review unit.
- **Ready and status.** Each ready, claimed, blocked and condition entry carries `owner` when set.
  Both commands take `--owner <h>` (repeatable) and `--unowned`; given together they are a union,
  and an entry matches when its owner names any of them. A handle outside the set is exit 2. Gates
  and threshold rewrites belong to milestones and are never filtered.
- **Export.** The raw graph in `tp epic export` carries both keys (additive). `--md` gains an owner
  column after tags.
- **Dashboard.** Display names stand in for handles where set. Every handle and name is author
  content and is escaped where it is written into the page, as the project name is.
  - The epic drawer lists the owners in their order.
  - The map gains an owner selector that keeps one owner's epics lit and dims the rest (the
    lifecycle band and search already dim this way), and a colour-by-owner switch. Today a node's
    colour is its state mark; with the switch on, the fill is the owner's colour and the border
    keeps the state mark, and the legend shows the two under separate headings. Colours go by
    position in the declared set (stable across reloads and machines), from an eight-colour palette
    defined for both themes; owners past the eighth share one "other" swatch. An epic with more
    than one owner is painted in its first owner's colour and carries a "+N" mark.
  - The operator view groups the conditions waiting on a decision under "waiting on <owner>". A
    condition with two owners is listed under each; a group's count is by membership, so the groups
    can sum past the conditions, and the header states the number of distinct conditions. Legend
    counts are by membership too, never by paint.
  - Unowned epics are their own group, colour and legend entry, counted and never hidden. The API
    keys that group by null, never by a string a handle could be.
- **A graph without owners.** With no `owners` declared, every output is what 1.22.3 wrote: no
  `owner` keys, no `--md` column, no selector, switch, unowned group or legend entry.

**Consequences.** An owner is a statement of the graph's authors, not an identity tp checks. The
set is a second place a handle lives, so a person's rename touches every epic they own.

## 2. Alternatives

- **Free-text owner without a set.** One write fewer, and typos become new people: the filter and
  the grouping split one person in two silently. The set is what makes the filter trustworthy.
- **A separate `deciders` field on conditions.** Two lists that mean "whose call" on two kinds of
  epic, and a lint rule to keep them apart; the operator chose one.
- **Owner as a tag (`owner:ana`).** Tags already drive tag checks and forbidden edges; overloading
  them would put people into rules about structure, and no epic command filters by tag today.
- **Colour by hash of the handle.** Stable without a set, but two people can draw the same colour
  and nothing bounds the palette.

## 3. Non-goals

- Owner never gates a command: tp cannot know who runs it, and `--by` is self-declared.
- No owner on a milestone and no default owner for a milestone's epics.
- No notifications, no linking of `--by` to handles, no per-owner capacity or load figures.

## 4. Release criterion

- A graph with no `owners`: `ready`, `status`, `export` and `export --md` byte-equal to 1.22.3's.
- A synthetic graph in a clone (three owners, an unowned condition, a condition with two owners, an
  owner removed while in use, a rename by one apply file) exercised through every command above.
- Export then import into an empty project round-trips both keys.
- The dashboard checked by hand in a browser, in both themes, against counts written down before
  opening it: drawer, selector, colour and legend, operator grouping with the two-owner condition,
  the unowned count, and a handle and name carrying markup shown as text.

Fixtures use invented handles only.
