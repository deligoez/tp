package engine

import (
	"regexp"
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
type floorBlock struct {
	Lines      []string
	IsTableRow bool
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
	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, floorBlock{Lines: current})
			current = nil
		}
	}

	inFence := false
	for _, line := range strings.Split(text, "\n") {
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
				blocks = append(blocks, floorBlock{Lines: []string{line}, IsTableRow: true})
			}
		case floorAtxHeadingRe.MatchString(line) || isFloorHorizontalRule(line):
			// dropped
		default:
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

// STUB — the no-split mutant, to observe the split test red.
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
	// The digit arm, literally §2.1's `[0-9]` — ASCII digits, not a Unicode
	// class, because the table names the character range.
	floorDigitRe = regexp.MustCompile(`[0-9]`)
	// The identifier arm. §2.1 says a backtick-delimited SPAN, so both
	// delimiters must be present in the same unit; the prototype tested for a
	// single backtick, which differs on 21 of this corpus's 7,368 units — every
	// one of them a span the sentence split cut in half.
	floorCodeSpanRe = regexp.MustCompile("`[^`]*`")
	// The verb arm. Whole words, because "Transition", "branches" and
	// "withheld" each contain a listed verb as a substring: over `spec/*.md` a
	// substring reading and this one disagree on 254 units. Case-insensitive,
	// because a claim's verb is routinely its sentence's first word — nine
	// units of this corpus reach the floor only through the fold, among them
	// "Measured while implementing: it can."
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
