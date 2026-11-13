package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTouches_TwoGlobsOverlapUnlessTheirLiteralPrefixesDiverge: the overlap
// test is conservative (spec/backlog/parallel-implement.md §2.2): two globs
// overlap unless the literal text before their first wildcard diverges,
// compared without case, and a glob that starts with a wildcard overlaps
// everything.
func TestTouches_TwoGlobsOverlapUnlessTheirLiteralPrefixesDiverge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b []string
		want bool
	}{
		{[]string{"internal/engine/*.go"}, []string{"internal/cli/*.go"}, false},
		{[]string{"internal/engine/*.go"}, []string{"internal/engine/lock.go"}, true},
		{[]string{"internal/engine/"}, []string{"internal/engine/sub/x.go"}, true},
		{[]string{"README.md"}, []string{"readme.md"}, true},
		{[]string{"*.md"}, []string{"internal/cli/a.go"}, true},
		{[]string{"docs/a.md", "docs/b.md"}, []string{"docs/c.md"}, false},
		{nil, []string{"x.go"}, true},
	} {
		assert.Equal(t, tc.want, TouchesOverlap(tc.a, tc.b), "%v vs %v", tc.a, tc.b)
	}
}

// TestTouches_APathIsInsideItsGlobs: a path matches a glob as filepath.Match
// does, a trailing / or /** taking the whole subtree.
func TestTouches_APathIsInsideItsGlobs(t *testing.T) {
	t.Parallel()
	globs := []string{"internal/engine/*.go", "docs/", "spec/**"}
	for path, want := range map[string]bool{
		"internal/engine/lock.go":     true,
		"internal/engine/sub/lock.go": false,
		"docs/qa.md":                  true,
		"docs/a/b.md":                 true,
		"spec/x/y.md":                 true,
		"README.md":                   false,
	} {
		assert.Equal(t, want, TouchesMatch(globs, path), path)
	}
}

// TestTouches_ValidateRefusesAMalformedTouches: a task file whose task holds
// a malformed touches glob fails validation as a schema error naming the task.
func TestTouches_ValidateRefusesAMalformedTouches(t *testing.T) {
	t.Parallel()
	tf := &model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{{
		ID: "t1", Title: "T", Status: model.StatusOpen, Acceptance: "a", EstimateMinutes: 5,
		SourceSections: []string{"## 1"}, Touches: []string{"../x.go"},
	}}}
	findings := validateSchema(tf)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "task t1: touches glob")
}

// TestTouches_AMalformedGlobIsNamed: validate refuses a glob filepath.Match
// cannot read, and an absolute or parent-escaping path.
func TestTouches_AMalformedGlobIsNamed(t *testing.T) {
	t.Parallel()
	assert.NoError(t, ValidTouches([]string{"internal/*.go", "docs/"}))
	assert.Error(t, ValidTouches([]string{"internal/[.go"}))
	assert.Error(t, ValidTouches([]string{"/etc/passwd"}))
	assert.Error(t, ValidTouches([]string{"../other/x.go"}))
	assert.Error(t, ValidTouches([]string{""}))
}
