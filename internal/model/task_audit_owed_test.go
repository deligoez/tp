package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The stamp is a copy of the resolved roles: a later change to the workflow
// value tp done read must not reach back into the closed task.
func TestTask_SetTaskAuditOwedCopiesTheRoles(t *testing.T) {
	roles := []string{"go-safety", "security"}
	var task Task
	task.SetTaskAuditOwed(roles)
	roles[0] = "changed"
	assert.Equal(t, []string{"go-safety", "security"}, task.TaskAuditOwed)
}

func TestTask_SetTaskAuditOwedEmptyWritesNoKey(t *testing.T) {
	task := Task{ID: "t1", TaskAuditOwed: []string{"stale"}}
	task.SetTaskAuditOwed([]string{})
	assert.Nil(t, task.TaskAuditOwed)

	data, err := json.Marshal(task)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "task_audit_owed")
}

func TestWorkflow_TaskAuditRolesSerializes(t *testing.T) {
	data, err := json.Marshal(Workflow{TaskAuditRoles: []string{}})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"task_audit_roles":[]`, "the resolved value always carries the key")
}

func TestWorkflowOverride_TaskAuditRolesPresence(t *testing.T) {
	var present WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{"task_audit_roles":[]}`), &present))
	require.NotNil(t, present.TaskAuditRoles, "a present empty list is an override")
	assert.False(t, present.IsEmpty())

	var absent WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{}`), &absent))
	assert.Nil(t, absent.TaskAuditRoles)
}
