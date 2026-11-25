package engine

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// foundTasks returns n found tasks already in a task file, the count the cap
// is measured against.
func foundTasks(n int) []model.Task {
	tasks := make([]model.Task, 0, n)
	for i := range n {
		tasks = append(tasks, model.Task{
			ID:     "earlier-" + strconv.Itoa(i),
			Status: model.StatusDone,
			Origin: &model.Origin{FinderTask: "t1", Location: "a.go:" + strconv.Itoa(i+1)},
		})
	}
	return tasks
}

// applyRequest is an unapplied Fix-shaped request at its own location, so
// no two of them are duplicates.
func applyRequest(seq int, blocks bool) FoundRequest {
	req := dedupRequest(seq, "f"+strconv.Itoa(seq)+".go:1", "go test -run T"+strconv.Itoa(seq), FoundStateUnapplied)
	req.Blocks = blocks
	return req
}

func planOpenedRequests(plan *FoundApplyPlan) []string {
	ids := []string{}
	for _, o := range plan.Opened {
		ids = append(ids, o.Request)
	}
	return ids
}

func planQueuedRequests(plan *FoundApplyPlan) []string {
	ids := []string{}
	for i := range plan.Entries {
		ids = append(ids, plan.Entries[i].Request)
	}
	return ids
}

// §10 row "Blocking first within the cap": with one slot left, the blocking
// request filed second takes it. The mutant that applies in arrival order
// opens F1 instead.
func TestPlanFoundApply_BlockingFirstWithinTheCap(t *testing.T) {
	reqs := []FoundRequest{applyRequest(1, false), applyRequest(2, true)}

	plan := PlanFoundApply(reqs, foundTasks(4), nil, 5)

	assert.Equal(t, []string{"F2"}, planOpenedRequests(&plan), "the blocking request takes the last slot")
	assert.Equal(t, []string{"F1"}, planQueuedRequests(&plan), "the non-blocking one past the cap is queued")
}

// §8: the cap counts the found tasks the task file holds, so five already
// there leave no slot for a sixth, whoever applied them.
func TestPlanFoundApply_FiveFoundTasksInTheFileQueueTheSixth(t *testing.T) {
	plan := PlanFoundApply([]FoundRequest{applyRequest(6, false)}, foundTasks(5), nil, 5)

	assert.Empty(t, plan.Opened)
	require.Len(t, plan.Entries, 1)
	entry := plan.Entries[0]
	assert.Equal(t, "F6", entry.Request)
	assert.Equal(t, FoundQueueOpen, entry.State)
	assert.Equal(t, "f6.go:1", entry.Location)
	assert.Equal(t, "go test -run T6", entry.Reproduction, "the committed entry carries the reproduction the git-ignored store holds")
	assert.Equal(t, "exit 0", entry.Expected)
	assert.Equal(t, "exit 1", entry.Observed)
	assert.Contains(t, entry.Question, "found_max")
	assert.Equal(t, 5, plan.FoundTasks)
}

// §8: found_max=0 is uncapped.
func TestPlanFoundApply_FoundMaxZeroOpensTheSixth(t *testing.T) {
	plan := PlanFoundApply([]FoundRequest{applyRequest(6, false)}, foundTasks(5), nil, 0)

	assert.Equal(t, []string{"F6"}, planOpenedRequests(&plan))
	assert.Empty(t, plan.Entries)
	assert.Equal(t, 6, plan.FoundTasks)
}

// The cap counts the tasks opened earlier in the same batch.
func TestPlanFoundApply_TheCapCountsTasksOpenedInTheBatch(t *testing.T) {
	reqs := []FoundRequest{applyRequest(1, false), applyRequest(2, false), applyRequest(3, false)}

	plan := PlanFoundApply(reqs, foundTasks(3), nil, 5)

	assert.Equal(t, []string{"F1", "F2"}, planOpenedRequests(&plan))
	assert.Equal(t, []string{"F3"}, planQueuedRequests(&plan))
}

// Round-4 finding 1: a Fix-shaped request past the cap is queued, and one
// that is not Fix-shaped goes to the decision unit, cap or no cap. A request
// with all four Fix fields and a question is not Fix-shaped (§2).
func TestPlanFoundApply_PastTheCapQueuesAndANonFixShapedRequestIsUndecided(t *testing.T) {
	question := applyRequest(2, false)
	question.Expected, question.Observed = "", ""
	question.Question = "which default?"
	both := applyRequest(3, false)
	both.Question = "or keep it?"
	reqs := []FoundRequest{applyRequest(1, false), question, both}

	plan := PlanFoundApply(reqs, foundTasks(5), nil, 5)

	assert.Equal(t, []string{"F1"}, planQueuedRequests(&plan))
	assert.Equal(t, []string{"F2", "F3"}, plan.Undecided)
	assert.Empty(t, plan.Opened)
	states := map[string]string{}
	for i := range plan.Requests {
		states[plan.Requests[i].ID] = plan.Requests[i].State
	}
	assert.Equal(t, map[string]string{"F1": FoundStateApplied, "F2": FoundStateUndecided, "F3": FoundStateUndecided}, states)
}

func TestPlanFoundApply_AFoundTaskCarriesTheRequestAndValidates(t *testing.T) {
	req := applyRequest(1, false)
	existing := []model.Task{{ID: "found-f1", Status: model.StatusDone, Origin: &model.Origin{Location: "z.go:1"}}}

	plan := PlanFoundApply([]FoundRequest{req}, existing, nil, 5)

	require.Len(t, plan.Tasks, 1)
	task := plan.Tasks[0]
	assert.NotEqual(t, "found-f1", task.ID, "a found task's id never collides with one in the file")
	assert.Equal(t, model.StatusOpen, task.Status)
	assert.Equal(t, &model.Origin{FinderTask: "finder", Location: "f1.go:1"}, task.Origin)
	assert.Empty(t, task.SourceSections)
	assert.Equal(t, []string{"finder"}, task.DependsOn, "a non-blocking found task depends on its finder (§4)")
	assert.Contains(t, task.Acceptance, req.Reproduction)
	assert.Nil(t, ValidateTaskEntry(&task, map[string]bool{"finder": true}))
	assert.Equal(t, []FoundOpened{{Request: "F1", Task: task.ID}}, plan.Opened)
	assert.Equal(t, task.ID, plan.Requests[0].Outcome)
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
}

func TestPlanFoundApply_AnAuditFinderBecomesTheOrigin(t *testing.T) {
	req := applyRequest(1, false)
	req.Finder = FoundFinder{AuditRound: 3, RowKey: "go-safety:file-a.go"}

	plan := PlanFoundApply([]FoundRequest{req}, nil, nil, 5)

	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, &model.Origin{AuditRound: 3, AuditRow: "go-safety:file-a.go", Location: "f1.go:1"}, plan.Tasks[0].Origin)
}

// A duplicate in the batch opens nothing, leaves the unapplied state, and
// takes the kept request's found task as its outcome.
func TestPlanFoundApply_ADuplicateInTheBatchSharesTheKeptTask(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "x.go:40", "go test -run X", FoundStateUnapplied),
		dedupRequest(2, "x.go:44", "go test -run X", FoundStateUnapplied),
	}

	plan := PlanFoundApply(reqs, nil, nil, 5)

	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, []FoundDuplicate{{Request: "F2", KeptRequest: "F1"}}, plan.Duplicates)
	for i := range plan.Requests {
		assert.Equal(t, FoundStateApplied, plan.Requests[i].State, plan.Requests[i].ID)
		assert.Equal(t, plan.Tasks[0].ID, plan.Requests[i].Outcome, plan.Requests[i].ID)
	}
}

// Requests already applied or undecided are left as they are.
func TestPlanFoundApply_OnlyUnappliedRequestsChange(t *testing.T) {
	done := dedupRequest(1, "a.go:1", "go test -run A", FoundStateApplied)
	done.Outcome = "found-f1"
	waiting := dedupRequest(2, "b.go:1", "go test -run B", FoundStateUndecided)
	reqs := []FoundRequest{done, waiting}

	plan := PlanFoundApply(reqs, nil, nil, 5)

	assert.Equal(t, reqs, plan.Requests)
	assert.Empty(t, plan.Tasks)
	assert.Empty(t, plan.Entries)
}
