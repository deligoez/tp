package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOverlapReport_UniqueShared computes per-role unique (sole-contributor) and
// shared (found_by >= 2) counts and flags a trim candidate (§8.5).
func TestOverlapReport_UniqueShared(t *testing.T) {
	// Clusters (found_by_roles):
	//   [implementer]            -> implementer unique
	//   [implementer, tester]    -> implementer shared, tester shared
	//   [tester, architect]      -> tester shared, architect shared
	//   [architect]              -> architect unique
	report := OverlapReport([][]string{
		{"implementer"},
		{"implementer", "tester"},
		{"tester", "architect"},
		{"architect"},
	})

	byRole := make(map[string]RoleOverlap)
	for _, r := range report {
		byRole[r.Role] = r
	}

	assert.Equal(t, RoleOverlap{Role: "implementer", Unique: 1, Shared: 1, TrimCandidate: false}, byRole["implementer"])
	assert.Equal(t, RoleOverlap{Role: "architect", Unique: 1, Shared: 1, TrimCandidate: false}, byRole["architect"])
	// tester is in two shared clusters and never sole -> trim candidate.
	assert.Equal(t, RoleOverlap{Role: "tester", Unique: 0, Shared: 2, TrimCandidate: true}, byRole["tester"])

	// Deterministic sort by role id.
	require.Len(t, report, 3)
	assert.Equal(t, "architect", report[0].Role)
	assert.Equal(t, "implementer", report[1].Role)
	assert.Equal(t, "tester", report[2].Role)
}

// TestOverlapReport_TrimCandidateOnlyWhenShared confirms a role that found
// nothing never appears and a purely-unique role is never a trim candidate.
func TestOverlapReport_TrimCandidateOnlyWhenShared(t *testing.T) {
	report := OverlapReport([][]string{
		{"implementer"},
		{"implementer"},
	})
	require.Len(t, report, 1)
	assert.Equal(t, "implementer", report[0].Role)
	assert.Equal(t, 2, report[0].Unique)
	assert.Equal(t, 0, report[0].Shared)
	assert.False(t, report[0].TrimCandidate, "unique-only role is never a trim candidate")
}

// TestOverlapReport_RegressionAndBlankExcluded confirms the built-in regression
// role and blank roles are never counted, so a cluster co-found only with
// regression counts as sole-contributor for the diversity role (§8.5).
func TestOverlapReport_RegressionAndBlankExcluded(t *testing.T) {
	report := OverlapReport([][]string{
		{"implementer", "regression"},   // regression stripped -> implementer sole
		{"implementer", "  ", "tester"}, // blank stripped -> implementer+tester shared
		{"regression"},                  // regression-only -> nobody
		{""},                            // blank-only -> nobody
	})

	byRole := make(map[string]RoleOverlap)
	for _, r := range report {
		byRole[r.Role] = r
	}
	require.Len(t, report, 2)
	assert.Equal(t, RoleOverlap{Role: "implementer", Unique: 1, Shared: 1, TrimCandidate: false}, byRole["implementer"])
	assert.Equal(t, RoleOverlap{Role: "tester", Unique: 0, Shared: 1, TrimCandidate: true}, byRole["tester"])
	assert.NotContains(t, byRole, "regression", "regression is never in the report")
}

// TestOverlapReport_Empty returns an empty (non-nil) slice for no clusters.
func TestOverlapReport_Empty(t *testing.T) {
	report := OverlapReport(nil)
	assert.NotNil(t, report)
	assert.Empty(t, report)
}
