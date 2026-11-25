package engine

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// FoundSettleRow is one open queue entry the person settles (spec/1.31.0.md
// §6): rejected when Reason is set, otherwise made into a task. Expected and
// Observed fill a task row's Fix fields where the entry has none.
type FoundSettleRow struct {
	Entry    string
	Reason   string
	Expected string
	Observed string
}

// FoundSettled names an entry and the task the person made of it.
type FoundSettled struct {
	Entry string `json:"entry"`
	Task  string `json:"task"`
}

// FoundSettlePlan is what one tp found settle does: the whole queue with
// each settled entry out of the open state, the tasks to append to the task
// file, and the edges that make each made entry's finders depend on its task.
type FoundSettlePlan struct {
	Queue    []FoundQueueEntry
	Tasks    []model.Task
	Edges    []FoundEdge
	Made     []FoundSettled
	Rejected []string
}

// PlanFoundSettle plans one tp found settle over a spec's tasks and queue.
// The batch is all or nothing: an entry that is unknown, not open, or named
// twice, a rejection without a reason, a task row without the Fix fields, or
// a Fix field on a rejection refuses it whole. A made task depends on
// nothing, so its finders' edges onto it close no cycle, and its origin
// names the entry, which keeps it out of found_max (CountFoundTasks).
func PlanFoundSettle(tasks []model.Task, queue []FoundQueueEntry, rows []FoundSettleRow) (FoundSettlePlan, error) {
	if len(rows) == 0 {
		return FoundSettlePlan{}, fmt.Errorf("name at least one entry to settle")
	}
	plan := FoundSettlePlan{Queue: slices.Clone(queue)}
	taken := make(map[string]bool, len(tasks))
	status := make(map[string]string, len(tasks))
	for i := range tasks {
		taken[tasks[i].ID] = true
		status[tasks[i].ID] = tasks[i].Status
	}
	seen := map[string]bool{}
	for _, row := range rows {
		i := slices.IndexFunc(plan.Queue, func(e FoundQueueEntry) bool { return e.ID == row.Entry })
		if err := checkFoundSettleRow(row, plan.Queue, i, seen); err != nil {
			return FoundSettlePlan{}, err
		}
		seen[row.Entry] = true
		entry := &plan.Queue[i]
		if strings.TrimSpace(row.Reason) != "" {
			entry.State, entry.Reason = FoundQueueRejected, strings.TrimSpace(row.Reason)
			plan.Rejected = append(plan.Rejected, entry.ID)
			continue
		}
		task, err := settledFoundTask(entry, &row, taken)
		if err != nil {
			return FoundSettlePlan{}, err
		}
		taken[task.ID] = true
		entry.State, entry.Task = FoundQueueSettled, task.ID
		plan.Tasks = append(plan.Tasks, task)
		plan.Made = append(plan.Made, FoundSettled{Entry: entry.ID, Task: task.ID})
		for _, f := range entry.Finders {
			if s, ok := status[f]; ok && s != model.StatusDone {
				plan.Edges = append(plan.Edges, FoundEdge{Task: f, DependsOn: task.ID})
			}
		}
	}
	return plan, nil
}

// checkFoundSettleRow refuses a row whose entry (at index i of queue, -1
// when absent) cannot be settled, or whose fields do not fit its end.
func checkFoundSettleRow(row FoundSettleRow, queue []FoundQueueEntry, i int, seen map[string]bool) error {
	reject := strings.TrimSpace(row.Reason) != ""
	switch {
	case i < 0:
		return fmt.Errorf("queue entry %s does not exist", row.Entry)
	case seen[row.Entry]:
		return fmt.Errorf("queue entry %s is named twice", row.Entry)
	case queue[i].State != FoundQueueOpen:
		return fmt.Errorf("queue entry %s is %s, not open", row.Entry, queue[i].State)
	case reject && (row.Expected != "" || row.Observed != ""):
		return fmt.Errorf("queue entry %s is rejected, so it takes no --expected or --observed", row.Entry)
	}
	return nil
}

// settledFoundTask is the task the person makes of an open entry: shaped
// like a found task (newFoundTask), its Fix fields the row's or else the
// entry's, its origin naming the entry and its first finder.
func settledFoundTask(entry *FoundQueueEntry, row *FoundSettleRow, taken map[string]bool) (model.Task, error) {
	expected := cmp.Or(strings.TrimSpace(row.Expected), entry.Expected)
	observed := cmp.Or(strings.TrimSpace(row.Observed), entry.Observed)
	if expected == "" || observed == "" {
		return model.Task{}, fmt.Errorf("queue entry %s has no expected and observed behaviour: give --expected %s=<text> and --observed %s=<text>", entry.ID, entry.ID, entry.ID)
	}
	base := "found-" + strings.ToLower(entry.ID)
	id := base
	for n := 2; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	var finder string
	if len(entry.Finders) > 0 {
		finder = entry.Finders[0]
	}
	return model.Task{
		ID:              id,
		Title:           "Fix " + entry.Location,
		Description:     foundTaskDescription(entry.Reproduction, expected, observed),
		Status:          model.StatusOpen,
		Tags:            []string{"found"},
		DependsOn:       []string{},
		EstimateMinutes: foundTaskEstimateMinutes,
		Acceptance:      fmt.Sprintf("- %#q shows %s, where it showed %s", entry.Reproduction, expected, observed),
		SourceSections:  []string{},
		Origin:          &model.Origin{FinderTask: finder, Location: entry.Location, QueueEntry: entry.ID},
	}, nil
}
