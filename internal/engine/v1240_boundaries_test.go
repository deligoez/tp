package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// TestWaiting_AcceptsExactlyTheCapAndRefusesOneMore pins the cap's edge: a
// waiting text of WaitingMaxBytes bytes is one short line, one byte more is not.
func TestWaiting_AcceptsExactlyTheCapAndRefusesOneMore(t *testing.T) {
	task := guidanceTask("t1", model.StatusWIP)
	task.Waiting = strings.Repeat("x", 200)
	assert.Empty(t, TaskSchemaErrors(&task), "200 bytes is within the cap")

	task.Waiting = strings.Repeat("x", 201)
	assert.Equal(t, []string{"task t1: waiting must be one line of at most 200 bytes"}, TaskSchemaErrors(&task))
}

// TestChainNote_ATwoTaskChainUnderCapTwo: the shortest chain, two tasks one
// after the other, already leaves cap 2's second slot idle.
func TestChainNote_ATwoTaskChainUnderCapTwo(t *testing.T) {
	chain := &model.TaskFile{Tasks: []model.Task{
		guidanceTask("a", model.StatusOpen),
		guidanceTask("b", model.StatusOpen, "a"),
	}}
	assert.Equal(t, "base; only a chain of 2 tasks remains, so run_max_parallel 2 runs one unit at a time",
		implementGuidance("base", chain, 2, nil))
}

// TestValidate_RefusesATaskFileWorktreeSetupCmd: the driver reads the field
// from the project layer only, so a task file carrying it is an error.
func TestValidate_RefusesATaskFileWorktreeSetupCmd(t *testing.T) {
	tf := &model.TaskFile{}
	assert.Empty(t, findingsWithRule(validateConvergence(tf), "schema"), "no field, no finding")

	cmd := "make deps"
	tf.Workflow.WorktreeSetupCmd = &cmd
	got := findingsWithRule(validateConvergence(tf), "schema")
	if assert.Len(t, got, 1) {
		assert.Equal(t, "error", got[0].Severity)
		assert.Equal(t, "workflow.worktree_setup_cmd is read only from the project layer; move it to .tp/config.json", got[0].Message)
	}
}

func findingsWithRule(fs []Finding, rule string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}
