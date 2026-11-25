package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// §4: a blocking request queued past found_max has no task outcome, so its
// entry holds the finder; a non-blocking one's entry holds nothing.
func TestPlanFoundApply_ABlockingRequestPastTheCapHoldsItsFinder(t *testing.T) {
	blocking, plain := applyRequest(1, true), applyRequest(2, false)
	blocking.Finder.Task, plain.Finder.Task = "finder", "finder"
	tasks := []model.Task{finderTask("finder"), {ID: "found-x", Status: model.StatusOpen, Origin: &model.Origin{Location: "x.go:1"}}}

	plan := PlanFoundApply([]FoundRequest{blocking, plain}, tasks, nil, 1)

	require.Len(t, plan.Entries, 2)
	assert.Equal(t, []string{"finder"}, plan.Entries[0].Finders)
	assert.Empty(t, plan.Entries[1].Finders)
}

// Round-4 findings 7 and 11: only a wip finder of a blocking request that
// waits and that no unit holds is returned to open. A done finder, a finder
// whose wait is over, a spawn-claimed one, a running one and a task that
// filed no blocking request are left as they are.
func TestFoundFindersToReopen_ReturnsOnlyAWaitingFinderNoUnitHolds(t *testing.T) {
	var reqs []FoundRequest
	for i, id := range []string{"queued", "edged", "done", "free", "claimed", "running"} {
		req := applyRequest(i+1, true)
		req.Finder.Task = id
		reqs = append(reqs, req)
	}
	plainReq := applyRequest(9, false)
	plainReq.Finder.Task = "plain"
	reqs = append(reqs, plainReq)
	claimed := finderTask("claimed", "fix")
	claimed.ClaimedBy = &model.ClaimedBy{Spawn: "toolu_1"}
	doneTask := finderTask("done", "fix")
	doneTask.Status = model.StatusDone
	tasks := []model.Task{
		{ID: "fix", Status: model.StatusOpen},
		finderTask("queued"), finderTask("edged", "fix"), doneTask, finderTask("free"),
		claimed, finderTask("running", "fix"), finderTask("plain", "fix"),
	}
	queue := []FoundQueueEntry{
		{State: FoundQueueOpen, Finders: []string{"queued"}},
		{State: FoundQueueDecided, Finders: []string{"free"}},
	}

	got := FoundFindersToReopen(reqs, tasks, queue, map[string]bool{"running": true})

	assert.Equal(t, []string{"queued", "edged"}, got)
}
