package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// foundRunningProject is a project root with a .tp directory and the path of
// a task file under it; no run state is written.
func foundRunningProject(t *testing.T) (root, taskFile string) {
	t.Helper()
	root = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o750))
	return root, filepath.Join(root, "spec.tasks.json")
}

// §4: a cycle no run has touched holds only the caller's own unit.
func TestFoundRunningUnits_NoRunStateNamesOnlySelf(t *testing.T) {
	root, taskFile := foundRunningProject(t)

	running, err := FoundRunningUnits(root, taskFile, "me")

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"me": true}, running)
}

// §4, round-4 findings 7 and 11: a probe that could not be performed is not
// "no unit runs". A corrupt run state and a run lock that cannot be stat'ed
// are returned as errors, so apply never reopens a finder a live unit holds.
func TestFoundRunningUnits_AFailedProbeIsAnError(t *testing.T) {
	live := `{"run_id":"r","started_at":"2026-11-18T00:00:00Z","phase":"implement","stop_reason":null,` +
		`"totals":{"units":1,"wall_clock_seconds":1,"spend_usd":0},"units":[` +
		`{"seq":1,"kind":"implement","id":"finder","attempt":1,"exit_code":null,"duration_seconds":null,"spend_usd":null,"log_path":"x"}]}`
	t.Run("corrupt run state", func(t *testing.T) {
		root, taskFile := foundRunningProject(t)
		require.NoError(t, os.WriteFile(RunStatePath(root, taskFile), []byte("{not json"), 0o600))

		_, err := FoundRunningUnits(root, taskFile, "")

		require.Error(t, err)
	})
	t.Run("run lock cannot be probed", func(t *testing.T) {
		root, taskFile := foundRunningProject(t)
		require.NoError(t, os.WriteFile(RunStatePath(root, taskFile), []byte(live), 0o600))
		// .tp/locks is a regular file, so stat'ing the lock under it fails
		// with ENOTDIR rather than ENOENT.
		require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "locks"), nil, 0o600))

		_, err := FoundRunningUnits(root, taskFile, "")

		require.Error(t, err)
	})
}

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
