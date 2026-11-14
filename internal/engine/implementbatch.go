package engine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ImplementBatch returns the task ids one implement batch runs
// (spec/backlog/parallel-implement.md §2.2): a wip task alone, since an
// interrupted unit is resumed before anything new starts; otherwise the ready
// task tp next would take, and, while the batch is under maxParallel, each
// later ready task whose touches overlap none already in it. Touches are a
// hint (v1.20.0): a task without them joins unscreened, and the pick and the
// gate catch a clash. A task marked solo by a failed integration never joins a
// batch and, leading, runs alone. An empty result means no task is ready.
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

// TouchesWarnings names each open task without touches when maxParallel is
// above 1: such a task joins a batch unscreened, so a clash with a sibling
// discards its unit and reruns it alone (tp validate's touches rule).
func TouchesWarnings(tf *model.TaskFile, maxParallel int) []Finding {
	if maxParallel <= 1 {
		return nil
	}
	var ids []string
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusDone && len(t.Touches) == 0 {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	// One finding for all of them: the fix is the same command per task.
	subject := fmt.Sprintf("%d open tasks have no touches, so they run", len(ids))
	if len(ids) == 1 {
		subject = "1 open task has no touches, so it runs"
	}
	return []Finding{{Severity: "warning", Rule: "touches", Message: fmt.Sprintf(
		`%s beside other tasks unscreened and one that shares a file with a batch sibling is discarded and rerun alone (run_max_parallel %d): %s; set each with tp set <id> 'touches=["path/to/file.go","dir/**"]'`,
		subject, maxParallel, strings.Join(ids, ", "))}}
}

// batchBriefs gives each unit of an implement batch the brief that names its
// task; any other set of units is left as it is.
func batchBriefs(units []NextUnit) {
	if !implementBatch(units) {
		return
	}
	for i := range units {
		units[i].BriefCommand = BatchBriefCommand(units[i].ID)
	}
}

// batchable reports whether a task may run beside another: it names what it
// touches and no failed integration marked it to run alone.
// batchable reports whether a task may run beside another: no failed
// integration marked it to run alone, and its id is safe in a branch name.
func batchable(t *model.Task) bool {
	return !t.Solo && batchSafeID(t.ID)
}

// batchSafeIDRe is the id a batch can carry into a branch name, a worktree
// path and a pending close's file name unescaped.
var batchSafeIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// batchSafeID reports whether a task id is safe in all three: no path
// separator, no "..", and no ".lock" suffix git refuses in a ref.
func batchSafeID(id string) bool {
	return batchSafeIDRe.MatchString(id) && !strings.Contains(id, "..") &&
		!strings.HasSuffix(id, ".lock") && !strings.HasSuffix(id, ".")
}

// disjointFromAll reports whether t's touches overlap no batch member's;
// only touches both tasks name can be compared, so a task without them is
// disjoint from every other.
func disjointFromAll(t *model.Task, batch []*model.Task) bool {
	if len(t.Touches) == 0 {
		return true
	}
	for _, b := range batch {
		if len(b.Touches) > 0 && TouchesOverlap(t.Touches, b.Touches) {
			return false
		}
	}
	return true
}
