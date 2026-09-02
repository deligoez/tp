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

	assert.ElementsMatch(t, []string{"review_max_rounds", "quality_gate"}, hoistedFields(&common))
}

func TestComputeCommonPolicy_AbsentFromAnyNotHoisted(t *testing.T) {
	overrides := []model.WorkflowOverride{
		{ReviewMaxRounds: iptr(8)},
		{}, // absent
	}
	assert.Nil(t, computeCommonPolicy(overrides).ReviewMaxRounds,
		"a field absent from any task file is not hoisted")
}

func TestComputeCommonPolicy_NothingToHoist(t *testing.T) {
	overrides := []model.WorkflowOverride{
		{ReviewMaxRounds: iptr(8)},
		{ReviewMaxRounds: iptr(3)}, // diverges → no common field
	}
	common := computeCommonPolicy(overrides)
	assert.Empty(t, hoistedFields(&common), "no field common to all files means nothing to hoist")
}

func TestMergeCommon_PreservesOtherFields(t *testing.T) {
	dst := model.WorkflowOverride{AuditMaxRounds: iptr(3)} // hand-set project field
	mergeCommon(&dst, &model.WorkflowOverride{ReviewMaxRounds: iptr(8)})
	require.NotNil(t, dst.ReviewMaxRounds)
	assert.Equal(t, 8, *dst.ReviewMaxRounds, "hoisted field is written")
	require.NotNil(t, dst.AuditMaxRounds)
	assert.Equal(t, 3, *dst.AuditMaxRounds, "other hand-set fields are preserved")
}

func TestComputeCommonPolicy_HoistsUnanimousReviewConvergeOn(t *testing.T) {
	overrides := []model.WorkflowOverride{
		{ReviewConvergeOn: sptr("all")},
		{ReviewConvergeOn: sptr("all")},
	}
	common := computeCommonPolicy(overrides)
	require.NotNil(t, common.ReviewConvergeOn)
	assert.Equal(t, "all", *common.ReviewConvergeOn, "a unanimous review_converge_on is hoisted")
	assert.Contains(t, hoistedFields(&common), "review_converge_on")
}

func TestComputeCommonPolicy_ReviewConvergeOnNotHoistedWhenDivergentOrAbsent(t *testing.T) {
	divergent := computeCommonPolicy([]model.WorkflowOverride{
		{ReviewConvergeOn: sptr("all")},
		{ReviewConvergeOn: sptr("blocking")},
	})
	assert.Nil(t, divergent.ReviewConvergeOn, "a divergent review_converge_on is not hoisted")

	partial := computeCommonPolicy([]model.WorkflowOverride{
		{ReviewConvergeOn: sptr("all")},
		{}, // unset in one file
	})
	assert.Nil(t, partial.ReviewConvergeOn, "review_converge_on absent from any file is not hoisted")
}

func TestMergeCommon_WritesReviewConvergeOn(t *testing.T) {
	dst := model.WorkflowOverride{AuditMaxRounds: iptr(3)} // hand-set project field
	mergeCommon(&dst, &model.WorkflowOverride{ReviewConvergeOn: sptr("all")})
	require.NotNil(t, dst.ReviewConvergeOn)
	assert.Equal(t, "all", *dst.ReviewConvergeOn, "hoisted review_converge_on is written")
	require.NotNil(t, dst.AuditMaxRounds)
	assert.Equal(t, 3, *dst.AuditMaxRounds, "other hand-set fields are preserved")
}

// audit_converge_on is hoisted on its twin's terms above: a value every scanned
// task file sets identically moves to the project layer, and one that diverges
// or is absent from any file does not. Three functions carry the field —
// computeCommonPolicy, hoistedFields and mergeCommon — and each has its own
// assertion here, because the named mutant is only the middle one and dropping
// either of the others is silently worse: strip-without-write destroys the
// value outright.
func TestComputeCommonPolicy_HoistsUnanimousAuditConvergeOn(t *testing.T) {
	common := computeCommonPolicy([]model.WorkflowOverride{
		{AuditConvergeOn: sptr("blocking")},
		{AuditConvergeOn: sptr("blocking")},
	})
	require.NotNil(t, common.AuditConvergeOn)
	assert.Equal(t, "blocking", *common.AuditConvergeOn, "a unanimous audit_converge_on is hoisted")
	assert.Contains(t, hoistedFields(&common), "audit_converge_on")
}

func TestComputeCommonPolicy_AuditConvergeOnNotHoistedWhenDivergentOrAbsent(t *testing.T) {
	divergent := computeCommonPolicy([]model.WorkflowOverride{
		{AuditConvergeOn: sptr("all")},
		{AuditConvergeOn: sptr("blocking")},
	})
	assert.Nil(t, divergent.AuditConvergeOn, "a divergent audit_converge_on is not hoisted")
	assert.NotContains(t, hoistedFields(&divergent), "audit_converge_on")

	partial := computeCommonPolicy([]model.WorkflowOverride{
		{AuditConvergeOn: sptr("all")},
		{}, // unset in one file
	})
	assert.Nil(t, partial.AuditConvergeOn, "audit_converge_on absent from any file is not hoisted")
}

func TestMergeCommon_WritesAuditConvergeOn(t *testing.T) {
	dst := model.WorkflowOverride{AuditMaxRounds: iptr(3)} // hand-set project field
	mergeCommon(&dst, &model.WorkflowOverride{AuditConvergeOn: sptr("blocking")})
	require.NotNil(t, dst.AuditConvergeOn)
	assert.Equal(t, "blocking", *dst.AuditConvergeOn, "hoisted audit_converge_on is written")
	require.NotNil(t, dst.AuditMaxRounds)
	assert.Equal(t, 3, *dst.AuditMaxRounds, "other hand-set fields are preserved")
}
