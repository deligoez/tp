package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

func guidanceTask(id, status string, deps ...string) model.Task {
	return model.Task{ID: id, Title: "do " + id, Status: status, Acceptance: "done", EstimateMinutes: 5, SourceSections: []string{"A"}, DependsOn: deps}
}

// TestWaitingNote_NamesWhatAResumedWipTaskWaitsOn: a wip task parked on a
// background job reads differently from an abandoned claim after a reset.
func TestWaitingNote_NamesWhatAResumedWipTaskWaitsOn(t *testing.T) {
	wip := guidanceTask("t1", model.StatusWIP)
	wip.Waiting = "the background model run"
	tf := &model.TaskFile{Tasks: []model.Task{wip}}
	got := implementGuidance("base", tf, 1, []NextUnit{{Kind: UnitImplement, ID: "t1"}})
	assert.Equal(t, "base; task t1 is wip and waiting on: the background model run; check that before redoing its work", got)

	wip.Waiting = ""
	tf.Tasks[0] = wip
	assert.Equal(t, "base", implementGuidance("base", tf, 1, []NextUnit{{Kind: UnitImplement, ID: "t1"}}))
}

// TestChainNote_OnlyWhenTheCapWouldRunMoreThanOne: a strict chain leaves every
// slot past the first idle, which is worth one line only under a cap above 1.
func TestChainNote_OnlyWhenTheCapWouldRunMoreThanOne(t *testing.T) {
	chain := &model.TaskFile{Tasks: []model.Task{
		guidanceTask("a", model.StatusDone),
		guidanceTask("b", model.StatusOpen, "a"),
		guidanceTask("c", model.StatusOpen, "b"),
		guidanceTask("d", model.StatusOpen, "c"),
	}}
	assert.Equal(t, "base; only a chain of 3 tasks remains, so run_max_parallel 3 runs one unit at a time",
		implementGuidance("base", chain, 3, nil))
	assert.Equal(t, "base", implementGuidance("base", chain, 1, nil), "cap 1 runs one unit anyway")

	wide := &model.TaskFile{Tasks: []model.Task{
		guidanceTask("a", model.StatusOpen),
		guidanceTask("b", model.StatusOpen),
		guidanceTask("c", model.StatusOpen, "a", "b"),
	}}
	assert.Equal(t, "base", implementGuidance("base", wide, 3, nil), "two tasks can run side by side")

	pair := &model.TaskFile{Tasks: []model.Task{
		guidanceTask("a", model.StatusOpen),
		guidanceTask("b", model.StatusOpen),
	}}
	assert.Equal(t, "base", implementGuidance("base", pair, 3, nil), "two independent tasks are not a chain")

	single := &model.TaskFile{Tasks: []model.Task{guidanceTask("a", model.StatusOpen)}}
	assert.Equal(t, "base", implementGuidance("base", single, 3, nil), "one task left is not a chain")
}
