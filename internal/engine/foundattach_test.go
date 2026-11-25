package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// touchingTask is a spec task in the given status whose touches are globs.
func touchingTask(id, status string, globs ...string) model.Task {
	return model.Task{ID: id, Status: status, SourceSections: []string{"## 2. Applying requests"}, Touches: globs}
}

func planAttachedTasks(plan *FoundApplyPlan) map[string]string {
	got := map[string]string{}
	for i := range plan.Attached {
		got[plan.Attached[i].Request] = plan.Attached[i].Task
	}
	return got
}

// §2: a request whose location falls inside an open task's files opens
// nothing, does not count against the cap, and is attached to that task; that
// task is its outcome. The cap is full here, so a request that counted would
// be queued.
func TestPlanFoundApply_ARequestInsideAnOpenTasksTouchesAttaches(t *testing.T) {
	tasks := append(foundTasks(5), touchingTask("t1", model.StatusOpen, "internal/x/"))
	req := dedupRequest(1, "internal/x/y.go:12", "go test ./internal/x -run TestY", FoundStateUnapplied)

	plan := PlanFoundApply([]FoundRequest{req}, tasks, nil, 5)

	assert.Empty(t, plan.Opened)
	assert.Empty(t, plan.Entries, "an attachment is not queued past the cap")
	assert.Equal(t, 5, plan.FoundTasks, "an attachment does not count against the cap")
	require.Len(t, plan.Attached, 1)
	assert.Equal(t, FoundAttached{Request: "F1", Task: "t1", Note: model.FoundNote{
		Request:      "F1",
		Location:     "internal/x/y.go:12",
		Reproduction: "go test ./internal/x -run TestY",
		Expected:     "exit 0",
		Observed:     "exit 1",
	}}, plan.Attached[0])
	require.Len(t, plan.Requests, 1)
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
	assert.Equal(t, "t1", plan.Requests[0].Outcome)
}

// With the cap one short, an attachment leaves the last slot to the request
// that opens a task.
func TestPlanFoundApply_AnAttachmentLeavesTheLastSlot(t *testing.T) {
	tasks := append(foundTasks(4), touchingTask("t1", model.StatusOpen, "internal/x/*.go"))
	reqs := []FoundRequest{
		dedupRequest(1, "internal/x/y.go:12", "go test -run TestY", FoundStateUnapplied),
		applyRequest(2, false),
	}

	plan := PlanFoundApply(reqs, tasks, nil, 5)

	assert.Equal(t, map[string]string{"F1": "t1"}, planAttachedTasks(&plan))
	assert.Equal(t, []string{"F2"}, planOpenedRequests(&plan))
	assert.Empty(t, plan.Entries)
	assert.Equal(t, 5, plan.FoundTasks)
}

// Round-4 finding 3: a request inside an open found task's file does not
// attach. A found task's files are its touches, which apply never sets; its
// origin location is not one of them, so a second defect in the same file
// opens its own task instead of merging into the first, which is the merge
// §2's Rejected line refuses. Under the mutant that counts the origin file as
// the found task's files, F2 attaches to found-f1.
func TestPlanFoundApply_ARequestInAnOpenFoundTasksFileOpensItsOwnTask(t *testing.T) {
	first := dedupRequest(1, "x.go:40", "go test -run TestX40", FoundStateUnapplied)
	plan := PlanFoundApply([]FoundRequest{first}, nil, nil, 5)
	require.Len(t, plan.Tasks, 1)
	tasks := plan.Tasks

	second := dedupRequest(2, "x.go:90", "go test -run TestX90", FoundStateUnapplied)
	plan = PlanFoundApply(append(plan.Requests, second), tasks, nil, 5)

	assert.Empty(t, plan.Attached)
	assert.Equal(t, []string{"F2"}, planOpenedRequests(&plan))
}

// Attachment is checked against the task file as it stood before the batch:
// two requests in one file with different reproductions open two tasks, never
// one task and an attachment to it.
func TestPlanFoundApply_TwoDefectsInOneFileOfOneBatchOpenTwoTasks(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "x.go:40", "go test -run TestX40", FoundStateUnapplied),
		dedupRequest(2, "x.go:44", "go test -run TestX44", FoundStateUnapplied),
	}

	plan := PlanFoundApply(reqs, nil, nil, 5)

	assert.Empty(t, plan.Attached)
	assert.Equal(t, []string{"F1", "F2"}, planOpenedRequests(&plan))
}

// Only a task not yet done takes an attachment, the finder's own task never
// does, and a task without touches names no files.
func TestPlanFoundApply_AttachmentSkipsDoneFinderAndTouchlessTasks(t *testing.T) {
	tasks := []model.Task{
		touchingTask("closed", model.StatusDone, "x.go"),
		touchingTask("finder", model.StatusWIP, "x.go"),
		touchingTask("bare", model.StatusOpen),
	}
	req := dedupRequest(1, "x.go:3", "go test -run TestX", FoundStateUnapplied)
	req.Finder.Task = "finder"

	plan := PlanFoundApply([]FoundRequest{req}, tasks, nil, 5)

	assert.Empty(t, plan.Attached)
	assert.Equal(t, []string{"F1"}, planOpenedRequests(&plan))

	tasks = append(tasks, touchingTask("running", model.StatusWIP, "x.go"))
	plan = PlanFoundApply([]FoundRequest{req}, tasks, nil, 5)
	assert.Equal(t, map[string]string{"F1": "running"}, planAttachedTasks(&plan), "a wip task other than the finder is open")
}

// The text brief lists an attached request with the fields it was filed with.
func TestBriefText_ListsTheAttachedRequests(t *testing.T) {
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "done", FoundNotes: []model.FoundNote{
		{Request: "F1", Location: "x.go:3", Reproduction: "go test -run TestX", Expected: "exit 0", Observed: "exit 1"},
		{Request: "F2", Location: "x.go:9", Reproduction: "go test -run TestQ", Question: "which default?", Alternatives: []string{"a", "b"}},
	}}}
	b, err := BuildBrief(&model.TaskFile{Tasks: tasks}, &tasks[0], "", CommitStrategyBuiltin, "", 0, false, true)
	require.NoError(t, err)

	assert.Contains(t, b.Text(), "\nfound_notes (requests filed inside this task's files):\n"+
		"- F1 at x.go:3\n  reproduction: go test -run TestX\n  expected: exit 0\n  observed: exit 1\n"+
		"- F2 at x.go:9\n  reproduction: go test -run TestQ\n  question: which default?\n  alternative: a\n  alternative: b\n")
}
