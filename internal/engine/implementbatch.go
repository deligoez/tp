package engine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ImplementUnits returns the task ids the implement phase runs next. running
// is the tasks a rolling pool is running now, which the driver passes from
// memory (spec/backlog/rolling-implement.md §2); nil outside a pool.
//
// A wip task no pool and no agent spawn runs is an interrupted unit, resumed alone before
// anything new starts; a solo task, or one whose id cannot name a branch, runs
// alone when it is next in line. Either waits, with nothing returned, while
// the pool still runs, so the pool drains for it. Otherwise the ready tasks
// fill the free slots under maxParallel in tp next's order, each one's touches
// overlapping none of the running tasks' or those already taken. Touches are a
// hint (v1.20.0): a task without them joins unscreened, and the pick and the
// gate catch a clash. An empty result means nothing can start now.
func ImplementUnits(tf *model.TaskFile, maxParallel int, running []string) []string {
	// A task an agent spawn holds runs, in that agent, whoever drives.
	running = append(slices.Clone(running), AgentHeld(tf)...)
	busy := make(map[string]bool, len(running))
	for _, id := range running {
		busy[id] = true
	}
	for i := range tf.Tasks {
		if t := &tf.Tasks[i]; t.Status == model.StatusWIP && !busy[t.ID] {
			return aloneOnceDrained(t.ID, running)
		}
	}
	ready := ReadyTasks(tf)
	if len(ready) == 0 {
		return nil
	}
	if lead := ready[0]; maxParallel <= 1 || !poolable(lead) {
		return aloneOnceDrained(lead.ID, running)
	}
	taken := make([]*model.Task, 0, maxParallel)
	for i := range tf.Tasks {
		if busy[tf.Tasks[i].ID] {
			taken = append(taken, &tf.Tasks[i])
		}
	}
	ids := make([]string, 0, maxParallel)
	for _, t := range ready {
		if len(taken) >= maxParallel {
			break
		}
		if poolable(t) && disjointFromAll(t, taken) {
			taken = append(taken, t)
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// aloneOnceDrained is a task that runs alone: itself once no pool runs, and
// nothing while one does.
func aloneOnceDrained(id string, running []string) []string {
	if len(running) > 0 {
		return nil
	}
	return []string{id}
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

// implementPool reports whether units start a rolling pool: more than one
// unit, all of the implement kind.
func implementPool(units []NextUnit) bool {
	return len(units) >= 2 && allImplement(units)
}

// allImplement reports whether every unit is of the implement kind.
func allImplement(units []NextUnit) bool {
	for i := range units {
		if units[i].Kind != UnitImplement {
			return false
		}
	}
	return true
}

// TouchesWarnings names each open task without touches when maxParallel is
// above 1: such a task joins a pool unscreened, so a clash with a sibling
// discards the later unit to integrate and reruns it alone (tp validate's
// touches rule).
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
		`%s beside other tasks unscreened, and on a clash the later unit to integrate is discarded and rerun alone (run_max_parallel %d): %s; set each with tp set <id> 'touches=["path/to/file.go","dir/**"]'; a pool worktree starts without ignored files, which the operator's worktree_setup_cmd can prepare`,
		subject, maxParallel, strings.Join(ids, ", "))}}
}

// poolBriefs gives each implement unit of a pool the brief that names its
// task, since the driver claims it: two or more implement units, or any
// implement unit beside a running pool. Any other set is left as it is.
func poolBriefs(units []NextUnit, pooled bool) {
	if !implementPool(units) && (!pooled || !allImplement(units)) {
		return
	}
	for i := range units {
		units[i].BriefCommand = PoolBriefCommand(units[i].ID)
	}
}

// poolable reports whether a task may run beside another: no failed
// integration marked it to run alone, and its id is safe in a branch name.
func poolable(t *model.Task) bool {
	return !t.Solo && poolSafeID(t.ID)
}

// poolSafeIDRe is the id a pool can carry into a branch name, a worktree
// path and a pending close's file name unescaped.
var poolSafeIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// poolSafeID reports whether a task id is safe in all three: no path
// separator, no "..", and no ".lock" suffix git refuses in a ref.
func poolSafeID(id string) bool {
	return poolSafeIDRe.MatchString(id) && !strings.Contains(id, "..") &&
		!strings.HasSuffix(id, ".lock") && !strings.HasSuffix(id, ".")
}

// disjointFromAll reports whether t's touches overlap no pool member's;
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
