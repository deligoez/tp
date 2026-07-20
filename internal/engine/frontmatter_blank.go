package engine

import "strings"

// BlankFrontmatter replaces the frontmatter block (opening ---, body,
// closing ---) with empty lines so every spec parser excludes it while
// absolute line numbers stay untouched. Content without frontmatter is
// returned unchanged.
func BlankFrontmatter(data []byte) []byte {
	fm := ParseFrontmatterBytes(data)
	if !fm.Present {
		return data
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i := fm.Lines.Start - 1; i < fm.Lines.End && i < len(lines); i++ {
		lines[i] = ""
	}
	return []byte(strings.Join(lines, "\n"))
}

// BlankFrontmatterLines applies the same exclusion to a pre-split line slice.
func BlankFrontmatterLines(lines []string) []string {
	fm := ParseFrontmatterBytes([]byte(strings.Join(lines, "\n")))
	if !fm.Present {
		return lines
	}
	out := make([]string, len(lines))
	copy(out, lines)
	for i := fm.Lines.Start - 1; i < fm.Lines.End && i < len(out); i++ {
		out[i] = ""
	}
	return out
}
