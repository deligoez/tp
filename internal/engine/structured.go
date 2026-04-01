package engine

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	tableRowRegex    = regexp.MustCompile(`^\s*\|(.+\|)+\s*$`)
	tableSepRegex    = regexp.MustCompile(`^\s*\|[\s\-:|]+\|[\s\-:|]*$`)
	numberedListRegex = regexp.MustCompile(`^\s*(\d+)\.\s+\S`)
)

// StructuredElements holds counts of structured elements found in a spec.
type StructuredElements struct {
	Tables        []TableInfo        `json:"tables"`
	NumberedLists []NumberedListInfo  `json:"numbered_lists"`
	CodeBlocks    int                `json:"code_blocks"`
	TotalRows     int                `json:"total_table_rows"`
	TotalItems    int                `json:"total_numbered_items"`
}

// TableInfo describes a markdown table found in the spec.
type TableInfo struct {
	Line     int    `json:"line"`
	Rows     int    `json:"rows"`
	Heading  string `json:"heading,omitempty"`
}

// NumberedListInfo describes a numbered list found in the spec.
type NumberedListInfo struct {
	Line     int    `json:"line"`
	Items    int    `json:"items"`
	LastNum  int    `json:"last_num"`
	Heading  string `json:"heading,omitempty"`
}

// CheckStructuredElements detects tables, numbered lists, and code blocks in a spec.
// Returns info-level findings summarizing what was found, so agents can verify
// that all structured elements are covered by task acceptance criteria.
func CheckStructuredElements(lines []string, headings []*Heading) ([]Finding, *StructuredElements) {
	elems := &StructuredElements{}
	var findings []Finding

	currentHeading := ""
	inCodeBlock := false

	i := 0
	for i < len(lines) {
		line := lines[i]
		lineNum := i + 1

		// Track code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				elems.CodeBlocks++
			}
			inCodeBlock = !inCodeBlock
			i++
			continue
		}

		if inCodeBlock {
			i++
			continue
		}

		// Track current heading
		if strings.HasPrefix(line, "#") {
			level, text := parseHeadingLine(line)
			if level > 0 {
				currentHeading = text
			}
			i++
			continue
		}

		// Detect table start
		if tableRowRegex.MatchString(line) && !tableSepRegex.MatchString(line) {
			table := TableInfo{Line: lineNum, Heading: currentHeading}
			// Count data rows (skip header + separator)
			j := i
			hasHeader := false
			for j < len(lines) {
				l := lines[j]
				if !tableRowRegex.MatchString(l) {
					break
				}
				if tableSepRegex.MatchString(l) {
					hasHeader = true
				} else {
					table.Rows++
				}
			j++
			}
			// If has header row, subtract it from data rows
			if hasHeader && table.Rows > 0 {
				table.Rows-- // header row is not a data row
			}
			if table.Rows > 0 {
				elems.Tables = append(elems.Tables, table)
				elems.TotalRows += table.Rows
			}
			i = j
			continue
		}

		// Detect numbered list
		if m := numberedListRegex.FindStringSubmatch(line); m != nil {
			list := NumberedListInfo{Line: lineNum, Heading: currentHeading}
			lastNum := 0
			j := i
			for j < len(lines) {
				if m2 := numberedListRegex.FindStringSubmatch(lines[j]); m2 != nil {
					list.Items++
					n := 0
					_, _ = fmt.Sscanf(m2[1], "%d", &n)
					if n > lastNum {
						lastNum = n
					}
					j++
				} else if strings.TrimSpace(lines[j]) == "" || strings.HasPrefix(strings.TrimSpace(lines[j]), "- ") {
					// continuation or sub-items
					j++
				} else {
					break
				}
			}
			list.LastNum = lastNum
			if list.Items > 0 {
				elems.NumberedLists = append(elems.NumberedLists, list)
				elems.TotalItems += list.Items
			}
			i = j
			continue
		}

		i++
	}

	// Generate findings
	if elems.TotalRows > 0 {
		findings = append(findings, Finding{
			Severity: "info",
			Rule:     "structured-elements",
			Message:  fmt.Sprintf("spec contains %d table(s) with %d data rows — verify each row maps to a task's acceptance criteria", len(elems.Tables), elems.TotalRows),
		})
		for _, t := range elems.Tables {
			findings = append(findings, Finding{
				Severity: "info",
				Rule:     "structured-elements",
				Line:     t.Line,
				Message:  fmt.Sprintf("table at line %d: %d data rows (section: %s)", t.Line, t.Rows, t.Heading),
			})
		}
	}

	if elems.TotalItems > 0 {
		findings = append(findings, Finding{
			Severity: "info",
			Rule:     "structured-elements",
			Message:  fmt.Sprintf("spec contains %d numbered list(s) with %d items — verify each item maps to a task", len(elems.NumberedLists), elems.TotalItems),
		})
		for _, nl := range elems.NumberedLists {
			findings = append(findings, Finding{
				Severity: "info",
				Rule:     "structured-elements",
				Line:     nl.Line,
				Message:  fmt.Sprintf("numbered list at line %d: %d items, #1-#%d (section: %s)", nl.Line, nl.Items, nl.LastNum, nl.Heading),
			})
		}
	}

	return findings, elems
}
