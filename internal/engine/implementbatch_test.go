package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func batchTask(id string, touches ...string) model.Task {
	return model.Task{ID: id, Status: model.StatusOpen, EstimateMinutes: 5, Touches: touches}
}

// TestImplementBatch_TakesDisjointReadyTasksUpToTheCap: the batch leads with
// the task tp next would take and adds, in the same order, ready tasks whose
// touches are disjoint from every task already in it, up to run_max_parallel
// (spec/backlog/parallel-implement.md §2.2).
func TestImplementBatch_TakesDisjointReadyTasksUpToTheCap(t *testing.T) {
	t.Parallel()
	tf := &model.TaskFile{Tasks: []model.Task{
		batchTask("a", "internal/engine/"),
		batchTask("b", "internal/engine/lock.go"),
		batchTask("c", "internal/cli/"),
		batchTask("d", "docs/"),
	}}
	assert.Equal(t, []string{"a", "c", "d"}, ImplementBatch(tf, 3))
	assert.Equal(t, []string{"a", "c"}, ImplementBatch(tf, 2))
	assert.Equal(t, []string{"a"}, ImplementBatch(tf, 1), "the default cap runs one at a time")
}

// TestTouchesWarnings: under run_max_parallel above 1 each open task without
// touches is named, since it joins a batch unscreened; at 1 nothing is.
func TestTouchesWarnings(t *testing.T) {
	t.Parallel()
	done := batchTask("d")
	done.Status = model.StatusDone
	tf := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("b", "x/"), batchTask("c"), done}}
	assert.Empty(t, TouchesWarnings(tf, 1))
	got := TouchesWarnings(tf, 3)
	require.Len(t, got, 1, "one finding names every such task")
	assert.Equal(t, Finding{Severity: "warning", Rule: "touches",
		Message: `2 open tasks have no touches, so they run beside other tasks unscreened, and on a clash the later unit to integrate is discarded and rerun alone (run_max_parallel 3): a, c; set each with tp set <id> 'touches=["path/to/file.go","dir/**"]'`}, got[0])
}

// TestTouchesWarnings_Boundaries: the rule starts at run_max_parallel 2, says
// nothing when every open task names its touches, and speaks of one task in the
// singular.
func TestTouchesWarnings_Boundaries(t *testing.T) {
	t.Parallel()
	one := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("b", "x/")}}
	got := TouchesWarnings(one, 2)
	require.Len(t, got, 1, "the cap of 2 already batches")
	assert.Contains(t, got[0].Message, "1 open task has no touches, so it runs beside")
	assert.Empty(t, TouchesWarnings(&model.TaskFile{Tasks: []model.Task{batchTask("b", "x/")}}, 3), "nothing to name")
	assert.True(t, TouchesOverlap([]string{"x/"}, nil), "an empty side still overlaps for any other caller")
}

// TestImplementBatch_TouchesAreAHint: a task without touches joins a batch,
// leading or behind, since only touches both tasks name can be compared; the
// pick and the gate catch a clash (v1.20.0, the operator's decision).
func TestImplementBatch_TouchesAreAHint(t *testing.T) {
	t.Parallel()
	noTouches := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("c", "internal/cli/")}}
	assert.Equal(t, []string{"a", "c"}, ImplementBatch(noTouches, 3))

	behind := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b"), batchTask("c", "x/y.go"), batchTask("d", "y/")}}
	assert.Equal(t, []string{"a", "b", "d"}, ImplementBatch(behind, 3), "named touches still keep an overlapping task out")
}

// TestImplementBatch_ASoloOrWIPOrUnsafeTaskRunsAlone: a task a failed
// integration marked solo runs alone and never joins a batch; a wip task is
// resumed alone before any batch.
func TestImplementBatch_ASoloOrWIPOrUnsafeTaskRunsAlone(t *testing.T) {
	t.Parallel()
	solo := batchTask("a", "x/")
	solo.Solo = true
	soloFirst := &model.TaskFile{Tasks: []model.Task{solo, batchTask("c", "y/")}}
	assert.Equal(t, []string{"a"}, ImplementBatch(soloFirst, 3))
	trailing := batchTask("z", "x/")
	trailing.Solo = true
	soloBehind := &model.TaskFile{Tasks: []model.Task{batchTask("c", "y/"), trailing}}
	assert.Equal(t, []string{"c"}, ImplementBatch(soloBehind, 3), "a solo task never joins a batch")

	wip := batchTask("w", "x/")
	wip.Status = model.StatusWIP
	withWIP := &model.TaskFile{Tasks: []model.Task{batchTask("a", "y/"), wip}}
	assert.Equal(t, []string{"w"}, ImplementBatch(withWIP, 3))

	assert.Empty(t, ImplementBatch(&model.TaskFile{}, 3))

	unsafe := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b/c", "y/"), batchTask("d..e", "z/"), batchTask("f.", "w/")}}
	assert.Equal(t, []string{"a"}, ImplementBatch(unsafe, 3), "an id unsafe in a branch name or a path never joins a batch")
}
