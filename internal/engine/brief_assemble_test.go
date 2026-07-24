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
