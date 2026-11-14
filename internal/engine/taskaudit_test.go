package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func TestResolveWorkflowLayers_TaskAuditRolesDefaultsEmpty(t *testing.T) {
	wf := ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
	require.NotNil(t, wf.TaskAuditRoles, "the default is an empty list, not an absent one")
	assert.Empty(t, wf.TaskAuditRoles)
}

func TestResolveWorkflowLayers_TaskAuditRolesReplaceSemantics(t *testing.T) {
	project := model.WorkflowOverride{TaskAuditRoles: &[]string{"go-safety"}}

	wf := ResolveWorkflowLayers(&model.WorkflowOverride{}, &project)
	assert.Equal(t, []string{"go-safety"}, wf.TaskAuditRoles, "an absent task layer inherits the project roles")

	task := []string{"security", "soundness"}
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{TaskAuditRoles: &task}, &project)
	assert.Equal(t, []string{"security", "soundness"}, wf.TaskAuditRoles, "the task layer replaces the project roles")

	empty := []string{}
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{TaskAuditRoles: &empty}, &project)
	require.NotNil(t, wf.TaskAuditRoles)
	assert.Empty(t, wf.TaskAuditRoles, "a present empty list wins over the project roles")
}

func TestParseWorkflowOverride_TaskAuditRoles(t *testing.T) {
	wo, warnings := parseWorkflowOverride([]byte(`{"task_audit_roles":["go-safety","security"]}`))
	assert.Empty(t, warnings)
	require.NotNil(t, wo.TaskAuditRoles)
	assert.Equal(t, []string{"go-safety", "security"}, *wo.TaskAuditRoles)
	assert.False(t, wo.IsEmpty(), "a present task_audit_roles is an override")

	wo, warnings = parseWorkflowOverride([]byte(`{"task_audit_roles":"go-safety"}`))
	assert.Nil(t, wo.TaskAuditRoles, "a wrong-typed value is ignored")
	assert.Equal(t, []string{"workflow.task_audit_roles: expected an array of strings, ignored"}, warnings)
}

func TestValidateTaskAuditRoles(t *testing.T) {
	assert.NoError(t, ValidateTaskAuditRoles(nil))
	assert.NoError(t, ValidateTaskAuditRoles([]string{"go-safety", "security"}))

	for name, tc := range map[string]struct {
		roles []string
		want  string
	}{
		"spec-coverage": {[]string{"go-safety", "spec-coverage"}, "only code roles may audit a task"},
		"blank":         {[]string{""}, "must be non-empty"},
		"not kebab":     {[]string{"Go_Safety"}, "must match"},
		"duplicate":     {[]string{"go-safety", "go-safety"}, "duplicate role"},
	} {
		err := ValidateTaskAuditRoles(tc.roles)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.want, name)
	}
}

func TestValidateTaskEntry_RefusesTaskAuditOwed(t *testing.T) {
	task := model.Task{ID: "t1", Title: "T", Acceptance: "a", SourceSections: []string{"## 1. A"}, TaskAuditOwed: []string{"go-safety"}}
	f := ValidateTaskEntry(&task, map[string]bool{"t1": true})
	require.NotNil(t, f, "a hand-written task cannot carry the debt tp done records")
	assert.Equal(t, "managed-field", f.Rule)
	assert.Contains(t, f.Msg, "task_audit_owed")
	assert.Contains(t, f.Hint, "tp done")
}

func TestValidate_TaskAuditOwedOnATaskThatIsNotDone(t *testing.T) {
	tf := &model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{
		{ID: "t1", Title: "T", Status: model.StatusOpen, DependsOn: []string{}, EstimateMinutes: 5,
			Acceptance: "a", SourceSections: []string{"## 1. A"}, TaskAuditOwed: []string{"go-safety"}},
	}}
	result := Validate(tf, "", false, false)
	assert.False(t, result.Valid, "a stamp on a task that is not done is a forged debt")
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "task_audit_owed") {
			found = true
		}
	}
	assert.True(t, found, "findings: %v", result.Findings)
}

func TestValidate_TaskAuditRolesWorkflowEntryIsChecked(t *testing.T) {
	roles := []string{"spec-coverage"}
	tf := &model.TaskFile{Version: 1, Spec: "s.md", Workflow: model.WorkflowOverride{TaskAuditRoles: &roles}, Tasks: []model.Task{}}
	result := Validate(tf, "", false, false)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "task_audit_roles") {
			found = true
		}
	}
	assert.True(t, found, "findings: %v", result.Findings)
}
