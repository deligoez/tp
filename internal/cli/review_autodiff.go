package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

const (
	diffBlockSectionLineCap = 40
	diffBlockTotalCharCap   = 6000
)

// buildChangedSectionsBlock renders the changed-sections block injected into
// every prompt: changed/added/removed heading list plus per-section new
// content capped at 40 lines per section and 6000 characters total.
func buildChangedSectionsBlock(dr *engine.DiffResult, sinceLabel string) string {
	if len(dr.Changed) == 0 && len(dr.Removed) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Changed sections since " + sinceLabel + "\n\n")
	for _, s := range dr.Changed {
		fmt.Fprintf(&b, "- %s: %s\n", s.Status, s.Heading)
	}
	for _, s := range dr.Removed {
		fmt.Fprintf(&b, "- REMOVED: %s\n", s.Heading)
	}

	total := 0
	for _, s := range dr.Changed {
		lines := strings.Split(s.Content, "\n")
		if len(lines) > diffBlockSectionLineCap {
			lines = append(lines[:diffBlockSectionLineCap], fmt.Sprintf("[...section truncated at %d lines]", diffBlockSectionLineCap))
		}
		sect := fmt.Sprintf("\n### %s\n%s\n", s.Heading, strings.Join(lines, "\n"))
		if total+len(sect) > diffBlockTotalCharCap {
			if remaining := diffBlockTotalCharCap - total; remaining > 0 {
				b.WriteString(sect[:remaining])
			}
			fmt.Fprintf(&b, "\n[...diff content truncated at %d chars]\n", diffBlockTotalCharCap)
			break
		}
		b.WriteString(sect)
		total += len(sect)
	}
	return b.String()
}

// newestEarlierSnapshot returns the newest snapshot with round < r, or (0, "")
// when none exists.
func newestEarlierSnapshot(specPath string, r int) (round int, path string) {
	for k := r - 1; k >= 1; k-- {
		p := filepath.Join(engine.ReviewStateDir(specPath), fmt.Sprintf("snapshot-round-%d.md", k))
		if _, err := os.Stat(p); err == nil {
			return k, p
		}
	}
	return 0, ""
}

// diffLinesOf reads and frontmatter-blanks a spec file for section diffing.
func diffLinesOf(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return engine.BlankFrontmatterLines(strings.Split(string(data), "\n"))
}
