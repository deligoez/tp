package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBrief_MachinePartsAndVerbatimAcceptance(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	acceptance := "Build the brief command. It prints five sections."
	prior := doneTask("prior", "Prior", "- prior done and verified", base)
	tasks := []model.Task{
		prior,
		{
			ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"prior"},
			Acceptance:      acceptance,
			SourceSections:  []string{"## 4. Brief anatomy"},
			SourceLines:     "41-57",
			EstimateMinutes: 15,
		},
	}
	tf := &model.TaskFile{Tasks: tasks}

	b, err := BuildBrief(tf, &tasks[1], "", CommitStrategyBuiltin, "go test ./...", 0, false, false)
	require.NoError(t, err)

	// §4.4: the five machine parts, keyed identity, task, prior_work, close, scope.
	assert.Equal(t, identityOneLine, b.Identity)
	assert.Equal(t, "unit", b.Task.ID)
	assert.Equal(t, ScopeFenceText(), b.Scope)
	assert.Equal(t, CloseRecipeText(CommitStrategyBuiltin, "go test ./..."), b.Close)
	require.NotNil(t, b.PriorWork)
	assert.Equal(t, []string{"prior"}, entryIDs(b.PriorWork.Entries))

	// §4.3: the acceptance text is verbatim from the task file.
	assert.Equal(t, acceptance, b.Task.Acceptance)
	// Anchors carry through to the machine part.
	assert.Equal(t, []string{"## 4. Brief anatomy"}, b.Task.SourceSections)
	assert.Equal(t, "41-57", b.Task.SourceLines)
}

// TestBriefText_MarkerStandsOnItsOwnLine: the marker the plugin's spawn hook
// reads is the text brief's first line, so pasting the brief carries it.
func TestBriefText_MarkerStandsOnItsOwnLine(t *testing.T) {
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "done"}}
	b, err := BuildBrief(&model.TaskFile{Tasks: tasks}, &tasks[0], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	b.Marker = "tp-task: unit spec/x.tasks.json"
	assert.True(t, strings.HasPrefix(b.Text(), "tp-task: unit spec/x.tasks.json\n\n## Identity"), b.Text())

	b.Marker = ""
	assert.True(t, strings.HasPrefix(b.Text(), "## Identity"), "no marker, no line")
}

// TestBriefText_ClaimTokenFollowsTheMarker: the claim token is a line of its
// own under the marker, which a hook older than the token ignores.
func TestBriefText_ClaimTokenFollowsTheMarker(t *testing.T) {
	tasks := []model.Task{{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "done"}}
	b, err := BuildBrief(&model.TaskFile{Tasks: tasks}, &tasks[0], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	b.Marker = "tp-task: unit spec/x.tasks.json"
	b.ClaimToken = "k7f2ab01"
	assert.True(t, strings.HasPrefix(b.Text(), "tp-task: unit spec/x.tasks.json\ntp-claim-token: k7f2ab01\n\n## Identity"), b.Text())
}

// TestCloseRecipeTextFor_TokenSitsInTheWorktreeCommand: under every strategy
// the claim token is part of the worktree close command, and without one the
// recipe names no flag.
func TestCloseRecipeTextFor_TokenSitsInTheWorktreeCommand(t *testing.T) {
	for _, strategy := range []string{CommitStrategyHC, CommitStrategyBuiltin, CommitStrategyAuto} {
		assert.Contains(t, CloseRecipeTextFor(strategy, "", "k7f2ab01", ""), "--claim-token k7f2ab01 --", strategy)
		assert.NotContains(t, CloseRecipeTextFor(strategy, "", "", ""), "--claim-token", strategy)
	}
}

// TestCloseRecipeTextFor_WorktreeCommandNamesTheTaskFile: the worktree close
// names the task file, so it resolves in a repository with several; without
// one the recipe names no --file.
func TestCloseRecipeTextFor_WorktreeCommandNamesTheTaskFile(t *testing.T) {
	for _, strategy := range []string{CommitStrategyHC, CommitStrategyBuiltin} {
		got := CloseRecipeTextFor(strategy, "", "k7f2ab01", "spec/a.tasks.json")
		assert.Contains(t, got, "tp done <id> --file spec/a.tasks.json --commit <sha> --claim-token k7f2ab01 -- ", strategy)
		assert.NotContains(t, CloseRecipeTextFor(strategy, "", "k7f2ab01", ""), "--file", strategy)
	}
}

// TestMainTreeCloseRecipeText_NamesTheMainTree: the close of a task whose
// touches name .tp/config.json sends the unit to the main tree by its root;
// the worktree close says nothing of it.
func TestMainTreeCloseRecipeText_NamesTheMainTree(t *testing.T) {
	const where = "work in the main tree at /repo/main, not in a worktree"
	for _, strategy := range []string{CommitStrategyHC, CommitStrategyBuiltin, CommitStrategyAuto} {
		assert.Contains(t, MainTreeCloseRecipeText(strategy, "", "/repo/main"), where, strategy)
		assert.NotContains(t, CloseRecipeTextFor(strategy, "", "k7f2ab01", "spec/a.tasks.json"), "work in the main tree", strategy)
	}
}
func TestBuildBrief_CompactDropsExcerptAndFileLists(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	prior := doneTask("prior", "Prior", "- prior done", base)
	prior.CommitFiles = []string{"a.go", "b.go"}
	prior.CommitSHAs = []string{"deadbee"}
	tasks := []model.Task{
		prior,
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, DependsOn: []string{"prior"},
			Acceptance: "Do it.", SourceSections: []string{"## 1. X"}, SourceLines: "1-2"},
	}
	tf := &model.TaskFile{Tasks: tasks}

	b, err := BuildBrief(tf, &tasks[1], "", CommitStrategyBuiltin, "go test", 0, false, true)
	require.NoError(t, err)

	// §12.1: spec_excerpt omitted under compact.
	assert.Empty(t, b.Task.SpecExcerpt)
	// The acceptance text, close recipe, and scope prohibitions are kept.
	assert.Equal(t, "Do it.", b.Task.Acceptance)
	assert.Contains(t, b.Close, "tp done")
	assert.Contains(t, b.Scope, "refactor")
	// §12.1: prior-work entries collapse to id and evidence summary only.
	require.Len(t, b.PriorWork.Entries, 1)
	e := b.PriorWork.Entries[0]
	assert.Equal(t, "prior", e.ID)
	assert.Empty(t, e.Title)
	assert.Empty(t, e.CommitFiles)
	assert.Empty(t, e.CommitSHAs)
	assert.Equal(t, "- prior done", e.EvidenceSummary)
}

func TestBuildBrief_FirstUnitHasNoPriorWork(t *testing.T) {
	tasks := []model.Task{
		{ID: "solo", Title: "Solo", Status: model.StatusOpen, Acceptance: "Only task.",
			SourceSections: []string{"## 1. X"}},
	}
	tf := &model.TaskFile{Tasks: tasks}

	b, err := BuildBrief(tf, &tasks[0], "", CommitStrategyBuiltin, "go test", 0, false, false)
	require.NoError(t, err)

	require.NotNil(t, b.PriorWork)
	assert.True(t, b.PriorWork.IsFirstUnit)
	assert.Empty(t, b.PriorWork.Entries)
}

func TestBrief_Text_FiveSectionsInOrderAndVerbatimAcceptance(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen,
			Acceptance: "ACCEPTANCE-MARKER", SourceSections: []string{"## 4. Brief anatomy"}},
	}}
	b, err := BuildBrief(tf, &tf.Tasks[0], "", CommitStrategyHC, "go test ./...", 0, false, false)
	require.NoError(t, err)

	text := b.Text()

	// §4.1: five sections in fixed order.
	idxIdentity := strings.Index(text, "## Identity")
	idxScope := strings.Index(text, "## Scope")
	idxPrior := strings.Index(text, "## Prior work")
	idxUnit := strings.Index(text, "## Your unit")
	idxClose := strings.Index(text, "## How to close")
	require.NotEqual(t, -1, idxIdentity)
	assert.Less(t, idxIdentity, idxScope)
	assert.Less(t, idxScope, idxPrior)
	assert.Less(t, idxPrior, idxUnit)
	assert.Less(t, idxUnit, idxClose)

	// §4.2: identity carries the one-unit-then-stop rule.
	assert.Contains(t, text, identityOneLine)
	// §4.3: verbatim acceptance, not restated.
	assert.Contains(t, text, "ACCEPTANCE-MARKER")
	// §8: the hc close recipe is present.
	assert.Contains(t, text, "--commit <sha>")
}

func TestBrief_Text_CompactShortensIdentityAndScope(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "Do it.",
			SourceSections: []string{"## 1. X"}},
	}}
	b, err := BuildBrief(tf, &tf.Tasks[0], "", CommitStrategyBuiltin, "go test", 0, false, true)
	require.NoError(t, err)
	text := b.Text()

	// §12.1: the scope boundary preamble is dropped under compact…
	assert.NotContains(t, text, "are the boundary of this unit's work")
	// …but the scope-fence prohibitions are never dropped.
	assert.Contains(t, text, "refactor")
	// Identity still carries the reset rule.
	assert.Contains(t, text, identityOneLine)
	// The close recipe and verbatim acceptance remain.
	assert.Contains(t, text, "tp done")
	assert.Contains(t, text, "Do it.")
}
