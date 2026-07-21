package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
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
