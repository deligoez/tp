package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDomainSkippedRoles names the user corpus roles whose domains omit the
// spec's domain (§9.1). With no user corpus the result is empty (the embedded
// corpus is domain-selected, never domain-filtered).
func TestDomainSkippedRoles(t *testing.T) {
	t.Run("user corpus: prose-only dropped under software", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
		revDir := filepath.Join(root, ".tp", "reviewers")
		writeRoleWithDomains(t, revDir, "sw", []string{"software"})
		writeRoleWithDomains(t, revDir, "prose", []string{"prose"})
		writeRoleWithDomains(t, revDir, "everywhere", nil)

		skipped := DomainSkippedRoles(root, "software", PhaseReviewers)
		ids := make([]string, 0, len(skipped))
		for _, s := range skipped {
			ids = append(ids, s.Role)
			assert.Equal(t, SkipDomainMismatch, s.Reason)
		}
		assert.Equal(t, []string{"prose"}, ids)
	})

	t.Run("no user corpus: empty (embedded defaults never domain-skipped)", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
		assert.Empty(t, DomainSkippedRoles(root, "software", PhaseReviewers))
	})
}
