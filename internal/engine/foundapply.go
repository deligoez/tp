package engine

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// foundTaskEstimateMinutes is a found task's estimate: one defect with its
// reproduction already written, at the top of the atomicity range tp
// validate warns outside of.
const foundTaskEstimateMinutes = 15

// FoundOpened names a request and the found task apply opened for it.
type FoundOpened struct {
	Request string `json:"request"`
	Task    string `json:"task"`
}

// FoundAttached names a request apply attached to an open task, the task, and
// the note it adds there (spec/1.31.0.md §2).
type FoundAttached struct {
	Request string          `json:"request"`
	Task    string          `json:"task"`
	Note    model.FoundNote `json:"-"`
}

// FoundApplyPlan is what tp found apply does to one batch (spec/1.31.0.md
// §2, §8): the found tasks to append to the task file, the open queue entries
// to append to the queue (numbered when written), and the store's requests
// with every unapplied one moved out of the unapplied state.
type FoundApplyPlan struct {
	Requests   []FoundRequest
	Tasks      []model.Task
	Entries    []FoundQueueEntry
	Opened     []FoundOpened
	Attached   []FoundAttached
	Undecided  []string
	Duplicates []FoundDuplicate
	// FoundTasks is the count found_max is measured against once the plan
	// is applied.
	FoundTasks int
}

// CountFoundTasks counts the found tasks a task file holds, the count
// found_max caps (spec/1.31.0.md §8). It is read from the task file, not
// from a run, so a stop and a restart do not reset it.
func CountFoundTasks(tasks []model.Task) int {
	n := 0
	for i := range tasks {
		if tasks[i].Origin != nil {
			n++
		}
	}
	return n
}

// foundApplyShaped reports whether apply opens a task for req: the four Fix
// fields and no question (spec/1.31.0.md §2). Anything else is the decision
// unit's.
func foundApplyShaped(req *FoundRequest) bool {
	return req.FixShaped() && strings.TrimSpace(req.Question) == ""
}

// PlanFoundApply plans one tp found apply over a spec's requests, task file
// tasks and queue (spec/1.31.0.md §2, §8). Duplicates open nothing. A request
// inside an open task's touches is attached to that task, whatever its shape
// and outside the cap (foundAttachTarget). Other Fix-shaped
// requests become found tasks, blocking ones first and then by arrival, while
// the task file holds fewer than foundMax found tasks (0 is uncapped); the
// ones past the cap become open queue entries. Requests that are not
// Fix-shaped wait, undecided, for the decision unit.
func PlanFoundApply(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry, foundMax int) FoundApplyPlan {
	kept, dups := DedupFoundRequests(reqs, tasks, queue)
	plan := FoundApplyPlan{Duplicates: dups, FoundTasks: CountFoundTasks(tasks)}
	slices.SortStableFunc(kept, func(a, b FoundRequest) int {
		if a.Blocks != b.Blocks {
			if a.Blocks {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Seq, b.Seq)
	})

	taken := make(map[string]bool, len(tasks))
	for i := range tasks {
		taken[tasks[i].ID] = true
	}
	outcomes := map[string]FoundRequest{}
	for i := range kept {
		req := kept[i]
		switch attachTo := foundAttachTarget(&req, tasks); {
		case attachTo != "":
			plan.Attached = append(plan.Attached, FoundAttached{Request: req.ID, Task: attachTo, Note: foundNote(&req)})
			req.State = FoundStateApplied
			req.Outcome = attachTo
		case !foundApplyShaped(&req):
			req.State = FoundStateUndecided
			plan.Undecided = append(plan.Undecided, req.ID)
		case foundMax != 0 && plan.FoundTasks >= foundMax:
			req.State = FoundStateApplied
			plan.Entries = append(plan.Entries, pastCapEntry(&req, foundMax))
		default:
			task := newFoundTask(&req, taken)
			taken[task.ID] = true
			plan.Tasks = append(plan.Tasks, task)
			plan.Opened = append(plan.Opened, FoundOpened{Request: req.ID, Task: task.ID})
			plan.FoundTasks++
			req.State = FoundStateApplied
			req.Outcome = task.ID
		}
		outcomes[req.ID] = req
	}
	slices.SortStableFunc(plan.Undecided, func(a, b string) int { return cmp.Compare(outcomes[a].Seq, outcomes[b].Seq) })
	for _, dup := range dups {
		outcomes[dup.Request] = FoundRequest{State: FoundStateApplied, Outcome: cmp.Or(dup.KeptTask, outcomes[dup.KeptRequest].Outcome)}
	}

	plan.Requests = make([]FoundRequest, len(reqs))
	for i := range reqs {
		plan.Requests[i] = reqs[i]
		if o, ok := outcomes[reqs[i].ID]; ok {
			plan.Requests[i].State = o.State
			plan.Requests[i].Outcome = o.Outcome
		}
	}
	return plan
}

// foundAttachTarget is the open task a request is attached to in place of a
// task of its own (spec/1.31.0.md §2), or "" when there is none: the first
// task, in task-file order, that is not done, is not the request's finder, and
// whose touches hold the location's file. A task's files are its touches, so
// a task without touches, a found task among them, names no files: a found
// task's origin location is the defect it fixes, and a second defect in that
// file opens its own task rather than merging into it (§2, Rejected). tasks is
// the task file before the batch, so a task opened in the same batch never
// takes an attachment.
func foundAttachTarget(req *FoundRequest, tasks []model.Task) string {
	file := strings.TrimPrefix(trailingLine.ReplaceAllString(strings.TrimSpace(req.Location), ""), "./")
	for i := range tasks {
		t := &tasks[i]
		if t.Status == model.StatusDone || t.ID == req.Finder.Task || len(t.Touches) == 0 {
			continue
		}
		if TouchesMatch(t.Touches, file) {
			return t.ID
		}
	}
	return ""
}

// foundNote is the note an attached request leaves on its task.
func foundNote(req *FoundRequest) model.FoundNote {
	return model.FoundNote{
		Request:      req.ID,
		Location:     req.Location,
		Reproduction: req.Reproduction,
		Expected:     req.Expected,
		Observed:     req.Observed,
		Question:     req.Question,
		Alternatives: req.Alternatives,
	}
}

// pastCapEntry is the open queue entry of a Fix-shaped request past the cap.
// The queue is committed and the request store is not, so the entry carries
// the Fix fields itself.
func pastCapEntry(req *FoundRequest, foundMax int) FoundQueueEntry {
	return FoundQueueEntry{
		State:        FoundQueueOpen,
		Request:      req.ID,
		Location:     req.Location,
		Question:     fmt.Sprintf("found_max (%d) is reached: open a task for this defect?", foundMax),
		Reproduction: req.Reproduction,
		Expected:     req.Expected,
		Observed:     req.Observed,
	}
}

// newFoundTask is the found task a Fix-shaped request opens: open, anchored
// by origin in place of source_sections, with an id no task in taken has.
func newFoundTask(req *FoundRequest, taken map[string]bool) model.Task {
	base := "found-" + strings.ToLower(req.ID)
	id := base
	for n := 2; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return model.Task{
		ID:              id,
		Title:           "Fix " + req.Location,
		Description:     foundTaskDescription(req.Reproduction, req.Expected, req.Observed),
		Status:          model.StatusOpen,
		Tags:            []string{"found"},
		DependsOn:       []string{},
		EstimateMinutes: foundTaskEstimateMinutes,
		Acceptance:      fmt.Sprintf("- %#q shows %s, where it showed %s", req.Reproduction, req.Expected, req.Observed),
		SourceSections:  []string{},
		Origin: &model.Origin{
			FinderTask: req.Finder.Task,
			AuditRound: req.Finder.AuditRound,
			AuditRow:   req.Finder.RowKey,
			Location:   req.Location,
		},
	}
}
