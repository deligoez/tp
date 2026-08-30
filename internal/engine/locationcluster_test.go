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

