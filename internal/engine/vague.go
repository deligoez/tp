package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// VagueWord defines a flagged word and its suggested replacement.
type VagueWord struct {
	Pattern    *regexp.Regexp
	Word       string
	Suggestion string
}

var vagueWords = []VagueWord{
	{regexp.MustCompile(`(?i)\bappropriate\b`), "appropriate", "Specify the exact condition"},
	{regexp.MustCompile(`(?i)\brelevant\b`), "relevant", "List the specific items"},
	{regexp.MustCompile(`(?i)\bas needed\b`), "as needed", "Define when it's needed"},
	{regexp.MustCompile(`(?i)\betc\.\b`), "etc.", "List all items explicitly"},
	{regexp.MustCompile(`(?i)\bvarious\b`), "various", "Enumerate the specific items"},
	{regexp.MustCompile(`(?i)\bsome\b`), "some", "Specify which ones or how many"},
	{regexp.MustCompile(`(?i)\bproper\b`), "proper", "Define what \"proper\" means"},
	{regexp.MustCompile(`(?i)\bproperly\b`), "properly", "Define what \"properly\" means"},
}

// Finding represents a single lint finding.
type Finding struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Context  string `json:"context,omitempty"`
}

// CheckVagueLanguage scans lines for vague words and returns findings.
func CheckVagueLanguage(lines []string) []Finding {
	var findings []Finding
	for i, line := range lines {
		for _, vw := range vagueWords {
			if vw.Pattern.MatchString(line) {
				findings = append(findings, Finding{
					Line:     i + 1,
					Severity: "warning",
					Rule:     "vague-language",
					Message:  fmt.Sprintf("%q is vague — %s", vw.Word, vw.Suggestion),
					Context:  strings.TrimSpace(line),
				})
			}
		}
	}
	return findings
}

// CheckDuplicateConsecutiveLines detects consecutive identical non-empty lines.
// Fenced code block contents (between ``` delimiters) are excluded.
func CheckDuplicateConsecutiveLines(lines []string) []Finding {
	var findings []Finding
	inCodeBlock := false
	prevLine := ""
	prevEmpty := true

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks (``` optionally followed by language)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			prevLine = ""
			prevEmpty = true
			continue
		}

		if inCodeBlock {
			continue
		}

		isEmpty := trimmed == ""
		if isEmpty {
			prevEmpty = true
			prevLine = ""
			continue
		}

		if !prevEmpty && trimmed == prevLine {
			ctx := trimmed
			if len(ctx) > 80 {
				ctx = ctx[:80]
			}
			findings = append(findings, Finding{
				Line:     i + 1,
				Severity: "warning",
				Rule:     "duplicate-line",
				Message:  "duplicate consecutive line",
				Context:  ctx,
			})
		}

		prevLine = trimmed
		prevEmpty = false
	}
	return findings
}

// CheckNumberingGaps detects gaps in numbered section headings.
func CheckNumberingGaps(headings []*Heading) []Finding {
	var findings []Finding

	// Group headings by parent prefix and level
	type groupKey struct {
		parent string
		level  int
	}
	groups := make(map[groupKey][]struct {
		num       int
		line      int
		text      string
		parent    string
		headingPx string
	})

	numRegex := regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s`)

	for _, h := range headings {
		match := numRegex.FindStringSubmatch(h.Text)
		if match == nil {
			continue
		}

		numStr := match[1]
		parts := strings.Split(numStr, ".")

		// Last part is the number within its group
		lastPart := parts[len(parts)-1]
		num, _ := strconv.Atoi(lastPart)

		// Parent prefix is everything except the last part
		parent := ""
		if len(parts) > 1 {
			parent = strings.Join(parts[:len(parts)-1], ".")
		}

		prefix := strings.Repeat("#", h.Level)
		key := groupKey{parent: parent, level: h.Level}
		groups[key] = append(groups[key], struct {
			num       int
			line      int
			text      string
			parent    string
			headingPx string
		}{num: num, line: h.Line, text: h.Text, parent: parent, headingPx: prefix})
	}

	// Check each group for gaps
	for _, entries := range groups {
		if len(entries) < 2 {
			continue
		}
		for i := 1; i < len(entries); i++ {
			prev := entries[i-1].num
			curr := entries[i].num
			parent := entries[i].parent
			px := entries[i].headingPx
			for missing := prev + 1; missing < curr; missing++ {
				prevFull := fmt.Sprintf("%s %d", parent, prev)
				currFull := fmt.Sprintf("%s %d", parent, curr)
				missFull := fmt.Sprintf("%s %d", parent, missing)
				if parent == "" {
					prevFull = fmt.Sprintf("%d", prev)
					currFull = fmt.Sprintf("%d", curr)
					missFull = fmt.Sprintf("%d", missing)
				}
				findings = append(findings, Finding{
					Line:     entries[i].line,
					Severity: "warning",
					Rule:     "numbering-gap",
					Message:  fmt.Sprintf("section numbering gap: %s %s → %s %s (missing %s)", px, prevFull, px, currFull, missFull),
					Context:  entries[i].text,
				})
			}
		}
	}

	return findings
}
