package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// writeRollupFile writes a task file under dir with one done task per
// {estimate, actual minutes} pair and one open task, and returns its path.
func writeRollupFile(t *testing.T, dir, name string, pairs ...[2]int) string {
	t.Helper()
	start := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	tf := model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{
		{ID: name + "-open", Title: "T", Status: model.StatusOpen, EstimateMinutes: 50, Acceptance: "a", DependsOn: []string{}},
	}}
	for i, p := range pairs {
		started, closed := start, start.Add(time.Duration(p[1])*time.Minute)
		tf.Tasks = append(tf.Tasks, model.Task{
			ID: name + string(rune('1'+i)), Title: "T", Status: model.StatusDone, EstimateMinutes: p[0],
			Acceptance: "a", DependsOn: []string{}, StartedAt: &started, ClosedAt: &closed,
		})
	}
	path := filepath.Join(dir, name+".tasks.json")
	require.NoError(t, model.WriteTaskFile(path, &tf))
	return path
}

// TestEpicRollup_SumsReportPerTaskPerEpic is spec/1.5.0.md §7: the
// estimate-versus-actual roll-up is what tp report computes per task, summed
// per epic over its bound files. b's two files hold three tracked tasks; the
// open task is not tracked, as in tp report; a has no bound file and no row;
// x of milestone B is not in A's roll-up.
func TestEpicRollup_SumsReportPerTaskPerEpic(t *testing.T) {
	dir := t.TempDir()
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "A", Root: true},
		{ID: "b", Title: "B", Milestone: "A", Root: true},
		{ID: "x", Title: "X", Milestone: "B", Root: true},
	}}
	bound := map[string][]string{
		"b": {writeRollupFile(t, dir, "b1", [2]int{5, 10}, [2]int{10, 6}), writeRollupFile(t, dir, "b2", [2]int{15, 30})},
		"x": {writeRollupFile(t, dir, "x", [2]int{5, 5})},
	}

	rows, err := epicRollup(g, bound, "A")

	require.NoError(t, err)
	assert.Equal(t, []epicRollupRow{{Epic: "b", EstimateMinutes: 30, ActualMinutes: 46, Tracked: 3}}, rows)

	rows, err = epicRollup(g, bound, "B")
	require.NoError(t, err)
	assert.Equal(t, []epicRollupRow{{Epic: "x", EstimateMinutes: 5, ActualMinutes: 5, Tracked: 1}}, rows)
}
