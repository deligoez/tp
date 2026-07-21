package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClusterFindings_EqualKeys clusters findings that share both location key
// and class, and keeps distinct keys apart (§8.3).
func TestClusterFindings_EqualKeys(t *testing.T) {
	findings := []ClusterFinding{
		{Location: "§8.2 detail", Class: "dedup-gap", Role: "implementer"},
		{Location: "§8.2 other wording", Class: "dedup-gap", Role: "tester"},   // same (key, class) -> cluster with 0
		{Location: "§8 overview", Class: "dedup-gap", Role: "architect"},       // §8 != §8.2 -> own cluster
		{Location: "§8.2 yet another", Class: "attribution", Role: "reviewer"}, // same key, different class -> own cluster
	}
	clusters := ClusterFindings(findings)
	require.Len(t, clusters, 3)
	assert.Equal(t, []int{0, 1}, clusters[0].Members, "§8.2+dedup-gap merges 0 and 1")
	assert.Equal(t, []int{2}, clusters[1].Members, "§8 is a distinct key")
	assert.Equal(t, []int{3}, clusters[2].Members, "same key, different class stays apart")
}

// TestClusterFindings_TrimRule treats class values equal after trimming
// surrounding whitespace (§8.3).
func TestClusterFindings_TrimRule(t *testing.T) {
	findings := []ClusterFinding{
		{Location: "§9", Class: "stale"},
		{Location: "§9", Class: "  stale  "},
	}
	clusters := ClusterFindings(findings)
	require.Len(t, clusters, 1)
	assert.Equal(t, []int{0, 1}, clusters[0].Members)
}

// TestClusterFindings_AbsentClassSingleton makes every absent-class finding its
// own cluster so an empty key cannot collapse unrelated findings (§8.3).
func TestClusterFindings_AbsentClassSingleton(t *testing.T) {
	findings := []ClusterFinding{
		{Location: "free-form location A", Class: ""},
		{Location: "free-form location B", Class: "   "}, // whitespace-only class is absent
		{Location: "free-form location A", Class: ""},    // same empty location key, still its own cluster
	}
	clusters := ClusterFindings(findings)
	require.Len(t, clusters, 3, "each absent-class finding is its own singleton")
	assert.Equal(t, []int{0}, clusters[0].Members)
	assert.Equal(t, []int{1}, clusters[1].Members)
	assert.Equal(t, []int{2}, clusters[2].Members)
}
