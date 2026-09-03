package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// §2.1 step 1's drop list, as it is applied. `scripts/floor-prototype.py` is
// the run these are ported from; the ordering below is part of the rule, not an
// implementation detail, because a fenced line that looks like a table row is
// code and a table separator row is not a horizontal rule.
var (
	floorFenceRe      = regexp.MustCompile("^\\s*(?:```|~~~)")
	floorTableRowRe   = regexp.MustCompile(`^\s*\|`)
	floorTableSepRe   = regexp.MustCompile(`^\s*\|[\s:|-]+\|\s*$`)
	floorAtxHeadingRe = regexp.MustCompile(`^\s*#`)
	floorWhitespaceRe = regexp.MustCompile(`\s+`)
	floorBlockquoteRe = regexp.MustCompile(`^\s*>\s?`)
	floorListMarkerRe = regexp.MustCompile(`^\s*(?:[-*+]\s+|\d+\.\s+)`)
)

// floorBlock is one block of §2.1 step 2: the lines that survived step 1's
// drop, grouped between blank lines. Lines are kept unjoined because step 3
// canonicalises them line by line before joining.
//
// IsTableRow marks the one block a later step must not treat as prose. A table
// data row is one unit however many full stops its cells hold (§2.1 step 1), so
// it is blocked on its own here and must never reach step 4's sentence split.
// The prototype carried the same fact as a sentinel string prefixed to the
// line, which cost it a bug — every table row anchored to §0 because the
// sentinel matched no line in the file — and a struct field has no such reach.
//
// Line is the 1-based line the block's first surviving line sits on, and it is
// where §7.3's anchor is resolved. It is carried here rather than recovered
// afterwards because recovering it is the prototype's recorded bug.
type floorBlock struct {
	Lines      []string
	IsTableRow bool
	Line       int
}

// isFloorHorizontalRule reports whether a line is a horizontal rule: three or
// more of `-`, `*` or `_` and nothing else but surrounding whitespace. It is a
// function rather than a pattern because the rule is "the same marker repeated"
// and Go's regexp has no backreference; `-*-` is not a rule, and neither is a
// two-character run.
func isFloorHorizontalRule(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 3 {
		return false
	}
	if s[0] != '-' && s[0] != '*' && s[0] != '_' {
		return false
	}
	return strings.Count(s, s[:1]) == len(s)
}

// floorBlocks is §2.1 steps 1 and 2: drop fenced blocks, ATX headings,
// horizontal rules and table separator rows, then split what remains into
// blank-line-separated blocks, with each table data row its own block.
//
// It takes the spec's TEXT. Nothing here opens a file, which is what lets a
// test of the derivation state its whole input.
//
// Dropping is not splitting: a heading between two prose lines that carry no
// blank line leaves those lines in one block, because step 2 splits on blank
// lines and the heading is gone by the time it runs.
func floorBlocks(text string) []floorBlock {
	blocks := make([]floorBlock, 0)
	var current []string
	currentLine := 0
	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, floorBlock{Lines: current, Line: currentLine})
			current = nil
		}
	}

	inFence := false
	for i, line := range strings.Split(text, "\n") {
		lineNo := i + 1
		switch {
		case floorFenceRe.MatchString(line):
			inFence = !inFence
		case inFence:
			// dropped with its fence
		case strings.TrimSpace(line) == "":
			flush()
		case floorTableRowRe.MatchString(line):
			if !floorTableSepRe.MatchString(line) {
				flush()
				blocks = append(blocks, floorBlock{
					Lines: []string{line}, IsTableRow: true, Line: lineNo,
				})
			}
		case floorAtxHeadingRe.MatchString(line) || isFloorHorizontalRule(line):
			// dropped
		default:
			if len(current) == 0 {
				currentLine = lineNo
			}
			current = append(current, line)
		}
	}
	flush()
	return blocks
}

// floorTableCells splits a table data row's body at each pipe that is not
// escaped, and returns the cells with their escapes resolved.
//
// It is a hand-written scan because the rule is "a pipe not preceded by a
// backslash" and Go's regexp has no lookbehind. The escape is dropped for a
// pipe — the backslash was there to keep the pipe out of this split, and §2.1
// wants the pipe as content — and kept for anything else, so a cell holding a
// path escape is not silently rewritten.
func floorTableCells(body string) []string {
	cells := make([]string, 0)
	var cur strings.Builder
	escaped := false
	for _, r := range body {
		switch {
		case escaped:
			if r != '|' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		cur.WriteRune('\\')
	}
	return append(cells, cur.String())
}

// floorTableRowUnit is §2.1 step 1's second paragraph: strip the outer pipes,
// join the cells with an em dash, collapse whitespace. Empty cells contribute
// nothing, so `| a || b |` reads "a — b" rather than carrying a gap.
//
// The trailing pipe is stripped only when it is not itself escaped. The
// prototype cut it with `\|$`, which on a row ending in an escaped pipe eats the
// pipe and leaves the bare backslash in the unit; §2.1's rule is that an escaped
// pipe is content, and this is the one input where the two readings differ.
func floorTableRowUnit(line string) string {
	body := strings.TrimPrefix(strings.TrimSpace(line), "|")
	if strings.HasSuffix(body, "|") && !strings.HasSuffix(body, `\|`) {
		body = body[:len(body)-1]
	}
	kept := make([]string, 0)
	for _, c := range floorTableCells(body) {
		if c = strings.TrimSpace(c); c != "" {
			kept = append(kept, c)
		}
	}
	return strings.TrimSpace(floorWhitespaceRe.ReplaceAllString(strings.Join(kept, " — "), " "))
}

// floorCanonicalise is §2.1 step 3 for a prose block: strip a leading `> ` from
// every line, strip a leading list marker from every line only when the block's
// first line opens a list, then join the block's lines with a single space and
// collapse whitespace runs to one.
//
// Both halves of the marker rule are repairs and they fail in opposite
// directions. Stripping only the first line leaves each later item's marker
// embedded, and step 4 then emits the bare strings `2.` through `9.` as units.
// Stripping every line unconditionally deletes text: hard-wrapped prose puts an
// ordinal at the start of a continuation line, and the unit silently loses it —
// same sentence, different `text_sha`, decided by where the line breaks fell.
//
// The collapse is not cosmetic. `text_sha` (§7.2) is the sha256 of exactly this
// string, so a surviving double space is a hash tp and a reader compute
// differently — and one is produced by ordinary markdown, where a `>` alone on a
// line canonicalises to nothing and leaves a gap where it was.
//
// The blockquote prefix is stripped per line rather than per block because this
// repository hard-wraps quoted prose, so every line of a quote carries the
// marker and a first-line-only strip would leave the rest inside the unit. The
// pattern is anchored, so a `>` anywhere but the start of a line is content, and
// a nested quote loses one level per pass exactly as the prototype's does.
func floorCanonicalise(lines []string) string {
	stripped := make([]string, 0, len(lines))
	for _, ln := range lines {
		stripped = append(stripped, floorBlockquoteRe.ReplaceAllString(ln, ""))
	}
	// The gate: a real list opens with a marker, so the block's FIRST line
	// decides for all of them. Stripping every line unconditionally deletes
	// text, and stripping only the first leaves the rest embedded.
	if len(stripped) > 0 && floorListMarkerRe.MatchString(stripped[0]) {
		for i, ln := range stripped {
			stripped[i] = floorListMarkerRe.ReplaceAllString(ln, "")
		}
	}
	joined := floorWhitespaceRe.ReplaceAllString(strings.Join(stripped, " "), " ")
	return strings.TrimSpace(joined)
}

// floorSplitUnits is §2.1 steps 4 and 5: split the canonical block at each `.`,
// `!` or `?` followed by whitespace, keep the terminator with the unit on its
// left, trim, and drop the empties.
//
// It is a hand-written scan rather than a regexp because the rule is stated from
// the terminator's side — Python's `(?<=[.!?])\s+`, which Go's regexp cannot
// express — and because the terminator must survive the cut. Splitting on the
// whitespace alone would either consume the terminator or need a capture, and
// both readings of that were live long enough for the spec to record the repair:
// they agree on the segmentation and disagree on every `text_sha`.
//
// Bytes are safe to compare here: every terminator and every space is ASCII, and
// no ASCII byte occurs inside a multi-byte UTF-8 sequence.
func floorSplitUnits(joined string) []string {
	units := make([]string, 0)
	keep := func(s string) {
		if u := strings.TrimSpace(s); u != "" {
			units = append(units, u)
		}
	}
	start := 0
	for i := 0; i < len(joined); i++ {
		if c := joined[i]; c != '.' && c != '!' && c != '?' {
			continue
		}
		// The whitespace run belongs to neither unit: it is the separator.
		j := i + 1
		for j < len(joined) && isFloorSpaceByte(joined[j]) {
			j++
		}
		if j == i+1 {
			continue // no whitespace follows, so this terminator does not split
		}
		keep(joined[start : i+1])
		start = j
		i = j - 1
	}
	keep(joined[start:])
	return units
}

// isFloorSpaceByte reports whether one byte is whitespace for §2.1 step 4's
// "followed by whitespace".
func isFloorSpaceByte(c byte) bool {
	return strings.IndexByte(" \t\n\v\f\r", c) >= 0
}

// floorUnitsFromBlock is the seam §2.1 steps 3 to 5 fill in — canonicalising a
// prose block, splitting it into sentences, and joining a table row's cells with
// an em dash. It exists so both kinds of block reach the later steps through one
// place and cannot diverge, and it is where the one decision steps 1 and 2
// already made is honoured: a table data row is exactly one unit however many
// full stops its cells hold, so it returns here and never reaches step 4's
// sentence split.
func floorUnitsFromBlock(b floorBlock) []string {
	if b.IsTableRow {
		if u := floorTableRowUnit(strings.Join(b.Lines, " ")); u != "" {
			return []string{u}
		}
		return nil
	}
	return floorSplitUnits(floorCanonicalise(b.Lines))
}

// FloorUnits derives §2.1's candidate units from a spec's text.
//
// The argument is the spec's TEXT, never a path: a figure or a test over a
// corpus is a standing tax that goes red when someone adds a spec, so the
// derivation is a pure function of a string and its tests state their whole
// input. Passing a path yields a one-line document whose single unit is the
// path.
func FloorUnits(text string) []string {
	units := make([]string, 0)
	for _, b := range floorBlocks(text) {
		units = append(units, floorUnitsFromBlock(b)...)
	}
	return units
}

// floorVerbs is §2.1's third arm, the twelve measurement verbs verbatim from
// the spec's table. The list is stated once and the pattern is built from it,
// so a verb cannot be present in one and absent from the other.
var floorVerbs = []string{
	"measured", "ran", "counted", "derived", "reproduced", "observed",
	"verified", "asserted", "recorded", "fired", "held", "refuted",
}

var (
	// The digit arm. §2.1 names the character range `[0-9]`, and RE2's `\d` IS
	// that range — ASCII only, where a Unicode class would need `\p{Nd}`. The
	// spelling is `\d` because gocritic's regexpSimplify rejects the longhand;
	// the meaning is the spec's, and zero is one of the ten (floor_test.go's
	// "the only digit is zero" case exists because every other fixture reaches
	// this arm through 1-9, so `[1-9]` passed the whole table).
	floorDigitRe = regexp.MustCompile(`\d`)
	// The identifier arm. §2.1 says a backtick-delimited SPAN, so both
	// delimiters must be present in the same unit;
	// `scripts/floor-prototype.py` tested for a single backtick until 60dbd4c8,
	// and every unit the two readings disagreed on was the same shape — a span
	// the sentence split cut in half. The decision is pinned by
	// TestTheIdentifierArmNeedsADelimitedSpan, which asserts both readings on
	// one sentence differing only in its closing backtick and carries the
	// corpus's own instance from `spec/0.12.0-review-rounds.md` verbatim. The
	// count over `spec/*.md` that stood here is deleted rather than refreshed:
	// §2.1 rules a figure over a glob worse than an unpinned one, because the
	// glob's size moves under it when a release adds a spec.
	floorCodeSpanRe = regexp.MustCompile("`[^`]*`")
	// The verb arm. Whole words, because "Transition", "branches" and
	// "withheld" each contain a listed verb as a substring. Whole-word matching
	// is strictly narrower than substring matching, so a unit the two readings
	// disagree on is always a false positive for the substring reading;
	// TestTheVerbArmMatchesWholeWordsOnly carries five and asserts the positives
	// beside them, so a rule that stopped matching altogether cannot pass it.
	// Case-insensitive, because a claim's verb is routinely its sentence's first
	// word: `spec/0.36.0.md`'s "Measured while implementing: it can." reaches the
	// floor through the fold and through nothing else.
	//
	// Both counts over `spec/*.md` that stood here are deleted for the reason
	// above, and the first was ambiguous as well as unpinned. Three readings of
	// "the substring reading and this one disagree on N units" — drop the word
	// boundary alone, drop the case fold with it, or count only the units whose
	// FLOOR verdict flips — give three different N at one commit, none of them
	// the 254 that stood here. The middle reading also disagrees in BOTH
	// directions, since a case-sensitive substring misses a sentence-initial
	// "Measured", so it cannot carry the false-positive claim above at all. A
	// figure whose counting rule is unstated is not reproducible even by someone
	// holding the same corpus.
	floorVerbRe = regexp.MustCompile(`(?i)\b(?:` + strings.Join(floorVerbs, "|") + `)\b`)
)

// floorHasDigit is §2.1's first arm.
func floorHasDigit(unit string) bool { return floorDigitRe.MatchString(unit) }

// floorHasCodeSpan is §2.1's second arm.
func floorHasCodeSpan(unit string) bool { return floorCodeSpanRe.MatchString(unit) }

// floorHasMeasurementVerb is §2.1's third arm.
func floorHasMeasurementVerb(unit string) bool { return floorVerbRe.MatchString(unit) }

// inFloor reports whether a unit is in the floor: §2.1's three arms, in
// disjunction. The arms are three separate predicates rather than one scan so
// each is assertable on its own as well as through this, and so dropping one
// cannot hide behind the other two.
func inFloor(unit string) bool {
	return floorHasDigit(unit) || floorHasCodeSpan(unit) || floorHasMeasurementVerb(unit)
}

// FloorTextSHA is §2.1 step 5's and §7.2's `text_sha`: the first twelve
// lowercase hex characters of the sha256 of exactly the canonical unit text,
// UTF-8 encoded.
//
// The algorithm is spelled out in the spec rather than left to the
// implementation, and this is why: §7.2 has a reader supply the hash for a
// claim tp did not emit, so a reader with no access to this source has to
// compute the same twelve characters. The first end-to-end run of the protocol
// agreed with tp only because the unit read the prototype's source.
//
// The truncation is written as the spec's sentence — twelve characters off the
// encoded digest — rather than as the six bytes it is equivalent to, so a
// reader comparing the two has nothing to re-derive.
//
// The argument is the canonical unit text, never the spec line it came from and
// never the index's display prefix: §2.2 records that most units of that
// document exceed the prefix, so a hash over one would carry a disposition
// across a sentence rewritten from character 61 on.
func FloorTextSHA(unit string) string {
	sum := sha256.Sum256([]byte(unit))
	return hex.EncodeToString(sum[:])[:12]
}

// FloorOrdinals is §7.2's `ordinal` for a round's units: the 1-based index of
// each unit among those sharing its `text_sha`, in emission order, and 1 when
// the hash is unique. The result is parallel to the argument.
//
// It takes the HASHES rather than the units, and that is the whole of the
// design. §7.2 keys the ordinal on `text_sha` and §8 joins on
// `(text_sha, ordinal)`; an implementation counting by unit text agrees with
// this one on every input but a sha256 collision, which no test can construct.
// The wrong reading would therefore be unobservable rather than merely
// untested, so the argument makes it unwriteable instead.
//
// The caller passes the units that carry a hash — the floor, not the set the
// arms cut (§2.2). That cannot renumber anything: `inFloor` is a function of
// the unit text alone, so two units with the same text are cut or kept
// together.
func FloorOrdinals(hashes []string) []int {
	ordinals := make([]int, 0, len(hashes))
	seen := make(map[string]int, len(hashes))
	for _, h := range hashes {
		seen[h]++
		ordinals = append(ordinals, seen[h])
	}
	return ordinals
}

// FloorIndexRow is one unit as §2.2's index carries it: a floor unit as
// `(unit_id, anchor, text_sha, ordinal, byte length)`, a unit the arms cut as
// `(unit_id, anchor)` and nothing else.
//
// There is no field for the unit's text, and that is the design rather than an
// omission. Two earlier drafts of §2.2 carried the first 60 bytes of each unit,
// and the first end-to-end run of the protocol refuted the shape: locating was
// never the problem — the run found 23 of 23 units from a 60-byte head — but
// EXTENT was, because one head stopped 90 bytes short of the defect inside its
// unit and the run graded the wrong sentence. A prefix locates a unit and hides
// where it ends. A struct with nowhere to put the text cannot regress to that;
// `tp ground <spec> --units` prints every unit's whole text in one call instead.
//
// There is no `Cut` flag either, and for the same kind of reason: a flag makes
// expressible the one row §2.2 forbids — a unit announced as cut that still
// ships a hash and the obligation that comes with it. The ABSENCE of the hash
// is the cut. That reading is sound because `FloorTextSHA` returns twelve hex
// characters for every input, the empty string included, so no floor unit can
// collide with the marker.
type FloorIndexRow struct {
	ID      string // §7.2's `unit_id`: `u<N>` over every unit §2.1 produces
	Anchor  string // §7.2's `anchor`: the `§n(.n)*` section the unit sits in
	TextSHA string // §7.2's `text_sha`; empty on a unit the arms cut
	Ordinal int    // §7.2's `ordinal`, within the units sharing that hash
	Bytes   int    // the unit's length in UTF-8 bytes
}

// String renders one index line in the labelled shape §2.2 settles on.
//
// The encoding is named rather than left open because §11 row 4's bound —
// `units × 48 + 256` bytes — is undecidable without it: three conforming
// serialisations of the same five fields differ by 70%, and one JSON object per
// unit is over the bound on all but one spec in this repository, its ~79-byte
// field-name skeleton dwarfing a ~53-byte payload. The sigils carry the labels
// instead: `§` before the anchor, `#` before the ordinal, `B` after the length.
//
// A cut unit renders as `<unit_id> <anchor> (cut)` and stops there: no hash, no
// ordinal, no length, no text. §2.2 accepts that the announcement alone is not
// always enough to decide whether to grade one — the second end-to-end run said
// so — because giving the cut set its text costs the whole saving the index
// exists for, and the reader has the spec file.
func (r FloorIndexRow) String() string {
	if r.TextSHA == "" {
		return r.ID + " " + r.Anchor + " " + floorIndexCutMarker
	}
	return fmt.Sprintf("%s %s %s #%d %dB", r.ID, r.Anchor, r.TextSHA, r.Ordinal, r.Bytes)
}

// FloorIndexRows derives §2.2's index rows for a spec's text, in document
// order.
//
// EVERY unit §2.1 produces gets a row. The ones the three arms keep carry the
// five fields; the ones they cut carry an id and an anchor and nothing else,
// because §2.2 announces the cut set so a reader knows the floor is not the
// document — both end-to-end runs of the protocol found defects in units the
// arms had cut. The obligation is unchanged: coverage still runs over the
// floor alone (§8), and a cut unit a reader grounds is a reader-added row.
//
// `unit_id` is the 1-based index of the unit over every unit §2.1 produces
// (§7.2), and since every unit has a row here, the id is also the row's
// position in this result. That is a property of this result and not of the
// id — `FloorUnitRows` numbers the same way over the floor alone — so the ids
// are derived from the unit index rather than from the row index. §2.2 gives
// the reason: numbering over the floor alone renumbers every later unit when
// an edit changes one unit's arms, and §8 joins dispositions across rounds.
//
// anchorOf supplies §7.2's `anchor` and is called with the same 1-based N that
// becomes the unit's id — never with the row's index. The anchor is a parameter
// because deriving it is a different question from segmenting: §7.3's rule is
// "the last `§n(.n)*` heading at or above the unit", which needs the unit's line
// in the file, and §2.1's derivation is a pure function of a string that has
// dropped every heading by step 1. anchorOf must not be nil.
func FloorIndexRows(text string, anchorOf func(unitIndex int) string) []FloorIndexRow {
	units := FloorUnits(text)

	kept := make([]int, 0, len(units))
	hashes := make([]string, 0, len(units))
	for i, u := range units {
		if !inFloor(u) {
			continue
		}
		kept = append(kept, i)
		hashes = append(hashes, FloorTextSHA(u))
	}
	ordinals := FloorOrdinals(hashes)

	rows := make([]FloorIndexRow, 0, len(units))
	for i := range units {
		rows = append(rows, FloorIndexRow{
			ID:     fmt.Sprintf("u%d", i+1),
			Anchor: anchorOf(i + 1),
		})
	}
	for j, i := range kept {
		rows[i].TextSHA = hashes[j]
		rows[i].Ordinal = ordinals[j]
		rows[i].Bytes = len(units[i])
	}
	return rows
}

// FormatFloorIndex renders the index: the commit line, one row per line, and a
// summary, each line terminated.
//
// It takes rows rather than a spec's text, so the one payload §2.2 forbids in
// the index is not reachable from here at all.
//
// The first line names the commit the index was derived from (§2.2), and the
// caller supplies it because deriving it is a different question from
// rendering: a round takes hours — the second end-to-end run took two and a
// half — and the spec moved three commits underneath the first one, so five
// dispositioned units no longer existed by the time it finished. §7.3 pins the
// round to a snapshot; this line says which snapshot the index is, so a stale
// one is recognisable without a diff.
//
// A caller with no commit to name gets the line anyway, saying `unknown`. The
// line is unconditional so that "derived at an unknown commit" and "derived at
// the commit on line 1" are one parse apart rather than a row count apart, and
// `unknown` cannot be mistaken for a revision because a revision is hex.
//
// The last line counts UNITS, never distinct hashes. That is the one thing
// about the summary §2.2 does not state and `scripts/floor-prototype.py` does:
// its counter is keyed by `text_sha`, so counting keys under-reports by exactly
// the collisions §8 introduces `ordinal` to handle. On `spec/0.1.0.md` at
// 4d839c79 that is four units — 337 floor units over 333 distinct hashes — in
// the direction that makes coverage look higher than it is. Re-derive the pair
// with `python3 scripts/floor-prototype.py --units spec/0.1.0.md | cut -f2 |
// sort | wc -l` and the same pipeline through `sort -u`. Counting the rows
// makes the wrong reading something a caller would have to build a set to get
// back to.
//
// The prototype's summary also names how many of the document's table data rows
// were segmented into units. That clause is not ported: it is derived from the
// spec's text, and this function is given rows precisely so that the text is
// out of its reach.
func FormatFloorIndex(commit string, rows []FloorIndexRow) string {
	return formatFloorIndex(commit, rows, nil)
}

// FormatFloorIndexCarried renders the index a round's PROMPT carries: §2.2's
// index, with each unit named in carried marked `(carried)` (§8).
//
// **Every row is still emitted.** §8 narrows the ask and not the index — "a
// reader who cannot see the whole floor cannot tell it what the floor missed"
// (§2.2) — so the marker is added to a row and never removes one.
//
// carried is a set of `unit_id`s and not the carried rows themselves, because
// the only thing a renderer may know about a disposition is which unit already
// has one. A nil or empty set renders §2.2's index unchanged, which is round 1
// and every round whose preceding one decided nothing that survived.
//
// The set is applied to floor rows alone. A cut unit owes no disposition, so it
// can inherit none, and a caller naming one gets `(cut)` back regardless — the
// row §2.2 forbids is unwriteable from here rather than merely unwritten.
func FormatFloorIndexCarried(commit string, rows []FloorIndexRow, carried map[string]bool) string {
	return formatFloorIndex(commit, rows, carried)
}

// formatFloorIndex is the one renderer behind both spellings, so §2.2's index
// and §8's marked view of it cannot drift into two shapes: the commit line, the
// rows, and the summary are written once.
func formatFloorIndex(commit string, rows []FloorIndexRow, carried map[string]bool) string {
	named := commit
	if named == "" {
		named = "unknown"
	}

	floor, cut := 0, 0
	var b strings.Builder
	b.WriteString("# commit " + named + "\n")
	for _, r := range rows {
		b.WriteString(r.String())
		if r.TextSHA == "" {
			b.WriteString("\n")
			cut++
			continue
		}
		if carried[r.ID] {
			b.WriteString(" " + floorIndexCarriedMarker)
		}
		b.WriteString("\n")
		floor++
	}
	fmt.Fprintf(&b, "# %d in floor, %d cut\n", floor, cut)
	return b.String()
}

// floorIndexCutMarker is what a cut unit's row carries where a floor unit's
// carries its hash, its ordinal and its length.
//
// One constant, read by the renderer and by the parser, so the two cannot spell
// the marker differently: a parser looking for a marker the renderer no longer
// writes would read every cut row as a malformed one, and a renderer writing a
// marker the parser does not know would do the same in the other direction.
const floorIndexCutMarker = "(cut)"

// floorIndexCarriedMarker is what §8's second pass puts on the row of a unit
// this round can already carry a disposition for.
//
// It is a SUFFIX to the five fields rather than a replacement for one of them,
// because a carried unit is still a floor unit: it keeps its hash, its ordinal
// and its length, and §8's coverage counts it in the denominator exactly as it
// counts a unit the round decides. `(cut)` replaces those fields because a cut
// unit has none to carry; this one is added to them.
const floorIndexCarriedMarker = "(carried)"

// ParseFloorIndex reads back the index an emission wrote (§7.3), returning the
// rows in file order.
//
// It exists because the floor is READ and never re-derived. §7.3 makes the
// emitted index the artifact a round is graded against precisely so that a spec
// edited between emit and record cannot silently re-floor it, and every later
// reader of that round — §8's coverage, §9's advisory — owes the round the same
// treatment. Deriving the floor again from the snapshot would agree today and
// disagree the moment §2.1's arms change under a round already in flight.
//
// Lines the renderer brackets the rows with are skipped: FormatFloorIndex writes
// a commit line first and a summary line last, and both open with `#`, which no
// row does. Every other non-blank line must be a row — a truncated or
// half-written floor is refused rather than read short, because reading it short
// silently shrinks §8's denominator, and a denominator that is too small makes
// coverage look higher than it is.
func ParseFloorIndex(text string) ([]FloorIndexRow, error) {
	rows := make([]FloorIndexRow, 0, 256)
	line := 0
	for raw := range strings.SplitSeq(text, "\n") {
		line++
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		row, err := parseFloorIndexRow(trimmed)
		if err != nil {
			return nil, fmt.Errorf("floor index line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseFloorIndexRow reads one index line in either of the two shapes
// FloorIndexRow.String renders.
//
// The two are told apart by their field count and their sigils, never by a
// field count alone: a three-field line is a cut row only when its third field
// is the marker. The alternative — reading any three-field line as
// id/anchor/hash — hands the cut unit a non-empty TextSHA, and §2.2 makes a
// non-empty hash the definition of a floor unit, so the cut unit would enter
// §8's denominator owing a disposition nobody was asked for.
func parseFloorIndexRow(line string) (FloorIndexRow, error) {
	f := strings.Fields(line)
	switch {
	case len(f) == 3 && f[2] == floorIndexCutMarker:
		return FloorIndexRow{ID: f[0], Anchor: f[1]}, nil
	case len(f) == 5 && strings.HasPrefix(f[3], "#") && strings.HasSuffix(f[4], "B"):
		ordinal, ordErr := strconv.Atoi(strings.TrimPrefix(f[3], "#"))
		size, sizeErr := strconv.Atoi(strings.TrimSuffix(f[4], "B"))
		if ordErr == nil && sizeErr == nil {
			return FloorIndexRow{ID: f[0], Anchor: f[1], TextSHA: f[2], Ordinal: ordinal, Bytes: size}, nil
		}
	}
	return FloorIndexRow{}, fmt.Errorf("not an index row: %q", line)
}

// FloorUnitRow is one line of `tp ground <spec> --units`: a floor unit's
// `unit_id` and the WHOLE canonical text that id stands for.
//
// There is no `text_sha` field, and that is §11 row 4b's whole content made
// unwriteable. The row's only claim is that the hash printed on a line is the
// sha256 of the text that follows it on the same line; a struct carrying both
// as settable fields can express a line where they disagree, which is exactly
// the defect row 4b's named mutant — printing a truncation — produces. Here the
// hash is derived from the text at render time, so no caller can state one.
//
// The companion to the index. `FloorIndexRow` says what a disposition is owed
// for and deliberately has nowhere to put the text; this says what each unit
// is, in one call, so a reader never guesses where a unit ends.
type FloorUnitRow struct {
	ID   string // §7.2's `unit_id`: `u<N>` over every unit §2.1 produces
	Text string // the whole canonical unit, never a prefix of it
}

// String renders one `--units` line: id, hash and the whole unit, tab-separated.
//
// The separator is a tab rather than a space because the third field is prose
// full of spaces, so a reader splitting the line has to know where the text
// begins; two tabs make "everything after the second separator" exact. The unit
// itself cannot contain one: §2.1 step 3 collapses every whitespace run in a
// prose block to a single space, and a table row's cells are joined with an em
// dash.
func (r FloorUnitRow) String() string {
	return r.ID + "\t" + FloorTextSHA(r.Text) + "\t" + r.Text
}

// FloorUnitRows derives the `--units` rows for a spec's text, in emission order.
//
// `unit_id` is numbered over EVERY unit §2.1 produces, the ones the arms cut
// included, exactly as `FloorIndexRows` numbers them — the two artifacts are
// read side by side and join on that id, so a unit numbered over the floor
// alone here would name a different unit there.
//
// Only floor units get a row. A cut unit owes no disposition (§2.2), and giving
// the cut set its text is the cost the index exists to avoid.
func FloorUnitRows(text string) []FloorUnitRow {
	rows := make([]FloorUnitRow, 0)
	for i, u := range FloorUnits(text) {
		if !inFloor(u) {
			continue
		}
		rows = append(rows, FloorUnitRow{ID: fmt.Sprintf("u%d", i+1), Text: u})
	}
	return rows
}

// FormatFloorUnits renders the units listing: one row per line, each terminated.
func FormatFloorUnits(rows []FloorUnitRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.String())
		b.WriteString("\n")
	}
	return b.String()
}

// floorSectionHeadingRe recognises the `§n(.n)*` heading §7.3's anchor is named
// after: an ATX heading of level 2 or deeper whose first token is a dotted
// number.
//
// The level bound is a decision §7.3 does not state, and both plausible bounds
// are wrong on an input this repository already holds. Level 1 is excluded
// because a spec's H1 is its title and a title is routinely a version:
// `spec/0.19.0-agent-friction.md` opens `# 0.19.0 — Agent Friction Reduction`,
// which under a level-1-inclusive rule anchors that document's whole preamble
// to `§0.19.0` rather than to §7.3's `§0`. Levels 4 to 6 are included because a
// numbered heading is a section at any depth, and
// `scripts/floor-prototype.py`'s level-2-to-3 rule reports `spec/0.36.0.md`'s
// `#### 4.2.1` as §4.2. TestAnH1IsNotASectionAndAnH4Is pins both ends in one
// constructed fixture carrying both shapes, where each rejected bound gets
// exactly one of the two entries wrong.
var floorSectionHeadingRe = regexp.MustCompile(`^\s*#{2,6}\s+(\d+(?:\.\d+)*)`)

// floorAnchorsByLine returns the section each line of text sits in: entry i-1
// is line i's anchor, `§0` until the first `§n(.n)*` heading. A heading line
// carries its own section, which is §7.3's "at or above".
//
// The scan tracks fences for the reason step 1 does — `## 9. Fenced` inside a
// code block is sample output, not a heading. `spec/0.1.0.md` and
// `spec/0.16.0-review-orchestration.md` each carry numbered headings inside a
// fenced block, and a fence-blind scan moves every unit after one of them into
// a section the document does not have.
func floorAnchorsByLine(text string) []string {
	lines := strings.Split(text, "\n")
	anchors := make([]string, len(lines))
	current := "§0"
	inFence := false
	for i, line := range lines {
		switch {
		case floorFenceRe.MatchString(line):
			inFence = !inFence
		case inFence:
			// a heading inside a fence is code
		default:
			if m := floorSectionHeadingRe.FindStringSubmatch(line); m != nil {
				current = "§" + m[1]
			}
		}
		anchors[i] = current
	}
	return anchors
}

// FloorAnchorOf returns §7.3's anchor function for one spec's text: given the
// 1-based index of a unit over EVERY unit §2.1 produces — the ones the arms cut
// included, which is the numbering §7.2's `unit_id` uses and the index
// FloorIndexRows passes — the last `§n(.n)*` heading at or above it, and `§0`
// before the first.
//
// The anchor is resolved at the line the unit's BLOCK opens on, never by
// locating the unit's text in the file, and that is this port's one substantive
// departure from `scripts/floor-prototype.py`. The prototype searched for a
// unit's first words; a table row's block began with a sentinel that matched no
// line in the file, so every table row in a document anchored to `§0` while the
// test asserting the `§0` case passed. No figure is stated for the size of that
// set because it is per-document and this release's own spec is still moving:
// `python3 scripts/floor-prototype.py --emit <spec> | tail -1` prints it — the
// document's table-row count, beside the floor and cut counts that sum to its
// units — and the same command through `grep -c '§0'` prints how many units this
// port actually leaves at `§0`. A line number carried on the block cannot fail
// to be found.
//
// Block granularity and unit granularity differ only for a block whose lines
// span a heading, which needs prose flush against the heading on both sides
// (§2.1 step 2 splits on blank lines, and a dropped heading is not a boundary).
// Measured over this repository's 54 specs: zero instances. A unit of such a
// block can straddle the heading, so no per-unit answer exists for it; the
// block keeps the section it opens in.
//
// An index this text has no unit for answers "" rather than `§0`: `§0` is a
// legal anchor and would ship as a structurally valid row, while §7.2 requires
// the field, so an empty one is rejected at record.
func FloorAnchorOf(text string) func(unitIndex int) string {
	byLine := floorAnchorsByLine(text)
	anchors := make([]string, 0)
	for _, b := range floorBlocks(text) {
		for range floorUnitsFromBlock(b) {
			anchors = append(anchors, byLine[b.Line-1])
		}
	}
	return func(unitIndex int) string {
		if unitIndex < 1 || unitIndex > len(anchors) {
			return ""
		}
		return anchors[unitIndex-1]
	}
}
