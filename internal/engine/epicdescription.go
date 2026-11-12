package engine

import "strings"

// splitEpicDescription cuts an epic block into what precedes its description
// and the description's text, un-indented. A description: line opens one
// only as the block's last top-level section: it has something before it,
// and every line after it is blank or indented, at least one of them there.
// Elsewhere, such as inside a multi-line acceptance or why, it is a
// continuation line and the block has no description.
func splitEpicDescription(block string) (rest, desc string, ok bool) {
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	i := len(lines) - 1
	for i >= 0 && (lines[i] == "" || strings.HasPrefix(lines[i], epicDescriptionIndent)) {
		i--
	}
	if i < 1 || i == len(lines)-1 || lines[i] != epicDescriptionHeader {
		return block, "", false
	}
	body := lines[i+1:]
	for j := range body {
		body[j] = strings.TrimPrefix(body[j], epicDescriptionIndent)
	}
	return strings.Join(lines[:i], "\n") + "\n", strings.Join(body, "\n"), true
}

// epicDescriptionOnly reports whether epic unit u, whose hash moved since
// prev, differs from it only in its own description. It compares hashes, not
// text: the hash of u's own block without the description and of its
// dependency blocks against the ones prev stored. A snapshot from before
// v1.11.0 stored neither and could hold no description, so there the block
// before must equal u's own block without the description, and that block
// with u's dependency blocks must hash to the hash before.
func epicDescriptionOnly(prev *epicSnapshot, u *EpicReviewUnit, before, hashBefore string) bool {
	if u.ownRest == "" {
		return false
	}
	if rest, kept := prev.OwnRests[u.Key]; kept {
		return rest == epicHash(u.ownRest) && prev.Tails[u.Key] == epicHash(u.hashTail)
	}
	return before == u.ownRest && epicHash(before+u.hashTail) == hashBefore
}

// EpicDescriptionDiff renders how the description of an Authored epic unit
// changed from the block before to u's block, as hunks of a line diff: "- "
// a removed line, "+ " an added one, "  " one unchanged line of context on
// either side of a change, and "…" for each run of unchanged lines left out.
// The own block without the description is the same in both, so each
// description is what follows it; no text is parsed for its shape.
func EpicDescriptionDiff(before string, u *EpicReviewUnit) string {
	return lineHunks(descriptionLines(strings.TrimPrefix(before, u.ownRest)), descriptionLines(strings.TrimPrefix(u.Block, u.ownRest)))
}

// descriptionLines returns the text lines of a rendered description section
// (epicDescriptionBlock), un-indented; none for an empty section.
func descriptionLines(section string) []string {
	body, found := strings.CutPrefix(section, epicDescriptionHeader+"\n")
	if !found {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimPrefix(lines[i], epicDescriptionIndent)
	}
	return lines
}

// diffOp is one line of a line diff: kind ' ', '-' or '+', and the line.
type diffOp struct {
	kind byte
	line string
}

// lineDiffOps diffs a against b through their longest common subsequence,
// removals before additions at each point they differ.
func lineDiffOps(a, b []string) []diffOp {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i, j = i+1, j+1
		case i < len(a) && (j == len(b) || lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	return ops
}

// lineHunks renders a's diff to b keeping every change and the one
// unchanged line on either side of it; each run of other unchanged lines
// becomes one "…" line.
func lineHunks(a, b []string) string {
	ops := lineDiffOps(a, b)
	keep := make([]bool, len(ops))
	for k, op := range ops {
		if op.kind != ' ' {
			for n := max(k-1, 0); n <= min(k+1, len(ops)-1); n++ {
				keep[n] = true
			}
		}
	}
	var out strings.Builder
	elided := false
	for k, op := range ops {
		if !keep[k] {
			if !elided {
				out.WriteString("…\n")
			}
			elided = true
			continue
		}
		elided = false
		out.WriteString(string(op.kind) + " " + op.line + "\n")
	}
	return out.String()
}
