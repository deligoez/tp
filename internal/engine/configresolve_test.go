package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestResolveWorkflowLayers_Ranking(t *testing.T) {
	// Override outranks project outranks built-in default, per field.
	override := model.WorkflowOverride{ReviewMaxRounds: ptr(5)}
	project := model.WorkflowOverride{ReviewMaxRounds: ptr(8), AuditMaxRounds: ptr(9)}

	wf := ResolveWorkflowLayers(override, project)
	assert.Equal(t, 5, wf.ReviewMaxRounds, "task override wins over project")
	assert.Equal(t, 9, wf.AuditMaxRounds, "project wins where override is absent")
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default wins where both layers are absent")
	assert.Equal(t, 600, wf.GateTimeoutSeconds, "built-in default gate timeout")
}

func TestResolveWorkflowLayers_QualityGatePrecedence(t *testing.T) {
	wf := ResolveWorkflowLayers(
		model.WorkflowOverride{},
		model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "make test", wf.QualityGate, "project quality_gate applies when no task override")

	wf = ResolveWorkflowLayers(
		model.WorkflowOverride{QualityGate: ptr("go test ./...")},
		model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "go test ./...", wf.QualityGate, "task override wins")
}

func TestResolveEffectiveWorkflow_SparseMerge(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	tp := filepath.Join(root, ".tp")
	require.NoError(t, os.Mkdir(tp, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8,"gate_timeout_seconds":1200,"review_clean_rounds":3}}`), 0o600))

	// The task file sets only review_max_rounds; it inherits the rest.
	wf, warns, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewMaxRounds: ptr(0)})
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "task override (explicit 0) wins")
	assert.Equal(t, 1200, wf.GateTimeoutSeconds, "inherited from project")
	assert.Equal(t, 3, wf.ReviewCleanRounds, "inherited from project")
	assert.Equal(t, 2, wf.AuditCleanRounds, "built-in default where neither layer sets it")
}

func TestResolveEffectiveWorkflow_NoConfigIsV023(t *testing.T) {
	root := t.TempDir() // no .tp/
	wf, _, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewMaxRounds: ptr(4)})
	require.NoError(t, err)
	assert.Equal(t, 4, wf.ReviewMaxRounds)
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default with no project config")
	assert.Equal(t, 600, wf.GateTimeoutSeconds)
}

func TestResolveWorkflowLayers_PresenceZeroWins(t *testing.T) {
	// review_max_rounds:0 (explicit no-cap) is a present override that must win
	// over a non-zero project value, not be mistaken for absent.
	wf := ResolveWorkflowLayers(
		model.WorkflowOverride{ReviewMaxRounds: ptr(0)},
		model.WorkflowOverride{ReviewMaxRounds: ptr(8)},
	)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "explicit 0 override wins over project 8")
}

func TestResolveWorkflowLayers_ChecksReplaceSemantics(t *testing.T) {
	projChecks := model.WorkflowOverride{Checks: &[]model.Check{{Class: "x", Cmd: "run-x"}}}

	// A present empty checks array replaces the project checks with nothing.
	empty := []model.Check{}
	wf := ResolveWorkflowLayers(model.WorkflowOverride{Checks: &empty}, projChecks)
	assert.Empty(t, wf.Checks, "present empty checks replaces project checks")

	// An absent checks key inherits the project checks.
	wf = ResolveWorkflowLayers(model.WorkflowOverride{}, projChecks)
	require.Len(t, wf.Checks, 1)
	assert.Equal(t, "x", wf.Checks[0].Class, "absent checks inherits project checks")
}
