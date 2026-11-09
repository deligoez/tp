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
	bound, err := EpicBoundFiles(root)
	require.NoError(t, err)
	states, err := DeriveEpicStates(g, bound)
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

// TestDeriveEpicStates_DoneNeedsEveryBoundFileAtRelease: with two files bound
// to b, one at release and one whose tasks are all done but whose audit has
// not converged, b is in_progress; it is done once both are at release.
func TestDeriveEpicStates_DoneNeedsEveryBoundFileAtRelease(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	first := writeStatusFile(t, root, "b1", model.StatusDone)
	second := writeStatusFile(t, root, "b2", model.StatusDone)
	recordRounds(t, first, 0, 2, true)
	assert.Equal(t, EpicStatusInProgress, epicStatusIn(t, g, root, "b"))

	recordRounds(t, second, 0, 2, true)
	assert.Equal(t, EpicStatusDone, epicStatusIn(t, g, root, "b"))
}

// TestDeriveEpicStates_ClaimOnBoundEpicIgnored: a claim on an epic with a
// bound file changes nothing (spec/1.5.0.md §3, C6), so a bound epic with only
// open tasks is specced, claimed or not.
func TestDeriveEpicStates_ClaimOnBoundEpicIgnored(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	g.Epics[1].Claimed = &model.EpicClaim{At: time.Now().UTC(), By: "agent-1"}
	writeStatusFile(t, root, "b", model.StatusOpen)
	assert.Equal(t, EpicStatusSpecced, epicStatusIn(t, g, root, "b"))
}

// TestDeriveEpicStates_SpecEditedAfterReleaseFallsBack: a bound spec edited
// after its release falls back to audit with its file, so its epic is in
// progress again until the audit converges (spec/1.5.0.md §3).
func TestDeriveEpicStates_SpecEditedAfterReleaseFallsBack(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	spec := writeStatusFile(t, root, "b", model.StatusDone)
	recordRounds(t, spec, 0, 2, true)
	require.Equal(t, EpicStatusDone, epicStatusIn(t, g, root, "b"))

	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n\nEdited.\n"), 0o600))
	assert.Equal(t, EpicStatusInProgress, epicStatusIn(t, g, root, "b"))
}

// epicDoneAtIn derives every epic of g against the files bound under root and
// returns id's done_at, nil when it has none.
func epicDoneAtIn(t *testing.T, g *model.EpicGraph, root, id string) *time.Time {
	t.Helper()
	bound, err := EpicBoundFiles(root)
	require.NoError(t, err)
	states, err := DeriveEpicStates(g, bound)
	require.NoError(t, err)
	return states[id].DoneAt
}

// TestDeriveEpicStates_TasklessDoneAtIsRecordTime: an epic with no bound file
// is done at its done record's time (spec/1.5.0.md §3); one not done has none.
func TestDeriveEpicStates_TasklessDoneAtIsRecordTime(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	at := epicDoneAtIn(t, g, root, "c")
	require.NotNil(t, at)
	assert.True(t, g.Epics[2].Done.At.Equal(*at))
	assert.Nil(t, epicDoneAtIn(t, g, root, "a"))
}

// recordAuditRoundsAt appends n clean audit rounds recorded at at, each
// matching the spec's current hash, so the file's audit converges at at.
func recordAuditRoundsAt(t *testing.T, specPath string, n int, at time.Time) {
	t.Helper()
	hash, err := SpecHash(specPath)
	require.NoError(t, err)
	st, err := EnsureReviewState(specPath)
	require.NoError(t, err)
	for range n {
		st.AuditRounds = append(st.AuditRounds, ReviewRound{
			Round: len(st.AuditRounds) + 1, Clean: true, SpecHash: hash, RecordedAt: at.Format(time.RFC3339),
		})
	}
	require.NoError(t, SaveReviewState(specPath, st))
}

// TestDeriveEpicStates_BoundDoneAtMovesWithConvergence: a bound epic is done at
// the time of the audit round that brought its last file to release; after a
// spec edit reopens it it has no done_at, and a new convergence moves done_at
// to the new round's time (spec/1.5.0.md §3).
func TestDeriveEpicStates_BoundDoneAtMovesWithConvergence(t *testing.T) {
	root := boundFilesRepo(t)
	g := statusChain()
	first := writeStatusFile(t, root, "b1", model.StatusDone)
	second := writeStatusFile(t, root, "b2", model.StatusDone)
	t1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	recordAuditRoundsAt(t, second, 2, t2)
	recordAuditRoundsAt(t, first, 2, t1)
	at := epicDoneAtIn(t, g, root, "b")
	require.NotNil(t, at)
	assert.True(t, t2.Equal(*at), "the last file to reach release sets done_at: %v", at)

	require.NoError(t, os.WriteFile(first, []byte("# Spec\n\nEdited.\n"), 0o600))
	assert.Nil(t, epicDoneAtIn(t, g, root, "b"), "a reopened epic has no done_at")

	t3 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	recordAuditRoundsAt(t, first, 2, t3)
	at = epicDoneAtIn(t, g, root, "b")
	require.NotNil(t, at)
	assert.True(t, t3.Equal(*at), "done again, done_at moves: %v", at)
}
