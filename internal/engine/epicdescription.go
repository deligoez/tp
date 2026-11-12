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

// epicDescriptionOnly reports whether unit u differs from the round before
// only in its own description: its block without the description is the
// one before read, and the block before with u's dependency blocks hashes
// to the hash before, so no dependency moved either.
func epicDescriptionOnly(before string, u *EpicReviewUnit, hashBefore string) bool {
	if before == u.Block {
		return false
	}
	restBefore, _, _ := splitEpicDescription(before)
	restNow, _, _ := splitEpicDescription(u.Block)
	return restBefore == restNow && epicHash(before+u.hashTail) == hashBefore
}

// EpicDescriptionDiff renders the change between the descriptions of two
// blocks of one epic as a line diff: "- " a removed line, "+ " an added one,
// "  " one both hold.
func EpicDescriptionDiff(beforeBlock, nowBlock string) string {
	_, before, _ := splitEpicDescription(beforeBlock)
	_, now, _ := splitEpicDescription(nowBlock)
	return lineDiff(descriptionLines(before), descriptionLines(now))
}

// descriptionLines splits a description into lines; none for an empty one.
func descriptionLines(desc string) []string {
	if desc == "" {
		return nil
	}
	return strings.Split(desc, "\n")
}

// lineDiff renders a against b through their longest common subsequence,
// removals before additions at each point they differ.
func lineDiff(a, b []string) string {
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
	var out strings.Builder
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			out.WriteString("  " + a[i] + "\n")
			i, j = i+1, j+1
		case i < len(a) && (j == len(b) || lcs[i+1][j] >= lcs[i][j+1]):
			out.WriteString("- " + a[i] + "\n")
			i++
		default:
			out.WriteString("+ " + b[j] + "\n")
			j++
		}
	}
	return out.String()
}
