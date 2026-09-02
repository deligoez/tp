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

// floorUnitsFromBlock is the seam §2.1 steps 3 to 5 fill in — canonicalising a
// prose block, splitting it into sentences, and joining a table row's cells with
// an em dash. It exists now so both kinds of block reach the later steps through
// one place and cannot diverge, and it is where the one decision steps 1 and 2
// already made will be honoured: a table data row is exactly one unit however
// many full stops its cells hold, so it never reaches step 4's sentence split.
func floorUnitsFromBlock(b floorBlock) []string {
	joined := strings.TrimSpace(strings.Join(b.Lines, " "))
	if joined == "" {
		return nil
	}
	return []string{joined}
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
