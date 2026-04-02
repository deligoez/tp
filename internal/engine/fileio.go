package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	AffectedPerFileCap = 8000
	AffectedTotalCap   = 50000
	PromptBudget       = 60000
	SpecContentCap     = 10000
	FindingsSummaryCap = 5000
)

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

func ReadAffectedFilesRaw(paths []string, maxPerFile, maxTotal int) map[string]string {
	result := make(map[string]string)
	total := 0
	for _, f := range paths {
		content, err := os.ReadFile(f)
		if err != nil {
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
		fmt.Fprintf(&b, "### %s (%d lines)\n", filepath.Base(p), lineCount)
		b.WriteString(c)
		b.WriteString("\n\n")
	}
	return b.String()
}
