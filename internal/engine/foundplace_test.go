package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// finderTask is the wip task a unit works while it files requests.
func finderTask(id string, deps ...string) model.Task {
	if deps == nil {
		deps = []string{}
	}
	return model.Task{ID: id, Status: model.StatusWIP, SourceSections: []string{"## 4. Placement"}, DependsOn: deps}
}

// §4: a non-blocking found task depends on its task finder; one from an audit
// row depends on nothing. Neither changes another task's edges.
func TestPlanFoundApply_ANonBlockingFoundTaskDependsOnItsFinder(t *testing.T) {
	tasks := []model.Task{finderTask("finder")}
	audit := applyRequest(2, false)
	audit.Finder = FoundFinder{AuditRound: 3, RowKey: "spec-coverage:§4"}

	plan := PlanFoundApply([]FoundRequest{applyRequest(1, false), audit}, tasks, nil, 5)

	require.Len(t, plan.Tasks, 2)
	assert.Equal(t, []string{"finder"}, plan.Tasks[0].DependsOn)
	assert.Equal(t, []string{}, plan.Tasks[1].DependsOn, "an audit row's found task depends on nothing")
	assert.Empty(t, plan.Edges)
	assert.Empty(t, plan.Entries)
}

// §4: after a blocking request the finder depends on the found task, and the
// found task does not depend on the finder: placement adds at most one edge.
func TestPlanFoundApply_ABlockingRequestMakesTheFinderDependOnItsFoundTask(t *testing.T) {
	tasks := []model.Task{finderTask("finder", "earlier")}

	plan := PlanFoundApply([]FoundRequest{applyRequest(1, true)}, tasks, nil, 5)

	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, []string{}, plan.Tasks[0].DependsOn)
	assert.Equal(t, []FoundEdge{{Task: "finder", DependsOn: "found-f1"}}, plan.Edges)
}

// §2 acceptance: a blocking duplicate of an applied request opens nothing; its
// finder depends on the kept task.
func TestPlanFoundApply_ABlockingDuplicatesFinderDependsOnTheKeptTask(t *testing.T) {
	first := dedupRequest(1, "x.go:40", "go test -run TestX", FoundStateUnapplied)
	first.Finder.Task = "a"
	plan := PlanFoundApply([]FoundRequest{first}, []model.Task{finderTask("a"), finderTask("b")}, nil, 5)
	require.Len(t, plan.Tasks, 1)
	tasks := append([]model.Task{finderTask("a"), finderTask("b")}, plan.Tasks...)

	dup := dedupRequest(2, "x.go:44", "go  test -run TestX", FoundStateUnapplied)
	dup.Finder.Task = "b"
	dup.Blocks = true
	plan = PlanFoundApply(append(plan.Requests, dup), tasks, nil, 5)

	assert.Empty(t, plan.Tasks)
	assert.Equal(t, []FoundEdge{{Task: "b", DependsOn: "found-f1"}}, plan.Edges)
	assert.Equal(t, "found-f1", plan.Requests[1].Outcome)
}

// §4: a non-blocking duplicate or attachment adds no edge; a blocking
// attachment makes the finder depend on the task it was attached to.
func TestPlanFoundApply_OnlyABlockingAttachmentAddsAnEdge(t *testing.T) {
	tasks := []model.Task{finderTask("finder"), touchingTask("t1", model.StatusOpen, "x.go")}
	quiet := dedupRequest(1, "x.go:3", "go test -run TestA", FoundStateUnapplied)
	dupOfQuiet := dedupRequest(2, "x.go:9", "go test -run TestA", FoundStateUnapplied)
	dupOfQuiet.Finder.Task = "other"

	plan := PlanFoundApply([]FoundRequest{quiet, dupOfQuiet}, tasks, nil, 5)
	assert.Equal(t, map[string]string{"F1": "t1"}, planAttachedTasks(&plan))
	assert.Empty(t, plan.Edges, "a non-blocking attachment and its duplicate add no edge")

	loud := dedupRequest(3, "x.go:3", "go test -run TestB", FoundStateUnapplied)
	loud.Blocks = true
	plan = PlanFoundApply([]FoundRequest{loud}, tasks, nil, 5)
	assert.Equal(t, []FoundEdge{{Task: "finder", DependsOn: "t1"}}, plan.Edges)
}

// §4 and round-4 finding 14 (cycle-entry-shape): finder files a non-blocking
// request, whose found task depends on it, then the same defect as blocking.
// Its edge would close the cycle finder -> found-f1 -> finder, so no edge is
// added and an open queue entry naming the cycle, the outcome task included,
// holds the finder instead. The duplicate keeps found-f1 as its outcome.
func TestPlanFoundApply_ABlockingEdgeThatClosesACycleQueuesInstead(t *testing.T) {
	tasks := []model.Task{finderTask("finder")}
	plan := PlanFoundApply([]FoundRequest{applyRequest(1, false)}, tasks, nil, 5)
	require.Len(t, plan.Tasks, 1)
	tasks = append(tasks, plan.Tasks...)

	again := applyRequest(1, true)
	again.ID, again.Seq = "F2", 2
	again.State = FoundStateUnapplied
	plan = PlanFoundApply(append(plan.Requests, again), tasks, nil, 5)

	assert.Empty(t, plan.Edges, "no edge closes the cycle")
	assert.Empty(t, plan.Tasks)
	require.Len(t, plan.Entries, 1)
	entry := plan.Entries[0]
	assert.Equal(t, FoundQueueOpen, entry.State)
	assert.Equal(t, "F2", entry.Request)
	assert.Equal(t, []string{"finder"}, entry.Finders)
	assert.Contains(t, entry.Question, "finder -> found-f1 -> finder")
	assert.Contains(t, entry.Question, "finder is blocked on found-f1,", "the question names the task the finder waits on")
	assert.Equal(t, again.Reproduction, entry.Reproduction)
	assert.Equal(t, "found-f1", plan.Requests[1].Outcome)
}

// The cycle check follows the whole graph, not one hop: t1 depends on t2,
// which depends on the finder, so a blocking attachment to t1 queues.
func TestPlanFoundApply_TheCycleCheckFollowsLongerPaths(t *testing.T) {
	t1 := touchingTask("t1", model.StatusOpen, "x.go")
	t1.DependsOn = []string{"t2"}
	t2 := touchingTask("t2", model.StatusOpen)
	t2.DependsOn = []string{"finder"}
	req := dedupRequest(1, "x.go:3", "go test -run TestX", FoundStateUnapplied)
	req.Blocks = true

	plan := PlanFoundApply([]FoundRequest{req}, []model.Task{finderTask("finder"), t1, t2}, nil, 5)

	assert.Empty(t, plan.Edges)
	require.Len(t, plan.Entries, 1)
	assert.Contains(t, plan.Entries[0].Question, "finder -> t1 -> t2 -> finder")
}

// A finder that already depends on the outcome gains no second copy of the edge.
func TestPlanFoundApply_AnExistingEdgeIsNotAddedTwice(t *testing.T) {
	req := dedupRequest(1, "x.go:3", "go test -run TestX", FoundStateUnapplied)
	req.Blocks = true

	plan := PlanFoundApply([]FoundRequest{req}, []model.Task{finderTask("finder", "t1"), touchingTask("t1", model.StatusOpen, "x.go")}, nil, 5)

	assert.Empty(t, plan.Edges)
	assert.Empty(t, plan.Entries)
}
