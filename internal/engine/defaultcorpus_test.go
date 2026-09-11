package engine

import (
	"strings"
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

// reviewerReportRule is the paragraph every software reviewer's instructions end
// with: a review reports only what would make the implementation wrong, detail
// the code will settle is not a finding, and a high finding carries a
// counterexample.
const reviewerReportRule = "Report only what would make the implementation wrong: a requirement that contradicts another, contradicts the spec's stated goal, or cannot be satisfied. Detail the code will settle — exact exit codes, JSON field names, error hints, helper names — is not a finding: the implementation decides it and its tests pin it. A `high` finding must carry a counterexample in `evidence`: two quoted spec lines that contradict each other, or a command and its output. Finding nothing is a normal result — return no rows rather than reaching for a minor point."

// TestDefaultCorpus_SoftwarePersonasNeutral pins the software reviewer
// instructions verbatim, so a change to the persona text every project without
// role files receives is deliberate, and emission stays neutral (§13.1): the
// architect's embedded focus asks generic questions, never tp's own invariants.
func TestDefaultCorpus_SoftwarePersonasNeutral(t *testing.T) {
	roles, err := DefaultCorpus("software", PhaseReviewers)
	require.NoError(t, err)
	byID := map[string]string{}
	focusByID := map[string][]string{}
	for i := range roles {
		byID[roles[i].ID] = roles[i].Instructions
		focusByID[roles[i].ID] = roles[i].Focus
		assert.True(t, strings.HasSuffix(roles[i].Instructions, " "+reviewerReportRule),
			"%s instructions end with the report rule", roles[i].ID)
	}
	assert.Equal(t, "You are a senior engineer who will build this tomorrow. Find the requirements that cannot be built as written — ones that contradict each other, contradict the stated goal, or depend on something the spec itself says does not exist. Cite a § location for every finding. "+reviewerReportRule, byID["implementer"])
	assert.Equal(t, "You are the engineer who will write the acceptance tests. Find requirements where two correct implementations would still fail each other's tests — a real ambiguity about what the behaviour is, not about how it is spelled. Cite a § location for every finding. "+reviewerReportRule, byID["tester"])
	assert.Equal(t, []string{
		"Does any section contradict another, or change existing behaviour without saying so?",
		"Which existing behaviour would break that the spec does not say it changes?",
		"Which section could be removed without changing a decision?",
	}, focusByID["architect"])
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
