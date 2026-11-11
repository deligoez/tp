package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestLifecycleBand_CountsWorkEpicsPerStageAndEarliestPhase is spec/1.8.0.md
// §6: the band counts work epics per stage and per spec phase, an epic with
// several bound specs once, at its earliest phase, a condition in neither
// axis, and an unbound task file not at all while a graph exists. Mutants
// counting every bound file, or an epic at its latest phase, fail it.
func TestLifecycleBand_CountsWorkEpicsPerStageAndEarliestPhase(t *testing.T) {
	t.Parallel()
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "a", Milestone: "M"}, {ID: "b", Milestone: "M"}, {ID: "c", Milestone: "M"},
		{ID: "d", Milestone: "M"}, {ID: "e", Milestone: "M"}, {ID: "f", Milestone: "M"},
		{ID: "x", Milestone: "M", Condition: true},
	}}
	states := map[string]EpicState{
		"a": {Status: EpicStatusPlanned}, "b": {Status: EpicStatusClaimed},
		"c": {Status: EpicStatusSpecced}, "d": {Status: EpicStatusInProgress},
		"e": {Status: EpicStatusDone}, "f": {Status: EpicStatusDone},
		"x": {Status: EpicStatusPlanned},
	}
	epicPhases := map[string][]string{
		"c": {PhaseReview},
		"d": {PhaseAudit, PhaseImplement},
		"e": {PhaseRelease},
	}
	band := LifecycleBand(g, states, epicPhases, []string{PhaseDecompose})
	assert.Equal(t, map[string]int{
		EpicStatusPlanned: 1, EpicStatusClaimed: 1, EpicStatusSpecced: 1,
		EpicStatusInProgress: 1, EpicStatusDone: 2,
	}, band.Stages)
	assert.Equal(t, map[string]int{
		PhaseReview: 1, PhaseDecompose: 0, PhaseImplement: 1, PhaseAudit: 0, PhaseRelease: 1,
	}, band.Phases)
	assert.Equal(t, BandPhasesOfEpics, band.PhasesOf)
}

// TestLifecycleBand_WithNoGraphThePhasesCountTaskFiles is §6: with no graph
// the phases count task files, and no epic stage counts anything.
func TestLifecycleBand_WithNoGraphThePhasesCountTaskFiles(t *testing.T) {
	t.Parallel()
	for _, g := range []*model.EpicGraph{nil, {}} {
		band := LifecycleBand(g, nil, nil, []string{PhaseReview, PhaseImplement, PhaseImplement})
		assert.Equal(t, map[string]int{
			PhaseReview: 1, PhaseDecompose: 0, PhaseImplement: 2, PhaseAudit: 0, PhaseRelease: 0,
		}, band.Phases)
		assert.Equal(t, map[string]int{
			EpicStatusPlanned: 0, EpicStatusClaimed: 0, EpicStatusSpecced: 0,
			EpicStatusInProgress: 0, EpicStatusDone: 0,
		}, band.Stages)
		assert.Equal(t, BandPhasesOfTaskFiles, band.PhasesOf)
	}
}

