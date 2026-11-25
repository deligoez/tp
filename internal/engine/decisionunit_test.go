package engine

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// spec/1.31.0.md §5: the decision unit's prompt says it never makes a rule-9
// decision and answers human when in doubt; it files no requests itself.
func TestUnitPrompt_TheDecisionUnitNeverMakesARule9DecisionAndAnswersHumanWhenInDoubt(t *testing.T) {
	prompt := UnitPrompt(UnitDecision, UnitTarget{ID: "F1,F2"})

	assert.Contains(t, prompt, "Run `tp found decide F1,F2` first")
	assert.Contains(t, prompt, "You never make a rule-9 decision")
	assert.Contains(t, prompt, "when in doubt, answer `human`")
	assert.NotContains(t, prompt, "tp found add", "the decision unit is never a finder")
	assert.NotContains(t, UnitPrompt(UnitImplement, UnitTarget{}), "rule-9", "only the decision unit is told this")
}

// decisionFixture writes a task file and the given requests, F1..Fn in
// arrival order, each in its state, and returns the task file.
func decisionFixture(t *testing.T, states ...string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, UpdateFoundRequests(dir, taskFile, func(reqs []FoundRequest) ([]FoundRequest, error) {
		for i, state := range states {
			id := "F" + strconv.Itoa(i+1)
			reqs = append(reqs, FoundRequest{ID: id, Seq: i + 1, Location: "x.go:1", Reproduction: "r", Question: "q", State: state})
		}
		return reqs, nil
	}))
	return taskFile
}

// One decision unit takes at most DecisionBatchMax undecided requests, in
// arrival order, and its durable write is none of them still undecided.
func TestDecisionUnits_TakeAtMostFiveUndecidedRequestsInArrivalOrder(t *testing.T) {
	u := FoundStateUndecided
	taskFile := decisionFixture(t, FoundStateApplied, u, u, u, u, u, u)
	target := UnitTarget{TaskFile: taskFile}

	units, err := decisionUnits(target)
	require.NoError(t, err)
	require.Len(t, units, 1)
	assert.Equal(t, UnitDecision, units[0].Kind)
	assert.Equal(t, "F2,F3,F4,F5,F6", units[0].ID)
	assert.Equal(t, "tp found decide F2,F3,F4,F5,F6", units[0].BriefCommand)

	target.ID = units[0].ID
	assert.False(t, UnitDecision.DurableWrite(target), "its requests are still undecided")
	root := filepath.Dir(taskFile)
	require.NoError(t, UpdateFoundRequests(root, taskFile, func(reqs []FoundRequest) ([]FoundRequest, error) {
		for i := 1; i <= 5; i++ {
			reqs[i].State = FoundStateApplied
		}
		return reqs, nil
	}))
	assert.True(t, UnitDecision.DurableWrite(target), "every request it named is answered")
	units, err = decisionUnits(UnitTarget{TaskFile: taskFile})
	require.NoError(t, err)
	assert.Equal(t, "F7", units[0].ID, "the rest go to the next unit")
	assert.False(t, UnitDecision.DurableWrite(UnitTarget{TaskFile: taskFile}), "a unit naming no request answered nothing")
}

// No undecided request, no decision unit.
func TestDecisionUnits_NoneWithoutAnUndecidedRequest(t *testing.T) {
	units, err := decisionUnits(UnitTarget{TaskFile: decisionFixture(t, FoundStateApplied, FoundStateUnapplied)})
	require.NoError(t, err)
	assert.Empty(t, units)
}

// spec/1.31.0.md §4: an undecided request holds no-ready-task back, so the
// decision unit runs first; without one the blocker stands.
func TestBuildBlockers_AnUndecidedRequestHoldsNoReadyTaskBack(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "a", Status: model.StatusOpen, DependsOn: []string{"missing"}}}}

	assert.Equal(t, []string{"no-ready-task"}, blockerCodes(BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})))
	assert.Empty(t, BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf, UndecidedRequests: true}))
}
