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
	Hint     string `json:"hint,omitempty"`
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
				prevFull := fmt.Sprintf("%s.%d", parent, prev)
				currFull := fmt.Sprintf("%s.%d", parent, curr)
				missFull := fmt.Sprintf("%s.%d", parent, missing)
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

// CheckOrphanListItems detects numbered lists that start at >1 or have gaps.
// Only checks lines matching "N. " pattern outside fenced code blocks.
func CheckOrphanListItems(lines []string) []Finding {
	var findings []Finding
	inCodeBlock := false
	listNumRegex := regexp.MustCompile(`^(\d+)\.\s`)

	type listGroup struct {
		startLine int
		numbers   []int
		lines     []int
	}

	var current *listGroup

	flush := func() {
		if current == nil || len(current.numbers) < 2 {
			current = nil
			return
		}
		// Check start > 1
		if current.numbers[0] != 1 {
			findings = append(findings, Finding{
				Line:     current.lines[0],
				Severity: "info",
				Rule:     "orphan-list-item",
				Message:  fmt.Sprintf("numbered list starts at %d (expected 1)", current.numbers[0]),
			})
		}
		// Check gaps
		for i := 1; i < len(current.numbers); i++ {
			expected := current.numbers[i-1] + 1
			actual := current.numbers[i]
			if actual != expected {
				findings = append(findings, Finding{
					Line:     current.lines[i],
					Severity: "info",
					Rule:     "orphan-list-item",
					Message:  fmt.Sprintf("numbered list gap: expected %d, got %d", expected, actual),
				})
			}
		}
		current = nil
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			flush()
			continue
		}
		if inCodeBlock {
			continue
		}

		match := listNumRegex.FindStringSubmatch(trimmed)
		if match != nil {
			num, _ := strconv.Atoi(match[1])
			if current == nil {
				current = &listGroup{startLine: i + 1}
			}
			current.numbers = append(current.numbers, num)
			current.lines = append(current.lines, i+1)
		} else if trimmed != "" {
			flush()
		}
	}
	flush()

	return findings
}

// CheckDuplicateParagraphs detects two consecutive identical paragraphs
// (blank-line separated blocks of text) — a copy-paste artifact the line-level
// duplicate check misses. Fenced code blocks are excluded, and a code block
// between two blocks breaks their adjacency. Single-line paragraphs that are
// headings (already covered by duplicate-heading) or horizontal rules are
// skipped to avoid double-reporting and structural false positives.
func CheckDuplicateParagraphs(lines []string) []Finding {
	var findings []Finding
	inCodeBlock := false

	var cur []string // current paragraph, right-trimmed lines
	curStart := 0
	var prev []string // immediately preceding paragraph; nil when adjacency broken

	endParagraph := func() {
		if len(cur) == 0 {
			return
		}
		if prev != nil && equalLines(prev, cur) && !skipDuplicateParagraph(cur) {
			ctx := strings.Join(cur, " ")
			if len(ctx) > 80 {
				ctx = ctx[:80]
			}
			findings = append(findings, Finding{
				Line:     curStart,
				Severity: "warning",
				Rule:     "duplicate-paragraph",
				Message:  "duplicate consecutive paragraph",
				Context:  ctx,
			})
		}
		prev = cur
		cur = nil
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			endParagraph()
			prev = nil // a code block between two paragraphs breaks adjacency
			continue
		}
		if inCodeBlock {
			continue
		}
		if trimmed == "" {
			endParagraph()
			continue
		}
		if len(cur) == 0 {
			curStart = i + 1
		}
		cur = append(cur, strings.TrimRight(line, " \t"))
	}
	endParagraph()

	return findings
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// skipDuplicateParagraph excludes single-line paragraphs that are headings
// (duplicate-heading covers them) or horizontal rules (---, ***, ___).
func skipDuplicateParagraph(content []string) bool {
	if len(content) != 1 {
		return false
	}
	t := strings.TrimSpace(content[0])
	if strings.HasPrefix(t, "#") {
		return true
	}
	return isHorizontalRule(t)
}

func isHorizontalRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != c && s[i] != ' ' {
			return false
		}
	}
	return true
}

var (
	crossRefSectionStep = regexp.MustCompile(`(?i)§\s*(\d+(?:\.\d+)*)[\s,]+step\s+(\d+)`)
	crossRefStepSection = regexp.MustCompile(`(?i)\bstep\s+(\d+)\s+(?:of|in)\s+§\s*(\d+(?:\.\d+)*)`)
	sectionNumPrefix    = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s`)
	listItemPrefix      = regexp.MustCompile(`^\s*(\d+)\.\s`)
)

// CheckBrokenCrossRefs detects references of the form "§X.Y step N" (or
// "step N of/in §X.Y") where section X.Y contains fewer than N numbered steps.
// It is deliberately conservative to keep the false-positive rate low: it fires
// only when the referenced section is a heading whose content holds a numbered
// list, and the referenced step exceeds the largest such list. References into
// sections with no numbered list, or to a number that matches no heading, are
// never reported. References inside fenced code blocks are ignored.
func CheckBrokenCrossRefs(lines []string, headings []*Heading) []Finding {
	sectionSteps := sectionStepCounts(lines, headings)
	if len(sectionSteps) == 0 {
		return nil
	}

	var findings []Finding
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		check := func(sectionNum string, step int) {
			steps, ok := sectionSteps[sectionNum]
			if !ok || steps == 0 || step <= steps {
				return
			}
			findings = append(findings, Finding{
				Line:     i + 1,
				Severity: "warning",
				Rule:     "broken-cross-ref",
				Message:  fmt.Sprintf("§%s step %d referenced but §%s has only %d numbered items", sectionNum, step, sectionNum, steps),
				Context:  strings.TrimSpace(line),
			})
		}

		for _, m := range crossRefSectionStep.FindAllStringSubmatch(line, -1) {
			step, _ := strconv.Atoi(m[2])
			check(m[1], step)
		}
		for _, m := range crossRefStepSection.FindAllStringSubmatch(line, -1) {
			step, _ := strconv.Atoi(m[1])
			check(m[2], step)
		}
	}

	return findings
}

// sectionStepCounts maps each numbered section heading to its step count,
// defined as the largest numbered list under that heading. The first occurrence
// of a repeated number wins.
func sectionStepCounts(lines []string, headings []*Heading) map[string]int {
	counts := make(map[string]int)
	total := len(lines)
	for idx, h := range headings {
		m := sectionNumPrefix.FindStringSubmatch(h.Text)
		if m == nil {
			continue
		}
		num := m[1]
		if _, exists := counts[num]; exists {
			continue
		}
		start, end := HeadingContentRange(headings, idx, total)
		if steps := largestListRun(lines, start, end); steps > 0 {
			counts[num] = steps
		}
	}
	return counts
}

// largestListRun scans the 1-based inclusive line range [start, end] for
// numbered list items and returns the size of the largest run. A run breaks on
// a numbered item whose value decreases below the previous item's (a new list)
// or on a non-list, non-blank content line; blank lines do not break a run. Size is
// max(item count, highest literal number), so both sequential (1. 2. 3.) and
// repeated-marker (1. 1. 1.) markdown numbering are counted correctly. Counting
// generously (including nested items) biases toward under-reporting broken refs.
func largestListRun(lines []string, start, end int) int {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	inCodeBlock := false
	best, count, maxNum, prevNum := 0, 0, 0, 0
	flush := func() {
		if count > best {
			best = count
		}
		if maxNum > best {
			best = maxNum
		}
		count, maxNum, prevNum = 0, 0, 0
	}
	for i := start - 1; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			flush()
			continue
		}
		if inCodeBlock {
			continue
		}
		if trimmed == "" {
			continue // blank lines do not break a list
		}
		if m := listItemPrefix.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if count > 0 && n < prevNum {
				flush() // a decrease in value starts a new list
			}
			count++
			if n > maxNum {
				maxNum = n
			}
			prevNum = n
		} else {
			flush() // a non-list content line ends the current run
		}
	}
	flush()
	return best
}
