package engine

import (
	"regexp"
	"strings"
)

// DefaultThresholdPlaceholder is the placeholder pattern a project without
// epic.threshold_placeholder uses (spec/1.8.0.md §8): text wholly inside one
// pair of square or angle brackets, or TBD, TODO or TBC alone, in any case.
const DefaultThresholdPlaceholder = `(?i)^(?:\[[^\[\]]*\]|<[^<>]*>|TBD|TODO|TBC)$`

// PlaceholderMatcher gives the placeholder verdict on a gate threshold: one
// the operator has not written yet.
type PlaceholderMatcher struct {
	re *regexp.Regexp
}

// NewPlaceholderMatcher compiles a project's placeholder pattern, which
// replaces the default; an empty pattern is the default. A pattern that does
// not compile is the compile error.
func NewPlaceholderMatcher(pattern string) (*PlaceholderMatcher, error) {
	if pattern == "" {
		pattern = DefaultThresholdPlaceholder
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &PlaceholderMatcher{re: re}, nil
}

// IsPlaceholder reports whether the trimmed threshold matches the pattern.
// An absent or blank threshold is never a placeholder: such a gate goes green
// once its members are done.
func (m *PlaceholderMatcher) IsPlaceholder(threshold string) bool {
	t := strings.TrimSpace(threshold)
	return t != "" && m.re.MatchString(t)
}
