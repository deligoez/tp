package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextUnitsRepo writes a spec and its adjacent task file in a fresh git-rooted
// directory and returns (taskFile, spec).
func nextUnitsRepo(t *testing.T) (taskFile, spec string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	spec = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	taskFile = filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"spec":"spec.md","tasks":[]}`), 0o600))
	return taskFile, spec
}

func unitKinds(units []NextUnit) []UnitKind {
	out := make([]UnitKind, 0, len(units))
	for _, u := range units {
		out = append(out, u.Kind)
	}
	return out
}

func unitIDs(units []NextUnit) []string {
	out := make([]string, 0, len(units))
	for _, u := range units {
		out = append(out, u.ID)
	}
	return out
}

// TestBuildNextUnits_RoundIsNilOutsideRoundBasedPhases: §4.1 — round is the
// number of the round the returned units belong to, and null outside a
// round-based phase. That is what makes the driver leave TP_ROUND unset rather
// than empty (§3.1.1).
func TestBuildNextUnits_RoundIsNilOutsideRoundBasedPhases(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "t1", Status: model.StatusOpen},
	}}

	for _, phase := range []string{PhaseImplement, PhaseDecompose, PhaseRelease} {
		_, round := BuildNextUnits(taskFile, spec, phase, tf, nil, nil)
		assert.Nil(t, round, "%s is not a round-based phase", phase)
	}

	for _, phase := range []string{PhaseReview, PhaseAudit} {
		_, round := BuildNextUnits(taskFile, spec, phase, tf, nil, nil)
		require.NotNil(t, round, "%s is round-based", phase)
		assert.Equal(t, 1, *round, "no round recorded yet: the phase is collecting round 1")
	}
}

// TestBuildNextUnits_RoundIsTheOneBeingCollected: with rounds recorded, the
// reported round is the next one — the round the returned role units are
// collecting, not the last one recorded.
func TestBuildNextUnits_RoundIsTheOneBeingCollected(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	st := &ReviewState{
		ReviewRounds: []ReviewRound{{Round: 1}, {Round: 2}},
		AuditRounds:  []ReviewRound{{Round: 1}},
	}

	_, reviewRound := BuildNextUnits(taskFile, spec, PhaseReview, tf, st, nil)
	require.NotNil(t, reviewRound)
	assert.Equal(t, 3, *reviewRound)

	_, auditRound := BuildNextUnits(taskFile, spec, PhaseAudit, tf, st, nil)
	require.NotNil(t, auditRound)
	assert.Equal(t, 2, *auditRound)
}

// TestBuildNextUnits_RolePanelIsTheCorpus: a round-based phase returns one role
// unit per active role, in corpus order, each carrying its kind's brief command.
func TestBuildNextUnits_RolePanelIsTheCorpus(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	review, _ := BuildNextUnits(taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"implementer", "tester", "architect"}, unitIDs(review))
	for _, u := range review {
		assert.Equal(t, UnitReviewRole, u.Kind)
		assert.Equal(t, "tp review "+spec, u.BriefCommand)
	}

	audit, _ := BuildNextUnits(taskFile, spec, PhaseAudit, tf, nil, nil)
	assert.Equal(t, []string{"spec-coverage", "security", "maintainability-conventions"}, unitIDs(audit))
	for _, u := range audit {
		assert.Equal(t, UnitAuditRole, u.Kind)
		assert.Equal(t, "tp audit "+spec, u.BriefCommand)
	}
}

// TestBuildNextUnits_RolePanelHonoursSpecDeactivation: the oracle resolves the
// panel the emitting command would resolve, so a role this spec deactivates is
// not a unit the driver spawns.
func TestBuildNextUnits_RolePanelHonoursSpecDeactivation(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	require.NoError(t, os.WriteFile(spec,
		[]byte("---\ntp:\n  review_roles:\n    tester:\n      enabled: false\n---\n# S\n"), 0o600))
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	units, _ := BuildNextUnits(taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"implementer", "architect"}, unitIDs(units))
}

// TestBuildNextUnits_ImplementAndDecomposeSubjects: §3.1.1's ids — the task id
// for implement (the WIP task first, so a crashed unit is resumed), the spec
// base name for decompose.
func TestBuildNextUnits_ImplementAndDecomposeSubjects(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "ready", Status: model.StatusOpen},
		{ID: "in-progress", Status: model.StatusWIP},
	}}

	units, _ := BuildNextUnits(taskFile, spec, PhaseImplement, tf, nil, nil)
	require.Len(t, units, 1)
	assert.Equal(t, NextUnit{Kind: UnitImplement, ID: "in-progress", BriefCommand: "tp next --brief"}, units[0])

	units, _ = BuildNextUnits(taskFile, spec, PhaseDecompose, tf, nil, nil)
	require.Len(t, units, 1)
	assert.Equal(t, NextUnit{Kind: UnitDecompose, ID: "spec", BriefCommand: "tp resume"}, units[0])
}

// TestBuildNextUnits_EmptyCases: §4.1's three empty cases — release, a phase
// whose work is blocked, and a phase awaiting an operator decision (the
// escalate blocker class). Never nil: an empty next_units serializes as [].
func TestBuildNextUnits_EmptyCases(t *testing.T) {
	taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{{ID: "t1", Status: model.StatusOpen}}}

	units, _ := BuildNextUnits(taskFile, spec, PhaseRelease, tf, nil, nil)
	assert.NotNil(t, units)
	assert.Empty(t, units, "release is the releasable condition itself")

	escalate := []Blocker{{Code: "review-budget-exhausted", Class: ClassEscalate}}
	units, round := BuildNextUnits(taskFile, spec, PhaseReview, tf, nil, escalate)
	assert.Empty(t, units, "raising the cap is a user decision, so the oracle offers no unit")
	require.NotNil(t, round, "the round is a property of the phase, not of the returned slice")
	assert.Equal(t, 1, *round)

	clearable := []Blocker{{Code: "unexplained-changes", Class: ClassAgentClearable}}
	units, _ = BuildNextUnits(taskFile, spec, PhaseImplement, tf, nil, clearable)
	assert.Len(t, units, 1, "an agent-clearable blocker does not stop the phase")

	blocked := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "t1", Status: model.StatusOpen, DependsOn: []string{"missing"}},
	}}
	units, _ = BuildNextUnits(taskFile, spec, PhaseImplement, blocked, nil, nil)
	assert.Empty(t, units, "no ready task: the phase's work is blocked")
}

// TestBuildNextUnits_NonConcurrentKindIsNeverAlongsideAnother is §4.1's
// concurrency invariant, checked over every kind rather than over the phases
// that happen to emit one today: only the two role kinds may share an array.
func TestBuildNextUnits_NonConcurrentKindIsNeverAlongsideAnother(t *testing.T) {
	for _, kind := range UnitKinds() {
		pair := []NextUnit{{Kind: kind, ID: "a"}, {Kind: UnitReviewRole, ID: "b"}}
		got := soloIfNotConcurrent(pair)
		if kind.Concurrent() {
			assert.Len(t, got, 2, "%s runs beside a sibling role", kind)
			continue
		}
		require.Len(t, got, 1, "%s runs alone", kind)
		assert.Equal(t, kind, got[0].Kind)
	}

	// A concurrent unit followed by a non-concurrent one is also trimmed: the
	// invariant is about the array, not about its first entry.
	mixed := soloIfNotConcurrent([]NextUnit{{Kind: UnitReviewRole, ID: "a"}, {Kind: UnitReviewRecord, ID: "1"}})
	assert.Equal(t, []UnitKind{UnitReviewRole}, unitKinds(mixed))
}

// TestRenderNextAction_RendersFirstUnit: §4.1 — next_action is a rendering of
// next_units[0], not a second opinion, so its payload names that unit. An empty
// array leaves next_action untouched.
func TestRenderNextAction_RendersFirstUnit(t *testing.T) {
	base := NextAction{Summary: "s", Payload: map[string]any{"round": 2}}

	unchanged := renderNextAction(base, nil)
	assert.Equal(t, map[string]any{"round": 2}, unchanged.Payload)

	rendered := renderNextAction(base, []NextUnit{
		{Kind: UnitReviewRole, ID: "implementer", BriefCommand: "tp review spec.md"},
		{Kind: UnitReviewRole, ID: "tester", BriefCommand: "tp review spec.md"},
	})
	assert.Equal(t, map[string]any{"kind": "review-role", "id": "implementer"}, rendered.Payload["unit"])
	assert.Equal(t, 2, rendered.Payload["round"], "the phase payload survives")
	assert.Equal(t, map[string]any{"round": 2}, base.Payload, "the caller's payload is not mutated")
}
