package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRoleWithDomains writes a role file with an explicit domains list.
func writeRoleWithDomains(t *testing.T, dir, stem string, domains []string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	role := `{"id":"` + stem + `","title":"T","instructions":"I"`
	if domains != nil {
		role += `,"domains":[`
		for i, d := range domains {
			if i > 0 {
				role += ","
			}
			role += `"` + d + `"`
		}
		role += `]`
	}
	role += `}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, stem+".json"), []byte(role), 0o600))
}

// TestResolveActiveCorpus_DomainFiltering

// TestResolveActiveCorpus_DomainFiltering filters user role files by the spec's
// domain: a role omitting the domain is dropped; a role with no domains applies.
func TestResolveActiveCorpus_DomainFiltering(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	revDir := filepath.Join(root, ".tp", "reviewers")
	writeRoleWithDomains(t, revDir, "sw-only", []string{"software"})
	writeRoleWithDomains(t, revDir, "prose-only", []string{"prose"})
	writeRoleWithDomains(t, revDir, "everywhere", nil) // no domains

	roles, warnings, err := ResolveActiveCorpus(root, "software", PhaseReviewers)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	got := make([]string, len(roles))
	for i := range roles {
		got[i] = roles[i].ID
	}
	assert.ElementsMatch(t, []string{"sw-only", "everywhere"}, got, "prose-only is filtered out; no-domains applies")
}

// TestResolveActiveCorpus_EmptyPanelFallback falls back to the embedded default
// when filtering empties the user panel (§6.2), and prose gets the leaner panel.
func TestResolveActiveCorpus_EmptyPanelFallback(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	revDir := filepath.Join(root, ".tp", "reviewers")
	// Every user role is prose-only, but the spec domain is software.
	writeRoleWithDomains(t, revDir, "prose-a", []string{"prose"})
	writeRoleWithDomains(t, revDir, "prose-b", []string{"prose"})

	roles, warnings, err := ResolveActiveCorpus(root, "software", PhaseReviewers)
	require.NoError(t, err)
	got := make([]string, len(roles))
	for i := range roles {
		got[i] = roles[i].ID
	}
	assert.Equal(t, []string{"implementer", "tester", "architect"}, got, "falls back to the full software embedded panel, not re-filtered")
	joined := ""
	for _, w := range warnings {
		joined += w + "\n"
	}
	assert.Contains(t, joined, "filtered out every reviewers role")
}

// TestResolveActiveCorpus_EmbeddedByDomain selects the embedded corpus by domain
// when no user files exist; prose is the leaner two-reviewer panel (§6.3).
func TestResolveActiveCorpus_EmbeddedByDomain(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	sw, warnings, err := ResolveActiveCorpus(root, "software", PhaseReviewers)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, sw, 3)

	prose, _, err := ResolveActiveCorpus(root, "prose", PhaseReviewers)
	require.NoError(t, err)
	got := make([]string, len(prose))
	for i := range prose {
		got[i] = prose[i].ID
	}
	assert.Equal(t, []string{"coherence", "soundness"}, got, "prose defaults to the leaner two-reviewer panel")
}

// TestResolveActiveCorpus_UnknownDomainWarns falls back to the software corpus
// with a lint warning for an unknown domain (§6.1).
func TestResolveActiveCorpus_UnknownDomainWarns(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	roles, warnings, err := ResolveActiveCorpus(root, "legal", PhaseReviewers)
	require.NoError(t, err)
	got := make([]string, len(roles))
	for i := range roles {
		got[i] = roles[i].ID
	}
	assert.Equal(t, []string{"implementer", "tester", "architect"}, got, "unknown domain uses the software corpus")
	joined := ""
	for _, w := range warnings {
		joined += w + "\n"
	}
	assert.Contains(t, joined, `unknown domain "legal"`)
}
