package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocationClusters_MultiRoleOnly: only a location whose contributing roles
// number two or more produces an entry, and Count reports the records at the
// location rather than the roles (§8a.1).
func TestLocationClusters_MultiRoleOnly(t *testing.T) {
	out := LocationClusters([]LocationClusterRecord{
		{Location: "§1 detail", Roles: []string{"implementer"}, Severity: "high"},
		{Location: "§1 other words", Roles: []string{"tester"}, Severity: "low"},
		{Location: "§1 third", Roles: []string{"tester"}, Severity: "high"},
		{Location: "§2", Roles: []string{"architect"}, Severity: "critical"},
		{Location: "§2 again", Roles: []string{"architect"}, Severity: "medium"},
	})
	require.Len(t, out, 1, "§2 has two records but one role")
	assert.Equal(t, "§1", out[0].Location, "grouped and reported by location key")
	assert.Equal(t, []string{"implementer", "tester"}, out[0].Roles)
	assert.Equal(t, []string{"high", "low"}, out[0].Severities, "deduplicated, most severe first")
	assert.Equal(t, 3, out[0].Count)
}

// TestLocationClusters_SeverityOrder pins the severity ordering — most severe
// first, with unrecognised severities last and broken lexicographically.
func TestLocationClusters_SeverityOrder(t *testing.T) {
	out := LocationClusters([]LocationClusterRecord{
		{Location: "§1", Roles: []string{"a"}, Severity: "low"},
		{Location: "§1", Roles: []string{"b"}, Severity: "critical"},
		{Location: "§1", Roles: []string{"c"}, Severity: "zebra"},
		{Location: "§1", Roles: []string{"d"}, Severity: "medium"},
		{Location: "§1", Roles: []string{"e"}, Severity: "alpaca"},
		{Location: "§1", Roles: []string{"f"}, Severity: "high"},
	})
	require.Len(t, out, 1)
	assert.Equal(t, []string{"critical", "high", "medium", "low", "alpaca", "zebra"}, out[0].Severities)
}

// TestLocationClusters_RegressionAndBlankExcluded: the built-in regression role
// and blank roles never contribute, so a location one diversity role and
// regression reported is not a multi-role location.
func TestLocationClusters_RegressionAndBlankExcluded(t *testing.T) {
	out := LocationClusters([]LocationClusterRecord{
		{Location: "§1", Roles: []string{"implementer"}, Severity: "high"},
		{Location: "§1 words", Roles: []string{RegressionRoleID}, Severity: "high"},
		{Location: "§1 more", Roles: []string{"  "}, Severity: "high"},
		{Location: "§2", Roles: []string{" tester ", "implementer", "tester"}, Severity: "low"},
		{Location: "§2 words", Roles: []string{"architect"}, Severity: "low"},
	})
	require.Len(t, out, 1, "§1 has only one diversity role once regression and blank drop")
	assert.Equal(t, "§2", out[0].Location)
	assert.Equal(t, []string{"architect", "implementer", "tester"}, out[0].Roles, "trimmed, deduplicated, sorted")
}

