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

