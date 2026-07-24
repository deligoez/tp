package engine

import (
	"testing"
	"time"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptrStr builds the pointer fields a done task carries.
func ptrStr(s string) *string { return &s }

// doneTask builds a done model.Task with the given id and closed time.
func doneTask(id, title, reason string, closed time.Time, deps ...string) model.Task {
	return model.Task{
		ID:           id,
		Title:        title,
		Status:       model.StatusDone,
		DependsOn:    deps,
		ClosedAt:     &closed,
		ClosedReason: ptrStr(reason),
	}
}

func TestSelectPriorWork_TransitiveDepsInTopoOrder(t *testing.T) {
	// c → b → a (each done); the unit depends on c, so all three are prior work,
	// in dependency-safe order a, b, c (foundation first).
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("a", "A", "- a done", base.Add(0)),
		doneTask("b", "B", "- b done", base.Add(1*time.Hour), "a"),
		doneTask("c", "C", "- c done", base.Add(2*time.Hour), "b"),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"c"}},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	require.Len(t, res.Entries, 3)
	assert.Equal(t, []string{"a", "b", "c"}, []string{res.Entries[0].ID, res.Entries[1].ID, res.Entries[2].ID})
	assert.False(t, res.IsFirstUnit)
	assert.Zero(t, res.OmittedCount)
}

func TestSelectPriorWork_WalksThroughNotYetDoneDeps(t *testing.T) {
	// unit → b(open) → a(done): a is a transitive done dependency even though b
	// is not done, because the unit transitively depends on a through b.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("a", "A", "- a done", base),
		{ID: "b", Title: "B", Status: model.StatusOpen, DependsOn: []string{"a"}},
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"b"}},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, entryIDs(res.Entries))
}

func TestSelectPriorWork_RecencyFillsAfterDeps(t *testing.T) {
	// 1 dep + 6 recent done tasks. Default recency is 5 → 1 dep + 5 recency.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("dep", "Dep", "- dep", base),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"dep"}},
	}
	for i := 0; i < 6; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('0'+i)), "R", "- recent", base.Add(time.Duration(i)*time.Hour)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	require.Len(t, res.Entries, 6)
	// dep first, then the 5 most recent (r5..r1), oldest recent (r0) omitted.
	assert.Equal(t, "dep", res.Entries[0].ID)
	assert.Equal(t, []string{"r5", "r4", "r3", "r2", "r1"}, entryIDs(res.Entries[1:]))
	assert.Equal(t, 1, res.OmittedCount, "one recency entry dropped by the default 5-count")
}

func TestSelectPriorWork_RecencyOrderedMostRecentFirst(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("old", "Old", "- old", base),
		doneTask("mid", "Mid", "- mid", base.Add(1*time.Hour)),
		doneTask("new", "New", "- new", base.Add(2*time.Hour)),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"new", "mid", "old"}, entryIDs(res.Entries))
}

func TestSelectPriorWork_TotalCapShrinksRecency(t *testing.T) {
	// 10 deps + 8 recency candidates: room = 12-10 = 2, so recency = min(5,2) = 2.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	unitDeps := make([]string, 10)
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen}}
	for i := 0; i < 10; i++ {
		id := "d" + string(rune('a'+i))
		unitDeps[i] = id
		tasks = append(tasks, doneTask(id, "D", "- d", base.Add(time.Duration(i)*time.Second)))
	}
	tasks[0].DependsOn = unitDeps
	for i := 0; i < 8; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('a'+i)), "R", "- r", base.Add(time.Duration(100+i)*time.Second)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	require.Len(t, res.Entries, 12, "deps(10) + recency(2) = 12, the total cap")
	assert.Equal(t, 6, res.OmittedCount, "8 recency candidates - 2 shown = 6 dropped by the cap")
}

func TestSelectPriorWork_DepsAlwaysIncludedBeyondCap(t *testing.T) {
	// 13 deps (beyond the 12 cap) + 4 recency candidates. All 13 deps stay; the
	// total exceeds 12 and recency is squeezed to zero (room = 12-13 < 0).
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	unitDeps := make([]string, 13)
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen}}
	for i := 0; i < 13; i++ {
		id := "d" + string(rune('a'+i))
		unitDeps[i] = id
		tasks = append(tasks, doneTask(id, "D", "- d", base.Add(time.Duration(i)*time.Second)))
	}
	tasks[0].DependsOn = unitDeps
	for i := 0; i < 4; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('a'+i)), "R", "- r", base.Add(time.Duration(100+i)*time.Second)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	require.Len(t, res.Entries, 13, "all deps always included beyond the cap")
	assert.Equal(t, 4, res.OmittedCount, "all 4 recency candidates dropped (room was 0)")
}

func TestSelectPriorWork_DedupsDepFromRecency(t *testing.T) {
	// "shared" is both a done dep and the most recent task; it appears once.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("shared", "Shared", "- shared", base.Add(10*time.Hour)),
		doneTask("other", "Other", "- other", base),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"shared"}},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"shared", "other"}, entryIDs(res.Entries), "dep first; recency excludes the dep")
}

func TestSelectPriorWork_ExcludesSelfAndNonDone(t *testing.T) {
	tasks := []model.Task{
		{ID: "wip", Title: "Wip", Status: model.StatusWIP},
		{ID: "open", Title: "Open", Status: model.StatusOpen},
		{ID: "unit", Title: "Unit", Status: model.StatusOpen},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	assert.Empty(t, res.Entries, "non-done tasks are never prior work")
}

func TestSelectPriorWork_FirstUnit(t *testing.T) {
	// No done tasks at all → the first unit of the project.
	tasks := []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen},
		{ID: "later", Title: "Later", Status: model.StatusOpen, DependsOn: []string{"unit"}},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, false)
	require.NoError(t, err)
	assert.Empty(t, res.Entries)
	assert.True(t, res.IsFirstUnit, "no done deps and no done candidates → first unit")
	assert.Zero(t, res.OmittedCount)
}

func TestSelectPriorWork_PriorZeroSuppressesRecencyNotFirstUnit(t *testing.T) {
	// Done work exists but --prior 0 suppresses recency entirely. It is NOT the
	// first unit; the omitted-count states how many were suppressed.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("done1", "D1", "- one", base),
		doneTask("done2", "D2", "- two", base.Add(1*time.Hour)),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen},
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 0, true)
	require.NoError(t, err)
	assert.Empty(t, res.Entries, "--prior 0 suppresses all recency")
	assert.False(t, res.IsFirstUnit, "done work exists, so it is not the first unit")
	assert.Equal(t, 2, res.OmittedCount)
}

func TestSelectPriorWork_PriorOverrideRemovesTotalCap(t *testing.T) {
	// 2 deps + 8 candidates, --prior 20 → 2 deps + 8 recency (no 12-cap).
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		doneTask("d1", "D1", "- d", base),
		doneTask("d2", "D2", "- d", base.Add(1*time.Second)),
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"d1", "d2"}},
	}
	for i := 0; i < 8; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('a'+i)), "R", "- r", base.Add(time.Duration(10+i)*time.Second)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 20, true)
	require.NoError(t, err)
	require.Len(t, res.Entries, 10, "deps(2) + recency(8), the 12-cap is overridden")
	assert.Zero(t, res.OmittedCount, "all 8 candidates fit under --prior 20")
}

func TestSelectPriorWork_PriorOverrideSmallerThanCandidates(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen},
	}
	for i := 0; i < 6; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('a'+i)), "R", "- r", base.Add(time.Duration(i)*time.Second)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 3, true)
	require.NoError(t, err)
	require.Len(t, res.Entries, 3)
	assert.Equal(t, 3, res.OmittedCount, "6 candidates - 3 shown")
}

func TestSelectPriorWork_PriorOverrideWithLargeDeps(t *testing.T) {
	// --prior with deps beyond 12: deps always included, recency = prior count.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	unitDeps := make([]string, 13)
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen}}
	for i := 0; i < 13; i++ {
		id := "d" + string(rune('a'+i))
		unitDeps[i] = id
		tasks = append(tasks, doneTask(id, "D", "- d", base.Add(time.Duration(i)*time.Second)))
	}
	tasks[0].DependsOn = unitDeps
	for i := 0; i < 3; i++ {
		tasks = append(tasks, doneTask("r"+string(rune('a'+i)), "R", "- r", base.Add(time.Duration(100+i)*time.Second)))
	}
	tf := &model.TaskFile{Tasks: tasks}

	res, err := SelectPriorWork(tf, "unit", 2, true)
	require.NoError(t, err)
	require.Len(t, res.Entries, 15, "deps(13) + recency(2)")
	assert.Equal(t, 1, res.OmittedCount, "3 candidates - 2 shown")
}

func TestSelectPriorWork_TaskNotFound(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "a", Status: model.StatusOpen}}}
	_, err := SelectPriorWork(tf, "missing", 0, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestValidatePriorCount_OutOfRange(t *testing.T) {
	assert.Error(t, ValidatePriorCount(-1, true))
	assert.Error(t, ValidatePriorCount(21, true))
	assert.Error(t, ValidatePriorCount(100, true))
}

func TestValidatePriorCount_InRange(t *testing.T) {
	for _, n := range []int{0, 1, 10, 20} {
		assert.NoError(t, ValidatePriorCount(n, true), "prior %d is in range", n)
	}
}

func TestValidatePriorCount_UnsetAlwaysPasses(t *testing.T) {
	// When --prior is absent the value is ignored, so even a wild int passes.
	assert.NoError(t, ValidatePriorCount(999, false))
	assert.NoError(t, ValidatePriorCount(-5, false))
}

func TestBuildPriorWorkEntry_FirstLineOfClosedReason(t *testing.T) {
	t.Run("multiline returns first line only", func(t *testing.T) {
		t1 := doneTask("x", "X", "- evidence line one\n- evidence line two\nmore", time.Now())
		e := buildPriorWorkEntry(&t1)
		assert.Equal(t, "- evidence line one", e.EvidenceSummary)
	})
	t.Run("single line returned whole", func(t *testing.T) {
		t1 := doneTask("x", "X", "- only line", time.Now())
		e := buildPriorWorkEntry(&t1)
		assert.Equal(t, "- only line", e.EvidenceSummary)
	})
	t.Run("nil closed reason yields empty summary", func(t *testing.T) {
		t1 := model.Task{ID: "x", Title: "X", Status: model.StatusDone}
		e := buildPriorWorkEntry(&t1)
		assert.Empty(t, e.EvidenceSummary)
	})
}

func TestBuildPriorWorkEntry_CommitFilesCappedAtFiveWithMore(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go"}
	t1 := doneTask("x", "X", "- done", time.Now())
	t1.CommitFiles = files
	e := buildPriorWorkEntry(&t1)

	assert.Equal(t, files[:5], e.CommitFiles, "first five paths shown")
	assert.Equal(t, 2, e.CommitFilesMore, "7 total - 5 shown = 2 more")
	assert.Equal(t, 7, e.CommitFilesTotal, "true total is the array length")
}

func TestBuildPriorWorkEntry_CommitFilesUnderFiveNoMore(t *testing.T) {
	files := []string{"a.go", "b.go"}
	t1 := doneTask("x", "X", "- done", time.Now())
	t1.CommitFiles = files
	e := buildPriorWorkEntry(&t1)

	assert.Equal(t, files, e.CommitFiles)
	assert.Zero(t, e.CommitFilesMore)
	assert.Equal(t, 2, e.CommitFilesTotal)
}

func TestBuildPriorWorkEntry_CommitFilesTotalUsesStoredCap(t *testing.T) {
	// 50-path cap (§6.4) applied: stored array is 50, true total is 60.
	files := make([]string, 50)
	for i := range files {
		files[i] = "f" + string(rune('a'+i%26)) + ".go"
	}
	t1 := doneTask("x", "X", "- done", time.Now())
	t1.CommitFiles = files
	t1.CommitFilesTotal = 60
	e := buildPriorWorkEntry(&t1)

	assert.Len(t, e.CommitFiles, 5, "entry shows five paths")
	assert.Equal(t, 55, e.CommitFilesMore, "60 true total - 5 shown = 55 more")
	assert.Equal(t, 60, e.CommitFilesTotal, "true total is the stored cap value")
}

func TestBuildPriorWorkEntry_CommitSHAs(t *testing.T) {
	t1 := doneTask("x", "X", "- done", time.Now())
	t1.CommitSHAs = []string{"abc123", "def456"}
	e := buildPriorWorkEntry(&t1)
	assert.Equal(t, []string{"abc123", "def456"}, e.CommitSHAs)
}

func TestBuildPriorWorkEntry_NoCommits(t *testing.T) {
	// A --covered-by close records no commits; the entry omits them cleanly.
	t1 := doneTask("x", "X", "- done", time.Now())
	e := buildPriorWorkEntry(&t1)
	assert.Empty(t, e.CommitSHAs)
	assert.Empty(t, e.CommitFiles)
	assert.Zero(t, e.CommitFilesMore)
	assert.Zero(t, e.CommitFilesTotal)
}

// entryIDs is a small test helper extracting the ordered entry ids.
func entryIDs(entries []PriorWorkEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}
