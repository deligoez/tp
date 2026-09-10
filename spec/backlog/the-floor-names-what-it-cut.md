# tp — The floor names what it cut

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `the-floor-names-what-it-cut-measurements.md` beside it, and this file stands
without them. It absorbs `floor-anchors-need-fixtures.md` as one test task (§7.1).

Class: **tool** — it changes how a spec is cut into units and what `--units` prints, and nothing
that decides whether a round converges.

## 1. The decision

**Context.** The floor splitter ends a sentence at every `.` followed by whitespace. A number that
is not the end of anything therefore ends a sentence too, and the floor's arms then keep the
fragment holding the digit and cut the fragment holding the claim. Nobody is told: the claim is
cut, a cut unit owes no disposition, and no round grades it. A field report (WB-3155) found it on a
Turkish spec, with an ordinal mid-sentence: `§6.2 ise 6. adımdan …`. An English control loses its
claim the same way: `Step 6. writes the round file …`. The same defect has a second shape
in this repository's own specs: a numbered list written directly under its lead-in line, with no
blank line between, never opens the marker gate, so every item's marker stays in the joined text,
each item's unit ends with the next item's number, and the last item is cut. The bare-marker units
this spec used to leave unexplained come from the same closed gate: where an item ends in a full
stop, the next item's marker is split off as a unit of its own. Both shapes are reproduced in the
sidecar under *Field report WB-3155* and *The splitter census*.

Separately, a reader who does find a defect in cut text can name the unit — the floor index gives
every cut unit an id — but cannot see its text or take its hash from tp, and the emitted prompt
tells the reader to compute the hash by hand.

**Decision.** Three parts. The splitter stops ending a sentence at a mid-sentence ordinal, opens
the marker gate for a list that follows a lead-in ending in `:`, and never emits a bare marker as a
unit (§3). `--units` lists cut units with their text and their hash (§2). The anchor function gets
the fixtures it has never had, as a test task with no decision attached (§7.1).

**Consequences.** Every unit whose segmentation changes gets a new `text_sha`, so the carry treats
it as new and the next round asks it once — including units no author touched. A floor can shrink
as well as grow: a list item kept only because it carried the next item's number is judged on its
own text afterwards, which is what the arms were always meant to do. §3 reverses a residue
`spec/1.0.0.md` §2.1 step 3 recorded as unrepairable, and inverts that spec's row 18e. The line
count of `tp ground <spec> --units` stops equalling `floor_size` (§2).

## 2. A cut unit can be named with its text

**What already works, corrected from an earlier draft of this file.** Cut units have had ids in the
floor index since v1.0.0, so a reader has an id to file against. What the reader lacks is the cut
unit's text and a hash tp computed: `--units` prints floor units only, and the prompt says to supply
`text_sha` yourself over the claim's own text. A reader who guesses the canonicalisation wrongly
records a row whose hash matches nothing. The measurement is in the sidecar under *§2 at HEAD*.

**The decision: `--units` lists cut units too**, each with its id, its `text_sha` under the same
canonicalisation the floor uses, and a marker saying it is cut. The prompt's instruction for a cut
unit names `--units` as where its hash comes from, in place of the instruction to compute one.

**The consequence stated, because three shipped places say otherwise.** `README.md`, `REFERENCE.md`
and a shipped test state that `floor_size` is the line count of `--units`. After this it is the
count of the lines not marked cut, and all three change with this release.

**Alternative not taken: an opt-in flag** keeps the old line count and costs the reader who most
needs cut text a second call. The text least visible to the floor is the text a grader most often
has to see — the arms keep a section's reasoning and drop the wording it produces — so it is listed
by default.

**What this does NOT do.** It does not put cut text in the floor or in the prompt's index, does not
make a cut unit owed a disposition, and does not change coverage or `--check`. The arms decide what
a round must answer; this decides what a reader can name.

## 3. The splitter does not end a sentence inside one

Three rules, each an amendment to `spec/1.0.0.md` §2.1 steps 3 and 4.

1. **A sentence does not end at an ordinal whose next word starts with a lowercase letter.** An
   ordinal here is a number standing as a word of its own followed by `.` (`6.`, not `v1.0.`). The
   test is on the next rune, not the next byte, so a lowercase `ş` counts exactly as `s` does. A
   number that ends a sentence before a capitalised one still splits (`… exits 1. The next …`),
   which is also the shape `spec/1.0.0.md` row 20 pins. The rule is narrowed to lowercase because
   the unnarrowed form — never split after `N.` — joins every sentence that ends in a number to the
   sentence after it.
2. **A numbered list directly under a lead-in line ending in `:` opens the marker gate.** The list
   is then segmented exactly as the same list with a blank line above it. The narrowing to `:` is
   what keeps the rule off hard-wrapped prose, where a sentence-final number can start a line with
   no list anywhere.
3. **A fragment whose whole text is an ordered-list marker is not a unit.** The test is the marker
   shape, never a length: a length floor says nothing about what it means and would start dropping
   real units the day someone writes a shorter one. This is what reaches the interleaved residue
   `spec/1.0.0.md` §2.1 step 3 declined to repair — a list whose items are separated by indented
   continuation paragraphs, so the block holding item N opens with prose. That section declined
   because the only repair it considered, stripping every line's marker, deletes text. This one
   deletes none: the fragment holds nothing but the marker.

**Prototype first — the first task, before this spec's first grading round.** Rule 2 is run over
this repository's `spec/` with the control that `… exit 1. The …` keeps splitting, and the result
decides whether rule 2 ships as written, ships narrowed, or leaves the release while rules 1 and 3
ship alone. A first run, made while this file was written, is in the sidecar under *The splitter
census*, with the lead-in shape it did not reach; the task decides that shape.

## 4. Moved

The two zeros, the `cut` envelope key and the `--status` cut delta are
`a-round-can-be-driven-from-the-envelope` §2.4 and §2.5.

## 5. Non-Goals

1. **No change to the floor's arms.** §3 changes which fragments reach them; what they keep is
   decided exactly as before, by the same three tests.
2. **No new workflow field, no gate, no convergence effect, no exit code.** Nothing here adds a
   knob to `.tp/config.json` or a task file's `workflow` block, and nothing changes `clean`, a
   streak, coverage or `--check` — the `--check` gate is `next-action-and-check-tell-the-truth`'s.
3. **Rule 2 names numbered lists only.** The defect is a split, and among list markers only a
   number ends in a terminator.
4. **The first-line gate stays.** Stripping every line's marker unconditionally deletes text, which
   is why `spec/1.0.0.md` §2.1 step 3 gates on the block's first line; nothing here reopens that.

## 6. Open question

**Should `--record` refuse a row naming a cut unit whose `text_sha` disagrees with the one `--units`
prints?** At HEAD it records such a row at exit 0 as off-floor (sidecar, *§2 at HEAD*); once tp
prints the hash, a refusal is possible. It is not decided here because a cut unit owes nothing, and
a refusal would make a row about text the round did not have to read able to fail the round.
Claim enumeration, the other question this file once carried, was decided on 2026-09-08; the
sidecar records it.

## 7. Tests

Every row derives from a numbered decision and names the mutant that must fail it. Rows 1, 4 and 6
quote their `HEAD` behaviour, because `HEAD` is their mutant; the other rows' subject does not exist
at `HEAD`, so their two counts belong to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 rule 1 | `Adım kuralı: §6.2 ise 6. adımdan sonra çalışır ve sonucu yazar.` and `Step 6. writes the round file before the record runs.` are one unit each. At `HEAD` each is two, and the fragment holding the claim is cut | `HEAD`: split at every `.` followed by whitespace |
| 2 | §3 rule 1 *the controls* | `The gate returns exit 1. The next command reads it.` stays two units; a sentence whose ordinal is followed by `şubeyi` stays one; `TestSection11Row20CanonicalForm` stays green | never split after `N.`, which joins the first pair; or test the next byte against `a`–`z`, which splits the second |
| 3 | §3 rule 1 *an inline list* | `Kural: §5.2 2., 3. ve 4. adımları sırayla çalıştırır.` is one unit | split at `3.` or `4.`, the field report's own second instance |
| 4 | §3 rule 2 | a lead-in ending in `:` followed on the next line by an unpunctuated three-item numbered list yields exactly the units of the same document with a blank line added above the list. At `HEAD` the two differ: the glued form's lead-in unit ends in `1.`, each item's unit ends with the next item's number, and the last item is cut | `HEAD`; or strip every line's marker unconditionally, which also fails row 20 |
| 5 | §3 rule 2 *the wrap* | a hard-wrapped paragraph whose continuation line begins `14. The …`, after a line not ending in `:`, keeps its `HEAD` segmentation | open the gate on any numbered line after any line |
| 6 | §3 rule 3 | the interleaved list `spec/1.0.0.md` row 18e builds produces no unit whose whole text matches `^[0-9]+[.)]$`, and the same other units as before. At `HEAD` it produces the bare markers, which `TestSection11Row18eInterleavedResidue` asserts and this release inverts | `HEAD`; or drop fragments under a fixed byte length, which row 7 fails |
| 7 | §3 rule 3 *not a length rule* | a legitimate two-byte unit that is not a marker (`v2`) survives | any length floor: two bytes is the marker's own length |
| 8 | §2 | `--units` lists a cut unit with its id and the cut marker, and the `text_sha` on its line equals the hash of the text printed beside it | list the cut text with no hash, which puts the reader back to guessing the canonicalisation |
| 9 | §2 *bounded* | on one spec with at least one cut unit, coverage and `--check` answer identically before and after §2 | count cut units toward coverage, which makes every document permanently uncovered |
| 10 | §2 *the prompt* | the emitted prompt's instruction for grading a cut unit names `--units` as the hash's source and no longer tells the reader to compute it | leave the old instruction beside the new listing, so two sources of the hash disagree |

### 7.1 The anchor fixtures — one test task, no decision

`engine.FloorAnchorOf` bills six shapes of unit to an anchor a reader would not predict; the six are
in the sidecar under *The anchor fixtures*, each built and run. **One test task writes all six
down, asserting the anchor the function produces at the implementing commit, and changes no
behaviour.** No decision is attached because a wrong anchor misleads nobody downstream today: the
index prints it, the reader copies it, and `--record` checks its form and nothing else (sidecar).
The fixtures exist so the first release that ships an anchor-keyed output starts from a written
table rather than rediscovering it — a grouping still totals the floor size whichever anchor each
unit gets, so no consumer can find these by checking its own sums. Two of the six are already
asserted exactly by shipped tests — `TestABlockThatStraddlesADroppedHeadingKeepsTheSectionItOpensIn`
and `TestTheAnchorIsTheLastNumberedHeadingAtOrAboveTheUnit` — and the task cites them rather than
duplicating them. The mutant for each new row is a change to the anchor function's heading pattern,
level bound or fence tracking that alters the anchor that row's fixture receives.
