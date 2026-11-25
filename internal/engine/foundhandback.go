package engine

import (
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// FoundHandbackEntry is the open queue entry a handed-back found task leaves
// (spec/1.31.0.md §3): the request the task answered and, as its finders,
// every task not done that depends on it, which the entry now holds out of
// the ready set. The request's fields come from the store when it still
// holds the request whose outcome the task is (the earliest by Seq, so a
// duplicate pointing at the task is not taken for it), and are read back from
// the task otherwise. The entry has no id yet; WriteFoundQueue numbers it.
func FoundHandbackEntry(task *model.Task, tasks []model.Task, reqs []FoundRequest) FoundQueueEntry {
	entry := FoundQueueEntry{State: FoundQueueOpen, Location: task.Origin.Location}
	if rep, exp, obs, ok := parseFoundTaskDescription(task.Description); ok {
		entry.Reproduction, entry.Expected, entry.Observed = rep, exp, obs
	} else {
		entry.Reproduction = task.Description
	}
	if req := handbackRequest(task.ID, reqs); req != nil {
		entry.Request = req.ID
		entry.Location, entry.Reproduction, entry.Expected, entry.Observed = req.Location, req.Reproduction, req.Expected, req.Observed
	}
	for i := range tasks {
		if tasks[i].Status != model.StatusDone && slices.Contains(tasks[i].DependsOn, task.ID) {
			entry.Finders = append(entry.Finders, tasks[i].ID)
		}
	}
	return entry
}

// handbackRequest is the earliest request whose outcome is the task, or nil.
func handbackRequest(taskID string, reqs []FoundRequest) *FoundRequest {
	var found *FoundRequest
	for i := range reqs {
		if reqs[i].Outcome == taskID && (found == nil || reqs[i].Seq < found.Seq) {
			found = &reqs[i]
		}
	}
	return found
}
