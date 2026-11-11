package engine

import (
	"fmt"
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

// ThresholdPlaceholderKey names the project config key of the pattern.
const ThresholdPlaceholderKey = "epic.threshold_placeholder"

// ProjectThresholdPlaceholder returns the placeholder pattern of the project
// whose .tp/ directory is tpDir, and whether its config.json set it: the
// default pattern when it did not, or when tpDir is "" (no .tp/ found).
func ProjectThresholdPlaceholder(tpDir string) (pattern string, fromProject bool, err error) {
	if tpDir == "" {
		return DefaultThresholdPlaceholder, false, nil
	}
	pc, _, err := LoadProjectConfig(tpDir)
	if err != nil {
		return "", false, err
	}
	if pc.Epic == nil || pc.Epic.ThresholdPlaceholder == nil || *pc.Epic.ThresholdPlaceholder == "" {
		return DefaultThresholdPlaceholder, false, nil
	}
	return *pc.Epic.ThresholdPlaceholder, true, nil
}

// ProjectPlaceholderMatcher is the matcher for the project's pattern; a
// pattern that does not compile is an error naming the key and the compile
// error.
func ProjectPlaceholderMatcher(tpDir string) (*PlaceholderMatcher, error) {
	pattern, _, err := ProjectThresholdPlaceholder(tpDir)
	if err != nil {
		return nil, err
	}
	m, err := NewPlaceholderMatcher(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ThresholdPlaceholderKey, err)
	}
	return m, nil
}

// IsPlaceholder reports whether the trimmed threshold matches the pattern.
// An absent or blank threshold is never a placeholder: such a gate goes green
// once its members are done.
func (m *PlaceholderMatcher) IsPlaceholder(threshold string) bool {
	t := strings.TrimSpace(threshold)
	return t != "" && m.re.MatchString(t)
}
