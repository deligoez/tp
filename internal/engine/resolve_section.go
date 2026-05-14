package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ResolveSection attempts to resolve a user-supplied section name to a canonical
// "## prefix + text" heading from the spec.
//
// Returns:
//
//	resolved   — canonical form ("## 4. Backend Migration") if a unique match is found
//	ambiguous  — true if multiple headings share the same text but differ in level
//	candidates — full canonical matches when ambiguous (for error messages)
//
// Acceptance rules:
//   - Canonical input ("## Heading") returns canonical when the heading exists.
//   - Plain-text input ("Heading") returns canonical when exactly one heading matches by text;
//     when multiple headings at different levels share the text, returns ambiguous.
//   - Whitespace is trimmed; the prefix and heading text are also normalized so
//     "##  Setup" and "## Setup " both resolve to "## Setup" when present.
//   - Empty input returns "", false, nil.
func ResolveSection(s string, headings []*Heading) (resolved string, ambiguous bool, candidates []string) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", false, nil
	}

	// Detect canonical form: leading '#' run followed by space, then text.
	level, text := splitCanonical(trimmed)

	// Build canonical headings once.
	canonicals := make([]string, len(headings))
	for i, h := range headings {
		canonicals[i] = canonicalHeading(h.Level, h.Text)
	}

	if level > 0 {
		want := canonicalHeading(level, text)
		for _, c := range canonicals {
			if c == want {
				return c, false, nil
			}
		}
		return "", false, nil
	}

	// Plain text: match by heading text only.
	var matches []string
	for i, h := range headings {
		if h.Text == trimmed {
			matches = append(matches, canonicals[i])
		}
	}
	if len(matches) == 1 {
		return matches[0], false, nil
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return "", true, matches
	}
	return "", false, nil
}

// SuggestSimilarSections returns up to 3 canonical headings closest to s by Levenshtein distance.
// Distances larger than half the input length are ignored. Returns nil if no candidates qualify.
func SuggestSimilarSections(s string, headings []*Heading) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || len(headings) == 0 {
		return nil
	}

	// Strip leading '#' prefix from the input for fairer comparison against heading text.
	cmp := trimmed
	if level, text := splitCanonical(trimmed); level > 0 {
		cmp = text
	}
	cmpLower := strings.ToLower(cmp)

	maxDist := len(cmp) / 2
	if maxDist < 1 {
		maxDist = 1
	}

	type scored struct {
		canonical string
		dist      int
	}
	var ranked []scored
	for _, h := range headings {
		d := levenshtein(cmpLower, strings.ToLower(h.Text))
		if d <= maxDist {
			ranked = append(ranked, scored{canonical: canonicalHeading(h.Level, h.Text), dist: d})
		}
	}
	if len(ranked) == 0 {
		return nil
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].dist != ranked[j].dist {
			return ranked[i].dist < ranked[j].dist
		}
		return ranked[i].canonical < ranked[j].canonical
	})
	out := make([]string, 0, 3)
	for i := 0; i < len(ranked) && i < 3; i++ {
		out = append(out, ranked[i].canonical)
	}
	return out
}

// canonicalHeading builds the "## Heading Text" form for a given level and text.
func canonicalHeading(level int, text string) string {
	return strings.Repeat("#", level) + " " + strings.TrimSpace(text)
}

// splitCanonical inspects s for a leading run of '#' followed by whitespace.
// Returns (level, text) when canonical, or (0, "") otherwise.
func splitCanonical(s string) (int, string) {
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level == 0 || level >= len(s) {
		return 0, ""
	}
	if s[level] != ' ' && s[level] != '\t' {
		return 0, ""
	}
	text := strings.TrimSpace(s[level:])
	if text == "" {
		return 0, ""
	}
	return level, text
}

// levenshtein computes the edit distance between two strings.
// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// AmbiguousSectionError reports a source_sections entry that matches multiple
// headings at different levels. Returned by NormalizeSourceSections so callers
// (tp import / tp add) can abort with an actionable message.
type AmbiguousSectionError struct {
	TaskID     string
	Entry      string
	Candidates []string
}

func (e *AmbiguousSectionError) Error() string {
	quoted := make([]string, len(e.Candidates))
	for i, c := range e.Candidates {
		quoted[i] = fmt.Sprintf("%q", c)
	}
	return fmt.Sprintf(
		"task %s: source_sections entry %q is ambiguous — matches: %s. Use the full canonical form (e.g. %q) to disambiguate.",
		e.TaskID, e.Entry, strings.Join(quoted, ", "), e.Candidates[0],
	)
}

// NormalizeSourceSections rewrites each task source_sections entry to canonical form
// using ResolveSection. Behavior:
//   - Unique resolution (canonical or plain): replaced with canonical form.
//   - Ambiguous entry: returns *AmbiguousSectionError naming the task and candidates.
//   - Unresolvable entry: left unchanged so downstream validation can report it
//     with did-you-mean suggestions.
//
// Caller passes parsed spec headings; if nil/empty, normalization is a no-op.
func NormalizeSourceSections(tasks []model.Task, headings []*Heading) error {
	if len(headings) == 0 {
		return nil
	}
	for ti := range tasks {
		for si, entry := range tasks[ti].SourceSections {
			resolved, ambiguous, candidates := ResolveSection(entry, headings)
			if ambiguous {
				return &AmbiguousSectionError{TaskID: tasks[ti].ID, Entry: entry, Candidates: candidates}
			}
			if resolved != "" {
				tasks[ti].SourceSections[si] = resolved
			}
		}
	}
	return nil
}
