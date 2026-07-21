package cli

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeCommonPolicy_HoistsUnanimousFields(t *testing.T) {
	overrides := []model.WorkflowOverride{
		{ReviewMaxRounds: iptr(8), GateTimeoutSeconds: iptr(600), QualityGate: sptr("go test ./...")},
		{ReviewMaxRounds: iptr(8), GateTimeoutSeconds: iptr(900), QualityGate: sptr("go test ./...")},
	}
	common := computeCommonPolicy(overrides)
	require.NotNil(t, common.ReviewMaxRounds)
	assert.Equal(t, 8, *common.ReviewMaxRounds, "a unanimous field is hoisted")
	require.NotNil(t, common.QualityGate)
	assert.Equal(t, "go test ./...", *common.QualityGate, "unanimous quality_gate is hoisted")
	assert.Nil(t, common.GateTimeoutSeconds, "a divergent field is not hoisted")

	assert.ElementsMatch(t, []string{"review_max_rounds", "quality_gate"}, hoistedFields(common))
}

func TestComputeCommonPolicy_AbsentFromAnyNotHoisted(t *testing.T) {
	overrides := []model.WorkflowOverride{
		{ReviewMaxRounds: iptr(8)},
		{}, // absent
	}
	assert.Nil(t, computeCommonPolicy(overrides).ReviewMaxRounds,
		"a field absent from any task file is not hoisted")
}

func TestMergeCommon_PreservesOtherFields(t *testing.T) {
	dst := model.WorkflowOverride{AuditMaxRounds: iptr(3)} // hand-set project field
	mergeCommon(&dst, model.WorkflowOverride{ReviewMaxRounds: iptr(8)})
	require.NotNil(t, dst.ReviewMaxRounds)
	assert.Equal(t, 8, *dst.ReviewMaxRounds, "hoisted field is written")
	require.NotNil(t, dst.AuditMaxRounds)
	assert.Equal(t, 3, *dst.AuditMaxRounds, "other hand-set fields are preserved")
}
