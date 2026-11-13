package engine

import (
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// ImplementBatch returns the task ids one implement batch runs
// (spec/backlog/parallel-implement.md §2.2): a wip task alone, since an
// interrupted unit is resumed before anything new starts; otherwise the ready
// task tp next would take, and, while the batch is under maxParallel, each
// later ready task whose touches are disjoint from every task already in it.
// A task without touches, or marked solo by a failed integration, never joins
// a batch and, leading, runs alone. An empty result means no task is ready.
func ImplementBatch(tf *model.TaskFile, maxParallel int) []string {
	id, wip, has := ImplementPreview(tf)
	if !has {
		return nil
	}
	if wip {
		return []string{id}
	}
	ready := ReadyTasks(tf)
	lead := ready[0]
	if maxParallel <= 1 || !batchable(lead) {
		return []string{lead.ID}
	}
	batch := []*model.Task{lead}
	for _, t := range ready[1:] {
		if len(batch) >= maxParallel {
			break
		}
		if batchable(t) && disjointFromAll(t, batch) {
			batch = append(batch, t)
		}
	}
	ids := make([]string, len(batch))
	for i, t := range batch {
		ids[i] = t.ID
	}
	return ids
}

// implementMaxParallel is the task file's effective run_max_parallel: its own
// override over the project layer over the default.
func implementMaxParallel(taskFilePath string, tf *model.TaskFile) int {
	if tf == nil {
		return RunMaxParallelDefault
	}
	override := tf.Workflow
	ClampWorkflowRanges(&override)
	project := ProjectWorkflowOverride(filepath.Dir(taskFilePath))
	return ResolveWorkflowLayers(&override, &project).RunMaxParallel
}

// implementBatch reports whether units are an implement batch: more than one
// unit, all of the implement kind.
func implementBatch(units []NextUnit) bool {
	if len(units) < 2 {
		return false
	}
	for i := range units {
		if units[i].Kind != UnitImplement {
			return false
		}
	}
	return true
}

// batchable reports whether a task may run beside another: it names what it
// touches and no failed integration marked it to run alone.
func batchable(t *model.Task) bool {
	return len(t.Touches) > 0 && !t.Solo
}

func disjointFromAll(t *model.Task, batch []*model.Task) bool {
	for _, b := range batch {
		if TouchesOverlap(t.Touches, b.Touches) {
			return false
		}
	}
	return true
}
