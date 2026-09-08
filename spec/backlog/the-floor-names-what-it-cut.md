# tp — The floor names what it cut

Class: tool

## 1. Overview

What the floor is made of, and what it will let a reader say about the text it did not keep. Four
decisions that share one subject: **a cut unit cannot be addressed** (§2), **a bare ordered-list
marker becomes a floor unit** (§3), **a figure about a file's own floor does not belong in that file**
(§3.1), and **the two zeros** — an empty floor for two different reasons, reported identically (§4).
Split out of `ground-command-friction.md` on 2026-09-08, where these were §4, §5, §5.1 and §16, with
the non-goals and the four test rows that belong to them; §4 arrived there from an absorbed
two-zeros spec, which is why its own measurements, non-goals and test rows sit in the measurements
file rather than here. §2 is the most valuable of the four defects one orchestrator measured against
this repository's own specs, and one of the two an independent field report confirmed. Every claim
below was run rather than reasoned about; the measurements and the commands that derive them are in
`the-floor-names-what-it-cut-measurements.md`, and no figure from that file is restated here. The
grounding programme and the two field reports are described in
`what-the-record-does-not-say-measurements.md` under *§1 The evidence base* and *§1.1 The second
evidence base*.

## 2. A cut unit cannot be addressed

**Measured.** `--units` lists floor units only, so a reader who finds a defect in text the floor's arms
cut has no id to file against, infers one from the splitter's merge behaviour, and computes a
`text_sha` by guessing the canonicalisation; where that fails the reader records no id at all, and the
recorded corpus carries rows with `unit_id: null` for exactly that reason (measurements file, "§4").
The text least visible to the floor is disproportionately the text worth grading — the arms index a
section's *reasoning* and drop the *wording it produces* — and it is the text a reader cannot cite.

**The decision: `--units` lists cut spans too**, each with an id, its `text_sha` under the same
canonicalisation the floor uses, and a marker saying it is cut. A reader then files against a real id
and a hash tp computed, and a wrong join becomes impossible rather than repairable.

**What this does NOT do, so it is not read as more than it is.** It does not put cut text in the
floor, does not make a cut unit owed a disposition, and does not change coverage or `--check`. The
floor's arms decide what a round *must* answer; this decides what a reader can *name*. Whether the
arms cut the right things is a different question and is not taken here.

## 3. A bare ordered-list marker becomes a floor unit

**Measured.** Floor units whose whole text matches `^[0-9]+[.)]$` exist across this repository's
specs, `spec/1.0.0.md` — the document that defines the floor — among them; the counting rule, the
enumerated set and the ids are in the measurements file's "§5". **The marker is not a line in the
source — the splitter makes it.** No line in the affected files matches `^\s*[0-9]+[.)]\s*$`; the
items are single lines of the form `N. **Bold phrase.** text…`, and the split happens *inside* the
line. Which items of that shape produce a marker unit is not explained and is left unexplained rather
than guessed: the shape is necessary and not sufficient, and an earlier mechanism asserted for it was
refuted by one command.

**The decision: the splitter drops a fragment whose whole text is an ordered-list marker.** The test
is the marker shape, not a length floor. A length floor would separate the markers from every
legitimate unit in today's corpus — the measurements file gives the two extremes — and it is still the
wrong rule, because it says nothing about what it means and would silently start dropping real units
the day someone writes a shorter one.

### 3.1 A figure about this file's own floor does not belong in this file

**The decision: no figure about this file's own floor appears in this file.** Writing the sentence
changes the quantity the sentence measures, so no ref can pin it; the rule now lives in
`skills/tp/SKILL.md` Step 0.5, and the two sentences of the original file that were false as they were
typed are the measurements file's "§5.1".

## 4. The two zeros

`tp ground` emits a floor of zero for two different reasons and says the same thing about both: a
document of headings and fenced blocks produces no unit at all, and a document whose every sentence the
arms dropped produces units and keeps none. `GroundStatus.Cut` and `--status --check` already separate
the two; the ask and the emission envelope — the two surfaces an operator meets first — do not. The
absorbed spec's measurements, non-goals and test rows are the measurements file's "The two zeros (from
the absorbed spec)". Three tasks, taken whole:

1. **The ask branches on `cut` and states no count.** The zero-floor ask becomes two literals that
   keep the opening clause `This round owes no dispositions:` — one saying §2.1 produced no unit, one
   saying it produced units and the arms cut every one. Neither literal states the cut count: the
   index block above it already does, and a count would force a `cut == 1` declension into a sentence
   that must not gain a third form.
2. **The emission envelope carries `cut`.** An integer `cut` key, present at zero rather than omitted,
   populated from the emitted index by the rule the floor size is read by; on a partly-cut floor it is
   the index's cut count beside a non-zero `floor_size`. `--status --check`'s `cut` stays sourced from
   `GroundStatus` alone.
3. **The ask tests become a generated `(floorSize, carried)` table**, with a `require` that the
   generated list holds a pair satisfying `2 ≤ carried < floorSize`, and each pair asserting the
   sentence from `This round owes` through the end of its clause rather than a prefix of it.

The sentences of these three that named a function, a parameter or an arm of a switch are in the
measurements file under *Implementation notes from the original body*; what stands here is what an
operator can observe.

## 5. Non-Goals

1. **No change to the floor's *arms*.** The arms are §2.2's cut step of the ground spec —
   `internal/cli/ground.go` prints *"§2.1 produced %d units and the arms cut every one"* and
   `groundcarry.go` has *"The absence of the hash is the cut (§2.2)"* — and nothing here touches it:
   §2 makes what they cut *nameable* without moving what they cut. §3 removes two-byte list markers
   from the floor — *fragments*, not sentences — so the non-goal stands under tp's own vocabulary.
2. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.
3. **The absorbed spec's own fences hold for §4** and are quoted in the measurements file under *What
   the absorbed spec fenced out*: no third sentence in the ask, no gate change, no exit-code change,
   no refusal, and the empty-payload hint is not repaired here.

## 6. Open questions inherited when `spec/candidates.md` was split

Claim enumeration — the weakest step of the grounding protocol, of which §3 is one small measured
piece — was decided on 2026-09-08: the floor's own arms define a claim, and intuition counts are not a
measurement. `spec/undecided.md` records it under *Decided — routed to a pending spec* and this file's
measurements record it under *Decided at the 2026-09-08 decision pass*. No open question remains here.

## 7. Tests

Every row derives from a numbered decision and names an input that must fail it. The mutant-column
reasoning trimmed from rows 3 and 4 is in the measurements file's "§8". §4's ten rows arrived with
the absorbed two-zeros spec and stayed in the measurements file, under *The absorbed spec's tests*;
they are that section's rows and are not restated here.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | `--units` lists an id for a span the floor cut, and the `text_sha` on that line equals the hash of the text printed beside it | list the cut span with no hash, which puts the reader back to guessing the canonicalisation — the exact failure the `unit_id: null` rows show |
| 2 | §2 *bounded* | a cut unit is **not** owed a disposition: coverage and `--check` answer identically before and after §2, on one spec with at least one cut span | count cut spans toward coverage, which makes every document permanently uncovered |
| 3 | §3 | a document with a `N. **Bold.** text` list produces no floor unit whose whole text matches `^[0-9]+[.)]$`, **and** produces the same number of other units as before | drop every fragment shorter than a fixed byte count, which also drops the shortest legitimate unit the measurements file names |
| 4 | §3 *not a length rule* | a legitimate **2-byte** unit that is not a marker (`v2`) survives | implement §3 as a length floor of any kind — two bytes is the marker's own length, so no threshold that drops the markers keeps this unit |
