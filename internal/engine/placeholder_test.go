package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlaceholderMatcher_DefaultPattern: under the default pattern a trimmed
// threshold wholly inside one pair of square or angle brackets, or TBD, TODO
// or TBC alone in any case, is a placeholder; written text is not.
func TestPlaceholderMatcher_DefaultPattern(t *testing.T) {
	m, err := NewPlaceholderMatcher("")
	require.NoError(t, err)
	for _, threshold := range []string{"[threshold]", "  <exit condition>  ", "[]", "TBD", "todo", " Tbc "} {
		assert.True(t, m.IsPlaceholder(threshold), "%q is a placeholder", threshold)
	}
	for _, threshold := range []string{
		"draft: 80% pass", "p95 under 200ms [measured weekly]", "[a] and [b]", "<a> [b]",
		"TBD by Friday", "TODO: write it", "[unclosed", "every member shipped",
	} {
		assert.False(t, m.IsPlaceholder(threshold), "%q is written", threshold)
	}
}

// TestPlaceholderMatcher_AProjectPatternReplacesTheDefault: a project pattern
// is matched against the trimmed threshold alone, the default no longer.
func TestPlaceholderMatcher_AProjectPatternReplacesTheDefault(t *testing.T) {
	m, err := NewPlaceholderMatcher("^draft:")
	require.NoError(t, err)
	assert.True(t, m.IsPlaceholder("  draft: 80% pass"))
	assert.False(t, m.IsPlaceholder("[threshold]"))
	assert.False(t, m.IsPlaceholder("TBD"))
}

// TestPlaceholderMatcher_AnAbsentThresholdIsNeverAPlaceholder: an empty or
// blank threshold is not a placeholder, even under a pattern matching it.
func TestPlaceholderMatcher_AnAbsentThresholdIsNeverAPlaceholder(t *testing.T) {
	for _, pattern := range []string{"", "^$", ".*"} {
		m, err := NewPlaceholderMatcher(pattern)
		require.NoError(t, err)
		assert.False(t, m.IsPlaceholder(""), "pattern %q", pattern)
		assert.False(t, m.IsPlaceholder(" \t "), "pattern %q", pattern)
	}
}

// TestPlaceholderMatcher_APatternThatDoesNotCompileIsAnError names the
// compile error.
func TestPlaceholderMatcher_APatternThatDoesNotCompileIsAnError(t *testing.T) {
	_, err := NewPlaceholderMatcher("([")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing")
}
