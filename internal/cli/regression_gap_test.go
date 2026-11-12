package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestEveryEmittedPromptHasAUnit is the whole-panel invariant of
// spec/backlog/run-regression-unit.md: under a run, every prompt a round's
// emission prints has a unit.
//
// It is asserted over the emitted set rather than over one role, so the next
// role a predicate appends to an emission -- the way regression is appended
// after a fix -- fails here the same way regression did. The units are built
// first, from the state the run holds when it asks the oracle, and the round is
// emitted after, which is the order a run does the two in.
//
// The other direction (a unit for a role nothing emits) is legal and is not
// asserted: an audit role skipped no-checklist-items is still spawned
// (TestEveryUnitBriefCommandRunsInItsOwnPhase).
func TestEveryEmittedPromptHasAUnit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		phase string
		setup func(t *testing.T) (dir, spec string, args []string)
	}{
		{
			// The relocated spec carries its recorded rounds and fixed
			// findings, so the emission is past round 1 and carries
			// regression -- the predicate-appended prompt the decision names.
			name: "review past a fix", phase: engine.PhaseReview,
			setup: func(t *testing.T) (string, string, []string) {
				spec := relocatedSpec(t, "spec/0.36.0.md")
				return filepath.Dir(spec), spec, []string{"review", filepath.Base(spec), "--json"}
			},
		},
		{
			name: "audit", phase: engine.PhaseAudit,
			setup: func(t *testing.T) (string, string, []string) {
				dir := divergentAuditRepo(t)
				return dir, filepath.Join(dir, "spec.md"), []string{"audit", "spec.md", "--json"}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, spec, args := tc.setup(t)
			st, err := engine.LoadReviewState(spec)
			require.NoError(t, err)

			units, _ := engine.BuildNextUnits(dir, filepath.Join(dir, "x.tasks.json"), spec, tc.phase, nil, st, nil)
			unitIDs := make([]string, 0, len(units))
			for i := range units {
				unitIDs = append(unitIDs, units[i].ID)
			}
			require.NotEmpty(t, unitIDs, "the unit set must be non-empty for the difference to mean anything")

			stdout, stderr, code := runTPIn(t, dir, args...)
			require.Equal(t, 0, code, "emission failed: %s", stderr)
			_, emitted := promptsOf(t, decodePayload(t, stdout))
			if tc.phase == engine.PhaseReview {
				require.Contains(t, emitted, engine.RegressionRoleID,
					"the fixture must be a round that carries regression, or it measures the easy case")
			}

			assert.Empty(t, difference(emitted, unitIDs),
				"every prompt the round emits is answered by a unit of the run")
		})
	}
}

// TestRegressionUnitBriefEmitsItsOwnPrompt closes the loop the invariant opens:
// the regression unit's own brief, run as a unit runs it, emits exactly the
// regression prompt, so the unit has something to answer.
func TestRegressionUnitBriefEmitsItsOwnPrompt(t *testing.T) {
	t.Parallel()
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	st, err := engine.LoadReviewState(spec)
	require.NoError(t, err)

	units, _ := engine.BuildNextUnits(dir, filepath.Join(dir, "x.tasks.json"), spec, engine.PhaseReview, nil, st, nil)
	var brief string
	for i := range units {
		if units[i].ID == engine.RegressionRoleID {
			assert.Equal(t, engine.UnitReviewRole, units[i].Kind,
				"regression is a unit of the same kind as the phase's other role units")
			brief = units[i].BriefCommand
		}
	}
	require.NotEmpty(t, brief, "a round that carries regression spawns a regression unit")

	fields := strings.Fields(brief)
	require.Equal(t, "tp", fields[0], "a brief command starts with tp")
	out, stderr, code := runTPIn(t, dir, fields[1:]...)
	require.Equal(t, 0, code, "the regression unit's brief must exit 0: %s", stderr)
	_, emitted := promptsOf(t, decodePayload(t, out))
	assert.Equal(t, []string{engine.RegressionRoleID}, emitted)
}

// TestOracleAndEmissionAgreeOnRegression pins the other direction for the one
// predicate-appended role: the run spawns a regression unit exactly when the
// round emits a regression prompt. A unit spawned for a round that carries none
// would run `--role regression` against an emission that does not know the
// name.
func TestOracleAndEmissionAgreeOnRegression(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		carries bool
		setup   func(t *testing.T) string
	}{
		{name: "round 1", carries: false, setup: func(t *testing.T) string {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
			return dir
		}},
		{name: "round 2 with nothing to guard", carries: false, setup: func(t *testing.T) string {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
			_, stderr, code := runTP(t, dir, "review", "spec.md")
			require.Equal(t, 0, code, "round 1 emission: %s", stderr)
			_, stderr, code = recordRound(t, dir, "")
			require.Equal(t, 0, code, "round 1 record: %s", stderr)
			return dir
		}},
		{name: "round 2 after a fix", carries: true, setup: roundTwoWithFixedFinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := tc.setup(t)
			spec := filepath.Join(dir, "spec.md")
			st, err := engine.LoadReviewState(spec)
			require.NoError(t, err)

			units, _ := engine.BuildNextUnits(dir, filepath.Join(dir, "spec.tasks.json"), spec, engine.PhaseReview, nil, st, nil)
			unitIDs := make([]string, 0, len(units))
			for i := range units {
				unitIDs = append(unitIDs, units[i].ID)
			}
			_, emitted := promptsOf(t, emitPayload(t, spec))

			assert.Equal(t, tc.carries, slices.Contains(emitted, engine.RegressionRoleID), "the emission")
			assert.Equal(t, tc.carries, slices.Contains(unitIDs, engine.RegressionRoleID), "the oracle")
		})
	}
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
