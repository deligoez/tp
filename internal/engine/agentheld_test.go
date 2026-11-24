package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentHeldFile has t1 wip under an agent spawn's claim, t2 waiting on it,
// and, when ready is set, t3 open and ready.
func agentHeldFile(ready bool) *model.TaskFile {
	tasks := []model.Task{
		{ID: "t1", Status: model.StatusWIP, EstimateMinutes: 5, ClaimedBy: &model.ClaimedBy{Session: "sessC", Spawn: "toolu_A"}},
		{ID: "t2", Status: model.StatusOpen, EstimateMinutes: 5, DependsOn: []string{"t1"}},
	}
	if ready {
		tasks = append(tasks, model.Task{ID: "t3", Status: model.StatusOpen, EstimateMinutes: 5})
	}
	return &model.TaskFile{Tasks: tasks}
}

// A task an agent spawn holds is that agent's: resume previews the next
// ready task instead, and the implement units count it as running.
func TestAnAgentHeldTaskIsNotOfferedToResume(t *testing.T) {
	t.Parallel()
	tf := agentHeldFile(true)
	id, wip, has := ImplementPreview(tf)
	assert.Equal(t, "t3", id)
	assert.False(t, wip)
	assert.True(t, has)
	assert.Empty(t, ImplementUnits(tf, 1, nil), "the agent holds the one slot")
	assert.Equal(t, []string{"t3"}, ImplementUnits(tf, 2, nil))
}

// When the only unblocked work is held by agents, nothing is escalated: the
// next step is the coordinator's pool.
func TestAgentHeldWorkIsNotANoReadyTask(t *testing.T) {
	t.Parallel()
	tf := agentHeldFile(false)
	assert.Empty(t, blockerCodes(BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})))
	next := BuildNextAction(PhaseImplement, "spec.md", "spec.md", tf, nil, false)
	require.NotNil(t, next.Command)
	assert.Equal(t, "tp pool --session sessC", *next.Command)
	assert.Equal(t, []string{"t1"}, next.Payload["agent_held"])
}

// A wip task no agent spawn holds is an interrupted unit, resumed first.
func TestAnUnheldWipTaskIsStillResumed(t *testing.T) {
	t.Parallel()
	tf := agentHeldFile(true)
	tf.Tasks[0].ClaimedBy = nil
	id, wip, _ := ImplementPreview(tf)
	assert.Equal(t, "t1", id)
	assert.True(t, wip)
	assert.Equal(t, []string{"t1"}, ImplementUnits(tf, 2, nil))
}
