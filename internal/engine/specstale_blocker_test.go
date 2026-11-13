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

// TestSpecStaleBlocker_InFlightOverTheCurrentTextNamesTheRecordAlone: a round
// in flight that was emitted from the current text reads it, so recording it
// is the whole way out, and there is nothing to discard.
func TestSpecStaleBlocker_InFlightOverTheCurrentTextNamesTheRecordAlone(t *testing.T) {
	t.Parallel()
	b := builtSpecStale(t, &BlockerInputs{ReviewInFlightRound: 3})
	assert.Equal(t, 3, b.Data["in_flight_round"])
	assert.Equal(t, []string{"tp review spec.md --record <file>"}, b.Data["commands"])
	assert.NotContains(t, b.Data, "operator")
	assert.Contains(t, b.Message, "review round 3 is in flight over the current text")
}

// TestSpecStaleBlocker_AtTheReviewCapNamesTheRaiseAsTheOperators: with the
// review budget spent, the emission exits 4, so it is no command for the
// agent; it is named under operator with the decision raising the cap maps
// to, raise-review-cap.
func TestSpecStaleBlocker_AtTheReviewCapNamesTheRaiseAsTheOperators(t *testing.T) {
	t.Parallel()
	b := builtSpecStale(t, &BlockerInputs{ReviewRounds: 3, ReviewMaxRounds: 3})
	assert.Equal(t, []string{}, b.Data["commands"])
	assert.Equal(t, map[string]any{
		"decision": EscalateRaiseReviewCap,
		"commands": []string{"tp review spec.md", "tp review spec.md --record <file>"},
	}, b.Data["operator"])
	assert.Contains(t, b.Message, "3-round cap")
	assert.Contains(t, b.Message, "operator's decision (raise-review-cap)")

	under := builtSpecStale(t, &BlockerInputs{ReviewRounds: 2, ReviewMaxRounds: 3})
	assert.Equal(t, []string{"tp review spec.md", "tp review spec.md --record <file>"}, under.Data["commands"])
	assert.NotContains(t, under.Data, "operator")
}

// TestSpecStaleBlocker_AtTheCapAfterTheRecordNamesTheRecordAlone: a round in
// flight over the old text whose record spends the budget leaves the record
// as the agent's only command, since the emission after it would exit 4; its
// discard stays the operator's way to a round over the new text within the cap.
func TestSpecStaleBlocker_AtTheCapAfterTheRecordNamesTheRecordAlone(t *testing.T) {
	t.Parallel()
	b := builtSpecStale(t, &BlockerInputs{ReviewRounds: 2, ReviewMaxRounds: 3, ReviewInFlightRound: 3, ReviewInFlightOverOldText: true})
	assert.Equal(t, []string{"tp review spec.md --record <file>"}, b.Data["commands"])
	assert.Equal(t, EscalateDiscardEmission, b.Data["operator"].(map[string]any)["decision"])
	assert.Contains(t, b.Message, "3-round cap")
}
