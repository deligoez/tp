package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// foundCoverageSpec writes a spec of two sections and returns its path.
func foundCoverageSpec(t *testing.T) string {
	t.Helper()
	spec := filepath.Join(t.TempDir(), "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n\n## A\n\nText a.\n\n## B\n\nText b.\n"), 0o600))
	return spec
}

// TestSpecCoverageLeavesFoundTasksOut: spec coverage reports the same figures
// before and after a found task is added, even when the found task carries
// source_lines over a section no spec task covers (spec/1.31.0.md §3).
func TestSpecCoverageLeavesFoundTasksOut(t *testing.T) {
	spec := foundCoverageSpec(t)
	anchored := entrySchemaTask()
	anchored.SourceSections = []string{"## A"}
	tf := &model.TaskFile{Tasks: []model.Task{anchored}}

	AutoFillCoverage(tf, spec)
	before := tf.Coverage
	beforeLines := ValidateLineCoverage(tf, spec)
	require.NotEmpty(t, beforeLines, "section B is uncovered before the found task is added")

	found := foundTask()
	found.SourceLines = "1-10"
	tf.Tasks = append(tf.Tasks, found)

	AutoFillCoverage(tf, spec)
	assert.Equal(t, before, tf.Coverage)
	assert.Equal(t, beforeLines, ValidateLineCoverage(tf, spec))
}

// TestLineCoverageCountsOnlySpecTasksWhenNoneIsAnchored: the no-anchor
// warning counts the spec's tasks, not the found tasks beside them.
func TestLineCoverageCountsOnlySpecTasksWhenNoneIsAnchored(t *testing.T) {
	spec := foundCoverageSpec(t)
	unanchored := entrySchemaTask()
	unanchored.SourceSections = []string{"## Missing"}
	tf := &model.TaskFile{Tasks: []model.Task{unanchored}}
	before := ValidateLineCoverage(tf, spec)
	require.NotEmpty(t, before)

	tf.Tasks = append(tf.Tasks, foundTask())
	after := ValidateLineCoverage(tf, spec)
	assert.Equal(t, before, after)
	require.Len(t, after, 1)
	assert.Contains(t, after[0].Message, "1 tasks have no usable source anchors", "the count is the spec's one task")
}
