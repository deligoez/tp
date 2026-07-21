package cli

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func iptr(v int) *int       { return &v }
func sptr(v string) *string { return &v }

func TestWorkflowDeviations_ReportsDifferingFields(t *testing.T) {
	project := model.WorkflowOverride{
		ReviewMaxRounds:   iptr(8),
		ReviewCleanRounds: iptr(2),
	}
	// Override differs on review_max_rounds; matches clean_rounds; sets an audit
	// cap the project does not (so it is not a deviation).
	override := model.WorkflowOverride{
		ReviewMaxRounds:   iptr(0),
		ReviewCleanRounds: iptr(2),
		AuditMaxRounds:    iptr(5),
	}

	devs := workflowDeviations("chapter-03.tasks.json", override, project)
	require.Len(t, devs, 1, "only the differing, project-set field is a deviation")
	d := devs[0]
	assert.Equal(t, "review_max_rounds", d["field"])
	assert.Equal(t, "0", d["override"])
	assert.Equal(t, "8", d["project"])
	assert.Equal(t, "chapter-03.tasks.json", d["file"])
}

func TestWorkflowDeviations_QualityGate(t *testing.T) {
	devs := workflowDeviations("x.tasks.json",
		model.WorkflowOverride{QualityGate: sptr("make test")},
		model.WorkflowOverride{QualityGate: sptr("go test ./...")},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "quality_gate", devs[0]["field"])
}
