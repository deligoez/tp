package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// FoundFindersToReopen returns, in task-file order, the finders tp found apply
// returns to open (spec/1.31.0.md §4). A finder is the task of a blocking
// request in reqs; it is returned to open when it is wip, waits (an open entry
// of queue holds it, or one of its dependencies is not done), and no unit
// holds it: no spawn claims it and running does not name it. A done finder
// and one whose unit still runs are left as they are, so apply never pulls a
// task from under a live unit (round-4 findings 7 and 11); an apply after that
// unit exits without closing returns it to open, since the request is read
// from the store and not from the batch.
func FoundFindersToReopen(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry, running map[string]bool) []string {
	finders := map[string]bool{}
	for i := range reqs {
		if reqs[i].Blocks && reqs[i].Finder.Task != "" {
			finders[reqs[i].Finder.Task] = true
		}
	}
	held := map[string]bool{}
	for i := range queue {
		if queue[i].State == FoundQueueOpen {
			for _, f := range queue[i].Finders {
				held[f] = true
			}
		}
	}
	done := map[string]bool{}
	for i := range tasks {
		if tasks[i].Status == model.StatusDone {
			done[tasks[i].ID] = true
		}
	}
	var reopen []string
	for i := range tasks {
		t := &tasks[i]
		switch {
		case !finders[t.ID] || t.Status != model.StatusWIP:
		case !held[t.ID] && t.DepsDone(done):
		case running[t.ID] || (t.ClaimedBy != nil && t.ClaimedBy.Spawn != ""):
		default:
			reopen = append(reopen, t.ID)
		}
	}
	return reopen
}

// FoundRunningUnits names the tasks a unit runs now, for tp found apply: the
// implement units of a live tp run, its rolling pool's (LiveRunningUnits) and
// a main-tree unit whose row has no exit code while the run holds its lock,
// plus self, the calling implement unit's own task ("" when it is none).
func FoundRunningUnits(root, taskFile, self string) map[string]bool {
	running := map[string]bool{}
	if self != "" {
		running[self] = true
	}
	for _, id := range LiveRunningUnits(root, taskFile) {
		running[id] = true
	}
	st, err := ReadRunState(root, taskFile)
	if err != nil || st.StopReason != nil {
		return running
	}
	if held, lockErr := RunLockHeld(taskFile); lockErr != nil || !held {
		return running
	}
	for i := range st.Units {
		if st.Units[i].Kind == UnitImplement && st.Units[i].ExitCode == nil {
			running[st.Units[i].ID] = true
		}
	}
	return running
}
