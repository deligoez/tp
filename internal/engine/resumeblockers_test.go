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
