package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFindingCluster_RepresentativeOrdering exercises the §8.4 total order:
// highest severity, then lexicographic role, then finding text.
func TestFindingCluster_RepresentativeOrdering(t *testing.T) {
	t.Run("highest severity wins", func(t *testing.T) {
		fs := []ClusterFinding{
			{Severity: "low", Role: "tester", Finding: "a"},
			{Severity: "critical", Role: "tester", Finding: "b"},
			{Severity: "medium", Role: "tester", Finding: "c"},
		}
		c := FindingCluster{Members: []int{0, 1, 2}}
		assert.Equal(t, 1, c.Representative(fs))
	})

	t.Run("lexicographic role breaks a severity tie", func(t *testing.T) {
		fs := []ClusterFinding{
			{Severity: "high", Role: "tester", Finding: "z"},
			{Severity: "high", Role: "architect", Finding: "y"},
		}
		c := FindingCluster{Members: []int{0, 1}}
		assert.Equal(t, 1, c.Representative(fs), "architect < tester")
	})

	t.Run("finding text breaks a same-role severity tie deterministically", func(t *testing.T) {
		fs := []ClusterFinding{
			{Severity: "high", Role: "tester", Finding: "beta"},
			{Severity: "high", Role: "tester", Finding: "alpha"},
		}
		c := FindingCluster{Members: []int{0, 1}}
		assert.Equal(t, 1, c.Representative(fs), "alpha < beta")
	})

	t.Run("empty cluster returns -1", func(t *testing.T) {
		assert.Equal(t, -1, FindingCluster{Members: []int{}}.Representative(nil))
	})
}

// TestFindingCluster_AttributionExclusion covers found_by_roles as the sorted set
// of distinct diversity roles with regression and blank roles excluded (§8.4).
func TestFindingCluster_AttributionExclusion(t *testing.T) {
	fs := []ClusterFinding{
		{Role: "tester"},
		{Role: "implementer"},
		{Role: "tester"},       // duplicate collapses
		{Role: "regression"},   // built-in excluded
		{Role: "  "},           // blank excluded
		{Role: " architect  "}, // trimmed before dedup/sort
	}
	c := FindingCluster{Members: []int{0, 1, 2, 3, 4, 5}}
	roles, count := c.Attribution(fs)
	assert.Equal(t, []string{"architect", "implementer", "tester"}, roles, "sorted distinct diversity roles")
	assert.Equal(t, 3, count)
}

// TestFindingCluster_AttributionZero covers the found_by-zero case: a cluster
// produced only by regression and/or absent-role findings has found_by 0 and no
// found_by_roles (§8.4).
func TestFindingCluster_AttributionZero(t *testing.T) {
	fs := []ClusterFinding{
		{Role: "regression"},
		{Role: ""},
		{Role: "regression"},
	}
	c := FindingCluster{Members: []int{0, 1, 2}}
	roles, count := c.Attribution(fs)
	assert.Empty(t, roles)
	assert.Equal(t, 0, count)
}
