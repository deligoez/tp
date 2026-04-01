package engine

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ValidateLineCoverage checks that task source_lines cover the spec's content lines.
// Reports uncovered ranges as warnings. Empty/whitespace-only lines and
// fenced code block delimiters are excluded from the total.
func ValidateLineCoverage(tf *model.TaskFile, specPath string) []Finding {
	var findings []Finding

	// Count content lines and collect their numbers
	contentLines, _, err := countContentLines(specPath)
	if err != nil {
		findings = append(findings, Finding{Severity: "warning", Rule: "line-coverage", Message: fmt.Sprintf("could not read spec: %v", err)})
		return findings
	}

	if len(contentLines) == 0 {
		return findings
	}

	// Build covered set from all tasks' source_lines
	covered := make(map[int]bool)
	tasksWithLines := 0
	for i := range tf.Tasks {
		sl := tf.Tasks[i].SourceLines
		if sl == "" {
			continue
		}
		tasksWithLines++
		ranges, parseErr := ParseLineRanges(sl)
		if parseErr != nil {
			findings = append(findings, Finding{
				Severity: "warning",
				Rule:     "line-coverage",
				Message:  fmt.Sprintf("task %s: invalid source_lines %q: %v", tf.Tasks[i].ID, sl, parseErr),
			})
			continue
		}
		for _, r := range ranges {
			for ln := r.Start; ln <= r.End; ln++ {
				covered[ln] = true
			}
		}
	}

	// If no tasks have source_lines, warn (can't compute coverage)
	if tasksWithLines == 0 {
		findings = append(findings, Finding{
			Severity: "warning",
			Rule:     "line-coverage",
			Message:  fmt.Sprintf("%d tasks missing source_lines — line coverage cannot be computed. Add source_lines (e.g. \"15-42\") to each task for spec coverage tracking.", len(tf.Tasks)),
		})
		return findings
	}

	// Find uncovered content lines
	var uncoveredLines []int
	for _, ln := range contentLines {
		if !covered[ln] {
			uncoveredLines = append(uncoveredLines, ln)
		}
	}

	if len(uncoveredLines) == 0 {
		return findings
	}

	// Collapse into ranges for readability
	gaps := collapseToRanges(uncoveredLines)
	coveredCount := len(contentLines) - len(uncoveredLines)
	pct := float64(coveredCount) * 100.0 / float64(len(contentLines))

	findings = append(findings, Finding{
		Severity: "warning",
		Rule:     "line-coverage",
		Message:  fmt.Sprintf("line coverage: %d/%d content lines (%.0f%%). %d uncovered lines in %d gap(s)", coveredCount, len(contentLines), pct, len(uncoveredLines), len(gaps)),
	})

	for _, g := range gaps {
		findings = append(findings, Finding{
			Severity: "warning",
			Rule:     "line-coverage",
			Line:     g.Start,
			Message:  fmt.Sprintf("uncovered lines %d-%d (%d lines)", g.Start, g.End, g.End-g.Start+1),
			Context:  "Verify these spec lines are covered by a task's source_lines or mark as context.",
		})
	}

	return findings
}

// countContentLines returns the line numbers that have meaningful content
// (non-empty, not pure whitespace, not fenced code delimiters).
// Also returns total line count.
func countContentLines(specPath string) (contentLines []int, totalLines int, err error) {
	f, err := os.Open(specPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inCodeBlock := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code blocks but include their content lines
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			// Code block delimiters are structural, include them
			contentLines = append(contentLines, lineNum)
			continue
		}

		// Skip empty/whitespace-only lines
		if trimmed == "" {
			continue
		}

		contentLines = append(contentLines, lineNum)
	}

	return contentLines, lineNum, scanner.Err()
}

// collapseToRanges converts a sorted slice of ints into contiguous ranges.
func collapseToRanges(lines []int) []LineRange {
	if len(lines) == 0 {
		return nil
	}
	sort.Ints(lines)

	var ranges []LineRange
	start := lines[0]
	prev := lines[0]

	for i := 1; i < len(lines); i++ {
		if lines[i] == prev+1 {
			prev = lines[i]
		} else {
			ranges = append(ranges, LineRange{Start: start, End: prev})
			start = lines[i]
			prev = lines[i]
		}
	}
	ranges = append(ranges, LineRange{Start: start, End: prev})
	return ranges
}
