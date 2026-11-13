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

// TestSpecStaleBlocker_InFlightOverTheOldTextNamesTheRecord: a round in flight
// whose snapshot is the old text refuses a new emission (exit 3), so the
// clearing commands start with that round's record, and its discard is named
// as the operator's decision rather than as a command to run.
func TestSpecStaleBlocker_InFlightOverTheOldTextNamesTheRecord(t *testing.T) {
	t.Parallel()
	b := builtSpecStale(t, &BlockerInputs{ReviewInFlightRound: 3, ReviewInFlightOverOldText: true})
	assert.Equal(t, 3, b.Data["in_flight_round"])
	assert.Equal(t, []string{"tp review spec.md --record <file>", "tp review spec.md", "tp review spec.md --record <file>"},
		b.Data["commands"])
	assert.Equal(t, map[string]any{
		"decision": EscalateDiscardEmission,
		"commands": []string{"tp review spec.md --force", "tp review spec.md --record <file>"},
	}, b.Data["operator"])
	assert.Contains(t, b.Message, "review round 3 is in flight over the old text")
	assert.Contains(t, b.Message, "record it first: tp review spec.md --record <file>")
	assert.Contains(t, b.Message, "operator's decision")
}

