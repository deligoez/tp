package engine

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Heading represents a parsed markdown heading.
type Heading struct {
	Level    int
	Text     string
	Line     int
	Parent   *Heading
	Children []*Heading
}

// ParseHeadings reads a markdown file and returns a flat list and a tree of headings.
func ParseHeadings(path string) ([]*Heading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec: %w", err)
	}
	defer f.Close()

	return ParseHeadingsFromScanner(bufio.NewScanner(f))
}

// ParseHeadingsFromScanner parses headings from a scanner (for testability).
func ParseHeadingsFromScanner(scanner *bufio.Scanner) ([]*Heading, error) {
	var headings []*Heading
	var stack []*Heading // tracks current ancestor chain
	lineNum := 0
	inCodeBlock := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Track fenced code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		if !strings.HasPrefix(line, "#") {
			continue
		}

		level, text := parseHeadingLine(line)
		if level == 0 {
			continue
		}

		h := &Heading{
			Level: level,
			Text:  text,
			Line:  lineNum,
		}

		// Find parent: walk back the stack to find the nearest heading with a lower level
		for len(stack) > 0 && stack[len(stack)-1].Level >= level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			h.Parent = stack[len(stack)-1]
			h.Parent.Children = append(h.Parent.Children, h)
		}

		stack = append(stack, h)
		headings = append(headings, h)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan spec: %w", err)
	}

	return headings, nil
}

// HeadingContentRange returns the line range (start, end) of content under a heading.
// end is the line before the next heading at the same or higher level, or EOF.
func HeadingContentRange(headings []*Heading, idx, totalLines int) (start, end int) {
	start = headings[idx].Line + 1
	if idx+1 < len(headings) {
		end = headings[idx+1].Line - 1
	} else {
		end = totalLines
	}
	return start, end
}

var headingRegexp = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func parseHeadingLine(line string) (level int, text string) {
	m := headingRegexp.FindStringSubmatch(line)
	if m == nil {
		return 0, ""
	}
	return len(m[1]), strings.TrimSpace(m[2])
}
