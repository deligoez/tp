package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// seriesDay is the n-th day of November 2026 at hour h, in UTC.
func seriesDay(n, h int) time.Time { return time.Date(2026, 11, n, h, 0, 0, 0, time.UTC) }

// writeSeriesFile writes <base>.md and <base>.tasks.json under root, the task
// file bound to epic, created on 2026-10-01, carrying boundAt when it is not
// nil and the tasks given, and returns the spec's path.
func writeSeriesFile(t *testing.T, root, base, epic string, boundAt *time.Time, tasks ...model.Task) string {
	t.Helper()
	spec := filepath.Join(root, base+".md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))
	for i := range tasks {
		tasks[i].ID = "t" + string(rune('1'+i))
		tasks[i].Title, tasks[i].Acceptance, tasks[i].EstimateMinutes = "T", "a", 5
		tasks[i].DependsOn, tasks[i].SourceSections = []string{}, []string{"x"}
	}
	tf := &model.TaskFile{
		Version: 1, Spec: base + ".md", Epic: epic, BoundAt: boundAt,
		CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Tasks: tasks,
	}
	require.NoError(t, model.WriteTaskFile(filepath.Join(root, base+".tasks.json"), tf))
	return spec
}

// seriesOf derives the summary over g in a repository rooted at root.
func seriesOf(t *testing.T, root string, g *model.EpicGraph) []SeriesDay {
	t.Helper()
	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))
	sum, err := DashboardSummaryOf(tpDir, g)
	require.NoError(t, err)
	return sum.Series
}

// TestDashboardSeries_DatesEachStageFromItsRecord is spec/1.8.0.md §17's
// acceptance: an epic claimed on the 1st, bound on the 2nd, started and
// brought to release on the 3rd counts as claimed, specced and done on those
// days, and the series ends at that last recorded event. The condition epic
// counts in no stage.
func TestDashboardSeries_DatesEachStageFromItsRecord(t *testing.T) {
	t.Parallel()
	root := boundFilesRepo(t)
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "e", Title: "E", Milestone: "M", Root: true, Claimed: &model.EpicClaim{At: seriesDay(1, 9), By: "agent"}},
		{ID: "c", Title: "C", Milestone: "M", Root: true, Condition: true, Acceptance: "x"},
	}}
	bound := seriesDay(2, 9)
	started := seriesDay(3, 8)
	spec := writeSeriesFile(t, root, "e", "e", &bound, model.Task{Status: model.StatusDone, StartedAt: &started})
	recordAuditRoundsAt(t, spec, 3, seriesDay(3, 10))

	assert.Equal(t, []SeriesDay{
		{Date: "2026-11-01", Claimed: 1},
		{Date: "2026-11-02", Specced: 1},
		{Date: "2026-11-03", Done: 1},
	}, seriesOf(t, root, g))
}

// TestDashboardSeries_AnUnknownBindingTimeStaysUnknown is spec/1.8.0.md §18
// row 12: a is bound by a file with no bound_at, b by one with it, and c by
// two files of which only one carries it. b is specced on its day; a and c
// are unknown, never placed on a guessed day. A binding time read from the
// file's created_at, or from c's one file that has it, turns it red.
func TestDashboardSeries_AnUnknownBindingTimeStaysUnknown(t *testing.T) {
	t.Parallel()
	root := boundFilesRepo(t)
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "M", Root: true},
		{ID: "b", Title: "B", Milestone: "M", Root: true},
		{ID: "c", Title: "C", Milestone: "M", Root: true},
	}}
	at := seriesDay(10, 9)
	writeSeriesFile(t, root, "a", "a", nil, model.Task{Status: model.StatusOpen})
	writeSeriesFile(t, root, "b", "b", &at, model.Task{Status: model.StatusOpen})
	writeSeriesFile(t, root, "c1", "c", &at, model.Task{Status: model.StatusOpen})
	writeSeriesFile(t, root, "c2", "c", nil, model.Task{Status: model.StatusOpen})

	assert.Equal(t, []SeriesDay{{Date: "2026-11-10", Specced: 1, Unknown: 2}}, seriesOf(t, root, g))
}

// TestDashboardSeries_AnUnboundUnclaimedEpicCountsAsPlanned: p has no claim
// and no bound file, so on every day of the series it is planned, never
// specced at a zero time nor claimed nor unknown; e's claim alone sets the
// series' one day.
func TestDashboardSeries_AnUnboundUnclaimedEpicCountsAsPlanned(t *testing.T) {
	t.Parallel()
	root := boundFilesRepo(t)
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "e", Title: "E", Milestone: "M", Root: true, Claimed: &model.EpicClaim{At: seriesDay(1, 9), By: "agent"}},
		{ID: "p", Title: "P", Milestone: "M", Root: true},
	}}
	assert.Equal(t, []SeriesDay{{Date: "2026-11-01", Planned: 1, Claimed: 1}}, seriesOf(t, root, g))
}
