package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultCorpus_MatchesTable asserts the embedded default corpus parses and
// matches the §5.1 table (ids and panel order) for both domains and both phases.
func TestDefaultCorpus_MatchesTable(t *testing.T) {
	cases := []struct {
		domain, phase string
		wantIDs       []string
	}{
		{"software", PhaseReviewers, []string{"implementer", "tester", "architect"}},
		{"software", PhaseAuditors, []string{"spec-coverage", "security", "maintainability-conventions"}},
		{"prose", PhaseReviewers, []string{"coherence", "soundness"}},
		{"prose", PhaseAuditors, []string{"spec-coverage", "soundness"}},
	}
	for _, c := range cases {
		roles, err := DefaultCorpus(c.domain, c.phase)
		require.NoError(t, err, "%s/%s parses", c.domain, c.phase)

		got := make([]string, len(roles))
		for i := range roles {
			got[i] = roles[i].ID
			assert.NotEmpty(t, roles[i].Title, "%s has a title", roles[i].ID)
			assert.NotEmpty(t, roles[i].Instructions, "%s has instructions", roles[i].ID)
			assert.Empty(t, roles[i].Domains, "embedded roles are partitioned by the §5.1 table, not a domains field")
		}
		assert.Equal(t, c.wantIDs, got, "%s/%s matches the §5.1 table in panel order", c.domain, c.phase)
	}
}

// TestDefaultCorpus_SoftwarePersonasNeutral spot-checks that the software
// reviewer instructions carry the pre-v0.25.0 persona text verbatim, so emission
// stays neutral (§13.1).
func TestDefaultCorpus_SoftwarePersonasNeutral(t *testing.T) {
	roles, err := DefaultCorpus("software", PhaseReviewers)
	require.NoError(t, err)
	byID := map[string]string{}
	for i := range roles {
		byID[roles[i].ID] = roles[i].Instructions
	}
	assert.Equal(t, "You are a senior engineer who must implement this spec tomorrow. Your goal is to find requirements that are missing, underspecified, or impossible to implement as stated.", byID["implementer"])
	assert.Equal(t, "You are a QA engineer who must write tests from this spec. Your goal is to find requirements that are ambiguous (two testers would write contradictory tests) or non-verifiable (cannot write a pass/fail test).", byID["tester"])
}

// TestDefaultCorpus_DomainsAndErrors covers the domain listing and the unknown
// domain/phase errors.
func TestDefaultCorpus_DomainsAndErrors(t *testing.T) {
	assert.Equal(t, []string{"software", "prose"}, DefaultCorpusDomains())
	assert.True(t, HasDefaultCorpus("software"))
	assert.True(t, HasDefaultCorpus("prose"))
	assert.False(t, HasDefaultCorpus("legal"))

	_, err := DefaultCorpus("legal", PhaseReviewers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embedded corpus for domain")

	_, err = DefaultCorpus("software", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown phase")
}
