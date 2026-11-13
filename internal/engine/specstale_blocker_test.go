package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtSpecStale(t *testing.T, in *BlockerInputs) Blocker {
	t.Helper()
	in.Phase, in.ReviewStale, in.SpecPath = PhaseImplement, true, "spec.md"
	in.TaskFile = &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(in)
	require.Len(t, bs, 1)
	require.Equal(t, "spec-stale", bs[0].Code)
	return bs[0]
}

// TestSpecStaleBlocker_DataCarriesTheClearingCommands: with no round in flight
// the data names the commands that clear the blocker, in order: emit a review
// round over the new text, then record it.
func TestSpecStaleBlocker_DataCarriesTheClearingCommands(t *testing.T) {
	t.Parallel()
	b := builtSpecStale(t, &BlockerInputs{})
	assert.Equal(t, []string{"tp review spec.md", "tp review spec.md --record <file>"}, b.Data["commands"])
	assert.NotContains(t, b.Data, "in_flight_round")
	assert.NotContains(t, b.Data, "operator")
}

