package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blockerCodes(bs []Blocker) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Code
	}
	return out
}

func TestBuildBlockers_UnexplainedChangesAgentClearable(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, Changes: []string{"a.txt", "b.txt"}, TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "unexplained-changes", bs[0].Code)
	assert.Equal(t, ClassAgentClearable, bs[0].Class)
	assert.NotEmpty(t, bs[0].Message)
	assert.Equal(t, 2, bs[0].Data["count"])
}

func TestBuildBlockers_NoReadyTaskEscalate(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "blocked", Status: model.StatusOpen, DependsOn: []string{"missing"}}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "no-ready-task", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, []string{"missing"}, bs[0].Data["blocked_by"])
}

func TestBuildBlockers_BudgetExhaustedAtCap(t *testing.T) {
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseReview, ReviewRounds: 4, ReviewMaxRounds: 4, TaskFile: &model.TaskFile{}})
	require.Len(t, bs, 1)
	assert.Equal(t, "review-budget-exhausted", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, 4, bs[0].Data["cap"])
}

func TestBuildBlockers_BudgetZeroNeverFires(t *testing.T) {
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseReview, ReviewRounds: 9, ReviewMaxRounds: 0, TaskFile: &model.TaskFile{}})
	assert.Empty(t, bs)
}

func TestBuildBlockers_SpecStaleEscalate(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, ReviewStale: true, SpecPath: "spec.md", TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "spec-stale", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, "spec.md", bs[0].Data["spec"])
}

// TestBuildBlockers_SpecStaleNamesTheClearingSequence: the spec-stale message
// fills this spec's path into the two steps that clear it — emit a review round
// over the new text, then record it — in both phases the blocker fires in.
func TestBuildBlockers_SpecStaleNamesTheClearingSequence(t *testing.T) {
	t.Parallel()
	const spec = "docs/specs/feature.md"
	for _, tc := range []struct {
		phase string
		tf    *model.TaskFile
	}{
		{PhaseImplement, &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusOpen}}}},
		{PhaseAudit, &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusDone}}}},
	} {
		bs := BuildBlockers(&BlockerInputs{Phase: tc.phase, ReviewStale: true, SpecPath: spec, TaskFile: tc.tf})
		require.Len(t, bs, 1, tc.phase)
		assert.Equal(t, "spec-stale", bs[0].Code, tc.phase)
		assert.Equal(t, spec, bs[0].Data["spec"], tc.phase)
		assert.Contains(t, bs[0].Message, "tp review "+spec+",", "%s: names the emission", tc.phase)
		assert.Contains(t, bs[0].Message, "tp review "+spec+" --record <file>", "%s: names the recording", tc.phase)
	}
}

func TestBuildBlockers_FixedEmissionOrder(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, Changes: []string{"x"}, ReviewStale: true, SpecPath: "s.md", TaskFile: tf})
	assert.Equal(t, []string{"unexplained-changes", "spec-stale"}, blockerCodes(bs))
}

func TestBuildBlockers_CleanIsEmptyNotNil(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "ready", Status: model.StatusOpen}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})
	assert.NotNil(t, bs)
	assert.Empty(t, bs)
}

// TestBudgetBlockerMessage_NamesTheOperatorOnlyForABlockingFixed: at the cap
// the blocker names the operator's two exits only when a blocking finding was
// marked fixed there. With none, the remaining findings are dispositioned like
// any other, and the message says so; with one, it says a fix no round re-read
// is the operator's to accept or to verify with one more round.
func TestBudgetBlockerMessage_NamesTheOperatorOnlyForABlockingFixed(t *testing.T) {
	t.Parallel()
	none := budgetBlockerMessage("review", 3, 0)
	assert.Contains(t, none, "disposition the remaining findings")
	assert.NotContains(t, none, "marked fixed")

	one := budgetBlockerMessage("review", 3, 1)
	assert.Contains(t, one, "1 blocking finding")
	assert.Contains(t, one, "marked fixed")
	assert.NotContains(t, one, "disposition the remaining findings")
}
