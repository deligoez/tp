package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// briefSpec writes a spec whose lines 1-3 a task's source_lines can excerpt.
func briefSpec(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(path, []byte("# Spec\n\n## 1. Section\n\nBody text.\n"), 0o600))
	return path
}

// foundBriefTask is the found task tp found apply opens for one Fix-shaped
// request, carrying source_lines so an excerpt would be cut if one were.
func foundBriefTask() model.Task {
	task := newFoundTask(&FoundRequest{
		ID: "F1", Location: "x.go:40", Reproduction: "go test ./x -run TestY",
		Expected: "exit 0", Observed: "exit 1, nil map", Finder: FoundFinder{Task: "t1"},
	}, map[string]bool{})
	task.SourceLines = "1-3"
	return task
}

// TestBuildBrief_AFoundTaskShowsItsRequestAndNoExcerpt: a found task's brief
// carries the request's location, reproduction, expected and observed, cuts
// no spec excerpt, and names tp found handback for a reproduction that does
// not fail (spec/1.31.0.md §3).
func TestBuildBrief_AFoundTaskShowsItsRequestAndNoExcerpt(t *testing.T) {
	spec := briefSpec(t)
	task := foundBriefTask()
	plain := task
	plain.ID, plain.Origin = "plain", nil
	tf := &model.TaskFile{Tasks: []model.Task{task, plain}}

	// The control: the same anchors on a spec task do cut an excerpt.
	pb, err := BuildBrief(tf, &tf.Tasks[1], spec, CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	require.NotEmpty(t, pb.Task.SpecExcerpt)
	assert.Nil(t, pb.Task.Request)

	for _, compact := range []bool{false, true} {
		b, err := BuildBrief(tf, &tf.Tasks[0], spec, CommitStrategyBuiltin, "", 0, false, compact)
		require.NoError(t, err)
		assert.Empty(t, b.Task.SpecExcerpt, "compact=%v", compact)
		require.NotNil(t, b.Task.Request, "compact=%v", compact)
		assert.Equal(t, BriefFoundRequest{
			Location: "x.go:40", Reproduction: "go test ./x -run TestY",
			Expected: "exit 0", Observed: "exit 1, nil map",
			FirstStep: foundFirstStep(task.ID),
		}, *b.Task.Request, "compact=%v", compact)

		text := b.Text()
		for _, want := range []string{
			"location: x.go:40", "reproduction: go test ./x -run TestY",
			"expected: exit 0", "observed: exit 1, nil map",
			`tp found handback found-f1 "<why>"`,
		} {
			assert.Contains(t, text, want, "compact=%v", compact)
		}
		assert.NotContains(t, text, "spec_excerpt", "compact=%v", compact)

		raw, err := json.Marshal(b)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"spec_excerpt"`, "compact=%v", compact)
		assert.Contains(t, string(raw), `"reproduction":"go test ./x -run TestY"`, "compact=%v", compact)
		assert.Contains(t, string(raw), "tp found handback found-f1", "compact=%v", compact)
	}
}

// TestBuildBrief_AFoundTaskWithAFreeFormDescriptionKeepsItWhole: a found task
// added by hand carries no apply-shaped description; its brief shows the
// description verbatim rather than dropping the reproduction.
func TestBuildBrief_AFoundTaskWithAFreeFormDescriptionKeepsItWhole(t *testing.T) {
	task := model.Task{
		ID: "found-x", Title: "Fix x", Status: model.StatusOpen, Acceptance: "- fixed",
		Description: "Run make x; it panics.",
		Origin:      &model.Origin{FinderTask: "t1", Location: "x.go:9"},
	}
	tf := &model.TaskFile{Tasks: []model.Task{task}}
	b, err := BuildBrief(tf, &tf.Tasks[0], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	require.NotNil(t, b.Task.Request)
	assert.Equal(t, "x.go:9", b.Task.Request.Location)
	assert.Empty(t, b.Task.Request.Reproduction)
	assert.Equal(t, "Run make x; it panics.", b.Task.Request.Description)
	assert.Contains(t, b.Text(), "Run make x; it panics.")
}

// TestParseFoundTaskDescription_RoundTripsWhatApplyWrites: the parser reads
// back the three fields newFoundTask writes, newlines inside a field included.
func TestParseFoundTaskDescription_RoundTripsWhatApplyWrites(t *testing.T) {
	desc := foundTaskDescription("a\nb", "c", "d\ne")
	rep, exp, obs, ok := parseFoundTaskDescription(desc)
	require.True(t, ok)
	assert.Equal(t, []string{"a\nb", "c", "d\ne"}, []string{rep, exp, obs})

	_, _, _, ok = parseFoundTaskDescription("free text")
	assert.False(t, ok)
}
