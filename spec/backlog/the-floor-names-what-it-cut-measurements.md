# the-floor-names-what-it-cut — measurements

Supplemental material for `the-floor-names-what-it-cut.md`; the spec stands without it. Figures are
quoted at the ref each block names; re-derive rather than reading one off the page. The first four
sections were written on 2026-09-11 against `18032abe` (`tp version
v1.1.2-0.20260910212136-18032abe405f`), in scratch directories outside the repository; everything
below *Earlier material* was moved here on 2026-09-08 from `ground-command-friction-measurements.md`
and keeps that file's section numbers.

## Field report WB-3155, verified 2026-09-11

### #7 — the floor splitter cuts a sentence at an ordinal, and the claim can be left unowned

**The claim.** In a Turkish spec's round 7, unit `u340` was `"§6.2 ise 6."` — the sentence split at
the ordinal `6.` mid-sentence. The actual claim (that §6.2 requires step 6 to write a key for every
dealer in scope) fell into `u341`, which the arms cut, so no round graded it. The same splitter
produced `"2., 3."` / `"ve 4."` in the report's §5.2, where the claim survived in a third unit by
luck. The report suggested either not treating `\d+\.` as a sentence end, or warning when a cut unit
neighbours a kept one.

**Verdict: CONFIRMED, and wider than reported.** Nothing in the claim was refuted. It is not
specific to Turkish, and it has a second shape the report did not name (the glued list, below).

**Reproduction.** A fixture of one `## 1. A` heading and three paragraphs — the Turkish sentence
`Adım kuralı: §6.2 ise 6. adımdan sonra çalışır ve sonucu yazar.`, the English control
`Step 6. writes the round file before the record runs.`, and the true sentence end
`The gate returns exit 1. The next command reads it.` — emitted with `tp ground ord.md` in a fresh
`git init` directory. The floor index read:

```
u1 §1 ca307edd5e1e #1 27B
u2 §1 (cut)
u3 §1 e532be183921 #1 7B
u4 §1 (cut)
u5 §1 239263023127 #1 24B
u6 §1 (cut)
# 3 in floor, 3 cut
```

`tp ground ord.md --units` printed only the kept halves: `Adım kuralı: §6.2 ise 6.`, `Step 6.`,
`The gate returns exit 1.` The cut units, read through the engine, were
`adımdan sonra çalışır ve sonucu yazar.`, `writes the round file before the record runs.` and
`The next command reads it.` — the first two are the claims; the third is a correct split.

The report's second instance, rebuilt as `Kural: §5.2 2., 3. ve 4. adımları sırayla çalıştırır.`,
reproduces exactly: `Kural: §5.2 2., 3.` (kept) | `ve 4.` (kept) | `adımları sırayla çalıştırır.`
(cut). A sentence whose ordinal is followed by a non-ASCII lowercase letter
(`Kural: §5.3 ise 7. şubeyi atlar.`) splits the same way, which is why the spec's rule 1 tests the
next rune and not the next byte.

**Source.** `internal/engine/floor.go:222-248` (`floorSplitUnits`: every `.`, `!` or `?` followed
by whitespace ends a unit), `:352-354` (`inFloor`: the digit, code-span and measurement-verb arms —
the ordinal half carries the digit, a Turkish claim carries none of the English verbs), `:200` (the
first-line marker gate).

**The report's second suggestion was not taken.** Warning on a cut unit beside a kept one reports
the split; rule 1 removes it for the reported shape. **This file's old §3 would not have fixed
#7**: dropping bare-marker fragments leaves `Adım kuralı: §6.2 ise 6.` exactly where it is.

## The splitter census

### Counting rule and script

**Counting rule.** The files are `git ls-files 'spec/*.md' 'spec/**/*.md'` minus every path under a
`.tp-review/` directory (round snapshots are copies of specs, and counting them counts one list once
per round). A *glued marker* is a line matching `^\s*\d+\.\s+` inside a prose block whose first line
is not a list marker, after the blockquote strip — so the first-line gate is closed and the marker
survives into the joined text. The *item after it* is the unit that starts with the item's first
24 characters. It uses `scripts/floor-prototype.py`'s derivation, and at `18032abe` it agrees with
the Go engine on every figure it prints (checked with a probe calling `engine.FloorUnits` and
`engine.FloorIndexRows` in a clone). Run it from the root of a worktree of `18032abe`
(`git worktree add <dir> 18032abe`): on any tree holding this spec, the Turkish and English
fixtures quoted in it and in this file are themselves in the corpus, and
`ordinal_split_next_lower` counts them (the value is not stated, for §5.1's reason below).

```
python3 - <<'EOF'
import importlib.util, re, subprocess, sys
sys.dont_write_bytecode = True  # a measurement writes nothing into the tree it measures
spec = importlib.util.spec_from_file_location("fp", "scripts/floor-prototype.py")
fp = importlib.util.module_from_spec(spec); spec.loader.exec_module(fp)
NUM, BARE = re.compile(r"^\s*\d+\.\s+"), re.compile(r"^[0-9]+[.)]$")
ORD_END = re.compile(r"(?:^|\s)\d+\.$")
files = [f for f in subprocess.run(["git", "ls-files", "spec/*.md", "spec/**/*.md"],
         capture_output=True, text=True).stdout.split() if "/.tp-review/" not in f]
glued = colon = item_cut = ord_lower = bare = 0
for f in files:
    for b in fp.blocks(open(f, encoding="utf-8").read()):
        if b[0].startswith(fp.TABLE_MARK):
            continue
        units = fp.units_from_block(b)
        bare += sum(1 for u in units if BARE.match(u))
        for left, right in zip(units, units[1:]):
            if ORD_END.search(left) and not BARE.match(left):
                ord_lower += right[0].islower()
        lines = [fp.BLOCKQUOTE.sub("", ln) for ln in b]
        if fp.LIST_MARKER.match(lines[0]):
            continue  # the first-line gate is open: every marker is stripped
        opened_by_colon = None
        for k in range(1, len(lines)):
            if not NUM.match(lines[k]):
                continue
            glued += 1
            if opened_by_colon is None:
                opened_by_colon = lines[k - 1].strip().endswith(":")
            colon += opened_by_colon
            head = re.sub(r"\s+", " ", NUM.sub("", lines[k])).strip()[:24]
            item = next(u for u in units if u.startswith(head))
            item_cut += not fp.in_floor(item)
print(f"files={len(files)} glued_markers={glued} in_a_list_opened_after_colon={colon} "
      f"item_after_marker_cut={item_cut} bare_marker_units={bare} "
      f"ordinal_split_next_lower={ord_lower}")
EOF
```

At `18032abe` it prints:

```
files=109 glued_markers=49 in_a_list_opened_after_colon=34 item_after_marker_cut=10 bare_marker_units=11 ordinal_split_next_lower=0
```

### Read one by one

**Every glued marker read one by one, all 49.** 37 are in nine real lists written directly under a
lead-in line — `spec/0.1.0.md` (three lists), `spec/0.13.0-review-perspectives.md` (two),
`spec/0.14.0-code-aware-review.md`, `spec/0.19.0-agent-friction.md`,
`spec/0.20.0-review-state.md`, `spec/0.21.0-skill-interview.md`. Eight of the nine lead-ins end in
`:`; the ninth, `spec/0.1.0.md:507`, ends in `:**` (`**Behavior priority:**`), which rule 2 as
written does not reach. 9 are the interleaved shape `spec/1.0.0.md` §2.1 step 3 names — items
separated by indented continuation paragraphs, in `spec/0.36.0.md` (items 5–9) and `spec/1.0.0.md`
(§2.1's own steps 2–5). 3 are not lists at all: a sentence ending in a number, wrapped so the
number starts a line — `spec/1.0.0.md:641` (`by test` / `14. The mapping`),
`spec/1.0.1-measurements.md:430` (`review round` / `1. Two of them`) and
`spec/backlog/emitting-does-not-lose-a-round-measurements.md:54` (`instead of` / `44. The cost`).
None of the three follows a `:`, and each is followed by a capital, so neither new rule touches them.

**The ten cut items, read one by one.** Seven are items of the colon-led glued lists that the arms
cut on their own text once no marker digit is attached — the last item of six lists
(`spec/0.1.0.md:604`, `:1258`, `spec/0.13.0-review-perspectives.md:114`, `:237`,
`spec/0.14.0-code-aware-review.md:168`, `spec/0.20.0-review-state.md:159`) and the first item of
`spec/0.21.0-skill-interview.md:76`, whose items end in full stops. Two are interleaved items whose
text holds no digit (`spec/1.0.0.md:157`, `:210`). One is a correct sentence split
(`emitting-does-not-lose-a-round-measurements.md:54`).

**Rule 1 changes no unit in this corpus** (`ordinal_split_next_lower=0`): the Turkish shape does not
occur in these English specs, and neither does the risk the lowercase narrowing guards against — a
sentence-final number followed by a sentence opening with a lowercase word such as `tp`. It is
verifiable here only on constructed fixtures, which is what the spec's rows 1–3 are.

### The three rules, prototyped

Implemented in a scratch clone of `18032abe` as a variant of the
engine's own block, canonicalise and split steps — rule 1 as "no split after `(^|\s)\d+\.` when the
next rune is `unicode.IsLower`", rule 2 as "a numbered line directly after a line ending in `:`
starts a new block when the current block's first line is not a marker", rule 3 as "drop a unit
matching `^[0-9]+[.)]$`" — and run over the same 109 files:

- Bare-marker units: 11 at `18032abe`; 9 after rules 1 and 2 (the two in
  `spec/0.21.0-skill-interview.md` go, the nine interleaved ones stay); 0 after rule 3.
- Floor units (kept by the arms): 44 disappear and 9 appear, in 8 files. Every one is a bare marker
  (the nine above), a list item or a lead-in: `The prompt contains: 1.`, `Role instruction 2.` …
  become `The prompt contains:` (cut) and one unit holding the whole unpunctuated list, which the
  arms judge on its own text — exactly what the same list with a blank line above it produces at
  `18032abe`. That is the carry cost the spec names: each unit that appears is asked once.
- On the glued three-item fixture of row 4 the units before and after were
  `The emission does three things: 1.` (kept), `It writes the snapshot beside the spec 2.` (kept),
  `It writes the floor file 3.` (kept), `It names the output path for the reader` (cut) →
  `The emission does three things:` (cut) and the three items as one cut unit.

**Not used.** A pair of counts relayed with this finding's verification carried no counting rule,
and neither reproduces under the rule above or under three naive source-level readings tried
beside it; neither is quoted anywhere.

## §2 at HEAD — what a reader can and cannot name

- **Cut units have ids.** `89101f27` (2026-09-03, first tagged in `v1.0.0`) made the index announce
  them: `u2 §1 (cut)` in the fixture above. This file's earlier *"has no id to file against"* was
  out of date from that commit on.
- **The prompt tells the reader to hash it.** The emitted prompt says, verbatim: *"A cut unit HAS an
  id … Either way supply `text_sha` yourself over the claim's own text: the index carries no hash
  for a cut unit."*
- **`--units` omits cut units.** On the fixture above it prints three lines for six units.
- **`--record` checks neither the hash nor the anchor of such a row.** A row naming `u2` with
  `text_sha` `000000000000` recorded at exit 0, reported as `off_floor: 1`. A row naming a floor unit
  with the right hash and `anchor: "§7"` on a document whose only section is §1 also recorded at
  exit 0: `internal/engine/groundrow.go:257-260` checks that the anchor is present and matches the
  `§n(.n)*` form, and production code reads a row's anchor nowhere else (`git grep -n '\.Anchor\b'
  -- 'internal/*.go' ':!*_test.go'` lists the index formatter, the carry's copy, the row's parse and
  write, and those checks).
- **The `unit_id: null` rows.** Over both round globs (`spec/.tp-review/*/ground-round-*.ndjson` and
  `spec/backlog/.tp-review/*/ground-round-*.ndjson`), 43 files carry 15 such rows at `18032abe`. The
  count cannot tell a reader who gave up on a cut unit's id from a genuine reader-added claim, so it
  is not evidence for §2 by itself.
- **Where the `--units` line count is stated as `floor_size`.** `README.md:437`,
  `skills/tp/REFERENCE.md:982`, and `internal/cli/ground_floor_size_test.go:54`.

## The anchor fixtures

Absorbed from `floor-anchors-need-fixtures.md` on 2026-09-11. Each fixture below was built in a
scratch directory and emitted with `tp ground <file>` at `18032abe`; the anchor is read off the
emitted floor index.

| # | fixture | anchor produced | what a reader expects | shipped test |
|---|---|---|---|---|
| 1 | `The tool MUST exit 1.` directly above `## 2. Second`, and `The tool MUST exit 2.` directly below it — no blank line on **either** side | `u2 §1` | `§2` | `TestABlockThatStraddlesADroppedHeadingKeepsTheSectionItOpensIn`, which asserts `§1` as a decision |
| 2 | a unit under `## Unnumbered parent` following a `### 1.1 Child` branch | `§1.1` | the preceding top-level numbered section, `§1` | `TestTheAnchorIsTheLastNumberedHeadingAtOrAboveTheUnit` (`## Motivation` after `### 1.1`) |
| 3 | a document whose numbered headings are `# 1. One` and `# 2. Two` | every unit `§0` | `§1`, `§2` | the level bound is pinned by `TestAnH1IsNotASectionAndAnH4Is`, on a version title; a numbered H1 is in no test |
| 4 | `## 9. Inside` in a ` ``` ` block that is itself inside a `~~~` block | `§9` | no anchor — it is code | none |
| 5 | `## 9. Indented` as an indented code block | `§9` | no anchor | none |
| 6 | `## 2026-09-07 release notes` and `## 0.19.0 — Agent Friction Reduction` | `§2026`, `§0.19.0` | no anchor — neither is a section number | none |

**Fixture 1's description is corrected.** The earlier file said *"no blank line above the heading"*;
with a blank line below the heading the unit anchors to `§2` correctly. The case needs prose flush on
both sides, because a dropped heading is not a block boundary and the anchor is resolved at the line
the block opens on (`internal/engine/floor.go:871-890`).

**All six reproduced.** A relay of this verification said three did; the other three were not built
in that run.

**Two of the mechanisms, as the code states them.** `floorSectionHeadingRe` matches `#{2,6}`
(`floor.go:803`), which is why a numbered H1 opens nothing (fixture 3), and its doc comment gives the
reason: a spec's H1 is routinely a version title. `floorFenceRe` toggles on ` ``` ` or `~~~`
without recording which opened the block (`floor.go:17`, `:821-822`), so a ` ``` ` line closes a
`~~~` fence (fixture 4) — while the scan's comment says *"a heading inside a fence is code"*
(`floor.go:824`). The implementing task narrows that comment to what fixtures 4 and 5 show.

**Why the family's notion of a code block is not reopened** (the absorbed spec's non-goal):
`~~~` and indented blocks are out of scope for every rule in `internal/engine/vague.go` by decision,
and fixtures 4 and 5 are in scope only because of the gap between that comment and the code.

## Earlier material (moved 2026-09-08 from `ground-command-friction-measurements.md`)

The section numbers in these headings are the original file's: its §4 is this spec's §2, its §5 is
this spec's §3 rule 3, its §5.1 was §3.1 (deleted from the body on 2026-09-11 — `skills/tp/SKILL.md`
Step 0.5's *"a number does not live in a spec"* covers it), and its §16 moved to
`a-round-can-be-driven-from-the-envelope.md` §2.4. Filenames inside `git show <ref>:<path>` and
`git log -- <path>` commands are the paths at that ref.

## Header: the count of decisions and why the file did not split

Deleted on 2026-09-11: it was the original file's front matter, counting its decisions and asking
whether it should split, and the 2026-09-08 split answered it. `git show
6f8e8564:spec/backlog/the-floor-names-what-it-cut-measurements.md` has it.

## §4 A cut unit cannot be addressed

**Superseded in part — see *§2 at HEAD* above.** Cut units have carried ids since `89101f27`, so the
*"no id to file against"* below was already out of date when it was written here; what it shows is
that `--units` did not list them.

**Measured.** `--units` lists floor units. A reader who finds a defect in text the floor's arms cut
has no id to file against:

```
$ tp ground spec/backlog/ground-command-friction.md --units | cut -f1 | grep -cE '^u(15|16|17|18|19|38|49|50|55)$'
0
```

Those nine ids are exactly the ones that round filed rows against, having inferred each from the
splitter's merge behaviour and computed its own `text_sha` by guessing the canonicalisation. Where
that reconstruction failed the reader gave up and recorded no id at all — **13 rows across the 28
recorded rounds carry `unit_id: null`** (one glob; *§2 at HEAD* gives the both-globs figure and why
it is not evidence on its own):

```
python3 -c 'import json,glob;print(sum(1 for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip() and json.loads(l).get("unit_id") is None))'
```

**Why this is the most valuable of the four.** Across the programme the sharpest finding of a round
sat in cut text again and again, and one round diagnosed why: the arms index a section's *reasoning*
and drop the *wording it produces* — a new central clause or a user-facing message string is exactly
the shape that gets cut. So the text least visible to the floor is disproportionately the text worth
grading, and it is the text a reader cannot cite.

## §5 A bare ordered-list marker becomes a floor unit

**Since explained — see *The splitter census*.** A marker becomes a bare unit when its line sits in a
block whose first line is not a marker (a list glued under its lead-in, or the interleaved shape)
and the text before it ends in a terminator; the two *"unexplained"* observations below are both
that. The figures below are at the refs they name.

**Measured.** Counting rule — floor units whose whole text matches `^[0-9]+[.)]$`, read from
`tp ground <spec> --units` (TSV: `unit_id`, `text_sha`, `text`), over an **enumerated** set: the
twenty files then named `spec/1.38.0.md` through `spec/1.58.0.md` (now under `spec/backlog/`), plus `spec/1.0.0.md` — twenty-one in all.
**At `9e672387`: 20 such units across 4 specs, out of 2,192 floor units scanned — 0.91%.** They are
`u84 u87 u89 u91` in `spec/backlog/brief-carries-the-forcing-sentences.md`, `u71 u73 u93 u100` in **`spec/1.0.0.md`** — the document that
defines the floor — `u191 u194 u196 u199 u204 u206 u211` in `spec/backlog/what-the-carry-can-promise.md`, and
`u128 u132 u135 u137 u141` in the spec since dropped into `spec/undecided.md` (then `spec/1.58.0.md`).

**The census moved 2.5× and the instance did not, and keeping those apart is what this paragraph is
for.** The figure first written here was *8 units across 2 specs out of 1,990 — 0.40%*, which
reproduces exactly at `22ed1c4a`, the commit that wrote it. **The eight ids it names still reproduce
at `9e672387`**; the twelve new markers all arrived in `spec/backlog/what-the-carry-can-promise.md` and the spec since dropped into `spec/undecided.md` (then `spec/1.57.0.md` and `spec/1.58.0.md`), added
the same day and edited heavily since. So the defect is neither rarer nor commoner than it was — the
corpus grew — and every share here is quoted with the ref it was measured at, because it will move
again before this is read.

**The set is enumerated because an earlier draft's phrase was not.** That draft said *"every pending
spec plus `spec/1.0.0.md`"* and quoted 1,968, and **no reading of that phrase returns it**: at
`22ed1c4a` the rule gives 1,990 for the twenty-one above and 2,064 adding this file. **A third
denominator stood here — adding this file too — and it is deleted rather than refreshed; §5.1
says why, because it was false when it was written.** The named ids reproduce under every reading;
only the denominator moved, which is the half a stated counting rule exists to pin.

**The marker is not a line in the source — the splitter makes it.** No line in either file matches
`^\s*[0-9]+[.)]\s*$` (`0` hits in both). The items are single lines of the form
`N. **Bold phrase.** text…`, and the split happens *inside* the line.

**Two things were unexplained at the time.** In `spec/backlog/brief-carries-the-forcing-sentences.md` the
items are numbered 1–5 and only `2.` through `5.` become units. In `spec/1.0.0.md`, 16 lines match
`^\s*[0-9]+[.)]\s+\*\*` and only 4 produce a marker unit. An earlier draft of this entry asserted a
mechanism — items whose bold phrase sits on the *following* line — and one command refuted it.

**The extremes a length floor would sit between.** The shortest legitimate
non-marker floor unit across the same corpus is **9 bytes** (`Measured:`, `spec/backlog/red-gate-procedure.md` `u49`, at
`9e672387`; the next shortest are 10) against the markers' 2, so any threshold in 3–8 separates
them.

## §5.1 A figure about this file's own floor does not belong in this file

**Two sentences in this release quoted this file's own floor size, and both were false when
they were written, for the same reason.** §5's triple `1,990 / 2,064 / 2,275` holds at `2c39f718`.
`git show 22ed1c4a -- spec/1.0.1.md` shows the sentence was **written** four commits after that,
where the same rule returns `1,990 / 2,064 / 2,295` — so `2,275` was already false as it was typed.
(At `9e672387` the first two are `2,192` and `2,266`; the third is not stated, for this
subsection's own reason.) It is `2,064 + 211`: this file's floor as it stood four commits earlier,
carried forward unrefreshed. And §10.3's *"it skips this file at 211"* is **that same stale
211**, in a sentence whose entire job is correcting an earlier unreproducible figure; the floor was
**247** at `339368c0`, where that correction was written, and **257** at `9e672387`. Walk it rather
than reading a trace off this page — `tp ground <this file> --units | wc -l`, run in a worktree of
each commit `git log --format=%h -- spec/1.0.1.md` names, returns a different value at nearly every
one of them.

**The decision it produced: no figure about a file's own floor appears in that file.** Writing the
sentence changes the quantity the sentence measures, so no ref can pin it; the sound forms are the
derivation with no value, or nothing. The rule now lives in `skills/tp/SKILL.md` Step 0.5 as *"a
number does not live in a spec"*, which is why the body's §3.1 was deleted on 2026-09-11; this
subsection is kept as the measurement that produced it.

## §8 Tests — the mutant-column prose trimmed from rows 8 and 9 (later rows 3 and 4, now rows 6 and 7)

Row 8: drop every fragment shorter than **10** bytes, which also drops `Measured:` — the 9-byte legitimate unit §5 names. That is the only length floor this row kills, and it is the least tempting one: over §5's own enumerated twenty-one files at `9e672387`, markers are 2 bytes and the shortest non-marker floor unit is `Measured:` at 9 (`spec/backlog/red-gate-procedure.md` `u49`; the next shortest are 10), so every threshold in §5's own blessed 3–8 range survives this row. **An earlier draft wrote *probed over 527 real floor units*, and that number is deleted rather than refreshed: it carries no counting rule and no set returns it** — the twenty-one files hold 2,192 units, 2,172 of them non-marker, `spec/1.0.0.md` alone 401, and units of 60 bytes or fewer 173. The argument never needed a denominator, only the two extremes, and both re-derive. Row 9 is what kills those.

Row 9: implement §5 as a length floor of any kind. Two bytes is the marker's own length, so no threshold that drops the markers can keep this unit under either reading of the comparison. An earlier draft asserted a **3**-byte unit and does not kill them: under *drop when length < T* — the reading §5's 3–8 range makes natural — **T = 3 satisfies rows 8 and 9 together**, dropping every 2-byte marker, keeping `Measured:` and keeping the 3-byte unit. That is a length floor, which is the rule §5 argues hardest against, and it survived the table until this round. **The input this row named was `Go` and it cannot be built as a floor unit** — `printf '# R\n\nThe count is 12 things. Go\n' > c.md; tp ground c.md --units` prints only `u1`, because a bare `Go` fails all three of §2.1's arms; and read as the code span it is 4 bytes, which survives `T = 3` and collapses this row's own argument. It *is* a real 2-byte fragment one level up, at the splitter, pre-`inFloor`, which is where §5's decision operates — so the two readings disagree about it and the row is only buildable under one of them. **`v2` is buildable under both**: 2 bytes, and it reaches the floor on the digit arm. The same fixture with `v2` in place of `Go` prints `u1` and `u2 fb04dcb6970e v2` (built and run at `9e672387`).

## Implementation notes from the original body

Moved on 2026-09-11 to `a-round-can-be-driven-from-the-envelope-measurements.md`, same heading,
with the section they belong to.

## The two zeros (from the absorbed spec)

Moved on 2026-09-11 to `a-round-can-be-driven-from-the-envelope-measurements.md`, same heading and
subsections, with the body section they back.

## Decided at the 2026-09-08 decision pass

Two entries of `spec/undecided.md` were decided onto the original spec, and both concern this one.

**From *A sentence rewritten in answer to a finding is exempt from the cut for one round*.** Decided:
**no exemption.** `tp ground --status` reports a `cut` **delta** per round — units cut that were
rewritten since the previous round — so a repair is visible once without being graded twice. Since
2026-09-11 the decision is carried by `a-round-can-be-driven-from-the-envelope.md` §2.5, and its
sidecar repeats this entry under the same heading.

**From *Claim enumeration in the grounding floor*.** Decided: **the floor's own arms define a claim.**
Intuition counts — 11 where a spec carried 17, and 10 where another carried 17 again after a second
read — are not a measurement and are retired. The one measured leftover is §5 above, a bare
ordered-list marker becoming a floor unit, and it stays there as a defect of the splitter rather
than as evidence about what a claim is.
