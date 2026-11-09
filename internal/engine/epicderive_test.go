package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusChain is §10's chain a→b→c→d with c and d done: each epic depends on
// the next, and b, the epic under test, starts with no claim and no record.
func statusChain() *model.EpicGraph {
	done := &model.EpicDone{At: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}
	dep := func(id string) []model.EpicDep { return []model.EpicDep{{ID: id, Why: "needs it"}} }
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "M", DependsOn: dep("b")},
		{ID: "b", Title: "B", Milestone: "M", DependsOn: dep("c")},
		{ID: "c", Title: "C", Milestone: "M", DependsOn: dep("d"), Done: done},
		{ID: "d", Title: "D", Milestone: "M", Root: true, Done: done},
	}}
}

// writeStatusFile writes <base>.md and <base>.tasks.json under root, the task
// file bound to epic b and holding one task per status given, and returns the
// spec's path. Called again for the same base it rewrites the task file only.
func writeStatusFile(t *testing.T, root, base string, statuses ...string) string {
	t.Helper()
	spec := filepath.Join(root, base+".md")
	if _, err := os.Stat(spec); os.IsNotExist(err) {
		require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))
	}
	tasks := make([]string, len(statuses))
	for i, s := range statuses {
		tasks[i] = `{"id":"t` + string(rune('1'+i)) + `","title":"T","status":"` + s +
			`","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}`
	}
	body := `{"version":1,"spec":"` + filepath.Base(spec) + `","epic":"b","tasks":[` + strings.Join(tasks, ",") + `]}`
	require.NoError(t, os.WriteFile(filepath.Join(root, base+".tasks.json"), []byte(body), 0o600))
	return spec
}

// epicStatusIn derives every epic of g against the files bound under root and
// returns id's status.
func epicStatusIn(t *testing.T, g *model.EpicGraph, root, id string) string {
	t.Helper()
	states, err := DeriveEpicStates(g, EpicBoundFiles(root))
	require.NoError(t, err)
	return states[id].Status
}

// TestDeriveEpicStates_BMovesAsItsFileMoves is spec/1.5.0.md §10 row 2: b is
// planned, claimed, specced once a file binds it, in_progress once a task is
// wip and still once every task is done, and done only once the file's audit
// converges. A status that read done from the tasks alone fails.
func TestDeriveEpicStates_BMovesAsItsFileMoves(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	assert.Equal(t, EpicStatusPlanned, epicStatusIn(t, g, root, "b"))

	g.Epics[1].Claimed = &model.EpicClaim{At: time.Now().UTC(), By: "agent-1"}
	assert.Equal(t, EpicStatusClaimed, epicStatusIn(t, g, root, "b"))

	writeStatusFile(t, root, "b", model.StatusOpen, model.StatusOpen)
	assert.Equal(t, EpicStatusSpecced, epicStatusIn(t, g, root, "b"))

	writeStatusFile(t, root, "b", model.StatusWIP, model.StatusOpen)
	assert.Equal(t, EpicStatusInProgress, epicStatusIn(t, g, root, "b"))

	spec := writeStatusFile(t, root, "b", model.StatusDone, model.StatusDone)
	assert.Equal(t, EpicStatusInProgress, epicStatusIn(t, g, root, "b"), "every task done, audit not converged")

	recordRounds(t, spec, 0, 2, true)
	assert.Equal(t, EpicStatusDone, epicStatusIn(t, g, root, "b"))
	assert.Equal(t, EpicStatusDone, epicStatusIn(t, g, root, "c"), "a taskless epic is done by its record")
}

