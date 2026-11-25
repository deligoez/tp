package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
)

// found_max resolves like the other workflow fields (spec/1.31.0.md §8): an
// unset field takes the built-in 5, and an explicit 0 at either layer stays 0,
// the uncapped value, rather than falling back to the default.
func TestFoundMax_DefaultAndExplicitZero(t *testing.T) {
	t.Parallel()
	wf := ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
	assert.Equal(t, 5, wf.FoundMax, "an unset found_max resolves to 5")

	zero := 0
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{FoundMax: &zero}, &model.WorkflowOverride{})
	assert.Equal(t, 0, wf.FoundMax, "found_max=0 at the task layer resolves to 0, uncapped")
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{FoundMax: &zero})
	assert.Equal(t, 0, wf.FoundMax, "found_max=0 at the project layer resolves to 0, uncapped")

	resolved, ok := ResolvedCapValue(&wf, "found_max")
	assert.True(t, ok, "found_max is a fenced cap field")
	assert.Zero(t, resolved)
	assert.True(t, UnattendedRaise("found_max", 0, 5), "0 over a real cap removes it: a raise")
	assert.True(t, UnattendedRaise("found_max", 6, 5), "a larger cap is a raise")
	assert.False(t, UnattendedRaise("found_max", 4, 5), "lowering the cap is not a raise")
}

// found_max parses from a project workflow block and is clamped like the
// other caps when out of range.
func TestFoundMax_ProjectLayerParsesAndClamps(t *testing.T) {
	t.Parallel()
	wf, warnings := resolveProjectWorkflow(t, `{"found_max": 3}`)
	assert.Empty(t, warnings)
	assert.Equal(t, 3, wf.FoundMax)

	wf, warnings = resolveProjectWorkflow(t, `{"found_max": -1}`)
	assert.NotEmpty(t, warnings, "a negative found_max is out of range")
	assert.Equal(t, 5, wf.FoundMax, "an out-of-range value falls back to the default")
}

// found_max accepts 0 through 100 and warns outside it.
func TestFoundMax_TheRangeIsZeroToOneHundred(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want int
		warn bool
	}{
		{`{"found_max": 0}`, 0, false},
		{`{"found_max": 100}`, 100, false},
		{`{"found_max": 101}`, 5, true},
	} {
		wf, warnings := resolveProjectWorkflow(t, tc.raw)
		assert.Equal(t, tc.want, wf.FoundMax, tc.raw)
		assert.Equal(t, tc.warn, len(warnings) > 0, tc.raw)
	}
}
