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
// touches is named, since it never runs beside another; at 1 nothing is.
func TestTouchesWarnings(t *testing.T) {
	t.Parallel()
	done := batchTask("d")
	done.Status = model.StatusDone
	tf := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("b", "x/"), done}}
	assert.Empty(t, TouchesWarnings(tf, 1))
	got := TouchesWarnings(tf, 3)
	require.Len(t, got, 1)
	assert.Equal(t, Finding{Severity: "warning", Rule: "touches",
		Message: "task a: no touches, so it never runs beside another task (run_max_parallel 3)"}, got[0])
}

// TestImplementBatch_ATaskWithoutTouchesOrMarkedSoloRunsAlone: a lead without
// touches, or one a failed integration marked solo, runs alone; such a task is
// never added behind a lead; and a wip task is resumed alone before any batch.
func TestImplementBatch_ATaskWithoutTouchesOrMarkedSoloRunsAlone(t *testing.T) {
	t.Parallel()
	noTouches := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("c", "internal/cli/")}}
	assert.Equal(t, []string{"a"}, ImplementBatch(noTouches, 3))

	behind := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b"), batchTask("c", "y/")}}
	assert.Equal(t, []string{"a", "c"}, ImplementBatch(behind, 3))

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

	unsafe := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b/c", "y/"), batchTask("d..e", "z/")}}
	assert.Equal(t, []string{"a"}, ImplementBatch(unsafe, 3), "an id unsafe in a branch name or a path never joins a batch")
}
