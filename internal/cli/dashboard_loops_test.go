package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summaryLoops starts the dashboard on dir and returns the first summary's
// open_loops and stale_loops figures.
func summaryLoops(t *testing.T, dir string) (open, stale int) {
	t.Helper()
	srv := startDashboard(t, dir, freePort(t))
	ev := waitForEvent(t, openEvents(t, srv), "summary")
	var sum struct {
		Plan struct {
			Figures struct {
				OpenLoops  int `json:"open_loops"`
				StaleLoops int `json:"stale_loops"`
			} `json:"figures"`
		} `json:"plan"`
	}
	require.NoError(t, json.Unmarshal([]byte(ev.data), &sum), ev.data)
	return sum.Plan.Figures.OpenLoops, sum.Plan.Figures.StaleLoops
}

// staleSlice returns a project under workflow whose --epic b slice recorded
// one round of rows and whose b changed after it.
func staleSlice(t *testing.T, workflow, rows string) string {
	t.Helper()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, workflow)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, rows, "--epic", "b")
	editEpicGraph(t, dir, `"title": "B"`, `"title": "B renamed"`)
	return dir
}

// TestDashboard_StaleLoopsCountOnlyWhatAReReviewSettles: a stale slice is
// stale_loops only when it would otherwise be done and is under its cap, so
// a re-review alone settles it. A clean uncapped one is counted; one whose
// last round left a blocking finding open needs a real round, and a capped
// one waits on the operator's grant, so neither is; every one stays in
// open_loops.
func TestDashboard_StaleLoopsCountOnlyWhatAReReviewSettles(t *testing.T) {
	t.Parallel()
	high := `{"severity":"high","finding":"f","location":"epic:b","evidence":"b"}` + "\n"
	for name, c := range map[string]struct {
		workflow, rows string
		stale          int
	}{
		"clean and under its cap":  {`{"review_clean_rounds": 1}`, "", 1},
		"an open blocking finding": {`{"review_clean_rounds": 1}`, high, 0},
		"clean at its cap":         {`{"review_clean_rounds": 1, "review_max_rounds": 1}`, "", 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := staleSlice(t, c.workflow, c.rows)
			if name == "clean at its cap" {
				require.True(t, *epicGrantStatus(t, dir).Grantable, "the capped slice is the operator's to grant")
			}
			open, stale := summaryLoops(t, dir)
			assert.Equal(t, 1, open, "every stale slice is an open loop")
			assert.Equal(t, c.stale, stale)
		})
	}
}
