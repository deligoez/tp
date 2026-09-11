package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/deligoez/tp/internal/output"
)

const (
	AffectedPerFileCap = 8000
	AffectedTotalCap   = 50000
	PromptBudget       = 60000
	SpecContentCap     = 10000
	FindingsSummaryCap = 5000
)

// SpecCut names a spec whose inline excerpt holds only its head: the path to
// read the rest from, the bytes the excerpt kept and the bytes there are in
// all. Both counts are over the text the prompt embeds, which is the spec with
// its frontmatter blanked.
type SpecCut struct {
	Path       string `json:"path"`
	KeptBytes  int    `json:"kept_bytes"`
	TotalBytes int    `json:"total_bytes"`
}

// CapSpecContent is the one cut every inline spec excerpt takes. Content that
// fits SpecContentCap comes back whole with a nil SpecCut. Longer content is cut
// to the last complete rune at or before the cap — a byte slice there kept
// the lead byte of a split rune, invalid UTF-8 the JSON encoder turned into
// U+FFFD — and followed by a marker line naming both counts and the path, so
// the role reading the prompt knows what it did not see and where it is. The
// SpecCut is what the caller puts in its payload.
func CapSpecContent(content, path string) (string, *SpecCut) {
	if len(content) <= SpecContentCap {
		return content, nil
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	cut := &SpecCut{Path: path, KeptBytes: runeBoundaryAtOrBefore(content, SpecContentCap), TotalBytes: len(content)}
	return content[:cut.KeptBytes] + fmt.Sprintf("\n[...spec truncated at %d of %d bytes; read the rest at %s]", cut.KeptBytes, cut.TotalBytes, cut.Path), cut
}

// runeBoundaryAtOrBefore returns the largest n' <= n at which s[:n'] ends on a
// rune boundary: n itself unless the rune starting just before it needs bytes
// past n. Bytes that are not UTF-8 to begin with are left where they are,
// since no boundary exists to back off to.
func runeBoundaryAtOrBefore(s string, n int) int {
	// The last rune start is at most utf8.UTFMax-1 bytes back; FullRuneInString
	// is false only for a valid prefix of a longer encoding, so an invalid byte
	// is never mistaken for a split rune.
	for start := n - 1; start >= 0 && start > n-utf8.UTFMax; start-- {
		if utf8.RuneStart(s[start]) {
			if utf8.FullRuneInString(s[start:n]) {
				return n
			}
			return start
		}
	}
	return n
}

type AffectedSummary struct {
	TotalFiles    int `json:"total_files"`
	TotalLines    int `json:"total_lines"`
	CharsIncluded int `json:"chars_included"`
}

func DedupPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

func ReadAffectedFiles(paths []string) map[string]string {
	return ReadAffectedFilesRaw(paths, AffectedPerFileCap, AffectedTotalCap)
}

func ReadAffectedFilesBudgetAware(paths []string, otherContent ...string) map[string]string {
	used := 0
	for _, c := range otherContent {
		used += len(c)
	}
	remaining := PromptBudget - used
	if remaining < 0 {
		remaining = 5000
	}

	if remaining >= AffectedTotalCap {
		return ReadAffectedFiles(paths)
	}

	return ReadAffectedFilesRaw(paths, AffectedPerFileCap, remaining)
}

// ReadAffectedFilesRaw reads the affected-file set for a prompt, capped per
// file and in total. A file it cannot read is named on stderr rather than
// dropped in silence: callers stat these paths up front, so an unreadable one
// is an anomaly, and its absence from the map is indistinguishable from a file
// that was never requested.
func ReadAffectedFilesRaw(paths []string, maxPerFile, maxTotal int) map[string]string {
	result := make(map[string]string)
	total := 0
	for _, f := range paths {
		content, err := os.ReadFile(f)
		if err != nil {
			output.Notice(fmt.Sprintf("warning: cannot read affected file %s; its contents were dropped from the prompt (%v)", f, err))
			continue
		}
		s := string(content)
		if len(s) > maxPerFile {
			s = s[:maxPerFile] + fmt.Sprintf("\n[...truncated at %d chars]", maxPerFile)
		}
		if total+len(s) > maxTotal {
			remaining := maxTotal - total
			if remaining > 100 {
				s = s[:remaining] + "\n[...truncated by total cap]"
				result[f] = s
			}
			break
		}
		result[f] = s
		total += len(s)
	}
	return result
}

func BuildAffectedSummary(paths []string, content map[string]string) *AffectedSummary {
	if len(paths) == 0 {
		return nil
	}
	totalLines := 0
	charsIncluded := 0
	for _, p := range paths {
		if c, ok := content[p]; ok {
			totalLines += strings.Count(c, "\n") + 1
			charsIncluded += len(c)
		} else if raw, err := os.ReadFile(p); err == nil {
			totalLines += strings.Count(string(raw), "\n") + 1
		}
	}
	return &AffectedSummary{
		TotalFiles:    len(paths),
		TotalLines:    totalLines,
		CharsIncluded: charsIncluded,
	}
}

func BuildAffectedSection(content map[string]string) string {
	if len(content) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("## Affected Files\n\n")
	sorted := make([]string, 0, len(content))
	for p := range content {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	for _, p := range sorted {
		c := content[p]
		lineCount := strings.Count(c, "\n") + 1
		fmt.Fprintf(&b, "### %s (%d lines)\n", p, lineCount)
		b.WriteString(c)
		b.WriteString("\n\n")
	}
	return b.String()
}
