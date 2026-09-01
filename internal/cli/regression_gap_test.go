package cli_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestEmittedRolesMinusUnitsIsExactlyRegression is v0.36.0 §6.2 property 11.
//
// `regression` is emitted to every reviewer and assigned to no unit: the panel
// tp review emits and the panel tp resume spawns are computed by different
// filters, and this is the one place they diverge in the emitted-side
// direction. Measured on this repository's own round 3, zero recorded findings
// carried the regression role -- every unit was told to process the regression
// prompt and none of them was the regression unit.
//
// It is asserted as SET EQUALITY rather than as "regression is in the
// difference", so a release that adds a regression unit -- or drops another
// role from the unit set -- has to update this deliberately instead of
// discovering it in a round's missing findings.
func TestEmittedRolesMinusUnitsIsExactlyRegression(t *testing.T) {
	// The relocated spec carries its recorded rounds, so the emission is past
	// round 1 and `regression` is emitted rather than skipped no-baseline --
	// the precondition the property names.
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	_, emitted := promptsOf(t, emitPayload(t, spec))
	require.Contains(t, emitted, engine.RegressionRoleID,
		"the property is stated at a round where regression emits; this round must be one")

	// The unit set for the same phase, from the same oracle tp run reads.
	units, _ := engine.BuildNextUnits(dir, filepath.Join(dir, "x.tasks.json"), spec,
		engine.PhaseReview, nil, nil, nil)
	unitIDs := make([]string, 0, len(units))
	for i := range units {
		require.Equal(t, engine.UnitReviewRole, units[i].Kind,
			"the review phase spawns review-role units")
		unitIDs = append(unitIDs, units[i].ID)
	}
	require.NotEmpty(t, unitIDs, "the unit set must be non-empty for the difference to mean anything")

	assert.Equal(t, []string{engine.RegressionRoleID}, difference(emitted, unitIDs),
		"the emitted role set minus the unit set is exactly {regression}")

	// And the other direction: no unit is spawned for a role nothing emits.
	assert.Empty(t, difference(unitIDs, emitted),
		"every unit's role is one the round emits a prompt for")
}

// difference returns the sorted members of a that are absent from b.
func difference(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, s := range b {
		in[s] = true
	}
	out := make([]string, 0)
	for _, s := range a {
		if !in[s] {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
